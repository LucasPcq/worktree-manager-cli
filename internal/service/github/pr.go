package github

import (
	"context"
	"fmt"

	"github.com/google/go-github/v72/github"

	"github.com/LucasPcq/wtm/internal/domain"
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

// ListPRsParams holds inputs for listing pull requests.
type ListPRsParams struct {
	ProjectDir string
	Filter     domain.PRFilter
	Username   string
}

// ListPRs fetches open PRs from the GitHub API and filters them.
func ListPRs(params ListPRsParams) ([]domain.PRInfo, error) {
	client, err := NewClientFromAuth()
	if err != nil {
		return nil, err
	}

	owner, repo, err := infra.RepoOwnerAndName(infra.RepoOwnerAndNameParams{
		ProjectDir: params.ProjectDir,
	})
	if err != nil {
		return nil, fmt.Errorf("resolve repo: %w", err)
	}

	ghPRs, _, err := client.PullRequests.List(context.Background(), owner, repo, &github.PullRequestListOptions{
		State:     "open",
		Sort:      "created",
		Direction: "desc",
		ListOptions: github.ListOptions{
			PerPage: 50,
		},
	})
	if err != nil {
		return nil, fmt.Errorf("list PRs: %w", err)
	}

	var prs []domain.PRInfo
	for _, gh := range ghPRs {
		pr := convertPR(gh)

		switch params.Filter {
		case domain.PRFilterMine:
			if pr.Author != params.Username {
				continue
			}
		case domain.PRFilterReviewRequested:
			if !isReviewRequested(gh, params.Username) {
				continue
			}
		}

		prs = append(prs, pr)
	}

	return prs, nil
}

// GetPRDetailParams holds inputs for fetching a single PR's detail.
type GetPRDetailParams struct {
	ProjectDir string
	Number     int
}

// GetPRDetail fetches a PR with its CI checks and reviews.
func GetPRDetail(params GetPRDetailParams) (domain.PRInfo, error) {
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

	ctx := context.Background()

	ghPR, _, err := client.PullRequests.Get(ctx, owner, repo, params.Number)
	if err != nil {
		return domain.PRInfo{}, fmt.Errorf("get PR: %w", err)
	}

	pr := convertPR(ghPR)

	// Fetch CI status
	pr.CIStatus = fetchCIStatus(ctx, client, owner, repo, ghPR.GetHead().GetSHA())

	// Fetch reviews
	pr.Reviews = fetchReviews(ctx, client, owner, repo, params.Number)

	return pr, nil
}

func convertPR(gh *github.PullRequest) domain.PRInfo {
	state := "open"
	if gh.GetDraft() {
		state = "draft"
	}

	return domain.PRInfo{
		Number:    gh.GetNumber(),
		Title:     gh.GetTitle(),
		Body:      gh.GetBody(),
		Author:    gh.GetUser().GetLogin(),
		Branch:    gh.GetHead().GetRef(),
		State:     state,
		Draft:     gh.GetDraft(),
		CreatedAt: gh.GetCreatedAt().Time,
		URL:       gh.GetHTMLURL(),
	}
}

func isReviewRequested(gh *github.PullRequest, username string) bool {
	for _, reviewer := range gh.RequestedReviewers {
		if reviewer.GetLogin() == username {
			return true
		}
	}
	return false
}

func fetchCIStatus(ctx context.Context, client *github.Client, owner, repo, sha string) domain.CIStatus {
	if sha == "" {
		return domain.CIStatusNone
	}

	result, _, err := client.Checks.ListCheckRunsForRef(ctx, owner, repo, sha, &github.ListCheckRunsOptions{
		ListOptions: github.ListOptions{PerPage: 100},
	})
	if err != nil || result.GetTotal() == 0 {
		return domain.CIStatusNone
	}

	hasPending := false
	for _, check := range result.CheckRuns {
		switch check.GetConclusion() {
		case "failure", "timed_out", "cancelled":
			return domain.CIStatusFailing
		case "":
			hasPending = true
		}
	}

	if hasPending {
		return domain.CIStatusPending
	}

	return domain.CIStatusPassing
}

func fetchReviews(ctx context.Context, client *github.Client, owner, repo string, number int) []domain.ReviewInfo {
	ghReviews, _, err := client.PullRequests.ListReviews(ctx, owner, repo, number, &github.ListOptions{
		PerPage: 50,
	})
	if err != nil {
		return nil
	}

	// Keep only the latest review per user
	latest := make(map[string]domain.ReviewInfo)
	for _, r := range ghReviews {
		user := r.GetUser().GetLogin()
		latest[user] = domain.ReviewInfo{
			User:  user,
			State: r.GetState(),
		}
	}

	var reviews []domain.ReviewInfo
	for _, r := range latest {
		reviews = append(reviews, r)
	}

	return reviews
}
