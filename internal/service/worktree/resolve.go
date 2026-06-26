package worktree

import (
	"fmt"
	"strings"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/infra"
)

// ResolveSyncBranchesParams holds inputs for ResolveSyncBranches.
type ResolveSyncBranchesParams struct {
	ProjectDir string
	Queries    []string
}

// ResolveSyncBranches maps each query (a branch name or unambiguous substring) to
// a concrete worktree branch. An unknown query returns domain.ErrBranchNotFound;
// an ambiguous substring returns a descriptive error listing the candidates.
func ResolveSyncBranches(params ResolveSyncBranchesParams) ([]string, error) {
	worktrees, err := infra.ListWorktrees(infra.ListWorktreesParams{
		ProjectDir: params.ProjectDir,
	})
	if err != nil {
		return nil, err
	}

	branches := make([]string, 0, len(params.Queries))
	for _, query := range params.Queries {
		branch, matchErr := matchSyncBranch(worktrees, query)
		if matchErr != nil {
			return nil, matchErr
		}
		branches = append(branches, branch)
	}
	return branches, nil
}

func matchSyncBranch(worktrees []domain.GitWorktree, query string) (string, error) {
	for _, wt := range worktrees {
		if wt.Branch == query {
			return wt.Branch, nil
		}
	}

	var matches []string
	for _, wt := range worktrees {
		if strings.Contains(wt.Branch, query) {
			matches = append(matches, wt.Branch)
		}
	}

	if len(matches) == 1 {
		return matches[0], nil
	}
	if len(matches) > 1 {
		return "", fmt.Errorf("%q is ambiguous: matches %s", query, strings.Join(matches, ", "))
	}
	return "", fmt.Errorf("%w: %s", domain.ErrBranchNotFound, query)
}

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
