// Package gopicker builds interactive pickers for the wtm go/resolve commands.
package gopicker

import (
	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/infra"
	"github.com/LucasPcq/wtm/internal/tui/components"
)

// RunWorktreePicker displays a filterable picker of worktrees.
// Returns the absolute path of the selected worktree, or ErrUserAborted on Ctrl+C.
func RunWorktreePicker(worktrees []infra.GitWorktree) (string, error) {
	items := make([]components.SelectItem, 0, len(worktrees))
	for _, wt := range worktrees {
		label := wt.Branch
		if wt.IsMain {
			label += " (parent)"
		}
		items = append(items, components.SelectItem{Label: label, Value: wt.Path})
	}

	sl := components.NewSelectList(components.NewSelectListParams{
		Title:       "Select worktree",
		Description: "",
		Items:       items,
	})

	result, err := components.RunStandaloneSelect(sl)
	if err != nil {
		return "", domain.ErrUserAborted
	}
	return result, nil
}
