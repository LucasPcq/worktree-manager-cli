package github

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/rules"
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

// CreatePR creates a pull request via gh CLI and returns the created PR info.
func CreatePR(params CreatePRParams) (domain.PRInfo, error) {
	if err := ensureAuth(); err != nil {
		return domain.PRInfo{}, err
	}

	args := []string{
		"pr", "create",
		"--title", params.Title,
		"--body", params.Body,
		"--base", params.Base,
		"--head", params.Head,
	}
	if params.Draft {
		args = append(args, "--draft")
	}

	data, err := runGH(params.ProjectDir, args...)
	if err != nil {
		return domain.PRInfo{}, fmt.Errorf("create PR: %w", err)
	}

	url := strings.TrimSpace(string(data))
	number, err := rules.ExtractPRNumber(url)
	if err != nil {
		return domain.PRInfo{}, fmt.Errorf("parse PR URL %q: %w", url, err)
	}

	return domain.PRInfo{
		Number: number,
		Title:  params.Title,
		Body:   params.Body,
		Branch: params.Head,
		Draft:  params.Draft,
		URL:    url,
	}, nil
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
