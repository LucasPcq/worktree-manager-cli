package worktree

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/infra"
)

// ListParams holds inputs for listing worktrees with status.
type ListParams struct {
	ProjectDir string
	Config     domain.Config
}

// List returns all worktrees enriched with git status, sorted with parent first
// then children by creation date (oldest first).
func List(params ListParams) ([]domain.WorktreeStatus, error) {
	gitWorktrees, err := infra.ListWorktrees(infra.ListWorktreesParams{
		ProjectDir: params.ProjectDir,
	})
	if err != nil {
		return nil, err
	}

	baseBranch := params.Config.Project.Worktrees.BaseBranch
	statuses := make([]domain.WorktreeStatus, 0, len(gitWorktrees))

	for _, gw := range gitWorktrees {
		status := buildStatus(gw, baseBranch)
		statuses = append(statuses, status)
	}

	sortStatuses(statuses)

	return statuses, nil
}

func buildStatus(gw infra.GitWorktree, baseBranch string) domain.WorktreeStatus {
	dirty, _ := infra.IsDirty(infra.IsDirtyParams{WorktreePath: gw.Path})

	ahead := 0
	if !gw.IsMain {
		ahead, _ = infra.CommitsAhead(infra.CommitsAheadParams{
			WorktreePath: gw.Path,
			BaseBranch:   baseBranch,
			Branch:       gw.Branch,
		})
	}

	return domain.WorktreeStatus{
		Branch:       gw.Branch,
		Path:         gw.Path,
		IsParent:     gw.IsMain,
		IsDirty:      dirty,
		CommitsAhead: ahead,
		CreatedAt:    worktreeCreatedAt(gw.Path),
	}
}

func worktreeCreatedAt(wtPath string) time.Time {
	metaPath := filepath.Join(wtPath, domain.MetaDirName, domain.MetaFileName)
	data, err := os.ReadFile(metaPath)
	if err == nil {
		var meta domain.WorktreeMetadata
		if json.Unmarshal(data, &meta) == nil && meta.CreatedAt != "" {
			t, parseErr := time.Parse(time.RFC3339, meta.CreatedAt)
			if parseErr == nil {
				return t
			}
		}
	}

	info, err := os.Stat(wtPath)
	if err != nil {
		return time.Time{}
	}
	return info.ModTime()
}

func sortStatuses(statuses []domain.WorktreeStatus) {
	sort.SliceStable(statuses, func(i, j int) bool {
		if statuses[i].IsParent != statuses[j].IsParent {
			return statuses[i].IsParent
		}
		return statuses[i].CreatedAt.Before(statuses[j].CreatedAt)
	})
}
