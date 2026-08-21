package runview

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/flow/runlogs"
	"github.com/LucasPcq/wtm/internal/rules"
	"github.com/LucasPcq/wtm/internal/styles"
)

// View draws one frame as a fixed number of rows: the terminal holds nothing
// else, so a row too many shifts everything under it and a row too wide wraps.
func (m Model) View() string {
	layout := m.layout()
	if m.height <= 0 || m.width <= 0 {
		return ""
	}

	lines := make([]string, 0, m.height)
	if layout.Header.Height > 0 {
		lines = append(lines, m.renderHeader(layout))
	}
	lines = append(lines, m.renderNotice(layout)...)
	lines = append(lines, m.renderBody(layout)...)
	if layout.Help.Height > 0 {
		lines = append(lines, m.renderHelp(layout))
	}
	return strings.Join(fit(lines, m.height), "\n")
}

// renderNotice draws the abort report as a band under the header. It takes rows
// from the body, never from what the panes hold: the output of the job that
// failed is the first thing the reader will want under it.
func (m Model) renderNotice(layout domain.RunViewLayout) []string {
	report := m.report()
	if layout.Notice.Height <= 0 || len(report) == 0 {
		return nil
	}

	shown := rules.ClipReport(report, layout.Notice.Height)
	lines := make([]string, 0, len(shown))
	for index, line := range shown {
		lines = append(lines, truncate(m.styleReportLine(index, len(shown), line), layout.Notice.Width))
	}
	return fit(lines, layout.Notice.Height)
}

func (m Model) styleReportLine(index, count int, line string) string {
	if index == 0 {
		return styles.DashboardDanger.Render(line)
	}
	if index == count-1 {
		return styles.Muted.Render(line)
	}
	return styles.Warning.Render(line)
}

// renderBody fills the rows between the header and the footer, and gives them up
// blank when there is not enough of them for a bordered panel to say anything.
func (m Model) renderBody(layout domain.RunViewLayout) []string {
	if layout.Pane.Height < domain.RunViewMinPanelRows || layout.Pane.Width < domain.RunViewMinPanelCols {
		return make([]string, max(layout.Pane.Height, 0))
	}

	body := m.renderPanePanel(layout)
	if layout.SidebarVisible {
		body = lipgloss.JoinHorizontal(lipgloss.Top, m.renderSidebar(layout), body)
	}
	return fit(strings.Split(body, "\n"), layout.Pane.Height)
}

func (m Model) renderHeader(layout domain.RunViewLayout) string {
	if layout.Header.Height <= 0 {
		return ""
	}

	left := styles.Bold.Render(domain.RunViewHeaderTitle)
	right := styles.Muted.Render(fmt.Sprintf(domain.RunViewRunningFmt, m.runningCount(), len(m.jobs)))
	if m.sequence.active {
		right = styles.Primary.Render(fmt.Sprintf(domain.RunViewStepFmt,
			m.sequence.step, m.sequence.steps, m.sequence.job))
	}
	if m.notice != "" {
		right = styles.Warning.Render(truncate(m.notice, layout.Header.Width))
	}
	if m.err != nil {
		right = styles.DangerText.Render(truncate(m.err.Error(), layout.Header.Width))
	}
	return spread(spreadParams{Left: left, Right: right, Width: layout.Header.Width})
}

func (m Model) runningCount() int {
	running := 0
	for _, view := range m.jobs {
		if view.Status == domain.JobStatusRunning {
			running++
		}
	}
	return running
}

func (m Model) renderSidebar(layout domain.RunViewLayout) string {
	width := layout.Sidebar.Width - domain.RunViewBorderWidth
	textWidth := width - 2
	lines := append(
		[]string{styles.Muted.Render(domain.RunViewJobsTitle)},
		m.renderJobRows(jobRowsParams{Width: textWidth, Rows: layout.SidebarRows})...,
	)

	return styles.RunViewSidebar.
		Width(width).
		Height(layout.Sidebar.Height - domain.RunViewBorderWidth).
		Render(strings.Join(lines, "\n"))
}

type jobRowsParams struct {
	Width int
	Rows  int
}

func (m Model) renderJobRows(params jobRowsParams) []string {
	visible := m.visible()
	if len(visible) == 0 {
		return []string{styles.Muted.Render(truncate(m.emptyMessage(), params.Width))}
	}

	now := time.Now()
	rows := make([]string, 0, params.Rows)
	for _, view := range visible[min(m.offset, len(visible)):] {
		if len(rows) == params.Rows {
			break
		}
		rows = append(rows, m.renderJobRow(jobRowParams{View: view, Width: params.Width, Now: now}))
	}
	return rows
}

type jobRowParams struct {
	View  runlogs.JobView
	Width int
	Now   time.Time
}

func (m Model) renderJobRow(params jobRowParams) string {
	cursor, name := " ", params.View.Name
	if params.View.Name == m.selected {
		cursor, name = styles.RunViewJobSelected.Render(domain.RunViewCursorMark), styles.RunViewJobSelected.Render(name)
	}
	uptime := rules.JobUptime(rules.JobUptimeParams{
		Job: domain.JobInfo{Status: params.View.Status, StartedAt: params.View.StartedAt},
		Now: params.Now,
	})

	return spread(spreadParams{
		Left:  cursor + m.jobMark(params.View) + " " + name,
		Right: styles.Muted.Render(uptime),
		Width: params.Width,
	})
}

func (m Model) jobMark(view runlogs.JobView) string {
	step, tracked := m.sequence.states[view.Name]
	return renderMark(rules.JobMark(rules.JobMarkParams{Status: view.Status, Step: step, Tracked: tracked}))
}

