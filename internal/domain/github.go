package domain

// PRInfo holds information about a GitHub pull request.
type PRInfo struct {
	Number         int      `json:"number"`
	Title          string   `json:"title"`
	Author         string   `json:"author"`
	Branch         string   `json:"branch"`
	BaseBranch     string   `json:"base_branch"` // branch the PR will be merged into
	State          string   `json:"state"`
	URL            string   `json:"url"`
	IsFork         bool     `json:"is_fork"`  // true when head.repo != base.repo
	Draft          bool     `json:"is_draft"` // true when the PR is a draft
	Checks         PRChecks `json:"checks"`
	ReviewDecision string   `json:"review_decision"`
}

// PRChecks aggregates a PR's status-check rollup. Passed, Failed and Pending
// summing to zero means no checks are configured at all — distinct from
// configured checks that are all still pending, which leaves Pending > 0.
type PRChecks struct {
	Passed  int `json:"passed"`
	Failed  int `json:"failed"`
	Pending int `json:"pending"`
}

// PRFilter determines which PRs to list.
type PRFilter string

const (
	PRFilterAll             PRFilter = "all"
	PRFilterReviewRequested PRFilter = "review_requested"
	PRFilterMine            PRFilter = "mine"
)

// GHConnection describes the availability of the GitHub CLI used to fetch PRs.
type GHConnection int

const (
	GHConnectionOK GHConnection = iota
	GHConnectionNotInstalled
	GHConnectionNotAuthenticated
)
