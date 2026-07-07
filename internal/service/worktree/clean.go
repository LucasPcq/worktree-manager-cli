package worktree

import (
	"fmt"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/infra"
	"github.com/LucasPcq/wtm/internal/rules"
	ghservice "github.com/LucasPcq/wtm/internal/service/github"
	"github.com/LucasPcq/wtm/internal/service/hooks"
)

// Check performs pre-deletion checks without deleting anything.
func Check(params domain.CleanParams) (domain.CleanCheckResult, error) {
	wt, err := infra.FindWorktreeByBranch(infra.FindWorktreeByBranchParams{
		ProjectDir: params.ProjectDir,
		Branch:     params.Branch,
	})
	if err != nil {
		return domain.CleanCheckResult{}, err
	}

	if wt.IsMain {
		return domain.CleanCheckResult{}, domain.ErrCannotCleanParent
	}

	unpushed, _ := infra.UnpushedCommits(infra.UnpushedCommitsParams{
		ProjectDir: params.ProjectDir,
		Branch:     params.Branch,
	})

	haspr, _, prurl := ghservice.HasOpenPR(ghservice.HasOpenPRParams{
		ProjectDir: params.ProjectDir,
		Branch:     params.Branch,
	})

	dirty, _ := infra.IsDirty(infra.IsDirtyParams{WorktreePath: wt.Path})

	return domain.CleanCheckResult{
		WorktreePath:    wt.Path,
		Branch:          params.Branch,
		UnpushedCommits: unpushed,
		HasOpenPR:       haspr,
		PRUrl:           prurl,
		IsDirty:         dirty,
		IsParent:        wt.IsMain,
	}, nil
}

// Clean removes the worktree and deletes the local branch.
func Clean(params domain.CleanParams) error {
	wt, err := infra.FindWorktreeByBranch(infra.FindWorktreeByBranchParams{
		ProjectDir: params.ProjectDir,
		Branch:     params.Branch,
	})
	if err != nil {
		return err
	}

	if wt.IsMain {
		return domain.ErrCannotCleanParent
	}

	if err := runOnCleanHooks(params, wt.Path); err != nil {
		return err
	}

	if err := infra.RemoveWorktree(infra.RemoveWorktreeParams{
		ProjectDir: params.ProjectDir,
		Path:       wt.Path,
		Force:      params.Force,
	}); err != nil {
		return fmt.Errorf("%w: %w", domain.ErrWorktreeRemoveFailed, err)
	}

	if err := infra.DeleteLocalBranch(infra.DeleteLocalBranchParams{
		ProjectDir: params.ProjectDir,
		Branch:     params.Branch,
		Force:      params.Force,
	}); err != nil {
		return fmt.Errorf("delete branch: %w", err)
	}

	return nil
}

// runOnCleanHooks runs the configured on_clean hooks in the worktree directory
// before it is removed (e.g. `docker compose down`). It runs after wtm has
// stopped its own services and before the directory is deleted, so hooks can
// still reference files being removed. A failing hook aborts the removal unless
// the entry sets continue_on_error.
func runOnCleanHooks(params domain.CleanParams, worktreePath string) error {
	if len(params.Config.Project.Hooks.OnClean) == 0 {
		return nil
	}

	mainPath, err := infra.FindMainWorktreePath(infra.FindMainWorktreeParams{
		ProjectDir: params.ProjectDir,
	})
	if err != nil {
		return fmt.Errorf("find main worktree: %w", err)
	}

	if err := hooks.RunHooks(hooks.RunHooksParams{
		Hooks:   params.Config.Project.Hooks.OnClean,
		WorkDir: worktreePath,
		Vars: rules.TemplateVars{
			Worktree: worktreePath,
			Branch:   params.Branch,
			Root:     mainPath,
		},
	}); err != nil {
		return fmt.Errorf("on_clean hooks: %w", err)
	}

	return nil
}

// ForceClean recovers a worktree whose `git worktree remove` failed on
// undeletable files: it deletes the directory with `sudo rm -rf`, prunes the
// stale git metadata, then deletes the local branch. Intended to run only after
// the user has confirmed the privileged deletion.
func ForceClean(params domain.ForceCleanParams) error {
	if err := infra.SudoDeleteDir(infra.SudoDeleteDirParams{Path: params.Path}); err != nil {
		return err
	}

	if err := infra.PruneWorktrees(params.ProjectDir); err != nil {
		return fmt.Errorf("prune worktrees: %w", err)
	}

	if err := infra.DeleteLocalBranch(infra.DeleteLocalBranchParams{
		ProjectDir: params.ProjectDir,
		Branch:     params.Branch,
		Force:      params.Force,
	}); err != nil {
		return fmt.Errorf("delete branch: %w", err)
	}

	return nil
}
