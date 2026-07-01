package output

import (
	"fmt"
	"io"

	"github.com/LucasPcq/wtm/internal/domain"
)

// FormatPrunePlan renders the dry-run / preview of a prune: the worktrees that
// would be removed (with their reason), the reparenting of surviving children,
// and anything skipped. Raw body — the command's frame owns the outer padding.
func FormatPrunePlan(w io.Writer, plan domain.PrunePlan) {
	if len(plan.Selected) == 0 {
		Message(w, "Nothing to prune.")
	} else {
		items := make([]AnnounceItem, 0, len(plan.Selected))
		for _, c := range plan.Selected {
			items = append(items, AnnounceItem{Label: c.Branch, Value: pruneReasonLabel(c.Reason)})
		}
		Announce(w, fmt.Sprintf("Would prune %d worktree(s):", len(plan.Selected)), items)
	}

	if len(plan.Reparents) > 0 {
		Blank(w)
		items := make([]AnnounceItem, 0, len(plan.Reparents))
		for _, r := range plan.Reparents {
			items = append(items, AnnounceItem{Label: r.Branch, Value: fmt.Sprintf("%s → %s", r.OldParent, r.NewParent)})
		}
		Announce(w, "Reparent orphaned children onto their grandparent:", items)
	}

	if len(plan.Skipped) > 0 {
		Blank(w)
		items := make([]AnnounceItem, 0, len(plan.Skipped))
		for _, s := range plan.Skipped {
			items = append(items, AnnounceItem{Label: s.Branch, Value: pruneReasonLabel(s.Reason)})
		}
		Announce(w, "Skipped:", items)
	}
}

// FormatPruneReparentProposal renders the proposed reparenting of pruned
// worktrees' orphaned children onto their grandparent, mirroring clean's
// FormatReparentProposal (a leading blank separator + an announce block of
// branch → old → new rows). Raw body — the caller's frame owns the padding.
func FormatPruneReparentProposal(w io.Writer, moves []domain.ReparentResult) {
	Blank(w)
	items := make([]AnnounceItem, 0, len(moves))
	for _, m := range moves {
		items = append(items, AnnounceItem{Label: m.Branch, Value: fmt.Sprintf("%s → %s", m.OldParent, m.NewParent)})
	}
	Announce(w, "Reparent orphaned children onto their grandparent:", items)
}

// FormatPruneResult renders the outcome of an executed prune: a success line per
// removed worktree, the reparenting applied, any children left orphaned, and
// skipped-with-reason warnings. Raw body — the command's frame owns the padding.
func FormatPruneResult(w io.Writer, result domain.PruneResult) {
	if len(result.Pruned) == 0 {
		Message(w, "Nothing to prune.")
	}
	for _, c := range result.Pruned {
		Success(w, fmt.Sprintf("Pruned %s (%s)", c.Branch, pruneReasonLabel(c.Reason)))
	}
	for _, r := range result.Reparented {
		Success(w, fmt.Sprintf("Reparented %s onto %s", r.Branch, r.NewParent))
	}
	for _, o := range result.Orphaned {
		Warning(w, fmt.Sprintf("%s still points at the removed parent %s — reparent it with `wtm reparent`", o.Branch, o.OldParent))
	}
	for _, s := range result.Skipped {
		Warning(w, fmt.Sprintf("Skipped %s (%s)", s.Branch, pruneReasonLabel(s.Reason)))
	}
}

// WritePruneResultJSON writes the prune outcome as JSON, ensuring the slices
// marshal as [] rather than null.
func WritePruneResultJSON(w io.Writer, result domain.PruneResult) error {
	if result.Pruned == nil {
		result.Pruned = []domain.PruneCandidate{}
	}
	if result.Reparented == nil {
		result.Reparented = []domain.ReparentResult{}
	}
	if result.Orphaned == nil {
		result.Orphaned = []domain.ReparentResult{}
	}
	if result.Skipped == nil {
		result.Skipped = []domain.PruneSkip{}
	}
	return encodeJSON(w, result)
}

// pruneReasonLabel maps a prune reason/skip constant to a short human phrase.
func pruneReasonLabel(reason string) string {
	switch reason {
	case domain.PruneReasonMerged:
		return "merged"
	case domain.PruneReasonPRMerged:
		return "PR merged"
	case domain.PruneReasonPRClosed:
		return "PR closed"
	case domain.PruneReasonGone:
		return "remote branch gone"
	case domain.PruneSkipBase:
		return "base branch"
	case domain.PruneSkipMain:
		return "main worktree"
	case domain.PruneSkipDirty:
		return "dirty — pass --force"
	default:
		return reason
	}
}
