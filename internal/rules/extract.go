package rules

import (
	"strings"

	"github.com/LucasPcq/wtm/internal/domain"
)

// porcelainUntracked is the two-char status git reports for untracked files.
const porcelainUntracked = "??"

// ClassifyExtractStatus maps a `git status --porcelain` status code to the
// extraction status used for display and apply strategy. Untracked files ("??")
// are copied verbatim; deleted files (status "D" in either column) carry a
// deletion in the patch; everything else is treated as a content modification.
func ClassifyExtractStatus(status string) domain.ExtractFileStatus {
	if status == porcelainUntracked {
		return domain.ExtractStatusUntracked
	}
	if strings.Contains(status, "D") {
		return domain.ExtractStatusDeleted
	}
	return domain.ExtractStatusModified
}

// ExtractStatusLabel returns the short, human-readable tag shown next to a file
// in the interactive selector. The labels share the same width so the file names
// line up.
func ExtractStatusLabel(status domain.ExtractFileStatus) string {
	switch status {
	case domain.ExtractStatusUntracked:
		return "new"
	case domain.ExtractStatusDeleted:
		return "del"
	default:
		return "mod"
	}
}
