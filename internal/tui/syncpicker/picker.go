// Package syncpicker renders a multi-select worktree picker for `wtm sync`,
// letting the user choose which worktrees to rebase (or the base, to refresh it).
package syncpicker

import (
	"errors"
	"os"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/styles"
	"github.com/LucasPcq/wtm/internal/tui/components"
)

// RunParams holds the inputs required to render the picker.
type RunParams struct {
	Statuses []domain.WorktreeStatus
}

// Run shows the multi-select picker and returns the branches the user checked.
// Returns domain.ErrUserAborted when the user presses Esc.
func Run(params RunParams) ([]string, error) {
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
		Steps:    []components.Step{{Name: "Worktrees", Model: ms}},
		Stderr:   true,
		ErrLabel: "sync picker",
	})
	if err != nil {
		return nil, err
	}

	step, ok := final.Steps()[0].Model.(components.MultiSelectModel)
	if !ok {
		return nil, errors.New("unexpected model type")
	}
	return step.Values(), nil
}

// label decorates a worktree branch with a short suffix so the user can tell the
// base apart and spot worktrees that sync will skip.
func label(status domain.WorktreeStatus) string {
	switch {
	case status.IsParent:
		return status.Branch + " (base)"
	case status.IsDirty:
		return status.Branch + " (dirty)"
	default:
		return status.Branch
	}
}
