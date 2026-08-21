package worktree

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/infra"
	"github.com/LucasPcq/wtm/internal/rules"
)

// WorktreeRef names one worktree: the repository it belongs to, where its state
// is kept, and which branch it holds. It is what every worktree-scoped lookup
// needs and all any of them needs.
type WorktreeRef struct {
	ProjectDir string
	StateDir   string
	Branch     string
}

// EnsureOrdinal returns the worktree's stable number, allocating and persisting
// one the first time it is asked for. The main checkout is ordinal 0 and is
// never written: it has no meta.json, so 0 in a linked worktree's metadata can
// only mean "not allocated yet".
//
// Allocation reads the ordinals of the worktrees git still lists, not every
// meta.json in the state dir — a removed worktree must give its number back.
func EnsureOrdinal(params WorktreeRef) (int, error) {
	worktrees, err := infra.ListWorktrees(infra.ListWorktreesParams{ProjectDir: params.ProjectDir})
	if err != nil {
		return 0, fmt.Errorf("list worktrees: %w", err)
	}

	if isMainBranch(worktrees, params.Branch) {
		return domain.MainWorktreeOrdinal, nil
	}

	if ordinal := settledOrdinal(worktrees, params); ordinal > domain.MainWorktreeOrdinal {
		return ordinal, nil
	}

	allocated := 0
	lockErr := infra.WithFileLock(infra.WithFileLockParams{
		Path: filepath.Join(params.StateDir, domain.OrdinalLockFileName),
		Do: func() error {
			// Re-read under the lock: another process may have allocated between
			// the check above and here, and its number is now taken.
			if ordinal := settledOrdinal(worktrees, params); ordinal > domain.MainWorktreeOrdinal {
				allocated = ordinal
				return nil
			}
			ordinal, err := allocateAndPersist(worktrees, params)
			allocated = ordinal
			return err
		},
	})
	if lockErr != nil {
		return 0, lockErr
	}
	return allocated, nil
}

// settledOrdinal returns the recorded ordinal when it is genuinely this
// worktree's, and zero when it must be re-allocated: never assigned, or shared
// with a live worktree. The second case is the self-repair — a duplicate that
// slipped in (a meta.json copied by hand, a lock that could not be taken) is
// corrected on the next run instead of silently colliding forever.
func settledOrdinal(worktrees []domain.GitWorktree, params WorktreeRef) int {
	meta, err := loadMetadata(params.StateDir, params.Branch)
	if err != nil || meta.Ordinal <= domain.MainWorktreeOrdinal {
		return 0
	}
	for _, taken := range otherOrdinals(worktrees, params) {
		if taken == meta.Ordinal {
			return 0
		}
	}
	return meta.Ordinal
}

func allocateAndPersist(worktrees []domain.GitWorktree, params WorktreeRef) (int, error) {
	ordinal := rules.AllocateOrdinal(otherOrdinals(worktrees, params))

	meta, err := loadMetadata(params.StateDir, params.Branch)
	if err != nil {
		meta = domain.WorktreeMetadata{}
	}
	meta.Ordinal = ordinal

	if err := writeMetadata(rules.WorktreeMetaDir(params.StateDir, params.Branch), meta); err != nil {
		return 0, err
	}
	return ordinal, nil
}

// otherOrdinals collects what every live worktree but this one holds. The main
// checkout is included as ordinal 0 so allocation never hands it out.
func otherOrdinals(worktrees []domain.GitWorktree, params WorktreeRef) []int {
	taken := make([]int, 0, len(worktrees))
	for _, wt := range worktrees {
		if wt.Branch == params.Branch {
			continue
		}
		if wt.IsMain {
			taken = append(taken, domain.MainWorktreeOrdinal)
			continue
		}
		meta, err := loadMetadata(params.StateDir, wt.Branch)
		if err != nil {
			continue
		}
		taken = append(taken, meta.Ordinal)
	}
	return taken
}

func isMainBranch(worktrees []domain.GitWorktree, branch string) bool {
	for _, wt := range worktrees {
		if wt.Branch == branch {
			return wt.IsMain
		}
	}
	return false
}

// purgeState drops a removed worktree's metadata directory, which nothing else
// ever deletes — leaving it behind would keep its ordinal reserved for good.
// Best effort: leftover state is not worth failing a removal that happened.
func purgeState(stateDir, branch string) {
	dir := rules.PurgeableMetaDir(rules.PurgeableMetaDirParams{StateDir: stateDir, Branch: branch})
	if dir == "" {
		return
	}
	_ = os.RemoveAll(dir)
}
