package run

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/LucasPcq/wtm/internal/commands/shared"
	"github.com/LucasPcq/wtm/internal/config"
	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/flow/runlogs"
	"github.com/LucasPcq/wtm/internal/output"
	"github.com/LucasPcq/wtm/internal/rules"
	"github.com/LucasPcq/wtm/internal/service/process"
	"github.com/LucasPcq/wtm/internal/tui/components"
)

func newStartCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   domain.CmdStart + " <job>",
		Short: "Start a single job",
		Long:  "Start an individual job by name (defined in run.toml).\nA task runs inline and blocks until it exits; a service opens the run view on itself, and -d starts it in the background instead.",
		Args:  cobra.ExactArgs(1),
		RunE:  runStart,
	}
	cmd.Flags().BoolP(domain.FlagDetach, "d", false, "Start the job and return instead of opening the run view")
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

	if err := shared.RequireRunInitialized(runCfg); err != nil {
		return err
	}

	job, ok := rules.FindJob(runCfg, args[0])
	if !ok {
		return fmt.Errorf("%w: %s", domain.ErrJobNotFound, args[0])
	}

	format, _ := cmd.Flags().GetString(domain.FlagOutput)
	detach, _ := cmd.Flags().GetBool(domain.FlagDetach)

	socketPath := process.SocketPath()
	if err := components.RunLoading(components.LoadingParams{
		Message: "Connecting to daemon…",
		Animate: rules.IsHumanFormat(format),
		Work:    func() error { return process.EnsureDaemon(socketPath) },
	}); err != nil {
		return fmt.Errorf("ensure daemon: %w", err)
	}

	surface := surfaceParams{
		Out:      cmd.OutOrStdout(),
		Err:      cmd.ErrOrStderr(),
		Format:   format,
		Service:  runlogs.NewService(runlogs.ServiceParams{SocketPath: socketPath}),
		Declared: []domain.JobConfig{job},
		Start:    []domain.JobConfig{job},
		Focus:    job.Name,
		WorkDir:  dir,
		LogDir:   jobLogDir(jobLogDirParams{StateDir: result.StateDir, Dir: dir}),
	}

	if wantsRunView(wantsRunViewParams{Format: format, Detach: detach, Inline: rules.RunsInline(job)}) {
		outcome, viewErr := startInView(surface)
		if viewErr != nil {
			return viewErr
		}
		if outcome.Aborted() {
			return domain.ErrAborted
		}
		return nil
	}

	return startJobInline(surface)
}

// startJobInline starts one job on the terminal the caller keeps — a task,
// whose output belongs to the scrollback, or a service under -d, a pipe or a
// machine-readable run. A job that fails is this command's whole subject, so it
// fails the command rather than reporting a partial state.
func startJobInline(params surfaceParams) error {
	human := rules.IsHumanFormat(params.Format)
	if human {
		output.FrameStart(params.Out)
	}

	outcome, err := startInline(params)
	if err != nil {
		return err
	}

	if outcome.Aborted() {
		// A reader that saw the output live is not told it twice.
		var captured []byte
		if !human {
			captured = outcome.FailedOutput
		}
		return fmt.Errorf("%s", failureReason(failureParams{Outcome: outcome, Output: captured}))
	}

	if !human {
		return output.WriteJobResultJSON(params.Out, outcome.Results[0])
	}

	output.FrameEnd(params.Out)
	return nil
}
