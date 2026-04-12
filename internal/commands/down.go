package commands

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/LucasPcq/wtm/internal/config"
	"github.com/LucasPcq/wtm/internal/output"
	"github.com/LucasPcq/wtm/internal/service/process"
)

// newSvcDownCmd creates the wtm svc down subcommand.
func newSvcDownCmd() *cobra.Command {
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
		output.Blank(cmd.OutOrStdout())
		output.Message(cmd.OutOrStdout(), "No services running.")
		output.Blank(cmd.OutOrStdout())
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
		output.Blank(cmd.OutOrStdout())
		for _, svc := range services {
			resp, sendErr := client.Send(process.Request{
				Action:  process.ActionStop,
				Name:    svc.Name,
				WorkDir: dir,
			})
			if sendErr != nil {
				output.Error(cmd.ErrOrStderr(), fmt.Sprintf("%s: %v", svc.Name, sendErr))
				continue
			}
			if resp.Status == process.StatusError {
				output.Error(cmd.ErrOrStderr(), fmt.Sprintf("%s: %s", svc.Name, resp.Message))
				continue
			}
			output.Success(cmd.OutOrStdout(), fmt.Sprintf("%s stopped", svc.Name))
		}
		output.Blank(cmd.OutOrStdout())
		return nil
	}

	resp, err := client.Send(process.Request{Action: process.ActionStopAll})
	if err != nil {
		return fmt.Errorf("stop all services: %w", err)
	}
	if resp.Status == process.StatusError {
		return fmt.Errorf("stop all: %s", resp.Message)
	}

	output.Blank(cmd.OutOrStdout())
	output.Success(cmd.OutOrStdout(), "All services stopped.")
	output.Blank(cmd.OutOrStdout())
	return nil
}
