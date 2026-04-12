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
		Short: "Stop services running in the current worktree",
		Long:  "Stop services running in the current worktree.\nWith a profile argument, stops only that profile's services.\nServices running in other worktrees are never touched.",
		Args:  cobra.MaximumNArgs(1),
		RunE:  runDown,
	}
	cmd.Flags().String(domain.FlagOutput, domain.OutputText, "Output format: text or json")
	cmd.Flags().Bool(domain.FlagAll, false, "Stop services across every worktree (bypasses per-worktree scoping)")
	return cmd
}

func runDown(cmd *cobra.Command, args []string) error {
	format, _ := cmd.Flags().GetString(domain.FlagOutput)
	all, _ := cmd.Flags().GetBool(domain.FlagAll)

	if all && len(args) > 0 {
		return fmt.Errorf("--all cannot be combined with a profile argument")
	}

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
		for _, svc := range services {
			stopSpinner := startSpinner(cmd.ErrOrStderr(), fmt.Sprintf("Stopping %s...", svc.Name))
			resp, sendErr := client.Send(process.Request{
				Action:  process.ActionStop,
				Name:    svc.Name,
				WorkDir: dir,
			})
			stopSpinner()
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

	req := process.Request{Action: process.ActionStopAll}
	if !all {
		dir, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("get working directory: %w", err)
		}
		req.WorkDir = dir
	}

	stopSpinner := startSpinner(cmd.ErrOrStderr(), "Stopping services...")
	resp, err := client.Send(req)
	stopSpinner()
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

	if len(resp.Services) == 0 {
		if all {
			output.Message(cmd.OutOrStdout(), "No services running.")
		} else {
			output.Message(cmd.OutOrStdout(), "No services running in this worktree.")
		}
		output.Blank(cmd.OutOrStdout())
		return nil
	}
	for i, svc := range resp.Services {
		if i > 0 {
			output.Blank(cmd.OutOrStdout())
		}
		output.Success(cmd.OutOrStdout(), fmt.Sprintf("%s stopped", svc.Name))
	}
	output.Blank(cmd.OutOrStdout())
	return nil
}
