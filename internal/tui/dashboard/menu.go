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
	// danger marks an entry that destroys something, so it reads as one before it
	// is activated.
	danger bool
	// disabled states why the entry cannot be used right now, and shows it.
	disabled string
}

// menuItems is what can be done to the selected worktree. An action that could
// never apply is not listed at all; one that cannot apply right now is listed
// with what is in its way, and is inert until that clears.
func (m Model) menuItems() []menuItem {
	selected, ok := m.selected()
	if !ok || selected.IsParent {
		return nil
	}

	item := menuItem{label: domain.DashboardMenuDelete, action: menuDelete, danger: true}
	if caption, busy := m.busyCaption(selected.Branch); busy {
		item.disabled = caption
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
	m.menuOpen, m.menuCursor, m.menuAnchor = true, firstEnabled(m.menuItems()), anchor
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

// moveMenu walks over what cannot be activated: a cursor parked on an inert
// entry is a keypress that does nothing.
func (m Model) moveMenu(delta int) Model {
	items := m.menuItems()
	for index := m.menuCursor + delta; index >= 0 && index < len(items); index += delta {
		if items[index].disabled == "" {
			m.menuCursor = index
			return m
		}
	}
	return m
}

// firstEnabled is where the cursor lands when the menu opens.
func firstEnabled(items []menuItem) int {
	for index, item := range items {
		if item.disabled == "" {
			return index
		}
	}
	return 0
}

func (m Model) activateMenu(index int) (Model, tea.Cmd) {
	items := m.menuItems()
	if index < 0 || index >= len(items) {
		return m, nil
	}
	item := items[index]
	if item.disabled != "" {
		return m, nil
	}
	m = m.closeMenu()

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
// was opened from, and overlay() pastes it over the frame. It names the worktree
// it acts on, then rules that off from the actions themselves.
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

	lines := []string{
		styles.DashboardMenuTitle.Render(truncate(selected.Branch, inner)),
		styles.DashboardRule.Render(strings.Repeat(domain.DashboardRuleGlyph, inner)),
	}
	if len(items) == 0 {
		lines = append(lines, styles.DashboardEmpty.Render(truncate(domain.DashboardMenuEmpty, inner)))
	}
	for index, item := range items {
		rendered := m.menuItemLines(menuItemParams{Item: item, Inner: inner, Focused: index == m.menuCursor})
		// Only what can be activated is marked: a zone over an inert entry is a
		// click that looks like it should do something.
		if item.disabled == "" {
			rendered[0] = m.zones.Mark(menuZone(index), rendered[0])
		}
		lines = append(lines, rendered...)
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

type menuItemParams struct {
	Item    menuItem
	Inner   int
	Focused bool
}

// menuItemLines draws one entry: its label, and under it what stands in its way
// when something does. The focused entry carries the tint the rest of the
// dashboard uses.
func (m Model) menuItemLines(params menuItemParams) []string {
	label := truncate(params.Item.label, params.Inner)
	if params.Item.disabled != "" {
		return []string{
			styles.DashboardDisabled.Render(label),
			rowIndent + styles.DashboardRowMeta.Render(truncate(params.Item.disabled, max(params.Inner-rowBarWidth, 0))),
		}
	}
	if params.Focused {
		return []string{styles.DashboardRowSelected.Width(params.Inner).Render(label)}
	}
	return []string{menuItemStyle(params.Item).Render(label)}
}

func menuItemStyle(item menuItem) lipgloss.Style {
	if item.danger {
		return styles.DashboardDanger
	}
	return styles.DashboardRow
}

type menuInnerWidthParams struct {
	Items  []menuItem
	Title  string
	Screen int
}

// menuInnerWidth sizes the menu on its longest line, gutter included, then keeps
// it inside the screen: the box is pasted whole, so it must never need trimming.
func menuInnerWidth(params menuInnerWidthParams) int {
	inner := max(lipgloss.Width(params.Title), lipgloss.Width(domain.DashboardMenuEmpty))
	for _, item := range params.Items {
		inner = max(inner, lipgloss.Width(item.label))
		if item.disabled != "" {
			inner = max(inner, lipgloss.Width(item.disabled)+rowBarWidth)
		}
	}
	return min(inner, params.Screen-domain.DashboardMenuChrome)
}
