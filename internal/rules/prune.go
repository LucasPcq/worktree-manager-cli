package rules

import (
	"github.com/LucasPcq/wtm/internal/domain"
)

// ClassifyPruneParams holds the pre-gathered data ClassifyPrune reasons over. All
// I/O (worktree listing, PR fetch, upstream-gone probing, current-worktree path
// resolution) is done by the caller so this stays pure.
type ClassifyPruneParams struct {
	Statuses []domain.WorktreeStatus
	Nodes    []domain.WorktreeNode
	// PRStates maps a branch to its normalized PR state (open/merged/closed).
	PRStates map[string]string
	// Gone maps a branch to whether its upstream tracking ref was deleted.
	Gone map[string]bool
	// Active reason filters.
	Merged     bool
	Closed     bool
	GoneFilter bool
	BaseBranch string
	// Force allows dirty worktrees to be selected instead of skipped.
	Force bool
}

// ClassifyPrune decides, without side effects, which worktrees a prune would
// remove, which it skips (and why), and how surviving children reparent onto
// their grandparent. A worktree is considered only when it matches an active
// filter; matches that are protected (base, main, current) or dirty (without
// Force) are recorded as skips.
func ClassifyPrune(params ClassifyPruneParams) domain.PrunePlan {
	nodeByBranch := make(map[string]domain.WorktreeNode, len(params.Nodes))
	for _, n := range params.Nodes {
		nodeByBranch[n.Branch] = n
	}

	var plan domain.PrunePlan
	selected := make(map[string]bool)

	for _, st := range params.Statuses {
		reason, matched := pruneMatchReason(st, params)
		if !matched {
			continue
		}

		node := nodeByBranch[st.Branch]
		if skip, reason := pruneSkipReason(pruneSkipParams{
			Status:     st,
			Node:       node,
			BaseBranch: params.BaseBranch,
			Force:      params.Force,
		}); skip {
			plan.Skipped = append(plan.Skipped, domain.PruneSkip{Branch: st.Branch, Reason: reason})
			continue
		}

		plan.Selected = append(plan.Selected, domain.PruneCandidate{
			Branch:       st.Branch,
			Path:         st.Path,
			Reason:       reason,
			IsDirty:      st.IsDirty,
			SourceBranch: node.SourceBranch,
		})
		selected[st.Branch] = true
	}

	plan.Reparents = pruneReparents(pruneReparentsParams{
		Selected:   plan.Selected,
		SelectedIn: selected,
		Nodes:      params.Nodes,
		BaseBranch: params.BaseBranch,
	})
	return plan
}

// pruneMatchReason returns the category that makes a worktree prunable under the
// active reason filters (merged/closed/gone). Merged is CommitsAhead==0 (no
// commits the base lacks) and does not catch squash-merges — gone/closed do.
func pruneMatchReason(st domain.WorktreeStatus, params ClassifyPruneParams) (string, bool) {
	if params.Merged && st.CommitsAhead == 0 {
		return domain.PruneReasonMerged, true
	}
	if params.Closed {
		switch params.PRStates[st.Branch] {
		case domain.PRStateMerged:
			return domain.PruneReasonPRMerged, true
		case domain.PRStateClosed:
			return domain.PruneReasonPRClosed, true
		}
	}
	if params.GoneFilter && params.Gone[st.Branch] {
		return domain.PruneReasonGone, true
	}
	return "", false
}

type pruneSkipParams struct {
	Status     domain.WorktreeStatus
	Node       domain.WorktreeNode
	BaseBranch string
	Force      bool
}

// pruneSkipReason reports whether a matched worktree must be spared, and why. The
// current worktree is intentionally NOT spared — like clean, prune removes it and
// the command redirects the shell to the base repo afterwards.
func pruneSkipReason(params pruneSkipParams) (bool, string) {
	if params.Status.Branch == params.BaseBranch {
		return true, domain.PruneSkipBase
	}
	if params.Node.IsMain || params.Status.IsParent {
		return true, domain.PruneSkipMain
	}
	if params.Status.IsDirty && !params.Force {
		return true, domain.PruneSkipDirty
	}
	return false, ""
}

