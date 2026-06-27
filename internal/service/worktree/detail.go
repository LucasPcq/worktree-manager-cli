package worktree

import (
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/rules"
)

// ParentBranchParams holds inputs for resolving a worktree's parent branch.
type ParentBranchParams struct {
	StateDir string
	Branch   string
}

// ParentBranch returns the branch the given worktree was created from, read from
// its metadata. Returns an empty string when no metadata is recorded.
func ParentBranch(params ParentBranchParams) string {
	return loadSourceBranch(params.StateDir, params.Branch)
}

func loadSourceBranch(stateDir, branch string) string {
	meta, err := loadMetadata(stateDir, branch)
	if err != nil {
		return ""
	}
	return meta.SourceBranch
}

// loadMetadata reads the full meta.json for a worktree, so callers can update one
// field while preserving the rest (CreatedAt, EnvStrategy).
func loadMetadata(stateDir, branch string) (domain.WorktreeMetadata, error) {
	metaPath := filepath.Join(rules.WorktreeMetaDir(stateDir, branch), domain.MetaFileName)
	data, err := os.ReadFile(metaPath)
	if err != nil {
		return domain.WorktreeMetadata{}, err
	}
	var meta domain.WorktreeMetadata
	if err := json.Unmarshal(data, &meta); err != nil {
		return domain.WorktreeMetadata{}, err
	}
	return meta, nil
}
