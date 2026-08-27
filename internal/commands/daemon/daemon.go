package daemon

import (
	"github.com/spf13/cobra"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/service/process"
)

// NewCmd creates the hidden wtm daemon command.
func NewCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:    domain.CmdDaemon,
		Short:  "Run the service manager daemon (internal)",
		Hidden: true,
		RunE:   runDaemon,
	}
	cmd.Flags().Int(domain.FlagProxyPort, 0, "Port the run proxy serves named job URLs on (0 disables it)")
	return cmd
}

func runDaemon(cmd *cobra.Command, _ []string) error {
	proxyPort, _ := cmd.Flags().GetInt(domain.FlagProxyPort)
	return process.RunDaemon(process.DaemonParams{
		SocketPath: process.SocketPath(),
		ProxyPort:  proxyPort,
	})
}
