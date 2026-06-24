package rules

import (
	"fmt"
	"slices"

	"github.com/LucasPcq/wtm/internal/domain"
)

// ValidatePRForCheckout returns an error if the PR cannot be checked out
// locally: fork PRs are intentionally out of scope (see README — wtm manages
// worktrees you can push to), and the branch must not already exist.
func ValidatePRForCheckout(pr domain.PRInfo, localBranches []string) error {
	if pr.IsFork {
		return fmt.Errorf("PR #%d is from a fork — wtm doesn't check out fork PRs by design; use `gh pr checkout %d`", pr.Number, pr.Number)
	}
	if slices.Contains(localBranches, pr.Branch) {
		return fmt.Errorf("local branch %q already exists — run `wtm clean %s` first", pr.Branch, pr.Branch)
	}
	return nil
}
