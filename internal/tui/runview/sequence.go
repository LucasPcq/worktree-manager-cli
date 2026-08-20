package runview

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/flow/runlogs"
)

// sequence is what the view knows of a profile being started. It decides
// nothing about the run — the order, what aborts it and what is left running
// are runlogs' answers, carried here as they are emitted.
type sequence struct {
	active bool
	job    string
	step   int
	steps  int
	states map[string]domain.JobStep
	// reason is what the daemon answered for the job that ended the sequence.
	reason  string
	outcome runlogs.Outcome
}

type eventMsg struct{ event runlogs.Event }

type runFinishedMsg struct {
	outcome runlogs.Outcome
	err     error
}

// sink carries a run's events to the model. A job's output never goes through
// the channel: it is written into the pane on this goroutine as it arrives, and
// the model's clock is what puts it on screen.
type sink struct {
	panes *paneStore
	msgs  chan<- tea.Msg
	done  <-chan struct{}
}

func (s sink) Emit(event runlogs.Event) {
	if event.Phase == runlogs.PhaseOutput {
		s.panes.write(writeChunkParams{Job: event.Job, Source: sourceSequence, Chunk: event.Chunk})
		return
	}
	select {
	case s.msgs <- eventMsg{event: event}:
	case <-s.done:
	}
}

func (m Model) applyEvent(msg eventMsg) (Model, tea.Cmd) {
	event := msg.event
	noticeLines := len(m.report())
	m.sequence.steps = max(event.Steps, m.sequence.steps)

	switch event.Phase {
	case runlogs.PhaseStarting:
		m.sequence.active = true
		m.sequence.job, m.sequence.step = event.Job, event.Step
		m.sequence.states[event.Job] = domain.JobStepStarting
	case runlogs.PhaseStarted:
		m.sequence.states[event.Job], m.sequence.job = domain.JobStepStarted, ""
	case runlogs.PhaseDone:
		m.sequence.states[event.Job], m.sequence.job = domain.JobStepDone, ""
	case runlogs.PhaseFailed:
		m.sequence.states[event.Job], m.sequence.job = domain.JobStepFailed, ""
		m.sequence.reason = event.Reason
	case runlogs.PhaseAborted, runlogs.PhaseReady:
		m.sequence.active, m.sequence.job = false, ""
		m.sequence.outcome = event.Outcome
	}

	model, cmd := m.followSequence(event)
	model, resized := model.resyncSize(noticeLines)

	// The clock is the only thing that draws a pane the run writes into: a
	// PhaseOutput never reaches the model, and a tick that landed before the
	// first phase found nothing scheduled and stopped.
	var tick tea.Cmd
	if model.sequence.active {
		model, tick = model.startTicking()
	}
	return model, tea.Batch(cmd, resized, tick, model.refreshCmd(), model.listenCmd())
}

// followSequence puts the job the sequence is acting on in front of the reader,
// until the reader takes the cursor themselves.
func (m Model) followSequence(event runlogs.Event) (Model, tea.Cmd) {
	if !m.following || event.Job == "" || event.Job == m.selected {
		return m.fillSelectedPane()
	}
	return m.setSelection(event.Job)
}

// applyRunFinished records what the run ended with. The error is the run's own,
// not a job's: a sequence that was detached from ended on the context, which is
// the view being left rather than anything to report.
func (m Model) applyRunFinished(msg runFinishedMsg) (Model, tea.Cmd) {
	noticeLines := len(m.report())
	m.sequence.active = false
	if msg.outcome.Recorded() {
		m.sequence.outcome = msg.outcome
	}
	if msg.err != nil && m.runCtx.Err() == nil {
		m.err = msg.err
	}
	model, resized := m.resyncSize(noticeLines)
	return model, tea.Batch(resized, model.refreshCmd())
}

// report is what an aborted profile has to say, one line per thing it says.
// Building it from the outcome rather than from a running tally is what keeps
// it true after a detach: the outcome is the run's own account of itself.
func (m Model) report() []string {
	outcome := m.sequence.outcome
	if m.dismissed || !outcome.Aborted() {
		return nil
	}

	lines := []string{
		domain.RunViewAbortTitle,
		fmt.Sprintf(domain.RunViewAbortFailedFmt, outcome.FailedStep, outcome.Steps, outcome.Failed, m.sequence.reason),
	}
	if len(outcome.Started) > 0 {
		lines = append(lines, fmt.Sprintf(domain.RunViewAbortRunningFmt, joinJobs(outcome.Started)))
	}
	if len(outcome.NotStarted) > 0 {
		lines = append(lines, fmt.Sprintf(domain.RunViewAbortNotStartedFmt, joinJobs(outcome.NotStarted)))
	}
	return append(lines, domain.RunViewAbortDismiss)
}

func joinJobs(jobs []string) string {
	return strings.Join(jobs, domain.RunViewRecapListSep)
}
