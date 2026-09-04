package dashboard

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/LucasPcq/wtm/internal/domain"
	logsflow "github.com/LucasPcq/wtm/internal/flow/run/logs"
	"github.com/LucasPcq/wtm/internal/flow/run/seam"
	"github.com/LucasPcq/wtm/internal/flow/runlogs"
	"github.com/LucasPcq/wtm/internal/rules"
	"github.com/LucasPcq/wtm/internal/styles"
)

// logsRequest is what a tail needs and the dashboard has to supply: the daemon
// keys a job on its name and the worktree it runs in.
type logsRequest struct {
	WorkDir string
	Job     string
	Jobs    []domain.JobConfig
	Lines   int
}

type logsTailMsg struct {
	branch string
	job    string
	lines  []string
	err    error
}

type LogsLoaderParams struct {
	ProjectDir string
	StateDir   string
}

// DefaultLogsLoader reads back what a job persisted, whether or not it still
// runs. NoProbe: nothing is being started here, so there is no port to wait for.
func DefaultLogsLoader(params LogsLoaderParams) func(logsRequest) ([]string, error) {
	return func(req logsRequest) ([]string, error) {
		board := seam.Open(seam.Params{
			ProjectDir: params.ProjectDir,
			StateDir:   params.StateDir,
			WorkDir:    req.WorkDir,
			Jobs:       req.Jobs,
			NoProbe:    true,
		}).Board()
		// No Refresh: History tails the job's log file, and asking the daemon
		// first made a dead daemon look like an unreadable log — in the very
		// case one opens the logs for.
		return board.History(runlogs.HistoryParams{Job: req.Job, Lines: req.Lines})
	}
}

// openLogsTab shows the panel's logs view. It lands on the first job that is
// up, or on the first declared one when nothing is: opening on a job rather
// than on a question is what removes the picker.
// openLogsTab shows the logs view in whichever host is on screen. On the
// Services tab that is its own body: arming the right-hand panel there left an
// invisible view holding esc, enter and the arrows.
func (m Model) openLogsTab() (Model, tea.Cmd) {
	if m.tab == tabServices {
		return m.openServiceLogs()
	}
	return m.openLogsTabOn("")
}

// openLogsTabOn is the same, on a job the surface already designates — a click
// on its row.
func (m Model) openLogsTabOn(job string) (Model, tea.Cmd) {
	status, ok := m.selected()
	if !ok {
		return m, nil
	}
	m.panelTab = panelLogs
	m.logsBranch = status.Branch
	m.logsJob = job
	if job == "" {
		m.logsJob = m.firstLogsJob()
	}
	m.logsLines, m.logsErr = nil, nil
	return m, m.tailLogsCmd()
}

// firstLogsJob is where the view opens: what is up, else the first declared
// job, else nothing — a project with no run module still opens the tab, which
// then says what would fill it.
func (m Model) firstLogsJob() string {
	jobs := m.logsJobs()
	if len(jobs) == 0 {
		return ""
	}
	for _, job := range jobs {
		if m.jobIsUp(job.Name) {
			return job.Name
		}
	}
	return jobs[0].Name
}

// closePanelLogs and closeServiceLogs each put away their own host. The tail
// itself is shared, so it is only dropped once neither host shows it: clearing
// it from one closed the other's view out from under it.
func (m Model) closePanelLogs() Model {
	m.panelTab = panelDetail
	return m.forgetHiddenLogs()
}

func (m Model) forgetHiddenLogs() Model {
	if m.logsOpen() {
		return m
	}
	m.logsBranch, m.logsJob = "", ""
	m.logsLines, m.logsErr = nil, nil
	return m
}

// logsOpen is true wherever the logs view is drawn: under the panel's LOGS tab,
// or as the Services tab's body. The keys it owns follow the view, not a host,
// and it is open even with no job to show — esc has to get back out of an
// empty view too.
func (m Model) logsOpen() bool { return m.panelTab == panelLogs || m.servicesLogs }

func (m Model) retail() (Model, tea.Cmd) { return m, m.tailLogsCmd() }

// watchLogsRequest is what enter hands runview: the job on screen, never the
// whole worktree. A view opened on one job that comes back showing every job of
// the worktree is not the same view.
func (m Model) watchLogsRequest() logsflow.Request {
	return logsflow.Request{
		Worktrees: []string{m.logsBranch},
		Cwd:       m.statusFor(m.logsBranch).Path,
		Job:       m.logsJob,
	}
}

func (m Model) tailLogsCmd() tea.Cmd {
	if m.params.LogsLoader == nil || !m.logsOpen() {
		return nil
	}
	load, branch, job := m.params.LogsLoader, m.logsBranch, m.logsJob
	req := logsRequest{
		WorkDir: m.statusFor(branch).Path,
		Job:     job,
		Jobs:    m.runConfig.Jobs,
		Lines:   domain.DashboardLogsLines,
	}
	return func() tea.Msg {
		lines, err := load(req)
		return logsTailMsg{branch: branch, job: job, lines: lines, err: err}
	}
}

