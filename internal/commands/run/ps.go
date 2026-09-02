package run

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"time"

	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/LucasPcq/wtm/internal/commands/shared"
	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/output"
	"github.com/LucasPcq/wtm/internal/tui/components"
	runpicker "github.com/LucasPcq/wtm/internal/tui/runpicker"
)

// newPsCmd creates the wtm run ps subcommand.
func newPsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   domain.CmdPs,
		Short: "List currently running jobs",
		Long:  "Show the jobs managed by the background daemon (name, kind, status, PID, uptime, worktree).\nIn a TTY, offers an interactive picker with stop/logs/restart actions.",
		RunE:  runPs,
	}
	shared.AddOutputFlag(cmd)
	return cmd
}

func runPs(cmd *cobra.Command, _ []string) error {
	dir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get working directory: %w", err)
	}
	if err := shared.GuardRunInitialized(dir); err != nil {
		return err
	}

	format, _ := cmd.Flags().GetString(domain.FlagOutput)

	if format == domain.OutputJSON {
		jobs, loadErr := shared.LoadJobs()
		if loadErr != nil {
			return loadErr
		}
		return output.WriteRunningJobsJSON(cmd.OutOrStdout(), jobs)
	}

	var jobs []domain.JobInfo
	loadErr := components.RunLoading(components.LoadingParams{
		Message: "Loading jobs…",
		Animate: true,
		Work: func() error {
			var e error
			jobs, e = shared.LoadJobs()
			return e
		},
	})
	if loadErr != nil {
		return loadErr
	}

	if !term.IsTerminal(int(os.Stdin.Fd())) {
		output.Frame(cmd.OutOrStdout(), func() {
			fmt.Fprint(cmd.OutOrStdout(), output.FormatRunningJobs(output.FormatRunningJobsParams{Jobs: jobs, Now: time.Now()}))
		})
		return nil
	}

	if len(jobs) == 0 {
		output.Frame(cmd.OutOrStdout(), func() {
			output.Message(cmd.OutOrStdout(), "No jobs running.")
		})
		return nil
	}

	pick, err := runpicker.RunPsPicker(jobs)
	if errors.Is(err, domain.ErrUserAborted) {
		return nil
	}
	if err != nil {
		return err
	}

	return execPsAction(cmd, pick)
}

func execPsAction(cmd *cobra.Command, pick runpicker.PsPickerResult) error {
	bin, err := os.Executable()
	if err != nil {
		return err
	}

	args, dir, ok := psInvocation(pick)
	if !ok {
		return nil
	}

	c := exec.Command(bin, args...)
	// The job's own directory, not this one: ps lists every repository the daemon
	// knows, and a worktree argument only ever resolves inside the current one.
	c.Dir = dir
	c.Stdin = os.Stdin
	c.Stdout = os.Stdout
	c.Stderr = os.Stderr
	return c.Run()
}

// psInvocation is the picked action as a command line: which subcommand to
// re-enter, and where to run it.
func psInvocation(pick runpicker.PsPickerResult) (args []string, dir string, ok bool) {
	jobFlag := "--" + domain.FlagJob

	switch pick.Action {
	case runpicker.ActionPsStop:
		args = []string{domain.CmdRun, domain.CmdStop, jobFlag, pick.Name}
	case runpicker.ActionPsLogs:
		args = []string{domain.CmdRun, domain.CmdLogs, jobFlag, pick.Name}
	case runpicker.ActionPsRestart:
		args = []string{domain.CmdRun, domain.CmdStart, jobFlag, pick.Name}
	case runpicker.ActionPsStopAll:
		args = []string{domain.CmdRun, domain.CmdDown, "--" + domain.FlagAll}
	default:
		return nil, "", false
	}
	return args, pick.WorkDir, true
}