func renderMark(mark domain.JobMark) string {
	switch mark {
	case domain.JobMarkStarting:
		return styles.Warning.Render(domain.RunViewMarkStarting)
	case domain.JobMarkRunning:
		return styles.Success.Render(domain.RunViewMarkRunning)
	case domain.JobMarkDone:
		return styles.Success.Render(domain.RunViewMarkDone)
	case domain.JobMarkCrashed:
		return styles.DangerText.Render(domain.RunViewMarkCrashed)
	default:
		return styles.Muted.Render(domain.RunViewMarkStopped)
	}
}

func (m Model) renderPanePanel(layout domain.RunViewLayout) string {
	view, found := m.selectedView()
	lines := append([]string{m.renderPaneTitle(paneTitleParams{View: view, Found: found, Width: layout.PaneCols})},
		m.renderPaneBody(paneBodyParams{View: view, Found: found, Layout: layout})...)

	return m.paneStyle().
		Width(layout.Pane.Width - domain.RunViewBorderWidth).
		Height(layout.Pane.Height - domain.RunViewBorderWidth).
		Render(strings.Join(clip(lines, layout.Pane.Height-domain.RunViewBorderWidth), "\n"))
}

func (m Model) paneStyle() lipgloss.Style {
	if m.focused {
		return styles.RunViewPaneFocused
	}
	return styles.RunViewPane
}

type paneTitleParams struct {
	View  runlogs.JobView
	Found bool
	Width int
}

func (m Model) renderPaneTitle(params paneTitleParams) string {
	if !params.Found {
		return ""
	}
	status := rules.LabelWithPorts(rules.LabelWithPortsParams{
		Label: string(params.View.Status),
		Ports: m.sequence.ports[params.View.Name],
	})
	left := styles.Bold.Render(params.View.Name) + styles.Muted.Render(domain.RunViewSeparator+status)
	return spread(spreadParams{Left: left, Right: styles.Muted.Render(m.paneOrigin(params.View)), Width: params.Width})
}

// paneOrigin says what the pane is showing: the job as it prints, the log file
// it left behind, or a point in a history the reader has scrolled back into.
func (m Model) paneOrigin(view runlogs.JobView) string {
	if m.focused {
		return domain.RunViewFocusLabel
	}
	entry, held := m.panes.entry(view.Name)
	if !held {
		return ""
	}
	if offset := entry.pane.ScrollOffset(); offset > 0 {
		return fmt.Sprintf(domain.RunViewPaneScrollFmt, offset)
	}
	if entry.source == sourceHistory {
		return domain.RunViewPaneHistoryLabel
	}
	return domain.RunViewPaneLiveLabel
}

type paneBodyParams struct {
	View   runlogs.JobView
	Found  bool
	Layout domain.RunViewLayout
}

func (m Model) renderPaneBody(params paneBodyParams) []string {
	if !params.Found {
		return []string{styles.Muted.Render(truncate(m.emptyMessage(), params.Layout.PaneCols))}
	}

	entry, held := m.panes.entry(params.View.Name)
	if !held {
		return []string{styles.Muted.Render(m.placeholder(params.View))}
	}
	rendered := entry.pane.Render()
	if strings.TrimSpace(ansi.Strip(rendered)) == "" {
		return []string{styles.Muted.Render(m.placeholderFor(params.View, entry))}
	}
	return strings.Split(rendered, "\n")
}

func (m Model) placeholderFor(view runlogs.JobView, entry jobPane) string {
	if entry.source == sourceHistory {
		return domain.RunViewPaneNoHistory
	}
	return m.placeholder(view)
}

func (m Model) placeholder(view runlogs.JobView) string {
	if view.Attachable {
		return domain.RunViewPaneWaiting
	}
	return domain.RunViewPaneNoHistory
}

func (m Model) emptyMessage() string {
	if m.filter != "" {
		return fmt.Sprintf(domain.RunViewNoMatchFmt, m.filter)
	}
	return domain.RunViewEmptyMessage
}

func (m Model) renderHelp(layout domain.RunViewLayout) string {
	if layout.Help.Height <= 0 {
		return ""
	}
	if m.focused {
		return styles.HelpBar.Render(truncate(
			fmt.Sprintf(domain.RunViewFocusHintFmt, m.selected, domain.RunViewFocusExitKey), layout.Help.Width))
	}
	if m.filtering {
		return styles.HelpBar.Render(truncate(
			domain.RunViewFilterPrompt+m.filter+styles.FilterCursor.Render(" ")+domain.RunViewSeparator+domain.RunViewHelpFilter, layout.Help.Width))
	}
	help := domain.RunViewHelpBrowse
	if m.filter != "" {
		help = fmt.Sprintf(domain.RunViewFilterHintFmt, m.filter) + domain.RunViewSeparator + help
	}
	return styles.HelpBar.Render(truncate(help, layout.Help.Width))
}

type spreadParams struct {
	Left  string
	Right string
	Width int
}

// spread puts Right flush against the far edge, and drops it whole rather than
// cutting through it when the row is too narrow to hold both.
func spread(params spreadParams) string {
	left := truncate(params.Left, params.Width)
	gap := params.Width - ansi.StringWidth(left) - ansi.StringWidth(params.Right)
	if gap < 1 {
		return left
	}
	return left + strings.Repeat(" ", gap) + params.Right
}

func truncate(text string, width int) string {
	if width <= 0 {
		return ""
	}
	return ansi.Truncate(text, width, "")
}

func clip(lines []string, height int) []string {
	if height <= 0 || len(lines) <= height {
		return lines
	}
	return lines[:height]
}

// fit makes a block exactly height rows: what does not fit is dropped, what is
// missing is blank.
func fit(lines []string, height int) []string {
	if height <= 0 {
		return nil
	}
	for len(lines) < height {
		lines = append(lines, "")
	}
	return clip(lines, height)
}
