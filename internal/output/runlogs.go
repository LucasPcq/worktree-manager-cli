package output

import (
	"fmt"
	"io"
	"strings"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/flow/runlogs"
	"github.com/LucasPcq/wtm/internal/rules"
	"github.com/LucasPcq/wtm/internal/styles"
)

type RunPrinterParams struct {
	Out io.Writer
	Err io.Writer
	// Profile names the run being reported. Empty prints no heading: a run.toml
	// with no profile has nothing to name.
	Profile string
	// Hyperlinks turns a job's URL into an OSC-8 link. Off for a pipe, a JSON
	// run, or anything that would only show the escape sequence.
	Hyperlinks bool
}

// RunPrinter renders a profile's start sequence as lines on the terminal the
// command was launched from. It writes a raw body: the command's frame owns the
// outer padding, and the blank line between two steps is this printer's.
type RunPrinter struct {
	out        io.Writer
	err        io.Writer
	profile    string
	hyperlinks bool
	printed    bool
}

func NewRunPrinter(params RunPrinterParams) *RunPrinter {
	return &RunPrinter{
		out:        params.Out,
		err:        params.Err,
		profile:    params.Profile,
		hyperlinks: params.Hyperlinks,
	}
}

func (p *RunPrinter) Emit(event runlogs.Event) {
	switch event.Phase {
	case runlogs.PhaseStarting:
		if p.printed {
			Blank(p.out)
		}
		if !p.printed && p.profile != "" {
			Message(p.out, styles.Bold.Render(fmt.Sprintf(domain.RunStreamProfileFmt, p.profile)))
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
		Success(p.out, p.jobLine(jobLineParams{Format: domain.RunStreamStartedFmt, Event: event}))
		p.devOrigins(event.DevOrigins)
	case runlogs.PhaseDone:
		Success(p.out, p.jobLine(jobLineParams{Format: domain.RunStreamDoneFmt, Event: event}))
	case runlogs.PhaseFailed:
		Error(p.err, event.Reason)
	case runlogs.PhaseProbed:
		p.probed(event.Probes)
	case runlogs.PhaseAborted:
		p.aborted(event.Outcome)
	case runlogs.PhaseReady:
		p.ready(event.Outcome)
	}
}

type jobLineParams struct {
	Format string
	Event  runlogs.Event
}

func (p *RunPrinter) jobLine(params jobLineParams) string {
	line := rules.LabelWithPorts(rules.LabelWithPortsParams{
		Label: fmt.Sprintf(params.Format, params.Event.Job),
		Ports: params.Event.Ports,
	})
	if params.Event.URL == "" {
		return line
	}
	return line + domain.RunURLSuffixSep + Hyperlink(HyperlinkParams{
		Text:    params.Event.URL,
		URL:     params.Event.URL,
		Enabled: p.hyperlinks,
	})
}

// devOrigins reports the one line a Next project is missing before its own name
// reaches it. Rendered where the ports report is, for the same reason: it is a
// finding about a job that started fine, not a failure.
func (p *RunPrinter) devOrigins(fixes []domain.DevOriginFix) {
	if len(fixes) == 0 {
		return
	}
	lines := make([]string, 0, len(fixes))
	for _, fix := range fixes {
		lines = append(lines, fix.Line)
	}
	Blank(p.err)
	Callout(p.err, domain.DevOriginsTitle, lines)
}

// probed reports only what the check could not confirm: a port that answered
// says nothing the "started" line did not already say.
func (p *RunPrinter) probed(probes []domain.PortProbe) {
	lines := rules.PortProbeLines(probes)
	if len(lines) == 0 {
		return
	}
	Blank(p.err)
	Callout(p.err, domain.PortProbeTitle, lines)
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

	probes := map[string][]domain.PortProbe{}
	for _, probe := range outcome.Probes {
		probes[probe.Job] = append(probes[probe.Job], probe)
	}

	for i := range results {
		results[i].Ports = probes[results[i].Name]
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
