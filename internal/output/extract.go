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
	InfoLine(w, "source", result.SourceBranch+" · "+sourceState(result.Kept))
	InfoLine(w, "worktree", result.TargetPath)
	Blank(w)
}

// sourceState describes what happened to the source worktree after a clean
// extraction: cleaned (move) or kept (copy).
func sourceState(kept bool) string {
	if kept {
		return styles.Warning.Render("kept")
	}
	return styles.Success.Render("clean")
}

// PrintExtractConflicts renders the rebase-style summary when changes were
// applied to the target with conflict markers, with both next-step paths.
func PrintExtractConflicts(w io.Writer, result domain.ExtractResult) {
	Blank(w)
	Warning(w, fmt.Sprintf("Applied to %s with conflicts", styles.Bold.Render(result.TargetBranch)))
	Blank(w)
	SectionTitle(w, "Conflicts to resolve in "+result.TargetBranch)
	for _, f := range result.Conflicts {
		fmt.Fprintf(w, "%s%s%s\n", Indent, Indent, styles.Warning.Render(f))
	}
	if len(result.Files) > len(result.Conflicts) {
		Blank(w)
		Message(w, "The other files were applied cleanly.")
	}
	Blank(w)
	Message(w, fmt.Sprintf("Nothing was removed from %s — your changes are safe there.", result.SourceBranch))
	Message(w, fmt.Sprintf("• Finish the split: resolve the conflicts in %s, then discard the same files in %s.", result.TargetBranch, result.SourceBranch))
	Message(w, fmt.Sprintf("• Undo: discard the applied changes in %s — %s stays untouched.", result.TargetBranch, result.SourceBranch))
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
