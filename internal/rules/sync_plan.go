package rules

import (
	"fmt"
	"sort"

	"github.com/LucasPcq/wtm/internal/domain"
)

// BuildSyncPlanParams holds inputs for building a cascade sync plan.
type BuildSyncPlanParams struct {
	Nodes      []domain.WorktreeNode
	BaseBranch string
}

// BuildSyncPlan orders managed worktrees so each parent is rebased before its
// children. Ordering is by depth in the parent tree (number of managed
// ancestors), which guarantees a child is never rebased onto a stale parent.
//
// The base branch (and the main worktree) is the root and never a rebase step.
// A node whose source branch is the base or an unmanaged branch is a root-level
// step. A cycle in the parent chain is a hard error.
func BuildSyncPlan(params BuildSyncPlanParams) (domain.SyncPlan, error) {
	managed := make(map[string]domain.WorktreeNode, len(params.Nodes))
	for _, node := range params.Nodes {
		managed[node.Branch] = node
	}

	type indexedStep struct {
		step  domain.SyncStep
		depth int
		order int
	}

	steps := make([]indexedStep, 0, len(params.Nodes))
	for i, node := range params.Nodes {
		if node.IsMain || node.Branch == params.BaseBranch {
			continue
		}

		depth, err := parentDepth(parentDepthParams{Branch: node.Branch, Managed: managed})
		if err != nil {
			return domain.SyncPlan{}, err
		}

		steps = append(steps, indexedStep{
			step: domain.SyncStep{
				Branch:           node.Branch,
				Path:             node.Path,
				SourceBranch:     node.SourceBranch,
				RebaseInProgress: node.RebaseInProgress,
			},
			depth: depth,
			order: i,
		})
	}

	sort.SliceStable(steps, func(i, j int) bool {
		if steps[i].depth != steps[j].depth {
			return steps[i].depth < steps[j].depth
		}
		return steps[i].order < steps[j].order
	})

	ordered := make([]domain.SyncStep, len(steps))
	for i, s := range steps {
		ordered[i] = s.step
	}

	return domain.SyncPlan{BaseBranch: params.BaseBranch, Steps: ordered}, nil
}

// FilterSyncSteps keeps only the steps whose branch is in selected, preserving
// the topological order of the input. An empty (or nil) selection is treated as
// "no filter" and returns the steps unchanged, so callers can pass nil to mean
// "every worktree". A branch in selected with no matching step (e.g. the base or
// an unknown branch) is simply ignored.
func FilterSyncSteps(steps []domain.SyncStep, selected []string) []domain.SyncStep {
	if len(selected) == 0 {
		return steps
	}

	wanted := make(map[string]struct{}, len(selected))
	for _, branch := range selected {
		wanted[branch] = struct{}{}
	}

	filtered := make([]domain.SyncStep, 0, len(steps))
	for _, step := range steps {
		if _, ok := wanted[step.Branch]; ok {
			filtered = append(filtered, step)
		}
	}
	return filtered
}

type parentDepthParams struct {
	Branch  string
	Managed map[string]domain.WorktreeNode
}

// parentDepth counts how many managed ancestors a branch has by walking up the
// SourceBranch chain. It stops at the first unmanaged source (base or external
// branch) and returns an error if the chain revisits a branch (a cycle).
func parentDepth(params parentDepthParams) (int, error) {
	seen := map[string]struct{}{params.Branch: {}}
	depth := 0
	current := params.Managed[params.Branch]

	for {
		parent, ok := params.Managed[current.SourceBranch]
		if !ok {
			return depth, nil
		}
		if _, looped := seen[parent.Branch]; looped {
			return 0, fmt.Errorf("cycle detected in worktree parent chain at %q", parent.Branch)
		}
		seen[parent.Branch] = struct{}{}
		depth++
		current = parent
	}
}

// ParentsOutsideCascadeParams holds inputs for ParentsOutsideCascade.
type ParentsOutsideCascadeParams struct {
	Steps      []domain.SyncStep
	BaseBranch string
}

// ParentsOutsideCascade lists the parents a cascade rebases onto but never
// refreshes. The base is excluded — it has its own fast-forward — and so is an
// unset parent. Status is left for the caller to resolve against the remote.
func ParentsOutsideCascade(params ParentsOutsideCascadeParams) []domain.ParentUpdate {
	covered := make(map[string]struct{}, len(params.Steps))
	for _, step := range params.Steps {
		covered[step.Branch] = struct{}{}
	}

	at := make(map[string]int, len(params.Steps))
	parents := make([]domain.ParentUpdate, 0, len(params.Steps))
	for _, step := range params.Steps {
		parent := step.SourceBranch
		if parent == "" || parent == params.BaseBranch {
			continue
		}
		if _, skip := covered[parent]; skip {
			continue
		}
		if i, seen := at[parent]; seen {
			parents[i].Children = append(parents[i].Children, step.Branch)
			continue
		}
		at[parent] = len(parents)
		parents = append(parents, domain.ParentUpdate{Branch: parent, Children: []string{step.Branch}})
	}
	return parents
}

// ParentBranchesParams holds inputs for ParentBranches.
type ParentBranchesParams struct {
	Nodes      []domain.WorktreeNode
	BaseBranch string
}

// ParentBranches lists the distinct recorded parents, base and unset excluded.
// It is what a run inspects before the selection is known: which parents end up
// uncovered depends on the selection, the state of each against its remote does not.
func ParentBranches(params ParentBranchesParams) []string {
	seen := make(map[string]struct{}, len(params.Nodes))
	branches := make([]string, 0, len(params.Nodes))
	for _, node := range params.Nodes {
		parent := node.SourceBranch
		if parent == "" || parent == params.BaseBranch {
			continue
		}
		if _, dup := seen[parent]; dup {
			continue
		}
		seen[parent] = struct{}{}
		branches = append(branches, parent)
	}
	return branches
}

// StaleParentsForParams holds inputs for StaleParentsFor.
type StaleParentsForParams struct {
	Uncovered  []domain.ParentUpdate
	Classified []domain.ParentUpdate
}

// StaleParentsFor pairs the parents a selection leaves uncovered with what an
// earlier inspection found: Children from the selection, Status and tips from the
// inspection, keeping only what a fast-forward would advance.
func StaleParentsFor(params StaleParentsForParams) []domain.ParentUpdate {
	status := make(map[string]domain.ParentUpdate, len(params.Classified))
	for _, update := range params.Classified {
		status[update.Branch] = update
	}

	stale := make([]domain.ParentUpdate, 0, len(params.Uncovered))
	for _, parent := range params.Uncovered {
		found, ok := status[parent.Branch]
		if !ok || found.Status != domain.ParentBehind {
			continue
		}
		found.Children = parent.Children
		stale = append(stale, found)
	}
	return stale
}
