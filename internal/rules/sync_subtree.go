package rules

import "github.com/LucasPcq/wtm/internal/domain"

type SyncSubtreeParams struct {
	Nodes []domain.WorktreeNode
	Root  string
}

// SyncSubtree lists Root and every managed worktree hanging under it, parents
// before children. It is what a surface pre-checks when the gesture is "sync this
// worktree": the selection stays exact, only what arrives checked changes.
func SyncSubtree(params SyncSubtreeParams) []string {
	children := make(map[string][]string, len(params.Nodes))
	known := make(map[string]bool, len(params.Nodes))
	for _, node := range params.Nodes {
		known[node.Branch] = true
		children[node.SourceBranch] = append(children[node.SourceBranch], node.Branch)
	}
	if !known[params.Root] {
		return nil
	}

	seen := map[string]bool{params.Root: true}
	subtree := []string{params.Root}
	for index := 0; index < len(subtree); index++ {
		for _, child := range children[subtree[index]] {
			if seen[child] {
				continue
			}
			seen[child] = true
			subtree = append(subtree, child)
		}
	}
	return subtree
}
