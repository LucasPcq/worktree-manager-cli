package rules

import "github.com/LucasPcq/wtm/internal/domain"

// CountChanges distributes porcelain entries by column: X is the index, Y is
// the working tree. A file can count in both.
func CountChanges(entries []domain.PorcelainEntry) domain.WorkingChanges {
	changes := domain.WorkingChanges{Files: entries}
	for _, entry := range entries {
		if entry.Status == domain.PorcelainUntracked {
			changes.Untracked++
			continue
		}
		if len(entry.Status) < 2 {
			continue
		}
		if entry.Status[0] != domain.PorcelainUnmodified {
			changes.Staged++
		}
		if entry.Status[1] != domain.PorcelainUnmodified {
			changes.Modified++
		}
	}
	return changes
}
