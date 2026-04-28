package run

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/LucasPcq/wtm/internal/commands/shared"
	"github.com/LucasPcq/wtm/internal/config"
	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/output"
	"github.com/LucasPcq/wtm/internal/rules"
	"github.com/LucasPcq/wtm/internal/service/process"
)

// newStartCmd creates the wtm run start subcommand.
func newStartCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   domain.CmdStart + " <job>",
		Short: "Start a single job",
		Long:  "Start an individual job by name (defined in .wtm/run.toml). Tasks run inline and block until they exit; services launch in the background.",
		Args:  cobra.ExactArgs(1),
		RunE:  runStart,
	}
	shared.AddOutputFlag(cmd)
	return cmd
}

func runStart(cmd *cobra.Command, args []string) error {
	dir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get working directory: %w", err)
	}

	result, ok := shared.LoadConfig(cmd, dir)
	if !ok {
		return nil
	}

	runCfg, err := config.LoadRun(result.ProjectDir)
	if err != nil {
		return fmt.Errorf("load run config: %w", err)
	}

	job, ok := rules.FindJob(runCfg, args[0])
	if !ok {
		return fmt.Errorf("job %q not found in config", args[0])
	}

	socketPath := process.SocketPath()
	if err := process.EnsureDaemon(socketPath); err != nil {
		return fmt.Errorf("ensure daemon: %w", err)
	}

	client := process.NewClient(socketPath)
	format, _ := cmd.Flags().GetString(domain.FlagOutput)

	if job.Kind == domain.JobKindTask {
		var stopSpinner func()
		if format != domain.OutputJSON {
			stopSpinner = shared.StartSpinner(cmd.ErrOrStderr(), fmt.Sprintf("Running task %s...", job.Name))
		}
		resp, err := client.Send(process.Request{
			Action:  process.ActionStart,
			Job:     &job,
			WorkDir: dir,
		})
		if stopSpinner != nil {
			stopSpinner()
		}
		if err != nil {
			return fmt.Errorf("task %s: %w", job.Name, err)
		}
		if resp.Status == process.StatusError {
			return fmt.Errorf("%s", resp.Message)
		}
		if format == domain.OutputJSON {
			return output.WriteJobResultJSON(cmd.OutOrStdout(), output.JobActionResult{
				Name:   job.Name,
				Status: domain.JobActionDone,
			})
		}
		output.Success(cmd.OutOrStdout(), fmt.Sprintf("%s done", job.Name))
		output.Blank(cmd.OutOrStdout())
		return nil
	}

	stopSpinner := shared.StartSpinner(cmd.ErrOrStderr(), fmt.Sprintf("Starting %s...", job.Name))
	resp, err := client.Send(process.Request{
		Action:  process.ActionStart,
		Job:     &job,
		WorkDir: dir,
	})
	stopSpinner()
	if err != nil {
		return fmt.Errorf("start %s: %w", job.Name, err)
	}
	if resp.Status == process.StatusError {
		return fmt.Errorf("%s", resp.Message)
	}

	if format == domain.OutputJSON {
		return output.WriteJobResultJSON(cmd.OutOrStdout(), output.JobActionResult{
			Name:   job.Name,
			Status: domain.JobActionStarted,
		})
	}

	output.Blank(cmd.OutOrStdout())
	output.Success(cmd.OutOrStdout(), fmt.Sprintf("%s started", job.Name))
	output.Blank(cmd.OutOrStdout())
	return nil
}
