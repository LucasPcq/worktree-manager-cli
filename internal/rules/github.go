package rules

import (
	"fmt"
	"regexp"
	"slices"
	"strconv"

	"github.com/LucasPcq/wtm/internal/domain"
)

var prNumberPattern = regexp.MustCompile(`/pull/(\d+)$`)

// ValidatePRForCheckout returns an error if the PR cannot be checked out
// locally: fork PRs are intentionally out of scope (see README — wtm manages
// worktrees you can push to), and the branch must not already exist.
func ValidatePRForCheckout(pr domain.PRInfo, localBranches []string) error {
	if pr.IsFork {
		return fmt.Errorf("PR #%d is from a fork — wtm doesn't check out fork PRs by design; use `gh pr checkout %d`", pr.Number, pr.Number)
	}
	if slices.Contains(localBranches, pr.Branch) {
		return fmt.Errorf("local branch %q already exists — run `wtm wt clean %s` first", pr.Branch, pr.Branch)
	}
	return nil
}

// ExtractPRNumber parses a GitHub PR URL and returns the PR number.
func ExtractPRNumber(url string) (int, error) {
	m := prNumberPattern.FindStringSubmatch(url)
	if len(m) < 2 {
		return 0, fmt.Errorf("no PR number found")
	}
	return strconv.Atoi(m[1])
}
