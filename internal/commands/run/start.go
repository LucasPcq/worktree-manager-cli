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

// newStartCmd creates the wtm run start subcommand.
func newStartCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   domain.CmdStart + " <job>",
		Short: "Start a single job",
		Long: "Start an individual job by name (defined in run.toml).\n" +
			"A service attaches: its output opens in the run view, and leaving the view detaches without stopping it.\n" +
			"-d starts it and returns the prompt instead.\n" +
			"A task always runs inline and blocks until it exits, with or without -d.",
		Args: cobra.ExactArgs(1),
		RunE: runStart,
	}
	cmd.Flags().BoolP(domain.FlagDetach, "d", false, "Start the service and return immediately instead of opening its output")
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
		return fmt.Errorf("job %q not found in config", args[0])
	}

	format, _ := cmd.Flags().GetString(domain.FlagOutput)
	detach, _ := cmd.Flags().GetBool(domain.FlagDetach)

	socketPath := process.SocketPath()
	if err := components.RunLoading(components.LoadingParams{
		Message: domain.RunDaemonConnecting,
		Animate: rules.IsHumanFormat(format),
		Work:    func() error { return process.EnsureDaemon(socketPath) },
	}); err != nil {
		return fmt.Errorf("ensure daemon: %w", err)
	}

	logDir := jobLogDir(jobLogDirParams{StateDir: result.StateDir, Dir: dir})

	surface := rules.DecideRunSurface(rules.RunSurfaceParams{
		Inline: job.Kind == domain.JobKindTask,
		Detach: detach,
		TTY:    isTTY(),
		Format: format,
	})
	if surface == domain.RunSurfaceView {
		seam := openRunSeam(runSeamParams{StateDir: result.StateDir, Dir: dir, Jobs: runCfg.Jobs})
		return showRunView(viewParams{
			Cmd:     cmd,
			Session: seam.session,
			Job:     job.Name,
			Start:   seam.starter([]domain.JobConfig{job}),
		})
	}

	params := startJobParams{
		Cmd:    cmd,
		Client: process.NewClient(socketPath),
		Job:    job,
		Dir:    dir,
		LogDir: logDir,
		Format: format,
	}
	if job.Kind == domain.JobKindTask {
		return startTaskInline(params)
	}
	return startServiceDetached(params)
}

type startJobParams struct {
	Cmd    *cobra.Command
	Client *process.Client
	Job    domain.JobConfig
	Dir    string
	LogDir string
	Format string
}

// startTaskInline runs a task where the caller can read it: a task is a
// foreground command, so its output belongs to the scrollback rather than to a
// screen that is given back when it ends.
func startTaskInline(params startJobParams) error {
	// JSON stays silent on stdout so the structured result remains a clean
	// document.
	var onOutput func([]byte)
	if params.Format != domain.OutputJSON {
		out := params.Cmd.OutOrStdout()
		output.FrameStart(out)
		output.Loading(out, fmt.Sprintf(domain.RunTaskRunningFmt, params.Job.Name))
		onOutput = func(chunk []byte) { _, _ = out.Write(chunk) }
	}

	resp, err := params.Client.SendStream(process.Request{
		Action:  process.ActionStart,
		Job:     &params.Job,
		WorkDir: params.Dir,
		LogDir:  params.LogDir,
	}, onOutput)
	if err != nil {
		return fmt.Errorf("task %s: %w", params.Job.Name, err)
	}
	if resp.Status == process.StatusError {
		return fmt.Errorf("%s", resp.Message)
	}

	if params.Format == domain.OutputJSON {
		return output.WriteJobResultJSON(params.Cmd.OutOrStdout(), domain.JobActionResult{
			Name:   params.Job.Name,
			Status: domain.JobActionDone,
		})
	}
	output.Success(params.Cmd.OutOrStdout(), fmt.Sprintf(domain.RunStreamDoneFmt, params.Job.Name))
	output.FrameEnd(params.Cmd.OutOrStdout())
	return nil
}

func startServiceDetached(params startJobParams) error {
	var resp process.Response
	if startErr := components.RunLoading(components.LoadingParams{
		Message: fmt.Sprintf(domain.RunStartingFmt, params.Job.Name),
		Animate: rules.IsHumanFormat(params.Format),
		Work: func() error {
			var e error
			resp, e = params.Client.Send(process.Request{
				Action:  process.ActionStart,
				Job:     &params.Job,
				WorkDir: params.Dir,
				LogDir:  params.LogDir,
			})
			return e
		},
	}); startErr != nil {
		return fmt.Errorf("start %s: %w", params.Job.Name, startErr)
	}
	if resp.Status == process.StatusError {
		return fmt.Errorf("%s", resp.Message)
	}

	if params.Format == domain.OutputJSON {
		return output.WriteJobResultJSON(params.Cmd.OutOrStdout(), domain.JobActionResult{
			Name:   params.Job.Name,
			Status: domain.JobActionStarted,
		})
	}

	out := params.Cmd.OutOrStdout()
	output.Frame(out, func() {
		output.Success(out, fmt.Sprintf(domain.RunStreamStartedFmt, params.Job.Name))
	})
	return nil
}
