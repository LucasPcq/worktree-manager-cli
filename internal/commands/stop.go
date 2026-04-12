package commands

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/LucasPcq/wtm/internal/output"
	"github.com/LucasPcq/wtm/internal/service/process"
)

// newSvcStopCmd creates the wtm svc stop subcommand.
func newSvcStopCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "stop <service>",
		Short: "Stop a single service",
		Long:  "Stop an individual running service by name.",
		Args:  cobra.ExactArgs(1),
		RunE:  runStop,
	}
}

func runStop(cmd *cobra.Command, args []string) error {
	dir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get working directory: %w", err)
	}

	socketPath := process.SocketPath()
	if !process.IsDaemonRunning(socketPath) {
		output.Blank(cmd.OutOrStdout())
		output.Message(cmd.OutOrStdout(), "No services running.")
		output.Blank(cmd.OutOrStdout())
		return nil
	}

	client := process.NewClient(socketPath)
	resp, err := client.Send(process.Request{
		Action:  process.ActionStop,
		Name:    args[0],
		WorkDir: dir,
	})
	if err != nil {
		return fmt.Errorf("stop %s: %w", args[0], err)
	}
	if resp.Status == process.StatusError {
		return fmt.Errorf("stop %s: %s", args[0], resp.Message)
	}

	output.Blank(cmd.OutOrStdout())
	output.Success(cmd.OutOrStdout(), fmt.Sprintf("%s stopped", args[0]))
	output.Blank(cmd.OutOrStdout())
	return nil
}
