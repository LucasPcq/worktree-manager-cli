// Package infra provides I/O, git command execution, and filesystem wrappers.
package infra

import (
	"fmt"
	"os/exec"
	"strconv"
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

// GitWorktree represents a worktree entry from git worktree list.
type GitWorktree struct {
	Path   string
	Branch string
	IsMain bool
}

// ListWorktreesParams holds inputs for listing worktrees.
type ListWorktreesParams struct {
	ProjectDir string
}

// ListWorktrees returns all git worktrees with their path and branch.
func ListWorktrees(params ListWorktreesParams) ([]GitWorktree, error) {
	cmd := exec.Command("git", "worktree", "list", "--porcelain")
	cmd.Dir = params.ProjectDir
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("git worktree list: %w", err)
	}

	var worktrees []GitWorktree
	var current GitWorktree
	isFirst := true

	for _, line := range strings.Split(string(out), "\n") {
		switch {
		case strings.HasPrefix(line, "worktree "):
			if current.Path != "" {
				worktrees = append(worktrees, current)
			}
			current = GitWorktree{
				Path:   strings.TrimPrefix(line, "worktree "),
				IsMain: isFirst,
			}
			isFirst = false
		case strings.HasPrefix(line, "branch "):
			ref := strings.TrimPrefix(line, "branch ")
			parts := strings.Split(ref, "/")
			current.Branch = parts[len(parts)-1]
		}
	}

	if current.Path != "" {
		worktrees = append(worktrees, current)
	}

	return worktrees, nil
}

// IsDirtyParams holds inputs for checking worktree dirty state.
type IsDirtyParams struct {
	WorktreePath string
}

// IsDirty checks if a worktree has uncommitted changes.
func IsDirty(params IsDirtyParams) (bool, error) {
	cmd := exec.Command("git", "-C", params.WorktreePath, "status", "--porcelain")
	out, err := cmd.Output()
	if err != nil {
		return false, fmt.Errorf("git status: %w", err)
	}
	return strings.TrimSpace(string(out)) != "", nil
}

// CommitsAheadParams holds inputs for counting commits ahead.
type CommitsAheadParams struct {
	WorktreePath string
	BaseBranch   string
	Branch       string
}

// CommitsAhead returns how many commits a branch is ahead of the base branch.
// Returns 0 if the base branch doesn't exist or on error.
func CommitsAhead(params CommitsAheadParams) (int, error) {
	cmd := exec.Command("git", "-C", params.WorktreePath, "rev-list", "--count",
		params.BaseBranch+".."+params.Branch)
	out, err := cmd.Output()
	if err != nil {
		return 0, nil
	}
	count, err := strconv.Atoi(strings.TrimSpace(string(out)))
	if err != nil {
		return 0, nil
	}
	return count, nil
}
