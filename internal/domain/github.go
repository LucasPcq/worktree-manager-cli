package domain

import "time"

// PRInfo holds information about a GitHub pull request.
type PRInfo struct {
	Number     int          `json:"number"`
	Title      string       `json:"title"`
	Body       string       `json:"body"`
	Author     string       `json:"author"`
	Branch     string       `json:"branch"`
	BaseBranch string       `json:"base_branch"` // branch the PR will be merged into
	State      string       `json:"state"`
	Draft      bool         `json:"draft"`
	CreatedAt  time.Time    `json:"created_at"`
	URL        string       `json:"url"`
	CIStatus   CIStatus     `json:"ci_status"`
	Reviews    []ReviewInfo `json:"reviews"`
	IsFork     bool         `json:"is_fork"` // true when head.repo != base.repo
}

// CIStatus represents the aggregate CI check status.
type CIStatus string

const (
	CIStatusPassing CIStatus = "passing"
	CIStatusFailing CIStatus = "failing"
	CIStatusPending CIStatus = "pending"
	CIStatusNone    CIStatus = "none"
)

// ReviewInfo holds a single reviewer's status.
type ReviewInfo struct {
	User  string `json:"user"`
	State string `json:"state"` // APPROVED, CHANGES_REQUESTED, PENDING, COMMENTED
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
