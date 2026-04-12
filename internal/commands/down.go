package commands

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/LucasPcq/wtm/internal/config"
	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/output"
	"github.com/LucasPcq/wtm/internal/service/process"
)

// newSvcDownCmd creates the wtm svc down subcommand.
func newSvcDownCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "down [profile]",
		Short: "Stop a service profile",
		Long:  "Stop all services in a profile.\nWithout arguments, stops all running services.",
		Args:  cobra.MaximumNArgs(1),
		RunE:  runDown,
	}
	cmd.Flags().String(domain.FlagOutput, domain.OutputText, "Output format: text or json")
	return cmd
}

func runDown(cmd *cobra.Command, args []string) error {
	format, _ := cmd.Flags().GetString(domain.FlagOutput)
	socketPath := process.SocketPath()

	if !process.IsDaemonRunning(socketPath) {
		if format == domain.OutputJSON {
			return output.WriteServiceResultsJSON(cmd.OutOrStdout(), nil)
		}
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

		root, err := projectRoot(dir)
		if err != nil {
			return err
		}

		svcCfg, err := config.LoadServices(root)
		if err != nil {
			return fmt.Errorf("load services config: %w", err)
		}

		profile, ok := svcCfg.FindProfile(args[0])
		if !ok {
			return fmt.Errorf("profile %q not found in config", args[0])
		}

		services := svcCfg.ProfileServices(profile)
		results := make([]output.ServiceActionResult, 0, len(services))
		if format != domain.OutputJSON {
			output.Blank(cmd.OutOrStdout())
		}
		for _, svc := range services {
			resp, sendErr := client.Send(process.Request{
				Action:  process.ActionStop,
				Name:    svc.Name,
				WorkDir: dir,
			})
			if sendErr != nil {
				results = append(results, output.ServiceActionResult{Name: svc.Name, Status: domain.ServiceActionError, Message: sendErr.Error()})
				if format != domain.OutputJSON {
					output.Error(cmd.ErrOrStderr(), fmt.Sprintf("%s: %v", svc.Name, sendErr))
				}
				continue
			}
			if resp.Status == process.StatusError {
				results = append(results, output.ServiceActionResult{Name: svc.Name, Status: domain.ServiceActionError, Message: resp.Message})
				if format != domain.OutputJSON {
					output.Error(cmd.ErrOrStderr(), fmt.Sprintf("%s: %s", svc.Name, resp.Message))
				}
				continue
			}
			results = append(results, output.ServiceActionResult{Name: svc.Name, Status: domain.ServiceActionStopped})
			if format != domain.OutputJSON {
				output.Success(cmd.OutOrStdout(), fmt.Sprintf("%s stopped", svc.Name))
			}
		}
		if format == domain.OutputJSON {
			return output.WriteServiceResultsJSON(cmd.OutOrStdout(), results)
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

	if format == domain.OutputJSON {
		stopped := make([]output.ServiceActionResult, 0, len(resp.Services))
		for _, svc := range resp.Services {
			stopped = append(stopped, output.ServiceActionResult{Name: svc.Name, Status: domain.ServiceActionStopped})
		}
		return output.WriteServiceResultsJSON(cmd.OutOrStdout(), stopped)
	}

	output.Blank(cmd.OutOrStdout())
	output.Success(cmd.OutOrStdout(), "All services stopped.")
	output.Blank(cmd.OutOrStdout())
	return nil
}