// A superseded tail landing late must not overwrite the panel now open — the
// same rule applyDetail follows.
func (m Model) applyLogsTail(msg logsTailMsg) Model {
	if msg.branch != m.logsBranch || msg.job != m.logsJob {
		return m
	}
	if msg.err != nil {
		m.logsErr = msg.err
		return m
	}
	m.logsLines, m.logsErr = msg.lines, nil
	return m
}

// clickLogsAddress answers a click on the address the logs view shows.
func (m Model) clickLogsAddress(msg tea.MouseMsg) (tea.Model, tea.Cmd, bool) {
	if !m.logsOpen() || !m.inZone(logsURLZone(), msg) {
		return m, nil, false
	}
	model, cmd := m.openJobURL(m.logsAddress().URL)
	return model, cmd, true
}

// clickLogsJob answers a click on the selection line. It serves both hosts:
// the line is the same component wherever it is drawn.
func (m Model) clickLogsJob(msg tea.MouseMsg) (tea.Model, tea.Cmd, bool) {
	if !m.logsOpen() {
		return m, nil, false
	}
	for _, job := range m.logsJobs() {
		if !m.inZone(logsJobZone(job.Name), msg) {
			continue
		}
		if job.Name == m.logsJob {
			return m, nil, true
		}
		m.logsJob, m.logsLines, m.logsErr = job.Name, nil, nil
		return m, m.tailLogsCmd(), true
	}
	return m, nil, false
}

// logsJobs are the jobs the selection line offers: every declared one, in
// run.toml's order. A stopped job keeps its place — History reads back what it
// persisted, which is exactly what one looks for after a crash.
func (m Model) logsJobs() []domain.JobConfig { return m.runConfig.Jobs }

// logsJobsLine heads the logs view: the jobs to switch between, and where the
// current one answers.
func (m Model) logsJobsLine(width int) string {
	// Cut to its own budget before it is styled and marked: spread clips the
	// segments it is handed, and a cut through a zone marker breaks that zone
	// silently — the same discipline the RUN rows follow.
	address := truncate(m.logsAddress().URL, max(width/2, 0))
	rendered := ""
	if address != "" {
		rendered = m.marks().Mark(logsURLZone(), styles.DashboardURL.Render(address))
	}
	return spread(m.logsJobChips(max(width-lipgloss.Width(rendered)-1, 0)), rendered, width)
}

// logsJobChips shows what fits around the current job, with a mark on each side
// that has more. A wrapping row would change the header's height from one
// worktree to the next, and the tail would start somewhere new each time.
func (m Model) logsJobChips(budget int) string {
	jobs := m.logsJobs()
	if len(jobs) == 0 {
		return ""
	}

	window := rules.LogsJobWindow(rules.LogsJobWindowParams{
		Jobs:    jobNames(jobs),
		Current: m.logsJob,
		Budget:  budget,
		Gap:     lipgloss.Width(domain.DashboardLogsJobGap),
		Marks:   lipgloss.Width(domain.DashboardLogsMoreBefore),
	})

	chips := make([]string, 0, window.End-window.Start)
	for _, job := range jobs[window.Start:window.End] {
		chips = append(chips, m.marks().Mark(logsJobZone(job.Name), m.logsJobChip(job)))
	}
	line := strings.Join(chips, domain.DashboardLogsJobGap)

	if window.Start > 0 {
		line = styles.DashboardRowMeta.Render(domain.DashboardLogsMoreBefore) + line
	}
	if window.End < len(jobs) {
		line += styles.DashboardRowMeta.Render(domain.DashboardLogsMoreAfter)
	}
	return line
}

func jobNames(jobs []domain.JobConfig) []string {
	names := make([]string, 0, len(jobs))
	for _, job := range jobs {
		names = append(names, job.Name)
	}
	return names
}

func (m Model) logsJobChip(job domain.JobConfig) string {
	glyph, style := domain.DetailJobDownGlyph, styles.DashboardRowMeta
	if m.jobIsUp(job.Name) {
		glyph, style = domain.DetailJobUpGlyph, styles.DashboardValue
	}
	if job.Name == m.logsJob {
		style = styles.DashboardRowSelected
	}
	return style.Render(glyph + domain.DetailGlyphGap + job.Name)
}

func (m Model) jobIsUp(name string) bool {
	workDir := m.statusFor(m.logsBranch).Path
	for _, info := range m.jobs {
		if info.Name == name && info.WorkDir == workDir && rules.IsJobUp(info.Status) {
			return true
		}
	}
	return false
}

