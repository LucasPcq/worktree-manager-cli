package wt

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/testutil/gittest"
)

func runWtCmd(t *testing.T, args ...string) (stdout, stderr string, err error) {
	t.Helper()
	root := &cobra.Command{Use: domain.AppName}
	root.AddGroup(
		&cobra.Group{ID: domain.CmdGroupWorktrees, Title: domain.CmdGroupWorktreesTitle},
		&cobra.Group{ID: domain.CmdGroupNavigate, Title: domain.CmdGroupNavigateTitle},
		&cobra.Group{ID: domain.CmdGroupStack, Title: domain.CmdGroupStackTitle},
	)
	for _, c := range NewCmds() {
		root.AddCommand(c)
	}
	var outBuf, errBuf bytes.Buffer
	root.SetOut(&outBuf)
	root.SetErr(&errBuf)
	root.SetArgs(args)
	err = root.Execute()
	return outBuf.String(), errBuf.String(), err
}

func TestWtCreateAndClean(t *testing.T) {
	dir := gittest.InitRepo(t)
	stateDir := filepath.Join(dir, ".git", "wtm")
	t.Setenv("WTM_PROJECT_DIR", dir)
	t.Setenv("WTM_STATE_DIR", stateDir)
	t.Setenv(domain.EnvGoFile, "")

	if err := setupMinimalConfig(t, stateDir); err != nil {
		t.Fatalf("setup config: %v", err)
	}

	branch := "feat/wt-e2e-test"

	_, _, err := runWtCmd(t, domain.CmdCreate, branch, "--from", "main", "--output", domain.OutputJSON)
	if err != nil {
		t.Fatalf("wt create: %v", err)
	}

	wtPath := filepath.Join(filepath.Dir(dir), ".trees", branch)
	if _, err := os.Stat(wtPath); err != nil {
		wtPath = filepath.Join(filepath.Dir(dir), ".trees", "feat-wt-e2e-test")
		if _, err2 := os.Stat(wtPath); err2 != nil {
			t.Fatalf("worktree dir not found at expected path: %v / %v", err, err2)
		}
	}

	_, _, err = runWtCmd(t, domain.CmdClean, branch, "--yes", "--force", "--output", domain.OutputJSON)
	if err != nil {
		t.Fatalf("wt clean: %v", err)
	}

	if _, err := os.Stat(wtPath); !os.IsNotExist(err) {
		t.Errorf("expected worktree dir to be removed, stat returned: %v", err)
	}
}

// TestCleanAxesStrictlySeparated verifies the LUC-119 bypass taxonomy for clean:
// --force is the safety axis, --yes is the confirmation axis. In JSON mode the
// confirmation cannot run, so --yes is required; --force alone is rejected. On an
// unsafe (dirty) worktree, --yes still refuses (safety kept) and directs to --force,
// while --yes --force removes it.
func TestCleanAxesStrictlySeparated(t *testing.T) {
	dir := gittest.InitRepo(t)
	stateDir := filepath.Join(dir, ".git", "wtm")
	t.Setenv("WTM_PROJECT_DIR", dir)
	t.Setenv("WTM_STATE_DIR", stateDir)
	t.Setenv(domain.EnvGoFile, "")
	if err := setupMinimalConfig(t, stateDir); err != nil {
		t.Fatalf("setup config: %v", err)
	}

	branch := "feat/axes"
	if _, _, err := runWtCmd(t, domain.CmdCreate, branch, "--from", "main", "--output", domain.OutputJSON); err != nil {
		t.Fatalf("wt create: %v", err)
	}
	wtPath := resolveWorktreePath(t, dir, branch)

	// --force alone no longer satisfies JSON mode: it is the safety axis, not a
	// confirmation bypass. The guard must ask for --yes.
	if _, _, err := runWtCmd(t, domain.CmdClean, branch, "--force", "--output", domain.OutputJSON); err == nil {
		t.Fatal("expected clean --force (no --yes) in JSON mode to be rejected")
	} else if !strings.Contains(err.Error(), "--yes") {
		t.Fatalf("expected the error to direct to --yes, got: %v", err)
	}

	// Make the worktree dirty so the safety check would refuse it.
	if err := os.WriteFile(filepath.Join(wtPath, "dirty.txt"), []byte("wip\n"), 0o644); err != nil {
		t.Fatalf("dirty the worktree: %v", err)
	}

	// --yes keeps the safety check: a dirty worktree is refused, pointing at --force.
	if _, _, err := runWtCmd(t, domain.CmdClean, branch, "--yes", "--output", domain.OutputJSON); err == nil {
		t.Fatal("expected clean --yes on a dirty worktree to be refused")
	} else if !strings.Contains(err.Error(), "--force") {
		t.Fatalf("expected the refusal to direct to --force, got: %v", err)
	}
	if _, err := os.Stat(wtPath); err != nil {
		t.Fatalf("worktree must still exist after a refused clean: %v", err)
	}

	// --yes --force lifts safety and removes the dirty worktree.
	if _, _, err := runWtCmd(t, domain.CmdClean, branch, "--yes", "--force", "--output", domain.OutputJSON); err != nil {
		t.Fatalf("clean --yes --force on a dirty worktree: %v", err)
	}
	if _, err := os.Stat(wtPath); !os.IsNotExist(err) {
		t.Errorf("expected the dirty worktree to be removed, stat: %v", err)
	}
}

