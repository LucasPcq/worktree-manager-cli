package prune

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/LucasPcq/wtm/internal/cmd/prune/wizard"
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

// mockPruneWizard implements the prune Wizard port and records the prompt it received.
type mockPruneWizard struct {
	result wizard.PruneChoice
	err    error
	called bool
	prompt wizard.PrunePrompt
}

func (m *mockPruneWizard) Run(p wizard.PrunePrompt) (wizard.PruneChoice, error) {
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

func gitRun(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %s: %s: %v", strings.Join(args, " "), out, err)
	}
}

// seedGoneWorktree creates a worktree on branch whose configured upstream has no
// remote-tracking ref, so `git for-each-ref --format=%(upstream:track)` reports
// [gone]. This yields a hermetic --gone prune candidate — no gh, no network. The
// origin URL points at the local repo (never a GitHub host) so any gh PR probe
// fails fast and gracefully.
func seedGoneWorktree(t *testing.T, source, branch string) {
	t.Helper()
	wtPath := filepath.Join(t.TempDir(), branch)
	gitRun(t, source, "worktree", "add", "-q", "-b", branch, wtPath, "HEAD")
	if err := exec.Command("git", "-C", source, "remote", "add", "origin", source).Run(); err != nil {
		// remote may already exist across multiple seeds; ignore.
		_ = err
	}
	gitRun(t, source, "config", "branch."+branch+".remote", "origin")
	gitRun(t, source, "config", "branch."+branch+".merge", "refs/heads/"+branch)
}

func newTestCmd(io *iostreams.IOStreams, pr prompter.Prompter, w wizard.Wizard, runF func(*PruneOptions) error) *cobra.Command {
	f := &cmdutil.Factory{
		IOStreams: io,
		Prompter:  pr,
		Progress:  stubProgress{},
		Config: func(dir string) (shared.ConfigResult, error) {
			return shared.LoadConfigDir(dir)
		},
	}
	cmd := NewCmdPrune(f, w, runF)
	cmd.SilenceUsage = true
	cmd.SilenceErrors = true
	return cmd
}

// Family 1: flags resolve into PruneOptions; an explicit reason flag disables the
// default-broad expansion; non-TTY → non-interactive.
func TestPrune_ResolvesFlags(t *testing.T) {
	setupRepo(t)
	io, _, _ := iostreams.Test()

	var captured *PruneOptions
	cmd := newTestCmd(io, &mockPrompter{}, &mockPruneWizard{}, func(o *PruneOptions) error {
		captured = o
		return nil
	})
	cmd.SetArgs([]string{"--merged", "--force", "--reparent-children"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}
	if captured == nil {
		t.Fatal("runF was not called")
	}
	if !captured.Merged || !captured.Force || !captured.ReparentChildren {
		t.Fatalf("flags poorly resolved: %+v", captured)
	}
	if captured.Closed || captured.Gone {
		t.Errorf("explicit --merged must not broaden to closed/gone: %+v", captured)
	}
	if captured.Interactive {
		t.Error("expected non-interactive (no TTY)")
	}
}

// Family 1b: with no reason flag, prune considers every finished worktree.
func TestPrune_DefaultBroadReasons(t *testing.T) {
	setupRepo(t)
	io, _, _ := iostreams.Test()

	var captured *PruneOptions
	cmd := newTestCmd(io, &mockPrompter{}, &mockPruneWizard{}, func(o *PruneOptions) error {
		captured = o
		return nil
	})
	cmd.SetArgs([]string{})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}
	if !captured.Merged || !captured.Closed || !captured.Gone {
		t.Errorf("no reason flag should broaden to all three: %+v", captured)
	}
}

// Family 2: the confirmation gate — non-interactive without --yes must error with a
// FlagError, never destroy silently, and never open the wizard.
func TestPrune_NonInteractiveWithoutYesErrors(t *testing.T) {
	setupRepo(t)
	io, _, _ := iostreams.Test()
	mw := &mockPruneWizard{}

	cmd := newTestCmd(io, &mockPrompter{}, mw, nil)
	cmd.SetArgs([]string{})

	err := cmd.Execute()
	var fe cmdutil.FlagError
	if !errors.As(err, &fe) {
		t.Fatalf("expected a FlagError (confirmation required), got %T: %v", err, err)
	}
	if mw.called {
		t.Error("wizard must not run in non-interactive mode")
	}
}

