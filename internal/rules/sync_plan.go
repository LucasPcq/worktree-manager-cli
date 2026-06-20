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
				Branch:       node.Branch,
				Path:         node.Path,
				SourceBranch: node.SourceBranch,
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
