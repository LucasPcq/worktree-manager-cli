package components

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/styles"
)

// CmdListModel amends the command of a job whose port variable it never
// mentions. wtm injects the variable; only the command can decide to read it,
// and this is the last moment before the config is written where that costs a
// keystroke rather than another command.
type CmdListModel struct {
	fixes   []domain.JobCmdFix
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

type NewCmdListParams struct {
	Title       string
	Description string
	Fixes       []domain.JobCmdFix
}

func NewCmdList(params NewCmdListParams) CmdListModel {
	return CmdListModel{
		fixes: params.Fixes,
		title: params.Title,
		desc:  params.Description,
		width: 80,
	}
}

func (m CmdListModel) Fixes() []domain.JobCmdFix { return m.fixes }
func (m CmdListModel) Done() bool                { return m.done }
func (m CmdListModel) Aborted() bool             { return m.aborted }
func (m CmdListModel) Init() tea.Cmd             { return nil }

func (m *CmdListModel) SetSize(params SetSizeParams) {
	m.width, m.height = params.Width, params.Height
}

func (m CmdListModel) doneRow() int { return len(m.fixes) }

func (m CmdListModel) Update(msg tea.Msg) (CmdListModel, tea.Cmd) {
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

// startEdit opens on the current command: it is amended, not retyped.
func (m CmdListModel) startEdit() CmdListModel {
	input := textinput.New()
	input.CharLimit = domain.CmdListCharLimit
	input.Width = max(domain.CmdListMinWidth, m.width-domain.CmdListWidthInset)
	input.SetValue(m.fixes[m.cursor].Cmd)
	input.CursorEnd()
	input.Focus()

	m.input = input
	m.editing = true
	m.err = ""
	return m
}

func (m CmdListModel) updateEdit(msg tea.Msg) (CmdListModel, tea.Cmd) {
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

func (m CmdListModel) saveEdit() CmdListModel {
	cmd := strings.TrimSpace(m.input.Value())
	if cmd == "" {
		m.err = domain.CmdListEmptyErr
		return m
	}

	m.fixes[m.cursor].Cmd = cmd
	m.editing = false
	m.err = ""
	return m
}

func (m CmdListModel) View() string {
	var b strings.Builder
	for i, fix := range m.fixes {
		label := fmt.Sprintf(domain.CmdListEntryFmt, fix.Job, stillMissing(fix), fix.Cmd)
		if m.editing && i == m.cursor {
			label = fmt.Sprintf(domain.CmdListEditFmt, fix.Job, strings.Join(fix.Vars, domain.CmdListVarSep), m.input.View())
		}
		m.renderRow(&b, label, i == m.cursor)
		b.WriteString("\n")
	}
	m.renderRow(&b, domain.CmdListDoneRow, m.cursor == m.doneRow())
	if m.err != "" {
		b.WriteString("\n\n")
		b.WriteString(errorBanner(m.err))
	}
	return b.String()
}

// stillMissing re-reads the command as it stands, so a row that has been fixed
// stops naming the variable it was flagged for and the list reads as a checklist.
func stillMissing(fix domain.JobCmdFix) string {
	var missing []string
	for _, name := range fix.Vars {
		if !strings.Contains(fix.Cmd, name) {
			missing = append(missing, name)
		}
	}
	if len(missing) == 0 {
		return domain.CmdListReferenced
	}
	return strings.Join(missing, domain.CmdListVarSep)
}

func (m CmdListModel) helpHint() string {
	if m.editing {
		return domain.CmdListEditHelp
	}
	return domain.CmdListHelp
}

func (m CmdListModel) renderRow(b *strings.Builder, label string, selected bool) {
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
