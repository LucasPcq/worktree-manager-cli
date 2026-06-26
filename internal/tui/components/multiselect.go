package components

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/LucasPcq/wtm/internal/styles"
)

// TagVariant selects the color of a MultiSelectItem's status tag.
type TagVariant int

const (
	// TagNeutral renders the tag in the muted color.
	TagNeutral TagVariant = iota
	// TagSuccess renders the tag in green.
	TagSuccess
	// TagWarning renders the tag in yellow.
	TagWarning
	// TagDanger renders the tag in red.
	TagDanger
)

// multiSelectTagWidth is the fixed column width of the optional status tag so
// labels stay aligned regardless of tag length.
const multiSelectTagWidth = 3

// MultiSelectItem represents a toggleable entry in a MultiSelect.
type MultiSelectItem struct {
	Label    string
	Value    string
	Selected bool
	// Tag is an optional short status word shown before the label. It is colored
	// per Variant on normal rows and left plain on the highlighted row.
	Tag     string
	Variant TagVariant
}

// MultiSelectModel is a checkbox list with space toggle and enter confirm.
type MultiSelectModel struct {
	items    []MultiSelectItem
	cursor   int
	width    int
	height   int
	offset   int
	title    string
	desc     string
	validate func([]string) error
	err      error
	done     bool
	aborted  bool
}

// NewMultiSelect creates a MultiSelectModel.
func NewMultiSelect(params NewMultiSelectParams) MultiSelectModel {
	return MultiSelectModel{
		items:    params.Items,
		title:    params.Title,
		desc:     params.Description,
		validate: params.Validate,
		width:    80,
	}
}

// NewMultiSelectParams holds inputs for NewMultiSelect.
type NewMultiSelectParams struct {
	Title       string
	Description string
	Items       []MultiSelectItem
	// Validate is called on Enter and after each toggle. When it returns an
	// error, Enter does not advance and the message is rendered below the list.
	Validate func([]string) error
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
		m.refreshValidation()
	case "a":
		m.toggleAll()
		m.refreshValidation()
	case "enter":
		if m.validate != nil {
			if err := m.validate(m.Values()); err != nil {
				m.err = err
				return m, nil
			}
		}
		m.done = true
	case "esc":
		m.aborted = true
	}

	m.clampOffset()
	return m, nil
}

// toggleAll selects every item, or clears the selection when all are already
// selected, so a single key flips between "all" and "none".
func (m *MultiSelectModel) toggleAll() {
	allSelected := true
	for _, item := range m.items {
		if !item.Selected {
			allSelected = false
			break
		}
	}
	for i := range m.items {
		m.items[i].Selected = !allSelected
	}
}

// refreshValidation re-runs validate against the current selection so the
// error clears the moment the user fixes the state.
func (m *MultiSelectModel) refreshValidation() {
	if m.validate == nil {
		return
	}
	if err := m.validate(m.Values()); err != nil {
		m.err = err
	} else {
		m.err = nil
	}
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
			line := "▸ " + check + " " + plainTag(item.Tag) + item.Label
			pad := m.width - PrintableWidth(line)
			if pad > 0 {
				line += strings.Repeat(" ", pad)
			}
			b.WriteString(styles.ListItemSelected.Render(line))
		} else {
			line := styles.Indent + check + " " + coloredTag(item.Tag, item.Variant) + item.Label
			b.WriteString(styles.ListItemNormal.Render(line))
		}

		if i < end-1 {
			b.WriteString("\n")
		}
	}

	if m.err != nil {
		b.WriteString("\n\n")
		b.WriteString(errorBanner(m.err.Error()))
	}

	return b.String()
}

// plainTag returns the padded, uncolored tag followed by a space, or an empty
// string when there is no tag. Used on the highlighted row where coloring would
// break the background fill.
func plainTag(tag string) string {
	if tag == "" {
		return ""
	}
	return padTag(tag) + " "
}

// coloredTag returns the padded tag colored per variant followed by a space, or
// an empty string when there is no tag.
func coloredTag(tag string, variant TagVariant) string {
	if tag == "" {
		return ""
	}
	return renderTag(variant, padTag(tag)) + " "
}

func padTag(tag string) string {
	pad := multiSelectTagWidth - PrintableWidth(tag)
	if pad > 0 {
		return tag + strings.Repeat(" ", pad)
	}
	return tag
}

func renderTag(variant TagVariant, tag string) string {
	switch variant {
	case TagSuccess:
		return styles.Success.Render(tag)
	case TagWarning:
		return styles.Warning.Render(tag)
	case TagDanger:
		return styles.DangerText.Render(tag)
	default:
		return styles.Muted.Render(tag)
	}
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
