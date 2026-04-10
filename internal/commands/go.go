package commands

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/LucasPcq/wtm/internal/domain"
)

// newWtGoCmd creates the wtm wt go subcommand (fallback when shell wrapper is not configured).
func newWtGoCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "go [branch]",
		Short: "Switch to a worktree",
		Long:  "Navigate to a worktree directory. Requires shell integration to work.",
		RunE:  runGo,
	}
}

func runGo(cmd *cobra.Command, _ []string) error {
	fmt.Fprintln(cmd.ErrOrStderr(), "wtm go requires shell integration to change your working directory.")
	fmt.Fprintln(cmd.ErrOrStderr())
	fmt.Fprintln(cmd.ErrOrStderr(), domain.MsgShellInitHint)
	return nil
}
