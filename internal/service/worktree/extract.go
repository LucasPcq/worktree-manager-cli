package worktree

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/infra"
)

// Extract moves the selected uncommitted files from the source worktree to the
// target worktree. It is transactional: the changes are applied to the target
// first and the source is cleaned only on success. Any conflict aborts the
// operation with the source left untouched. With Keep set, the source is not
// cleaned (copy instead of move).
func Extract(params domain.ExtractParams) (domain.ExtractResult, error) {
	if params.SourcePath == params.TargetPath {
		return domain.ExtractResult{}, domain.ErrSameWorktree
	}
	if len(params.Files) == 0 {
		return domain.ExtractResult{}, domain.ErrNoFilesSelected
	}

	tracked, untracked := splitByStatus(params.Files)

	if err := ensureNoUntrackedCollision(untrackedCollisionParams{
		TargetPath:   params.TargetPath,
		TargetBranch: params.TargetBranch,
		Untracked:    untracked,
	}); err != nil {
		return domain.ExtractResult{}, err
	}

	patch, err := infra.DiffFiles(infra.DiffFilesParams{
		WorktreePath: params.SourcePath,
		Files:        tracked,
	})
	if err != nil {
		return domain.ExtractResult{}, err
	}

	if conflicts := conflictingFiles(params.SourcePath, params.TargetPath, tracked); len(conflicts) > 0 {
		return domain.ExtractResult{}, conflictError(conflicts, params.TargetBranch)
	}

	if err := infra.ApplyPatch(infra.ApplyPatchParams{
		WorktreePath: params.TargetPath,
		Patch:        patch,
		ThreeWay:     true,
	}); err != nil {
		rollbackTarget(params.TargetPath, patch, tracked)
		return domain.ExtractResult{}, conflictError(tracked, params.TargetBranch)
	}

	if err := copyUntracked(copyUntrackedParams{
		SourcePath: params.SourcePath,
		TargetPath: params.TargetPath,
		Files:      untracked,
	}); err != nil {
		rollbackTarget(params.TargetPath, patch, tracked)
		removeTargetFiles(params.TargetPath, untracked)
		return domain.ExtractResult{}, err
	}

	if !params.Keep {
		if err := cleanSource(cleanSourceParams{
			SourcePath: params.SourcePath,
			Patch:      patch,
			Tracked:    tracked,
			Untracked:  untracked,
		}); err != nil {
			return domain.ExtractResult{}, fmt.Errorf("clean source: %w", err)
		}
	}

	return domain.ExtractResult{
		Files:        params.Files,
		TargetPath:   params.TargetPath,
		TargetBranch: params.TargetBranch,
		Kept:         params.Keep,
	}, nil
}

func splitByStatus(files []domain.ExtractFile) (tracked, untracked []string) {
	for _, f := range files {
		if f.Status == domain.ExtractStatusUntracked {
			untracked = append(untracked, f.Path)
			continue
		}
		tracked = append(tracked, f.Path)
	}
	return tracked, untracked
}

type untrackedCollisionParams struct {
	TargetPath   string
	TargetBranch string
	Untracked    []string
}

func ensureNoUntrackedCollision(params untrackedCollisionParams) error {
	var clashing []string
	for _, f := range params.Untracked {
		if infra.FileExists(filepath.Join(params.TargetPath, f)) {
			clashing = append(clashing, f)
		}
	}
	if len(clashing) == 0 {
		return nil
	}

	verb := "already exists"
	if len(clashing) > 1 {
		verb = "already exist"
	}
	return fmt.Errorf("%w: %s %s in %q — remove %s there or extract to another worktree",
		domain.ErrExtractConflict, strings.Join(clashing, ", "), verb, params.TargetBranch, them(len(clashing)))
}

// conflictingFiles returns the tracked files whose changes do not apply cleanly
// onto the target, checked one by one so the message can name them.
func conflictingFiles(sourcePath, targetPath string, tracked []string) []string {
	var conflicts []string
	for _, f := range tracked {
		patch, err := infra.DiffFiles(infra.DiffFilesParams{WorktreePath: sourcePath, Files: []string{f}})
		if err != nil {
			conflicts = append(conflicts, f)
			continue
		}
		if err := infra.ApplyPatch(infra.ApplyPatchParams{
			WorktreePath: targetPath,
			Patch:        patch,
			ThreeWay:     true,
			Check:        true,
		}); err != nil {
			conflicts = append(conflicts, f)
		}
	}
	return conflicts
}

// conflictError builds a readable, actionable conflict message naming the files
// that clash with the target worktree.
func conflictError(files []string, targetBranch string) error {
	verb := "was also modified"
	if len(files) > 1 {
		verb = "were also modified"
	}
	return fmt.Errorf("%w: %s %s in %q — resolve %s there or extract to another worktree",
		domain.ErrExtractConflict, strings.Join(files, ", "), verb, targetBranch, them(len(files)))
}

// them returns the object pronoun matching the file count.
func them(n int) string {
	if n == 1 {
		return "it"
	}
	return "them"
}

type copyUntrackedParams struct {
	SourcePath string
	TargetPath string
	Files      []string
}

func copyUntracked(params copyUntrackedParams) error {
	for _, f := range params.Files {
		if err := infra.CopyPath(infra.CopyPathParams{
			SourceDir: params.SourcePath,
			TargetDir: params.TargetPath,
			RelPath:   f,
		}); err != nil {
			return fmt.Errorf("copy %s: %w", f, err)
		}
	}
	return nil
}

type cleanSourceParams struct {
	SourcePath string
	Patch      []byte
	Tracked    []string
	Untracked  []string
}

// cleanSource reverts the extracted changes in the source worktree by
// reverse-applying the same patch and unstaging the affected paths, then deletes
// the extracted untracked files.
func cleanSource(params cleanSourceParams) error {
	if err := infra.ApplyPatch(infra.ApplyPatchParams{
		WorktreePath: params.SourcePath,
		Patch:        params.Patch,
		Reverse:      true,
	}); err != nil {
		return err
	}
	if err := infra.ResetPaths(infra.ResetPathsParams{
		WorktreePath: params.SourcePath,
		Files:        params.Tracked,
	}); err != nil {
		return err
	}
	removeTargetFiles(params.SourcePath, params.Untracked)
	return nil
}

// rollbackTarget best-effort undoes a partial application on the target so a
// failed extraction leaves it as it was.
func rollbackTarget(targetPath string, patch []byte, tracked []string) {
	_ = infra.ApplyPatch(infra.ApplyPatchParams{
		WorktreePath: targetPath,
		Patch:        patch,
		ThreeWay:     true,
		Reverse:      true,
	})
	_ = infra.ResetPaths(infra.ResetPathsParams{WorktreePath: targetPath, Files: tracked})
}

func removeTargetFiles(dir string, files []string) {
	for _, f := range files {
		_ = os.RemoveAll(filepath.Join(dir, f))
	}
}
