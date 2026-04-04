package commands

import (
	"github.com/spf13/cobra"

	"github.com/LucasPcq/wtm/internal/service/process"
)

// NewDaemonCmd creates the hidden wtm daemon command.
func NewDaemonCmd() *cobra.Command {
	return &cobra.Command{
		Use:    "daemon",
		Short:  "Run the service manager daemon (internal)",
		Hidden: true,
		RunE:   runDaemonCmd,
	}
}

func runDaemonCmd(_ *cobra.Command, _ []string) error {
	return process.RunDaemon(process.DaemonParams{
		SocketPath: process.SocketPath(),
	})
}
