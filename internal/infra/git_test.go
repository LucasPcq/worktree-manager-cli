package infra

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// initTestRepo creates a temporary git repo with an initial commit and returns its path.
func initTestRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()

	commands := [][]string{
		{"git", "init"},
		{"git", "config", "user.email", "test@test.com"},
		{"git", "config", "user.name", "Test"},
		{"git", "commit", "--allow-empty", "-m", "initial"},
	}

	for _, args := range commands {
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("%s failed: %s: %v", strings.Join(args, " "), out, err)
		}
	}

	return dir
}

func createBranch(t *testing.T, dir string, name string) {
	t.Helper()
	cmd := exec.Command("git", "branch", name)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git branch %s: %s: %v", name, out, err)
	}
}

func TestListLocalBranches(t *testing.T) {
	dir := initTestRepo(t)
	createBranch(t, dir, "feature/login")
	createBranch(t, dir, "develop")

	branches, err := ListLocalBranches(ListBranchesParams{ProjectDir: dir})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(branches) < 3 {
		t.Fatalf("expected at least 3 branches (main/master, feature/login, develop), got %d: %v", len(branches), branches)
	}

	found := map[string]bool{}
	for _, b := range branches {
		found[b] = true
	}

	if !found["feature/login"] {
		t.Error("missing feature/login branch")
	}
	if !found["develop"] {
		t.Error("missing develop branch")
	}
}

func TestCreateWorktreeNewBranch(t *testing.T) {
	dir := initTestRepo(t)
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
	dir := initTestRepo(t)
	createBranch(t, dir, "existing-branch")
	wtPath := filepath.Join(t.TempDir(), "existing-wt")

	err := CreateWorktree(CreateWorktreeParams{
		ProjectDir: dir,
		Path:       wtPath,
		Branch:     "existing-branch",
		FromBranch: "HEAD",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if _, err := os.Stat(wtPath); os.IsNotExist(err) {
		t.Error("worktree directory was not created")
	}
}

func TestFindMainWorktreePath(t *testing.T) {
	dir := initTestRepo(t)

	mainPath, err := FindMainWorktreePath(FindMainWorktreeParams{ProjectDir: dir})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// The main worktree path should match the repo dir (resolve symlinks for temp dirs)
	resolvedDir, _ := filepath.EvalSymlinks(dir)
	resolvedMain, _ := filepath.EvalSymlinks(mainPath)

	if resolvedMain != resolvedDir {
		t.Errorf("expected main worktree path %s, got %s", resolvedDir, resolvedMain)
	}
}
