package output

import (
	"fmt"
	"io"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/styles"
)

// WriteExtractJSON writes the JSON payload for `wt extract`.
func WriteExtractJSON(w io.Writer, result domain.ExtractResult) error {
	return encodeJSON(w, result)
}

// PrintExtractResult renders the human-facing summary of an extraction: a
// headline, the moved files with colored status tags, and the target worktree.
func PrintExtractResult(w io.Writer, result domain.ExtractResult) {
	verb := "Moved"
	if result.Kept {
		verb = "Copied"
	}

	Blank(w)
	Success(w, fmt.Sprintf("%s %s to %s", verb, pluralizeFiles(len(result.Files)), styles.Bold.Render(result.TargetBranch)))
	Blank(w)
	for _, f := range result.Files {
		fmt.Fprintf(w, "%s%s  %s  %s\n", Indent, Indent, extractTag(f.Status), f.Path)
	}
	Blank(w)
	InfoLine(w, "worktree", result.TargetPath)
	Blank(w)
}

// extractTag renders the aligned, colored status tag for a file: yellow "mod",
// green "new", red "del".
func extractTag(status domain.ExtractFileStatus) string {
	switch status {
	case domain.ExtractStatusUntracked:
		return styles.Success.Render("new")
	case domain.ExtractStatusDeleted:
		return styles.DangerText.Render("del")
	default:
		return styles.Warning.Render("mod")
	}
}

func pluralizeFiles(n int) string {
	if n == 1 {
		return "1 file"
	}
	return fmt.Sprintf("%d files", n)
}
