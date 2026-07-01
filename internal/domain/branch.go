package domain

// DivergenceState classifies how a local branch relates to its origin
// remote-tracking counterpart, derived from the ahead/behind commit counts.
type DivergenceState int

const (
	// DivergenceUnknown means the branch has no origin counterpart (local-only)
	// or the comparison was not computed; no divergence badge is shown.
	DivergenceUnknown DivergenceState = iota
	// DivergenceUpToDate means local and origin point at the same commit.
	DivergenceUpToDate
	// DivergenceBehind means origin has commits the local branch lacks
	// (fast-forward possible).
	DivergenceBehind
	// DivergenceAhead means the local branch has commits not yet on origin.
	DivergenceAhead
	// DivergenceDiverged means each side has commits the other lacks.
	DivergenceDiverged
)

// AheadBehind holds the commit counts of a local branch relative to its origin
// counterpart: Ahead commits exist only locally, Behind commits exist only on
// origin.
type AheadBehind struct {
	Ahead  int
	Behind int
}

// BranchCandidate is a branch offered in a picker as a worktree start-point or
// parent. Name is the git ref used as the value: a bare local name ("feature")
// or a remote-tracking ref ("origin/feature"). IsRemote tags the latter so the
// UI can badge it and group remotes separately. Ahead/Behind/State describe how
// a local branch diverges from its origin counterpart (zero/Unknown when there
// is none) so the UI can warn before a worktree starts from a stale ref.
type BranchCandidate struct {
	Name     string
	IsRemote bool
	Ahead    int
	Behind   int
	State    DivergenceState
}
