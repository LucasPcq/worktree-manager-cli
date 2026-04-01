package worktree

import (
	"fmt"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/infra"
)

// CleanParams holds inputs for cleaning a worktree.
type CleanParams struct {
	ProjectDir string
	Branch     string
	Force      bool
	Config     domain.Config
}

// CleanCheckResult holds the pre-deletion check results.
type CleanCheckResult struct {
	WorktreePath    string
	Branch          string
	UnpushedCommits int
	HasOpenPR       bool
	PRUrl           string
	IsDirty         bool
	IsParent        bool
}

// HasWarnings returns true if any condition warrants showing the force option.
func (c CleanCheckResult) HasWarnings() bool {
	return c.UnpushedCommits > 0 || c.HasOpenPR || c.IsDirty
}

// Check performs pre-deletion checks without deleting anything.
func Check(params CleanParams) (CleanCheckResult, error) {
	wt, err := infra.FindWorktreeByBranch(infra.FindWorktreeByBranchParams{
		ProjectDir: params.ProjectDir,
		Branch:     params.Branch,
	})
	if err != nil {
		return CleanCheckResult{}, err
	}

	if wt.IsMain {
		return CleanCheckResult{}, domain.ErrCannotCleanParent
	}

	unpushed, _ := infra.UnpushedCommits(infra.UnpushedCommitsParams{
		ProjectDir: params.ProjectDir,
		Branch:     params.Branch,
	})

	haspr, prurl := infra.HasOpenPR(infra.HasOpenPRParams{
		ProjectDir: params.ProjectDir,
		Branch:     params.Branch,
	})

	dirty, _ := infra.IsDirty(infra.IsDirtyParams{WorktreePath: wt.Path})

	return CleanCheckResult{
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
func Clean(params CleanParams) error {
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
