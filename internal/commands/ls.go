package commands

import (
	"errors"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/LucasPcq/wtm/internal/config"
	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/output"
	"github.com/LucasPcq/wtm/internal/service/state"
	"github.com/LucasPcq/wtm/internal/service/worktree"
)

// NewLsCmd creates the wtm ls command.
func NewLsCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "ls",
		Short: "List all worktrees",
		Long:  "List all git worktrees with their status (clean/dirty, commits ahead).",
		RunE:  runLs,
	}
}

func runLs(cmd *cobra.Command, _ []string) error {
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

	statuses, err := worktree.List(worktree.ListParams{
		ProjectDir: dir,
		Config:     cfg,
	})
	if err != nil {
		return fmt.Errorf("list worktrees: %w", err)
	}

	currentState, _ := state.Load()

	fmt.Fprint(cmd.OutOrStdout(), output.FormatWorktreeList(output.FormatWorktreeListParams{
		Statuses:     statuses,
		ActiveBranch: currentState.ActiveWorktree,
	}))

	return nil
}
