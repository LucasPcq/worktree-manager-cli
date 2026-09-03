package dashboard

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/rules"
	"github.com/LucasPcq/wtm/internal/styles"
)

func (m Model) renderServices(layout domain.DashboardLayout) string {
	return m.renderPanel(panelParams{
		Rect:  layout.List,
		Title: domain.DashboardServicesTitle,
		Body:  m.servicesBody(layout),
		Zone:  zoneServices,
	})
}

// servicesBlocks is what the daemon holds up, read off the poll: jobs and
// addresses, never a lazily loaded detail.
func (m Model) servicesBlocks() []rules.RunWorktreeBlock { return m.board }

// nearestServiceJob is the first job row from index in the given direction,
// then in the other: a header and a gap are drawn, never selected.
func (m Model) nearestServiceJob(index, direction int) int {
	if len(m.services) == 0 {
		return 0
	}
	index = rules.ClampIndex(index, len(m.services))
	for _, step := range []int{direction, -direction} {
		for cursor := index; cursor >= 0 && cursor < len(m.services); cursor += step {
			if m.services[cursor].Kind == domain.ServicesRowJob {
				return cursor
			}
		}
	}
	return index
}

func (m Model) selectedService() (domain.ServicesRow, bool) {
	if m.servicesCursor < 0 || m.servicesCursor >= len(m.services) {
		return domain.ServicesRow{}, false
	}
	row := m.services[m.servicesCursor]
	return row, row.Kind == domain.ServicesRowJob
}

// stepServices walks job rows, never landing on a header: the cursor's subject
// is a job, and a header is a caption over several of them.
func (m Model) stepServices(delta int) Model {
	if delta == 0 || len(m.services) == 0 {
		return m
	}
	direction := 1
	if delta < 0 {
		direction = -1
	}
	cursor := m.servicesCursor
	for range max(delta, -delta) {
		next := m.nearestServiceJob(cursor+direction, direction)
		if next == cursor {
			break
		}
		cursor = next
	}
	m.servicesCursor = cursor
	return m
}

// servicesVisible is the window the tab draws, cursor included.
func (m Model) servicesVisible(layout domain.DashboardLayout) []domain.ServicesRow {
	rows := layout.ServicesRows
	if rows <= 0 || len(m.services) == 0 {
		return nil
	}
	end := min(m.servicesOffset+rows, len(m.services))
	if m.servicesOffset >= end {
		return nil
	}
	return m.services[m.servicesOffset:end]
}

// servicesBody stacks one block per worktree: its name and count on a header
// row, then the same typed rows the detail panel's RUN section draws, so the
// two surfaces cannot read differently.
func (m Model) servicesBody(layout domain.DashboardLayout) []string {
	width := layout.List.Width - borderWidth - paddingWidth
	if width <= 0 {
		return nil
	}

	if m.servicesLogs {
		return m.logsViewBody(logsViewParams{Width: width, Height: panelBodyHeight(layout.List)})
	}
	if len(m.services) == 0 {
		return m.servicesEmptyLines(width)
	}

	// The name column is sized across every block, not per block: the addresses
	// are what this tab is read down, and a column restarting at each worktree
	// would break that read.
	nameWidth := 0
	for _, row := range m.services {
		if row.Kind == domain.ServicesRowJob {
			nameWidth = max(nameWidth, len([]rune(row.Job.Key)))
		}
	}

	visible := m.servicesVisible(layout)
	lines := make([]string, 0, len(visible))
	for position, row := range visible {
		index := m.servicesOffset + position
		switch row.Kind {
		case domain.ServicesRowGap:
			lines = append(lines, "")
		case domain.ServicesRowHeader:
			lines = append(lines, spread(
				styles.DashboardRowName.Render(row.Branch),
				styles.DashboardRowMeta.Render(fmt.Sprintf(domain.DashboardServicesUpFmt, row.Up)),
				width,
			))
		case domain.ServicesRowJob:
			line := sectionRowLines(sectionRowLinesParams{
				Rows: []domain.DetailRow{row.Job}, Width: width, NameWidth: nameWidth,
				MarkAddress: func(_ domain.DetailRow, cell string) string {
					return m.marks().Mark(servicesURLZone(index), cell)
				},
			})[0]
			lines = append(lines, m.marks().Mark(servicesRowZone(index), m.servicesJobLine(index, line)))
		}
	}
	return lines
}

// openServiceLogs turns the tab's body into the logs view, on the job under the
// cursor. Full width: the addresses are why this tab has it, and a log line is
// no narrower than an address.
func (m Model) openServiceLogs() (Model, tea.Cmd) {
	row, ok := m.selectedService()
	if !ok {
		return m, nil
	}
	m.servicesLogs = true
	m.logsBranch, m.logsJob = row.Branch, row.Job.Key
	m.logsLines, m.logsErr = nil, nil
	return m, m.tailLogsCmd()
}

func (m Model) closeServiceLogs() Model {
	m.servicesLogs = false
	m.logsBranch, m.logsJob = "", ""
	m.logsLines, m.logsErr = nil, nil
	return m
}

// servicesJobLine marks the row under the cursor the way a list row is marked:
// the accent bar in its gutter, so the two tabs read as one dashboard.
func (m Model) servicesJobLine(index int, line string) string {
	if index != m.servicesCursor {
		return line
	}
	return styles.DashboardRowBar.Render(rowBar) + strings.TrimPrefix(line, " ")
}

// servicesEmptyLines names every route into this tab, the single job included:
// naming only the profile is what the first version did.
func (m Model) servicesEmptyLines(width int) []string {
	command := 0
	for _, row := range domain.DashboardServicesEmptyRows {
		command = max(command, len([]rune(row[0])))
	}

	lines := []string{styles.DashboardEmpty.Render(truncate(domain.DashboardServicesEmpty, width)), ""}
	for _, row := range domain.DashboardServicesEmptyRows {
		head := domain.DetailListIndent + pad(truncate(row[0], width), command)
		budget := max(width-lipgloss.Width(head)-len(domain.DetailColumnGap), 0)
		lines = append(lines,
			styles.DashboardValue.Render(head)+
				domain.DetailColumnGap+
				styles.DashboardRowMeta.Render(truncate(row[1], budget)))
	}
	return lines
}

// clickServiceAddress answers a click on a Services address cell. The row's own
// zone is left to clickRow, which selects it; enter then opens its logs.
func (m Model) clickServiceAddress(msg tea.MouseMsg) (tea.Model, tea.Cmd, bool) {
	if m.tab != tabServices || m.servicesLogs {
		return m, nil, false
	}
	for index, row := range m.services {
		if row.Kind != domain.ServicesRowJob || !m.inZone(servicesURLZone(index), msg) {
			continue
		}
		model, cmd := m.openJobURL(row.Job.URL)
		return model, cmd, true
	}
	return m, nil, false
}
