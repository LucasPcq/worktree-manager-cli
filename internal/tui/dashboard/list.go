package dashboard

import (
	"strings"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/styles"
	"github.com/LucasPcq/wtm/internal/tui/worktreepicker"
)

const (
	rowMarkerSelected = "▌▸ "
	rowMarkerPlain    = "   "
)

func (m Model) renderList(layout domain.DashboardLayout) string {
	return m.renderPanel(panelParams{
		Rect:           layout.List,
		Title:          domain.DashboardListTitle,
		TitleRight:     styles.DashboardAddButton.Render(domain.DashboardAddLabel),
		TitleRightZone: zoneAdd,
		Body:           m.listBody(layout),
		Zone:           zoneList,
	})
}

func (m Model) listBody(layout domain.DashboardLayout) []string {
	width := layout.List.Width - borderWidth - paddingWidth
	if !m.loaded {
		return []string{styles.DashboardEmpty.Render(truncate(domain.LoadingWorktreesText, width))}
	}
	if len(m.statuses) == 0 {
		return []string{styles.DashboardEmpty.Render(truncate(domain.DashboardEmptyList, width))}
	}

	end := min(m.offset+layout.ListRows, len(m.statuses))
	rows := make([]string, 0, max(end-m.offset, 0))
	for index := m.offset; index < end; index++ {
		rows = append(rows, m.zones.Mark(rowZone(index), m.renderRow(index, width)))
		if m.menuOpen && index == m.cursor {
			rows = append(rows, m.menuRows(width)...)
		}
	}
	return rows
}

// renderRow lays the branch and its tags against a right-aligned state pill. The
// selection is a colored marker rather than a filled background, so the tag
// colors survive.
func (m Model) renderRow(index, width int) string {
	status := m.statuses[index]

	marker := styles.Muted.Render(rowMarkerPlain)
	branch := styles.DashboardValue.Render(status.Branch)
	if index == m.cursor {
		marker = styles.SelectedMarker.Render(rowMarkerSelected)
		branch = styles.Bold.Render(status.Branch)
	}

	tags := make([]string, 0, 4)
	for _, tag := range worktreepicker.BuildTags(worktreepicker.BuildTagsParams{Status: status, PRs: m.prs}) {
		tags = append(tags, tag.Render())
	}

	left := marker + branch
	if len(tags) > 0 {
		left += "  " + strings.Join(tags, " ")
	}
	return spread(left, worktreepicker.BuildStatus(status).Render(), width)
}
