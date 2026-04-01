// Package clean builds interactive huh forms for the wtm clean command.
package clean

import (
	"errors"
	"fmt"
	"os"
	"sort"

	"github.com/charmbracelet/huh"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/infra"
	"github.com/LucasPcq/wtm/internal/service/worktree"
)

// ConfirmResult holds the user's choices from the confirm dialog.
type ConfirmResult struct {
	Confirmed bool
	Force     bool
}

// RunConfirm displays warnings from pre-deletion checks and asks for confirmation.
// Returns ErrUserAborted if the user declines.
func RunConfirm(check worktree.CleanCheckResult) (ConfirmResult, error) {
	printWarnings(check)

	fmt.Fprintf(os.Stderr, "\nWill delete:\n  worktree  %s\n  branch    %s\n\n", check.WorktreePath, check.Branch)

	var choice string

	options := []huh.Option[string]{
		huh.NewOption("Yes, delete", "yes"),
	}

	if check.HasWarnings() {
		options = append(options, huh.NewOption("Yes, force delete (bypass all checks)", "force"))
	}

	options = append(options, huh.NewOption("No, cancel", "no"))

	form := huh.NewForm(
		huh.NewGroup(
			huh.NewSelect[string]().
				Title("Proceed with deletion?").
				Options(options...).
				Value(&choice),
		),
	)

	if err := form.Run(); err != nil {
		if errors.Is(err, huh.ErrUserAborted) {
			return ConfirmResult{}, domain.ErrUserAborted
		}
		return ConfirmResult{}, err
	}

	if choice == "no" {
		return ConfirmResult{}, domain.ErrUserAborted
	}

	return ConfirmResult{
		Confirmed: true,
		Force:     choice == "force",
	}, nil
}

// RunWorktreePicker displays a filterable picker of worktrees (excluding parent).
// Returns ErrUserAborted on Ctrl+C.
func RunWorktreePicker(projectDir string) (string, error) {
	worktrees, err := infra.ListWorktrees(infra.ListWorktreesParams{ProjectDir: projectDir})
	if err != nil {
		return "", fmt.Errorf("list worktrees: %w", err)
	}

	var options []huh.Option[string]
	for _, wt := range worktrees {
		if wt.IsMain {
			continue
		}
		options = append(options, huh.NewOption(wt.Branch, wt.Branch))
	}

	sort.Slice(options, func(i, j int) bool {
		return options[i].Value < options[j].Value
	})

	if len(options) == 0 {
		return "", fmt.Errorf("no worktrees to clean (only the parent worktree exists)")
	}

	var selected string

	form := huh.NewForm(
		huh.NewGroup(
			huh.NewSelect[string]().
				Title("Select worktree to clean").
				Description("The parent worktree cannot be cleaned").
				Options(options...).
				Filtering(true).
				Value(&selected),
		),
	)

	if err := form.Run(); err != nil {
		if errors.Is(err, huh.ErrUserAborted) {
			return "", domain.ErrUserAborted
		}
		return "", err
	}

	return selected, nil
}

func printWarnings(check worktree.CleanCheckResult) {
	if check.IsDirty {
		fmt.Fprintf(os.Stderr, "⚠ Worktree has uncommitted changes\n")
	}
	if check.UnpushedCommits > 0 {
		fmt.Fprintf(os.Stderr, "⚠ %d commit(s) not pushed to remote\n", check.UnpushedCommits)
	}
	if check.HasOpenPR {
		fmt.Fprintf(os.Stderr, "⚠ Open PR: %s\n", check.PRUrl)
	}
}