func TestCleanRedirectsToBaseWhenInsideWorktree(t *testing.T) {
	dir := gittest.InitRepo(t)
	stateDir := filepath.Join(dir, ".git", "wtm")
	t.Setenv("WTM_PROJECT_DIR", dir)
	t.Setenv("WTM_STATE_DIR", stateDir)

	goFile := filepath.Join(t.TempDir(), "go-file")
	t.Setenv(domain.EnvGoFile, goFile)

	if err := setupMinimalConfig(t, stateDir); err != nil {
		t.Fatalf("setup config: %v", err)
	}

	branch := "feat/redirect-test"
	if _, _, err := runWtCmd(t, domain.CmdCreate, branch, "--from", "main", "--output", domain.OutputJSON); err != nil {
		t.Fatalf("wt create: %v", err)
	}

	wtPath := resolveWorktreePath(t, dir, branch)

	restore := chdir(t, wtPath)
	if _, _, err := runWtCmd(t, domain.CmdClean, branch, "--yes", "--force", "--output", domain.OutputJSON); err != nil {
		restore()
		t.Fatalf("wt clean: %v", err)
	}
	restore()

	got, err := os.ReadFile(goFile)
	if err != nil {
		t.Fatalf("read go-file: %v", err)
	}
	if string(got) != dir {
		t.Errorf("go-file = %q, want base repo %q", string(got), dir)
	}
}

func TestCleanDoesNotRedirectFromOtherWorktree(t *testing.T) {
	dir := gittest.InitRepo(t)
	stateDir := filepath.Join(dir, ".git", "wtm")
	t.Setenv("WTM_PROJECT_DIR", dir)
	t.Setenv("WTM_STATE_DIR", stateDir)

	goFile := filepath.Join(t.TempDir(), "go-file")
	t.Setenv(domain.EnvGoFile, goFile)

	if err := setupMinimalConfig(t, stateDir); err != nil {
		t.Fatalf("setup config: %v", err)
	}

	branch := "feat/other-test"
	if _, _, err := runWtCmd(t, domain.CmdCreate, branch, "--from", "main", "--output", domain.OutputJSON); err != nil {
		t.Fatalf("wt create: %v", err)
	}

	// Stay in the base repo while cleaning a different worktree.
	restore := chdir(t, dir)
	if _, _, err := runWtCmd(t, domain.CmdClean, branch, "--yes", "--force", "--output", domain.OutputJSON); err != nil {
		restore()
		t.Fatalf("wt clean: %v", err)
	}
	restore()

	if _, err := os.Stat(goFile); !os.IsNotExist(err) {
		t.Errorf("expected no go-file to be written, stat returned: %v", err)
	}
}

// resolveWorktreePath returns the on-disk path of the worktree for branch,
// tolerating the sanitized-name fallback used when the raw name has slashes.
func resolveWorktreePath(t *testing.T, dir, branch string) string {
	t.Helper()
	candidate := filepath.Join(filepath.Dir(dir), ".trees", branch)
	if _, err := os.Stat(candidate); err == nil {
		return candidate
	}
	sanitized := filepath.Join(filepath.Dir(dir), ".trees", strings.ReplaceAll(branch, "/", "-"))
	if _, err := os.Stat(sanitized); err != nil {
		t.Fatalf("worktree dir not found at %q or %q", candidate, sanitized)
	}
	return sanitized
}

// chdir switches into path and returns a function restoring the previous cwd.
func chdir(t *testing.T, path string) func() {
	t.Helper()
	prev, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	if err := os.Chdir(path); err != nil {
		t.Fatalf("chdir %q: %v", path, err)
	}
	return func() { _ = os.Chdir(prev) }
}

func setupMinimalConfig(t *testing.T, stateDir string) error {
	t.Helper()
	if err := os.MkdirAll(stateDir, 0o755); err != nil {
		return err
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
	return os.WriteFile(filepath.Join(stateDir, domain.ConfigFileName), []byte(content), 0o644)
}
