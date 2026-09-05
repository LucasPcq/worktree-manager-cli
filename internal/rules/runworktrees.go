package rules

import (
	"sort"

	"github.com/LucasPcq/wtm/internal/domain"
)

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

type BranchesWithJobsUpParams struct {
	Jobs     []domain.JobInfo
	Statuses []domain.WorktreeStatus
}

// BranchesWithJobsUp names the worktrees the daemon holds something up in,
// sorted so two polls of the same state ask for the same thing. A job is keyed
// on its WorkDir; the branch is what every other layer designates a worktree
// by, and a detached worktree names none.
func BranchesWithJobsUp(params BranchesWithJobsUpParams) []string {
	running := RunningJobsByWorktree(params.Jobs)
	branches := make([]string, 0, len(running))
	for _, status := range params.Statuses {
		if status.Branch == "" || running[status.Path] == 0 {
			continue
		}
		branches = append(branches, status.Branch)
	}
	sort.Strings(branches)
	return branches
}

// SameRunJobs says whether two readings of run.toml declare the same jobs, in
// the sense a surface caching a per-worktree view cares about: the name, the
// ports and the published url are exactly what WorktreeJobAddresses reads, so a
// port edited without a rename is a change too.
func SameRunJobs(before, after domain.RunConfig) bool {
	if len(before.Jobs) != len(after.Jobs) {
		return false
	}
	for index, job := range before.Jobs {
		if !sameJob(job, after.Jobs[index]) {
			return false
		}
	}
	return true
}

func sameJob(before, after domain.JobConfig) bool {
	if before.Name != after.Name || !samePorts(before.Ports, after.Ports) {
		return false
	}
	if before.URL == nil || after.URL == nil {
		return before.URL == after.URL
	}
	return *before.URL == *after.URL
}

func samePorts(before, after map[string]int) bool {
	if len(before) != len(after) {
		return false
	}
	for name, port := range before {
		if after[name] != port {
			return false
		}
	}
	return true
}

// JobsUpIn names the jobs already running in any of these worktrees. A start
// has to weigh them alongside what it is about to start: a runner started a
// moment ago holds its children just as surely as one started in the same
// gesture.
func JobsUpIn(jobs []domain.JobInfo, workDirs []string) []string {
	wanted := make(map[string]bool, len(workDirs))
	for _, dir := range workDirs {
		wanted[dir] = true
	}

	var names []string
	seen := map[string]bool{}
	for _, job := range jobs {
		if !IsJobUp(job.Status) || !wanted[job.WorkDir] || seen[job.Name] {
			continue
		}
		seen[job.Name] = true
		names = append(names, job.Name)
	}
	return names
}
