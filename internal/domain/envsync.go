package domain

// EnvFileResult is the reconciliation outcome for one configured env target of a
// worktree. Diff carries the per-key verdict; Source is the display label of the
// value cascade the strategy selected; Applied reports whether the reconciled
// content was written back; ParentFallback flags a "parent" strategy that sourced
// from main because the parent had no local worktree.
type EnvFileResult struct {
	Target   string      `json:"target"`
	Strategy EnvStrategy `json:"strategy"`
	Source   string      `json:"source"`
	Diff     EnvDiff     `json:"diff"`
	Applied  bool        `json:"applied"`
	// ParentBranch is the recorded parent branch consulted by the "parent" strategy
	// (empty for other strategies). ParentFallback is true when that parent had no
	// local worktree, so values came from main instead.
	ParentBranch   string `json:"parent_branch,omitempty"`
	ParentFallback bool   `json:"parent_fallback,omitempty"`
}

// EnvSyncResult is the full outcome of `wtm env` on one worktree across all its
// configured env files. Check mirrors --check (read-only diagnostic, nothing
// written).
type EnvSyncResult struct {
	Branch string          `json:"branch"`
	Mode   EnvMode         `json:"mode"`
	Check  bool            `json:"check"`
	Files  []EnvFileResult `json:"files"`
}

// HasDrift reports whether any file has a key needing attention (missing,
// conflicting, or orphaned) — i.e. the worktree is not fully reconciled.
func (r EnvSyncResult) HasDrift() bool {
	for _, f := range r.Files {
		if f.Diff.HasStatus(EnvKeyMissing) || f.Diff.HasStatus(EnvKeyConflict) || f.Diff.HasStatus(EnvKeyOrphan) {
			return true
		}
	}
	return false
}
