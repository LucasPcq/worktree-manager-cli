package infra

import (
	"os/exec"
	"strings"
	"testing"

	"github.com/LucasPcq/wtm/internal/testutil/gittest"
)

func TestListLocalBranches(t *testing.T) {
	dir := gittest.InitRepo(t)
	gittest.CreateBranch(t, dir, "feature/login")
	gittest.CreateBranch(t, dir, "develop")

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

func TestCurrentBranch(t *testing.T) {
	dir := gittest.InitRepo(t)

	branch, err := CurrentBranch(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if branch == "" {
		t.Fatal("expected non-empty branch name")
	}

	if branch != "main" && branch != "master" {
		t.Errorf("expected default branch (main or master), got %s", branch)
	}
}

func TestCommitsAhead(t *testing.T) {
	dir := gittest.InitRepo(t)
	gittest.CreateBranch(t, dir, "feature")

	cmd := exec.Command("git", "checkout", "feature")
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git checkout: %s: %v", out, err)
	}

	commitCmd := exec.Command("git", "commit", "--allow-empty", "-m", "ahead")
	commitCmd.Dir = dir
	if out, err := commitCmd.CombinedOutput(); err != nil {
		t.Fatalf("git commit: %s: %v", out, err)
	}

	branchCmd := exec.Command("git", "branch", "--format=%(refname:short)")
	branchCmd.Dir = dir
	branchOut, _ := branchCmd.Output()
	var baseBranch string
	for _, b := range strings.Split(strings.TrimSpace(string(branchOut)), "\n") {
		if b != "feature" {
			baseBranch = b
			break
		}
	}

	count, err := CommitsAhead(CommitsAheadParams{
		WorktreePath: dir,
		BaseBranch:   baseBranch,
		Branch:       "feature",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if count != 1 {
		t.Errorf("expected 1 commit ahead, got %d", count)
	}
}

func TestCommitsAhead_NoneAhead(t *testing.T) {
	dir := gittest.InitRepo(t)

	branch, err := CurrentBranch(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	gittest.CreateBranch(t, dir, "no-ahead")

	count, err := CommitsAhead(CommitsAheadParams{
		WorktreePath: dir,
		BaseBranch:   branch,
		Branch:       "no-ahead",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if count != 0 {
		t.Errorf("expected 0 commits ahead, got %d", count)
	}
}

func TestDeleteLocalBranch(t *testing.T) {
	dir := gittest.InitRepo(t)
	gittest.CreateBranch(t, dir, "to-delete")

	err := DeleteLocalBranch(DeleteLocalBranchParams{
		ProjectDir: dir,
		Branch:     "to-delete",
		Force:      false,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	branches, _ := ListLocalBranches(ListBranchesParams{ProjectDir: dir})
	for _, b := range branches {
		if b == "to-delete" {
			t.Error("branch should have been deleted")
		}
	}
}
