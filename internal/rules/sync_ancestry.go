package rules

import "github.com/LucasPcq/wtm/internal/domain"

type SyncAncestryParams struct {
	Nodes []domain.WorktreeNode
	Leaf  string
}

// SyncAncestry lists what a worktree needs up to date before it can be rebased:
// its root, every managed ancestor down to it, and itself — parents first, in the
// order the cascade replays them. Rebasing a worktree without its ancestors
// replays it onto a parent nobody refreshed, which is the stale-parent problem
// the run otherwise has to ask about. Descendants are left out: they are their
// own gesture, made from their own row.
func SyncAncestry(params SyncAncestryParams) []string {
	byBranch := make(map[string]domain.WorktreeNode, len(params.Nodes))
	for _, node := range params.Nodes {
		byBranch[node.Branch] = node
	}
	current, known := byBranch[params.Leaf]
	if !known {
		return nil
	}

	seen := map[string]bool{current.Branch: true}
	chain := []string{current.Branch}
	for {
		parent, found := byBranch[current.SourceBranch]
		if !found || seen[parent.Branch] {
			break
		}
		seen[parent.Branch] = true
		chain = append(chain, parent.Branch)
		current = parent
	}

	ordered := make([]string, 0, len(chain))
	for index := len(chain) - 1; index >= 0; index-- {
		ordered = append(ordered, chain[index])
	}
	return ordered
}
