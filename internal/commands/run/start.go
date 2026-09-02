package run

import (
	"fmt"
	"os"
	"path/filepath"

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
		Use:   domain.CmdStart + " [worktree]",
		Short: "Start a single job",
		Long: "Start one job of [worktree] — the current one when omitted, picked interactively when there is a terminal.\n" +
			"The job is named with --job; without it, a fully interactive run offers a picker.\n" +
			"A service attaches: its output opens in the run view, and leaving the view detaches without stopping it.\n" +
			"-d starts it and returns the prompt instead.\n" +
			"A task always runs inline and blocks until it exits, with or without -d.",
		Args: cobra.MaximumNArgs(1),
		RunE: runStart,
	}
	shared.AddJobFlag(cmd, "Job to start (required without a terminal or in --output json mode)")
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

	format, _ := cmd.Flags().GetString(domain.FlagOutput)
	detach, _ := cmd.Flags().GetBool(domain.FlagDetach)
	interactive := isTTY() && rules.IsHumanFormat(format)

	jobName, _ := cmd.Flags().GetString(domain.FlagJob)
	resolved, err := resolveInputs(inputsParams{
		Args:        args,
		Cwd:         dir,
		ProjectDir:  result.ProjectDir,
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
	if err := components.RunLoading(components.LoadingParams{
		Message: domain.RunDaemonConnecting,
		Animate: rules.IsHumanFormat(format),
		Work: func() error {
			return process.EnsureDaemon(process.DaemonParams{SocketPath: socketPath, ProxyPort: rules.ProxyPort(result.Config.Global)})
		},
	}); err != nil {
		return fmt.Errorf("ensure daemon: %w", err)
	}

	logDir := jobLogDir(jobLogDirParams{StateDir: result.StateDir, Dir: resolved.Dir})
	env := jobEnv(jobEnvParams{ProjectDir: result.ProjectDir, StateDir: result.StateDir, Dir: resolved.Dir})

	surface := rules.DecideRunSurface(rules.RunSurfaceParams{
		Inline: job.Kind == domain.JobKindTask,
		Detach: detach,
		TTY:    isTTY(),
		Format: format,
	})
	if surface == domain.RunSurfaceView {
		seam := openRunSeam(runSeamParams{ProjectDir: result.ProjectDir, StateDir: result.StateDir, Dir: resolved.Dir, Jobs: runCfg.Jobs, ProxyPort: rules.ProxyPort(result.Config.Global)})
		return showRunView(viewParams{
			Cmd:   cmd,
			Board: seam.board,
			Job:   job.Name,
			Start: seam.starter(resolvedProfile{Jobs: []domain.JobConfig{job}}),
		})
	}

	params := startJobParams{
		Cmd:    cmd,
		Client: process.NewClient(socketPath),
		Job:    job,
		Dir:    resolved.Dir,
		LogDir: logDir,
		Env:    env,
		Format: format,
		RouteHost: rules.RouteHost(rules.RouteHostParams{
			Job:      job,
			Worktree: env[domain.EnvWorktree],
			Project:  filepath.Base(result.ProjectDir),
		}),
		ProxyPort: rules.ProxyPort(result.Config.Global),
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
	Env    map[string]string
	Format string
	// RouteHost is the name the proxy is to serve this job under, and ProxyPort
	// where it serves it. This path talks to the daemon directly, so it carries
	// what internal/flow/runlogs would otherwise have resolved.
	RouteHost string
	ProxyPort int
}

type startedLineParams struct {
	Label string
	Ports map[string]int
	// ServedPort is the public port the daemon answered, not what this command
	// asked for: a name nothing serves is worse than a port.
	ServedPort int
}

// startedLine is what a human surface prints once the daemon has answered: the
// same composition `run up` uses, so one job reads the same either way.
func (p startJobParams) startedLine(params startedLineParams) string {
	return output.JobLine(output.JobLineParams{
		Label: params.Label,
		Ports: params.Ports,
		URL: rules.JobURL(rules.JobURLParams{
			Job:        p.Job,
			Ports:      params.Ports,
			Host:       p.RouteHost,
			PublicPort: params.ServedPort,
		}),
		Hyperlinks: rules.IsHumanFormat(p.Format) && isTTY(),
	})
}

// proxyNotice is what a forked daemon could not tell anyone about: it either
// fell back to another port, or found the whole window taken.
func (p startJobParams) proxyNotice(servedPort int) {
	if p.ProxyPort == 0 || p.RouteHost == "" || servedPort == p.ProxyPort {
		return
	}
	if servedPort == 0 {
		output.Callout(p.Cmd.ErrOrStderr(), domain.ProxyUnavailableTitle, []string{
			fmt.Sprintf(domain.ProxyUnavailableFmt, p.ProxyPort),
		})
		return
	}
	output.Callout(p.Cmd.ErrOrStderr(), domain.ProxyMovedTitle, []string{
		fmt.Sprintf(domain.ProxyMovedFmt, p.ProxyPort, servedPort),
	})
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
		Action:    process.ActionStart,
		Job:       &params.Job,
		WorkDir:   params.Dir,
		LogDir:    params.LogDir,
		Env:       params.Env,
		RouteHost: params.RouteHost,
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
	output.Success(params.Cmd.OutOrStdout(), params.startedLine(startedLineParams{
		Label:      fmt.Sprintf(domain.RunStreamDoneFmt, params.Job.Name),
		Ports:      resp.Ports,
		ServedPort: resp.ProxyPublicPort,
	}))
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
				Action:    process.ActionStart,
				Job:       &params.Job,
				WorkDir:   params.Dir,
				LogDir:    params.LogDir,
				Env:       params.Env,
				RouteHost: params.RouteHost,
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
		output.Success(out, params.startedLine(startedLineParams{
			Label:      fmt.Sprintf(domain.RunStreamStartedFmt, params.Job.Name),
			Ports:      resp.Ports,
			ServedPort: resp.ProxyPublicPort,
		}))
	})
	params.proxyNotice(resp.ProxyPort)
	return nil
}
