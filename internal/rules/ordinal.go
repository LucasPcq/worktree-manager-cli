package rules

import "github.com/LucasPcq/wtm/internal/domain"

// AllocateOrdinal returns the smallest number no live worktree holds. Ordinal 0
// belongs to the main checkout, so allocation starts at 1; taken may contain
// duplicates, zeroes and gaps, and a gap is reused rather than grown past —
// a worktree removed frees its number for the next one.
func AllocateOrdinal(taken []int) int {
	used := make(map[int]bool, len(taken))
	for _, n := range taken {
		used[n] = true
	}

	for candidate := domain.MainWorktreeOrdinal + 1; ; candidate++ {
		if !used[candidate] {
			return candidate
		}
	}
}
