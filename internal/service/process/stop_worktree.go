package process

import "github.com/LucasPcq/wtm/internal/rules"

// StopWorktreeJobs stops every job attached to workDir via the daemon, starting
// one if none is listening: a detached stack outlives the daemon that launched
// it, so the answer to "are there jobs here" lives in the index, not in whoever
// happens to be running. Returns true if a stop request was sent.
func StopWorktreeJobs(client *Client, workDir string) bool {
	resp, err := client.Send(Request{Action: ActionList})
	if err != nil {
		return false
	}

	for _, svc := range resp.Jobs {
		if svc.WorkDir == workDir && rules.IsJobUp(svc.Status) {
			_, _ = client.Send(Request{Action: ActionStopAll, WorkDir: workDir})
			return true
		}
	}

	return false
}
