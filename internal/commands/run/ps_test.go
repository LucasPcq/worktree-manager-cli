package run

import (
	"strings"
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/service/worktree"
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

// The re-exec inherits this terminal, so a sub-command left with no positional
// would open its own worktree picker and let the reader undo the job they just
// chose. The branch is named alongside the directory.
func TestPsActionNamesTheWorktree(t *testing.T) {
	projectDir := setupWorktrees(t)

	pick := runpicker.PsPickerResult{
		Name:    "api",
		Action:  runpicker.ActionPsStop,
		WorkDir: projectDir,
	}

	args, _, ok := psInvocation(pick)
	if !ok {
		t.Fatal("the action produced no command")
	}
	branch, err := worktree.CurrentBranch(worktree.CurrentBranchParams{Dir: projectDir})
	if err != nil {
		t.Fatalf("branch: %v", err)
	}
	if got := strings.Join(args, " "); !strings.Contains(got, " "+branch+" ") {
		t.Errorf("args %v do not name the worktree %q, so the child would open a picker", args, branch)
	}
}

// A worktree git cannot name — a detached HEAD, or a directory outside any
// repository — falls back to the directory alone rather than passing a bogus
// positional the child would fail to resolve.
func TestPsActionOmitsAnUnnameableWorktree(t *testing.T) {
	args, dir, ok := psInvocation(runpicker.PsPickerResult{
		Name:    "api",
		Action:  runpicker.ActionPsStop,
		WorkDir: t.TempDir(),
	})
	if !ok {
		t.Fatal("the action produced no command")
	}
	if len(args) != 4 {
		t.Errorf("args %v carry a positional for a worktree git cannot name", args)
	}
	if dir == "" {
		t.Error("the command lost the job's directory as well")
	}
}
