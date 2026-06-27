package rules

import "github.com/LucasPcq/wtm/internal/domain"

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
}

// DecidePush resolves whether to push the rebased branches. With branches to
// push and neither --push nor --no-push, an interactive run is asked (PushConfirm);
// a non-interactive run only pushes when --push is set.
func DecidePush(params DecidePushParams) PushDecision {
	if params.PushableCount == 0 || params.NoPush {
		return PushSkip
	}
	if params.Push {
		return PushForce
	}
	if !params.Interactive {
		return PushSkip
	}
	return PushConfirm
}
