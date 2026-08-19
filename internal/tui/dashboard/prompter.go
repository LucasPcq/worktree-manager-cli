package dashboard

import (
	tea "github.com/charmbracelet/bubbletea"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/flow"
)

// flowMsg wraps what a running flow sends the dashboard. Everything the flow
// goroutine produces travels this way: it never touches the model itself.
type flowMsg struct{ inner tea.Msg }

// promptMsg hands a whole session to the dashboard and blocks the flow on reply
// until the modal has been through it.
type promptMsg struct {
	title   string
	shape   modalShape
	session flow.Session
	reply   chan<- promptReply
}

type opDoneMsg struct {
	id  int
	err error
}

// opTargetMsg names the worktree a run holds, as soon as its answers say which
// one it is.
type opTargetMsg struct {
	id     int
	target string
}

type createdMsg struct{ branch string }

type cleanedMsg struct{ branch string }

// reparentedMsg says the parent metadata changed, so the rows showing "from <x>"
// have to be re-read.
type reparentedMsg struct{}

// prunedMsg says several worktrees are gone at once, so the whole list has to be
// re-read rather than one row dropped.
type prunedMsg struct{}

// listenCmd delivers the next message a flow goroutine posted. Update re-arms it
// on every flowMsg, so there is exactly one reader on the channel at all times.
func listenCmd(msgs <-chan tea.Msg) tea.Cmd {
	return func() tea.Msg { return flowMsg{inner: <-msgs} }
}

// prompter is the dashboard half of flow.Prompter: it turns a session into a
// modal and waits, on the flow's goroutine, for what the user answered.
type prompter struct {
	send  func(tea.Msg)
	title string
	shape modalShape
	// opID and targetKey let the answers name the worktree the run holds, without
	// the dashboard having to know what the flow asks.
	opID      int
	targetKey string
}

func (p prompter) Interactive() bool { return true }

func (p prompter) Ask(session flow.Session) (flow.Answers, error) {
	reply := make(chan promptReply, 1)
	p.send(promptMsg{title: p.title, shape: p.shape, session: session, reply: reply})
	answered := <-reply
	if answered.err == nil && p.targetKey != "" {
		p.send(opTargetMsg{id: p.opID, target: answered.answers.Value(p.targetKey)})
	}
	return answered.answers, answered.err
}

// Confirm is a standalone decision, asked as a one-question form: a flow reaches
// it after an execution, when there is no session left to join.
func (p prompter) Confirm(params flow.ConfirmParams) (bool, error) {
	session := flow.Session{Steps: []flow.Step{{
		Kind:        flow.StepRecap,
		Key:         keyConfirm,
		Title:       params.Title,
		Description: confirmDescription(params),
		Options:     confirmOptions(params),
	}}}

	answers, err := prompter{send: p.send, title: params.Title, shape: modalForm}.Ask(session)
	if err != nil {
		return false, err
	}
	return answers.Value(keyConfirm) == confirmYes, nil
}

// confirmOptions names both outcomes when the caller named them: closing the
// modal is a way out, not an answer, so a two-outcome decision has to offer both.
func confirmOptions(params flow.ConfirmParams) []flow.Option {
	if params.YesLabel == "" {
		return []flow.Option{{Label: domain.DashboardConfirmLabel, Value: confirmYes}}
	}
	return []flow.Option{
		{Label: params.NoLabel, Value: confirmNo},
		{Separator: true},
		{Label: params.YesLabel, Value: confirmYes},
	}
}

const (
	keyConfirm = "dashboard.confirm"
	confirmYes = "yes"
	confirmNo  = "no"
)

func confirmDescription(params flow.ConfirmParams) string {
	if params.Warning == "" {
		return params.Description
	}
	if params.Description == "" {
		return params.Warning
	}
	return params.Description + "\n" + params.Warning
}
