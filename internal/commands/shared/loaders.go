package shared

import (
	"errors"

	"github.com/LucasPcq/wtm/internal/config"
	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/rules"
	ghservice "github.com/LucasPcq/wtm/internal/service/github"
	"github.com/LucasPcq/wtm/internal/service/process"
)

// LoadPRsGraceful fetches open PRs for the project, returning nil on error.
func LoadPRsGraceful(projectDir string) []domain.PRInfo {
	prs, _ := LoadPRs(projectDir)
	return prs
}

// LoadPRs fetches open PRs for the project and reports the GitHub CLI
// connection status, distinguishing "no PRs" from "gh unavailable" so callers
// can hint the user. Returns nil PRs on any error.
func LoadPRs(projectDir string) ([]domain.PRInfo, domain.GHConnection) {
	return LoadPRsFiltered(projectDir, domain.PRFilterAll)
}

// LoadPRsWithChecks is LoadPRs plus the CI rollup and the review decision. Only
// the dashboard renders those, and asking for them costs a per-pull-request
// resolution, so the other surfaces stay on the narrow field set.
func LoadPRsWithChecks(projectDir string) ([]domain.PRInfo, domain.GHConnection) {
	return loadPRs(loadPRsParams{ProjectDir: projectDir, Filter: domain.PRFilterAll, WithChecks: true})
}

// LoadPRsFiltered is LoadPRs with an explicit filter (all, mine, review-requested).
func LoadPRsFiltered(projectDir string, filter domain.PRFilter) ([]domain.PRInfo, domain.GHConnection) {
	return loadPRs(loadPRsParams{ProjectDir: projectDir, Filter: filter})
}

type loadPRsParams struct {
	ProjectDir string
	Filter     domain.PRFilter
	WithChecks bool
}

func loadPRs(params loadPRsParams) ([]domain.PRInfo, domain.GHConnection) {
	prs, err := ghservice.ListPRs(ghservice.ListPRsParams{
		ProjectDir: params.ProjectDir,
		Filter:     params.Filter,
		WithChecks: params.WithChecks,
	})
	if err == nil {
		return prs, domain.GHConnectionOK
	}
	if errors.Is(err, domain.ErrGHNotInstalled) {
		return nil, domain.GHConnectionNotInstalled
	}
	if errors.Is(err, domain.ErrGHNotAuthenticated) {
		return nil, domain.GHConnectionNotAuthenticated
	}
	return nil, domain.GHConnectionOK
}

// LoadPRsAllStatesGraceful fetches PRs across all states (open/merged/closed)
// for the project, returning nil on any error. Used by `wtm tree --with-prs` to
// surface merged/closed PRs as clean candidates.
func LoadPRsAllStatesGraceful(projectDir string) []domain.PRInfo {
	prs, _ := ghservice.ListPRsAllStates(projectDir)
	return prs
}

// LoadJobsGraceful fetches the daemon's jobs, returning nil when there are none
// to fetch. A daemon exits once no foreground job is left, so nobody listening
// says nothing about whether detached stacks are up: when the index still holds
// some, one is started to read them back. When it holds nothing there is nothing
// to report, and no daemon is forked for it.
func LoadJobsGraceful() []domain.JobInfo {
	jobs, _ := LoadJobs()
	return jobs
}

// LoadJobs is LoadJobsGraceful for the callers whose whole output is that list.
// The error worth surfacing is a daemon of another build: reported as "no jobs",
// it would be the exact silence the version handshake exists to break.
func LoadJobs() ([]domain.JobInfo, error) {
	socketPath := process.SocketPath()
	if !process.IsDaemonRunning(socketPath) {
		if !process.HasAnyIndexedJob() {
			return nil, nil
		}
		global, err := config.LoadGlobal()
		if err != nil {
			return nil, nil
		}
		if err := process.EnsureDaemon(process.DaemonParams{
			SocketPath: socketPath,
			ProxyPort:  rules.ProxyPort(global),
		}); err != nil {
			return nil, nil
		}
	}
	resp, err := process.NewClient(socketPath).Send(process.Request{Action: process.ActionList})
	if err != nil {
		if errors.Is(err, domain.ErrDaemonVersionMismatch) {
			return nil, err
		}
		return nil, nil
	}
	return resp.Jobs, nil
}