// logsAddress is where the job on screen answers — read off logsBranch, never
// off the selected worktree: the two part company as soon as the logs view is
// hosted by the Services tab.
func (m Model) logsAddress() domain.JobAddress {
	return m.addresses[m.logsBranch][m.logsJob]
}

// stepLogsJob walks the selection line, clamped at both ends: the line is a row
// of chips, not a carousel, so an end that wraps would lose the reader.
func (m Model) stepLogsJob(delta int) Model {
	jobs := m.logsJobs()
	if len(jobs) == 0 {
		return m
	}
	// A run.toml edited under the view can drop the job on screen. Landing on
	// the first one would make an arrow jump somewhere unrelated, so an unknown
	// job is re-seated before it is stepped from.
	index, known := logsJobIndex(jobs, m.logsJob)
	if !known {
		m.logsJob, m.logsLines, m.logsErr = jobs[0].Name, nil, nil
		return m
	}
	next := jobs[rules.ClampIndex(index+delta, len(jobs))]
	if next.Name == m.logsJob {
		return m
	}
	m.logsJob, m.logsLines, m.logsErr = next.Name, nil, nil
	return m
}

type logsViewParams struct {
	Width  int
	Height int
}

// logsViewBody is the logs view wherever it is hosted: the right-hand panel
// under its LOGS tab, and the Services tab at full width. One component, two
// hosts — the alternative was writing "show a tail" twice. The newest lines are
// the ones kept: this is a glance at what just happened, and runview is what
// scrolls.
func (m Model) logsViewBody(params logsViewParams) []string {
	if params.Width <= 0 {
		return nil
	}

	// No rule under the selection line: the tab bar already draws one two rows
	// above, and a second so close reads as a box rather than as a separation.
	// A project with no job has no line to draw at all.
	head := []string{}
	if len(m.logsJobs()) > 0 {
		head = append(head, m.logsJobsLine(params.Width), "")
	}
	hint := styles.DashboardRowMeta.Render(truncate(domain.DashboardLogsHint, params.Width))
	budget := max(params.Height-len(head)-domain.DashboardLogsChrome, 0)

	// Padded to its whole budget so the hint sits on the panel's last row: a
	// reminder that follows the tail lands somewhere new on every job.
	body := m.logsTailLines(logsTailParams{Budget: budget, Width: params.Width})
	for len(body) < budget {
		body = append(body, "")
	}
	return append(append(head, body...), "", hint)
}

func (m Model) logsBody(layout domain.DashboardLayout) []string {
	return m.logsViewBody(logsViewParams{
		Width:  layout.Detail.Width - borderWidth - paddingWidth,
		Height: tabbedPanelBodyHeight(layout.Detail),
	})
}

type logsTailParams struct {
	Budget int
	Width  int
}

func (m Model) logsTailLines(params logsTailParams) []string {
	if empty := m.logsEmptyLines(params.Width); empty != nil {
		return empty
	}
	if m.logsErr != nil {
		return []string{styles.DashboardBlockers.Render(truncate(
			fmt.Sprintf(domain.DashboardUnavailableFmt, m.logsErr), params.Width))}
	}

	kept := m.logsLines[max(len(m.logsLines)-params.Budget, 0):]
	rendered := make([]string, 0, len(kept))
	for _, line := range kept {
		rendered = append(rendered, styles.DashboardValue.Render(truncate(line, params.Width)))
	}
	return rendered
}

// logsEmptyLines tells the three ways this view can have nothing to show apart,
// because the answer differs: a project with no run module needs `wtm run init`,
// a job that never ran needs starting, and a job that ran and wrote nothing is
// simply quiet. Returns nil when there is a tail to draw.
func (m Model) logsEmptyLines(width int) []string {
	if len(m.logsJobs()) == 0 {
		return m.logsNotice(width, domain.DashboardLogsNoModule, domain.DashboardLogsNoModuleHint)
	}
	if len(m.logsLines) > 0 || m.logsErr != nil {
		return nil
	}
	if m.jobIsUp(m.logsJob) {
		return m.logsNotice(width, domain.DashboardLogsQuiet, domain.DashboardLogsQuietHint)
	}
	return m.logsNotice(width, domain.DashboardLogsNeverRan, domain.DashboardLogsNeverRanHint)
}

func (m Model) logsNotice(width int, title, hint string) []string {
	return []string{
		styles.DashboardEmpty.Render(truncate(title, width)),
		"",
		styles.DashboardRowMeta.Render(truncate(hint, width)),
	}
}

func logsJobIndex(jobs []domain.JobConfig, name string) (index int, known bool) {
	for position, job := range jobs {
		if job.Name == name {
			return position, true
		}
	}
	return 0, false
}
