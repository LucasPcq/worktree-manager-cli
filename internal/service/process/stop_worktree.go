package process

import "github.com/LucasPcq/wtm/internal/domain"

// StopWorktreeJobs stops all running jobs attached to workDir via the daemon.
// Returns true if a stop request was sent, false if the daemon was not running
// or no jobs were active for that workDir.
func StopWorktreeJobs(client *Client, workDir string) bool {
	resp, err := client.Send(Request{Action: ActionList})
	if err != nil {
		return false
	}

	for _, svc := range resp.Jobs {
		if svc.WorkDir == workDir && svc.Status == domain.JobStatusRunning {
			_, _ = client.Send(Request{Action: ActionStopAll, WorkDir: workDir})
			return true
		}
	}

	return false
}
