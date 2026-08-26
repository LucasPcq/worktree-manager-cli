package components

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/styles"
)

// KindListModel settles the kind of every job it is given. Both kinds sit on
// each row with the current one filled, so the answer is a type being named
// rather than a box whose meaning the reader has to remember.
type KindListModel struct {
	entries []domain.JobKindChoice
	cursor  int
	width   int
	height  int
	title   string
	desc    string
	done    bool
	aborted bool
}

type NewKindListParams struct {
	Title       string
	Description string
	Entries     []domain.JobKindChoice
}

func NewKindList(params NewKindListParams) KindListModel {
	return KindListModel{
		entries: params.Entries,
		title:   params.Title,
		desc:    params.Description,
		width:   80,
	}
}

func (m KindListModel) Entries() []domain.JobKindChoice { return m.entries }
func (m KindListModel) Done() bool                      { return m.done }
func (m KindListModel) Aborted() bool                   { return m.aborted }
func (m KindListModel) Init() tea.Cmd                   { return nil }

func (m *KindListModel) SetSize(params SetSizeParams) {
	m.width, m.height = params.Width, params.Height
}

func (m KindListModel) Update(msg tea.Msg) (KindListModel, tea.Cmd) {
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
		if m.cursor < len(m.entries)-1 {
			m.cursor++
		}
	case "left", "h":
		return m.setKind(domain.JobKindTask), nil
	case "right", "l":
		return m.setKind(domain.JobKindService), nil
	case " ":
		return m.setKind(m.other()), nil
	case "enter":
		m.done = true
	case "esc":
		m.aborted = true
	}

	return m, nil
}

func (m KindListModel) other() domain.JobKind {
	if m.entries[m.cursor].Kind == domain.JobKindService {
		return domain.JobKindTask
	}
	return domain.JobKindService
}

func (m KindListModel) setKind(kind domain.JobKind) KindListModel {
	if len(m.entries) == 0 {
		return m
	}
	m.entries[m.cursor].Kind = kind
	return m
}

func (m KindListModel) View() string {
	var b strings.Builder
	column := m.kindColumn()
	for i, entry := range m.entries {
		m.renderRow(&b, entry, i == m.cursor, column)
		if i < len(m.entries)-1 {
			b.WriteString("\n")
		}
	}
	return b.String()
}

// kindColumn is where the two kinds start on every row: just past the longest
// job, not at the right edge — on a wide terminal the eye would otherwise have
// to cross the screen to read the answer.
func (m KindListModel) kindColumn() int {
	longest := 0
	for _, entry := range m.entries {
		if w := PrintableWidth(fmt.Sprintf(domain.KindListEntryFmt, entry.Label, entry.Cmd)); w > longest {
			longest = w
		}
	}
	return PrintableWidth(styles.Indent) + longest + domain.KindListGap
}

func (m KindListModel) helpHint() string { return domain.KindListHelp }

// renderRow lays the job out on the left and its two kinds on the right, padded
// apart so the kinds line up into a column the eye can scan.
func (m KindListModel) renderRow(b *strings.Builder, entry domain.JobKindChoice, selected bool, column int) {
	prefix := styles.Indent
	if selected {
		prefix = "▸ "
	}
	left := prefix + fmt.Sprintf(domain.KindListEntryFmt, entry.Label, entry.Cmd)

	gap := column - PrintableWidth(left)
	if gap < domain.KindListGap {
		gap = domain.KindListGap
	}
	line := left + strings.Repeat(" ", gap) + kindRadios(entry.Kind)

	if !selected {
		b.WriteString(styles.ListItemNormal.Render(line))
		return
	}
	if pad := m.width - PrintableWidth(line); pad > 0 {
		line += strings.Repeat(" ", pad)
	}
	b.WriteString(styles.ListItemSelected.Render(line))
}

func kindRadios(kind domain.JobKind) string {
	task, service := domain.KindRadioOn, domain.KindRadioOff
	if kind == domain.JobKindService {
		task, service = domain.KindRadioOff, domain.KindRadioOn
	}
	return fmt.Sprintf(domain.KindListRadiosFmt, task, service)
}
