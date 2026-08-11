package rules

import (
	"fmt"
	"strings"

	"github.com/LucasPcq/wtm/internal/domain"
)

// ParsePorcelainZ parses the output of `git status --porcelain -z`. The -z form
// terminates every record with NUL and disables path quoting, so paths come out
// verbatim — spaces, non-ASCII and all. A rename or copy record is followed by a
// second field holding the original path; it must be consumed with its record or
// the whole stream desynchronizes. Ignored entries are dropped.
func ParsePorcelainZ(out []byte) ([]domain.PorcelainEntry, error) {
	fields := strings.Split(string(out), domain.PorcelainFieldSep)

	var entries []domain.PorcelainEntry
	for i := 0; i < len(fields); i++ {
		record := fields[i]
		if record == "" {
			continue
		}
		if len(record) <= domain.PorcelainPathOffset {
			return nil, fmt.Errorf("%w: %q", domain.ErrPorcelainMalformed, record)
		}

		status := record[:domain.PorcelainPathOffset-1]
		entry := domain.PorcelainEntry{Status: status, Path: record[domain.PorcelainPathOffset:]}

		if isRenameStatus(status) {
			if i+1 >= len(fields) || fields[i+1] == "" {
				return nil, fmt.Errorf("%w: rename record %q has no origin path", domain.ErrPorcelainMalformed, record)
			}
			entry.OrigPath = fields[i+1]
			i++
		}

		if status == domain.PorcelainIgnored {
			continue
		}
		entries = append(entries, entry)
	}

	return entries, nil
}

// isRenameStatus reports whether a record carries a second, origin-path field.
// Git puts the R/C code in the index column, but both columns are checked so an
// unexpected layout cannot desynchronize the stream.
func isRenameStatus(status string) bool {
	return strings.Contains(status, domain.PorcelainRename) ||
		strings.Contains(status, domain.PorcelainCopy)
}
