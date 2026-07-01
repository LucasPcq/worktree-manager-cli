// Package syncpicker renders a multi-select worktree picker for `wtm sync`,
// letting the user choose which worktrees to rebase (or the base, to refresh it)
// and how a conflicting rebase should be handled.
package syncpicker

import (
	"errors"
	"os"
	"strings"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/styles"
	"github.com/LucasPcq/wtm/internal/tui/components"
)

// Conflict-mode choices for the second wizard step.
const (
	conflictModeNormal = "normal"
	conflictModeKeep   = "keep"
)

// RunParams holds the inputs required to render the picker.
type RunParams struct {
	Statuses []domain.WorktreeStatus
	// DefaultKeepConflict pre-selects the "keep conflicts in progress" choice on
	// the conflict-mode step (e.g. when --keep-conflict was passed on the CLI).
	DefaultKeepConflict bool
}

// RunResult is the picker outcome: the chosen worktrees and the conflict mode.
type RunResult struct {
	Branches     []string
	KeepConflict bool
}

// Run shows the two-step picker — a worktree multi-select followed by a
// conflict-handling choice — and returns the checked branches plus whether to
// keep conflicts in progress. Returns domain.ErrUserAborted when the user
// presses Esc.
func Run(params RunParams) (RunResult, error) {
	// The picker may be reached through a shell wrapper that captures stdout, so
	// force lipgloss to detect color against stderr (the TTY).
	styles.UseRendererOn(os.Stderr)

	items := make([]components.MultiSelectItem, 0, len(params.Statuses))
	for _, status := range params.Statuses {
		items = append(items, components.MultiSelectItem{
			Label: label(status),
			Value: status.Branch,
		})
	}

	ms := components.NewMultiSelect(components.NewMultiSelectParams{
		Title:       "Select worktrees to sync",
		Description: "Space to toggle, a to select all, / to filter, enter to confirm, esc to cancel.",
		Items:       items,
		Validate: func(selected []string) error {
			if len(selected) == 0 {
				return errors.New("select at least one worktree")
			}
			return nil
		},
	})

	final, err := components.RunWizard(components.RunWizardParams{
		Steps: []components.Step{
			{Name: "Worktrees", Model: ms},
			{
				Name:  "On conflict",
				Model: conflictModeStep(nil, params.DefaultKeepConflict),
				Build: func(prev []components.Step) any {
					return conflictModeStep(prev, params.DefaultKeepConflict)
				},
			},
		},
		Stderr:   true,
		ErrLabel: "sync picker",
	})
	if err != nil {
		return RunResult{}, err
	}

	steps := final.Steps()
	msModel, ok := steps[0].Model.(components.MultiSelectModel)
	if !ok {
		return RunResult{}, errors.New("unexpected model type")
	}
	modeModel, ok := steps[1].Model.(components.SelectListModel)
	if !ok {
		return RunResult{}, errors.New("unexpected model type")
	}

	return RunResult{
		Branches:     msModel.Values(),
		KeepConflict: modeModel.Value() == conflictModeKeep,
	}, nil
}

// conflictModeStep builds the conflict-handling choice. The description echoes the
// worktrees selected in the prior step so the decision is made in context; the
// "keep" option leads (and is highlighted, being the danger row) when defaultKeep
// is set so a prior --keep-conflict is pre-selected.
func conflictModeStep(prev []components.Step, defaultKeep bool) components.SelectListModel {
	desc := "Choose what happens when a rebase hits a conflict."
	if selected := selectedBranches(prev); len(selected) > 0 {
		desc = "About to sync: " + strings.Join(selected, ", ") + "\n\n" + desc
	}

	normalItem := components.SelectItem{
		Label: "Sync normally — abort & keep worktrees clean on conflict",
		Value: conflictModeNormal,
	}
	keepItem := components.SelectItem{
		Label:  "Keep conflicts in progress — leave the rebase in its worktree for manual resolution",
		Value:  conflictModeKeep,
		Danger: true,
	}

	items := []components.SelectItem{normalItem, keepItem}
	if defaultKeep {
		items = []components.SelectItem{keepItem, normalItem}
	}

	return components.NewSelectList(components.NewSelectListParams{
		Title:       "On conflict",
		Description: desc,
		Items:       items,
	})
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

// label decorates a worktree branch with a short suffix so the user can tell the
// base apart and spot worktrees that sync will skip.
func label(status domain.WorktreeStatus) string {
	switch {
	case status.IsParent:
		return status.Branch + " (base)"
	case status.RebaseInProgress:
		return status.Branch + " (rebasing)"
	case status.IsDirty:
		return status.Branch + " (dirty)"
	default:
		return status.Branch
	}
}
