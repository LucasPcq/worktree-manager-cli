package infra

import (
	"fmt"
	"os/exec"
	"strings"
)

// CreateWorktreeParams holds inputs for creating a git worktree.
type CreateWorktreeParams struct {
	ProjectDir string
	Path       string
	Branch     string
	FromBranch string
}

// CreateWorktree creates a new git worktree. If the branch does not exist,
// it is created with -b. If it already exists, the worktree checks it out.
func CreateWorktree(params CreateWorktreeParams) error {
	if branchExists(params.ProjectDir, params.Branch) {
		return createWorktreeExisting(params)
	}
	return createWorktreeNew(params)
}

func createWorktreeNew(params CreateWorktreeParams) error {
	cmd := exec.Command("git", "worktree", "add", "-b", params.Branch, params.Path, params.FromBranch)
	cmd.Dir = params.ProjectDir
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git worktree add: %s: %w", strings.TrimSpace(string(out)), err)
	}
	return nil
}

func createWorktreeExisting(params CreateWorktreeParams) error {
	cmd := exec.Command("git", "worktree", "add", params.Path, params.Branch)
	cmd.Dir = params.ProjectDir
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git worktree add: %s: %w", strings.TrimSpace(string(out)), err)
	}
	return nil
}

// RemoveWorktreeParams holds inputs for removing a git worktree.
type RemoveWorktreeParams struct {
	ProjectDir string
	Path       string
	Force      bool
}

// RemoveWorktree removes a git worktree directory.
func RemoveWorktree(params RemoveWorktreeParams) error {
	args := []string{"worktree", "remove", params.Path}
	if params.Force {
		args = append(args, "--force")
	}

	cmd := exec.Command("git", args...)
	cmd.Dir = params.ProjectDir
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git worktree remove: %s: %w", strings.TrimSpace(string(out)), err)
	}
	return nil
}
