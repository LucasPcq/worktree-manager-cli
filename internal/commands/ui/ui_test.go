package ui

import (
	"errors"
	"testing"

	"github.com/LucasPcq/wtm/internal/commands/shared"
	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/rules"
)

// buildRunParams is what threads the real working directory into the
// dashboard as Cwd — distinct from ProjectDir, which LoadConfig may have
// resolved upward from a nested worktree. dashboard.Run itself needs a real
// terminal to exercise, so this is the seam that keeps the wiring testable.
func TestBuildRunParamsThreadsTheWorkingDirectoryAsCwd(t *testing.T) {
	const cwd = "/repo/wt/feat-a"
	result := shared.ConfigResult{ProjectDir: "/repo", StateDir: "/repo/.wtm"}

	params := buildRunParams(cwd, result)

	if params.Cwd != cwd {
		t.Errorf("Cwd = %q, want %q — the directory wtm ui actually ran from", params.Cwd, cwd)
	}
	if params.ProjectDir != result.ProjectDir {
		t.Errorf("ProjectDir = %q, want %q", params.ProjectDir, result.ProjectDir)
	}
	if params.StateDir != result.StateDir {
		t.Errorf("StateDir = %q, want %q", params.StateDir, result.StateDir)
	}
}

// The test process has no terminal, so runUI can be driven straight through its
// two agent-facing refusals.
func TestUIRefusesToRunWhereItCannotBeSeen(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want error
	}{
		{"json output", []string{"--" + domain.FlagOutput, domain.OutputJSON}, domain.ErrDashboardJSON},
		{"no terminal", nil, domain.ErrDashboardNotInteractive},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cmd := NewCmd()
			cmd.SetArgs(tc.args)
			cmd.SilenceErrors, cmd.SilenceUsage = true, true

			err := cmd.Execute()

			if !errors.Is(err, tc.want) {
				t.Fatalf("err = %v, want %v", err, tc.want)
			}
			if code := rules.ExitCode(err); code != domain.ExitCodeError {
				t.Errorf("exit code = %d, want %d", code, domain.ExitCodeError)
			}
		})
	}
}

// --output json is refused before the terminal check, so an agent gets the
// message about its own invocation rather than one about the terminal.
func TestJSONRefusalWinsOverTheTerminalCheck(t *testing.T) {
	cmd := NewCmd()
	cmd.SetArgs([]string{"--" + domain.FlagOutput, domain.OutputJSON})
	cmd.SilenceErrors, cmd.SilenceUsage = true, true

	if err := cmd.Execute(); !errors.Is(err, domain.ErrDashboardJSON) {
		t.Fatalf("err = %v, want the JSON refusal", err)
	}
}

func TestUITakesNoArguments(t *testing.T) {
	cmd := NewCmd()
	cmd.SetArgs([]string{"some-branch"})
	cmd.SilenceErrors, cmd.SilenceUsage = true, true

	if err := cmd.Execute(); err == nil {
		t.Fatal("wtm ui takes no arguments")
	}
}
