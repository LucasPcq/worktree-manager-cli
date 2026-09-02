package run

import (
	"errors"
	"fmt"
	"os"
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
	shared.AddYesFlag(cmd, "Skip the interactive picker; print the table instead")
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

	yes, _ := cmd.Flags().GetBool(domain.FlagYes)
	if yes || !term.IsTerminal(int(os.Stdin.Fd())) {
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

// psDispatch is the picked action as this process will run it. The worktree
// comes from the job, never from here: ps lists every repository the daemon
// knows, so acting in the current directory would act on the wrong worktree —
// the bug LUC-211 set out to fix.
func psDispatch(cmd *cobra.Command, pick runpicker.PsPickerResult) (dispatchParams, bool) {
	switch pick.Action {
	case runpicker.ActionPsStop, runpicker.ActionPsLogs, runpicker.ActionPsRestart, runpicker.ActionPsStopAll:
	default:
		return dispatchParams{}, false
	}
	return dispatchParams{
		Cmd:     cmd,
		WorkDir: pick.WorkDir,
		Job:     pick.Name,
		Format:  domain.OutputText,
	}, true
}

func execPsAction(cmd *cobra.Command, pick runpicker.PsPickerResult) error {
	params, ok := psDispatch(cmd, pick)
	if !ok {
		return nil
	}
	switch pick.Action {
	case runpicker.ActionPsStop:
		return params.dispatchStop()
	case runpicker.ActionPsLogs:
		return params.dispatchLogs()
	case runpicker.ActionPsRestart:
		return params.dispatchStart()
	default:
		return params.dispatchDown(true)
	}
}
