package worktree

import (
	"strings"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/infra"
)

// Resolve resolves a branch query to a worktree path.
// Returns a direct path on exact match, or a list of candidates if ambiguous.
// An empty query returns all worktrees for the picker.
func Resolve(params domain.ResolveParams) (domain.ResolveResult, error) {
	worktrees, err := infra.ListWorktrees(infra.ListWorktreesParams{
		ProjectDir: params.ProjectDir,
	})
	if err != nil {
		return domain.ResolveResult{}, err
	}

	if params.Query == "" {
		return domain.ResolveResult{
			Ambiguous: true,
			Matches:   worktrees,
		}, nil
	}

	// Exact match first
	for _, wt := range worktrees {
		if wt.Branch == params.Query {
			return domain.ResolveResult{Path: wt.Path}, nil
		}
	}

	// Substring match
	var matches []domain.GitWorktree
	for _, wt := range worktrees {
		if strings.Contains(wt.Branch, params.Query) {
			matches = append(matches, wt)
		}
	}

	if len(matches) == 1 {
		return domain.ResolveResult{Path: matches[0].Path}, nil
	}

	if len(matches) > 1 {
		return domain.ResolveResult{
			Ambiguous: true,
			Matches:   matches,
		}, nil
	}

	return domain.ResolveResult{}, domain.ErrWorktreeNotFound
}
