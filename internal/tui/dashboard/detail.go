package dashboard

import (
	"fmt"
	"strings"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/styles"
)

type detailModel struct {
	selected  *domain.WorktreeStatus
	lastError string
}

func (m detailModel) view() string {
	var sections []string

	if m.lastError != "" {
		sections = append(sections,
			styles.Warning.Render("  Last error:")+"\n"+
				styles.Muted.Render("  "+m.lastError),
		)
	}

	if m.selected == nil {
		if len(sections) == 0 {
			return styles.NormalItem.Render("  Select a worktree to view details")
		}
		return strings.Join(sections, "\n\n")
	}

	s := m.selected
	info := fmt.Sprintf(
		"  %s\n\n  %s %s\n  %s %s",
		styles.Bold.Render(s.Branch),
		styles.Muted.Render("Path:"),
		s.Path,
		styles.Muted.Render("Status:"),
		formatDetailStatus(s),
	)
	sections = append([]string{info}, sections...)

	return strings.Join(sections, "\n\n")
}

func formatDetailStatus(s *domain.WorktreeStatus) string {
	status := "clean"
	if s.IsDirty {
		status = "dirty"
	}
	if s.CommitsAhead > 0 {
		return fmt.Sprintf("%s, %d commits ahead", status, s.CommitsAhead)
	}
	return status
}
