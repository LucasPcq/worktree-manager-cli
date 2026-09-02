package rules

import "github.com/LucasPcq/wtm/internal/domain"

type ReconcileJobParams struct {
	Record domain.JobRecord
	// WorkDirExists says whether the worktree the job was started in is still
	// there. Its stop command runs in that directory, so an entry pointing at a
	// deleted one can only be dropped.
	WorkDirExists bool
}

// ReconcileDecision is what becomes of one indexed job when a daemon reads the
// index back. Adopt false drops the entry; no decision ever stops or restarts
// anything, which is what keeps a daemon start-up free of side effects.
type ReconcileDecision struct {
	Status domain.JobStatus
	Adopt  bool
}

// ReconcileJob decides what a daemon makes of an indexed job at start-up. It
// verifies nothing: the truth belongs to whoever owns the process — Docker for
// a detached stack — and a detached entry says what wtm actually knows, which
// is that it launched the job and has not seen it since.
func ReconcileJob(params ReconcileJobParams) ReconcileDecision {
	if !params.WorkDirExists {
		return ReconcileDecision{}
	}
	if params.Record.Config.Kind != domain.JobKindService {
		return ReconcileDecision{}
	}
	if IsDetached(params.Record.Config) {
		return ReconcileDecision{Status: domain.JobStatusDetached, Adopt: true}
	}
	// A foreground service is drained through a PTY the daemon owns, so it died
	// with it. Reported rather than hidden: its log is on disk and `run logs`
	// reads it.
	return ReconcileDecision{Status: domain.JobStatusCrashed, Adopt: true}
}

// IsIndexedJobStatus reports whether a job in this state belongs in the durable
// index. The index holds what is live, not what has been — which is also why a
// stop needs no explicit purge: the entry leaves with the state.
func IsIndexedJobStatus(status domain.JobStatus) bool {
	return status == domain.JobStatusRunning || status == domain.JobStatusDetached
}
