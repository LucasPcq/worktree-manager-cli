package domain

import "time"

// AuthToken represents the stored GitHub OAuth token.
type AuthToken struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
	Scope       string `json:"scope"`
	User        string `json:"user"`
}

// PRInfo holds information about a GitHub pull request.
type PRInfo struct {
	Number    int
	Title     string
	Body      string
	Author    string
	Branch    string
	State     string
	Draft     bool
	CreatedAt time.Time
	URL       string
	CIStatus  CIStatus
	Reviews   []ReviewInfo
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
	User  string
	State string // APPROVED, CHANGES_REQUESTED, PENDING, COMMENTED
}

// PRFilter determines which PRs to list.
type PRFilter string

const (
	PRFilterAll             PRFilter = "all"
	PRFilterReviewRequested PRFilter = "review_requested"
	PRFilterMine            PRFilter = "mine"
)
