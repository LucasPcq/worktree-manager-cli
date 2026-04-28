package daemon

import (
	"github.com/spf13/cobra"

	"github.com/LucasPcq/wtm/internal/service/process"
)

// NewCmd creates the hidden wtm daemon command.
func NewCmd() *cobra.Command {
	return &cobra.Command{
		Use:    "daemon",
		Short:  "Run the service manager daemon (internal)",
		Hidden: true,
		RunE:   runDaemon,
	}
}

func runDaemon(_ *cobra.Command, _ []string) error {
	return process.RunDaemon(process.DaemonParams{
		SocketPath: process.SocketPath(),
	})
}
