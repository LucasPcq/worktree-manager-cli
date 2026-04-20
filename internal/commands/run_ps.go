package commands

import (
	"errors"
	"fmt"
	"os"
	"os/exec"

	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/output"
	runpicker "github.com/LucasPcq/wtm/internal/tui/run"
)

// newRunPsCmd creates the wtm run ps subcommand.
func newRunPsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   domain.CmdPs,
		Short: "List currently running jobs",
		Long:  "Show the jobs managed by the background daemon (name, kind, status, PID, worktree).\nIn a TTY, offers an interactive picker with stop/logs/restart actions.",
		RunE:  runRunPs,
	}
	addOutputFlag(cmd)
	return cmd
}

func runRunPs(cmd *cobra.Command, _ []string) error {
	format, _ := cmd.Flags().GetString(domain.FlagOutput)
	jobs := loadJobsGraceful()

	if format == domain.OutputJSON {
		return output.WriteRunningJobsJSON(cmd.OutOrStdout(), jobs)
	}

	if !term.IsTerminal(int(os.Stdin.Fd())) {
		fmt.Fprint(cmd.OutOrStdout(), output.FormatRunningJobs(jobs))
		return nil
	}

	if len(jobs) == 0 {
		output.Blank(cmd.OutOrStdout())
		output.Message(cmd.OutOrStdout(), "No jobs running.")
		output.Blank(cmd.OutOrStdout())
		return nil
	}

	pick, err := runpicker.RunPsPicker(jobs)
	if errors.Is(err, domain.ErrUserAborted) {
		return nil
	}
	if err != nil {
		return err
	}

	return execRunPsAction(cmd, pick)
}

func execRunPsAction(cmd *cobra.Command, pick runpicker.PsPickerResult) error {
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
