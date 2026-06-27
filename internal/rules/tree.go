package rules

import (
	"sort"
	"time"

	"github.com/LucasPcq/wtm/internal/domain"
)

// ForestNode is an enriched worktree fed into BuildForest: the parent link
// (SourceBranch), the creation time (for sibling ordering) and the per-node
// status flags already computed by the service layer.
type ForestNode struct {
	Branch       string
	Path         string
	SourceBranch string
	IsMain       bool
	CreatedAt    time.Time
	Status       domain.TreeNodeStatus
}

// BuildForestParams holds the inputs for assembling the rendered forest.
type BuildForestParams struct {
	Nodes []ForestNode
}

// BuildForest assembles a forest of worktrees from their parent links. Unlike
// BuildSyncPlan it never fails: a parent with no managed worktree becomes a
// grey "virtual root", and a cycle in the parent chain is broken (the looping
// node is promoted to a root and flagged InCycle) so the tree stays finite.
//
// Roots are ordered main-first, then real orphan roots by creation date, then
// virtual roots alphabetically. Siblings are ordered by creation date.
func BuildForest(params BuildForestParams) domain.Forest {
	managed := make(map[string]ForestNode, len(params.Nodes))
	for _, n := range params.Nodes {
		managed[n.Branch] = n
	}

	inCycle := detectForestCycles(managed)

	childrenByParent := make(map[string][]ForestNode)
	virtualParents := make(map[string]struct{})
	var realRoots []ForestNode

	for _, n := range params.Nodes {
		if n.IsMain || n.SourceBranch == "" || inCycle[n.Branch] {
			realRoots = append(realRoots, n)
			continue
		}
		if _, ok := managed[n.SourceBranch]; ok {
			childrenByParent[n.SourceBranch] = append(childrenByParent[n.SourceBranch], n)
			continue
		}
		// Parent branch exists in metadata but has no managed worktree: group its
		// children under a virtual root standing in for the unmanaged branch.
		virtualParents[n.SourceBranch] = struct{}{}
		childrenByParent[n.SourceBranch] = append(childrenByParent[n.SourceBranch], n)
	}

	b := forestBuilder{
		childrenByParent: childrenByParent,
		inCycle:          inCycle,
		visited:          make(map[string]struct{}, len(params.Nodes)),
	}

	sortRealRoots(realRoots)
	roots := make([]domain.TreeNode, 0, len(realRoots)+len(virtualParents))
	for _, r := range realRoots {
		roots = append(roots, b.build(r))
	}

	virtualNames := make([]string, 0, len(virtualParents))
	for name := range virtualParents {
		virtualNames = append(virtualNames, name)
	}
	sort.Strings(virtualNames)
	for _, name := range virtualNames {
		roots = append(roots, b.buildVirtual(name))
	}

	return domain.Forest{Roots: roots}
}

type forestBuilder struct {
	childrenByParent map[string][]ForestNode
	inCycle          map[string]bool
	visited          map[string]struct{}
}

func (b *forestBuilder) build(n ForestNode) domain.TreeNode {
	b.visited[n.Branch] = struct{}{}

	status := n.Status
	if b.inCycle[n.Branch] {
		status.InCycle = true
	}

	node := domain.TreeNode{
		Branch:   n.Branch,
		Path:     n.Path,
		IsMain:   n.IsMain,
		Status:   status,
		Children: b.childrenOf(n.Branch),
	}
	return node
}

func (b *forestBuilder) buildVirtual(name string) domain.TreeNode {
	b.visited[name] = struct{}{}
	return domain.TreeNode{
		Branch:    name,
		IsVirtual: true,
		Children:  b.childrenOf(name),
	}
}

func (b *forestBuilder) childrenOf(parent string) []domain.TreeNode {
	children := b.childrenByParent[parent]
	sortSiblings(children)

	out := make([]domain.TreeNode, 0, len(children))
	for _, child := range children {
		if _, seen := b.visited[child.Branch]; seen {
			continue
		}
		out = append(out, b.build(child))
	}
	return out
}

// detectForestCycles reports which branches sit on a parent cycle, i.e. walking
// the SourceBranch chain returns to the branch itself.
func detectForestCycles(managed map[string]ForestNode) map[string]bool {
	inCycle := make(map[string]bool)
	for branch := range managed {
		seen := make(map[string]struct{})
		current := branch
		for {
			node, ok := managed[current]
			if !ok {
				break
			}
			if _, looped := seen[current]; looped {
				break
			}
			seen[current] = struct{}{}
			current = node.SourceBranch
			if current == branch {
				inCycle[branch] = true
				break
			}
		}
	}
	return inCycle
}

// sortRealRoots orders roots main-first then by creation date (oldest first).
func sortRealRoots(roots []ForestNode) {
	sort.SliceStable(roots, func(i, j int) bool {
		if roots[i].IsMain != roots[j].IsMain {
			return roots[i].IsMain
		}
		return roots[i].CreatedAt.Before(roots[j].CreatedAt)
	})
}

// sortSiblings orders children by creation date (oldest first).
func sortSiblings(nodes []ForestNode) {
	sort.SliceStable(nodes, func(i, j int) bool {
		return nodes[i].CreatedAt.Before(nodes[j].CreatedAt)
	})
}
