package ui

import (
	"errors"
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/rules"
)

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
