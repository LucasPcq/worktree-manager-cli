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
	TargetPath   string
	TargetBranch string
	Files        []ExtractFile
	Keep         bool
}

// ExtractResult is the outcome of a successful extraction.
type ExtractResult struct {
	Files        []ExtractFile `json:"files"`
	TargetPath   string        `json:"target_path"`
	TargetBranch string        `json:"target_branch"`
	Kept         bool          `json:"kept"`
}
