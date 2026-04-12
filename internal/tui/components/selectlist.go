package components

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/LucasPcq/wtm/internal/styles"
)

// SelectItem represents a single entry in a SelectList.
type SelectItem struct {
	Label     string
	Value     string
	Badges    []Badge
	Disabled  bool
	Danger    bool
	Separator bool
}

// SelectListModel is a navigable list with full-row highlight and inline filtering.
type SelectListModel struct {
	items     []SelectItem
	filtered  []int
	cursor    int
	filter    string
	filtering bool
	width     int
	height    int
	offset    int
	title     string
	desc      string
	chosen    bool
	aborted   bool
}

// NewSelectList creates a SelectList with the given title, description, and items.
func NewSelectList(params NewSelectListParams) SelectListModel {
	m := SelectListModel{
		items: params.Items,
		title: params.Title,
		desc:  params.Description,
		width: 80,
	}
	m.refilter()
	m.snapToSelectable()
	return m
}

// NewSelectListParams holds inputs for NewSelectList.
type NewSelectListParams struct {
	Title       string
	Description string
	Items       []SelectItem
}

// Chosen returns true after the user confirmed a selection.
func (m SelectListModel) Chosen() bool { return m.chosen }

// Aborted returns true after the user pressed Esc (outside filter mode).
func (m SelectListModel) Aborted() bool { return m.aborted }

// Value returns the selected item's value.
func (m SelectListModel) Value() string {
	if m.cursor >= 0 && m.cursor < len(m.filtered) {
		return m.items[m.filtered[m.cursor]].Value
	}
	return ""
}

// Init satisfies tea.Model.
func (m SelectListModel) Init() tea.Cmd { return nil }

// Update handles key events for navigation, filtering, and selection.
func (m SelectListModel) Update(msg tea.Msg) (SelectListModel, tea.Cmd) {
	keyMsg, ok := msg.(tea.KeyMsg)
	if !ok {
		return m, nil
	}

	if m.filtering {
		m = m.updateFilter(keyMsg)
	} else {
		m = m.updateNormal(keyMsg)
	}

	m.clampOffset()
	return m, nil
}

func (m SelectListModel) updateNormal(msg tea.KeyMsg) SelectListModel {
	switch msg.String() {
	case "up", "k":
		m.moveUp()
	case "down", "j":
		m.moveDown()
	case "enter":
		if len(m.filtered) > 0 {
			m.chosen = true
		}
	case "esc":
		m.aborted = true
	case "/":
		m.filtering = true
		m.filter = ""
	}
	return m
}

func (m SelectListModel) updateFilter(msg tea.KeyMsg) SelectListModel {
	switch msg.String() {
	case "esc":
		m.filtering = false
		m.filter = ""
		m.refilter()
		m.snapToSelectable()
	case "backspace":
		if len(m.filter) > 0 {
			m.filter = m.filter[:len(m.filter)-1]
		}
		if m.filter == "" {
			m.filtering = false
		}
		m.refilter()
		m.snapToSelectable()
	case "enter":
		if len(m.filtered) > 0 {
			m.filtering = false
			m.chosen = true
		}
	case "up", "k":
		m.moveUp()
	case "down", "j":
		m.moveDown()
	default:
		if len(msg.String()) == 1 && msg.String() >= " " {
			m.filter += msg.String()
			m.refilter()
			m.snapToSelectable()
		}
	}
	return m
}

// View renders the list with full-row highlight on the selected item.
func (m SelectListModel) View() string {
	var b strings.Builder

	if m.filtering {
		b.WriteString(styles.FilterPrompt.Render("/ " + m.filter))
		b.WriteString("\n\n")
	}

	visibleHeight := m.visibleHeight()
	if visibleHeight <= 0 {
		visibleHeight = len(m.filtered)
	}

	end := m.offset + visibleHeight
	if end > len(m.filtered) {
		end = len(m.filtered)
	}

	for vi := m.offset; vi < end; vi++ {
		idx := m.filtered[vi]
		item := m.items[idx]
		selected := vi == m.cursor

		if item.Separator {
			b.WriteString(m.renderSeparator())
		} else {
			b.WriteString(m.renderItem(item, selected))
		}
		if vi < end-1 {
			b.WriteString("\n")
		}
	}

	if len(m.filtered) == 0 {
		b.WriteString(styles.Muted.Render("  No matches"))
	}

	return b.String()
}

