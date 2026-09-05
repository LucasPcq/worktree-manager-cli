package components

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/rules"
	"github.com/LucasPcq/wtm/internal/styles"
)

// RouteListModel asks, service by service, where it reads the port wtm declares
// for it. Rows arrive pre-filled: the answer this step exists for is the one the
// reader wants to change.
type RouteListModel struct {
	rows    []domain.PortRouteRow
	cursor  int
	width   int
	height  int
	title   string
	desc    string
	done    bool
	aborted bool
}

type NewRouteListParams struct {
	Title       string
	Description string
	Rows        []domain.PortRouteRow
}

func NewRouteList(params NewRouteListParams) RouteListModel {
	return RouteListModel{
		rows:  params.Rows,
		title: params.Title,
		desc:  params.Description,
		width: 80,
	}
}

func (m RouteListModel) Rows() []domain.PortRouteRow { return m.rows }
func (m RouteListModel) Done() bool                  { return m.done }
func (m RouteListModel) Aborted() bool               { return m.aborted }
func (m RouteListModel) Init() tea.Cmd               { return nil }

func (m *RouteListModel) SetSize(params SetSizeParams) {
	m.width, m.height = params.Width, params.Height
}

func (m RouteListModel) Routes() map[domain.PortRef]domain.PortRoute {
	routes := make(map[domain.PortRef]domain.PortRoute, len(m.rows))
	for _, row := range m.rows {
		routes[domain.PortRef{Job: row.Job, Name: row.Port}] = row.Route
	}
	return routes
}

func (m RouteListModel) doneRow() int { return len(m.rows) }

func (m RouteListModel) Update(msg tea.Msg) (RouteListModel, tea.Cmd) {
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
	case " ", "enter":
		if m.cursor == m.doneRow() {
			m.done = true
			return m, nil
		}
		return m.toggle(), nil
	case "esc":
		m.aborted = true
	}

	return m, nil
}

func (m RouteListModel) toggle() RouteListModel {
	rows := make([]domain.PortRouteRow, len(m.rows))
	copy(rows, m.rows)
	if rows[m.cursor].Route == domain.PortRouteEnv {
		rows[m.cursor].Route = domain.PortRouteCommand
	} else {
		rows[m.cursor].Route = domain.PortRouteEnv
	}
	m.rows = rows
	return m
}

func (m RouteListModel) View() string {
	jobWidth, portWidth := rules.PortRouteWidths(m.rows)

	var b strings.Builder
	for i, row := range m.rows {
		m.renderRow(&b, rules.PortRouteRowLabel(row, jobWidth, portWidth), i == m.cursor)
		b.WriteString("\n")
	}
	m.renderRow(&b, domain.WizardDoneRow, m.cursor == m.doneRow())
	return b.String()
}

func (m RouteListModel) helpActions() []string { return []string{domain.HelpSwitchRoute} }

func (m RouteListModel) helpModal() string { return "" }

func (m RouteListModel) renderRow(b *strings.Builder, label string, selected bool) {
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
