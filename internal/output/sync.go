package output

import (
	"fmt"
	"io"
	"strings"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/styles"
)

// FormatSyncPlan prints the ordered cascade preview: which branch is rebased
// onto which parent, in execution order.
func FormatSyncPlan(w io.Writer, plan domain.SyncPlan) {
	Blank(w)
	SectionTitle(w, fmt.Sprintf("Sync plan (base: %s)", plan.BaseBranch))
	if len(plan.Steps) == 0 {
		Message(w, styles.Muted.Render("No worktrees to sync."))
		Blank(w)
		return
	}
	for i, step := range plan.Steps {
		parent := step.SourceBranch
		if parent == "" {
			parent = styles.Muted.Render("unknown parent")
		}
		InfoLine(w, fmt.Sprintf("%d.", i+1), fmt.Sprintf("%s %s %s", step.Branch, styles.Muted.Render("←"), parent))
	}
	Blank(w)
}

// FormatSyncResult prints the detailed recap of a sync run: the base update,
// one line per branch (parent, target commit, before→after, replayed count),
// and a trailing summary of which branches are ready to push (or were pushed).
func FormatSyncResult(w io.Writer, result domain.SyncResult) {
	Blank(w)
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
	Blank(w)

	for _, step := range result.Steps {
		printStep(w, step)
	}

	Blank(w)
	printPushSummary(w, result.Steps)
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
		Error(w, fmt.Sprintf("%s conflict on %s — rebase aborted (working tree clean), resolve manually", step.Branch, step.SourceBranch))
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
		if step.Status != domain.SyncStatusSynced {
			continue
		}
		if step.Pushed {
			pushed = append(pushed, step.Branch)
			continue
		}
		ready = append(ready, step.Branch)
	}

	if len(pushed) > 0 {
		Success(w, fmt.Sprintf("Pushed %d branch(es) (force-with-lease): %s", len(pushed), strings.Join(pushed, ", ")))
	}
	if len(ready) > 0 {
		Message(w, fmt.Sprintf("%d branch(es) ready to push (force-with-lease): %s",
			len(ready), strings.Join(ready, ", ")))
	}
	if len(pushed) == 0 && len(ready) == 0 {
		Message(w, styles.Muted.Render("Nothing was rebased — no branch to push."))
	}
}

// WriteSyncResultJSON writes the sync result as pretty-printed JSON.
func WriteSyncResultJSON(w io.Writer, result domain.SyncResult) error {
	return encodeJSON(w, result)
}