func (m SelectListModel) renderItem(item SelectItem, selected bool) string {
	if selected {
		return m.renderSelectedItem(item)
	}
	return m.renderNormalItem(item)
}

func (m SelectListModel) renderSelectedItem(item SelectItem) string {
	// Build plain text content — no nested ANSI styles so the background fills cleanly.
	line := "▸ " + item.Label + m.renderBadgesPlain(item.Badges)

	pad := m.width - len(line)
	if pad > 0 {
		line += strings.Repeat(" ", pad)
	}

	return styles.ListItemSelected.Render(line)
}

func (m SelectListModel) renderNormalItem(item SelectItem) string {
	label := item.Label
	if item.Danger {
		label = styles.DangerText.Render(label)
	}

	var badges string
	if len(item.Badges) > 0 {
		parts := make([]string, len(item.Badges))
		for i, badge := range item.Badges {
			parts[i] = badge.Render()
		}
		badges = " " + strings.Join(parts, " ")
	}

	return styles.ListItemNormal.Render("  " + label + badges)
}

func (m SelectListModel) renderBadgesPlain(badges []Badge) string {
	if len(badges) == 0 {
		return ""
	}
	parts := make([]string, len(badges))
	for i, badge := range badges {
		parts[i] = badge.Text
	}
	return " " + strings.Join(parts, " ")
}

func (m SelectListModel) renderSeparator() string {
	width := m.width - 4
	if width < 4 {
		width = 4
	}
	return styles.Divider.Render("  " + strings.Repeat("─", width))
}

func (m *SelectListModel) refilter() {
	m.filtered = m.filtered[:0]
	lower := strings.ToLower(m.filter)
	for i, item := range m.items {
		if m.filter != "" && item.Separator {
			continue
		}
		if m.filter != "" && !strings.Contains(strings.ToLower(item.Label), lower) {
			continue
		}
		m.filtered = append(m.filtered, i)
	}
	if m.cursor >= len(m.filtered) {
		m.cursor = max(0, len(m.filtered)-1)
	}
}

func (m *SelectListModel) snapToSelectable() {
	if len(m.filtered) == 0 {
		return
	}
	if !m.isSelectable(m.cursor) {
		m.moveDown()
	}
	if !m.isSelectable(m.cursor) {
		m.moveUp()
	}
}

func (m *SelectListModel) moveUp() {
	for i := m.cursor - 1; i >= 0; i-- {
		if m.isSelectable(i) {
			m.cursor = i
			return
		}
	}
}

func (m *SelectListModel) moveDown() {
	for i := m.cursor + 1; i < len(m.filtered); i++ {
		if m.isSelectable(i) {
			m.cursor = i
			return
		}
	}
}

func (m SelectListModel) isSelectable(idx int) bool {
	if idx < 0 || idx >= len(m.filtered) {
		return false
	}
	item := m.items[m.filtered[idx]]
	return !item.Separator && !item.Disabled
}

func (m SelectListModel) visibleHeight() int {
	if m.height <= 0 {
		return 0
	}
	overhead := 0
	if m.filtering {
		overhead = 2
	}
	return max(1, m.height-overhead)
}

func (m *SelectListModel) clampOffset() {
	visibleHeight := m.visibleHeight()
	if visibleHeight <= 0 {
		return
	}
	if m.cursor < m.offset {
		m.offset = m.cursor
	}
	if m.cursor >= m.offset+visibleHeight {
		m.offset = m.cursor - visibleHeight + 1
	}
	if m.offset < 0 {
		m.offset = 0
	}
}
