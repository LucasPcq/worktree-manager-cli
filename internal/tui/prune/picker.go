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

// Confirm choices for the second step.
const (
	confirmYes   = "yes"
	confirmForce = "force"
	confirmNo    = "no"
)

// Step names, used for construction and value extraction.
const (
	stepWorktrees = "Worktrees"
	stepConfirm   = "Confirm"
	stepReparent  = "Reparent children"
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
		Description: "Space to toggle, a to select all, / to filter, enter to continue, esc to cancel.",
		Items:       items,
	})

	steps := []components.Step{
		{Name: stepWorktrees, Model: ms},
		{
			Name:  stepConfirm,
			Model: confirmStep(nil, plan),
			Build: func(prev []components.Step) any { return confirmStep(prev, plan) },
		},
	}
	if params.ReparentPreview != nil {
		steps = append(steps, reparentStep(params.ReparentPreview))
	}

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
	confirm, ok := finalSteps[1].Model.(components.SelectListModel)
	if !ok {
		return RunResult{}, errors.New("unexpected model type")
	}

	if confirm.Value() == confirmNo {
		return RunResult{}, domain.ErrUserAborted
	}
	result := RunResult{
		Branches: msModel.Values(),
		Force:    confirm.Value() == confirmForce,
	}
	if len(finalSteps) > 2 && !final.Skipped(2) {
		if rc, ok := finalSteps[2].Model.(components.ConfirmModel); ok {
			result.ReparentAsked = true
			result.ReparentChildren = rc.Confirmed()
		}
	}
	return result, nil
}

// reparentStep builds the conditional reparent confirmation: it derives the
// orphaned children from the live selection and force choice, and is skipped when
// the prune was cancelled or leaves no children to reparent. Hosted in the wizard
// so Esc returns to the confirm step instead of aborting the whole prune.
func reparentStep(preview ReparentPreviewFunc) components.Step {
	return components.ConfirmStep(components.ConfirmStepParams{
		Name:     stepReparent,
		YesLabel: "reparent",
		NoLabel:  "leave orphaned",
		Decide: func(prev []components.Step) (bool, components.NewConfirmParams) {
			if confirmChoice(prev) == confirmNo {
				return false, components.NewConfirmParams{}
			}
			moves := preview(selectedBranches(prev), confirmChoice(prev) == confirmForce)
			if len(moves) == 0 {
				return false, components.NewConfirmParams{}
			}
			return true, components.NewConfirmParams{
				Title:       fmt.Sprintf(domain.PruneReparentPrompt, len(moves)),
				Description: reparentProposalText(moves),
				DefaultYes:  true,
			}
		},
	})
}

// confirmChoice reads the value chosen on the confirm step.
func confirmChoice(prev []components.Step) string {
	for _, s := range prev {
		if s.Name != stepConfirm {
			continue
		}
		if sl, ok := s.Model.(components.SelectListModel); ok {
			return sl.Value()
		}
	}
	return ""
}

// reparentProposalText lists the children a prune would orphan as the reparent
// confirmation's description (the wizard applies its own muted style).
func reparentProposalText(moves []domain.ReparentResult) string {
	lines := make([]string, 0, len(moves)+1)
	lines = append(lines, domain.PruneReparentIntro)
	for _, m := range moves {
		lines = append(lines, fmt.Sprintf("  %s: %s → %s", m.Branch, m.OldParent, m.NewParent))
	}
	return strings.Join(lines, "\n")
}

// confirmStep builds the confirmation choice. It recaps the count and the
// reparenting that will follow, and offers a danger "force" option only when an
// unsafe worktree (dirty, unpushed, or open PR) is currently checked.
func confirmStep(prev []components.Step, plan domain.PrunePlan) components.SelectListModel {
	selected := selectedBranches(prev)
	desc := confirmDescription(selected)

	items := []components.SelectItem{{Label: "Yes, prune", Value: confirmYes}}
	if anyUnsafeSelected(selected, plan) {
		items = append(items,
			components.SelectItem{Separator: true},
			components.SelectItem{Label: "Yes, force prune (bypass safety checks)", Value: confirmForce, Danger: true},
		)
	}
	items = append(items,
		components.SelectItem{Separator: true},
		components.SelectItem{Label: "No, cancel", Value: confirmNo},
	)

	return components.NewSelectList(components.NewSelectListParams{
		Title:       "Proceed with prune?",
		Description: desc,
		Items:       items,
	})
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