type pruneReparentsParams struct {
	Selected   []domain.PruneCandidate
	SelectedIn map[string]bool
	Nodes      []domain.WorktreeNode
	BaseBranch string
}

// pruneReparents computes the grandparent moves for children of pruned worktrees.
// A child that is itself pruned is skipped; a grandparent that is also pruned
// falls back to the base branch so no child is left pointing at a removed parent.
func pruneReparents(params pruneReparentsParams) []domain.ReparentResult {
	var reparents []domain.ReparentResult
	for _, cand := range params.Selected {
		grandparent := cand.SourceBranch
		if grandparent == "" || params.SelectedIn[grandparent] {
			grandparent = params.BaseBranch
		}
		children := ChildrenOf(ChildrenOfParams{Nodes: params.Nodes, Branch: cand.Branch})
		for _, child := range children {
			if params.SelectedIn[child.Branch] {
				continue
			}
			reparents = append(reparents, domain.ReparentResult{
				Branch:    child.Branch,
				OldParent: cand.Branch,
				NewParent: grandparent,
			})
		}
	}
	return reparents
}

// FinalizePrunePlanParams holds inputs for FinalizePrunePlan.
type FinalizePrunePlanParams struct {
	Plan       domain.PrunePlan
	Chosen     []string
	BaseBranch string
	// Force keeps dirty worktrees in the selection; without it, a chosen dirty
	// worktree is dropped back to Skipped (the interactive confirm gates this).
	Force bool
}

// FinalizePrunePlan restricts a full prune plan to the user-chosen subset of
// candidates (after an interactive deselect) and recomputes reparenting so no
// surviving worktree is left pointing at a removed parent. It works purely from
// the plan — each candidate carries its SourceBranch, so parent chains among
// pruned worktrees are reconstructable without the node graph. Without Force, a
// chosen dirty worktree is moved to Skipped rather than removed. Skips carry
// through unchanged.
func FinalizePrunePlan(params FinalizePrunePlanParams) domain.PrunePlan {
	chosen := make(map[string]bool, len(params.Chosen))
	for _, b := range params.Chosen {
		chosen[b] = true
	}

	candByBranch := make(map[string]domain.PruneCandidate, len(params.Plan.Selected))
	for _, c := range params.Plan.Selected {
		candByBranch[c.Branch] = c
	}

	out := domain.PrunePlan{Skipped: params.Plan.Skipped}

	// A chosen-but-dirty worktree is only removed with Force; otherwise drop it
	// from the selection and record it as skipped (dirty).
	if !params.Force {
		for _, c := range params.Plan.Selected {
			if c.IsDirty && chosen[c.Branch] {
				chosen[c.Branch] = false
				out.Skipped = append(out.Skipped, domain.PruneSkip{Branch: c.Branch, Reason: domain.PruneSkipDirty})
			}
		}
	}

	for _, c := range params.Plan.Selected {
		if chosen[c.Branch] {
			out.Selected = append(out.Selected, c)
		}
	}

	// Surviving children of a still-chosen pruned parent keep their move (its
	// target was already resolved to a survivor or the base).
	for _, r := range params.Plan.Reparents {
		if chosen[r.OldParent] {
			out.Reparents = append(out.Reparents, r)
		}
	}

	// A candidate the user deselected whose parent is still being pruned now
	// survives and must reparent onto its first surviving ancestor.
	for _, c := range params.Plan.Selected {
		if chosen[c.Branch] || !chosen[c.SourceBranch] {
			continue
		}
		out.Reparents = append(out.Reparents, domain.ReparentResult{
			Branch:    c.Branch,
			OldParent: c.SourceBranch,
			NewParent: survivingAncestor(candByBranch, chosen, c.SourceBranch, params.BaseBranch),
		})
	}

	return out
}

// survivingAncestor walks up the pruned-candidate parent chain from start until it
// reaches a branch that is not being pruned, falling back to base.
func survivingAncestor(cands map[string]domain.PruneCandidate, chosen map[string]bool, start, base string) string {
	cur := start
	for chosen[cur] {
		next := cands[cur].SourceBranch
		if next == "" {
			return base
		}
		cur = next
	}
	if cur == "" {
		return base
	}
	return cur
}
