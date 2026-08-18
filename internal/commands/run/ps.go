package run

import (
	"errors"
	"fmt"
	"os"
	"os/exec"

	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/LucasPcq/wtm/internal/commands/shared"
	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/output"
	"github.com/LucasPcq/wtm/internal/service/process"
	"github.com/LucasPcq/wtm/internal/tui/components"
	runpicker "github.com/LucasPcq/wtm/internal/tui/runpicker"
)

// newPsCmd creates the wtm run ps subcommand.
func newPsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   domain.CmdPs,
		Short: "List currently running jobs",
		Long:  "Show the jobs managed by the background daemon (name, kind, status, PID, worktree).\nIn a TTY, offers an interactive picker with stop/logs/restart actions.",
		RunE:  runPs,
	}
	shared.AddOutputFlag(cmd)
	return cmd
}

func runPs(cmd *cobra.Command, _ []string) error {
	if err := process.SupportedOnPlatform(); err != nil {
		return err
	}

	dir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get working directory: %w", err)
	}
	if err := shared.GuardRunInitialized(dir); err != nil {
		return err
	}

	format, _ := cmd.Flags().GetString(domain.FlagOutput)

	if format == domain.OutputJSON {
		return output.WriteRunningJobsJSON(cmd.OutOrStdout(), shared.LoadJobsGraceful())
	}

	var jobs []domain.JobInfo
	_ = components.RunLoading(components.LoadingParams{
		Message: "Loading jobs…",
		Animate: true,
		Work:    func() error { jobs = shared.LoadJobsGraceful(); return nil },
	})

	if !term.IsTerminal(int(os.Stdin.Fd())) {
		output.Frame(cmd.OutOrStdout(), func() {
			fmt.Fprint(cmd.OutOrStdout(), output.FormatRunningJobs(jobs))
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

	var args []string
	switch pick.Action {
	case runpicker.ActionPsStop:
		args = []string{domain.CmdRun, domain.CmdStop, pick.Name}
	case runpicker.ActionPsLogs:
		args = []string{domain.CmdRun, domain.CmdLogs, pick.Name}
	case runpicker.ActionPsRestart:
		args = []string{domain.CmdRun, domain.CmdStart, pick.Name}
	case runpicker.ActionPsStopAll:
		args = []string{domain.CmdRun, domain.CmdDown, "--all"}
	default:
		return nil
	}

	c := exec.Command(bin, args...)
	c.Stdin = os.Stdin
	c.Stdout = os.Stdout
	c.Stderr = os.Stderr
	return c.Run()
}
