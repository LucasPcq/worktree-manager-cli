package infra

import (
	"fmt"
	"os/exec"
	"strconv"
	"strings"
)

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
