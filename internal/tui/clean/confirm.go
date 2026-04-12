// Package clean builds interactive pickers for the wtm clean command.
package clean

import (
	"fmt"
	"os"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/output"
	"github.com/LucasPcq/wtm/internal/service/worktree"
	"github.com/LucasPcq/wtm/internal/tui/components"
)

// ConfirmResult holds the user's choices from the confirm dialog.
type ConfirmResult struct {
	Confirmed bool
	Force     bool
}

// RunConfirm displays warnings from pre-deletion checks and asks for confirmation.
// Returns ErrUserAborted if the user declines.
func RunConfirm(check worktree.CleanCheckResult) (ConfirmResult, error) {
	w := os.Stderr

	output.Blank(w)
	printWarnings(w, check)

	output.Blank(w)
	output.SectionTitle(w, "Will delete:")
	output.InfoLine(w, "worktree", check.WorktreePath)
	output.InfoLine(w, "branch  ", check.Branch)
	output.Blank(w)

	items := []components.SelectItem{
		{Label: "Yes, delete", Value: "yes"},
	}

	if check.HasWarnings() {
		items = append(items,
			components.SelectItem{Separator: true},
			components.SelectItem{Label: "Yes, force delete (bypass all checks)", Value: "force", Danger: true},
		)
	}

	items = append(items,
		components.SelectItem{Separator: true},
		components.SelectItem{Label: "No, cancel", Value: "no"},
	)

	sl := components.NewSelectList(components.NewSelectListParams{
		Title: "Proceed with deletion?",
		Items: items,
	})

	choice, err := components.RunStandaloneSelect(sl)
	if err != nil {
		return ConfirmResult{}, domain.ErrUserAborted
	}

	if choice == "no" {
		return ConfirmResult{}, domain.ErrUserAborted
	}

	return ConfirmResult{
		Confirmed: true,
		Force:     choice == "force",
	}, nil
}

func printWarnings(w *os.File, check worktree.CleanCheckResult) {
	if check.IsDirty {
		output.Warning(w, "Worktree has uncommitted changes")
	}
	if check.UnpushedCommits > 0 {
		output.Warning(w, fmt.Sprintf("%d commit(s) not pushed to remote", check.UnpushedCommits))
	}
	if check.HasOpenPR {
		output.Warning(w, "Open PR: "+check.PRUrl)
	}
}
