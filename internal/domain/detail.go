package domain

import "time"

// CommitSummary is a history line as the detail panel displays it.
type CommitSummary struct {
	SHA     string
	Subject string
	Author  string
	At      time.Time
}

// DiffStat is the volume of a diff, without the per-file breakdown.
type DiffStat struct {
	Insertions int
	Deletions  int
}

// DetailFamily names a group of data in the detail view that can fail independently.
type DetailFamily string

const (
	DetailFamilyCommits  DetailFamily = "commits"
	DetailFamilyChanges  DetailFamily = "changes"
	DetailFamilyEnv      DetailFamily = "env"
	DetailFamilyBlockers DetailFamily = "blockers"
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
	LoadedAt time.Time

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

// DetailSection is one block of the detail panel, already reduced to its plain
// text lines. Rendering decides nothing: it styles and it stacks.
type DetailSection struct {
	Key   string
	Title string
	Lines []string
}

// DetailSectionDropOrder is the order sections give up their place when the
// panel runs out of height: the last one listed falls first. The vital strip
// and the blockers line are not in it — they never fall.
var DetailSectionDropOrder = []string{
	DetailSectionReview,
	DetailSectionChanges,
	DetailSectionActivity,
	DetailSectionLinks,
}
