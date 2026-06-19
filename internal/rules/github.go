package rules

import (
	"fmt"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"unicode"

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

// BranchTitleFromName generates a human-readable PR title from a branch name.
func BranchTitleFromName(branch string) string {
	prefixes := []string{"feature/", "fix/", "chore/", "hotfix/", "bugfix/", "refactor/", "docs/"}
	for _, prefix := range prefixes {
		if strings.HasPrefix(branch, prefix) {
			branch = strings.TrimPrefix(branch, prefix)
			break
		}
	}

	issueIDPattern := regexp.MustCompile(`^[A-Z]+-\d+-`)
	branch = issueIDPattern.ReplaceAllString(branch, "")
	branch = strings.ReplaceAll(branch, "-", " ")
	branch = strings.ReplaceAll(branch, "_", " ")
	branch = strings.TrimSpace(branch)
	if len(branch) == 0 {
		return branch
	}
	runes := []rune(branch)
	runes[0] = unicode.ToUpper(runes[0])
	return string(runes)
}

// ExtractPRNumber parses a GitHub PR URL and returns the PR number.
func ExtractPRNumber(url string) (int, error) {
	m := prNumberPattern.FindStringSubmatch(url)
	if len(m) < 2 {
		return 0, fmt.Errorf("no PR number found")
	}
	return strconv.Atoi(m[1])
}
