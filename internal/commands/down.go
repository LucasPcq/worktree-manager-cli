package commands

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/LucasPcq/wtm/internal/config"
	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/output"
	"github.com/LucasPcq/wtm/internal/rules"
	"github.com/LucasPcq/wtm/internal/service/process"
)

// newRunDownCmd creates the wtm run down subcommand.
func newRunDownCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   domain.CmdDown + " [profile]",
		Short: "Stop jobs running in the current worktree",
		Long:  "Stop jobs running in the current worktree.\nWith a profile argument, stops only that profile's jobs.\nJobs running in other worktrees are never touched.",
		Args:  cobra.MaximumNArgs(1),
		RunE:  runDown,
	}
	addOutputFlag(cmd)
	cmd.Flags().Bool(domain.FlagAll, false, "Stop jobs across every worktree (bypasses per-worktree scoping)")
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
			return output.WriteJobResultsJSON(cmd.OutOrStdout(), nil)
		}
		output.Blank(cmd.OutOrStdout())
		output.Message(cmd.OutOrStdout(), "No jobs running.")
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

		runCfg, err := config.LoadRun(root)
		if err != nil {
			return fmt.Errorf("load run config: %w", err)
		}

		profile, ok := rules.FindProfile(runCfg, args[0])
		if !ok {
			return fmt.Errorf("profile %q not found in config", args[0])
		}

		jobs := rules.ProfileJobs(runCfg, profile)
		results := make([]output.JobActionResult, 0, len(jobs))
		for _, job := range jobs {
			stopSpinner := startSpinner(cmd.ErrOrStderr(), fmt.Sprintf("Stopping %s...", job.Name))
			resp, sendErr := client.Send(process.Request{
				Action:  process.ActionStop,
				Name:    job.Name,
				WorkDir: dir,
			})
			stopSpinner()
			if sendErr != nil {
				results = append(results, output.JobActionResult{Name: job.Name, Status: domain.JobActionError, Message: sendErr.Error()})
				if format != domain.OutputJSON {
					output.Error(cmd.ErrOrStderr(), fmt.Sprintf("%s: %v", job.Name, sendErr))
				}
				continue
			}
			if resp.Status == process.StatusError {
				results = append(results, output.JobActionResult{Name: job.Name, Status: domain.JobActionError, Message: resp.Message})
				if format != domain.OutputJSON {
					output.Error(cmd.ErrOrStderr(), fmt.Sprintf("%s: %s", job.Name, resp.Message))
				}
				continue
			}
			results = append(results, output.JobActionResult{Name: job.Name, Status: domain.JobActionStopped})
			if format != domain.OutputJSON {
				output.Success(cmd.OutOrStdout(), fmt.Sprintf("%s stopped", job.Name))
			}
		}
		if format == domain.OutputJSON {
			return output.WriteJobResultsJSON(cmd.OutOrStdout(), results)
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

	stopSpinner := startSpinner(cmd.ErrOrStderr(), "Stopping jobs...")
	resp, err := client.Send(req)
	stopSpinner()
	if err != nil {
		return fmt.Errorf("stop all jobs: %w", err)
	}
	if resp.Status == process.StatusError {
		return fmt.Errorf("stop all: %s", resp.Message)
	}

	if format == domain.OutputJSON {
		stopped := make([]output.JobActionResult, 0, len(resp.Jobs))
		for _, job := range resp.Jobs {
			stopped = append(stopped, output.JobActionResult{Name: job.Name, Status: domain.JobActionStopped})
		}
		return output.WriteJobResultsJSON(cmd.OutOrStdout(), stopped)
	}

	if len(resp.Jobs) == 0 {
		if all {
			output.Message(cmd.OutOrStdout(), "No jobs running.")
		} else {
			output.Message(cmd.OutOrStdout(), "No jobs running in this worktree.")
		}
		output.Blank(cmd.OutOrStdout())
		return nil
	}
	for i, job := range resp.Jobs {
		if i > 0 {
			output.Blank(cmd.OutOrStdout())
		}
		output.Success(cmd.OutOrStdout(), fmt.Sprintf("%s stopped", job.Name))
	}
	output.Blank(cmd.OutOrStdout())
	return nil
}
