package commands

import (
	"errors"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/LucasPcq/wtm/internal/config"
	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/infra"
	"github.com/LucasPcq/wtm/internal/service/worktree"
	gopicker "github.com/LucasPcq/wtm/internal/tui/go"
)

// NewFocusCmd creates the wtm focus command.
func NewFocusCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "focus [branch]",
		Short: "Switch the active environment",
		Long:  "Run on_blur hooks on the current worktree and on_focus hooks on the target.\nWithout arguments, shows an interactive picker.",
		Args:  cobra.MaximumNArgs(1),
		RunE:  runFocus,
	}

	cmd.Flags().Bool(domain.FlagOff, false, "Stop the active environment and clear state")

	return cmd
}

func runFocus(cmd *cobra.Command, args []string) error {
	off, _ := cmd.Flags().GetBool(domain.FlagOff)

	dir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get working directory: %w", err)
	}

	cfg, err := config.Load(config.LoadParams{ProjectDir: dir})
	if errors.Is(err, domain.ErrConfigNotFound) {
		fmt.Fprintln(cmd.ErrOrStderr(), "No .wtm.toml found. Run `wtm init` first.")
		return nil
	}
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	if off {
		if err := worktree.Unfocus(worktree.UnfocusParams{Config: cfg}); err != nil {
			return err
		}
		fmt.Fprintln(cmd.OutOrStdout(), "✓ Environment stopped.")
		return nil
	}

	branch, err := resolveFocusBranch(args, dir)
	if err != nil {
		if errors.Is(err, domain.ErrUserAborted) {
			return nil
		}
		return err
	}

	if err := worktree.Focus(worktree.FocusParams{
		ProjectDir: dir,
		Branch:     branch,
		Config:     cfg,
	}); err != nil {
		return err
	}

	fmt.Fprintf(cmd.OutOrStdout(), "✓ Focused on %s\n", branch)
	return nil
}

func resolveFocusBranch(args []string, dir string) (string, error) {
	if len(args) > 0 {
		return args[0], nil
	}

	worktrees, err := infra.ListWorktrees(infra.ListWorktreesParams{ProjectDir: dir})
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
