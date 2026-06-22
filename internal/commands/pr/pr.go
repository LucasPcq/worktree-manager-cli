package pr

import (
	"github.com/spf13/cobra"

	"github.com/LucasPcq/wtm/internal/domain"
)

// NewCmd creates the wtm pr command group.
func NewCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     domain.CmdPr,
		Short:   "Manage pull requests",
		GroupID: domain.CmdGroupCore,
	}

	cmd.AddCommand(newListCmd())
	cmd.AddCommand(newCheckoutCmd())

	return cmd
}
