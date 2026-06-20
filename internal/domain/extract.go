package domain

// ExtractFileStatus classifies an uncommitted file selected for extraction.
type ExtractFileStatus string

const (
	// ExtractStatusModified marks a tracked file with content changes vs HEAD.
	ExtractStatusModified ExtractFileStatus = "modified"

	// ExtractStatusUntracked marks a file unknown to git (porcelain status "??").
	ExtractStatusUntracked ExtractFileStatus = "untracked"

	// ExtractStatusDeleted marks a tracked file removed from the working tree.
	ExtractStatusDeleted ExtractFileStatus = "deleted"
)

// ExtractFile is one uncommitted file eligible for extraction to another worktree.
type ExtractFile struct {
	Path   string            `json:"path"`
	Status ExtractFileStatus `json:"status"`
}

// ExtractParams holds inputs for moving uncommitted files from the source
// worktree to the target worktree.
type ExtractParams struct {
	SourcePath   string
	SourceBranch string
	TargetPath   string
	TargetBranch string
	Files        []ExtractFile
	Keep         bool
	// ConflictMode selects what happens when the changes do not apply cleanly:
	// OnConflictAbort (default) leaves everything untouched; OnConflictResolve
	// writes conflict markers into the target and keeps the source intact.
	ConflictMode string
}

// ConflictCheckParams holds inputs for detecting which selected files would not
// apply cleanly onto the target worktree.
type ConflictCheckParams struct {
	SourcePath string
	TargetPath string
	Files      []ExtractFile
}

// ExtractResult is the outcome of a successful extraction.
type ExtractResult struct {
	Files        []ExtractFile `json:"files"`
	TargetPath   string        `json:"target_path"`
	TargetBranch string        `json:"target_branch"`
	SourceBranch string        `json:"source_branch"`
	Kept         bool          `json:"kept"`
	// Conflicts lists the files written with conflict markers in resolve mode.
	// Empty on a clean extraction.
	Conflicts []string `json:"conflicts"`
}
