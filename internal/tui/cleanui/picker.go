// Package cleanui builds the interactive worktree picker and confirm wizard for the
// wtm clean command.
package cleanui

import (
	"fmt"
	"sort"

	"github.com/LucasPcq/wtm/internal/infra"
	"github.com/LucasPcq/wtm/internal/tui/components"
)

// pickableItems lists the worktrees that can be cleaned (every worktree but the
// main one), sorted by branch. Returns an error when none exist.
func pickableItems(projectDir string) ([]components.SelectItem, error) {
	worktrees, err := infra.ListWorktrees(infra.ListWorktreesParams{ProjectDir: projectDir})
	if err != nil {
		return nil, fmt.Errorf("list worktrees: %w", err)
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
		return nil, fmt.Errorf("no worktrees to clean (only the parent worktree exists)")
	}
	return items, nil
}
