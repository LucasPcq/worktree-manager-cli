package rules

import (
	"net/url"
	"path/filepath"
	"sort"
	"strings"

	"github.com/LucasPcq/wtm/internal/domain"
)

// SanitizeBranchName replaces slashes with dashes for use as a directory name.
func SanitizeBranchName(name string) string {
	return strings.ReplaceAll(name, "/", "-")
}

// EncodeBranchSegment percent-encodes a branch name for use as a single
// path segment under <state-dir>/worktrees/. Slashes become %2F so that
// `feat/x` doesn't create a nested directory.
func EncodeBranchSegment(branch string) string {
	return url.PathEscape(branch)
}

// WorktreeMetaDir returns <state-dir>/worktrees/<encoded-branch>/, the
// directory holding meta.json for one worktree.
func WorktreeMetaDir(stateDir, branch string) string {
	return filepath.Join(stateDir, domain.WorktreesSubdir, EncodeBranchSegment(branch))
}

// ResolveEnvStrategy returns override cast to EnvStrategy if non-empty, otherwise strategy.
// The caller is responsible for ensuring override is a valid EnvStrategy value;
// validation occurs at the config boundary before this function is called.
func ResolveEnvStrategy(strategy domain.EnvStrategy, override string) domain.EnvStrategy {
	if override != "" {
		return domain.EnvStrategy(override)
	}
	return strategy
}

// SortStatuses sorts worktree statuses: parent first, then children by creation date (oldest first).
func SortStatuses(statuses []domain.WorktreeStatus) {
	sort.SliceStable(statuses, func(i, j int) bool {
		if statuses[i].IsParent != statuses[j].IsParent {
			return statuses[i].IsParent
		}
		return statuses[i].CreatedAt.Before(statuses[j].CreatedAt)
	})
}

// IsPathWithin reports whether target is dir itself or nested under it.
func IsPathWithin(dir, target string) bool {
	rel, err := filepath.Rel(dir, target)
	if err != nil {
		return false
	}
	return rel == "." || (rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)))
}

// HasWarnings reports whether any condition warrants showing the force option.
func HasWarnings(result domain.CleanCheckResult) bool {
	return result.UnpushedCommits > 0 || result.HasOpenPR || result.IsDirty
}

// FilterStatusesByMatches returns the subset of statuses whose branch appears in matches.
// Returns statuses unchanged when matches is empty.
func FilterStatusesByMatches(statuses []domain.WorktreeStatus, matches []domain.GitWorktree) []domain.WorktreeStatus {
	if len(matches) == 0 {
		return statuses
	}
	allowed := make(map[string]struct{}, len(matches))
	for _, m := range matches {
		allowed[m.Branch] = struct{}{}
	}
	out := make([]domain.WorktreeStatus, 0, len(matches))
	for _, s := range statuses {
		if _, ok := allowed[s.Branch]; ok {
			out = append(out, s)
		}
	}
	return out
}
