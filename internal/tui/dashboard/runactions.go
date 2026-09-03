package dashboard

import (
	tea "github.com/charmbracelet/bubbletea"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/flow"
	downflow "github.com/LucasPcq/wtm/internal/flow/run/down"
	logsflow "github.com/LucasPcq/wtm/internal/flow/run/logs"
	upflow "github.com/LucasPcq/wtm/internal/flow/run/up"
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
func runWorktree(selected domain.WorktreeStatus) string { return selected.Path }

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
