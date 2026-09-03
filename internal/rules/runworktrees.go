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

// SameJobNames says whether two readings of run.toml declare the same jobs. It
// is what a surface caching a per-worktree view asks before rebuilding it: the
// ports and the urls follow the names, and nothing else in the file changes
// what that view holds.
func SameJobNames(before, after domain.RunConfig) bool {
	if len(before.Jobs) != len(after.Jobs) {
		return false
	}
	for index, job := range before.Jobs {
		if after.Jobs[index].Name != job.Name {
			return false
		}
	}
	return true
}
