// Package clean builds the interactive worktree picker and confirm wizard for the
// wtm clean command.
package clean

import (
	"fmt"
	"sort"

	"github.com/LucasPcq/wtm/internal/domain"
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

// RunWorktreePicker displays a filterable picker of worktrees (excluding parent).
// Used by the non-wizard paths (--force / --yes); the full interactive path hosts
// the picker inside RunWizard instead. Returns ErrUserAborted on Esc.
func RunWorktreePicker(projectDir string) (string, error) {
	items, err := pickableItems(projectDir)
	if err != nil {
		return "", err
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
