package wt

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/testutil/gittest"
)

func runWtCmd(t *testing.T, args ...string) (stdout, stderr string, err error) {
	t.Helper()
	cmd := NewCmd()
	var outBuf, errBuf bytes.Buffer
	cmd.SetOut(&outBuf)
	cmd.SetErr(&errBuf)
	cmd.SetArgs(args)
	err = cmd.Execute()
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

	_, _, err = runWtCmd(t, domain.CmdClean, branch, "--force", "--output", domain.OutputJSON)
	if err != nil {
		t.Fatalf("wt clean: %v", err)
	}

	if _, err := os.Stat(wtPath); !os.IsNotExist(err) {
		t.Errorf("expected worktree dir to be removed, stat returned: %v", err)
	}
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

[github]
auto_draft = false

[agents]

[integrations]
vscode_project_manager = false
cursor_project_manager = false
`
	return os.WriteFile(filepath.Join(stateDir, domain.ConfigFileName), []byte(content), 0o644)
}
