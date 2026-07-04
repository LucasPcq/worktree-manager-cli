package clean

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/LucasPcq/wtm/internal/cmd/clean/wizard"
	"github.com/LucasPcq/wtm/internal/commands/shared"
	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/progress"
	"github.com/LucasPcq/wtm/internal/prompter"
	"github.com/LucasPcq/wtm/internal/testutil/gittest"
	"github.com/LucasPcq/wtm/pkg/cmdutil"
	"github.com/LucasPcq/wtm/pkg/iostreams"
)

// mockPrompter records the generic input interactions; the wizard and the progress
// spinner are separate capabilities.
type mockPrompter struct {
	confirmResult bool
	confirmErr    error
	confirmCalled bool
}

func (m *mockPrompter) Confirm(string, bool) (bool, error) {
	m.confirmCalled = true
	return m.confirmResult, m.confirmErr
}

// stubProgress runs the work synchronously, no spinner.
type stubProgress struct{}

func (stubProgress) Run(_ progress.RunParams, work func() error) error { return work() }

// mockWizard implements the clean Wizard port and records the prompt it received.
type mockWizard struct {
	result wizard.CleanChoice
	err    error
	called bool
	prompt wizard.CleanPrompt
}

func (m *mockWizard) Run(p wizard.CleanPrompt) (wizard.CleanChoice, error) {
	m.called = true
	m.prompt = p
	return m.result, m.err
}

func setupRepo(t *testing.T) string {
	t.Helper()
	dir := gittest.InitRepo(t)
	stateDir := filepath.Join(dir, ".git", "wtm")
	t.Setenv("WTM_PROJECT_DIR", dir)
	t.Setenv("WTM_STATE_DIR", stateDir)
	t.Setenv(domain.EnvGoFile, "")

	if err := os.MkdirAll(stateDir, 0o755); err != nil {
		t.Fatalf("mkdir state dir: %v", err)
	}
	content := `#:schema ./schemas/project.schema.json
[worktrees]
base_path = "../.trees"
base_branch = "main"

[env]
strategy = "example"
copy_files = []

[hooks]
on_create = []
`
	if err := os.WriteFile(filepath.Join(stateDir, domain.ConfigFileName), []byte(content), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}
	return dir
}

func newTestCmd(io *iostreams.IOStreams, pr prompter.Prompter, w wizard.Wizard, runF func(*CleanOptions) error) *cobra.Command {
	f := &cmdutil.Factory{
		IOStreams: io,
		Prompter:  pr,
		Progress:  stubProgress{},
		Config: func(dir string) (shared.ConfigResult, error) {
			return shared.LoadConfigDir(dir)
		},
	}
	cmd := NewCmdClean(f, w, runF)
	cmd.SilenceUsage = true
	cmd.SilenceErrors = true
	return cmd
}

