package clean

import (
	"errors"
	"fmt"
	"sort"

	"github.com/charmbracelet/huh"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/infra"
)

// RunWorktreePicker displays a filterable picker of worktrees (excluding parent).
// Returns ErrUserAborted on Ctrl+C.
func RunWorktreePicker(projectDir string) (string, error) {
	worktrees, err := infra.ListWorktrees(infra.ListWorktreesParams{ProjectDir: projectDir})
	if err != nil {
		return "", fmt.Errorf("list worktrees: %w", err)
	}

	var options []huh.Option[string]
	for _, wt := range worktrees {
		if wt.IsMain {
			continue
		}
		options = append(options, huh.NewOption(wt.Branch, wt.Branch))
	}

	sort.Slice(options, func(i, j int) bool {
		return options[i].Value < options[j].Value
	})

	if len(options) == 0 {
		return "", fmt.Errorf("no worktrees to clean (only the parent worktree exists)")
	}

	var selected string

	form := huh.NewForm(
		huh.NewGroup(
			huh.NewSelect[string]().
				Title("Select worktree to clean").
				Description("The parent worktree cannot be cleaned").
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
