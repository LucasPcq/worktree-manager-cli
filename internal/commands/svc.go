package commands

import (
	"github.com/spf13/cobra"

	"github.com/LucasPcq/wtm/internal/domain"
)

// NewSvcCmd creates the wtm svc command group.
func NewSvcCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "svc",
		Short:   "Manage services",
		GroupID: domain.CmdGroupCore,
	}

	cmd.AddCommand(newSvcListCmd())
	cmd.AddCommand(newSvcPsCmd())
	cmd.AddCommand(newSvcUpCmd())
	cmd.AddCommand(newSvcDownCmd())
	cmd.AddCommand(newSvcStartCmd())
	cmd.AddCommand(newSvcStopCmd())
	cmd.AddCommand(newSvcLogsCmd())

	return cmd
}
