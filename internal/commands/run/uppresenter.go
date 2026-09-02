package run

import (
	"fmt"

	"github.com/LucasPcq/wtm/internal/commands/shared"
	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/flow/run/seam"
	stopflow "github.com/LucasPcq/wtm/internal/flow/run/stop"
	upflow "github.com/LucasPcq/wtm/internal/flow/run/up"
	"github.com/LucasPcq/wtm/internal/flow/runlogs"
	"github.com/LucasPcq/wtm/internal/output"
	"github.com/LucasPcq/wtm/internal/rules"
)

// upPresenter is the CLI half of the up flow: the flow decides what to start,
// this decides where it is watched — the full-screen view, a stream of lines,
// or a JSON document.
type upPresenter struct {
	shared.CLIPresenter
	detach bool
}

func (p upPresenter) Sequence(params seam.SequenceParams) (runlogs.Outcome, error) {
	switch rules.DecideRunSurface(rules.RunSurfaceParams{Detach: p.detach, TTY: isTTY(), Format: p.Format}) {
	case domain.RunSurfaceView:
		return showRunView(viewParams{
			Cmd:     p.Cmd,
			Board:   params.Board,
			Profile: params.Profile,
			Start:   params.Start,
		})
	case domain.RunSurfaceMachine:
		return runForMachine(streamParams{Cmd: p.Cmd, Start: params.Start})
	default:
		return runOnStream(streamParams{
			Cmd:        p.Cmd,
			Profile:    params.Profile,
			Start:      params.Start,
			Hyperlinks: p.Human && isTTY(),
		})
	}
}

// concluded is what the runner does with an outcome the surface has already
// shown: nothing but the exit code. Every surface has named the jobs itself,
// so an error here would only repeat them (LUC-198).
func concluded(outcome upflow.Outcome) error {
	if outcome.Aborted {
		return domain.ErrAborted
	}
	return nil
}

// startPresenter is `run start`'s half: the same three surfaces as `run up`,
// because starting one job is the same sequence over a list of one. Only the
// machine surface differs — LUC-198 froze its shape as an object, since the
// command acts on exactly one job.
type startPresenter struct {
	shared.CLIPresenter
	detach bool
}

func (p startPresenter) Sequence(params seam.SequenceParams) (runlogs.Outcome, error) {
	surface := rules.DecideRunSurface(rules.RunSurfaceParams{
		Inline: params.Inline,
		Detach: p.detach,
		TTY:    isTTY(),
		Format: p.Format,
	})
	switch surface {
	case domain.RunSurfaceView:
		return showRunView(viewParams{Cmd: p.Cmd, Board: params.Board, Job: params.Job, Start: params.Start})
	case domain.RunSurfaceMachine:
		return p.machine(params)
	default:
		return runOnStream(streamParams{Cmd: p.Cmd, Start: params.Start, Hyperlinks: p.Human && isTTY()})
	}
}

// machine answers with the one job's result, whether or not it worked: the
// module's rule is that the shape follows the arity and the exit code follows
// the success (LUC-198). A failed job writing nothing at all left a machine
// reader with an exit code and no cause, which is exactly what the `output`
// field of `run up`'s array exists to avoid.
func (p startPresenter) machine(params seam.SequenceParams) (runlogs.Outcome, error) {
	outcome, err := params.Start(p.Cmd.Context(), nil)
	if err != nil {
		return outcome, err
	}
	return outcome, output.WriteJobResultJSON(p.Cmd.OutOrStdout(), jobResult(jobResultParams{
		Job:     params.Job,
		Inline:  params.Inline,
		Outcome: outcome,
	}))
}

type jobResultParams struct {
	Job     string
	Inline  bool
	Outcome runlogs.Outcome
}

// jobResult is the entry the sequence already recorded for this job — the very
// one `run up` would publish for it — so the two commands say the same thing
// about the same job rather than agreeing by coincidence. Nothing recorded means
// the run never reached it, which only a start that failed outright can produce.
func jobResult(params jobResultParams) domain.JobActionResult {
	for _, result := range output.RunOutcomeResults(params.Outcome) {
		if result.Name == params.Job {
			return result
		}
	}
	if params.Inline {
		return domain.JobActionResult{Name: params.Job, Status: domain.JobActionDone}
	}
	return domain.JobActionResult{Name: params.Job, Status: domain.JobActionStarted}
}

// stopPresenter reports the one job `run stop` acted on.
type stopPresenter struct {
	shared.CLIPresenter
}

func (p stopPresenter) Stopped(outcome stopflow.Outcome) error {
	if p.Format == domain.OutputJSON {
		// An object like every other answer this command gives: the shape follows
		// the arity of the command, never the branch it happened to take
		// (LUC-198). Nothing was running, so the job is stopped either way.
		return output.WriteJobResultJSON(p.Cmd.OutOrStdout(), domain.JobActionResult{
			Name:   outcome.Job,
			Status: domain.JobActionStopped,
		})
	}
	out := p.Cmd.OutOrStdout()
	if outcome.NoDaemon {
		output.Frame(out, func() { output.Message(out, domain.RunNoJobsRunning) })
		return nil
	}
	output.Frame(out, func() { output.Success(out, fmt.Sprintf(domain.RunStoppedFmt, outcome.Job)) })
	return nil
}
