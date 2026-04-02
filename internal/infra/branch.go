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
