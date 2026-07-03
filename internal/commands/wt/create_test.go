package wt

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/testutil/gittest"
)

func TestWtCreateFromRemoteBranch(t *testing.T) {
	dir := gittest.InitRepo(t)
	stateDir := filepath.Join(dir, ".git", "wtm")
	t.Setenv("WTM_PROJECT_DIR", dir)
	t.Setenv("WTM_STATE_DIR", stateDir)
	t.Setenv(domain.EnvGoFile, "")

	if err := setupMinimalConfig(t, stateDir); err != nil {
		t.Fatalf("setup config: %v", err)
	}

	// Fake a remote-tracking branch that has no local counterpart.
	cmd := exec.Command("git", "update-ref", "refs/remotes/origin/upstream", "HEAD")
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git update-ref: %s: %v", out, err)
	}

	branch := "feat/from-remote"
	if _, _, err := runWtCmd(t, domain.CmdCreate, branch, "--from", "origin/upstream", "--output", domain.OutputJSON, "--"+domain.FlagYes); err != nil {
		t.Fatalf("wt create --from origin/upstream: %v", err)
	}

	if _, err := os.Stat(resolveWorktreePath(t, dir, branch)); err != nil {
		t.Fatalf("worktree not created from remote branch: %v", err)
	}
}

func TestWtCreateFromUnknownBranchFails(t *testing.T) {
	dir := gittest.InitRepo(t)
	stateDir := filepath.Join(dir, ".git", "wtm")
	t.Setenv("WTM_PROJECT_DIR", dir)
	t.Setenv("WTM_STATE_DIR", stateDir)
	t.Setenv(domain.EnvGoFile, "")

	if err := setupMinimalConfig(t, stateDir); err != nil {
		t.Fatalf("setup config: %v", err)
	}

	if _, _, err := runWtCmd(t, domain.CmdCreate, "feat/x", "--from", "origin/ghost", "--output", domain.OutputJSON, "--"+domain.FlagYes); err == nil {
		t.Fatal("expected an error for a nonexistent source branch")
	}
}
