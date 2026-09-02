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
	"github.com/LucasPcq/wtm/internal/flow/run/seam"
	"github.com/LucasPcq/wtm/internal/output"
	"github.com/LucasPcq/wtm/internal/rules"
	"github.com/LucasPcq/wtm/internal/service/process"
	"github.com/LucasPcq/wtm/internal/tui/components"
)

// newUpCmd creates the wtm run up subcommand.
func newUpCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   domain.CmdUp + " [worktree]",
		Short: "Start a profile's jobs",
		Long: "Start every job in a profile, in declared order, in [worktree] — the current one when omitted, picked interactively when there is a terminal.\n" +
			"Without --profile, uses the default profile (or shows a picker if multiple exist).\n" +
			"Once the jobs are up, each declared port is checked: a port nothing answers on is\n" +
			"reported rather than announced as bound. It never fails the run — see --no-probe\n" +
			"and run.toml's port_probe_timeout.\n" +
			"Tasks block the profile and abort it on failure; services launch in the background.\n" +
			"The run view opens on the jobs as they start; leaving it detaches without stopping them, and -d skips it.",
		Args: cobra.MaximumNArgs(1),
		RunE: runUp,
	}

	shared.AddProfileFlag(cmd, "Profile to start (defaults to the default profile, or a picker when several exist)")
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

	format, _ := cmd.Flags().GetString(domain.FlagOutput)
	detach, _ := cmd.Flags().GetBool(domain.FlagDetach)

	profileName, _ := cmd.Flags().GetString(domain.FlagProfile)
	defaultProfile, _ := rules.DefaultProfile(runCfg)

	resolved, err := resolveInputs(inputsParams{
		Args:        args,
		Cwd:         dir,
		ProjectDir:  result.ProjectDir,
		Interactive: isTTY() && rules.IsHumanFormat(format),
		Pick:        true,
		Second: secondAxis{
			Given:    profileName,
			Profiles: askableProfiles(runCfg),
			Start:    defaultProfile.Name,
		},
	})
	if err != nil {
		return err
	}

	profile, err := resolveProfile(resolved.Second, runCfg)
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

	if err := handleConcurrentJobs(cmd, process.NewClient(socketPath), resolved.Dir); err != nil {
		return err
	}

	noProbe, _ := cmd.Flags().GetBool(domain.FlagNoProbe)
	runSeam := seam.Open(seam.Params{
		ProjectDir:  result.ProjectDir,
		StateDir:    result.StateDir,
		WorkDir:     resolved.Dir,
		Jobs:        profile.Jobs,
		ProbeBudget: rules.PortProbeBudget(runCfg),
		NoProbe:     noProbe,
		ProxyPort:   rules.ProxyPort(result.Config.Global),
	})
	start := runSeam.Starter(seam.StartParams{Profile: profile.Name, Jobs: profile.Jobs})

	switch rules.DecideRunSurface(rules.RunSurfaceParams{Detach: detach, TTY: isTTY(), Format: format}) {
	case domain.RunSurfaceView:
		return showRunView(viewParams{Cmd: cmd, Board: runSeam.Board(), Profile: profile.Name, Start: start})
	case domain.RunSurfaceMachine:
		return runForMachine(streamParams{Cmd: cmd, Start: start})
	default:
		return runOnStream(streamParams{
			Cmd:        cmd,
			Profile:    profile.Name,
			Start:      start,
			Hyperlinks: rules.IsHumanFormat(format) && isTTY(),
		})
	}
}

// resolveProfile answers both halves of "what is this run": the jobs to start
// and the name to put on them. Dropping the name left `run up` unable to say
// which of several profiles it had just brought up (LUC-208).
func resolveProfile(name string, cfg domain.RunConfig) (resolvedProfile, error) {
	if name != "" {
		profile, ok := rules.FindProfile(cfg, name)
		if !ok {
			return resolvedProfile{}, fmt.Errorf("profile %q not found in config", name)
		}
		return profileRun(cfg, profile), nil
	}

	profile, ok := rules.DefaultProfile(cfg)
	if !ok {
		return resolvedProfile{Jobs: rules.JobsWithoutProfile(cfg)}, nil
	}
	return profileRun(cfg, profile), nil
}

// askableProfiles is the profile question, or nothing when there is no question:
// a config with one profile — or none — has a safe default and is never asked.
func askableProfiles(cfg domain.RunConfig) []domain.ProfileConfig {
	if len(cfg.Profiles) <= 1 {
		return nil
	}
	return cfg.Profiles
}

// resolvedProfile is what `run up` settled on: a name for the run and the jobs
// it starts. The name is empty for a config declaring no profile at all.
type resolvedProfile struct {
	Name string
	Jobs []domain.JobConfig
}

func profileRun(cfg domain.RunConfig, profile domain.ProfileConfig) resolvedProfile {
	return resolvedProfile{Name: profile.Name, Jobs: rules.ProfileJobs(cfg, profile)}
}

// handleConcurrentJobs asks about the jobs another worktree is running before
// the target takes their ports. "Another" is measured against the target, never
// against the current directory: `run up X` must not offer to stop X's own jobs.
// It is deliberately left outside flow/runlogs: it is the only question `run up`
// asks, its --exclusive/--parallel axis is not part of the bypass model yet, and
// worktree isolation (LUC-99/100) is meant to remove the conflict rather than
// move the prompt.
func handleConcurrentJobs(cmd *cobra.Command, client *process.Client, targetDir string) error {
	exclusiveFlag, _ := cmd.Flags().GetBool(domain.FlagExclusive)
	parallelFlag, _ := cmd.Flags().GetBool(domain.FlagParallel)

	if parallelFlag {
		return nil
	}

	otherWorktrees, otherNames := findOtherRunningJobs(client, targetDir)
	if len(otherWorktrees) == 0 {
		return nil
	}

	if exclusiveFlag {
		return stopOtherJobs(client, otherWorktrees, cmd)
	}

	return promptConcurrentJobs(cmd, client, otherWorktrees, otherNames)
}

func findOtherRunningJobs(client *process.Client, targetDir string) (map[string]bool, map[string][]string) {
	resp, err := client.Send(process.Request{Action: process.ActionList})
	if err != nil {
		return nil, nil
	}

	otherWorktrees := make(map[string]bool)
	otherNames := make(map[string][]string)

	for _, job := range resp.Jobs {
		if !rules.IsJobUp(job.Status) {
			continue
		}
		if job.WorkDir == targetDir {
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
