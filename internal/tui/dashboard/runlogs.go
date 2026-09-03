package dashboard

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/flow"
	"github.com/LucasPcq/wtm/internal/flow/run/seam"
	"github.com/LucasPcq/wtm/internal/flow/run/target"
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

// logsJobMsg carries the job the picker settled on back onto the UI goroutine.
// An empty job means the picker was dismissed, which opens nothing.
type logsJobMsg struct{ job string }

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

// openLogsPanel turns the detail panel into a job's tail. The panel speaks of
// one worktree's job, so it is keyed on both and closed by any move off it.
func (m Model) openLogsPanel(job string) (Model, tea.Cmd) {
	status, ok := m.selected()
	if !ok || job == "" {
		return m, nil
	}
	m.logsBranch, m.logsJob = status.Branch, job
	m.logsLines, m.logsErr = nil, nil
	return m, m.tailLogsCmd()
}

func (m Model) closeLogsPanel() Model {
	m.logsBranch, m.logsJob = "", ""
	m.logsLines, m.logsErr = nil, nil
	return m
}

func (m Model) logsOpen() bool { return m.logsJob != "" }

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

// logsBody replaces the detail's sections while a job's tail is open. The
// newest lines are the ones kept: this panel is a glance at what just
// happened, and runview is what scrolls.
func (m Model) logsBody(layout domain.DashboardLayout) []string {
	width := layout.Detail.Width - borderWidth - paddingWidth
	if width <= 0 {
		return nil
	}

	head := []string{
		spread(
			styles.DashboardBranch.Render(truncate(m.logsHeader(), width)),
			styles.DashboardURL.Render(m.addressFor(m.logsJob).URL),
			width,
		),
		styles.DashboardRule.Render(strings.Repeat(domain.DashboardRuleGlyph, width)),
		"",
	}
	hint := styles.DashboardRowMeta.Render(truncate(domain.DashboardLogsHint, width))
	budget := max(panelBodyHeight(layout.Detail)-len(head)-2, 0)

	return append(append(head, m.logsTailLines(budget, width)...), "", hint)
}

func (m Model) logsTailLines(budget, width int) []string {
	if m.logsErr != nil {
		return []string{styles.DashboardBlockers.Render(truncate(
			fmt.Sprintf(domain.DashboardUnavailableFmt, m.logsErr), width))}
	}
	if len(m.logsLines) == 0 {
		return []string{styles.DashboardEmpty.Render(truncate(domain.DashboardLogsEmpty, width))}
	}

	kept := m.logsLines[max(len(m.logsLines)-budget, 0):]
	rendered := make([]string, 0, len(kept))
	for _, line := range kept {
		rendered = append(rendered, styles.DashboardValue.Render(truncate(line, width)))
	}
	return rendered
}

func (m Model) logsHeader() string {
	workDir := m.statusFor(m.logsBranch).Path
	for _, info := range m.jobs {
		if info.Name != m.logsJob || info.WorkDir != workDir || !rules.IsJobUp(info.Status) {
			continue
		}
		state := domain.DetailJobUpLabel
		if uptime := rules.JobUptime(rules.JobUptimeParams{Job: info, Now: time.Now()}); uptime != "" {
			state += " " + uptime
		}
		return fmt.Sprintf(domain.DetailLogsHeaderFmt, m.logsJob, state)
	}
	return m.logsJob
}

// askLogsJob puts the job question to the picker rather than guessing one: the
// detail panel has no cursor, and reading a job's logs is not a mutation, so
// this borrows target.JobStep without going through a flow.
func (m Model) askLogsJob() (Model, tea.Cmd) {
	if len(m.runConfig.Jobs) == 0 {
		return m.refuse(domain.DashboardRunNotConfigured), nil
	}
	if _, ok := m.selected(); !ok {
		return m, nil
	}

	reply := make(chan promptReply, 1)
	model, cmd := m.openModal(promptMsg{
		title:   domain.DashboardMenuRunLogs,
		shape:   modalStepper,
		session: flow.Session{Steps: []flow.Step{target.JobStep(target.JobParams{Jobs: m.runConfig.Jobs})}},
		reply:   reply,
	})
	return model, tea.Batch(cmd, func() tea.Msg {
		answered := <-reply
		if answered.err != nil {
			return logsJobMsg{}
		}
		return logsJobMsg{job: answered.answers.Value(target.KeyJob)}
	})
}
