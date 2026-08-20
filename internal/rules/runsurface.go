package rules

import (
	"strings"

	"github.com/LucasPcq/wtm/internal/domain"
)

type RunSurfaceParams struct {
	Format string
	// TTY reports that there is a terminal to take over.
	TTY bool
	// Detach is the caller asking for their terminal back.
	Detach bool
	// Inline marks a job that holds the terminal for as long as it runs.
	Inline bool
}

// UseRunView reports whether a run command may take the terminal over with the
// full-screen job view. Everything else — a machine format, a pipe, -d, a task
// — keeps the line-by-line output that survives in the scrollback.
func UseRunView(params RunSurfaceParams) bool {
	if params.Detach || params.Inline || !params.TTY {
		return false
	}
	return IsHumanFormat(params.Format)
}

// RunsInline reports whether starting the job holds the terminal until it
// exits. A task is a foreground command: its output belongs to the scrollback,
// not to a pane that closes with the view.
func RunsInline(job domain.JobConfig) bool {
	return job.Kind == domain.JobKindTask
}

// StreamsStartOutput reports whether a job writes to the terminal while it
// starts: a task for as long as it runs, a detached launcher until it hands its
// work over. A service the daemon backgrounds says nothing on the way up.
func StreamsStartOutput(job domain.JobConfig) bool {
	return RunsInline(job) || IsDetached(job)
}

type FailureOutputParams struct {
	Results []domain.JobActionResult
	Job     string
	Output  []byte
}

// WithFailureOutput folds what the job that ended a run had printed into its
// result. A reader that never saw the live stream — a JSON consumer, a CI log —
// has the daemon's reason and nothing else, and "exit status 1" is not why.
func WithFailureOutput(params FailureOutputParams) []domain.JobActionResult {
	logs := strings.TrimSpace(string(params.Output))
	if params.Job == "" || logs == "" {
		return params.Results
	}

	results := make([]domain.JobActionResult, len(params.Results))
	copy(results, params.Results)
	for i := range results {
		if results[i].Name != params.Job || results[i].Status != domain.JobActionError {
			continue
		}
		results[i].Message = joinFailureReason(results[i].Message, logs)
	}
	return results
}

func joinFailureReason(reason string, logs string) string {
	if reason == "" {
		return logs
	}
	return reason + "\n" + logs
}
