package wt

import (
	"github.com/spf13/cobra"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/output"
	"github.com/LucasPcq/wtm/internal/rules"
	"github.com/LucasPcq/wtm/internal/service/shell"
)

// newGoCmd creates the wtm go subcommand (fallback when shell wrapper is not configured).
func newGoCmd() *cobra.Command {
	return &cobra.Command{
		Use:   domain.CmdGo + " [branch]",
		Short: "Switch to a worktree",
		Long:  "Navigate to a worktree directory. Requires shell integration to work.",
		RunE:  runGo,
	}
}

func runGo(cmd *cobra.Command, _ []string) error {
	output.Frame(cmd.ErrOrStderr(), func() {
		output.Warning(cmd.ErrOrStderr(), "wtm go requires shell integration to change your working directory.")
		output.Blank(cmd.ErrOrStderr())
		output.Message(cmd.ErrOrStderr(), rules.ShellInitHint(shell.DetectShell()))
	})
	return nil
}
