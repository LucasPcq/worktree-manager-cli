package rules

import (
	"fmt"

	"github.com/LucasPcq/wtm/internal/domain"
)

// SyncIncludesBaseParams holds inputs for SyncIncludesBase.
type SyncIncludesBaseParams struct {
	Selected   []string
	BaseBranch string
}

// SyncIncludesBase reports whether the selection asks to refresh the base branch.
// The --all case (nil selection) always does; an explicit selection does when it
// names the base. It lets the command run a base-only refresh with no rebase steps.
func SyncIncludesBase(params SyncIncludesBaseParams) bool {
	if params.Selected == nil {
		return true
	}
	for _, branch := range params.Selected {
		if branch == params.BaseBranch {
			return true
		}
	}
	return false
}

// BaseIsTargetParams holds inputs for BaseIsTarget.
type BaseIsTargetParams struct {
	Steps      []domain.SyncStep
	Selected   []string
	BaseBranch string
}

// BaseIsTarget reports whether the base plays any role in this run: a step
// rebases onto it, the selection names it, or the run has no step at all (the
// base-only refresh). When it is false the base is left completely alone —
// fetching and fast-forwarding it would be a side effect on a branch the run was
// never asked about, which is what happens when every worktree hangs off some
// other parent.
func BaseIsTarget(params BaseIsTargetParams) bool {
	for _, step := range params.Steps {
		if step.SourceBranch == params.BaseBranch {
			return true
		}
	}
	if len(params.Steps) == 0 {
		return SyncIncludesBase(SyncIncludesBaseParams{
			Selected:   params.Selected,
			BaseBranch: params.BaseBranch,
		})
	}
	for _, branch := range params.Selected {
		if branch == params.BaseBranch {
			return true
		}
	}
	return false
}

// PushableCount counts the rebased steps that have a pending push not yet done.
func PushableCount(steps []domain.SyncStepResult) int {
	count := 0
	for _, step := range steps {
		if step.PushPending && !step.Pushed {
			count++
		}
	}
	return count
}

// HasSyncFailure reports whether any step ended in a conflict or error.
func HasSyncFailure(steps []domain.SyncStepResult) bool {
	for _, step := range steps {
		if step.Status == domain.SyncStatusConflict || step.Status == domain.SyncStatusError {
			return true
		}
	}
	return false
}

// PushDecision is the resolved push action after applying the non-interactive
// push flags and the pushable count.
type PushDecision int

const (
	// PushSkip means do not push: nothing is pushable, --no-push was given, or a
	// non-interactive run was not told to push with --push.
	PushSkip PushDecision = iota
	// PushForce means push without prompting (--push with pushable branches).
	PushForce
	// PushConfirm means an interactive run should ask before pushing.
	PushConfirm
)

// DecidePushParams holds inputs for DecidePush.
type DecidePushParams struct {
	Push          bool
	NoPush        bool
	Interactive   bool
	PushableCount int
	// Yes is --yes: run without the push prompt. Pushing force-pushes with lease,
	// so the safe default under --yes is not to push; the user opts in with --push.
	Yes bool
}

// DecidePush resolves whether to push the rebased branches. With branches to
// push and neither --push nor --no-push, an interactive run is asked (PushConfirm);
// a non-interactive or --yes run only pushes when --push is set (safe default: no push).
func DecidePush(params DecidePushParams) PushDecision {
	if params.PushableCount == 0 || params.NoPush {
		return PushSkip
	}
	if params.Push {
		return PushForce
	}
	if params.Yes || !params.Interactive {
		return PushSkip
	}
	return PushConfirm
}

// ParentDecision is the resolved action for the parents a cascade rebases onto
// without covering them with a step of its own.
type ParentDecision int

const (
	// ParentLeaveAsIs means rebase onto the parents as they are: nothing is stale,
	// --no-ff-parents was given, or an unattended run was not told to refresh them.
	ParentLeaveAsIs ParentDecision = iota
	// ParentFastForward means refresh them without prompting (--ff-parents).
	ParentFastForward
	// ParentAsk means an interactive run should ask before refreshing them.
	ParentAsk
)

// DecideParentFastForwardParams holds inputs for DecideParentFastForward.
type DecideParentFastForwardParams struct {
	FF          bool
	NoFF        bool
	Interactive bool
	StaleCount  int
	// Yes is --yes: run without the prompt. Advancing a branch the user did not
	// name is a side effect they did not ask for, so the unattended default is to
	// leave it alone and report it; they opt in with --ff-parents.
	Yes bool
}

// DecideParentFastForward resolves whether the stale parents are fast-forwarded.
// With stale parents and neither flag, an interactive run is asked (ParentAsk);
// an unattended or --yes run only refreshes them when --ff-parents is set.
func DecideParentFastForward(params DecideParentFastForwardParams) ParentDecision {
	if params.StaleCount == 0 || params.NoFF {
		return ParentLeaveAsIs
	}
	if params.FF {
		return ParentFastForward
	}
	if params.Yes || !params.Interactive {
		return ParentLeaveAsIs
	}
	return ParentAsk
}

// StaleParents keeps the parents that a fast-forward would actually advance. A
// diverged parent is reported but never actionable: no fast-forward exists.
func StaleParents(updates []domain.ParentUpdate) []domain.ParentUpdate {
	stale := make([]domain.ParentUpdate, 0, len(updates))
	for _, update := range updates {
		if update.Status == domain.ParentBehind {
			stale = append(stale, update)
		}
	}
	return stale
}

// CommitCountLabel renders a commit distance for prose ("1 commit" / "3 commits").
// It is shared by the two surfaces that state how far a branch is behind — the
// recap in output/ and the wizard step in tui/ — which cannot import each other.
func CommitCountLabel(n int) string {
	if n == 1 {
		return "1 commit"
	}
	return fmt.Sprintf("%d commits", n)
}
