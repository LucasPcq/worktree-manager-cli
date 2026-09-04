package dashboard

import (
	tea "github.com/charmbracelet/bubbletea"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/flow"
	downflow "github.com/LucasPcq/wtm/internal/flow/run/down"
	logsflow "github.com/LucasPcq/wtm/internal/flow/run/logs"
	startflow "github.com/LucasPcq/wtm/internal/flow/run/start"
	stopflow "github.com/LucasPcq/wtm/internal/flow/run/stop"
	upflow "github.com/LucasPcq/wtm/internal/flow/run/up"
	"github.com/LucasPcq/wtm/internal/service/integration"
	"github.com/LucasPcq/wtm/internal/service/runconfig"
)

// runRequest is what every run action needs and the dashboard has to fetch:
// run.toml, which the flows take as a business input rather than reading
// themselves.
func (m Model) runRequest(selected domain.WorktreeStatus) (domain.RunConfig, bool) {
	cfg, err := runconfig.Load(m.params.StateDir)
	if err != nil {
		return domain.RunConfig{}, false
	}
	if len(cfg.Jobs) == 0 {
		return domain.RunConfig{}, false
	}
	if selected.Path == "" {
		return domain.RunConfig{}, false
	}
	return cfg, true
}

// runWorktree is what a run started from a row is told to act on. A row already
// designates a worktree, so it goes in where the positional would: target
// presets it, and the worktree step is read back in the recap instead of being
// put to someone who has just answered it by clicking.
//
// The branch, never the path: the positional is resolved by name
// (worktree.Resolve matches branches, exactly then by substring), so a path
// matches nothing and refuses the run. A detached worktree carries no branch
// and names nothing — the flow then falls back to Cwd, which is this same row.
func runWorktree(selected domain.WorktreeStatus) string { return selected.Branch }

// startRunUp brings a worktree's default profile up, detached: the surface is
// given back and the progress goes to the output panel and to the held row's
// stage. Starting three worktrees in a row is the case this serves, and each
// one used to cost an open and an exit of the run view. Watching is what the
// LOGS tab is for.
func (m Model) startRunUp(selected domain.WorktreeStatus) (Model, tea.Cmd) {
	if reason, refused := m.busyReason(selected.Branch); refused {
		return m.refuse(reason), nil
	}
	cfg, ok := m.runRequest(selected)
	if !ok {
		return m.refuse(domain.DashboardRunNotConfigured), nil
	}

	declared := upflow.Operation()
	m, id := m.beginOp(beginParams{Operation: declared, Target: selected.Branch})
	send := m.sender()

	params := upflow.Params{
		Context: m.flowContext(),
		Request: upflow.Request{Worktrees: []string{runWorktree(selected)}, Cwd: selected.Path, Config: cfg},
		Prompter: prompter{
			send:      send,
			title:     domain.DashboardMenuRunUp,
			shape:     modalStepper,
			opID:      id,
			targetKey: declared.TargetKey,
		},
		Presenter: runPresenter{presenter: presenter{send: send, id: id}, Watcher: detachedWatcher{send: send, id: id}},
	}

	return m, tea.Batch(m.spinner.Tick, func() tea.Msg {
		_, err := upflow.Run(params)
		return opDoneMsg{id: id, err: err}
	})
}

// startRunJob starts one job the user names, detached like startRunUp: a job is
// a request of its own, and `target.JobStep` has no safe default — which is why
// its picker opens here rather than a job being guessed.
func (m Model) startRunJob(selected domain.WorktreeStatus) (Model, tea.Cmd) {
	if reason, refused := m.busyReason(selected.Branch); refused {
		return m.refuse(reason), nil
	}
	cfg, ok := m.runRequest(selected)
	if !ok {
		return m.refuse(domain.DashboardRunNotConfigured), nil
	}

	declared := startflow.Operation()
	m, id := m.beginOp(beginParams{Operation: declared, Target: selected.Branch})
	send := m.sender()

	params := startflow.Params{
		Context: m.flowContext(),
		Request: startflow.Request{Worktree: runWorktree(selected), Cwd: selected.Path, Config: cfg},
		Prompter: prompter{
			send:      send,
			title:     domain.DashboardMenuRunStart,
			shape:     modalStepper,
			opID:      id,
			targetKey: declared.TargetKey,
		},
		Presenter: runPresenter{presenter: presenter{send: send, id: id}, Watcher: detachedWatcher{send: send, id: id}},
	}

	return m, tea.Batch(m.spinner.Tick, func() tea.Msg {
		_, err := startflow.Run(params)
		return opDoneMsg{id: id, err: err}
	})
}

