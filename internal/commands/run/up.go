package run

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/LucasPcq/wtm/internal/commands/shared"
	"github.com/LucasPcq/wtm/internal/config"
	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/output"
	"github.com/LucasPcq/wtm/internal/rules"
	"github.com/LucasPcq/wtm/internal/service/process"
	"github.com/LucasPcq/wtm/internal/tui/components"
)

// newUpCmd creates the wtm run up subcommand.
func newUpCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   domain.CmdUp + " [profile]",
		Short: "Start a profile's jobs",
		Long:  "Start every job in a profile, in declared order.\nWithout arguments, uses the default profile (or shows a picker if multiple exist).\nTasks block the profile and abort it on failure; services launch detached.",
		Args:  cobra.MaximumNArgs(1),
		RunE:  runUp,
	}

	cmd.Flags().Bool(domain.FlagExclusive, false, "Stop jobs on other worktrees before starting")
	cmd.Flags().Bool(domain.FlagParallel, false, "Start without stopping other worktrees")
	cmd.Flags().BoolP(domain.FlagDetach, "d", false, "Start jobs and return immediately instead of tailing their logs")
	shared.AddOutputFlag(cmd)

	return cmd
}

func runUp(cmd *cobra.Command, args []string) error {
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

	warnings, errs := rules.ValidateRun(runCfg)
	for _, warning := range warnings {
		output.Warning(cmd.ErrOrStderr(), warning)
	}
	if len(errs) > 0 {
		for _, e := range errs {
			output.Error(cmd.ErrOrStderr(), e)
		}
		return fmt.Errorf("invalid run config")
	}

	jobs, err := resolveProfileJobs(args, runCfg)
	if err != nil {
		return err
	}

	format, _ := cmd.Flags().GetString(domain.FlagOutput)

	socketPath := process.SocketPath()
	if err := components.RunLoading(components.LoadingParams{
		Message: "Connecting to daemon…",
		Animate: rules.IsHumanFormat(format),
		Work:    func() error { return process.EnsureDaemon(socketPath) },
	}); err != nil {
		return fmt.Errorf("ensure daemon: %w", err)
	}

	client := process.NewClient(socketPath)
	logDir := rules.WorktreeLogDir(rules.WorktreeLogDirParams{StateDir: result.StateDir, WorkDir: dir})

	if err := handleConcurrentJobs(cmd, client, dir); err != nil {
		return err
	}
	results := make([]output.JobActionResult, 0, len(jobs))
	var started []domain.JobConfig

	for i := range jobs {
		job := jobs[i]

		// Tasks and services abort the profile the same way on failure: stop
		// launching the rest, leave what's running, and report the partial
		// state (or emit the JSON results in machine mode).
		if job.Kind == domain.JobKindTask {
			if err := runTaskJob(jobRunParams{
				Cmd: cmd, Client: client, Job: job, WorkDir: dir, LogDir: logDir,
				Format: format, Results: &results,
			}); err != nil {
				return abortProfile(cmd, jobs, i, started, results, format)
			}
			continue
		}

		// Detached launchers (docker compose up -d) stream their startup output
		// live like a task, then stay running in the background.
		if rules.IsDetached(job) {
			if err := runDetachedJob(jobRunParams{
				Cmd: cmd, Client: client, Job: job, WorkDir: dir, LogDir: logDir,
				Format: format, Results: &results, Started: &started,
			}); err != nil {
				return abortProfile(cmd, jobs, i, started, results, format)
			}
			continue
		}

		var resp process.Response
		sendErr := components.RunLoading(components.LoadingParams{
			Message: fmt.Sprintf("Starting %s…", job.Name),
			Animate: rules.IsHumanFormat(format),
			Work: func() error {
				var e error
				resp, e = client.Send(process.Request{
					Action:  process.ActionStart,
					Job:     &job,
					WorkDir: dir,
					LogDir:  logDir,
				})
				return e
			},
		})
		if sendErr != nil {
			results = append(results, output.JobActionResult{Name: job.Name, Status: domain.JobActionError, Message: sendErr.Error()})
			if format != domain.OutputJSON {
				output.Error(cmd.ErrOrStderr(), fmt.Sprintf("%s: %v", job.Name, sendErr))
			}
			return abortProfile(cmd, jobs, i, started, results, format)
		}
		if resp.Status == process.StatusError {
			// A repeat start (re-running `run up` while services are up) is
			// benign: count the job as running and keep going.
			if strings.Contains(resp.Message, domain.JobAlreadyRunningSuffix) {
				results = append(results, output.JobActionResult{Name: job.Name, Status: domain.JobActionStarted})
				started = append(started, job)
				if format != domain.OutputJSON {
					output.Blank(cmd.OutOrStdout())
					output.Success(cmd.OutOrStdout(), fmt.Sprintf("%s already running", job.Name))
				}
				continue
			}
			results = append(results, output.JobActionResult{Name: job.Name, Status: domain.JobActionError, Message: resp.Message})
			if format != domain.OutputJSON {
				output.Error(cmd.ErrOrStderr(), resp.Message)
			}
			return abortProfile(cmd, jobs, i, started, results, format)
		}
		results = append(results, output.JobActionResult{Name: job.Name, Status: domain.JobActionStarted})
		started = append(started, job)
		if format != domain.OutputJSON {
			output.Blank(cmd.OutOrStdout())
			output.Success(cmd.OutOrStdout(), fmt.Sprintf("%s started", job.Name))
		}
	}

	if format == domain.OutputJSON {
		return output.WriteJobResultsJSON(cmd.OutOrStdout(), results)
	}

	detach, _ := cmd.Flags().GetBool(domain.FlagDetach)
	if detach || !term.IsTerminal(int(os.Stdin.Fd())) {
		output.Blank(cmd.OutOrStdout())
		return nil
	}

	return watchProfileServices(cmd, jobs, dir)
}

