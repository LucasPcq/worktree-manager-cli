package shared

import (
	"errors"

	"github.com/LucasPcq/wtm/internal/domain"
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
	prs, err := ghservice.ListPRs(ghservice.ListPRsParams{
		ProjectDir: projectDir,
		Filter:     domain.PRFilterAll,
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

// LoadJobsGraceful fetches the daemon's running jobs, returning nil when the daemon is not running.
func LoadJobsGraceful() []domain.JobInfo {
	socketPath := process.SocketPath()
	if !process.IsDaemonRunning(socketPath) {
		return nil
	}
	client := process.NewClient(socketPath)
	resp, err := client.Send(process.Request{Action: process.ActionList})
	if err != nil {
		return nil
	}
	return resp.Jobs
}