// Family 1: flags resolve into CleanOptions; non-TTY → non-interactive.
func TestClean_ResolvesFlags(t *testing.T) {
	setupRepo(t)
	io, _, _ := iostreams.Test()

	var captured *CleanOptions
	cmd := newTestCmd(io, &mockPrompter{}, &mockWizard{}, func(o *CleanOptions) error {
		captured = o
		return nil
	})
	cmd.SetArgs([]string{"feat", "--force", "--reparent-children"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}
	if captured == nil {
		t.Fatal("runF was not called")
	}
	if captured.Branch != "feat" || !captured.Force || !captured.ReparentChildren {
		t.Fatalf("options poorly resolved: %+v", captured)
	}
	if captured.Interactive {
		t.Error("expected non-interactive (no TTY)")
	}
}

// Family 2: the confirmation gate — non-interactive without --yes must error, never
// destroy silently, and never open the wizard.
func TestClean_NonInteractiveWithoutYesErrors(t *testing.T) {
	setupRepo(t)
	io, _, _ := iostreams.Test()
	mw := &mockWizard{}

	cmd := newTestCmd(io, &mockPrompter{}, mw, nil)
	cmd.SetArgs([]string{"feat"})

	err := cmd.Execute()
	var fe cmdutil.FlagError
	if !errors.As(err, &fe) {
		t.Fatalf("expected a FlagError (confirmation required), got %T: %v", err, err)
	}
	if mw.called {
		t.Error("wizard must not run in non-interactive mode")
	}
}

func TestClean_NonInteractiveNoBranchErrors(t *testing.T) {
	setupRepo(t)
	io, _, _ := iostreams.Test()

	cmd := newTestCmd(io, &mockPrompter{}, &mockWizard{}, nil)
	cmd.SetArgs([]string{})

	if err := cmd.Execute(); !errors.Is(err, domain.ErrCleanBranchRequired) {
		t.Fatalf("expected ErrCleanBranchRequired, got %v", err)
	}
}

func TestClean_JSONWithoutYesOrDryRunErrors(t *testing.T) {
	setupRepo(t)
	io, _, _ := iostreams.Test()

	cmd := newTestCmd(io, &mockPrompter{}, &mockWizard{}, nil)
	cmd.SetArgs([]string{"feat", "--output", "json"})

	err := cmd.Execute()
	var fe cmdutil.FlagError
	if !errors.As(err, &fe) {
		t.Fatalf("expected a FlagError for --output json without --yes, got %v", err)
	}
}

// Family 3: --yes broadens to "full non-interactive mode" — no picker even on a TTY.
func TestClean_YesWithoutBranchErrorsEvenOnTTY(t *testing.T) {
	setupRepo(t)
	io, _, _ := iostreams.TestInteractive()
	mw := &mockWizard{}

	cmd := newTestCmd(io, &mockPrompter{}, mw, nil)
	cmd.SetArgs([]string{"--yes"})

	if err := cmd.Execute(); !errors.Is(err, domain.ErrCleanBranchRequired) {
		t.Fatalf("expected ErrCleanBranchRequired under --yes without branch, got %v", err)
	}
	if mw.called {
		t.Error("no picker must run under --yes")
	}
}

// Family 4: dry-run previews without mutating and needs neither --yes nor a TTY.
func TestClean_DryRunPreviewsWithoutMutation(t *testing.T) {
	setupRepo(t)
	io, out, _ := iostreams.Test()
	mw := &mockWizard{}

	cmd := newTestCmd(io, &mockPrompter{}, mw, nil)
	cmd.SetArgs([]string{"feat", "--dry-run"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}
	got := out.String()
	if !strings.Contains(got, "dry-run: no changes made") {
		t.Errorf("missing dry-run marker: %q", got)
	}
	if !strings.Contains(got, "feat") {
		t.Errorf("expected branch in the preview: %q", got)
	}
	if mw.called {
		t.Error("wizard must not run for dry-run")
	}
}

// Regression: dry-run is intrinsically non-interactive (it never opens the picker),
// so a missing branch must error — in a pipe AND on a TTY — instead of previewing an
// empty branch name.
func TestClean_DryRunWithoutBranchErrors(t *testing.T) {
	cases := map[string]func() *iostreams.IOStreams{
		"pipe": func() *iostreams.IOStreams { io, _, _ := iostreams.Test(); return io },
		"tty":  func() *iostreams.IOStreams { io, _, _ := iostreams.TestInteractive(); return io },
	}
	for name, mkIO := range cases {
		t.Run(name, func(t *testing.T) {
			setupRepo(t)

			cmd := newTestCmd(mkIO(), &mockPrompter{}, &mockWizard{}, nil)
			cmd.SetArgs([]string{"--dry-run"})

			if err := cmd.Execute(); !errors.Is(err, domain.ErrCleanBranchRequired) {
				t.Fatalf("expected ErrCleanBranchRequired for --dry-run without branch, got %v", err)
			}
		})
	}
}

func TestClean_DryRunJSONAllowedWithoutYes(t *testing.T) {
	setupRepo(t)
	io, out, _ := iostreams.Test()

	cmd := newTestCmd(io, &mockPrompter{}, &mockWizard{}, nil)
	cmd.SetArgs([]string{"feat", "--dry-run", "--output", "json"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}
	if !strings.Contains(out.String(), `"dry_run": true`) {
		t.Errorf("expected dry_run:true in JSON, got %q", out.String())
	}
}

// Family 5: the interactive path drives the wizard port with the right prompt.
func TestClean_InteractiveInvokesWizard(t *testing.T) {
	setupRepo(t)
	io, _, _ := iostreams.TestInteractive()
	mw := &mockWizard{result: wizard.CleanChoice{Branch: "feat"}}

	cmd := newTestCmd(io, &mockPrompter{}, mw, nil)
	cmd.SetArgs([]string{"--force"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}
	if !mw.called {
		t.Fatal("wizard was not invoked")
	}
	if mw.prompt.PreselectedBranch != "" {
		t.Errorf("expected empty preselected branch (picker), got %q", mw.prompt.PreselectedBranch)
	}
	if !mw.prompt.Force {
		t.Error("expected --force threaded into the wizard as a preset")
	}
	if mw.prompt.Check == nil || mw.prompt.ReparentPreview == nil {
		t.Error("expected service-backed closures wired into the prompt")
	}
}

func TestClean_InteractiveAbortIsClean(t *testing.T) {
	setupRepo(t)
	io, out, _ := iostreams.TestInteractive()
	mw := &mockWizard{result: wizard.CleanChoice{Aborted: true}}

	cmd := newTestCmd(io, &mockPrompter{}, mw, nil)
	cmd.SetArgs([]string{})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("expected nil on user abort, got %v", err)
	}
	if !strings.Contains(out.String(), "Aborted") {
		t.Errorf("expected an Aborted message, got %q", out.String())
	}
}
