package run

import (
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/tui/runpicker"
)

// `run ps` lists the jobs of every repository the daemon knows, so the action it
// dispatches has to carry the job's own worktree. Acting in the current
// directory acted on whichever worktree ps was launched from, not on the job the
// picker showed — the bug LUC-211 set out to fix.
func TestPsActionRunsWhereTheJobDoes(t *testing.T) {
	pick := runpicker.PsPickerResult{
		Name:    "api",
		Action:  runpicker.ActionPsLogs,
		WorkDir: "/somewhere/else",
	}

	params, ok := psDispatch(nil, pick)
	if !ok {
		t.Fatal("the action dispatched nothing")
	}
	if params.WorkDir != pick.WorkDir {
		t.Errorf("the action runs in %q, want the job's own directory %q", params.WorkDir, pick.WorkDir)
	}
	if params.Job != pick.Name {
		t.Errorf("the action names %q, want the picked job %q", params.Job, pick.Name)
	}
}

// A picker only ever runs on a terminal, so what it dispatches reports to a
// person rather than as a document.
func TestPsActionReportsToAPerson(t *testing.T) {
	params, ok := psDispatch(nil, runpicker.PsPickerResult{Action: runpicker.ActionPsStopAll})
	if !ok {
		t.Fatal("the action dispatched nothing")
	}
	if params.Format != domain.OutputText {
		t.Errorf("format = %q, want %q", params.Format, domain.OutputText)
	}
}

func TestPsUnknownActionRunsNothing(t *testing.T) {
	if _, ok := psDispatch(nil, runpicker.PsPickerResult{Action: "nope"}); ok {
		t.Error("an unknown action dispatched something")
	}
}
