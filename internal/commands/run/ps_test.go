package run

import (
	"strings"
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/tui/runpicker"
)

// `run ps` lists the jobs of every repository the daemon knows. Re-entering the
// binary in the current directory acted on whichever worktree ps was launched
// from, not on the job the picker showed — the bug LUC-211 set out to fix.
func TestPsActionRunsWhereTheJobDoes(t *testing.T) {
	pick := runpicker.PsPickerResult{
		Name:    "api",
		Action:  runpicker.ActionPsLogs,
		WorkDir: "/somewhere/else",
	}

	args, dir, ok := psInvocation(pick)
	if !ok {
		t.Fatal("the action produced no command")
	}
	if dir != pick.WorkDir {
		t.Errorf("the command runs in %q, want the job's own directory %q", dir, pick.WorkDir)
	}
	if want := "--" + domain.FlagJob + " api"; !strings.Contains(strings.Join(args, " "), want) {
		t.Errorf("args %v do not name the job with %q", args, want)
	}
}

func TestPsStopAllNeedsNoJob(t *testing.T) {
	args, _, ok := psInvocation(runpicker.PsPickerResult{Action: runpicker.ActionPsStopAll})
	if !ok {
		t.Fatal("the action produced no command")
	}
	if joined := strings.Join(args, " "); !strings.Contains(joined, "--"+domain.FlagAll) {
		t.Errorf("args %v do not carry --%s", args, domain.FlagAll)
	}
}

func TestPsUnknownActionRunsNothing(t *testing.T) {
	if _, _, ok := psInvocation(runpicker.PsPickerResult{Action: "nope"}); ok {
		t.Error("an unknown action produced a command to run")
	}
}
