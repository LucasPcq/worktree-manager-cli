package commands

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/LucasPcq/wtm/internal/service/process"
)

// NewDownCmd creates the wtm down command.
func NewDownCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "down [service]",
		Short: "Stop running services",
		Long:  "Stop a single service by name, or all services if no argument is given.",
		Args:  cobra.MaximumNArgs(1),
		RunE:  runDown,
	}
}

func runDown(cmd *cobra.Command, args []string) error {
	socketPath := process.SocketPath()

	if !process.IsDaemonRunning(socketPath) {
		fmt.Fprintln(cmd.OutOrStdout(), "No services running.")
		return nil
	}

	client := process.NewClient(socketPath)

	if len(args) > 0 {
		resp, err := client.Send(process.Request{
			Action: process.ActionStop,
			Name:   args[0],
		})
		if err != nil {
			return fmt.Errorf("stop service: %w", err)
		}
		if resp.Status == process.StatusError {
			return fmt.Errorf("stop %s: %s", args[0], resp.Message)
		}
		fmt.Fprintf(cmd.OutOrStdout(), "  ✓ %s stopped\n", args[0])
		return nil
	}

	resp, err := client.Send(process.Request{Action: process.ActionStopAll})
	if err != nil {
		return fmt.Errorf("stop all services: %w", err)
	}
	if resp.Status == process.StatusError {
		return fmt.Errorf("stop all: %s", resp.Message)
	}

	fmt.Fprintln(cmd.OutOrStdout(), "  ✓ All services stopped.")
	return nil
}
