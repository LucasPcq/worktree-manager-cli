package clean

import (
	"fmt"
	"os"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/rules"
	"github.com/LucasPcq/wtm/internal/styles"
	"github.com/LucasPcq/wtm/internal/tui/components"
)

// Delete-step action choices (the constant "No, cancel" row uses
// domain.WizardCancelValue).
const (
	deleteYes   = "yes"
	deleteForce = "force"
)

// Reparent-step choices.
const (
	reparentYes = "reparent"
	reparentNo  = "orphan"
)

// Step names, used for construction and value extraction.
const (
	stepPicker   = "Worktree"
	stepReparent = "Reparent children"
	stepDelete   = "Delete"
)

// CheckFunc runs the pre-deletion safety check for a branch. It swallows errors
// into a zero result — doClean is the idempotent safety net for a worktree that
// vanished or is the parent.
type CheckFunc func(branch string) domain.CleanCheckResult

// ReparentPreviewFunc returns the children a clean of branch would orphan.
type ReparentPreviewFunc func(branch string) domain.CleanReparentPlan

// RunWizardParams holds inputs for the unified clean wizard.
type RunWizardParams struct {
	ProjectDir string
	// PreselectedBranch skips the picker (a branch was given as an argument); the
	// safety check is then supplied via PreCheck.
	PreselectedBranch string
	PreCheck          *domain.CleanCheckResult
	// Check runs the safety check asynchronously when the delete step is entered,
	// for a branch chosen in the picker (unused when PreselectedBranch is set).
	Check           CheckFunc
	ReparentPreview ReparentPreviewFunc
	// Force is --force: lift safety refusals. The wizard still shows the delete
	// recap (confirmation) with the warnings visible; confirming applies the force.
	Force bool
	// ReparentChildren is --reparent-children: reparent orphaned children onto the
	// grandparent without prompting. When true the reparent step is skipped and the
	// recap states the forced reparent.
	ReparentChildren bool
}

// RunResult is the wizard outcome.
type RunResult struct {
	Branch string
	Force  bool
	// ReparentAsked is true when the reparent step was shown; ReparentChildren then
	// holds the answer.
	ReparentAsked    bool
	ReparentChildren bool
}

// checkRequestMsg asks the message handler to run the safety check for a branch;
// checkDoneMsg carries its result back.
type (
	checkRequestMsg struct{ branch string }
	checkDoneMsg    struct{ check domain.CleanCheckResult }
)

