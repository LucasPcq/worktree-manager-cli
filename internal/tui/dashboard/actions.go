package dashboard

import (
	"errors"
	"fmt"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/flow"
	cleanflow "github.com/LucasPcq/wtm/internal/flow/clean"
	createflow "github.com/LucasPcq/wtm/internal/flow/create"
)

func (m Model) flowContext() flow.Context {
	return flow.Context{
		ProjectDir: m.params.ProjectDir,
		StateDir:   m.params.StateDir,
		Config:     m.params.Config,
	}
}

// sender hands a flow goroutine the one way it may reach the model: a message on
// the dashboard's channel, delivered by listenCmd on the UI goroutine.
func (m Model) sender() func(tea.Msg) {
	msgs := m.msgs
	return func(msg tea.Msg) { msgs <- msg }
}

func (m Model) startCreate() (Model, tea.Cmd) {
	if reason, refused := m.busyReason(""); refused {
		return m.refuse(reason), nil
	}
	// The branch is only known once the wizard is answered; the prompter names it
	// then, and the run holds it from there on.
	declared := createflow.Operation()
	m, id := m.beginOp(beginParams{Operation: declared})
	send := m.sender()

	params := createflow.Params{
		Context: m.flowContext(),
		Prompter: prompter{
			send:      send,
			title:     domain.DashboardCreateTitle,
			shape:     modalStepper,
			opID:      id,
			targetKey: declared.TargetKey,
		},
		Presenter: createPresenter{presenter{send: send}},
	}

	return m, func() tea.Msg {
		_, err := createflow.Run(params)
		return opDoneMsg{id: id, err: err}
	}
}

// startClean hands the removal to the same flow the CLI runs. The dashboard
// never presets Force: lifting a refusal is an answer the user gives in the
// modal, one refusal at a time.
func (m Model) startClean(branch string) (Model, tea.Cmd) {
	if reason, refused := m.busyReason(branch); refused {
		return m.refuse(reason), nil
	}
	declared := cleanflow.Operation()
	m, id := m.beginOp(beginParams{Operation: declared, Target: branch})
	send := m.sender()

	params := cleanflow.Params{
		Context: m.flowContext(),
		Request: cleanflow.Request{
			Branch:     branch,
			BaseBranch: m.params.Config.Project.Worktrees.BaseBranch,
		},
		Prompter: prompter{
			send:      send,
			title:     domain.DashboardDeleteTitle,
			shape:     modalForm,
			opID:      id,
			targetKey: declared.TargetKey,
		},
		Presenter: cleanPresenter{presenter{send: send}},
	}

	return m, func() tea.Msg {
		_, err := cleanflow.Run(params)
		return opDoneMsg{id: id, err: err}
	}
}

// busyReason states why nothing may act on a worktree right now: a run already
// holds it, or one holds the whole dashboard. This is where the mode a flow
// declares is enforced — once, rather than at every action site.
func (m Model) busyReason(target string) (string, bool) {
	if op, ok := m.ops.holding(target); ok {
		return fmt.Sprintf(domain.DashboardBusyFmt, target, op.kind), true
	}
	if op, ok := m.ops.blocking(); ok {
		return fmt.Sprintf(domain.DashboardBlockedByFmt, op.kind), true
	}
	return "", false
}

// busyCaption is what a menu entry says under itself while something holds its
// worktree: the same fact as busyReason, worded to sit under a label rather than
// to stand as a line in the output panel.
func (m Model) busyCaption(target string) (string, bool) {
	if op, ok := m.ops.holding(target); ok {
		return fmt.Sprintf(domain.DashboardBusyCaptionFmt, op.kind), true
	}
	if op, ok := m.ops.blocking(); ok {
		return fmt.Sprintf(domain.DashboardBusyCaptionFmt, op.kind), true
	}
	return "", false
}

// refuse states the refusal where the user is already looking for the run that
// caused it.
func (m Model) refuse(text string) Model {
	m.outputExpanded = true
	return m.appendOutput(OutputLineMsg{Text: text}).reflow()
}

type beginParams struct {
	Operation flow.Operation
	// Target is the worktree the run holds, when the surface already knows it.
	Target string
}

// beginOp records the run and opens the output panel: a run whose output is
// folded away is one the user cannot follow.
func (m Model) beginOp(params beginParams) (Model, int) {
	declared := params.Operation
	ops, id := m.ops.begin(operation{kind: declared.Kind, mode: declared.Mode, target: params.Target})
	m.ops = ops
	m.outputExpanded = true
	return m.reflow(), id
}

func (m Model) finishOp(msg opDoneMsg) Model {
	op, _ := m.ops.byID(msg.id)
	m.ops = m.ops.end(msg.id)
	if msg.err == nil || errors.Is(msg.err, domain.ErrUserAborted) {
		return m
	}

	m = m.appendOutput(OutputLineMsg{
		Text: fmt.Sprintf(domain.DashboardFailedFmt, domain.DashboardOperationLabel, msg.err),
	})

	// The privileged removal prompts for a password on the terminal this surface
	// is holding, so it is never offered here — the way to it is named instead.
	if errors.Is(msg.err, domain.ErrWorktreeRemoveFailed) {
		return m.appendOutput(OutputLineMsg{Text: fmt.Sprintf(domain.DashboardPrivilegedHintFmt, op.target)})
	}
	return m
}

// applyFlow handles what a running flow posted. Nothing here mutates anything the
// flow goroutine can see: it only reads the message it was handed.
func (m Model) applyFlow(msg tea.Msg) (Model, tea.Cmd) {
	switch msg := msg.(type) {
	case promptMsg:
		return m.openModal(msg)
	case OutputLineMsg:
		return m.appendOutput(msg), nil
	case opTargetMsg:
		m.ops = m.ops.retarget(msg.id, msg.target)
		return m, nil
	case createdMsg:
		m.selectBranch = msg.branch
		return m, m.loadWorktreesCmd(false)
	case cleanedMsg:
		return m, m.loadWorktreesCmd(false)
	}
	return m, nil
}

// openModal refuses a second question rather than stacking it: two modals would
// leave the user answering one flow while another waits behind it, unseen.
func (m Model) openModal(msg promptMsg) (Model, tea.Cmd) {
	if m.modal.open {
		return m, replyCmd(msg.reply, promptReply{err: domain.ErrUserAborted})
	}
	modal, cmd := newModal(modalParams{
		Title:   msg.title,
		Shape:   msg.shape,
		Session: msg.session,
		Reply:   msg.reply,
		Width:   m.width,
		Height:  m.height,
	})
	m.modal = modal
	return m, cmd
}

func (m Model) appendOutput(msg OutputLineMsg) Model {
	m.outputLines = append(append([]string(nil), m.outputLines...), msg.Text)
	m.outputOffset = max(len(m.outputLines)-m.layout().OutputLines, 0)
	return m
}
