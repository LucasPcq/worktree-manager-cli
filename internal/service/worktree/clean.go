package worktree

import (
	"fmt"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/infra"
	ghservice "github.com/LucasPcq/wtm/internal/service/github"
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

	if err := infra.RemoveWorktree(infra.RemoveWorktreeParams{
		ProjectDir: params.ProjectDir,
		Path:       wt.Path,
		Force:      params.Force,
	}); err != nil {
		return fmt.Errorf("remove worktree: %w", err)
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
