package run

import (
	"context"
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/flow/runlogs"
	"github.com/LucasPcq/wtm/internal/output"
	"github.com/LucasPcq/wtm/internal/service/process"
	"github.com/LucasPcq/wtm/internal/tui/runview"
)

// isTTY reports whether this command owns a terminal. It is a variable so a
// test can answer yes: which surface a run gets hangs on it, and nothing else
// makes a pipe look like a terminal.
var isTTY = func() bool { return term.IsTerminal(int(os.Stdin.Fd())) }

// showRunView is the full-screen surface, a variable for the same reason: a
// test can watch a command hand over to it without bubbletea taking over a
// terminal the test does not have.
var showRunView = openRunView

type runSeamParams struct {
	StateDir string
	Dir      string
	// Jobs are the worktree's declared jobs, which the view lists whether or not
	// this run touches them.
	Jobs []domain.JobConfig
}

// runSeam is this command's end of internal/flow/runlogs: the daemon's view of
// the worktree's jobs and the start sequence a surface drives. Opening it
// assumes the daemon is already up — the caller is what made sure of that.
type runSeam struct {
	service runlogs.Service
	session runlogs.Session
	workDir string
	logDir  string
}

func openRunSeam(params runSeamParams) runSeam {
	logDir := jobLogDir(jobLogDirParams{StateDir: params.StateDir, Dir: params.Dir})
	service := runlogs.NewService(runlogs.ServiceParams{SocketPath: process.SocketPath()})
	return runSeam{
		service: service,
		session: runlogs.NewSession(runlogs.SessionParams{
			Service: service,
			Jobs:    params.Jobs,
			WorkDir: params.Dir,
			LogDir:  logDir,
		}),
		workDir: params.Dir,
		logDir:  logDir,
	}
}

// starter is the start sequence as a surface drives it. Cancelling the context
// it is called with ends the reporting, never the jobs: a view the reader
// walked away from stops being written to while the daemon keeps running what
// it started.
func (s runSeam) starter(jobs []domain.JobConfig) runview.StartFunc {
	return func(ctx context.Context, sink runlogs.Sink) (runlogs.Outcome, error) {
		return runlogs.Run(ctx, runlogs.RunParams{
			Service: s.service,
			Sink:    sink,
			Jobs:    jobs,
			WorkDir: s.workDir,
			LogDir:  s.logDir,
		})
	}
}

type viewParams struct {
	Cmd     *cobra.Command
	Session runlogs.Session
	Job     string
	Start   runview.StartFunc
}

// openRunView hands the terminal to the full-screen view and frames what it
// leaves behind. The view returns its recap rather than printing it: the
// alternate screen has to be given back before anything is written to the
// scrollback underneath it.
func openRunView(params viewParams) error {
	result, err := runview.Run(runview.Params{
		Session: params.Session,
		Job:     params.Job,
		Start:   params.Start,
	})
	if err != nil {
		return err
	}

	if result.Recap != "" {
		out := params.Cmd.OutOrStdout()
		output.Frame(out, func() { fmt.Fprintln(out, result.Recap) })
	}
	if result.Outcome.Aborted() {
		return domain.ErrAborted
	}
	return nil
}

type streamParams struct {
	Cmd   *cobra.Command
	Start runview.StartFunc
}

// runOnStream reports a start sequence as lines on the terminal it was launched
// from — `-d`, a pipe, a CI job. An aborted run ends on stderr, which is where
// its frame closes.
func runOnStream(params streamParams) error {
	out, errOut := params.Cmd.OutOrStdout(), params.Cmd.ErrOrStderr()

	output.FrameStart(out)
	outcome, err := params.Start(params.Cmd.Context(), output.NewRunPrinter(output.RunPrinterParams{Out: out, Err: errOut}))
	if err != nil {
		return err
	}

	if outcome.Aborted() {
		output.FrameEnd(errOut)
		return domain.ErrAborted
	}
	output.FrameEnd(out)
	return nil
}

// runForMachine emits the run's outcome as a JSON document and exits zero even
// when the profile aborted: the failure is a result entry, and a half-written
// document would be worse than an exit code for whoever is parsing it.
func runForMachine(params streamParams) error {
	outcome, err := params.Start(params.Cmd.Context(), nil)
	if err != nil {
		return err
	}
	return output.WriteRunOutcomeJSON(params.Cmd.OutOrStdout(), outcome)
}
