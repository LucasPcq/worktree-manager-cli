package rules

import (
	"fmt"

	"github.com/LucasPcq/wtm/internal/domain"
)

// CleanUnsafeReason reports why a worktree is unsafe to remove without --force,
// precedence dirty → unpushed → open PR (the twin of pruneUnsafeReason). An empty
// string means the worktree is safe to remove.
func CleanUnsafeReason(check domain.CleanCheckResult) string {
	if check.IsDirty {
		return "has uncommitted changes"
	}
	if check.UnpushedCommits > 0 {
		return fmt.Sprintf("has %d unpushed commit(s)", check.UnpushedCommits)
	}
	if check.HasOpenPR {
		return "has an open pull request"
	}
	return ""
}

// DecideCleanReparent resolves whether a cleaned worktree's orphaned children are
// reparented onto the grandparent, for the non-interactive paths: no children → no,
// --reparent-children → yes, otherwise the safe default (leave orphaned).
func DecideCleanReparent(plan domain.CleanReparentPlan, flag bool) bool {
	if len(plan.Children) == 0 {
		return false
	}
	return flag
}

// CleanReparentPreview returns the children that would be reparented (plan preview /
// dry-run), or nil when reparenting is not applied.
func CleanReparentPreview(plan domain.CleanReparentPlan, apply bool) []domain.ReparentResult {
	if !apply || len(plan.Children) == 0 {
		return nil
	}
	return plan.Children
}

// CleanOrphanedChildren returns the children left dangling because reparenting was
// not authorized.
func CleanOrphanedChildren(plan domain.CleanReparentPlan, apply bool) []domain.ReparentResult {
	if apply || len(plan.Children) == 0 {
		return nil
	}
	return plan.Children
}
