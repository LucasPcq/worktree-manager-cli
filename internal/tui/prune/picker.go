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

// RunResult is the picker outcome: the checked branches and whether the user
// confirmed with force (allowing unsafe worktrees to be removed).
type RunResult struct {
	Branches []string
	Force    bool
}

// Run shows the candidate multi-select (unsafe ones tagged and left unchecked)
// then a confirmation screen. Returns domain.ErrUserAborted on Esc or "No".
func Run(plan domain.PrunePlan) (RunResult, error) {
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

	final, err := components.RunWizard(components.RunWizardParams{
		Steps: []components.Step{
			{Name: "Worktrees", Model: ms},
			{
				Name:  "Confirm",
				Model: confirmStep(nil, plan),
				Build: func(prev []components.Step) any { return confirmStep(prev, plan) },
			},
		},
		Stderr:   true,
		ErrLabel: "prune picker",
	})
	if err != nil {
		return RunResult{}, err
	}

	steps := final.Steps()
	msModel, ok := steps[0].Model.(components.MultiSelectModel)
	if !ok {
		return RunResult{}, errors.New("unexpected model type")
	}
	confirm, ok := steps[1].Model.(components.SelectListModel)
	if !ok {
		return RunResult{}, errors.New("unexpected model type")
	}

	if confirm.Value() == confirmNo {
		return RunResult{}, domain.ErrUserAborted
	}
	return RunResult{
		Branches: msModel.Values(),
		Force:    confirm.Value() == confirmForce,
	}, nil
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
	case domain.PruneReasonPRMerged, domain.PruneReasonMerged:
		return "merged", components.TagNeutral
	case domain.PruneReasonPRClosed:
		return "closed", components.TagNeutral
	default:
		return "", components.TagNeutral
	}
}
