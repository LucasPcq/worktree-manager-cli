package run

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

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
		Long: "Start every job in a profile, in declared order.\n" +
			"Without arguments, uses the default profile (or shows a picker if multiple exist).\n" +
			"Once the jobs are up, each declared port is checked: a port nothing answers on is\n" +
			"reported rather than announced as bound. It never fails the run — see --no-probe\n" +
			"and run.toml's port_probe_timeout.\n" +
			"Tasks block the profile and abort it on failure; services launch in the background.\n" +
			"The run view opens on the jobs as they start; leaving it detaches without stopping them, and -d skips it.",
		Args: cobra.MaximumNArgs(1),
		RunE: runUp,
	}

	cmd.Flags().Bool(domain.FlagExclusive, false, "Stop jobs on other worktrees before starting")
	cmd.Flags().Bool(domain.FlagParallel, false, "Start without stopping other worktrees")
	cmd.Flags().BoolP(domain.FlagDetach, "d", false, "Start the jobs and return immediately instead of opening their output")
	cmd.Flags().Bool(domain.FlagNoProbe, false, "Skip the check that each declared port was actually bound")
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
	detach, _ := cmd.Flags().GetBool(domain.FlagDetach)

	socketPath := process.SocketPath()
	if err := components.RunLoading(components.LoadingParams{
		Message: domain.RunDaemonConnecting,
		Animate: rules.IsHumanFormat(format),
		Work:    func() error { return process.EnsureDaemon(socketPath) },
	}); err != nil {
		return fmt.Errorf("ensure daemon: %w", err)
	}

	if err := handleConcurrentJobs(cmd, process.NewClient(socketPath), dir); err != nil {
		return err
	}

	noProbe, _ := cmd.Flags().GetBool(domain.FlagNoProbe)
	seam := openRunSeam(runSeamParams{
		ProjectDir: result.ProjectDir,
		StateDir:   result.StateDir,
		Dir:        dir,
		Jobs:       runCfg.Jobs,
		Prober:     newProber(rules.PortProbeBudget(runCfg), noProbe),
	})
	start := seam.starter(jobs)

	switch rules.DecideRunSurface(rules.RunSurfaceParams{Detach: detach, TTY: isTTY(), Format: format}) {
	case domain.RunSurfaceView:
		return showRunView(viewParams{Cmd: cmd, Session: seam.session, Start: start})
	case domain.RunSurfaceMachine:
		return runForMachine(streamParams{Cmd: cmd, Start: start})
	default:
		return runOnStream(streamParams{Cmd: cmd, Start: start})
	}
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
			return rules.JobsWithoutProfile(cfg), nil
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
		// The one thing `default` still does now the picker always opens: it
		// says which entry the run lands on.
		Start: defaultProfile.Name,
	})

	selected, err := components.RunStandaloneSelect(sl)
	if err != nil {
		return domain.ProfileConfig{}, domain.ErrUserAborted
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

// handleConcurrentJobs asks about the jobs another worktree is running before
// this one takes their ports. It is deliberately left outside flow/runlogs: it
// is the only question `run up` asks, its --exclusive/--parallel axis is not
// part of the bypass model yet, and worktree isolation (LUC-99/100) is meant to
// remove the conflict rather than move the prompt.
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
