package domain

import "time"

// CommitSummary is a history line as the detail panel displays it.
type CommitSummary struct {
	SHA     string
	Subject string
	Author  string
	At      time.Time
}

// DiffStat is the volume of a diff, without the per-file breakdown. It comes
// straight off `git diff --shortstat`, so FilesChanged is one aggregate
// count, not the per-status split WorkingChanges.Files carries.
type DiffStat struct {
	FilesChanged int
	Insertions   int
	Deletions    int
}

// DetailFamily names a group of data in the detail view that can fail independently.
type DetailFamily string

const (
	DetailFamilyCommits    DetailFamily = "commits"
	DetailFamilyChanges    DetailFamily = "changes"
	DetailFamilyEnv        DetailFamily = "env"
	DetailFamilyBlockers   DetailFamily = "blockers"
	DetailFamilyBranchDiff DetailFamily = "branch_diff"
)

// WorkingChanges is the state of the working tree, counted by porcelain column.
// A file staged and then modified again ("MM") counts in both Staged and Modified.
type WorkingChanges struct {
	Modified   int
	Untracked  int
	Staged     int
	Insertions int
	Deletions  int
	Files      []PorcelainEntry
}

// EnvDriftSummary summarizes .env drift without per-key detail. Configured
// distinguishes "no drift" from "no env files configured" — the renderer must
// not present the latter as a success.
type EnvDriftSummary struct {
	Configured  bool
	Missing     int
	Conflicting int
	Orphan      int
}

// WorktreeDetail is what the detail panel displays beyond WorktreeStatus.
// It loads lazily on selection: never in the poll.
type WorktreeDetail struct {
	Branch   string
	Commits  []CommitSummary
	Changes  WorkingChanges
	Children []string
	Blockers []CleanBlocker
	EnvDrift EnvDriftSummary
	// BranchDiff is the branch's committed volume against its base
	// (`git diff base...branch --shortstat`), ACTIVITY's counterpart to
	// CHANGES' uncommitted volume. Left zero for the parent worktree, which
	// has no base to diff against.
	BranchDiff DiffStat
	// RunAddresses is where each declared job answers in this worktree, keyed by
	// job name. Nil for a project with no run module, and for a worktree whose
	// environment could not be read — a port computed on a missing offset would
	// be wrong, and a wrong port reads as a truth.
	RunAddresses map[string]JobAddress
	LoadedAt     time.Time

	// Failures names families that could not be read. A family absent from
	// the map was read successfully, even if empty: this is what distinguishes
	// legitimate absence from render-time failure.
	Failures map[DetailFamily]error
}

// Chip is one element of the vital strip. State marks the only coloured chip:
// the working-tree state.
type Chip struct {
	Text  string
	State bool
	Kind  ChipKind
}

type ChipKind string

const (
	ChipKindClean    ChipKind = "clean"
	ChipKindDirty    ChipKind = "dirty"
	ChipKindRebasing ChipKind = "rebasing"
	ChipKindNeutral  ChipKind = "neutral"
)

type DetailCellKind string

const (
	DetailCellGlyph   DetailCellKind = "glyph"
	DetailCellName    DetailCellKind = "name"
	DetailCellAddress DetailCellKind = "address"
	DetailCellMeta    DetailCellKind = "meta"
	// DetailCellNote is a standalone muted body line inside a rowed section —
	// the "… N more" fold, which is not a job and has no columns to align on.
	DetailCellNote DetailCellKind = "note"
)

type DetailCell struct {
	Kind DetailCellKind
	Text string
}

// DetailRow is one line of a rowed section before its columns are sized: only
// the renderer can measure a cell once it is styled. Key names what the row
// designates, URL what clicking it opens.
type DetailRow struct {
	Key   string
	Cells []DetailCell
	Up    bool
	URL   string
}

// DetailSection is one block of the detail panel, already reduced to its plain
// text lines. Rendering decides nothing: it styles and it stacks.
type DetailSection struct {
	Key   string
	Title string
	// TitleRight is a summary of the section, rendered flush right on its
	// heading row (mirrors panelParams.TitleRight in tui/dashboard/render.go)
	// and dropped whole, never truncated, when the panel is too narrow for it.
	TitleRight string
	Lines      []string
	// Rows is read instead of Lines when it is non-nil: a section whose body is
	// a table cannot arrive pre-padded, since rules/ cannot measure a styled
	// cell.
	Rows []DetailRow
}

// DetailSectionDropOrder is the order sections give up their place when the
// panel runs out of height: the last one listed falls first. The vital strip
// and the blockers line are not in it — they never fall.
var DetailSectionDropOrder = []string{
	DetailSectionRun,
	DetailSectionReview,
	DetailSectionChanges,
	DetailSectionActivity,
	DetailSectionLinks,
}