// watchProfileServices tails the foreground services that a profile just
// started, unifying `run up` + `run logs` into a single command. A lone
// service is attached directly (full PTY, so its own TUI — turbo, vite —
// renders natively and stays interactive); two or more are multiplexed as
// color-prefixed log lines. Detached launchers (docker compose up -d) have no
// attachable output and are skipped. Ctrl+C detaches without stopping anything.
func watchProfileServices(cmd *cobra.Command, jobs []domain.JobConfig, dir string) error {
	watchable := make(map[string]bool)
	for i := range jobs {
		job := jobs[i]
		if job.Kind == domain.JobKindService && !rules.IsDetached(job) {
			watchable[job.Name] = true
		}
	}
	if len(watchable) == 0 {
		output.Blank(cmd.OutOrStdout())
		return nil
	}

	socketPath := process.SocketPath()
	client := process.NewClient(socketPath)
	resp, err := client.Send(process.Request{Action: process.ActionList})
	if err != nil {
		output.Blank(cmd.OutOrStdout())
		return nil
	}

	var running []domain.JobInfo
	for _, job := range resp.Jobs {
		if job.WorkDir == dir && job.Status == domain.JobStatusRunning && watchable[job.Name] {
			running = append(running, job)
		}
	}
	if len(running) == 0 {
		output.Blank(cmd.OutOrStdout())
		return nil
	}

	output.Blank(cmd.OutOrStdout())
	output.Loading(cmd.OutOrStdout(), "Tailing logs — Ctrl+C to detach (services keep running)")
	output.Blank(cmd.OutOrStdout())

	var watchErr error
	if len(running) == 1 {
		watchErr = attachSingleJob(socketPath, running[0].Name, dir)
	} else {
		watchErr = multiplexJobs(socketPath, dir, running)
	}

	output.Blank(cmd.ErrOrStderr())
	output.Message(cmd.ErrOrStderr(), "Detached. Services keep running in the background.")
	output.Loading(cmd.ErrOrStderr(), "wtm run logs to reattach · wtm run down to stop")
	return watchErr
}

// jobRunParams carries what starting one job of a profile needs. Started is
// only read by the detached launcher, the sole kind that stays registered.
type jobRunParams struct {
	Cmd     *cobra.Command
	Client  *process.Client
	Job     domain.JobConfig
	WorkDir string
	LogDir  string
	Format  string
	Results *[]output.JobActionResult
	Started *[]domain.JobConfig
}

