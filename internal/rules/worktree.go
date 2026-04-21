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