func TestPrune_JSONWithoutYesOrDryRunErrors(t *testing.T) {
	setupRepo(t)
	io, _, _ := iostreams.Test()

	cmd := newTestCmd(io, &mockPrompter{}, &mockPruneWizard{}, nil)
	cmd.SetArgs([]string{"--output", "json"})

	err := cmd.Execute()
	var fe cmdutil.FlagError
	if !errors.As(err, &fe) {
		t.Fatalf("expected a FlagError for --output json without --yes, got %v", err)
	}
}

// Family 3: --yes broadens to "full non-interactive mode" — the picker is folded off
// even on a TTY.
func TestPrune_YesFoldsOffInteractive(t *testing.T) {
	setupRepo(t)
	io, _, _ := iostreams.TestInteractive()

	var captured *PruneOptions
	cmd := newTestCmd(io, &mockPrompter{}, &mockPruneWizard{}, func(o *PruneOptions) error {
		captured = o
		return nil
	})
	cmd.SetArgs([]string{"--yes"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}
	if captured.Interactive {
		t.Error("--yes must fold off interactive mode even on a TTY")
	}
}

// Family 3b: on a real --yes run, no picker is opened (empty repo → nothing to prune).
func TestPrune_YesRunsWithoutPicker(t *testing.T) {
	setupRepo(t)
	io, out, _ := iostreams.TestInteractive()
	mw := &mockPruneWizard{}

	cmd := newTestCmd(io, &mockPrompter{}, mw, nil)
	// --gone --no-fetch --force keeps the scan hermetic (no gh, no fetch).
	cmd.SetArgs([]string{"--yes", "--gone", "--no-fetch", "--force"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}
	if mw.called {
		t.Error("no picker must run under --yes")
	}
	if !strings.Contains(out.String(), "Nothing to prune") {
		t.Errorf("expected 'Nothing to prune' on an empty repo, got %q", out.String())
	}
}

// Family 4: dry-run needs neither --yes nor a TTY, and previews (dry_run:true) in JSON.
func TestPrune_DryRunJSONAllowedWithoutYes(t *testing.T) {
	source := setupRepo(t)
	seedGoneWorktree(t, source, "feat-gone")
	io, out, _ := iostreams.Test()
	mw := &mockPruneWizard{}

	cmd := newTestCmd(io, &mockPrompter{}, mw, nil)
	cmd.SetArgs([]string{"--gone", "--no-fetch", "--force", "--dry-run", "--output", "json"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}
	if !strings.Contains(out.String(), `"dry_run": true`) {
		t.Errorf("expected dry_run:true in JSON, got %q", out.String())
	}
	if mw.called {
		t.Error("wizard must not run for dry-run")
	}
}

// Family 5: the interactive path drives the wizard port with the right prompt, and
// (FIX 2) does NOT force by default.
func TestPrune_InteractiveInvokesWizard(t *testing.T) {
	source := setupRepo(t)
	seedGoneWorktree(t, source, "feat-gone")
	io, out, _ := iostreams.TestInteractive()
	mw := &mockPruneWizard{result: wizard.PruneChoice{Aborted: true}}

	cmd := newTestCmd(io, &mockPrompter{}, mw, nil)
	cmd.SetArgs([]string{"--gone", "--no-fetch"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}
	if !mw.called {
		t.Fatal("wizard was not invoked")
	}
	if len(mw.prompt.Plan.Selected) == 0 {
		t.Error("expected the gone worktree as a prune candidate in the prompt")
	}
	if mw.prompt.ReparentPreview == nil {
		t.Error("expected the reparent-preview closure wired into the prompt")
	}
	if mw.prompt.Force {
		t.Error("expected Force=false in the prompt without --force")
	}
	if !strings.Contains(out.String(), "Aborted") {
		t.Errorf("expected an Aborted message on cancel, got %q", out.String())
	}
}

// Family 6: --force is threaded into the wizard as a preset (parity with clean), so
// the picker pre-lifts refusals without re-asking.
func TestPrune_ForceThreadedIntoWizard(t *testing.T) {
	source := setupRepo(t)
	seedGoneWorktree(t, source, "feat-gone")
	io, _, _ := iostreams.TestInteractive()
	mw := &mockPruneWizard{result: wizard.PruneChoice{Aborted: true}}

	cmd := newTestCmd(io, &mockPrompter{}, mw, nil)
	cmd.SetArgs([]string{"--gone", "--no-fetch", "--force"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}
	if !mw.called {
		t.Fatal("wizard was not invoked")
	}
	if !mw.prompt.Force {
		t.Error("expected --force threaded into the wizard as a preset")
	}
}
