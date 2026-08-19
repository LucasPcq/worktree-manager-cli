package dashboard

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/rules"
	"github.com/LucasPcq/wtm/internal/styles"
)

// borderWidth and paddingWidth are what lipgloss adds around a panel's text; the
// rect a panel is given must account for both. buttonPadding is what the button
// style adds around its label.
const (
	borderWidth   = 2
	paddingWidth  = 2
	buttonPadding = 4
)

type panelParams struct {
	Rect  domain.Rect
	Title string
	// TitleZone, when set, makes the whole title row its own clickable zone.
	TitleZone string
	// TitleRight is rendered flush right on the title row, under its own zone. It
	// is dropped whole when the panel is too narrow for it: trimming it would cut
	// through its marker and take the zone with it.
	TitleRight     string
	TitleRightZone string
	Body           []string
	Zone           string
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
	if rightWidth := lipgloss.Width(params.TitleRight); params.TitleRight != "" && rightWidth+1 < textWidth {
		left := styles.DashboardPanelTitle.Render(truncate(params.Title, textWidth-rightWidth-1))
		gap := textWidth - lipgloss.Width(left) - rightWidth
		title = left + strings.Repeat(" ", max(gap, 0)) + m.marks().Mark(params.TitleRightZone, params.TitleRight)
	}
	if params.TitleZone != "" {
		title = m.marks().Mark(params.TitleZone, title)
	}

	lines := append([]string{title, ""}, params.Body...)
	if len(lines) > contentHeight {
		lines = lines[:contentHeight]
	}

	box := styles.DashboardPanel.
		Width(params.Rect.Width - borderWidth).
		Height(contentHeight).
		Render(strings.Join(lines, "\n"))

	return m.marks().Mark(params.Zone, box)
}

// renderHeader is the dashboard's top bar: the wordmark, the tabs, and the count
// of what is listed. The active tab is named by weight and by the rule under it.
//
// Whole tabs are dropped rather than the bar trimmed: a hard trim would cut
// through a zone marker and break that tab's hit-testing.
func (m Model) renderHeader(layout domain.DashboardLayout) string {
	wordmark := styles.DashboardWordmark.Render(domain.DashboardWordmark)
	used := lipgloss.Width(wordmark)

	rendered := []string{wordmark}
	activeStart, activeWidth := 0, 0
	for index, title := range tabs {
		style := styles.DashboardTabInactive
		if index == m.tab {
			style = styles.DashboardTabActive
		}
		tab := style.Render(title)
		if used+lipgloss.Width(tab) > layout.Tabs.Width {
			break
		}
		if index == m.tab {
			activeStart, activeWidth = used, lipgloss.Width(tab)
		}
		used += lipgloss.Width(tab)
		rendered = append(rendered, m.marks().Mark(tabZone(index), tab))
	}

	bar := lipgloss.JoinHorizontal(lipgloss.Top, rendered...)
	if right := m.headerRight(layout.Tabs.Width - used); right != "" {
		bar += strings.Repeat(" ", layout.Tabs.Width-used-lipgloss.Width(right)) + right
	}

	return lipgloss.JoinVertical(lipgloss.Left, bar, tabRule(tabRuleParams{
		Width:       layout.Tabs.Width,
		ActiveStart: activeStart,
		ActiveWidth: activeWidth,
	}))
}

// headerRight is the bar's right cluster: the two global actions, then the count
// of what the active tab lists. Whole segments are dropped rather than the bar
// trimmed — a hard trim would cut through a zone marker and take that button's
// hit-testing with it.
func (m Model) headerRight(room int) string {
	count := m.countLabel()
	for _, variant := range []struct{ add, actions, count string }{
		{domain.DashboardAddLabelLong, domain.DashboardActionsLabel, count},
		{domain.DashboardAddLabel, domain.DashboardActionsLabel, count},
		{domain.DashboardAddLabel, domain.DashboardActionsShort, count},
		{domain.DashboardAddLabel, domain.DashboardActionsShort, ""},
		{"", domain.DashboardActionsShort, ""},
	} {
		add, actions := "", ""
		if variant.add != "" {
			add = m.marks().Mark(zoneAdd, styles.DashboardAddButton.Render(variant.add))
		}
		if variant.actions != "" {
			actions = m.marks().Mark(zoneActions, styles.DashboardHeaderButton.Render(variant.actions))
		}
		cluster := joinHeader(add, actions, variant.count)
		if lipgloss.Width(cluster)+1 <= room {
			return cluster
		}
	}
	return ""
}

func joinHeader(segments ...string) string {
	parts := make([]string, 0, len(segments))
	for _, segment := range segments {
		if segment != "" {
			parts = append(parts, segment)
		}
	}
	return strings.Join(parts, " ")
}

