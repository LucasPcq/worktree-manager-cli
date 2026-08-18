package dashboard

import (
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/styles"
)

// borderWidth and paddingWidth are what lipgloss adds around a panel's text; the
// rect a panel is given must account for both.
const (
	borderWidth  = 2
	paddingWidth = 2
)

type panelParams struct {
	Rect  domain.Rect
	Title string
	// TitleZone, when set, makes the whole title row its own clickable zone.
	TitleZone string
	// TitleRight is rendered flush right on the title row. It carries its own zone
	// marking, so the title beside it must not be marked too.
	TitleRight string
	Body       []string
	Zone       string
}

// renderPanel draws a titled, bordered box filling Rect exactly and registers it
// as one mouse zone. Body is clipped to the rows the layout allotted.
func (m Model) renderPanel(params panelParams) string {
	textWidth := params.Rect.Width - borderWidth - paddingWidth
	contentHeight := params.Rect.Height - borderWidth
	if textWidth <= 0 || contentHeight <= 0 {
		return ""
	}

	title := styles.DashboardPanelTitle.Render(pad(truncate(params.Title, textWidth), textWidth))
	if params.TitleRight != "" {
		title = spread(styles.DashboardPanelTitle.Render(truncate(params.Title, textWidth)), params.TitleRight, textWidth)
	}
	if params.TitleZone != "" {
		title = m.zones.Mark(params.TitleZone, title)
	}

	lines := append([]string{title}, params.Body...)
	if len(lines) > contentHeight {
		lines = lines[:contentHeight]
	}

	box := styles.DashboardPanel.
		Width(params.Rect.Width - borderWidth).
		Height(contentHeight).
		Render(strings.Join(lines, "\n"))

	return m.zones.Mark(params.Zone, box)
}

// renderTabs drops whole tabs that do not fit rather than trimming the bar: a
// hard trim would cut through a zone marker and break the tab's hit-testing.
func (m Model) renderTabs(layout domain.DashboardLayout) string {
	rendered := make([]string, 0, len(tabs))
	used := 0
	for index, title := range tabs {
		style := styles.DashboardTabInactive
		if index == m.tab {
			style = styles.DashboardTabActive
		}
		tab := style.Render(title)
		if used+lipgloss.Width(tab) > layout.Tabs.Width {
			break
		}
		used += lipgloss.Width(tab)
		rendered = append(rendered, m.zones.Mark(tabZone(index), tab))
	}
	return lipgloss.JoinHorizontal(lipgloss.Top, rendered...)
}

func (m Model) renderHelpBar(layout domain.DashboardLayout) string {
	hint := domain.DashboardHelpWide
	switch {
	case m.loadErr != nil:
		hint = m.loadErr.Error()
	case layout.Narrow && m.detailOpen:
		hint = domain.DashboardHelpDetail
	case layout.Narrow:
		hint = domain.DashboardHelpNarrow
	}
	return styles.DashboardHelp.Render(truncate(hint, max(layout.Help.Width-paddingWidth, 0)))
}

// renderHelpOverlay replaces the frame while `?` is held open: every clickable
// zone paired with the key that does the same thing.
func (m Model) renderHelpOverlay() string {
	rows := [][2]string{
		{"↑↓ · j k", "select a worktree (or click a row)"},
		{"g · G", "first · last worktree"},
		{"pgup · pgdown", "page through the list"},
		{"wheel", "scroll the list or the output panel"},
		{"n", "new worktree (or click + new)"},
		{"m", "actions on the selected worktree (or right-click a row)"},
		{"tab · shift+tab", "switch view (or click a tab)"},
		{"enter · →", "open the detail (narrow terminals)"},
		{"esc · ←", "close the detail"},
		{"o", "fold/unfold the output panel (or click its header)"},
		{"shift+↑ · shift+↓", "scroll the output panel"},
		{"r", "refresh worktrees and pull requests"},
		{"?", "close this help"},
		{"q · ctrl+c", "quit"},
	}

	lines := make([]string, 0, len(rows)+2)
	lines = append(lines, styles.DashboardBranch.Render(domain.DashboardHelpTitle), "")
	for _, row := range rows {
		lines = append(lines, styles.DashboardLabel.Render(pad(row[0], 18))+styles.DashboardValue.Render(row[1]))
	}

	box := styles.DashboardPanel.Render(strings.Join(lines, "\n"))
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, box)
}

// truncate clips plain text to a display width, marking the cut with an ellipsis.
func truncate(text string, width int) string {
	if width <= 0 {
		return ""
	}
	if lipgloss.Width(text) <= width {
		return text
	}
	if width == 1 {
		return "…"
	}
	runes := []rune(text)
	for len(runes) > 0 && lipgloss.Width(string(runes))+1 > width {
		runes = runes[:len(runes)-1]
	}
	return string(runes) + "…"
}

// truncateRendered hard-trims styled text. It must never see marked content: a
// cut through a zone marker silently breaks that zone's bounds.
func truncateRendered(text string, width int) string {
	if width <= 0 || lipgloss.Width(text) <= width {
		return text
	}
	return styles.DashboardClip.MaxWidth(width).Render(text)
}

func pad(text string, width int) string {
	if gap := width - lipgloss.Width(text); gap > 0 {
		return text + strings.Repeat(" ", gap)
	}
	return text
}

// spread lays a left and a right segment on one row of the given width, keeping
// the right one whole and clipping the left when they collide.
func spread(left, right string, width int) string {
	rightWidth := lipgloss.Width(right)
	if rightWidth >= width {
		return truncateRendered(right, width)
	}
	left = truncateRendered(left, width-rightWidth-1)
	gap := width - lipgloss.Width(left) - rightWidth
	return left + strings.Repeat(" ", max(gap, 0)) + right
}
