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

// startRunUp brings a worktree's default profile up, in the run view. The view
// takes the terminal for as long as it is open and gives it back on exit, the
// jobs carrying on without it — the same contract as `wtm run up`.
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
		Request: upflow.Request{Worktree: runWorktree(selected), Cwd: selected.Path, Config: cfg},
		Prompter: prompter{
			send:      send,
			title:     domain.DashboardMenuRunUp,
			shape:     modalStepper,
			opID:      id,
			targetKey: declared.TargetKey,
		},
		Presenter: runPresenter{presenter: presenter{send: send, id: id}, watcher: watcher{send: send}},
	}

	return m, tea.Batch(m.spinner.Tick, func() tea.Msg {
		_, err := upflow.Run(params)
		return opDoneMsg{id: id, err: err}
	})
}

// startRunJob starts one job the user names, in the run view: a job is a
// request of its own, and `target.JobStep` has no safe default — which is why
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
		Presenter: runPresenter{presenter: presenter{send: send, id: id}, watcher: watcher{send: send}},
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

// startRunLogs opens the run view on what the worktree has, starting nothing.
func (m Model) startRunLogs(selected domain.WorktreeStatus) (Model, tea.Cmd) {
	if reason, refused := m.busyReason(selected.Branch); refused {
		return m.refuse(reason), nil
	}
	cfg, ok := m.runRequest(selected)
	if !ok {
		return m.refuse(domain.DashboardRunNotConfigured), nil
	}

	m, id := m.beginOp(beginParams{
		Operation: flow.Operation{Kind: domain.OpKindRunLogs, Mode: flow.ModeBlocking},
	})
	send := m.sender()

	params := logsflow.Params{
		Context:   m.flowContext(),
		Request:   logsflow.Request{Worktree: runWorktree(selected), Cwd: selected.Path, Config: cfg},
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
		if !m.inZone(runRowZone(job.Name), msg) {
			continue
		}
		url := m.addressFor(job.Name).URL
		if url == "" {
			return m, nil, true
		}
		model, cmd := m.openJobURL(url)
		return model, cmd, true
	}
	return m, nil, false
}

// addressFor is where the selected worktree's job answers.
func (m Model) addressFor(job string) domain.JobAddress {
	return m.details[m.selectedBranch()].RunAddresses[job]
}

// Off the UI goroutine: the opener hands the url to the desktop, and calling it
// inside Update would freeze the program.
func (m Model) openJobURL(url string) (Model, tea.Cmd) {
	opener := m.urlOpener()
	if opener == nil {
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
