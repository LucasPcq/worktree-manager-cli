package dashboard

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/flow"
	cleanflow "github.com/LucasPcq/wtm/internal/flow/clean"
	createflow "github.com/LucasPcq/wtm/internal/flow/create"
	pruneflow "github.com/LucasPcq/wtm/internal/flow/prune"
	reparentflow "github.com/LucasPcq/wtm/internal/flow/reparent"
	"github.com/LucasPcq/wtm/internal/rules"
)

// presenter is the dashboard half of flow.Presenter: every phase of a run lands
// in the bottom output panel, one line at a time.
type presenter struct {
	send func(tea.Msg)
}

func (p presenter) line(text string) { p.send(OutputLineMsg{Text: text}) }

func (p presenter) Stage(params flow.StageParams) error {
	p.line(params.Message)
	return params.Work()
}

// HookPhase streams the hooks as they run: RunHooks writes from this goroutine,
// so the sink turns its bytes into lines and posts each one as a message rather
// than touching the model.
func (p presenter) HookPhase(params flow.HookPhaseParams) error {
	p.line(params.Title)
	sink := &flow.LineWriter{Emit: p.line}
	err := params.Run(sink)
	sink.Flush()
	return err
}

// Notice keeps the abort to itself: on a surface where the modal closing is the
// answer, logging "Aborted." for every question the user backed out of turns the
// panel into a list of things that did not happen.
func (p presenter) Notice(notice flow.Notice) {
	if notice.IsAbort() {
		return
	}
	p.line(notice.Text)
	for _, line := range notice.Lines {
		p.line(line)
	}
}

func (p presenter) Status(notice flow.Notice) {
	p.line(notice.Text)
	for _, line := range notice.Lines {
		p.line(line)
	}
}

type createPresenter struct{ presenter }

func (p createPresenter) Created(outcome createflow.Outcome) error {
	if outcome.Aborted {
		return nil
	}
	p.line(fmt.Sprintf(domain.DashboardFinishedFmt, domain.OpKindCreate, outcome.Branch))
	p.send(createdMsg{branch: outcome.Branch})
	return nil
}

type cleanPresenter struct{ presenter }

func (p cleanPresenter) Cleaned(outcome cleanflow.Outcome) error {
	if outcome.AlreadyAbsent {
		p.line(fmt.Sprintf(domain.CleanAlreadyAbsentFmt, outcome.Branch))
	} else {
		p.line(fmt.Sprintf(domain.DashboardFinishedFmt, domain.OpKindClean, outcome.Branch))
	}
	for _, child := range outcome.Reparented {
		p.line(fmt.Sprintf(domain.CleanReparentedFmt, child.Branch, child.NewParent))
	}
	for _, child := range outcome.OrphanedChildren {
		p.line(fmt.Sprintf(domain.CleanStillOrphanedFmt, child.Branch, child.OldParent))
	}
	p.send(cleanedMsg{branch: outcome.Branch})
	return nil
}

type reparentPresenter struct{ presenter }

func (p reparentPresenter) Reparented(outcome reparentflow.Outcome) error {
	for _, result := range outcome.Results {
		p.line(fmt.Sprintf(domain.ReparentedFmt, result.Branch, result.OldParent, result.NewParent))
	}
	// The change is metadata only; the rebase is a separate run the user starts.
	p.line(domain.ReparentSyncHintBare)
	p.send(reparentedMsg{})
	return nil
}

type prunePresenter struct{ presenter }

func (p prunePresenter) Pruned(outcome pruneflow.Outcome) error {
	if outcome.Empty {
		p.line(domain.PruneNothingToPrune)
		return nil
	}
	for _, candidate := range outcome.Result.Pruned {
		p.line(fmt.Sprintf(domain.DashboardFinishedFmt, domain.OpKindPrune, candidate.Branch))
	}
	for _, child := range outcome.Result.Reparented {
		p.line(fmt.Sprintf(domain.CleanReparentedFmt, child.Branch, child.NewParent))
	}
	for _, child := range outcome.Result.Orphaned {
		p.line(fmt.Sprintf(domain.CleanStillOrphanedFmt, child.Branch, child.OldParent))
	}
	// A worktree the user checked and the safety re-gate then dropped has to say
	// so: without this it simply vanishes from a run the user asked for.
	for _, skip := range outcome.Result.Skipped {
		p.line(fmt.Sprintf(domain.PruneSkippedFmt, skip.Branch, rules.PruneReasonLabel(skip.Reason)))
	}
	p.send(prunedMsg{})
	return nil
}
