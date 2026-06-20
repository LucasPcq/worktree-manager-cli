package domain

// WorktreeNode is a managed worktree paired with the branch it was created from.
// It is the input to building a sync plan: the SourceBranch links a node to its
// parent in the dependency tree.
type WorktreeNode struct {
	Branch       string
	Path         string
	SourceBranch string
	IsMain       bool
}

// SyncStep is a single rebase to perform: rebase Branch onto SourceBranch.
type SyncStep struct {
	Branch       string
	Path         string
	SourceBranch string
}

// SyncPlan is the ordered list of rebases for a cascade. Steps are sorted
// topologically so a parent is always rebased before its children.
type SyncPlan struct {
	BaseBranch string
	Steps      []SyncStep
}

// SyncStepStatus is the outcome of a single step in a sync run.
type SyncStepStatus string

const (
	// SyncStatusSynced means the branch was rebased onto its (refreshed) parent.
	SyncStatusSynced SyncStepStatus = "synced"
	// SyncStatusUpToDate means the parent had no new commits; nothing to replay.
	SyncStatusUpToDate SyncStepStatus = "up_to_date"
	// SyncStatusSkippedDirty means the worktree had uncommitted changes and was skipped.
	SyncStatusSkippedDirty SyncStepStatus = "skipped_dirty"
	// SyncStatusSkippedAncestor means an ancestor failed or was skipped, so this
	// branch was skipped to avoid rebasing onto a stale parent.
	SyncStatusSkippedAncestor SyncStepStatus = "skipped_ancestor"
	// SyncStatusDiverged means the local branch and origin/<branch> have both
	// moved (no fast-forward possible), so the branch was left untouched for the
	// user to reconcile manually. Its descendants are skipped.
	SyncStatusDiverged SyncStepStatus = "diverged"
	// SyncStatusConflict means the rebase hit a conflict and was aborted (clean state).
	SyncStatusConflict SyncStepStatus = "conflict"
	// SyncStatusError means the rebase failed for a non-conflict reason.
	SyncStatusError SyncStepStatus = "error"
	// SyncStatusUnknownParent means no parent could be determined (missing metadata).
	SyncStatusUnknownParent SyncStepStatus = "unknown_parent"
)

// SyncStepResult carries the full detail of one step so the recap can reassure
// the user (parent, target commit, before→after, replayed count) before pushing.
type SyncStepResult struct {
	Branch          string         `json:"branch"`
	SourceBranch    string         `json:"source_branch"`
	Status          SyncStepStatus `json:"status"`
	OldTip          string         `json:"old_tip"`
	NewTip          string         `json:"new_tip"`
	OntoTip         string         `json:"onto_tip"`
	CommitsReplayed int            `json:"commits_replayed"`
	// PushPending is true when the branch has local commits that origin/<branch>
	// lacks (ahead of, or rewritten relative to, its remote) and is eligible for
	// a force-with-lease push. Set for synced and up_to_date branches.
	PushPending bool   `json:"push_pending"`
	Pushed      bool   `json:"pushed"`
	Detail      string `json:"detail,omitempty"`
}

// SyncResult is the outcome of a full cascade sync.
type SyncResult struct {
	BaseBranch  string           `json:"base_branch"`
	BaseUpdated bool             `json:"base_updated"`
	BaseOldTip  string           `json:"base_old_tip"`
	BaseNewTip  string           `json:"base_new_tip"`
	Steps       []SyncStepResult `json:"steps"`
}
