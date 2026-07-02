// Package reparent renders the interactive multi-step wizard for `wtm reparent`:
// multi-select the worktrees to reparent, then pick their new parent — with a
// breadcrumb and back-navigation between the steps.
package reparent

import (
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/infra"
	"github.com/LucasPcq/wtm/internal/service/branch"
	"github.com/LucasPcq/wtm/internal/tui/branchrefresh"
	"github.com/LucasPcq/wtm/internal/tui/components"
)

const (
	stepWorktrees  = "Select worktrees"
	stepConfirm    = "Confirm"
	reparentAction = "reparent"
)

// RunWizardParams holds the inputs for the reparent wizard. A non-empty Branches or
// NewParent presets that value and drops its picker step (e.g. the worktrees were
// given on the command line, only the parent needs choosing).
type RunWizardParams struct {
	ProjectDir string
	Branches   []string
	NewParent  string
	// CurrentParents maps each worktree to its currently recorded parent, shown as
	// an indication on the new-parent step. The recorded parent may no longer exist
	// as a branch (e.g. merged then cleaned), so it is surfaced as text, never as a
	// list badge that would silently vanish.
	CurrentParents map[string]string
}

// RunResult is the outcome of the wizard: the worktrees to reparent and the branch
// they should now be rebased onto.
type RunResult struct {
	Branches  []string
	NewParent string
}

// RunWizard drives the reparent flow as a single multi-step wizard. The new-parent
// step is rebuilt from the chosen worktrees so it can exclude them from the candidate
// list. An Esc on the first step aborts (ErrUserAborted); on a later step it steps
// back without quitting.
func RunWizard(params RunWizardParams) (RunResult, error) {
	branches, err := listWorktreeBranches(params.ProjectDir)
	if err != nil {
		return RunResult{}, err
	}
	holder := &[]domain.BranchCandidate{}
	*holder = branch.Candidates(branch.ListParams{ProjectDir: params.ProjectDir})

	steps := make([]components.Step, 0, 3)
	worktreeStepIdx := -1
	if len(params.Branches) == 0 {
		if len(branches) == 0 {
			return RunResult{}, fmt.Errorf("no worktrees to reparent")
		}
		worktreeStepIdx = len(steps)
		steps = append(steps, worktreeStep(branches))
	}

	parentStepIdx := -1
	if params.NewParent == "" {
		parentStepIdx = len(steps)
		steps = append(steps, parentStep(parentStepParams{
			Holder:          holder,
			PresetBranches:  params.Branches,
			WorktreeStepIdx: worktreeStepIdx,
			CurrentParents:  params.CurrentParents,
		}))
	}

	// The recap is always the last step, even when both the worktrees and the parent
	// were given as flags — the interactive run still confirms before recording.
	steps = append(steps, recapStep(recapStepParams{
		WorktreeStepIdx: worktreeStepIdx,
		ParentStepIdx:   parentStepIdx,
		PresetBranches:  params.Branches,
		PresetParent:    params.NewParent,
		CurrentParents:  params.CurrentParents,
	}))

	// The background branch refresh only feeds the parent picker; skip it (and its
	// loading spinner) when --to fixed the parent and there is no picker.
	wp := components.RunWizardParams{Steps: steps, Stderr: true, ErrLabel: "reparent wizard"}
	if parentStepIdx >= 0 {
		wp.InitCmd = branchrefresh.Cmd(params.ProjectDir)
		wp.Loading = true
		wp.LoadingText = domain.LoadingBranchesText
		wp.OnMsg = branchrefresh.Handler(params.ProjectDir, holder)
	}
	final, err := components.RunWizard(wp)
	if err != nil {
		return RunResult{}, err
	}

	done := final.Steps()
	if stepValueByName(done, stepConfirm) == domain.WizardCancelValue {
		return RunResult{}, domain.ErrUserAborted
	}
	result := RunResult{Branches: params.Branches, NewParent: params.NewParent}
	if worktreeStepIdx >= 0 {
		result.Branches = stepValues(done, worktreeStepIdx)
	}
	if parentStepIdx >= 0 {
		result.NewParent = stepValue(done, parentStepIdx)
	}
	return result, nil
}

// worktreeStep builds the multi-select of worktrees to reparent, requiring at least
// one to be checked before advancing.
func worktreeStep(branches []string) components.Step {
	items := make([]components.MultiSelectItem, 0, len(branches))
	for _, b := range branches {
		items = append(items, components.MultiSelectItem{Label: b, Value: b})
	}
	return components.Step{
		Name: stepWorktrees,
		Model: components.NewMultiSelect(components.NewMultiSelectParams{
			Title:       "Select worktrees to reparent",
			Description: "Choose the worktrees whose parent you want to change",
			Items:       items,
			Validate: func(selected []string) error {
				if len(selected) == 0 {
					return errors.New("select at least one worktree")
				}
				return nil
			},
		}),
		Summary: components.MultiSelectSummary("none"),
	}
}

