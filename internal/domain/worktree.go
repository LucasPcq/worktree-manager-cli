package domain

import "time"

// Worktree represents a git worktree managed by wtm.
type Worktree struct {
	Name   string
	Path   string
	Branch string
}

// WorktreeMetadata is written to <state-dir>/worktrees/<branch>/meta.json
// for every worktree managed by wtm.
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

// GitWorktree represents a worktree entry from git worktree list.
type GitWorktree struct {
	Path   string
	Branch string
	IsMain bool
}

// CreateParams holds all inputs needed to create a new worktree.
type CreateParams struct {
	ProjectDir      string
	StateDir        string
	Branch          string
	FromBranch      string
	Config          Config
	EnvFromOverride string
}

// CreateResult holds the output of a successful worktree creation.
type CreateResult struct {
	Branch   string           `json:"branch"`
	Path     string           `json:"path"`
	Metadata WorktreeMetadata `json:"metadata"`
}

// CleanParams holds inputs for cleaning a worktree.
type CleanParams struct {
	ProjectDir string
	Branch     string
	Force      bool
	Config     Config
}

// CleanCheckResult holds the pre-deletion check results.
type CleanCheckResult struct {
	WorktreePath    string
	Branch          string
	UnpushedCommits int
	HasOpenPR       bool
	PRUrl           string
	IsDirty         bool
	IsParent        bool
}

// ListParams holds inputs for listing worktrees with status.
type ListParams struct {
	ProjectDir string
	StateDir   string
	Config     Config
}

// ResolveParams holds inputs for resolving a branch query to a worktree path.
type ResolveParams struct {
	ProjectDir string
	Query      string
}

// ResolveResult indicates whether the resolution is direct or needs a picker.
type ResolveResult struct {
	Path      string
	Ambiguous bool
	Matches   []GitWorktree
}
