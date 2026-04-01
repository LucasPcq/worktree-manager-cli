// Package infra provides I/O, git command execution, and filesystem wrappers.
package infra

import (
	"fmt"
	"os/exec"
	"strings"
)

// ListBranchesParams holds inputs for listing local branches.
type ListBranchesParams struct {
	ProjectDir string
}

// ListLocalBranches returns all local branch names sorted alphabetically.
func ListLocalBranches(params ListBranchesParams) ([]string, error) {
	cmd := exec.Command("git", "branch", "--format=%(refname:short)")
	cmd.Dir = params.ProjectDir
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("git branch: %w", err)
	}

	var branches []string
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed != "" {
			branches = append(branches, trimmed)
		}
	}

	return branches, nil
}

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

func branchExists(projectDir string, branch string) bool {
	cmd := exec.Command("git", "rev-parse", "--verify", "refs/heads/"+branch)
	cmd.Dir = projectDir
	return cmd.Run() == nil
}

// FindMainWorktreeParams holds inputs for finding the main worktree path.
type FindMainWorktreeParams struct {
	ProjectDir string
}

// FindMainWorktreePath returns the path of the main (first) worktree.
func FindMainWorktreePath(params FindMainWorktreeParams) (string, error) {
	cmd := exec.Command("git", "worktree", "list", "--porcelain")
	cmd.Dir = params.ProjectDir
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("git worktree list: %w", err)
	}

	for _, line := range strings.Split(string(out), "\n") {
		if strings.HasPrefix(line, "worktree ") {
			return strings.TrimPrefix(line, "worktree "), nil
		}
	}

	return "", fmt.Errorf("no worktree found in output")
}
