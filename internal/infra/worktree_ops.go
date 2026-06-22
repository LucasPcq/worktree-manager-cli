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

// MoveWorktreeParams holds inputs for moving a git worktree to a new path.
type MoveWorktreeParams struct {
	ProjectDir string
	From       string
	To         string
	Force      bool
}

// MoveWorktree relocates a git worktree directory via `git worktree move`. The
// parent directory of To must already exist. Force passes `--force` twice, which
// git requires to move a locked worktree (and is a harmless no-op otherwise).
func MoveWorktree(params MoveWorktreeParams) error {
	args := []string{"worktree", "move"}
	if params.Force {
		args = append(args, "--force", "--force")
	}
	args = append(args, params.From, params.To)

	cmd := exec.Command("git", args...)
	cmd.Dir = params.ProjectDir
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git worktree move: %s: %w", strings.TrimSpace(string(out)), err)
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
