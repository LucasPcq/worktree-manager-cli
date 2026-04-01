// Package gopicker builds interactive huh forms for the wtm go/resolve commands.
package gopicker

import (
	"errors"

	"github.com/charmbracelet/huh"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/infra"
)

// RunWorktreePicker displays a filterable picker of worktrees.
// Returns the absolute path of the selected worktree, or ErrUserAborted on Ctrl+C.
func RunWorktreePicker(worktrees []infra.GitWorktree) (string, error) {
	options := make([]huh.Option[string], 0, len(worktrees))
	for _, wt := range worktrees {
		label := wt.Branch
		if wt.IsMain {
			label += " (parent)"
		}
		options = append(options, huh.NewOption(label, wt.Path))
	}

	var selected string

	form := huh.NewForm(
		huh.NewGroup(
			huh.NewSelect[string]().
				Title("Select worktree").
				Options(options...).
				Filtering(true).
				Value(&selected),
		),
	)

	if err := form.Run(); err != nil {
		if errors.Is(err, huh.ErrUserAborted) {
			return "", domain.ErrUserAborted
		}
		return "", err
	}

	return selected, nil
}
