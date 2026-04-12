package commands

import (
	"github.com/spf13/cobra"

	"github.com/LucasPcq/wtm/internal/domain"
)

// NewWtCmd creates the wtm wt command group.
func NewWtCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "wt",
		Short:   "Manage worktrees",
		GroupID: domain.CmdGroupCore,
	}

	cmd.AddCommand(newWtListCmd())
	cmd.AddCommand(newWtCreateCmd())
	cmd.AddCommand(newWtCleanCmd())
	cmd.AddCommand(newWtGoCmd())
	cmd.AddCommand(newWtSwitchCmd())

	return cmd
}
