package dashboard

import (
	"fmt"
	"strings"
	"time"

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
	// panelChromeRows is what renderPanel prepends to Body before drawing it:
	// the title row and the blank row under it.
	panelChromeRows = 2
)

// panelBodyHeight is the row budget renderPanel leaves for Body once its own
// border and chrome rows are accounted for.
func panelBodyHeight(rect domain.Rect) int {
	return max(rect.Height-borderWidth-panelChromeRows, 0)
}

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

// renderHeader is the dashboard's top bar: the context line (where you are —
// wordmark included) on top, then the tabs and the count of what is listed,
// then the rule underlining the active tab. The wordmark appears once, on the
// context line; the bar below starts directly with the tabs. No rule sits
// between the first two lines — the tab rule underneath already separates the
// header from the body.
//
// Whole tabs are dropped rather than the bar trimmed: a hard trim would cut
// through a zone marker and break that tab's hit-testing.
func (m Model) renderHeader(layout domain.DashboardLayout) string {
	// A terminal too short for the full 3-line header is degenerate — nothing
	// else fits either — but it must not overflow. ComputeDashboardLayout
	// already shrinks Tabs.Height to what actually fits; the renderer honors
	// it by dropping whole lines from the bottom (rule, then bar) rather than
	// emitting a fixed line count regardless of the budget, mirroring how
	// renderPanel returns "" rather than drawing past its own Rect.
	if layout.Tabs.Height <= 0 {
		return ""
	}
	context := m.renderContextLine(layout.Tabs.Width)
	if layout.Tabs.Height == 1 {
		return context
	}

	// The wordmark already opens the context line above; the bar starts
	// directly with the tabs so it is not drawn twice.
	rendered := make([]string, 0, len(tabs))
	used, activeStart, activeWidth := 0, 0, 0
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
	if layout.Tabs.Height == 2 {
		return lipgloss.JoinVertical(lipgloss.Left, context, bar)
	}

	return lipgloss.JoinVertical(lipgloss.Left, context, bar, tabRule(tabRuleParams{
		Width:       layout.Tabs.Width,
		ActiveStart: activeStart,
		ActiveWidth: activeWidth,
	}))
}

// renderContextLine is the header's "where you are" line: the wordmark, the
// repo, the base branch and the worktree the shell is currently in, with the
// fetch-staleness notice flush right when the origin refs have gone stale.
//
// Segments drop whole, right to left — fetched, then the active worktree,
// then base, then the repo — the same variant-list mechanic headerRight uses
// for its own buttons: a half-drawn label reads as a wrong one, so a segment
// that does not fit is dropped rather than cut.
func (m Model) renderContextLine(width int) string {
	wordmark := styles.DashboardWordmark.Render(domain.DashboardWordmark)
	fetched := m.fetchedLabel()

	for _, variant := range []struct{ repo, base, active, fetched bool }{
		{true, true, true, true},
		{true, true, true, false},
		{true, true, false, false},
		{true, false, false, false},
		{false, false, false, false},
	} {
		left := wordmark + m.contextLeft(contextLeftParams{Repo: variant.repo, Base: variant.base, Active: variant.active})
		right := ""
		if variant.fetched {
			right = fetched
		}
		if line, ok := fitContextLine(left, right, width); ok {
			return line
		}
	}
	// The last variant above already tried the bare wordmark alone; if that
	// did not fit either, nothing will — the line goes empty rather than
	// overflow the width it was given.
	return ""
}

type contextLeftParams struct {
	Repo   bool
	Base   bool
	Active bool
}

// contextLeft builds the segments after the wordmark, joined by
// DashboardContextSep. A segment with no data to show (no repo name resolved,
// no base configured, cwd outside every known worktree) is skipped rather than
// printed empty.
func (m Model) contextLeft(params contextLeftParams) string {
	segments := make([]string, 0, 3)
	if params.Repo && m.repoName != "" {
		segments = append(segments, m.repoName)
	}
	if params.Base && m.baseBranch != "" {
		segments = append(segments, fmt.Sprintf(domain.DashboardBaseFmt, m.baseBranch))
	}
	if params.Active && m.activeBranch != "" {
		segments = append(segments, domain.DashboardActiveGlyph+" "+m.activeBranch)
	}
	if len(segments) == 0 {
		return ""
	}
	return styles.DashboardContext.Render(" " + strings.Join(segments, domain.DashboardContextSep))
}

// fetchedLabel is the header's only non-permanent element: it appears only
// once the origin refs are stale enough to matter, and it is the view's
// property, not the selected worktree's — every origin badge in the list is
// equally old.
func (m Model) fetchedLabel() string {
	now := time.Now()
	if !rules.FetchIsStale(rules.FetchStalenessParams{FetchedAt: m.fetchedAt, Now: now}) {
		return ""
	}
	if m.fetchedAt.IsZero() {
		return styles.DashboardContext.Render(domain.DashboardNeverFetched)
	}
	age := rules.RelativeAge(rules.RelativeAgeParams{At: m.fetchedAt, Now: now})
	return styles.DashboardContext.Render(fmt.Sprintf(domain.DashboardFetchedFmt, age))
}

// fitContextLine lays the left cluster and the right notice on one row of the
// given width, keeping both whole. It reports whether they fit at all rather
// than clipping either one.
func fitContextLine(left, right string, width int) (string, bool) {
	if right == "" {
		if lipgloss.Width(left) <= width {
			return left, true
		}
		return "", false
	}
	if lipgloss.Width(left)+1+lipgloss.Width(right) > width {
		return "", false
	}
	gap := width - lipgloss.Width(left) - lipgloss.Width(right)
	return left + strings.Repeat(" ", gap) + right, true
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
	if layout.Help.Height <= 0 {
		return ""
	}
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
