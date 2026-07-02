// Package prune renders the interactive flow for `wtm prune`: a multi-select of
// the worktrees that matched the prune filters, followed by a confirmation that
// surfaces a force option when an unsafe worktree (dirty, unpushed commits, or an
// open PR) is checked (mirroring clean).
package prune

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/styles"
	"github.com/LucasPcq/wtm/internal/tui/components"
)

// Confirm-step action choices (the constant "No, cancel" row uses
// domain.WizardCancelValue).
const (
	confirmYes   = "yes"
	confirmForce = "force"
)

// Reparent-step choices.
const (
	reparentYes = "reparent"
	reparentNo  = "orphan"
)

// Step names, used for construction and value extraction.
const (
	stepWorktrees = "Worktrees"
	stepReparent  = "Reparent children"
	stepConfirm   = "Confirm"
)

// ReparentPreviewFunc computes, for the current selection and force choice, the
// children a prune would orphan and could reparent onto their grandparent.
// Injected by the command layer so the picker stays free of prune business logic.
type ReparentPreviewFunc func(chosen []string, force bool) []domain.ReparentResult

// RunParams holds the inputs for the prune picker.
type RunParams struct {
	Plan domain.PrunePlan
	// ReparentPreview derives the reparent moves shown in the final confirmation
	// step from the live selection; nil disables the step.
	ReparentPreview ReparentPreviewFunc
}

// RunResult is the picker outcome: the checked branches, whether the user
// confirmed with force (allowing unsafe worktrees to be removed), and — when the
// reparent step applied — whether surviving children should be reparented.
type RunResult struct {
	Branches []string
	Force    bool
	// ReparentAsked is true when the reparent step was shown; ReparentChildren
	// then holds the user's answer (reparent vs leave orphaned).
	ReparentAsked    bool
	ReparentChildren bool
}

// Run shows the candidate multi-select (unsafe ones tagged and left unchecked),
// a confirmation screen, then — when children would be orphaned — a reparent
// confirmation, all in one wizard. Returns domain.ErrUserAborted on Esc at the
// first step or the explicit "No, cancel".
func Run(params RunParams) (RunResult, error) {
	plan := params.Plan
	// The picker may be reached through a shell wrapper that captures stdout, so
	// force lipgloss to detect color against stderr (the TTY).
	styles.UseRendererOn(os.Stderr)

	items := make([]components.MultiSelectItem, 0, len(plan.Selected))
	for _, c := range plan.Selected {
		tag, variant := candidateTag(c)
		items = append(items, components.MultiSelectItem{
			Label:    c.Branch,
			Value:    c.Branch,
			Selected: c.UnsafeReason == "", // unsafe worktrees are opt-in
			Tag:      tag,
			Variant:  variant,
		})
	}

	ms := components.NewMultiSelect(components.NewMultiSelectParams{
		Title:       "Select worktrees to prune",
		Description: domain.MultiSelectHint,
		Items:       items,
	})

	// Order: multi-select → reparent (optional select) → confirm recap (last).
	// The reparent decision precedes the recap so the recap is reliably the final,
	// unconditional action point.
	steps := []components.Step{
		{Name: stepWorktrees, Model: ms},
	}
	if params.ReparentPreview != nil {
		steps = append(steps, reparentStep(params.ReparentPreview))
	}
	steps = append(steps, confirmStep(plan, params.ReparentPreview))

	final, err := components.RunWizard(components.RunWizardParams{
		Steps:    steps,
		Stderr:   true,
		ErrLabel: "prune picker",
	})
	if err != nil {
		return RunResult{}, err
	}

	finalSteps := final.Steps()
	msModel, ok := finalSteps[0].Model.(components.MultiSelectModel)
	if !ok {
		return RunResult{}, errors.New("unexpected model type")
	}

	if stepSelectValue(finalSteps, stepConfirm) == domain.WizardCancelValue {
		return RunResult{}, domain.ErrUserAborted
	}
	result := RunResult{
		Branches: msModel.Values(),
		Force:    stepSelectValue(finalSteps, stepConfirm) == confirmForce,
	}
	if idx := stepIndex(finalSteps, stepReparent); idx >= 0 && !final.Skipped(idx) {
		result.ReparentAsked = true
		result.ReparentChildren = stepSelectValue(finalSteps, stepReparent) == reparentYes
	}
	return result, nil
}

// reparentStep builds the conditional reparent decision as a select: it derives
// the orphaned children from the live selection and is skipped (with a reason)
// when the prune leaves no children to reparent. It precedes the confirm recap so
// every option merely advances — Esc returns to the multi-select, never aborting.
// The preview assumes force so it lists the maximal set of children; the actual
// reparent plan is recomputed by the command layer from the confirmed force value.
func reparentStep(preview ReparentPreviewFunc) components.Step {
	return components.ChoiceStep(components.ChoiceStepParams{
		Name:    stepReparent,
		Summary: reparentSummary,
		Decide: func(prev []components.Step) (bool, string, components.NewSelectListParams) {
			moves := preview(selectedBranches(prev), true)
			if len(moves) == 0 {
				return false, "no children to reparent", components.NewSelectListParams{}
			}
			return true, "", components.NewSelectListParams{
				Description: reparentProposalText(moves),
				Items: []components.SelectItem{
					{Label: fmt.Sprintf("Reparent onto grandparent (%d)", len(moves)), Value: reparentYes},
					{Separator: true},
					{Label: "Leave orphaned", Value: reparentNo},
				},
			}
		},
	})
}

