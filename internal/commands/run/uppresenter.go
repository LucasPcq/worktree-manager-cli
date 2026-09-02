package run

import (
	"github.com/LucasPcq/wtm/internal/commands/shared"
	"github.com/LucasPcq/wtm/internal/domain"
	upflow "github.com/LucasPcq/wtm/internal/flow/run/up"
	"github.com/LucasPcq/wtm/internal/flow/runlogs"
	"github.com/LucasPcq/wtm/internal/rules"
)

// upPresenter is the CLI half of the up flow: the flow decides what to start,
// this decides where it is watched — the full-screen view, a stream of lines,
// or a JSON document.
type upPresenter struct {
	shared.CLIPresenter
	detach bool
}

func (p upPresenter) Sequence(params upflow.SequenceParams) (runlogs.Outcome, error) {
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
