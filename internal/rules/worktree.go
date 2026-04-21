package rules

import (
	"sort"
	"strings"

	"github.com/LucasPcq/wtm/internal/domain"
)

// SanitizeBranchName replaces slashes with dashes for use as a directory name.
func SanitizeBranchName(name string) string {
	return strings.ReplaceAll(name, "/", "-")
}

// ResolveEnvStrategy returns the override strategy if set, otherwise the default.
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
