package infra

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/LucasPcq/wtm/internal/testutil/gittest"
)

func TestCreateWorktreeNewBranch(t *testing.T) {
	dir := gittest.InitRepo(t)
	wtPath := filepath.Join(t.TempDir(), "my-worktree")

	err := CreateWorktree(CreateWorktreeParams{
		ProjectDir: dir,
		Path:       wtPath,
		Branch:     "feat-new",
		FromBranch: "HEAD",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if _, err := os.Stat(wtPath); os.IsNotExist(err) {
		t.Error("worktree directory was not created")
	}
}

func TestCreateWorktreeExistingBranch(t *testing.T) {
	dir := gittest.InitRepo(t)
	gittest.CreateBranch(t, dir, "existing-branch")
	wtPath := filepath.Join(t.TempDir(), "existing-wt")

	err := CreateWorktree(CreateWorktreeParams{
		ProjectDir:  dir,
		Path:        wtPath,
		Branch:      "existing-branch",
		ReuseBranch: true,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if _, err := os.Stat(wtPath); os.IsNotExist(err) {
		t.Error("worktree directory was not created")
	}
}

func TestRemoveWorktree(t *testing.T) {
	dir := gittest.InitRepo(t)
	wtPath := filepath.Join(t.TempDir(), "wt-remove")

	_ = CreateWorktree(CreateWorktreeParams{
		ProjectDir: dir,
		Path:       wtPath,
		Branch:     "feat-remove",
		FromBranch: "HEAD",
	})

	err := RemoveWorktree(RemoveWorktreeParams{
		ProjectDir: dir,
		Path:       wtPath,
		Force:      false,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if _, statErr := os.Stat(wtPath); !os.IsNotExist(statErr) {
		t.Error("worktree directory should be removed")
	}
}
