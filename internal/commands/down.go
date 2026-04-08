package commands

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/LucasPcq/wtm/internal/config"
	"github.com/LucasPcq/wtm/internal/service/process"
)

// NewDownCmd creates the wtm down command.
func NewDownCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "down [profile]",
		Short: "Stop a service profile",
		Long:  "Stop all services in a profile.\nWithout arguments, stops all running services.",
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
		dir, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("get working directory: %w", err)
		}

		svcCfg, err := config.LoadServices(dir)
		if err != nil {
			return fmt.Errorf("load services config: %w", err)
		}

		profile, ok := svcCfg.FindProfile(args[0])
		if !ok {
			return fmt.Errorf("profile %q not found in config", args[0])
		}

		services := svcCfg.ProfileServices(profile)
		for _, svc := range services {
			resp, sendErr := client.Send(process.Request{
				Action:  process.ActionStop,
				Name:    svc.Name,
				WorkDir: dir,
			})
			if sendErr != nil {
				fmt.Fprintf(cmd.ErrOrStderr(), "  ✗ %s: %v\n", svc.Name, sendErr)
				continue
			}
			if resp.Status == process.StatusError {
				fmt.Fprintf(cmd.ErrOrStderr(), "  ✗ %s: %s\n", svc.Name, resp.Message)
				continue
			}
			fmt.Fprintf(cmd.OutOrStdout(), "  ✓ %s stopped\n", svc.Name)
		}
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
