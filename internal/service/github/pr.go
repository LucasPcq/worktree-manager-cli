package github

import (
	"context"

	"github.com/google/go-github/v72/github"

	"github.com/LucasPcq/wtm/internal/infra"
)

// HasOpenPRParams holds inputs for checking open PRs.
type HasOpenPRParams struct {
	ProjectDir string
	Branch     string
}

// HasOpenPR checks if a branch has an open PR via the GitHub API.
// Returns false gracefully if auth is not configured or the API call fails.
func HasOpenPR(params HasOpenPRParams) (bool, string) {
	client, err := NewClientFromAuth()
	if err != nil {
		return false, ""
	}

	owner, repo, err := infra.RepoOwnerAndName(infra.RepoOwnerAndNameParams{
		ProjectDir: params.ProjectDir,
	})
	if err != nil {
		return false, ""
	}

	prs, _, err := client.PullRequests.List(context.Background(), owner, repo, &github.PullRequestListOptions{
		Head:  owner + ":" + params.Branch,
		State: "open",
	})
	if err != nil || len(prs) == 0 {
		return false, ""
	}

	return true, prs[0].GetHTMLURL()
}
