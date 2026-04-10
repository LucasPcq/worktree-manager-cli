package commands

import (
	"errors"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/infra"
	"github.com/LucasPcq/wtm/internal/service/worktree"
	gopicker "github.com/LucasPcq/wtm/internal/tui/go"
)

// newWtFocusCmd creates the wtm wt focus subcommand.
func newWtFocusCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "focus [branch]",
		Short: "Switch active worktree and run focus/blur hooks",
		Long:  "Run on_blur hooks on the current worktree and on_focus hooks on the target.\nWithout arguments, shows an interactive picker.",
		Args:  cobra.MaximumNArgs(1),
		RunE:  runFocus,
	}

	cmd.Flags().Bool(domain.FlagOff, false, "Run blur hooks on active worktree and clear state")

	return cmd
}

func runFocus(cmd *cobra.Command, args []string) error {
	off, _ := cmd.Flags().GetBool(domain.FlagOff)

	dir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get working directory: %w", err)
	}

	result, ok := loadConfig(cmd, dir)
	if !ok {
		return nil
	}

	if off {
		if err := worktree.Unfocus(worktree.UnfocusParams{ProjectDir: result.ProjectDir, Config: result.Config}); err != nil {
			return err
		}
		fmt.Fprintln(cmd.OutOrStdout(), "✓ Active worktree cleared.")
		return nil
	}

	branch, err := resolveFocusBranch(args, result.ProjectDir)
	if err != nil {
		if errors.Is(err, domain.ErrUserAborted) {
			return nil
		}
		return err
	}

	if err := worktree.Focus(worktree.FocusParams{
		ProjectDir: result.ProjectDir,
		Branch:     branch,
		Config:     result.Config,
	}); err != nil {
		return err
	}

	fmt.Fprintf(cmd.OutOrStdout(), "✓ Focused on %s\n", branch)
	return nil
}

func resolveFocusBranch(args []string, projectDir string) (string, error) {
	if len(args) > 0 {
		return args[0], nil
	}

	worktrees, err := infra.ListWorktrees(infra.ListWorktreesParams{ProjectDir: projectDir})
	if err != nil {
		return "", fmt.Errorf("list worktrees: %w", err)
	}

	path, err := gopicker.RunWorktreePicker(worktrees)
	if err != nil {
		return "", err
	}

	for _, wt := range worktrees {
		if wt.Path == path {
			return wt.Branch, nil
		}
	}

	return "", domain.ErrWorktreeNotFound
}