// RunWizard runs the full clean flow in one wizard: worktree picker (when no
// branch was given) → delete confirmation → reparent confirmation. The safety
// check for a picked worktree queries the PR state over the network, so it runs
// asynchronously behind the shared loading spinner. Esc on the delete or reparent
// step goes back a step instead of aborting the whole clean. Returns
// domain.ErrUserAborted on Esc at the first step or the explicit "No, cancel".
func RunWizard(params RunWizardParams) (RunResult, error) {
	// The picker may be reached through a shell wrapper that captures stdout, so
	// force lipgloss to detect color against stderr (the TTY).
	styles.UseRendererOn(os.Stderr)

	hasPicker := params.PreselectedBranch == ""
	var steps []components.Step

	// Order: [picker] → [reparent select] → delete recap (last). The reparent
	// decision precedes the recap so the recap is reliably the final, unconditional
	// action point.
	if hasPicker {
		items, err := pickableItems(params.ProjectDir)
		if err != nil {
			return RunResult{}, err
		}
		steps = append(steps, components.Step{
			Name: stepPicker,
			Model: components.NewSelectList(components.NewSelectListParams{
				Title:       "Select worktree to clean",
				Description: "The parent worktree cannot be cleaned",
				Items:       items,
			}),
			Summary: components.SelectSummary,
		})
		// Reparent is conditional on the picked branch, so it is a ChoiceStep that
		// auto-skips (with a reason) when the branch has no orphaned children.
		// --reparent-children forces reparenting without a prompt, so the step is
		// omitted entirely then.
		if !params.ReparentChildren {
			steps = append(steps, reparentStep(params.ReparentPreview))
		}
	} else if params.ReparentPreview != nil && !params.ReparentChildren {
		// Branch is known up front: compute children synchronously and add a concrete
		// reparent step only when there are any. A ChoiceStep cannot be the first
		// step (the wizard never builds or auto-skips step 0), so this path avoids it.
		if plan := params.ReparentPreview(params.PreselectedBranch); len(plan.Children) > 0 {
			steps = append(steps, reparentConcreteStep(plan))
		}
	}

	deleteIdx := len(steps)
	if hasPicker {
		// Empty placeholder: Enter is a no-op until the async check replaces it with
		// the real options, so the user can't confirm before seeing the warnings.
		steps = append(steps, components.Step{
			Name:    stepDelete,
			Model:   deletePlaceholder(),
			Recap:   true,
			Summary: components.SelectSummary,
			OnEnter: func(prev []components.Step) tea.Cmd {
				branch := pickerValue(prev)
				return func() tea.Msg { return checkRequestMsg{branch: branch} }
			},
		})
	} else {
		check := domain.CleanCheckResult{}
		if params.PreCheck != nil {
			check = *params.PreCheck
		}
		branch := params.PreselectedBranch
		steps = append(steps, components.Step{
			Name:    stepDelete,
			Model:   deleteStep(check, ""),
			Recap:   true,
			Summary: components.SelectSummary,
			// Rebuilt on entry so the recap reflects the reparent choice made just
			// before it (when the branch has children; otherwise delete is step 0 and
			// Build does not run, which is fine — there is no reparent line then).
			Build: func(prev []components.Step) any {
				return deleteStep(check, reparentRecapLine(prev, params.ReparentPreview, branch, params.ReparentChildren))
			},
		})
	}

	final, err := components.RunWizard(components.RunWizardParams{
		Steps:    steps,
		Stderr:   true,
		ErrLabel: "clean wizard",
		OnMsg: func(w *components.WizardModel, msg tea.Msg) (tea.Cmd, bool) {
			switch m := msg.(type) {
			case checkRequestMsg:
				// Clear any options left from a previously-picked worktree so the
				// spinner isn't shown over stale buttons while the new check runs.
				w.UpdateStepModel(deleteIdx, func(any) any { return deletePlaceholder() })
				loadCmd := w.StartLoading("Checking worktree…")
				return tea.Batch(loadCmd, runCheckCmd(m.branch, params.Check)), true
			case checkDoneMsg:
				result := m.check
				line := reparentRecapLine(w.Steps(), params.ReparentPreview, pickerValue(w.Steps()), params.ReparentChildren)
				w.UpdateStepModel(deleteIdx, func(any) any { return deleteStep(result, line) })
				w.SetLoading(false)
				return nil, true
			}
			return nil, false
		},
	})
	if err != nil {
		return RunResult{}, err
	}

	finalSteps := final.Steps()
	deleteSel, ok := finalSteps[deleteIdx].Model.(components.SelectListModel)
	if !ok {
		return RunResult{}, fmt.Errorf("unexpected model type")
	}
	if deleteSel.Value() == domain.WizardCancelValue {
		return RunResult{}, domain.ErrUserAborted
	}

	result := RunResult{
		Branch: params.PreselectedBranch,
		Force:  params.Force || deleteSel.Value() == deleteForce,
	}
	if hasPicker {
		result.Branch = pickerValue(finalSteps)
	}

	// --reparent-children forces the reparent without a step; otherwise the answer
	// comes from the reparent step when it ran.
	if params.ReparentChildren {
		result.ReparentAsked = true
		result.ReparentChildren = true
	} else if idx := stepIndex(finalSteps, stepReparent); idx >= 0 && !final.Skipped(idx) {
		result.ReparentAsked = true
		result.ReparentChildren = stepValue(finalSteps, stepReparent) == reparentYes
	}
	return result, nil
}

// runCheckCmd runs the safety check off the UI goroutine so the spinner animates.
func runCheckCmd(branch string, check CheckFunc) tea.Cmd {
	return func() tea.Msg { return checkDoneMsg{check: check(branch)} }
}

// deletePlaceholder is the empty delete step shown while the async safety check
// runs: with no items, Enter is a no-op, so the user cannot confirm before the
// real options (with any warnings) replace it.
func deletePlaceholder() components.SelectListModel {
	return components.NewSelectList(components.NewSelectListParams{Title: "Proceed with deletion?"})
}

// deleteStep builds the delete confirmation, recapping what will be removed (and
// the reparent decision made earlier) and offering a danger "force" option only
// when the worktree is unsafe (dirty, unpushed commits, or an open PR).
func deleteStep(check domain.CleanCheckResult, reparentLine string) components.SelectListModel {
	items := []components.SelectItem{{Label: "Yes, delete", Value: deleteYes}}
	if rules.HasWarnings(check) {
		items = append(items,
			components.SelectItem{Separator: true},
			components.SelectItem{Label: "Yes, force delete (bypass all checks)", Value: deleteForce, Danger: true},
		)
	}
	items = append(items,
		components.SelectItem{Separator: true},
		components.SelectItem{Label: domain.WizardCancelLabel, Value: domain.WizardCancelValue},
	)

	return components.NewSelectList(components.NewSelectListParams{
		Title:       "Proceed with deletion?",
		Description: deleteDescription(check, reparentLine),
		Items:       items,
	})
}

