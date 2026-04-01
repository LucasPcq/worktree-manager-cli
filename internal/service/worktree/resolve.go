package worktree

import (
	"strings"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/infra"
)

// ResolveParams holds inputs for resolving a branch query to a worktree path.
type ResolveParams struct {
	ProjectDir string
	Query      string
}

// ResolveResult indicates whether the resolution is direct or needs a picker.
type ResolveResult struct {
	Path      string
	Ambiguous bool
	Matches   []infra.GitWorktree
}

// Resolve resolves a branch query to a worktree path.
// Returns a direct path on exact match, or a list of candidates if ambiguous.
// An empty query returns all worktrees for the picker.
func Resolve(params ResolveParams) (ResolveResult, error) {
	worktrees, err := infra.ListWorktrees(infra.ListWorktreesParams{
		ProjectDir: params.ProjectDir,
	})
	if err != nil {
		return ResolveResult{}, err
	}

	if params.Query == "" {
		return ResolveResult{
			Ambiguous: true,
			Matches:   worktrees,
		}, nil
	}

	// Exact match first
	for _, wt := range worktrees {
		if wt.Branch == params.Query {
			return ResolveResult{Path: wt.Path}, nil
		}
	}

	// Substring match
	var matches []infra.GitWorktree
	for _, wt := range worktrees {
		if strings.Contains(wt.Branch, params.Query) {
			matches = append(matches, wt)
		}
	}

	if len(matches) == 1 {
		return ResolveResult{Path: matches[0].Path}, nil
	}

	if len(matches) > 1 {
		return ResolveResult{
			Ambiguous: true,
			Matches:   matches,
		}, nil
	}

	return ResolveResult{}, domain.ErrWorktreeNotFound
}
