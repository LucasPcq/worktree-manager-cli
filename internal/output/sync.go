package output

import (
	"fmt"
	"io"
	"strings"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/styles"
)

// FormatSyncPlan prints the ordered cascade preview: which branch is rebased
// onto which parent, in execution order. It emits a raw body with no outer blank
// lines; the caller's frame owns the outer vertical padding.
func FormatSyncPlan(w io.Writer, plan domain.SyncPlan) {
	SectionTitle(w, fmt.Sprintf("Sync plan (base: %s)", plan.BaseBranch))
	if len(plan.Steps) == 0 {
		Message(w, styles.Muted.Render("No worktrees to sync."))
		return
	}
	for i, step := range plan.Steps {
		parent := step.SourceBranch
		if parent == "" {
			parent = styles.Muted.Render("unknown parent")
		}
		InfoLine(w, fmt.Sprintf("%d.", i+1), fmt.Sprintf("%s %s %s", step.Branch, styles.Muted.Render("←"), parent))
	}
}

// FormatSyncResult prints the detailed recap of a sync run: the base update and
// one line per branch (parent, target commit, before→after, replayed count).
// It is rendered BEFORE the push decision so the user sees what happened before
// being asked to push. The push summary is a separate block
// (see FormatSyncPushSummary).
//
// Raw body: no outer blank lines. The command's frame (FrameStart/FrameEnd) owns
// the top/bottom padding; the inter-section blank between the base line and the
// per-branch lines is kept as a genuine separator.
func FormatSyncResult(w io.Writer, result domain.SyncResult) {
	if result.BaseUpdated {
		InfoLine(w, "Base", fmt.Sprintf("%s  %s → %s  %s",
			result.BaseBranch,
			styles.Muted.Render(result.BaseOldTip),
			styles.Muted.Render(result.BaseNewTip),
			styles.Muted.Render(fmt.Sprintf("(fast-forwarded from origin/%s)", result.BaseBranch))))
	} else {
		InfoLine(w, "Base", fmt.Sprintf("%s  %s  %s",
			result.BaseBranch,
			styles.Muted.Render(result.BaseOldTip),
			styles.Muted.Render("(already up to date / no fast-forward)")))
	}

	// The blank separates the base line from the per-branch lines; with no steps
	// (a base-only refresh) it would stack against the push summary's leading
	// blank, so it is only emitted when there is something to separate.
	if len(result.Steps) == 0 {
		return
	}
	Blank(w)
	for _, step := range result.Steps {
		printStep(w, step)
	}
}

// FormatSyncPushSummary prints the trailing summary of which branches were
// pushed or are ready to push. Rendered AFTER the push decision so it reflects
// the final state. Leads with a blank, never trails.
func FormatSyncPushSummary(w io.Writer, steps []domain.SyncStepResult) {
	Blank(w)
	printPushSummary(w, steps)
}

func printStep(w io.Writer, step domain.SyncStepResult) {
	switch step.Status {
	case domain.SyncStatusSynced:
		Success(w, syncedLine(step))
	case domain.SyncStatusUpToDate:
		Message(w, fmt.Sprintf("%s %s %s",
			styles.Muted.Render("="), step.Branch, styles.Muted.Render("already up to date")))
	case domain.SyncStatusSkippedDirty:
		Warning(w, fmt.Sprintf("%s skipped — uncommitted changes (descendants skipped)", step.Branch))
	case domain.SyncStatusSkippedAncestor:
		Warning(w, fmt.Sprintf("%s skipped — ancestor %s was not synced", step.Branch, step.SourceBranch))
	case domain.SyncStatusDiverged:
		Warning(w, fmt.Sprintf("%s skipped — diverged from origin/%s (reconcile manually); descendants skipped", step.Branch, step.Branch))
	case domain.SyncStatusUnknownParent:
		Warning(w, fmt.Sprintf("%s skipped — no recorded parent branch", step.Branch))
	case domain.SyncStatusConflict:
		Danger(w, fmt.Sprintf("%s conflict on %s — rebase aborted (working tree clean); descendants skipped", step.Branch, step.SourceBranch))
	case domain.SyncStatusError:
		Error(w, fmt.Sprintf("%s failed — %s", step.Branch, step.Detail))
	}
}

func syncedLine(step domain.SyncStepResult) string {
	pushed := ""
	if step.Pushed {
		pushed = styles.Success.Render("  (pushed)")
	}
	return fmt.Sprintf("%s rebased onto %s (%s)   %s   %s → %s%s",
		step.Branch,
		step.SourceBranch,
		styles.Muted.Render(step.OntoTip),
		styles.Muted.Render(fmt.Sprintf("%d commits", step.CommitsReplayed)),
		styles.Muted.Render(step.OldTip),
		styles.Muted.Render(step.NewTip),
		pushed)
}

func printPushSummary(w io.Writer, steps []domain.SyncStepResult) {
	pushed := make([]string, 0)
	ready := make([]string, 0)
	for _, step := range steps {
		if step.Pushed {
			pushed = append(pushed, step.Branch)
			continue
		}
		if step.PushPending {
			ready = append(ready, step.Branch)
		}
	}

	if len(pushed) > 0 {
		Success(w, fmt.Sprintf("Pushed %d branch(es) (force-with-lease): %s", len(pushed), strings.Join(pushed, ", ")))
	}
	if len(ready) > 0 {
		Message(w, fmt.Sprintf("%d branch(es) ready to push (force-with-lease): %s",
			len(ready), strings.Join(ready, ", ")))
	}
	if len(pushed) == 0 && len(ready) == 0 {
		Message(w, styles.Muted.Render("Everything is in sync with origin — nothing to push."))
	}
}

// WriteSyncResultJSON writes the sync result as pretty-printed JSON.
func WriteSyncResultJSON(w io.Writer, result domain.SyncResult) error {
	return encodeJSON(w, result)
}
