package domain

import "time"

// Worktree represents a git worktree managed by wtm.
type Worktree struct {
	Name   string
	Path   string
	Branch string
}

// WorktreeMetadata is written to .wtm/meta.json inside each created worktree.
type WorktreeMetadata struct {
	SourceBranch string      `json:"source_branch"`
	CreatedAt    string      `json:"created_at"`
	EnvStrategy  EnvStrategy `json:"env_strategy"`
}

// WorktreeStatus holds the display state of a worktree for wtm ls.
type WorktreeStatus struct {
	Branch       string
	Path         string
	IsParent     bool
	IsDirty      bool
	CommitsAhead int
	CreatedAt    time.Time
}

// WorktreeListEntry is the JSON-serializable projection of a worktree for the
// `wt list --output json` payload.
type WorktreeListEntry struct {
	Branch       string            `json:"branch"`
	Path         string            `json:"path"`
	IsParent     bool              `json:"is_parent"`
	IsDirty      bool              `json:"is_dirty"`
	CommitsAhead int               `json:"commits_ahead"`
	CreatedAt    time.Time         `json:"created_at"`
	PR           *WorktreeListPR   `json:"pr"`
	Services     []string          `json:"services"`
}

// WorktreeListPR is the nested PR summary embedded in WorktreeListEntry.
type WorktreeListPR struct {
	Number int    `json:"number"`
	URL    string `json:"url"`
	State  string `json:"state"`
}
