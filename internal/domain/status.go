package domain

// PorcelainEntry is one record of `git status --porcelain -z` output.
type PorcelainEntry struct {
	// Status is the raw two-character XY field, spaces included (" M", "??", "RM").
	// The column a code sits in carries meaning, so it is never trimmed here.
	Status string

	// Path is the file path. For a rename or a copy it is the new path.
	Path string

	// OrigPath is the path the file came from, set only for renames and copies.
	OrigPath string
}
