package clean

import (
	"fmt"
	"sort"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/infra"
	"github.com/LucasPcq/wtm/internal/tui/components"
)

// RunWorktreePicker displays a filterable picker of worktrees (excluding parent).
// Returns ErrUserAborted on Ctrl+C.
func RunWorktreePicker(projectDir string) (string, error) {
	worktrees, err := infra.ListWorktrees(infra.ListWorktreesParams{ProjectDir: projectDir})
	if err != nil {
		return "", fmt.Errorf("list worktrees: %w", err)
	}

	var items []components.SelectItem
	for _, wt := range worktrees {
		if wt.IsMain {
			continue
		}
		items = append(items, components.SelectItem{Label: wt.Branch, Value: wt.Branch})
	}

	sort.Slice(items, func(i, j int) bool {
		return items[i].Value < items[j].Value
	})

	if len(items) == 0 {
		return "", fmt.Errorf("no worktrees to clean (only the parent worktree exists)")
	}

	sl := components.NewSelectList(components.NewSelectListParams{
		Title:       "Select worktree to clean",
		Description: "The parent worktree cannot be cleaned",
		Items:       items,
	})

	result, err := components.RunStandaloneSelect(sl)
	if err != nil {
		return "", domain.ErrUserAborted
	}
	return result, nil
}