// countLabel counts what the active tab lists: worktrees, or the nodes of the
// forest — which includes the parents that have none.
func (m Model) countLabel() string {
	if m.tab == tabTree {
		if !m.treeLoaded {
			return ""
		}
		format := domain.DashboardTreeCountFmt
		if len(m.treeRows) == 1 {
			format = domain.DashboardTreeCountOneFmt
		}
		return styles.DashboardCount.Render(fmt.Sprintf(format, len(m.treeRows)))
	}
	if !m.loaded {
		return ""
	}
	format := domain.DashboardCountFmt
	if len(m.statuses) == 1 {
		format = domain.DashboardCountOneFmt
	}
	return styles.DashboardCount.Render(fmt.Sprintf(format, len(m.statuses)))
}

type tabRuleParams struct {
	Width       int
	ActiveStart int
	ActiveWidth int
}

// tabRule underlines the active tab and carries a quieter rule across the rest,
// which is what separates the header from the panels below it.
func tabRule(params tabRuleParams) string {
	if params.Width <= 0 {
		return ""
	}
	if params.ActiveWidth <= 0 {
		return styles.DashboardRule.Render(strings.Repeat(domain.DashboardRuleGlyph, params.Width))
	}

	before := styles.DashboardRule.Render(strings.Repeat(domain.DashboardRuleGlyph, params.ActiveStart))
	under := styles.DashboardTabRule.Render(strings.Repeat(domain.DashboardActiveRuleGlyph, params.ActiveWidth))
	rest := max(params.Width-params.ActiveStart-params.ActiveWidth, 0)
	return before + under + styles.DashboardRule.Render(strings.Repeat(domain.DashboardRuleGlyph, rest))
}

func (m Model) renderHelpBar(layout domain.DashboardLayout) string {
	hint := domain.DashboardHelpWide
	switch {
	case m.loadErr != nil:
		hint = m.loadErr.Error()
	case m.tab == tabTree:
		hint = domain.DashboardHelpTree
	case layout.Narrow && m.detailOpen:
		hint = domain.DashboardHelpDetail
	case layout.Narrow:
		hint = domain.DashboardHelpNarrow
	}
	return styles.DashboardHelp.Render(truncate(hint, max(layout.Help.Width-paddingWidth, 0)))
}

// helpBox is the key and mouse reference: every clickable zone paired with the
// key that does the same thing. Like the other overlays it is a box pasted over
// the frame, sized on its own content.
func (m Model) helpBox() (string, domain.Rect) {
	rows := [][2]string{
		{"↑↓ · j k", "select a worktree (or click a row)"},
		{"g · G", "first · last worktree"},
		{"pgup · pgdown", "page through the list"},
		{"wheel", "scroll the list or the output panel"},
		{"n", "new worktree (or click + new)"},
		{"m", "actions on the selected worktree (or right-click a row)"},
		{"a", "actions on several worktrees (or click ⋯ actions)"},
		{"tab · shift+tab", "switch view (or click a tab)"},
		{"enter · →", "open the detail (narrow terminals)"},
		{"esc · ←", "close the detail"},
		{"o", "fold/unfold the output panel (or click its header)"},
		{"shift+↑ · shift+↓", "scroll the output panel"},
		{"r", "refresh worktrees and pull requests"},
		{"?", "close this help"},
		{"q · ctrl+c", "quit"},
	}

	textWidth := helpTextWidth(rows, m.width)
	if textWidth <= 0 {
		return "", domain.Rect{}
	}

	lines := make([]string, 0, len(rows)+2)
	lines = append(lines, styles.DashboardModalTitle.Render(truncate(domain.DashboardHelpTitle, textWidth)), "")
	for _, row := range rows {
		lines = append(lines, truncate(styles.DashboardLabel.Render(pad(row[0], helpKeyWidth))+
			styles.DashboardValue.Render(row[1]), textWidth))
	}

	box := styles.DashboardModal.Width(textWidth + modalPadding).Render(strings.Join(lines, "\n"))
	return box, rules.CenterRect(rules.CenterRectParams{
		Width:        lipgloss.Width(box),
		Height:       lipgloss.Height(box),
		ScreenWidth:  m.width,
		ScreenHeight: m.height,
	})
}

// helpKeyWidth is the column the descriptions line up on.
const helpKeyWidth = 18

// helpTextWidth sizes the box on its longest row, then keeps it inside the
// screen: an overlay is pasted whole, so it must never need trimming.
func helpTextWidth(rows [][2]string, screenWidth int) int {
	widest := lipgloss.Width(domain.DashboardHelpTitle)
	for _, row := range rows {
		widest = max(widest, helpKeyWidth+lipgloss.Width(row[1]))
	}
	return min(widest, screenWidth-domain.DashboardModalChrome-modalPadding)
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
