package github

import (
	"fmt"
	"strconv"

	"github.com/LucasPcq/wtm/internal/domain"
)

// HasOpenPRParams holds inputs for checking open PRs.
type HasOpenPRParams struct {
	ProjectDir string
	Branch     string
}

// HasOpenPR checks if a branch has an open PR via the gh CLI. Returns the PR
// number and URL when found. Returns false gracefully if gh is not installed,
// not authenticated, or the call fails.
func HasOpenPR(params HasOpenPRParams) (bool, int, string) {
	if err := ensureAuth(); err != nil {
		return false, 0, ""
	}

	type prItem struct {
		Number int    `json:"number"`
		URL    string `json:"url"`
	}

	data, err := runGH(params.ProjectDir, "pr", "list",
		"--head", params.Branch,
		"--state", "open",
		"--json", "number,url",
		"--limit", "1",
	)
	if err != nil {
		return false, 0, ""
	}

	items, err := parseJSON[[]prItem](data)
	if err != nil || len(items) == 0 {
		return false, 0, ""
	}

	return true, items[0].Number, items[0].URL
}

// ListPRsParams holds inputs for listing pull requests.
type ListPRsParams struct {
	ProjectDir string
	Filter     domain.PRFilter
	// Lightweight drops the heavy `body` field from the fetch. Use it for
	// pickers that only render PR rows; leave false when the body is needed.
	Lightweight bool
}

// ListPRs fetches open PRs via gh CLI and filters them.
func ListPRs(params ListPRsParams) ([]domain.PRInfo, error) {
	if err := ensureAuth(); err != nil {
		return nil, err
	}

	fields := domain.GHPRFieldsFull
	if params.Lightweight {
		fields = domain.GHPRFieldsLight
	}

	args := []string{
		"pr", "list",
		"--state", "open",
		"--json", fields,
		"--limit", "50",
	}

	switch params.Filter {
	case domain.PRFilterMine:
		args = append(args, "--author", "@me")
	case domain.PRFilterReviewRequested:
		args = append(args, "--search", "review-requested:@me")
	}

	data, err := runGH(params.ProjectDir, args...)
	if err != nil {
		return nil, fmt.Errorf("list PRs: %w", err)
	}

	ghPRs, err := parseJSON[[]ghPR](data)
	if err != nil {
		return nil, err
	}

	prs := make([]domain.PRInfo, 0, len(ghPRs))
	for _, g := range ghPRs {
		prs = append(prs, convertGHPR(g))
	}
	return prs, nil
}

// GetPRDetailParams holds inputs for fetching a single PR's detail.
type GetPRDetailParams struct {
	ProjectDir string
	Number     int
}

// GetPRDetail fetches a PR with its CI checks and reviews via gh CLI.
func GetPRDetail(params GetPRDetailParams) (domain.PRInfo, error) {
	if err := ensureAuth(); err != nil {
		return domain.PRInfo{}, err
	}

	data, err := runGH(params.ProjectDir, "pr", "view", strconv.Itoa(params.Number),
		"--json", "number,title,author,headRefName,baseRefName,isDraft,createdAt,url,body,isCrossRepository,statusCheckRollup,reviews",
	)
	if err != nil {
		return domain.PRInfo{}, fmt.Errorf("get PR: %w", err)
	}

	g, err := parseJSON[ghPR](data)
	if err != nil {
		return domain.PRInfo{}, err
	}

	return convertGHPR(g), nil
}