// recapStepParams holds inputs for the reparent recap.
type recapStepParams struct {
	WorktreeStepIdx int
	ParentStepIdx   int
	PresetBranches  []string
	PresetParent    string
	CurrentParents  map[string]string
}

// recapStep builds the final, unconditional recap: it restates the worktrees and
// their new parent, then offers "Yes, reparent" then the constant "No, cancel".
func recapStep(params recapStepParams) components.Step {
	return components.RecapStep(components.RecapStepParams{
		Name: stepConfirm,
		Build: func(prev []components.Step) components.RecapContent {
			branches := params.PresetBranches
			if params.WorktreeStepIdx >= 0 {
				branches = stepValues(prev, params.WorktreeStepIdx)
			}
			parent := params.PresetParent
			if params.ParentStepIdx >= 0 {
				parent = stepValue(prev, params.ParentStepIdx)
			}
			return components.RecapContent{
				Description: recapBody(branches, parent, params.CurrentParents),
				Actions: []components.SelectItem{
					{Label: "Yes, reparent", Value: reparentAction},
				},
			}
		},
	})
}

// recapBody restates the pending reparent. A single worktree also shows its current
// parent (the common case, where the before/after is the point); a batch lists the
// worktrees and the shared new parent, current parents being heterogeneous.
func recapBody(branches []string, parent string, current map[string]string) string {
	if len(branches) == 1 {
		cur := current[branches[0]]
		if cur == "" {
			cur = "(none recorded)"
		}
		return strings.Join([]string{
			"Worktree:        " + branches[0],
			"Current parent:  " + cur,
			"New parent:      " + parent,
			"",
			"Applied on the next sync.",
		}, "\n")
	}
	labels := make([]string, len(branches))
	for i, b := range branches {
		old := current[b]
		if old == "" {
			old = "none"
		}
		labels[i] = fmt.Sprintf("%s (from %s)", b, old)
	}
	return strings.Join([]string{
		fmt.Sprintf("Worktrees (%d):  %s", len(branches), strings.Join(labels, ", ")),
		"New parent:     " + parent,
		"",
		"Applied on the next sync.",
	}, "\n")
}

// stepValueByName reads the chosen value of the named SelectList step.
func stepValueByName(steps []components.Step, name string) string {
	for _, s := range steps {
		if s.Name != name {
			continue
		}
		if sl, ok := s.Model.(components.SelectListModel); ok {
			return sl.Value()
		}
	}
	return ""
}

type parentStepParams struct {
	Holder          *[]domain.BranchCandidate
	PresetBranches  []string
	WorktreeStepIdx int
	CurrentParents  map[string]string
}

func parentStep(params parentStepParams) components.Step {
	return components.ParentStep(components.ParentStepParams{
		Name:   "Select new parent",
		Title:  "Select new parent branch",
		Holder: params.Holder,
		Resolve: func(prev []components.Step) components.ParentStepContext {
			selected := params.PresetBranches
			if params.WorktreeStepIdx >= 0 {
				selected = stepValues(prev, params.WorktreeStepIdx)
			}
			return components.ParentStepContext{
				Excludes:    selected,
				Description: parentStepDescription(selected, params.CurrentParents),
			}
		},
	})
}

// parentStepDescription tells the user what the worktrees are rebased onto today, so
// the change is explicit. For a single worktree the recorded parent is shown even
// when it no longer exists as a branch (merged + cleaned), since that is exactly when
// reparenting is needed; for several, individual parents are elided.
func parentStepDescription(selected []string, current map[string]string) string {
	if len(selected) == 1 {
		return singleParentDescription(current[selected[0]])
	}
	if len(selected) == 0 {
		return singleParentDescription("")
	}
	return fmt.Sprintf("Pick the new parent for the %d selected worktrees — applied on the next sync", len(selected))
}

func singleParentDescription(currentParent string) string {
	if currentParent == "" {
		return "No parent recorded yet — pick the branch to rebase onto on the next sync"
	}
	return fmt.Sprintf("Currently rebased onto %s — pick the new parent (applied on the next sync)", currentParent)
}

func listWorktreeBranches(projectDir string) ([]string, error) {
	worktrees, err := infra.ListWorktrees(infra.ListWorktreesParams{ProjectDir: projectDir})
	if err != nil {
		return nil, fmt.Errorf("list worktrees: %w", err)
	}

	branches := make([]string, 0, len(worktrees))
	for _, wt := range worktrees {
		if wt.IsMain {
			continue
		}
		branches = append(branches, wt.Branch)
	}
	sort.Strings(branches)
	return branches, nil
}

// stepValue reads the chosen value of a single-select step.
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

// stepValues reads the checked values of a multi-select step.
func stepValues(steps []components.Step, idx int) []string {
	if idx < 0 || idx >= len(steps) {
		return nil
	}
	ms, ok := steps[idx].Model.(components.MultiSelectModel)
	if !ok {
		return nil
	}
	return ms.Values()
}
