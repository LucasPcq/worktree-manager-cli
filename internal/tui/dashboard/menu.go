package dashboard

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

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
	item := menuItem{label: domain.DashboardMenuDelete, action: menuDelete}
	if selected, ok := m.selected(); ok {
		if reason, busy := m.busyReason(selected.Branch); busy {
			item.disabled = reason
		}
	}
	return []menuItem{item}
}

// openMenu hangs the menu off a cell. The right button is not always delivered —
// some terminals keep it for their own paste — so KeyMenu opens the very same
// menu on the selected row.
func (m Model) openMenu(anchor domain.Rect) Model {
	if _, ok := m.selected(); !ok {
		return m
	}
	m.menuOpen, m.menuCursor, m.menuAnchor = true, 0, anchor
	return m
}

// selectedRowPoint is the last line of the selected row, so the keyboard opens
// the menu where the mouse would have — under the row, not over its own lines.
func (m Model) selectedRowPoint() domain.Rect {
	layout := m.layout()
	top := layout.List.Y + domain.DashboardChromeHeight - 1 + domain.DashboardTitleGap
	stride := domain.DashboardRowHeight + domain.DashboardRowGap
	return domain.Rect{
		X: layout.List.X + borderWidth,
		Y: top + (m.cursor-m.offset)*stride + domain.DashboardRowHeight - 1,
	}
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
		return m.refuse(item.disabled), nil
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

// menuBox draws the context menu as a floating box: it is anchored on the cell it
// was opened from, and overlay() pastes it over the frame.
func (m Model) menuBox() (string, domain.Rect) {
	selected, ok := m.selected()
	if !ok {
		return "", domain.Rect{}
	}

	items := m.menuItems()
	inner := menuInnerWidth(menuInnerWidthParams{Items: items, Title: selected.Branch, Screen: m.width})
	if inner <= 0 {
		return "", domain.Rect{}
	}

	lines := make([]string, 0, len(items)+1)
	lines = append(lines, styles.DashboardMenuTitle.Render(pad(truncate(selected.Branch, inner), inner)))
	for index, item := range items {
		lines = append(lines, m.zones.Mark(menuZone(index), styleMenuRow(menuRowParams{
			Text:     pad(truncate(menuLabel(item), inner), inner),
			Focused:  index == m.menuCursor,
			Disabled: item.disabled != "",
		})))
	}

	box := styles.DashboardMenu.Render(strings.Join(lines, "\n"))
	rect := rules.ComputeMenuRect(rules.MenuRectParams{
		AnchorX:      m.menuAnchor.X,
		AnchorY:      m.menuAnchor.Y,
		Width:        lipgloss.Width(box),
		Height:       lipgloss.Height(box),
		ScreenWidth:  m.width,
		ScreenHeight: m.height,
	})
	return box, rect
}

type menuInnerWidthParams struct {
	Items  []menuItem
	Title  string
	Screen int
}

// menuInnerWidth sizes the menu on its longest line, then keeps it inside the
// screen: the box is pasted whole, so it must never need trimming.
func menuInnerWidth(params menuInnerWidthParams) int {
	inner := lipgloss.Width(params.Title)
	for _, item := range params.Items {
		inner = max(inner, lipgloss.Width(menuLabel(item)))
	}
	return min(inner, params.Screen-domain.DashboardMenuChrome)
}

func menuLabel(item menuItem) string {
	if item.disabled != "" {
		return item.label + domain.DashboardMenuDisabledMark
	}
	return item.label
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
