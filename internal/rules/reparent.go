package rules

import (
	"fmt"

	"github.com/LucasPcq/wtm/internal/domain"
)

// ValidateReparentParams holds inputs for validating a parent change.
type ValidateReparentParams struct {
	Nodes      []domain.WorktreeNode
	Branch     string
	NewParent  string
	BaseBranch string
}

// ValidateReparent checks that pointing Branch's parent at NewParent is legal:
// the branch must be a managed worktree, must not become its own parent, and the
// resulting parent graph must stay acyclic. Cycle detection reuses BuildSyncPlan
// so there is a single source of truth for the topological invariant.
func ValidateReparent(params ValidateReparentParams) error {
	if params.NewParent == params.Branch {
		return fmt.Errorf("%w: %s", domain.ErrReparentSelf, params.Branch)
	}

	found := false
	mutated := make([]domain.WorktreeNode, len(params.Nodes))
	for i, node := range params.Nodes {
		if node.Branch == params.Branch {
			node.SourceBranch = params.NewParent
			found = true
		}
		mutated[i] = node
	}
	if !found {
		return fmt.Errorf("%w: %s", domain.ErrWorktreeNotFound, params.Branch)
	}

	if _, err := BuildSyncPlan(BuildSyncPlanParams{Nodes: mutated, BaseBranch: params.BaseBranch}); err != nil {
		return err
	}
	return nil
}

// ChildrenOf returns the managed worktrees whose recorded parent is branch.
func ChildrenOf(nodes []domain.WorktreeNode, branch string) []domain.WorktreeNode {
	children := make([]domain.WorktreeNode, 0)
	for _, node := range nodes {
		if node.SourceBranch == branch {
			children = append(children, node)
		}
	}
	return children
}
