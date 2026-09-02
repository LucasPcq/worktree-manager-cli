package runpicker

import (
	"errors"
	"fmt"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/tui/components"
)

type WorktreePickerParams struct {
	Worktrees []domain.GitWorktree
	// Current is the path the cursor opens on — the worktree the command was
	// launched from, so validating with Enter reproduces the old cwd behavior.
	Current string
	// Running counts the jobs up in each worktree, keyed by path.
	Running map[string]int
}

// RunWorktreePicker asks which worktree a run command acts on. It is only ever
// reached from a fully interactive run: every other surface names the worktree
// or falls back to the current directory.
func RunWorktreePicker(params WorktreePickerParams) (domain.GitWorktree, error) {
	items := make([]components.SelectItem, 0, len(params.Worktrees))
	for _, wt := range params.Worktrees {
		var badges []components.Badge
		if count := params.Running[wt.Path]; count > 0 {
			badges = append(badges, components.Badge{
				Text:    fmt.Sprintf(domain.RunWorktreeJobsFmt, count),
				Variant: components.BadgeSuccess,
			})
		}
		if wt.Path == params.Current {
			badges = append(badges, components.Badge{Text: domain.RunWorktreeCurrent, Variant: components.BadgeNeutral})
		}
		items = append(items, components.SelectItem{
			Label:  wt.Branch,
			Value:  wt.Path,
			Badges: badges,
		})
	}

	path, err := components.RunStandaloneSelect(components.NewSelectList(components.NewSelectListParams{
		Title:       domain.RunWorktreePickerTitle,
		Description: domain.RunWorktreePickerDesc,
		Items:       items,
		Start:       params.Current,
	}))
	if err != nil {
		if errors.Is(err, components.ErrAborted) {
			return domain.GitWorktree{}, domain.ErrUserAborted
		}
		return domain.GitWorktree{}, err
	}

	for _, wt := range params.Worktrees {
		if wt.Path == path {
			return wt, nil
		}
	}
	return domain.GitWorktree{}, fmt.Errorf("picked worktree %q is not in the list", path)
}
