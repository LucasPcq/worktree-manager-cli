package github

import (
	"fmt"
	"slices"

	"github.com/LucasPcq/wtm/internal/domain"
)

// ValidatePRForCheckout returns an error if the PR cannot be checked out
// locally: forks are unsupported, and the branch must not already exist.
func ValidatePRForCheckout(pr domain.PRInfo, localBranches []string) error {
	if pr.IsFork {
		return fmt.Errorf("PR #%d is from a fork — fork support is tracked in LUC-40", pr.Number)
	}
	if slices.Contains(localBranches, pr.Branch) {
		return fmt.Errorf("local branch %q already exists — run `wtm wt clean %s` first", pr.Branch, pr.Branch)
	}
	return nil
}
