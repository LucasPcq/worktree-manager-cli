package shared

import (
	"github.com/LucasPcq/wtm/internal/domain"
	ghservice "github.com/LucasPcq/wtm/internal/service/github"
	"github.com/LucasPcq/wtm/internal/service/process"
)

// LoadPRsGraceful fetches open PRs for the project, returning nil on error.
func LoadPRsGraceful(projectDir string) []domain.PRInfo {
	prs, err := ghservice.ListPRs(ghservice.ListPRsParams{
		ProjectDir: projectDir,
		Filter:     domain.PRFilterAll,
	})
	if err != nil {
		return nil
	}
	return prs
}

// LoadJobsGraceful fetches the daemon's running jobs, returning nil when the daemon is not running.
func LoadJobsGraceful() []process.JobInfo {
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