func runTaskJob(params jobRunParams) error {
	cmd, client, job := params.Cmd, params.Client, params.Job
	format, results := params.Format, params.Results
	// We always capture the streamed output so failures carry the "why" — live
	// on stdout in text mode (the user reads it as it runs), and into the JSON
	// result's message in machine mode (LLM/CI never sees the live stream).
	var captured bytes.Buffer
	onOutput := func(chunk []byte) { _, _ = captured.Write(chunk) }
	if format != domain.OutputJSON {
		out := cmd.OutOrStdout()
		output.Blank(out)
		output.Loading(out, fmt.Sprintf("Running task %s", job.Name))
		onOutput = func(chunk []byte) {
			_, _ = captured.Write(chunk)
			_, _ = out.Write(chunk)
		}
	}

	resp, err := client.SendStream(process.Request{
		Action:  process.ActionStart,
		Job:     &job,
		WorkDir: params.WorkDir,
		LogDir:  params.LogDir,
	}, onOutput)
	if err != nil {
		*results = append(*results, output.JobActionResult{Name: job.Name, Status: domain.JobActionError, Message: err.Error()})
		return fmt.Errorf("task %s: %w", job.Name, err)
	}
	if resp.Status == process.StatusError {
		// In text mode the output already streamed live, so the result only
		// needs the concise reason; JSON consumers get the captured logs too.
		message := resp.Message
		if logs := strings.TrimSpace(captured.String()); logs != "" && format == domain.OutputJSON {
			message = resp.Message + "\n" + logs
		}
		*results = append(*results, output.JobActionResult{Name: job.Name, Status: domain.JobActionError, Message: message})
		if format != domain.OutputJSON {
			output.Error(cmd.ErrOrStderr(), resp.Message)
		}
		return fmt.Errorf("task %s failed", job.Name)
	}

	*results = append(*results, output.JobActionResult{Name: job.Name, Status: domain.JobActionDone})
	if format != domain.OutputJSON {
		output.Success(cmd.OutOrStdout(), fmt.Sprintf("%s done", job.Name))
	}
	return nil
}

// runDetachedJob starts a detached launcher (e.g. docker compose up -d) and
// streams its output live — the launcher runs, prints its lines, and exits,
// freeing the terminal — exactly like a task, except the job stays registered
// as running afterwards. Returns a non-nil error only when the profile should
// abort; an already-running launcher is a benign no-op.
func runDetachedJob(params jobRunParams) error {
	cmd, client, job := params.Cmd, params.Client, params.Job
	format, results, started := params.Format, params.Results, params.Started
	// Always capture the streamed output so a failure carries the "why" — live
	// on stdout in text mode, and into the JSON result's message in machine mode.
	var captured bytes.Buffer
	onOutput := func(chunk []byte) { _, _ = captured.Write(chunk) }
	if format != domain.OutputJSON {
		out := cmd.OutOrStdout()
		output.Blank(out)
		output.Loading(out, fmt.Sprintf("Starting %s", job.Name))
		onOutput = func(chunk []byte) {
			_, _ = captured.Write(chunk)
			_, _ = out.Write(chunk)
		}
	}

	resp, err := client.SendStream(process.Request{
		Action:  process.ActionStart,
		Job:     &job,
		WorkDir: params.WorkDir,
		LogDir:  params.LogDir,
	}, onOutput)
	if err != nil {
		*results = append(*results, output.JobActionResult{Name: job.Name, Status: domain.JobActionError, Message: err.Error()})
		if format != domain.OutputJSON {
			output.Error(cmd.ErrOrStderr(), fmt.Sprintf("%s: %v", job.Name, err))
		}
		return fmt.Errorf("service %s: %w", job.Name, err)
	}
	if resp.Status == process.StatusError {
		// Re-running `run up` while the launcher's work is already up is benign.
		if strings.Contains(resp.Message, domain.JobAlreadyRunningSuffix) {
			*results = append(*results, output.JobActionResult{Name: job.Name, Status: domain.JobActionStarted})
			*started = append(*started, job)
			if format != domain.OutputJSON {
				output.Success(cmd.OutOrStdout(), fmt.Sprintf("%s already running", job.Name))
			}
			return nil
		}
		// In text mode the output already streamed live, so the result only
		// needs the concise reason; JSON consumers get the captured logs too.
		message := resp.Message
		if logs := strings.TrimSpace(captured.String()); logs != "" && format == domain.OutputJSON {
			message = resp.Message + "\n" + logs
		}
		*results = append(*results, output.JobActionResult{Name: job.Name, Status: domain.JobActionError, Message: message})
		if format != domain.OutputJSON {
			output.Error(cmd.ErrOrStderr(), resp.Message)
		}
		return fmt.Errorf("service %s failed", job.Name)
	}

	*results = append(*results, output.JobActionResult{Name: job.Name, Status: domain.JobActionStarted})
	*started = append(*started, job)
	if format != domain.OutputJSON {
		output.Success(cmd.OutOrStdout(), fmt.Sprintf("%s started", job.Name))
	}
	return nil
}

