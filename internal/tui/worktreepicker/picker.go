// Package worktreepicker renders a rich worktree selector with the same
// styling (breadcrumb, badges, spacing) as the interactive `wt list` wizard.
package worktreepicker

import (
	"fmt"
	"os"
	"strconv"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/service/process"
	"github.com/LucasPcq/wtm/internal/styles"
	"github.com/LucasPcq/wtm/internal/tui/components"
)

// RunParams holds the inputs required to render the picker.
type RunParams struct {
	Statuses     []domain.WorktreeStatus
	ActiveBranch string
	PRs          []domain.PRInfo
	Services     []process.JobInfo
	Title        string
}

// Run shows the picker and returns the selected worktree.
// Returns domain.ErrUserAborted when the user presses Esc.
func Run(params RunParams) (domain.WorktreeStatus, error) {
	if params.Title == "" {
		params.Title = "Select a worktree"
	}

	// This picker is commonly invoked from `wtm resolve` through a shell
	// wrapper that captures stdout. Force lipgloss to detect color support
	// against stderr (the TTY) so selected rows keep their accent styling.
	styles.UseRendererOn(os.Stderr)

	items := make([]components.SelectItem, 0, len(params.Statuses))
	for i, s := range params.Statuses {
		items = append(items, components.SelectItem{
			Label:  s.Branch,
			Value:  strconv.Itoa(i),
			Badges: BuildBadges(s, params.PRs, params.Services),
		})
	}

	wiz := components.NewWizard([]components.Step{
		{
			Name:  params.Title,
			Model: components.NewSelectList(components.NewSelectListParams{Title: params.Title, Items: items}),
		},
	})

	finalModel, err := tea.NewProgram(wiz, tea.WithOutput(os.Stderr)).Run()
	if err != nil {
		return domain.WorktreeStatus{}, fmt.Errorf("worktree picker: %w", err)
	}

	final, ok := finalModel.(components.WizardModel)
	if !ok || final.Aborted() {
		return domain.WorktreeStatus{}, domain.ErrUserAborted
	}

	sl, ok := final.Steps()[0].Model.(components.SelectListModel)
	if !ok {
		return domain.WorktreeStatus{}, fmt.Errorf("unexpected model type")
	}

	idx, err := strconv.Atoi(sl.Value())
	if err != nil {
		return domain.WorktreeStatus{}, fmt.Errorf("parse worktree index: %w", err)
	}
	if idx < 0 || idx >= len(params.Statuses) {
		return domain.WorktreeStatus{}, fmt.Errorf("worktree index out of range")
	}

	return params.Statuses[idx], nil
}

// BuildBadges returns the styled badges for a worktree row (parent, PR,
// services, dirty/clean) used by both `wt list` and the switch picker.
func BuildBadges(s domain.WorktreeStatus, prs []domain.PRInfo, services []process.JobInfo) []components.Badge {
	var badges []components.Badge
	if s.IsParent {
		badges = append(badges, components.Badge{Text: "parent", Variant: components.BadgeNeutral})
	}
	for _, pr := range prs {
		if pr.Branch == s.Branch {
			badges = append(badges, components.Badge{Text: fmt.Sprintf("PR #%d", pr.Number), Variant: components.BadgeSuccess})
			break
		}
	}
	for _, svc := range services {
		if svc.WorkDir == s.Path && svc.Status == domain.JobStatusRunning {
			badges = append(badges, components.Badge{Text: "services", Variant: components.BadgeSuccess})
			break
		}
	}
	if s.IsDirty {
		badges = append(badges, components.Badge{Text: "dirty", Variant: components.BadgeWarning})
	} else {
		badges = append(badges, components.Badge{Text: "clean", Variant: components.BadgeNeutral})
	}
	return badges
}
