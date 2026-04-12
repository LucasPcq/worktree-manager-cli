package components

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/LucasPcq/wtm/internal/styles"
)

// MultiSelectItem represents a toggleable entry in a MultiSelect.
type MultiSelectItem struct {
	Label    string
	Value    string
	Selected bool
}

// MultiSelectModel is a checkbox list with space toggle and enter confirm.
type MultiSelectModel struct {
	items   []MultiSelectItem
	cursor  int
	width   int
	height  int
	offset  int
	title   string
	desc    string
	done    bool
	aborted bool
}

// NewMultiSelect creates a MultiSelectModel.
func NewMultiSelect(params NewMultiSelectParams) MultiSelectModel {
	return MultiSelectModel{
		items: params.Items,
		title: params.Title,
		desc:  params.Description,
		width: 80,
	}
}

// NewMultiSelectParams holds inputs for NewMultiSelect.
type NewMultiSelectParams struct {
	Title       string
	Description string
	Items       []MultiSelectItem
}

// Done returns true after the user confirmed.
func (m MultiSelectModel) Done() bool { return m.done }

// Aborted returns true after the user pressed Esc.
func (m MultiSelectModel) Aborted() bool { return m.aborted }

// Values returns the values of all selected items.
func (m MultiSelectModel) Values() []string {
	var vals []string
	for _, item := range m.items {
		if item.Selected {
			vals = append(vals, item.Value)
		}
	}
	return vals
}

// Init satisfies tea.Model.
func (m MultiSelectModel) Init() tea.Cmd { return nil }

// Update handles key events.
func (m MultiSelectModel) Update(msg tea.Msg) (MultiSelectModel, tea.Cmd) {
	keyMsg, ok := msg.(tea.KeyMsg)
	if !ok {
		return m, nil
	}

	switch keyMsg.String() {
	case "up", "k":
		if m.cursor > 0 {
			m.cursor--
		}
	case "down", "j":
		if m.cursor < len(m.items)-1 {
			m.cursor++
		}
	case " ":
		if m.cursor >= 0 && m.cursor < len(m.items) {
			m.items[m.cursor].Selected = !m.items[m.cursor].Selected
		}
	case "enter":
		m.done = true
	case "esc":
		m.aborted = true
	}

	m.clampOffset()
	return m, nil
}

// View renders the checkbox list.
func (m MultiSelectModel) View() string {
	if len(m.items) == 0 {
		return styles.Muted.Render("  No items")
	}

	var b strings.Builder

	visibleHeight := m.visibleHeight()
	if visibleHeight <= 0 {
		visibleHeight = len(m.items)
	}

	end := m.offset + visibleHeight
	if end > len(m.items) {
		end = len(m.items)
	}

	for i := m.offset; i < end; i++ {
		item := m.items[i]
		selected := i == m.cursor

		check := "[ ]"
		if item.Selected {
			check = "[✓]"
		}

		if selected {
			line := "▸ " + check + " " + item.Label
			pad := m.width - len(line)
			if pad > 0 {
				line += strings.Repeat(" ", pad)
			}
			b.WriteString(styles.ListItemSelected.Render(line))
		} else {
			b.WriteString(styles.ListItemNormal.Render("  " + check + " " + item.Label))
		}

		if i < end-1 {
			b.WriteString("\n")
		}
	}

	return b.String()
}

func (m MultiSelectModel) visibleHeight() int {
	if m.height <= 0 {
		return 0
	}
	return max(1, m.height)
}

func (m *MultiSelectModel) clampOffset() {
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