// deleteDescription recaps the warnings, what the delete will remove, and the
// reparent decision (when there are children).
func deleteDescription(check domain.CleanCheckResult, reparentLine string) string {
	var lines []string
	if check.IsDirty {
		lines = append(lines, "Worktree has uncommitted changes")
	}
	if check.UnpushedCommits > 0 {
		lines = append(lines, fmt.Sprintf("%d commit(s) not pushed to remote", check.UnpushedCommits))
	}
	if check.HasOpenPR {
		lines = append(lines, "Open PR: "+check.PRUrl)
	}
	if len(lines) > 0 {
		lines = append(lines, "")
	}
	lines = append(lines,
		"Will delete:",
		"  worktree  "+check.WorktreePath,
		"  branch    "+check.Branch,
	)
	if reparentLine != "" {
		lines = append(lines, "", reparentLine)
	}
	return strings.Join(lines, "\n")
}

// reparentRecapLine summarizes the reparent decision for the delete recap, or ""
// when the branch has no orphaned children. forced is --reparent-children: the
// step was omitted, so the recap states the reparent without reading a step value.
func reparentRecapLine(steps []components.Step, preview ReparentPreviewFunc, branch string, forced bool) string {
	if preview == nil || branch == "" {
		return ""
	}
	plan := preview(branch)
	if len(plan.Children) == 0 {
		return ""
	}
	if forced || stepValue(steps, stepReparent) == reparentYes {
		return fmt.Sprintf("Then reparent %d child worktree(s) onto %s.", len(plan.Children), plan.Grandparent)
	}
	return fmt.Sprintf("Then leave %d child worktree(s) orphaned.", len(plan.Children))
}

// reparentStep builds the conditional reparent decision as a select for the
// picker path: it derives the orphaned children from the picked branch and is
// skipped (with a reason) when there are none. It precedes the delete recap so
// every option merely advances — Esc returns to the picker, never aborting.
func reparentStep(preview ReparentPreviewFunc) components.Step {
	return components.ChoiceStep(components.ChoiceStepParams{
		Name:    stepReparent,
		Summary: reparentSummary,
		Decide: func(prev []components.Step) (bool, string, components.NewSelectListParams) {
			plan := preview(branchFromPrev(prev, ""))
			if len(plan.Children) == 0 {
				return false, "no orphaned children", components.NewSelectListParams{}
			}
			return true, "", reparentSelectParams(plan)
		},
	})
}

// reparentConcreteStep builds an always-shown reparent select for the preselected
// path, where the branch (and thus its orphaned children) is known up front. It
// avoids ChoiceStep because it may be the wizard's first step.
func reparentConcreteStep(plan domain.CleanReparentPlan) components.Step {
	return components.Step{
		Name:    stepReparent,
		Model:   components.NewSelectList(reparentSelectParams(plan)),
		Summary: reparentSummary,
	}
}

// reparentSelectParams builds the shared reparent SelectList (reparent onto the
// grandparent vs leave orphaned), recapping the moves in the description.
func reparentSelectParams(plan domain.CleanReparentPlan) components.NewSelectListParams {
	return components.NewSelectListParams{
		Description: reparentProposalText(plan),
		Items: []components.SelectItem{
			{Label: fmt.Sprintf("Reparent onto %s (%d)", plan.Grandparent, len(plan.Children)), Value: reparentYes},
			{Separator: true},
			{Label: "Leave orphaned", Value: reparentNo},
		},
	}
}

// reparentSummary labels the reparent choice in the completed-step summaries.
func reparentSummary(model any) string {
	sl, ok := model.(components.SelectListModel)
	if !ok {
		return ""
	}
	if sl.Value() == reparentYes {
		return "reparent"
	}
	return "leave orphaned"
}

// pickerValue reads the branch chosen on the picker step.
func pickerValue(steps []components.Step) string {
	return stepValue(steps, stepPicker)
}

// branchFromPrev resolves the branch being cleaned: the picker choice, else the
// preselected argument branch.
func branchFromPrev(steps []components.Step, preselected string) string {
	if v := stepValue(steps, stepPicker); v != "" {
		return v
	}
	return preselected
}

// stepIndex returns the position of the named step, or -1 when absent.
func stepIndex(steps []components.Step, name string) int {
	for i, s := range steps {
		if s.Name == name {
			return i
		}
	}
	return -1
}

func stepValue(steps []components.Step, name string) string {
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

// reparentProposalText lists the children a clean would orphan as the reparent
// confirmation's description (the wizard applies its own muted style).
func reparentProposalText(plan domain.CleanReparentPlan) string {
	lines := make([]string, 0, len(plan.Children)+1)
	lines = append(lines, domain.CleanReparentIntro)
	for _, c := range plan.Children {
		lines = append(lines, fmt.Sprintf("  • %s will rebase onto %s instead of %s", c.Branch, c.NewParent, c.OldParent))
	}
	return strings.Join(lines, "\n")
}
