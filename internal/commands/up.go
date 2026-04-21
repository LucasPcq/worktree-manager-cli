package commands

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/LucasPcq/wtm/internal/config"
	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/output"
	"github.com/LucasPcq/wtm/internal/rules"
	"github.com/LucasPcq/wtm/internal/service/process"
	"github.com/LucasPcq/wtm/internal/tui/components"
)

// newRunUpCmd creates the wtm run up subcommand.
func newRunUpCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   domain.CmdUp + " [profile]",
		Short: "Start a profile's jobs",
		Long:  "Start every job in a profile, in declared order.\nWithout arguments, uses the default profile (or shows a picker if multiple exist).\nTasks block the profile and abort it on failure; services launch detached.",
		Args:  cobra.MaximumNArgs(1),
		RunE:  runUp,
	}

	cmd.Flags().Bool(domain.FlagExclusive, false, "Stop jobs on other worktrees before starting")
	cmd.Flags().Bool(domain.FlagParallel, false, "Start without stopping other worktrees")
	addOutputFlag(cmd)

	return cmd
}

func runUp(cmd *cobra.Command, args []string) error {
	dir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get working directory: %w", err)
	}

	result, ok := loadConfig(cmd, dir)
	if !ok {
		return nil
	}

	runCfg, err := config.LoadRun(result.ProjectDir)
	if err != nil {
		return fmt.Errorf("load run config: %w", err)
	}

	warnings, errs := config.ValidateRun(runCfg)
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

	for i := range jobs {
		job := jobs[i]

		if job.Kind == domain.JobKindTask {
			if err := runTaskJob(cmd, client, job, dir, format, &results); err != nil {
				if format == domain.OutputJSON {
					return output.WriteJobResultsJSON(cmd.OutOrStdout(), results)
				}
				output.Blank(cmd.OutOrStdout())
				return err
			}
			continue
		}

		stopSpinner := startSpinner(cmd.ErrOrStderr(), fmt.Sprintf("Starting %s...", job.Name))
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
		if format != domain.OutputJSON {
			output.Success(cmd.OutOrStdout(), fmt.Sprintf("%s started", job.Name))
		}
	}

	if format == domain.OutputJSON {
		return output.WriteJobResultsJSON(cmd.OutOrStdout(), results)
	}

	output.Blank(cmd.OutOrStdout())
	return nil
}

// runTaskJob blocks while the daemon runs a task and surfaces the outcome to
// the user with a spinner. Output is not streamed here — the task's hub is
// live on the daemon so the user can `wtm run logs <task>` from another
// terminal to follow along. Returns an error when the task fails so the
// caller can abort the profile.
func runTaskJob(cmd *cobra.Command, client *process.Client, job domain.JobConfig, dir string, format string, results *[]output.JobActionResult) error {
	var stopSpinner func()
	if format != domain.OutputJSON {
		stopSpinner = startSpinner(cmd.ErrOrStderr(), fmt.Sprintf("Running task %s...", job.Name))
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

func resolveProfileJobs(args []string, cfg domain.RunConfig) ([]domain.JobConfig, error) {
	if len(args) > 0 {
		profile, ok := rules.FindProfile(cfg, args[0])
		if !ok {
			return nil, fmt.Errorf("profile %q not found in config", args[0])
		}
		return rules.ProfileJobs(cfg, profile), nil
	}

	// 1 profile or less → use default
	if len(cfg.Profiles) <= 1 {
		profile, ok := rules.DefaultProfile(cfg)
		if !ok {
			return cfg.Jobs, nil
		}
		return rules.ProfileJobs(cfg, profile), nil
	}

	// 2+ profiles → interactive picker
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

// handleConcurrentJobs checks if jobs are running on other worktrees and
// handles the exclusive/parallel decision.
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