// stopRunJob stops one job the user names. Nothing is attached to, so no view
// opens: what became of the job is reported in the output panel.
func (m Model) stopRunJob(selected domain.WorktreeStatus) (Model, tea.Cmd) {
	if reason, refused := m.busyReason(selected.Branch); refused {
		return m.refuse(reason), nil
	}
	cfg, ok := m.runRequest(selected)
	if !ok {
		return m.refuse(domain.DashboardRunNotConfigured), nil
	}

	declared := stopflow.Operation()
	m, id := m.beginOp(beginParams{Operation: declared, Target: selected.Branch})
	send := m.sender()

	params := stopflow.Params{
		Context: m.flowContext(),
		Request: stopflow.Request{Worktree: runWorktree(selected), Cwd: selected.Path, Config: cfg},
		Prompter: prompter{
			send:      send,
			title:     domain.DashboardMenuRunStop,
			shape:     modalStepper,
			opID:      id,
			targetKey: declared.TargetKey,
		},
		Presenter: stopPresenter{presenter: presenter{send: send, id: id}},
	}

	return m, tea.Batch(m.spinner.Tick, func() tea.Msg {
		_, err := stopflow.Run(params)
		return opDoneMsg{id: id, err: err}
	})
}

// startRunDown stops what the worktree has running. It asks nothing — stopping
// everything here is the command's safe default — so the modal never opens.
func (m Model) startRunDown(selected domain.WorktreeStatus) (Model, tea.Cmd) {
	if reason, refused := m.busyReason(selected.Branch); refused {
		return m.refuse(reason), nil
	}
	cfg, ok := m.runRequest(selected)
	if !ok {
		return m.refuse(domain.DashboardRunNotConfigured), nil
	}

	m, id := m.beginOp(beginParams{Operation: downflow.Operation(), Target: selected.Branch})
	send := m.sender()

	params := downflow.Params{
		Context:   m.flowContext(),
		Request:   downflow.Request{Worktree: runWorktree(selected), Cwd: selected.Path, Config: cfg},
		Prompter:  flow.Unattended{},
		Presenter: downPresenter{presenter: presenter{send: send, id: id}},
	}

	return m, tea.Batch(m.spinner.Tick, func() tea.Msg {
		_, err := downflow.Run(params)
		return opDoneMsg{id: id, err: err}
	})
}

// watchLogs hands the terminal to the run view, on the job the logs view is
// showing. It is the one run action that takes the terminal on purpose: the
// full session is what it is for.
func (m Model) watchLogs() (Model, tea.Cmd) {
	selected := m.statusFor(m.logsBranch)
	if reason, refused := m.busyReason(selected.Branch); refused {
		return m.refuse(reason), nil
	}
	cfg, ok := m.runRequest(selected)
	if !ok {
		return m.refuse(domain.DashboardRunNotConfigured), nil
	}

	request := m.watchLogsRequest()
	request.Config = cfg

	m, id := m.beginOp(beginParams{
		Operation: flow.Operation{Kind: domain.OpKindRunLogs, Mode: flow.ModeBlocking},
	})
	send := m.sender()

	params := logsflow.Params{
		Context:   m.flowContext(),
		Request:   request,
		Prompter:  flow.Unattended{},
		Presenter: logsPresenter{presenter: presenter{send: send, id: id}, watcher: watcher{send: send}},
	}

	return m, tea.Batch(m.spinner.Tick, func() tea.Msg {
		_, err := logsflow.Run(params)
		return opDoneMsg{id: id, err: err}
	})
}

// clickRunRow answers a click on a RUN row: the job's address when it publishes
// one. A job that publishes none leads to its logs instead.
func (m Model) clickRunRow(msg tea.MouseMsg) (tea.Model, tea.Cmd, bool) {
	for _, job := range m.runConfig.Jobs {
		// The address cell first: clicking what you read opens what you read,
		// and the rest of the row leads to the job's logs.
		if m.inZone(runURLZone(job.Name), msg) {
			model, cmd := m.openJobURL(m.addressFor(job.Name).URL)
			return model, cmd, true
		}
		if !m.inZone(runRowZone(job.Name), msg) {
			continue
		}
		model, cmd := m.openLogsTabOn(job.Name)
		return model, cmd, true
	}
	return m, nil, false
}

// addressFor is where the selected worktree's job answers.
func (m Model) addressFor(job string) domain.JobAddress {
	return m.addresses[m.selectedBranch()][job]
}

// Off the UI goroutine: the opener hands the url to the desktop, and calling it
// inside Update would freeze the program.
func (m Model) openJobURL(url string) (Model, tea.Cmd) {
	opener := m.urlOpener()
	if opener == nil || url == "" {
		return m, nil
	}
	return m, func() tea.Msg { return openURLMsg{err: opener(url)} }
}

func (m Model) urlOpener() func(string) error {
	if m.params.URLOpener != nil {
		return m.params.URLOpener
	}
	return integration.OpenURL
}

// openSelectedAddress opens the address of the job the surface designates: the
// one on the logs view's selection line, or the one under the Services cursor.
// The DETAIL panel designates none — it has no cursor — so it stays silent, the
// same way KeyOpenPR is silent where no PR is designated.
func (m Model) openSelectedAddress() (Model, tea.Cmd) {
	if m.logsOpen() {
		return m.openJobURL(m.logsAddress().URL)
	}
	if m.tab != tabServices {
		return m, nil
	}
	row, ok := m.selectedService()
	if !ok {
		return m, nil
	}
	return m.openJobURL(m.addresses[row.Branch][row.Job.Key].URL)
}
