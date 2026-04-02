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