// reparentSummary labels the reparent choice in the completed-step summaries.
func reparentSummary(model any) string {
	sl, ok := model.(components.SelectListModel)
	if !ok {
		return ""
	}
	if sl.Value() == reparentYes {
		return "reparent onto grandparent"
	}
	return "leave orphaned"
}

// stepSelectValue reads the chosen value of the named SelectList step.
func stepSelectValue(steps []components.Step, name string) string {
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

// stepIndex returns the position of the named step, or -1 when absent.
func stepIndex(steps []components.Step, name string) int {
	for i, s := range steps {
		if s.Name == name {
			return i
		}
	}
	return -1
}

// reparentProposalText lists the children a prune would orphan and where each
// would be reparented, as the reparent step's description.
func reparentProposalText(moves []domain.ReparentResult) string {
	lines := make([]string, 0, len(moves)+1)
	lines = append(lines, domain.PruneReparentIntro)
	for _, m := range moves {
		lines = append(lines, fmt.Sprintf("  • %s will rebase onto %s instead of %s", m.Branch, m.NewParent, m.OldParent))
	}
	return strings.Join(lines, "\n")
}

// confirmStep builds the final recap step. It recaps what will be pruned, the
// reparent decision made earlier, and offers a danger "force" option only when an
// unsafe worktree (dirty, unpushed, or open PR) is currently checked, followed by
// the constant "No, cancel" row.
func confirmStep(plan domain.PrunePlan, preview ReparentPreviewFunc) components.Step {
	return components.RecapStep(components.RecapStepParams{
		Name: stepConfirm,
		Build: func(prev []components.Step) components.RecapContent {
			selected := selectedBranches(prev)
			desc := confirmDescription(selected)
			if line := reparentRecapLine(prev, preview, selected); line != "" {
				desc += "\n\n" + line
			}
			actions := []components.SelectItem{{Label: "Yes, prune", Value: confirmYes}}
			if anyUnsafeSelected(selected, plan) {
				actions = append(actions,
					components.SelectItem{Separator: true},
					components.SelectItem{Label: "Yes, force prune (bypass safety checks)", Value: confirmForce, Danger: true},
				)
			}
			return components.RecapContent{
				Description: desc,
				Actions:     actions,
			}
		},
	})
}

// reparentRecapLine summarizes the reparent decision (made on the earlier step)
// for the recap, or "" when there are no children to reparent.
func reparentRecapLine(prev []components.Step, preview ReparentPreviewFunc, selected []string) string {
	if preview == nil {
		return ""
	}
	moves := preview(selected, true)
	if len(moves) == 0 {
		return ""
	}
	if stepSelectValue(prev, stepReparent) == reparentYes {
		return fmt.Sprintf("Then reparent %d child worktree(s) onto their grandparent.", len(moves))
	}
	return fmt.Sprintf("Then leave %d child worktree(s) orphaned.", len(moves))
}

// confirmDescription recaps the worktrees the confirmed prune will remove. The
// reparenting of surviving children is validated on its own screen afterwards
// (like clean), so it is not repeated here.
func confirmDescription(selected []string) string {
	if len(selected) == 0 {
		return "No worktrees selected — nothing will be pruned."
	}
	return fmt.Sprintf("Will prune %d worktree(s): %s", len(selected), strings.Join(selected, ", "))
}

// anyUnsafeSelected reports whether a currently-checked candidate is unsafe to
// remove without force (dirty, unpushed commits, or an open PR).
func anyUnsafeSelected(selected []string, plan domain.PrunePlan) bool {
	chosen := toSet(selected)
	for _, c := range plan.Selected {
		if c.UnsafeReason != "" && chosen[c.Branch] {
			return true
		}
	}
	return false
}

func toSet(branches []string) map[string]bool {
	set := make(map[string]bool, len(branches))
	for _, b := range branches {
		set[b] = true
	}
	return set
}

// selectedBranches reads the worktrees checked in the first (multi-select) step.
func selectedBranches(prev []components.Step) []string {
	if len(prev) == 0 {
		return nil
	}
	ms, ok := prev[0].Model.(components.MultiSelectModel)
	if !ok {
		return nil
	}
	return ms.Values()
}

// candidateTag maps a candidate to a short colored tag: unsafe candidates (dirty,
// unpushed, open PR) read as danger, everything else shows its prune reason in a
// muted/warning tone.
func candidateTag(c domain.PruneCandidate) (string, components.TagVariant) {
	switch c.UnsafeReason {
	case domain.PruneSkipDirty:
		return "dirty", components.TagDanger
	case domain.PruneSkipUnpushed:
		return "unpushed", components.TagDanger
	case domain.PruneSkipOpenPR:
		return "open PR", components.TagDanger
	}
	switch c.Reason {
	case domain.PruneReasonGone:
		return "gone", components.TagWarning
	case domain.PruneReasonPRMerged:
		return "merged", components.TagNeutral
	case domain.PruneReasonPRClosed:
		return "closed", components.TagNeutral
	default:
		return "", components.TagNeutral
	}
}
