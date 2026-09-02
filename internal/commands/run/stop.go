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
	"github.com/LucasPcq/wtm/internal/tui/components"
)

// newStopCmd creates the wtm run stop subcommand.
func newStopCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   domain.CmdStop + " [worktree]",
		Short: "Stop a single job",
		Long:  "Stop one running job of [worktree] — the current one when omitted, picked interactively when there is a terminal.\nThe job is named with --job; without it, a fully interactive run offers a picker.",
		Args:  cobra.MaximumNArgs(1),
		RunE:  runStop,
	}
	shared.AddJobFlag(cmd, "Job to stop (required without a terminal or in --output json mode)")
	shared.AddOutputFlag(cmd)
	return cmd
}

func runStop(cmd *cobra.Command, args []string) error {
	dir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get working directory: %w", err)
	}

	format, _ := cmd.Flags().GetString(domain.FlagOutput)
	interactive := isTTY() && rules.IsHumanFormat(format)

	// The job is resolved against run.toml so a typo'd name fails with a precise
	// exit code instead of silently no-opping at the daemon.
	stateDir, err := shared.StateDir(dir)
	if err != nil {
		return err
	}
	runCfg, err := config.LoadRun(stateDir)
	if err != nil {
		return fmt.Errorf("load run config: %w", err)
	}
	if err := shared.RequireRunInitialized(runCfg); err != nil {
		return err
	}

	projectDir, err := shared.ProjectRoot(dir)
	if err != nil {
		return err
	}
	jobName, _ := cmd.Flags().GetString(domain.FlagJob)
	resolved, err := resolveInputs(inputsParams{
		Args:        args,
		Cwd:         dir,
		ProjectDir:  projectDir,
		Interactive: interactive,
		Pick:        true,
		Second:      secondAxis{Given: jobName, Jobs: runCfg.Jobs, Required: true},
	})
	if err != nil {
		return err
	}

	job, err := declaredJob(runCfg, resolved.Second)
	if err != nil {
		return err
	}

	socketPath := process.SocketPath()
	if !process.IsDaemonRunning(socketPath) {
		// An object like every other answer this command gives: the shape follows
		// the arity of the command, never the branch it happened to take
		// (LUC-198). Nothing was running, so the job is stopped either way.
		if format == domain.OutputJSON {
			return output.WriteJobResultJSON(cmd.OutOrStdout(), domain.JobActionResult{
				Name:   job.Name,
				Status: domain.JobActionStopped,
			})
		}
		output.Frame(cmd.OutOrStdout(), func() {
			output.Message(cmd.OutOrStdout(), "No jobs running.")
		})
		return nil
	}

	client := process.NewClient(socketPath)
	var resp process.Response
	err = components.RunLoading(components.LoadingParams{
		Message: fmt.Sprintf("Stopping %s…", job.Name),
		Animate: rules.IsHumanFormat(format),
		Work: func() error {
			var e error
			resp, e = client.Send(process.Request{
				Action:  process.ActionStop,
				Name:    job.Name,
				WorkDir: resolved.Dir,
			})
			return e
		},
	})
	if err != nil {
		return fmt.Errorf("stop %s: %w", job.Name, err)
	}
	if resp.Status == process.StatusError {
		return fmt.Errorf("stop %s: %s", job.Name, resp.Message)
	}

	if format == domain.OutputJSON {
		return output.WriteJobResultJSON(cmd.OutOrStdout(), domain.JobActionResult{
			Name:   job.Name,
			Status: domain.JobActionStopped,
		})
	}

	output.Frame(cmd.OutOrStdout(), func() {
		output.Success(cmd.OutOrStdout(), fmt.Sprintf("%s stopped", job.Name))
	})
	return nil
}
