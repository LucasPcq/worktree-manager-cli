// Package infra provides I/O, git command execution, and filesystem wrappers.
package infra

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"strconv"
	"strings"

	"github.com/LucasPcq/wtm/internal/domain"
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
			current.Branch = strings.TrimPrefix(ref, "refs/heads/")
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

// DeleteLocalBranchParams holds inputs for deleting a local branch.
type DeleteLocalBranchParams struct {
	ProjectDir string
	Branch     string
	Force      bool
}

// DeleteLocalBranch deletes a local git branch.
func DeleteLocalBranch(params DeleteLocalBranchParams) error {
	flag := "-d"
	if params.Force {
		flag = "-D"
	}

	cmd := exec.Command("git", "branch", flag, params.Branch)
	cmd.Dir = params.ProjectDir
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git branch %s: %s: %w", flag, strings.TrimSpace(string(out)), err)
	}
	return nil
}

// UnpushedCommitsParams holds inputs for counting unpushed commits.
type UnpushedCommitsParams struct {
	ProjectDir string
	Branch     string
}

// UnpushedCommits returns the count of local commits not present on the remote.
// Returns 0 if there is no remote tracking branch.
func UnpushedCommits(params UnpushedCommitsParams) (int, error) {
	cmd := exec.Command("git", "rev-list", "--count", "origin/"+params.Branch+".."+params.Branch)
	cmd.Dir = params.ProjectDir
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

// HasOpenPRParams holds inputs for checking open PRs.
type HasOpenPRParams struct {
	ProjectDir string
	Branch     string
}

// HasOpenPR checks if a branch has an open PR via the gh CLI.
// Returns false gracefully if gh is not installed or fails.
func HasOpenPR(params HasOpenPRParams) (bool, string) {
	ghPath, err := exec.LookPath("gh")
	if err != nil {
		return false, ""
	}

	cmd := exec.Command(ghPath, "pr", "view", params.Branch, "--json", "url,state")
	cmd.Dir = params.ProjectDir
	out, err := cmd.Output()
	if err != nil {
		return false, ""
	}

	var result struct {
		URL   string `json:"url"`
		State string `json:"state"`
	}
	if json.Unmarshal(out, &result) != nil {
		return false, ""
	}

	if result.State == "OPEN" {
		return true, result.URL
	}
	return false, ""
}

