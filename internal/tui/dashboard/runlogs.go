package dashboard

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

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
		if err := board.Refresh(); err != nil {
			return nil, err
		}
		return board.History(runlogs.HistoryParams{Job: req.Job, Lines: req.Lines})
	}
}

// openLogsTab shows the panel's logs view. It lands on the first job that is
// up, or on the first declared one when nothing is: opening on a job rather
// than on a question is what removes the picker.
func (m Model) openLogsTab() (Model, tea.Cmd) { return m.openLogsTabOn("") }

// openLogsTabOn is the same, on a job the surface already designates — a click
// on its row.
func (m Model) openLogsTabOn(job string) (Model, tea.Cmd) {
	if !m.logsAvailable() {
		return m, nil
	}
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

func (m Model) firstLogsJob() string {
	jobs := m.logsJobs()
	for _, job := range jobs {
		if m.jobIsUp(job.Name) {
			return job.Name
		}
	}
	return jobs[0].Name
}

func (m Model) closePanelLogs() Model {
	m.panelTab = panelDetail
	m.logsBranch, m.logsJob = "", ""
	m.logsLines, m.logsErr = nil, nil
	return m
}

// logsOpen is true wherever the logs view is drawn: under the panel's LOGS tab,
// or as the Services tab's body. The keys it owns follow the view, not a host.
func (m Model) logsOpen() bool {
	return m.logsJob != "" && (m.panelTab == panelLogs || m.servicesLogs)
}

func (m Model) retail() (Model, tea.Cmd) { return m, m.tailLogsCmd() }

// watchLogsRequest is what enter hands runview: the job on screen, never the
// whole worktree. A view opened on one job that comes back showing every job of
// the worktree is not the same view.
func (m Model) watchLogsRequest() logsflow.Request {
	return logsflow.Request{
		Worktree: m.logsBranch,
		Cwd:      m.statusFor(m.logsBranch).Path,
		Job:      m.logsJob,
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

// logsJobs are the jobs the selection line offers: every declared one, in
// run.toml's order. A stopped job keeps its place — History reads back what it
// persisted, which is exactly what one looks for after a crash.
func (m Model) logsJobs() []domain.JobConfig { return m.runConfig.Jobs }

// logsJobsLine heads the logs view: the jobs to switch between, and where the
// current one answers.
func (m Model) logsJobsLine(width int) string {
	jobs := m.logsJobs()
	chips := make([]string, 0, len(jobs))
	for _, job := range jobs {
		chips = append(chips, m.logsJobChip(job))
	}
	return spread(
		strings.Join(chips, domain.DashboardLogsJobGap),
		styles.DashboardURL.Render(m.logsAddress().URL),
		width,
	)
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
	index := 0
	for position, job := range jobs {
		if job.Name == m.logsJob {
			index = position
			break
		}
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

	head := []string{
		m.logsJobsLine(params.Width),
		styles.DashboardRule.Render(strings.Repeat(domain.DashboardRuleGlyph, params.Width)),
		"",
	}
	hint := styles.DashboardRowMeta.Render(truncate(domain.DashboardLogsHint, params.Width))
	budget := max(params.Height-len(head)-domain.DashboardLogsChrome, 0)

	return append(append(head, m.logsTailLines(logsTailParams{Budget: budget, Width: params.Width})...), "", hint)
}

func (m Model) logsBody(layout domain.DashboardLayout) []string {
	return m.logsViewBody(logsViewParams{
		Width:  layout.Detail.Width - borderWidth - paddingWidth,
		Height: panelBodyHeight(layout.Detail) - domain.DashboardPanelTabsChrome,
	})
}

type logsTailParams struct {
	Budget int
	Width  int
}

func (m Model) logsTailLines(params logsTailParams) []string {
	if m.logsErr != nil {
		return []string{styles.DashboardBlockers.Render(truncate(
			fmt.Sprintf(domain.DashboardUnavailableFmt, m.logsErr), params.Width))}
	}
	if len(m.logsLines) == 0 {
		return []string{styles.DashboardEmpty.Render(truncate(domain.DashboardLogsEmpty, params.Width))}
	}

	kept := m.logsLines[max(len(m.logsLines)-params.Budget, 0):]
	rendered := make([]string, 0, len(kept))
	for _, line := range kept {
		rendered = append(rendered, styles.DashboardValue.Render(truncate(line, params.Width)))
	}
	return rendered
}
