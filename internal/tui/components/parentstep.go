package components

import "github.com/LucasPcq/wtm/internal/domain"

// ParentStepContext is what a ParentStep resolves, on entry, from the completed
// prior wizard steps: which worktree to exclude from the candidate list and the
// description to render above it. Keeping this per-entry lets the parent list track
// a worktree chosen in an earlier step.
type ParentStepContext struct {
	Exclude     string
	Description string
}

// ParentStepParams configures the shared "pick a parent branch" wizard step used by
// both `wtm reparent` (change a managed worktree's parent) and `wtm relocate` (assign
// a parent when adopting an external worktree). The two commands own different
// concerns — logical rebase target vs. physical adoption — but the act of choosing a
// parent branch is identical, so the picker lives here once.
//
// The step is a single-select, refreshable ('r') list of candidate branches. Resolve
// derives the per-entry exclusion and description so each caller keeps its own wording
// and its own way of learning which worktree is being targeted (a fixed branch for a
// relocate adoption, or the worktree chosen in an earlier reparent step).
type ParentStepParams struct {
	Name         string
	Title        string
	Holder       *[]domain.BranchCandidate
	Pinned       string
	PinnedSuffix string
	Callout      bool
	Resolve      func(prev []Step) ParentStepContext
}

// ParentStep builds the shared parent-branch picker step. The candidate list is
// rebuilt on entry (Build) from Resolve, so it reflects the worktree chosen earlier
// and any branch list refreshed in the background.
func ParentStep(params ParentStepParams) Step {
	build := func(prev []Step) any {
		ctx := params.Resolve(prev)
		return NewSelectList(NewSelectListParams{
			Title:       params.Title,
			Description: ctx.Description,
			Items: BranchItems(BranchItemsParams{
				Candidates:   *params.Holder,
				Pinned:       params.Pinned,
				PinnedSuffix: params.PinnedSuffix,
				Exclude:      ctx.Exclude,
			}),
		})
	}
	return Step{
		Name:       params.Name,
		Model:      build(nil), // placeholder; rebuilt from the resolved worktree on entry
		Build:      build,
		CanRefresh: true,
		Summary:    SelectSummary,
		Callout:    params.Callout,
	}
}
