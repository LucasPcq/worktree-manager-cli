// Package sync runs the `wtm sync` flow.
package sync

import (
	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/flow"
)

type Request struct {
	// Branches and All fix the selection (branch args / --all): the step is not
	// asked, but stays read back in the recap.
	Branches []string
	All      bool
	// Precheck is what arrives checked when the selection step IS asked. A surface
	// that already knows a likely answer offers it; the selection stays exact.
	Precheck     []string
	KeepConflict bool
	FFParents    bool
	NoFFParents  bool
	Push         bool
	NoPush       bool
	// DryRun previews the cascade and rebases nothing.
	DryRun     bool
	BaseBranch string
}

type Outcome struct {
	Result domain.SyncResult
	Plan   domain.SyncPlan
	// Empty reports that the selection resolved to nothing to rebase.
	Empty   bool
	Aborted bool
}

// Presenter carries the three moments of a sync: the plan a run that cannot ask
// has to print, the recap the user reads before being asked to push, and the
// conclusion. They are separate because the push question falls between the last
// two.
type Presenter interface {
	flow.Presenter
	Planned(domain.SyncPlan)
	Rebased(domain.SyncResult)
	Synced(Outcome) error
}

type Params struct {
	Context   flow.Context
	Request   Request
	Prompter  flow.Prompter
	Presenter Presenter
}

// Operation declares how a surface must schedule a sync: it rebases several
// worktrees and asks first, so it holds the surface for its whole run and needs
// no per-worktree lock.
func Operation() flow.Operation {
	return flow.Operation{Kind: domain.OpKindSync, Mode: flow.ModeBlocking}
}

type syncFlow struct {
	ctx       flow.Context
	request   Request
	prompter  flow.Prompter
	presenter Presenter

	statuses   []domain.WorktreeStatus
	classified []domain.ParentUpdate
	// plan is the cascade the recap previews; it is rebuilt for the final
	// selection before the run, because the answers may have narrowed it.
	plan domain.SyncPlan
}
