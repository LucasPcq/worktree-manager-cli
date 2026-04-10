package github

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"unicode"

	"github.com/google/go-github/v72/github"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/infra"
)

// CreatePRParams holds inputs for creating a pull request.
type CreatePRParams struct {
	ProjectDir string
	Title      string
	Body       string
	Head       string
	Base       string
	Draft      bool
}

// CreatePR creates a pull request on GitHub and returns the created PR info.
func CreatePR(params CreatePRParams) (domain.PRInfo, error) {
	client, err := NewClientFromAuth()
	if err != nil {
		return domain.PRInfo{}, err
	}

	owner, repo, err := infra.RepoOwnerAndName(infra.RepoOwnerAndNameParams{
		ProjectDir: params.ProjectDir,
	})
	if err != nil {
		return domain.PRInfo{}, fmt.Errorf("resolve repo: %w", err)
	}

	ghPR, _, err := client.PullRequests.Create(context.Background(), owner, repo, &github.NewPullRequest{
		Title: github.Ptr(params.Title),
		Body:  github.Ptr(params.Body),
		Head:  github.Ptr(params.Head),
		Base:  github.Ptr(params.Base),
		Draft: github.Ptr(params.Draft),
	})
	if err != nil {
		return domain.PRInfo{}, fmt.Errorf("create PR: %w", err)
	}

	return convertPR(ghPR), nil
}

// BranchTitleFromName generates a human-readable PR title from a branch name.
func BranchTitleFromName(branch string) string {
	// Strip standard prefixes
	prefixes := []string{"feature/", "fix/", "chore/", "hotfix/", "bugfix/", "refactor/", "docs/"}
	for _, prefix := range prefixes {
		if strings.HasPrefix(branch, prefix) {
			branch = strings.TrimPrefix(branch, prefix)
			break
		}
	}

	// Strip issue IDs like "LUC-13-" or "ENG-42-"
	issueIDPattern := regexp.MustCompile(`^[A-Z]+-\d+-`)
	branch = issueIDPattern.ReplaceAllString(branch, "")

	// Replace separators with spaces
	branch = strings.ReplaceAll(branch, "-", " ")
	branch = strings.ReplaceAll(branch, "_", " ")

	// Capitalize first letter
	branch = strings.TrimSpace(branch)
	if len(branch) == 0 {
		return branch
	}

	runes := []rune(branch)
	runes[0] = unicode.ToUpper(runes[0])
	return string(runes)
}

// DetectPRTemplate looks for a PR template in the repository.
// Returns the template content or empty string if none found.
func DetectPRTemplate(projectDir string) string {
	candidates := []string{
		filepath.Join(projectDir, ".github", "pull_request_template.md"),
		filepath.Join(projectDir, ".github", "PULL_REQUEST_TEMPLATE.md"),
	}

	for _, path := range candidates {
		content, err := os.ReadFile(path)
		if err == nil {
			return string(content)
		}
	}

	// Check PULL_REQUEST_TEMPLATE directory
	templateDir := filepath.Join(projectDir, ".github", "PULL_REQUEST_TEMPLATE")
	entries, err := os.ReadDir(templateDir)
	if err == nil {
		for _, entry := range entries {
			if entry.IsDir() {
				continue
			}
			content, err := os.ReadFile(filepath.Join(templateDir, entry.Name()))
			if err == nil {
				return string(content)
			}
		}
	}

	return ""
}
