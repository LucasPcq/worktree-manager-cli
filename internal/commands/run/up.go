package run

import (
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

	result, ok := shared.LoadConfig(cmd, dir)
	if !ok {
		return nil
	}

	runCfg, err := config.LoadRun(result.StateDir)
	if err != nil {
		return fmt.Errorf("load run config: %w", err)
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

	socketPath := process.SocketPath()
	if err := process.EnsureDaemon(socketPath); err != nil {
		return fmt.Errorf("ensure daemon: %w", err)
	}

	client := process.NewClient(socketPath)

	if err := handleConcurrentJobs(cmd, client, dir); err != nil {
		return err
	}

	format, _ := cmd.Flags().GetString(domain.FlagOutput)
	results := make([]output.JobActionResult, 0, len(jobs))
	var started []domain.JobConfig

	for i := range jobs {
		job := jobs[i]

		if job.Kind == domain.JobKindTask {
			if err := runTaskJob(cmd, client, job, dir, format, &results); err != nil {
				if format == domain.OutputJSON {
					return output.WriteJobResultsJSON(cmd.OutOrStdout(), results)
				}
				// A failing task aborts the remaining jobs. We deliberately
				// leave already-started services running (docker/DB stay up for
				// the fix-and-retry loop) and report the partial state instead.
				reportProfileAbort(cmd, jobs, i, started)
				return domain.ErrAborted
			}
			continue
		}

		stopSpinner := shared.StartSpinner(cmd.ErrOrStderr(), fmt.Sprintf("Starting %s...", job.Name))
		resp, sendErr := client.Send(process.Request{
			Action:  process.ActionStart,
			Job:     &job,
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
				output.Error(cmd.ErrOrStderr(), resp.Message)
			}
			continue
		}
		results = append(results, output.JobActionResult{Name: job.Name, Status: domain.JobActionStarted})
		started = append(started, job)
		if format != domain.OutputJSON {
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

	var running []process.JobInfo
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

func runTaskJob(cmd *cobra.Command, client *process.Client, job domain.JobConfig, dir string, format string, results *[]output.JobActionResult) error {
	// JSON mode stays silent on stdout (the structured results are the payload),
	// so we don't stream raw task output that would corrupt the JSON document.
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
		*results = append(*results, output.JobActionResult{Name: job.Name, Status: domain.JobActionError, Message: err.Error()})
		return fmt.Errorf("task %s: %w", job.Name, err)
	}
	if resp.Status == process.StatusError {
		*results = append(*results, output.JobActionResult{Name: job.Name, Status: domain.JobActionError, Message: resp.Message})
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
