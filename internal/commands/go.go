package commands

import (
	"github.com/spf13/cobra"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/output"
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
	output.Blank(cmd.ErrOrStderr())
	output.Warning(cmd.ErrOrStderr(), "wtm go requires shell integration to change your working directory.")
	output.Blank(cmd.ErrOrStderr())
	output.Message(cmd.ErrOrStderr(), domain.MsgShellInitHint)
	output.Blank(cmd.ErrOrStderr())
	return nil
}
