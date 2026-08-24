package components

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/styles"
)

const (
	portInputCharLimit = 5
	portInputWidth     = 12
)

// PortListModel reviews the ports detection pre-filled. Enter on a row edits its
// base, Enter on the Done row confirms — the same shape as HookListModel, so a
// reader who knows one knows the other.
type PortListModel struct {
	entries []domain.PortEntry
	cursor  int
	width   int
	height  int
	title   string
	desc    string
	done    bool
	aborted bool

	editing bool
	input   textinput.Model
	err     string
}

type NewPortListParams struct {
	Title       string
	Description string
	Entries     []domain.PortEntry
}

func NewPortList(params NewPortListParams) PortListModel {
	return PortListModel{
		entries: params.Entries,
		title:   params.Title,
		desc:    params.Description,
		width:   80,
	}
}

func (m PortListModel) Entries() []domain.PortEntry { return m.entries }
func (m PortListModel) Done() bool                  { return m.done }
func (m PortListModel) Aborted() bool               { return m.aborted }
func (m PortListModel) Init() tea.Cmd               { return nil }

func (m *PortListModel) SetSize(params SetSizeParams) {
	m.width, m.height = params.Width, params.Height
}

func (m PortListModel) doneRow() int { return len(m.entries) }

func (m PortListModel) Update(msg tea.Msg) (PortListModel, tea.Cmd) {
	if m.editing {
		return m.updateEdit(msg)
	}

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
		if m.cursor < m.doneRow() {
			m.cursor++
		}
	case "enter":
		if m.cursor == m.doneRow() {
			m.done = true
			return m, nil
		}
		return m.startEdit(), nil
	case "esc":
		m.aborted = true
	}

	return m, nil
}

func (m PortListModel) startEdit() PortListModel {
	input := textinput.New()
	input.CharLimit = portInputCharLimit
	input.Width = portInputWidth
	// Empty, with the detected port as placeholder: a port is retyped, never
	// edited character by character, and an empty entry keeps what was detected.
	input.Placeholder = strconv.Itoa(m.entries[m.cursor].Base)
	input.Focus()

	m.input = input
	m.editing = true
	m.err = ""
	return m
}

func (m PortListModel) updateEdit(msg tea.Msg) (PortListModel, tea.Cmd) {
	keyMsg, ok := msg.(tea.KeyMsg)
	if !ok {
		var cmd tea.Cmd
		m.input, cmd = m.input.Update(msg)
		return m, cmd
	}

	switch keyMsg.String() {
	case "enter":
		return m.saveEdit(), nil
	case "esc":
		m.editing = false
		m.err = ""
		return m, nil
	}

	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	return m, cmd
}

// saveEdit refuses anything that is not a usable port rather than overwriting a
// detected value that works.
func (m PortListModel) saveEdit() PortListModel {
	raw := strings.TrimSpace(m.input.Value())
	if raw == "" {
		m.editing = false
		m.err = ""
		return m
	}

	port, err := strconv.Atoi(raw)
	if err != nil || port < domain.PortMin || port > domain.PortMax {
		m.err = fmt.Sprintf(domain.PortListRangeErrFmt, raw, domain.PortMin, domain.PortMax)
		return m
	}

	m.entries[m.cursor].Base = port
	m.editing = false
	m.err = ""
	return m
}

func (m PortListModel) View() string {
	var b strings.Builder
	for i, entry := range m.entries {
		label := fmt.Sprintf(domain.PortListEntryFmt, entry.Job, entry.Name, entry.Base)
		if m.editing && i == m.cursor {
			label = fmt.Sprintf(domain.PortListEditFmt, entry.Job, entry.Name, m.input.View())
		}
		m.renderRow(&b, label, i == m.cursor)
		b.WriteString("\n")
	}
	m.renderRow(&b, domain.PortListDoneRow, m.cursor == m.doneRow())
	if m.err != "" {
		b.WriteString("\n\n")
		b.WriteString(errorBanner(m.err))
	}
	return b.String()
}

// helpHint is the wizard help bar for this step.
func (m PortListModel) helpHint() string {
	if m.editing {
		return domain.PortListEditHelp
	}
	return domain.PortListHelp
}

func (m PortListModel) renderRow(b *strings.Builder, label string, selected bool) {
	if selected {
		line := "▸ " + label
		if pad := m.width - PrintableWidth(line); pad > 0 {
			line += strings.Repeat(" ", pad)
		}
		b.WriteString(styles.ListItemSelected.Render(line))
		return
	}
	b.WriteString(styles.ListItemNormal.Render(styles.Indent + label))
}