// abortProfile ends a profile run early after a job failed. In JSON mode it
// emits the accumulated results (the error entry is already in there) and exits
// zero so the document stays parseable; in text mode it prints the partial-state
// report and returns ErrAborted so the process exits non-zero without a second
// error line on top of the report.
func abortProfile(cmd *cobra.Command, jobs []domain.JobConfig, failedIdx int, started []domain.JobConfig, results []output.JobActionResult, format string) error {
	if format == domain.OutputJSON {
		return output.WriteJobResultsJSON(cmd.OutOrStdout(), results)
	}
	reportProfileAbort(cmd, jobs, failedIdx, started)
	return domain.ErrAborted
}

// reportProfileAbort prints the partial state after a task aborts a profile:
// the step that failed, the services left running (never torn down, so the
// fix-and-retry loop keeps docker/DB warm), the jobs that never started, and
// the two next actions. Keeps the user out of a silent intermediate state.
func reportProfileAbort(cmd *cobra.Command, jobs []domain.JobConfig, failedIdx int, started []domain.JobConfig) {
	w := cmd.ErrOrStderr()
	output.Blank(w)
	output.Warning(w, fmt.Sprintf("Profile aborted at step %d/%d (%s).", failedIdx+1, len(jobs), jobs[failedIdx].Name))

	if len(started) > 0 {
		names := make([]string, len(started))
		for i, j := range started {
			names[i] = j.Name
		}
		output.InfoLine(w, "Left running:", strings.Join(names, ", "))
	}

	if failedIdx+1 < len(jobs) {
		notStarted := make([]string, 0, len(jobs)-failedIdx-1)
		for _, j := range jobs[failedIdx+1:] {
			notStarted = append(notStarted, j.Name)
		}
		output.InfoLine(w, "Not started: ", strings.Join(notStarted, ", "))
	}

	output.Blank(w)
	output.Loading(w, "fix and re-run `wtm run up` · `wtm run down` to stop everything")
	output.Blank(w)
}

func resolveProfileJobs(args []string, cfg domain.RunConfig) ([]domain.JobConfig, error) {
	if len(args) > 0 {
		profile, ok := rules.FindProfile(cfg, args[0])
		if !ok {
			return nil, fmt.Errorf("profile %q not found in config", args[0])
		}
		return rules.ProfileJobs(cfg, profile), nil
	}

	if len(cfg.Profiles) <= 1 {
		profile, ok := rules.DefaultProfile(cfg)
		if !ok {
			return cfg.Jobs, nil
		}
		return rules.ProfileJobs(cfg, profile), nil
	}

	profile, err := pickProfile(cfg)
	if err != nil {
		return nil, err
	}

	return rules.ProfileJobs(cfg, profile), nil
}

