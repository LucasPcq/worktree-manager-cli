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
		Long:  "Start an individual job by name (defined in run.toml). Tasks run inline and block until they exit; services launch in the background.",
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

	result, err := shared.LoadConfig(cmd, dir)
	if err != nil {
		return err
	}

	runCfg, err := config.LoadRun(result.StateDir)
	if err != nil {
		return fmt.Errorf("load run config: %w", err)
	}

	job, ok := rules.FindJob(runCfg, args[0])
	if !ok {
		return fmt.Errorf("job %q not found in config", args[0])
	}

	format, _ := cmd.Flags().GetString(domain.FlagOutput)

	socketPath := process.SocketPath()
	var stopDaemonSpinner func()
	if format != domain.OutputJSON {
		stopDaemonSpinner = shared.StartSpinner(cmd.ErrOrStderr(), "Connecting to daemon…")
	}
	if err := process.EnsureDaemon(socketPath); err != nil {
		if stopDaemonSpinner != nil {
			stopDaemonSpinner()
		}
		return fmt.Errorf("ensure daemon: %w", err)
	}
	if stopDaemonSpinner != nil {
		stopDaemonSpinner()
	}

	client := process.NewClient(socketPath)

	if job.Kind == domain.JobKindTask {
		// Stream the task's output live (text mode); JSON mode stays silent on
		// stdout so the structured result remains a clean JSON document.
		var onOutput func([]byte)
		if format != domain.OutputJSON {
			out := cmd.OutOrStdout()
			output.Blank(out)
			output.Loading(out, fmt.Sprintf("Running task %s", job.Name))
			onOutput = func(chunk []byte) { _, _ = out.Write(chunk) }
		}
		resp, err := client.SendStream(process.Request{
			Action:  process.ActionStart,
			Job:     &job,
			WorkDir: dir,
		}, onOutput)
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

	stopSpinner := shared.StartSpinner(cmd.ErrOrStderr(), fmt.Sprintf("Starting %s…", job.Name))
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
