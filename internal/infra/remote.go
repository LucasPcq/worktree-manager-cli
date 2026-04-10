package infra

import (
	"fmt"
	"os/exec"
	"strings"
)

// RepoOwnerAndNameParams holds inputs for extracting the GitHub owner and repo name.
type RepoOwnerAndNameParams struct {
	ProjectDir string
}

// RepoOwnerAndName parses the origin remote URL to extract the GitHub owner and repository name.
// Supports both SSH (git@github.com:Owner/Repo.git) and HTTPS (https://github.com/Owner/Repo.git).
func RepoOwnerAndName(params RepoOwnerAndNameParams) (string, string, error) {
	cmd := exec.Command("git", "remote", "get-url", "origin")
	cmd.Dir = params.ProjectDir
	out, err := cmd.Output()
	if err != nil {
		return "", "", fmt.Errorf("get remote URL: %w", err)
	}

	rawURL := strings.TrimSpace(string(out))

	owner, repo, ok := parseRemoteURL(rawURL)
	if !ok {
		return "", "", fmt.Errorf("cannot parse remote URL: %s", rawURL)
	}

	return owner, repo, nil
}

// parseRemoteURL extracts owner and repo from a git remote URL.
func parseRemoteURL(rawURL string) (string, string, bool) {
	// SSH: git@github.com:Owner/Repo.git
	if strings.HasPrefix(rawURL, "git@") {
		parts := strings.SplitN(rawURL, ":", 2)
		if len(parts) != 2 {
			return "", "", false
		}
		return splitOwnerRepo(parts[1])
	}

	// HTTPS: https://github.com/Owner/Repo.git
	trimmed := strings.TrimPrefix(rawURL, "https://")
	trimmed = strings.TrimPrefix(trimmed, "http://")
	segments := strings.Split(trimmed, "/")
	if len(segments) < 3 {
		return "", "", false
	}
	return splitOwnerRepo(segments[1] + "/" + segments[2])
}

// splitOwnerRepo splits "Owner/Repo.git" into owner and repo.
func splitOwnerRepo(path string) (string, string, bool) {
	path = strings.TrimSuffix(path, ".git")
	parts := strings.SplitN(path, "/", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", "", false
	}
	return parts[0], parts[1], true
}