func pickProfile(cfg domain.RunConfig) (domain.ProfileConfig, error) {
	defaultProfile, _ := rules.DefaultProfile(cfg)

	items := make([]components.SelectItem, 0, len(cfg.Profiles))
	for _, p := range cfg.Profiles {
		label := p.Name
		if len(p.Jobs) > 0 {
			label += fmt.Sprintf(" (%s)", joinJobNames(p.Jobs))
		}
		items = append(items, components.SelectItem{Label: label, Value: p.Name})
	}

	sl := components.NewSelectList(components.NewSelectListParams{
		Title:       "Select profile",
		Description: "Which profile to start?",
		Items:       items,
	})

	selected, err := components.RunStandaloneSelect(sl)
	if err != nil {
		return domain.ProfileConfig{}, domain.ErrUserAborted
	}
	if selected == "" {
		selected = defaultProfile.Name
	}

	profile, ok := rules.FindProfile(cfg, selected)
	if !ok {
		return domain.ProfileConfig{}, fmt.Errorf("profile %q not found", selected)
	}

	return profile, nil
}

func joinJobNames(names []string) string {
	result := ""
	for i, n := range names {
		if i > 0 {
			result += ", "
		}
		result += n
	}
	return result
}

func handleConcurrentJobs(cmd *cobra.Command, client *process.Client, currentDir string) error {
	exclusiveFlag, _ := cmd.Flags().GetBool(domain.FlagExclusive)
	parallelFlag, _ := cmd.Flags().GetBool(domain.FlagParallel)

	if parallelFlag {
		return nil
	}

	otherWorktrees, otherNames := findOtherRunningJobs(client, currentDir)
	if len(otherWorktrees) == 0 {
		return nil
	}

	if exclusiveFlag {
		return stopOtherJobs(client, otherWorktrees, cmd)
	}

	return promptConcurrentJobs(cmd, client, otherWorktrees, otherNames)
}

func findOtherRunningJobs(client *process.Client, currentDir string) (map[string]bool, map[string][]string) {
	resp, err := client.Send(process.Request{Action: process.ActionList})
	if err != nil {
		return nil, nil
	}

	otherWorktrees := make(map[string]bool)
	otherNames := make(map[string][]string)

	for _, job := range resp.Jobs {
		if job.Status != domain.JobStatusRunning {
			continue
		}
		if job.WorkDir == currentDir {
			continue
		}
		otherWorktrees[job.WorkDir] = true
		otherNames[job.WorkDir] = append(otherNames[job.WorkDir], job.Name)
	}

	return otherWorktrees, otherNames
}

func promptConcurrentJobs(cmd *cobra.Command, client *process.Client, otherWorktrees map[string]bool, otherNames map[string][]string) error {
	output.Blank(cmd.ErrOrStderr())
	for dir, names := range otherNames {
		short := filepath.Base(dir)
		output.Warning(cmd.ErrOrStderr(), fmt.Sprintf("Jobs running on %s (%s)", short, strings.Join(names, ", ")))
	}

	items := []components.SelectItem{
		{Label: "Yes, stop and start here", Value: "yes"},
		{Label: "No, run in parallel", Value: "no"},
	}

	sl := components.NewSelectList(components.NewSelectListParams{
		Title: "Stop other jobs before starting?",
		Items: items,
	})

	choice, err := components.RunStandaloneSelect(sl)
	if err != nil {
		return domain.ErrUserAborted
	}

	if choice == "yes" {
		output.Blank(cmd.ErrOrStderr())
		return stopOtherJobs(client, otherWorktrees, cmd)
	}
	return nil
}

func stopOtherJobs(client *process.Client, worktrees map[string]bool, cmd *cobra.Command) error {
	for dir := range worktrees {
		resp, err := client.Send(process.Request{
			Action:  process.ActionStopAll,
			WorkDir: dir,
		})
		if err != nil {
			output.Error(cmd.ErrOrStderr(), fmt.Sprintf("stop jobs in %s: %v", filepath.Base(dir), err))
			continue
		}
		if resp.Status == process.StatusError {
			output.Error(cmd.ErrOrStderr(), fmt.Sprintf("stop jobs in %s: %s", filepath.Base(dir), resp.Message))
			continue
		}
		output.Success(cmd.ErrOrStderr(), fmt.Sprintf("Stopped jobs in %s", filepath.Base(dir)))
	}
	return nil
}
