package dashboard

import (
	"context"
	"fmt"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/flow/run/seam"
	"github.com/LucasPcq/wtm/internal/flow/runlogs"
)

// detachedWatcher drives a start sequence without asking for the terminal. The
// dashboard has a list one wants to come back over, so a start there gives the
// surface back and reports; watching a run is what the logs view is for.
type detachedWatcher struct {
	send func(tea.Msg)
	id   int
}

func (w detachedWatcher) Sequence(params seam.SequenceParams) (runlogs.Outcome, error) {
	if params.Start == nil {
		return runlogs.Outcome{}, nil
	}
	// A conversion, not a literal: the sink needs exactly what the watcher holds,
	// and the two must stay that way — a field added to one has to be answered
	// on the other.
	return params.Start(context.Background(), detachedSink(w))
}

// detachedSink turns the sequence's events into output-panel lines and into the
// stage the held row shows beside its spinner. PhaseOutput, PhaseAborted and
// PhaseProbed say nothing: a job's raw bytes belong to the logs view, and an
// abort is already carried by the Outcome Sequence returns.
type detachedSink struct {
	send func(tea.Msg)
	id   int
}

func (s detachedSink) Emit(event runlogs.Event) {
	switch event.Phase {
	case runlogs.PhaseStarting:
		stage := fmt.Sprintf(domain.RunDetachedStartingFmt, event.Job, event.Step, event.Steps)
		s.send(opStageMsg{id: s.id, stage: stage})
		s.line(stage)
	case runlogs.PhaseStarted:
		if event.AlreadyRunning {
			s.line(fmt.Sprintf(domain.RunDetachedAlreadyFmt, event.Job))
			return
		}
		s.line(fmt.Sprintf(domain.RunDetachedStartedFmt, event.Job))
		if event.URL != "" {
			s.line(fmt.Sprintf(domain.RunDetachedAddressFmt, event.Job, event.URL))
		}
	// A task that ran to its end is done, not up: PhaseDone concludes a task,
	// where PhaseStarted announces a service.
	case runlogs.PhaseDone:
		s.line(fmt.Sprintf(domain.RunDetachedDoneFmt, event.Job))
	case runlogs.PhaseFailed:
		s.line(fmt.Sprintf(domain.RunDetachedFailedFmt, event.Job, event.Reason))
	case runlogs.PhaseNotice:
		s.line(event.Notice)
	}
}

func (s detachedSink) line(text string) {
	if text == "" {
		return
	}
	s.send(OutputLineMsg{Text: text})
}
