package rules

import "github.com/LucasPcq/wtm/internal/domain"

// RunningJobsByWorktree counts what the daemon's index holds per working
// directory, keyed the way the index itself keys it. A job the index knows but
// nothing is running is not counted: the picker annotates what is up, not what
// was once started.
func RunningJobsByWorktree(jobs []domain.JobInfo) map[string]int {
	counts := make(map[string]int)
	for _, job := range jobs {
		if !IsJobUp(job.Status) {
			continue
		}
		if job.WorkDir == "" {
			continue
		}
		counts[job.WorkDir]++
	}
	return counts
}
