package worktree

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/infra"
	"github.com/LucasPcq/wtm/internal/rules"
)

// statusWorkers bounds concurrent git probes when building worktree status.
const statusWorkers = 8

// List returns all worktrees enriched with git status, sorted with parent first
// then children by creation date (oldest first).
func List(params domain.ListParams) ([]domain.WorktreeStatus, error) {
	gitWorktrees, err := infra.ListWorktrees(infra.ListWorktreesParams{
		ProjectDir: params.ProjectDir,
	})
	if err != nil {
		return nil, err
	}

	baseBranch := params.Config.Project.Worktrees.BaseBranch
	statuses := make([]domain.WorktreeStatus, len(gitWorktrees))

	sem := make(chan struct{}, statusWorkers)
	var wg sync.WaitGroup
	for i, gitWorktree := range gitWorktrees {
		wg.Add(1)
		sem <- struct{}{}
		go func(idx int, wt domain.GitWorktree) {
			defer wg.Done()
			defer func() { <-sem }()
			statuses[idx] = buildStatus(wt, baseBranch, params.StateDir)
		}(i, gitWorktree)
	}
	wg.Wait()

	rules.SortStatuses(statuses)

	return statuses, nil
}

func buildStatus(gitWorktree domain.GitWorktree, baseBranch, stateDir string) domain.WorktreeStatus {
	dirty, _ := infra.IsDirty(infra.IsDirtyParams{WorktreePath: gitWorktree.Path})

	ahead := 0
	if !gitWorktree.IsMain {
		ahead, _ = infra.CommitsAhead(infra.CommitsAheadParams{
			WorktreePath: gitWorktree.Path,
			BaseBranch:   baseBranch,
			Branch:       gitWorktree.Branch,
		})
	}

	return domain.WorktreeStatus{
		Branch:       gitWorktree.Branch,
		Path:         gitWorktree.Path,
		IsParent:     gitWorktree.IsMain,
		IsDirty:      dirty,
		CommitsAhead: ahead,
		CreatedAt:    worktreeCreatedAt(stateDir, gitWorktree.Branch, gitWorktree.Path),
	}
}

// worktreeCreatedAt reads the recorded creation time from the per-worktree
// meta.json. Falls back to the worktree directory mtime when meta is absent
// (e.g. worktrees created outside wtm).
func worktreeCreatedAt(stateDir, branch, fallbackPath string) time.Time {
	metaPath := filepath.Join(rules.WorktreeMetaDir(stateDir, branch), domain.MetaFileName)
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

	info, err := os.Stat(fallbackPath)
	if err != nil {
		return time.Time{}
	}
	return info.ModTime()
}
