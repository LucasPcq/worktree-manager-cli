package run

import (
	"context"
	"fmt"
	"io"
	"os"

	"golang.org/x/term"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/flow/runlogs"
	"github.com/LucasPcq/wtm/internal/output"
	"github.com/LucasPcq/wtm/internal/rules"
	"github.com/LucasPcq/wtm/internal/tui/runview"
)

// surfaceParams is what a run command hands to the surface it picked: the seam
// to the daemon, the jobs the surface lists, and the ones it starts.
type surfaceParams struct {
	Out    io.Writer
	Err    io.Writer
	Format string

	Service runlogs.Service
	// Declared are the jobs the surface lists, Start the ones it runs a start
	// sequence on. A surface that only reads what is already running starts none.
	Declared []domain.JobConfig
	Start    []domain.JobConfig
	// Focus is the job the view opens on.
	Focus   string
	WorkDir string
	LogDir  string
}

type wantsRunViewParams struct {
	Format string
	// Detach is the -d flag: the caller asking for their terminal back.
	Detach bool
	// Inline is a job that owns the terminal while it runs — see rules.RunsInline.
	Inline bool
}

// wantsRunView adds the terminal to what the caller asked for; the choice
// itself is the rule's.
func wantsRunView(params wantsRunViewParams) bool {
	return rules.UseRunView(rules.RunSurfaceParams{
		Format: params.Format,
		TTY:    term.IsTerminal(int(os.Stdin.Fd())),
		Detach: params.Detach,
		Inline: params.Inline,
	})
}

// startInView runs a start sequence inside the full-screen view. The view is
// opened before the first job so everything the run prints lands in a pane, and
// the recap it leaves behind is framed here on the terminal it gives back.
func startInView(params surfaceParams) (runlogs.Outcome, error) {
	result, err := runview.Run(runview.Params{
		Session: newSession(params),
		Job:     params.Focus,
		Start:   startFunc(params),
	})
	if err != nil {
		return runlogs.Outcome{}, err
	}
	if result.Recap != "" {
		output.Frame(params.Out, func() { fmt.Fprintln(params.Out, result.Recap) })
	}
	return result.Outcome, nil
}

// startInline runs a start sequence on the terminal the caller keeps: each job
// says where it stands, and the ones that print on their way up print here. A
// machine-readable run installs no sink at all — its whole answer is the
// document the command writes from the Outcome.
func startInline(params surfaceParams) (runlogs.Outcome, error) {
	var sink runlogs.Sink
	if rules.IsHumanFormat(params.Format) {
		sink = newTextSink(params)
	}
	return runlogs.Run(context.Background(), runParams(params, sink))
}

func startFunc(params surfaceParams) runview.StartFunc {
	if len(params.Start) == 0 {
		return nil
	}
	return func(ctx context.Context, sink runlogs.Sink) (runlogs.Outcome, error) {
		return runlogs.Run(ctx, runParams(params, sink))
	}
}

func runParams(params surfaceParams, sink runlogs.Sink) runlogs.RunParams {
	return runlogs.RunParams{
		Service: params.Service,
		Sink:    sink,
		Jobs:    params.Start,
		WorkDir: params.WorkDir,
		LogDir:  params.LogDir,
	}
}

func newSession(params surfaceParams) runlogs.Session {
	return runlogs.NewSession(runlogs.SessionParams{
		Service: params.Service,
		Jobs:    params.Declared,
		WorkDir: params.WorkDir,
		LogDir:  params.LogDir,
	})
}

// textSink is the CLI half of the run seam: what a start sequence looks like
// when it is read as it scrolls by.
type textSink struct {
	out  io.Writer
	err  io.Writer
	jobs map[string]domain.JobConfig
}

func newTextSink(params surfaceParams) textSink {
	jobs := make(map[string]domain.JobConfig, len(params.Start))
	for _, job := range params.Start {
		jobs[job.Name] = job
	}
	return textSink{out: params.Out, err: params.Err, jobs: jobs}
}

func (s textSink) Emit(event runlogs.Event) {
	switch event.Phase {
	case runlogs.PhaseStarting:
		s.announce(event.Job)
	case runlogs.PhaseOutput:
		_, _ = s.out.Write(event.Chunk)
	case runlogs.PhaseDone:
		output.Success(s.out, fmt.Sprintf(domain.RunJobDoneFmt, event.Job))
	case runlogs.PhaseStarted:
		s.started(event)
	case runlogs.PhaseFailed:
		output.Error(s.err, event.Reason)
	}
}

// announce heads the output a job is about to print. A job the daemon
// backgrounds prints nothing, so it is announced by the line that follows it.
func (s textSink) announce(name string) {
	job, declared := s.jobs[name]
	if !declared || !rules.StreamsStartOutput(job) {
		return
	}
	output.Blank(s.out)
	if rules.RunsInline(job) {
		output.Loading(s.out, fmt.Sprintf(domain.RunTaskRunningFmt, name))
		return
	}
	output.Loading(s.out, fmt.Sprintf(domain.RunJobStartingFmt, name))
}

func (s textSink) started(event runlogs.Event) {
	if event.AlreadyRunning {
		output.Success(s.out, fmt.Sprintf(domain.RunJobAlreadyRunningFmt, event.Job))
		return
	}
	output.Success(s.out, fmt.Sprintf(domain.RunJobStartedFmt, event.Job))
}

type failureParams struct {
	Outcome runlogs.Outcome
	// Output is what the failed job had printed, for a reader that never saw it
	// live; nil for one that did.
	Output []byte
}

// failureReason is what the run answered for the job that ended it.
func failureReason(params failureParams) string {
	results := rules.WithFailureOutput(rules.FailureOutputParams{
		Results: params.Outcome.Results,
		Job:     params.Outcome.Failed,
		Output:  params.Output,
	})
	for _, result := range results {
		if result.Name == params.Outcome.Failed && result.Status == domain.JobActionError {
			return result.Message
		}
	}
	return params.Outcome.Failed
}
