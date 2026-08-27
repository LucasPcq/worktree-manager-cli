package runview

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/flow/runlogs"
	"github.com/LucasPcq/wtm/internal/rules"
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
	// ports is what each job bound as it started, kept for the pane title: the
	// run is the only moment the daemon reports them.
	ports map[string]map[string]int
	// urls is where each job answers, kept for the same reason as ports: the run
	// is the only moment it is reported.
	urls map[string]string
	// devOrigins are the config lines the started jobs are missing, collected as
	// they are reported so the recap can name them all at once.
	devOrigins []domain.DevOriginFix
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
		s.panes.write(writeChunkParams{
			Job:          event.Job,
			Source:       sourceSequence,
			Chunk:        event.Chunk,
			NormalizeEOL: rules.RunsOnPipe(event.Kind),
		})
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
		m.sequence.remember(event)
	case runlogs.PhaseDone:
		m.sequence.states[event.Job], m.sequence.job = domain.JobStepDone, ""
		m.sequence.remember(event)
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

// report is the notice area: what the run has to say once it has stopped
// moving. An abort outranks a silent port — a profile that never finished is
// the bigger news, and both at once would bury it. It is built from the outcome
// rather than from a running tally, which is what keeps it true after a detach.
func (m Model) report() []string {
	outcome := m.sequence.outcome
	if m.dismissed {
		return nil
	}
	if !outcome.Aborted() {
		return m.probeReport()
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

// probeReport names the declared ports nothing answered on, once the sequence
// is over. A run still starting has nothing to conclude yet.
func (m Model) probeReport() []string {
	if m.sequence.active || !m.sequence.outcome.Recorded() {
		return nil
	}
	lines := rules.PortProbeLines(m.sequence.outcome.Probes)
	if len(lines) == 0 {
		return m.devOriginsReport()
	}
	return append(append([]string{domain.PortProbeTitle}, lines...), domain.RunViewAbortDismiss)
}

// devOriginsReport names the Next projects that will refuse their own name. It
// yields to the port report above for the same reason that one yields to an
// abort: one notice area, the more urgent finding first.
func (m Model) devOriginsReport() []string {
	if len(m.sequence.devOrigins) == 0 {
		return nil
	}
	lines := []string{domain.DevOriginsTitle}
	for _, fix := range m.sequence.devOrigins {
		lines = append(lines, fix.Line)
	}
	return append(lines, domain.RunViewAbortDismiss)
}

func joinJobs(jobs []string) string {
	return strings.Join(jobs, domain.RunViewRecapListSep)
}

func (s *sequence) remember(event runlogs.Event) {
	if len(event.Ports) > 0 {
		if s.ports == nil {
			s.ports = map[string]map[string]int{}
		}
		s.ports[event.Job] = event.Ports
	}
	s.devOrigins = append(s.devOrigins, event.DevOrigins...)

	if event.URL == "" {
		return
	}
	if s.urls == nil {
		s.urls = map[string]string{}
	}
	s.urls[event.Job] = event.URL
}
