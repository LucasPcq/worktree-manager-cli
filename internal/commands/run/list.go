package run

import (
	"errors"
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/LucasPcq/wtm/internal/commands/shared"
	"github.com/LucasPcq/wtm/internal/config"
	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/output"
	runpicker "github.com/LucasPcq/wtm/internal/tui/runpicker"
)

// newListCmd creates the wtm run list subcommand.
func newListCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   domain.CmdList,
		Short: "List jobs and profiles declared in run.toml",
		Long:  "Show the jobs and profiles configured for the project.\nIn a TTY, offers an interactive picker with start/stop/logs actions.",
		RunE:  runList,
	}
	shared.AddYesFlag(cmd, "Skip the interactive picker; print the table instead")
	shared.AddOutputFlag(cmd)
	return cmd
}

func runList(cmd *cobra.Command, _ []string) error {
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
	if format == domain.OutputJSON {
		return output.WriteRunConfigJSON(cmd.OutOrStdout(), runCfg)
	}

	yes, _ := cmd.Flags().GetBool(domain.FlagYes)
	if yes || !term.IsTerminal(int(os.Stdin.Fd())) {
		output.Frame(cmd.OutOrStdout(), func() {
			fmt.Fprint(cmd.OutOrStdout(), output.FormatRunConfig(runCfg))
		})
		return nil
	}

	if len(runCfg.Jobs) == 0 && len(runCfg.Profiles) == 0 {
		output.Frame(cmd.OutOrStdout(), func() {
			output.Message(cmd.OutOrStdout(), "No jobs or profiles defined in run.toml.")
		})
		return nil
	}

	pick, err := runpicker.RunListPicker(runCfg)
	if errors.Is(err, domain.ErrUserAborted) {
		return nil
	}
	if err != nil {
		return err
	}

	return execListAction(cmd, pick, dir)
}

// execListAction runs the picked action in this process, on the worktree the
// command was launched from: `run list` shows what run.toml declares here, not
// what is running anywhere.
func execListAction(cmd *cobra.Command, pick runpicker.ListPickerResult, workDir string) error {
	params := dispatchParams{Cmd: cmd, WorkDir: workDir, Format: domain.OutputText}
	switch {
	case pick.Kind == runpicker.KindProfile && pick.Action == runpicker.ActionUp:
		params.Profile = pick.Name
		return params.dispatchUp()
	case pick.Kind == runpicker.KindProfile && pick.Action == runpicker.ActionDown:
		params.Profile = pick.Name
		return params.dispatchDown(false)
	case pick.Kind == runpicker.KindJob && pick.Action == runpicker.ActionStart:
		params.Job = pick.Name
		return params.dispatchStart()
	case pick.Kind == runpicker.KindJob && pick.Action == runpicker.ActionStop:
		params.Job = pick.Name
		return params.dispatchStop()
	case pick.Kind == runpicker.KindJob && pick.Action == runpicker.ActionLogs:
		params.Job = pick.Name
		return params.dispatchLogs()
	default:
		return nil
	}
}
