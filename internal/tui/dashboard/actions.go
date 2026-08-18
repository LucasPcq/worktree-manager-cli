package dashboard

import (
	"errors"
	"fmt"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/flow"
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
	declared := createflow.Operation()
	m, id := m.beginOp(declared)
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

// beginOp records the run and opens the output panel: a run whose output is
// folded away is one the user cannot follow.
func (m Model) beginOp(declared flow.Operation) (Model, int) {
	ops, id := m.ops.begin(operation{kind: declared.Kind, mode: declared.Mode})
	m.ops = ops
	m.outputExpanded = true
	return m.reflow(), id
}

func (m Model) finishOp(msg opDoneMsg) Model {
	m.ops = m.ops.end(msg.id)
	if msg.err == nil || errors.Is(msg.err, domain.ErrUserAborted) {
		return m
	}
	return m.appendOutput(OutputLineMsg{
		Text: fmt.Sprintf(domain.DashboardFailedFmt, domain.DashboardOperationLabel, msg.err),
	})
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
