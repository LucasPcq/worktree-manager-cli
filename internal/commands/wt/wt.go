package wt

import (
	"github.com/spf13/cobra"

	"github.com/LucasPcq/wtm/internal/domain"
)

// NewCmd creates the wtm wt command group.
func NewCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     domain.CmdWt,
		Short:   "Manage worktrees",
		GroupID: domain.CmdGroupCore,
	}

	cmd.AddCommand(newListCmd())
	cmd.AddCommand(newCreateCmd())
	cmd.AddCommand(newCleanCmd())
	cmd.AddCommand(newSyncCmd())
	cmd.AddCommand(newGoCmd())
	cmd.AddCommand(newSwitchCmd())
	cmd.AddCommand(newExtractCmd())

	return cmd
}
