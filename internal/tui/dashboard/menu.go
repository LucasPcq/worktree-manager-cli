package dashboard

import (
	tea "github.com/charmbracelet/bubbletea"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/rules"
	"github.com/LucasPcq/wtm/internal/styles"
)

// menuAction is what a context menu entry does. One entry today; the list is
// what a second one plugs into.
type menuAction int

const (
	menuDelete menuAction = iota
)

type menuItem struct {
	label  string
	action menuAction
	// disabled states why the entry cannot be used right now, and shows it.
	disabled string
}

func (m Model) menuItems() []menuItem {
	return []menuItem{{label: domain.DashboardMenuDelete, action: menuDelete}}
}

// openMenu anchors the menu on a worktree row. The right button is not always
// delivered — some terminals keep it for their own paste — so KeyMenu opens the
// very same menu on the selected row.
func (m Model) openMenu() Model {
	if _, ok := m.selected(); !ok {
		return m
	}
	m.menuOpen, m.menuCursor = true, 0
	return m
}

func (m Model) closeMenu() Model {
	m.menuOpen, m.menuCursor = false, 0
	return m
}

func (m Model) moveMenu(delta int) Model {
	m.menuCursor = rules.ClampIndex(m.menuCursor+delta, len(m.menuItems()))
	return m
}

func (m Model) activateMenu(index int) (Model, tea.Cmd) {
	items := m.menuItems()
	if index < 0 || index >= len(items) {
		return m, nil
	}
	item := items[index]
	m = m.closeMenu()
	if item.disabled != "" {
		return m.appendOutput(OutputLineMsg{Text: item.disabled}), nil
	}

	selected, ok := m.selected()
	if !ok {
		return m, nil
	}
	switch item.action {
	case menuDelete:
		return m.startClean(selected.Branch)
	}
	return m, nil
}

// menuRows draws the menu as a dropdown hanging off its row, inside the list
// panel: an overlay would have to cut through the rows it covers, and every zone
// marked in them with it.
func (m Model) menuRows(width int) []string {
	items := m.menuItems()
	rows := make([]string, 0, len(items))
	for index, item := range items {
		label := menuBranchGlyph(index, len(items)) + item.label
		if item.disabled != "" {
			label += domain.DashboardMenuDisabledMark
		}
		rows = append(rows, m.zones.Mark(menuZone(index), styleMenuRow(menuRowParams{
			Text:     truncate(label, width),
			Focused:  index == m.menuCursor,
			Disabled: item.disabled != "",
		})))
	}
	return rows
}

type menuRowParams struct {
	Text     string
	Focused  bool
	Disabled bool
}

func styleMenuRow(params menuRowParams) string {
	switch {
	case params.Disabled:
		return styles.DashboardDisabled.Render(params.Text)
	case params.Focused:
		return styles.DashboardRowFocused.Render(params.Text)
	}
	return styles.DashboardRow.Render(params.Text)
}

func menuBranchGlyph(index, total int) string {
	if index == total-1 {
		return domain.DashboardMenuLastGlyph
	}
	return domain.DashboardMenuGlyph
}
