// Package reparent renders the interactive multi-step wizard for `wtm reparent`:
// pick the worktree to reparent, then pick its new parent — with a breadcrumb and
// back-navigation between the two steps.
package reparent

import (
	"fmt"
	"sort"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/infra"
	"github.com/LucasPcq/wtm/internal/service/branch"
	"github.com/LucasPcq/wtm/internal/tui/branchrefresh"
	"github.com/LucasPcq/wtm/internal/tui/components"
)

// RunWizardParams holds the inputs for the reparent wizard. A non-empty Branch or
// NewParent presets that value and drops its picker step (e.g. the branch was
// given on the command line, only the parent needs choosing).
type RunWizardParams struct {
	ProjectDir string
	Branch     string
	NewParent  string
	// CurrentParents maps each worktree to its currently recorded parent, shown as
	// an indication on the new-parent step. The recorded parent may no longer exist
	// as a branch (e.g. merged then cleaned), so it is surfaced as text, never as a
	// list badge that would silently vanish.
	CurrentParents map[string]string
}

// RunResult is the outcome of the wizard: the worktree to reparent and the branch
// it should now be rebased onto.
type RunResult struct {
	Branch    string
	NewParent string
}

// RunWizard drives the reparent flow as a single multi-step wizard. The new-parent
// step is rebuilt from the chosen worktree so it can exclude that worktree from the
// candidate list. An Esc on the first step aborts (ErrUserAborted); on a later step
// it steps back without quitting.
func RunWizard(params RunWizardParams) (RunResult, error) {
	worktrees, err := listWorktreeItems(params.ProjectDir)
	if err != nil {
		return RunResult{}, err
	}
	holder := &[]domain.BranchCandidate{}
	*holder = branch.Candidates(branch.ListParams{ProjectDir: params.ProjectDir})

	steps := make([]components.Step, 0, 2)
	worktreeStep := -1
	if params.Branch == "" {
		if len(worktrees) == 0 {
			return RunResult{}, fmt.Errorf("no worktrees to reparent")
		}
		worktreeStep = len(steps)
		steps = append(steps, components.Step{
			Name: "Select worktree",
			Model: components.NewSelectList(components.NewSelectListParams{
				Title:       "Select worktree to reparent",
				Description: "Choose the worktree whose parent you want to change",
				Items:       worktrees,
			}),
			Summary: components.SelectSummary,
		})
	}

	parentStepIdx := -1
	if params.NewParent == "" {
		parentStepIdx = len(steps)
		steps = append(steps, parentStep(parentStepParams{
			Holder:          holder,
			PresetExclude:   params.Branch,
			WorktreeStepIdx: worktreeStep,
			CurrentParents:  params.CurrentParents,
		}))
	}

	final, err := components.RunWizard(components.RunWizardParams{
		Steps:       steps,
		Stderr:      true,
		ErrLabel:    "reparent wizard",
		InitCmd:     branchrefresh.Cmd(params.ProjectDir),
		Loading:     true,
		LoadingText: domain.LoadingBranchesText,
		OnMsg:       branchrefresh.Handler(params.ProjectDir, holder),
	})
	if err != nil {
		return RunResult{}, err
	}

	done := final.Steps()
	result := RunResult{Branch: params.Branch, NewParent: params.NewParent}
	if worktreeStep >= 0 {
		result.Branch = stepValue(done, worktreeStep)
	}
	if parentStepIdx >= 0 {
		result.NewParent = stepValue(done, parentStepIdx)
	}
	return result, nil
}

type parentStepParams struct {
	Holder          *[]domain.BranchCandidate
	PresetExclude   string
	WorktreeStepIdx int
	CurrentParents  map[string]string
}

func parentStep(params parentStepParams) components.Step {
	build := func(prev []components.Step) any {
		exclude := params.PresetExclude
		if params.WorktreeStepIdx >= 0 {
			exclude = stepValue(prev, params.WorktreeStepIdx)
		}
		return components.NewSelectList(components.NewSelectListParams{
			Title:       "Select new parent branch",
			Description: parentStepDescription(params.CurrentParents[exclude]),
			Items:       parentItems(*params.Holder, exclude),
		})
	}

	return components.Step{
		Name:       "Select new parent",
		Model:      build(nil), // placeholder; rebuilt from the chosen worktree
		Build:      build,
		CanRefresh: true,
		Summary:    components.SelectSummary,
	}
}

// parentStepDescription tells the user what the worktree is rebased onto today,
// so the change is explicit. The recorded parent is shown even when it no longer
// exists as a branch (merged + cleaned), since that is exactly when reparenting is
// needed.
func parentStepDescription(currentParent string) string {
	if currentParent == "" {
		return "No parent recorded yet — pick the branch to rebase onto on the next sync"
	}
	return fmt.Sprintf("Currently rebased onto %s — pick the new parent (applied on the next sync)", currentParent)
}

func parentItems(branches []domain.BranchCandidate, exclude string) []components.SelectItem {
	return components.BranchItems(components.BranchItemsParams{
		Candidates: branches,
		Exclude:    exclude,
	})
}

func listWorktreeItems(projectDir string) ([]components.SelectItem, error) {
	worktrees, err := infra.ListWorktrees(infra.ListWorktreesParams{ProjectDir: projectDir})
	if err != nil {
		return nil, fmt.Errorf("list worktrees: %w", err)
	}

	items := make([]components.SelectItem, 0, len(worktrees))
	for _, wt := range worktrees {
		if wt.IsMain {
			continue
		}
		items = append(items, components.SelectItem{Label: wt.Branch, Value: wt.Branch})
	}
	sort.Slice(items, func(i, j int) bool { return items[i].Value < items[j].Value })
	return items, nil
}

func stepValue(steps []components.Step, idx int) string {
	if idx < 0 || idx >= len(steps) {
		return ""
	}
	sl, ok := steps[idx].Model.(components.SelectListModel)
	if !ok {
		return ""
	}
	return sl.Value()
}
