package rules

import (
	"fmt"

	"github.com/LucasPcq/wtm/internal/domain"
)

// ValidatePRForCheckout returns an error if the PR cannot be checked out
// locally: fork PRs are intentionally out of scope (see README — wtm manages
// worktrees you can push to). An existing local branch is not a conflict — the
// worktree checks it out as-is; only a branch already held by another worktree
// is refused, downstream in worktree.Create.
func ValidatePRForCheckout(pr domain.PRInfo) error {
	if pr.IsFork {
		return fmt.Errorf("PR #%d is from a fork — wtm doesn't check out fork PRs by design; use `gh pr checkout %d`", pr.Number, pr.Number)
	}
	return nil
}

// PRFilterParams selects which open PRs to list. Review takes precedence over
// Mine when both are set.
type PRFilterParams struct {
	Review bool
	Mine   bool
}

// PRFilterFor maps the --review / --mine flags to a PR list filter.
func PRFilterFor(params PRFilterParams) domain.PRFilter {
	switch {
	case params.Review:
		return domain.PRFilterReviewRequested
	case params.Mine:
		return domain.PRFilterMine
	default:
		return domain.PRFilterAll
	}
}

// FirstNonEmpty returns the first non-empty string in values, or "" if none.
func FirstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}
