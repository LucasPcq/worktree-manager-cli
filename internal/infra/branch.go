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
	out, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("git branch: %s: %w", strings.TrimSpace(string(out)), err)
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

func branchExists(projectDir string, branch string) bool {
	cmd := exec.Command("git", "rev-parse", "--verify", "refs/heads/"+branch)
	cmd.Dir = projectDir
	return cmd.Run() == nil
}

// CurrentBranch returns the name of the currently checked-out branch.
func CurrentBranch(projectDir string) (string, error) {
	cmd := exec.Command("git", "branch", "--show-current")
	cmd.Dir = projectDir
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("git branch --show-current: %w", err)
	}
	branch := strings.TrimSpace(string(out))
	if branch == "" {
		return "", fmt.Errorf("detached HEAD, no branch checked out")
	}
	return branch, nil
}

// BranchExistsOnRemoteParams holds inputs for checking remote branch existence.
type BranchExistsOnRemoteParams struct {
	ProjectDir string
	Branch     string
}

// BranchExistsOnRemote returns true if the branch exists on origin.
func BranchExistsOnRemote(params BranchExistsOnRemoteParams) bool {
	cmd := exec.Command("git", "rev-parse", "--verify", "origin/"+params.Branch)
	cmd.Dir = params.ProjectDir
	return cmd.Run() == nil
}

// FetchBranchParams holds inputs for fetching a specific branch from origin.
type FetchBranchParams struct {
	ProjectDir string
	Branch     string
}

// FetchBranch runs `git fetch origin <branch>` to update the remote-tracking ref.
func FetchBranch(params FetchBranchParams) error {
	cmd := exec.Command("git", "fetch", "origin", params.Branch)
	cmd.Dir = params.ProjectDir
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git fetch origin %s: %s", params.Branch, strings.TrimSpace(string(out)))
	}
	return nil
}

// PushBranchParams holds inputs for pushing a branch.
type PushBranchParams struct {
	ProjectDir string
	Branch     string
}

// PushBranch pushes the branch to origin with upstream tracking.
func PushBranch(params PushBranchParams) error {
	cmd := exec.Command("git", "push", "-u", "origin", params.Branch)
	cmd.Dir = params.ProjectDir
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git push: %s", strings.TrimSpace(string(out)))
	}
	return nil
}
