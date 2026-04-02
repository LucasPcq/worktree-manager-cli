package infra

import (
	"fmt"
	"os/exec"
	"strings"

	"github.com/LucasPcq/wtm/internal/domain"
)

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
			current.Branch = strings.TrimPrefix(ref, "refs/heads/")
		}
	}

	if current.Path != "" {
		worktrees = append(worktrees, current)
	}

	return worktrees, nil
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

// FindWorktreeByBranchParams holds inputs for finding a worktree by branch name.
type FindWorktreeByBranchParams struct {
	ProjectDir string
	Branch     string
}

// FindWorktreeByBranch returns the worktree matching the given branch name.
func FindWorktreeByBranch(params FindWorktreeByBranchParams) (GitWorktree, error) {
	worktrees, err := ListWorktrees(ListWorktreesParams{ProjectDir: params.ProjectDir})
	if err != nil {
		return GitWorktree{}, err
	}

	for _, wt := range worktrees {
		if wt.Branch == params.Branch {
			return wt, nil
		}
	}

	return GitWorktree{}, fmt.Errorf("%w: %s", domain.ErrWorktreeNotFound, params.Branch)
}
