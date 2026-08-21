package output

import (
	"fmt"
	"io"
	"strings"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/flow/runlogs"
	"github.com/LucasPcq/wtm/internal/rules"
)

type RunPrinterParams struct {
	Out io.Writer
	Err io.Writer
}

// RunPrinter renders a profile's start sequence as lines on the terminal the
// command was launched from. It writes a raw body: the command's frame owns the
// outer padding, and the blank line between two steps is this printer's.
type RunPrinter struct {
	out     io.Writer
	err     io.Writer
	printed bool
}

func NewRunPrinter(params RunPrinterParams) *RunPrinter {
	return &RunPrinter{out: params.Out, err: params.Err}
}

func (p *RunPrinter) Emit(event runlogs.Event) {
	switch event.Phase {
	case runlogs.PhaseStarting:
		if p.printed {
			Blank(p.out)
		}
		p.printed = true
		Loading(p.out, fmt.Sprintf(domain.RunStreamStepFmt, event.Step, event.Steps, event.Job))
	case runlogs.PhaseOutput:
		_, _ = p.out.Write(event.Chunk)
	case runlogs.PhaseStarted:
		if event.AlreadyRunning {
			Success(p.out, fmt.Sprintf(domain.RunStreamAlreadyFmt, event.Job))
			return
		}
		Success(p.out, rules.LabelWithPorts(rules.LabelWithPortsParams{
			Label: fmt.Sprintf(domain.RunStreamStartedFmt, event.Job),
			Ports: event.Ports,
		}))
	case runlogs.PhaseDone:
		Success(p.out, rules.LabelWithPorts(rules.LabelWithPortsParams{
			Label: fmt.Sprintf(domain.RunStreamDoneFmt, event.Job),
			Ports: event.Ports,
		}))
	case runlogs.PhaseFailed:
		Error(p.err, event.Reason)
	case runlogs.PhaseAborted:
		p.aborted(event.Outcome)
	case runlogs.PhaseReady:
		p.ready(event.Outcome)
	}
}

// aborted reports the partial state a failed job left behind: where the profile
// stopped, what nothing tore down, and what it never reached. The job's own
// output already streamed past — this says what is left, not why.
func (p *RunPrinter) aborted(outcome runlogs.Outcome) {
	Blank(p.err)
	Warning(p.err, fmt.Sprintf(domain.RunAbortStepFmt, outcome.FailedStep, outcome.Steps, outcome.Failed))

	if len(outcome.Started) > 0 {
		InfoLine(p.err, domain.RunAbortRunningLabel, joinJobNames(outcome.Started))
	}
	if len(outcome.NotStarted) > 0 {
		InfoLine(p.err, domain.RunAbortNotStartedLabel, joinJobNames(outcome.NotStarted))
	}

	Blank(p.err)
	Loading(p.err, domain.RunAbortHint)
}

func (p *RunPrinter) ready(outcome runlogs.Outcome) {
	if len(outcome.Started) == 0 {
		return
	}
	Blank(p.out)
	Loading(p.out, domain.RunStreamNextHint)
}

// WriteRunOutcomeJSON writes what a run did as the array of job results every
// `run` command emits, with one addition on the job that ended it: the output it
// had written and the code it exited with. A caller reading JSON never saw the
// live stream, and the daemon's message alone ("task migrate failed: exit status
// 1") does not say why.
func WriteRunOutcomeJSON(w io.Writer, outcome runlogs.Outcome) error {
	results := make([]domain.JobActionResult, len(outcome.Results))
	copy(results, outcome.Results)

	for i := range results {
		if results[i].Name != outcome.Failed || results[i].Status != domain.JobActionError {
			continue
		}
		results[i].Output = string(outcome.FailedOutput)
		results[i].ExitCode = outcome.FailedExitCode
	}

	return WriteJobResultsJSON(w, results)
}

func joinJobNames(names []string) string {
	return strings.Join(names, domain.RunViewRecapListSep)
}
