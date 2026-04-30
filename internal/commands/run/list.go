package run

import (
	"errors"
	"fmt"
	"os"
	"os/exec"

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
	shared.AddOutputFlag(cmd)
	return cmd
}

func runList(cmd *cobra.Command, _ []string) error {
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

	format, _ := cmd.Flags().GetString(domain.FlagOutput)
	if format == domain.OutputJSON {
		return output.WriteRunConfigJSON(cmd.OutOrStdout(), runCfg)
	}

	if !term.IsTerminal(int(os.Stdin.Fd())) {
		fmt.Fprint(cmd.OutOrStdout(), output.FormatRunConfig(runCfg))
		return nil
	}

	if len(runCfg.Jobs) == 0 && len(runCfg.Profiles) == 0 {
		output.Message(cmd.OutOrStdout(), "No jobs or profiles defined in run.toml.")
		return nil
	}

	pick, err := runpicker.RunListPicker(runCfg)
	if errors.Is(err, domain.ErrUserAborted) {
		return nil
	}
	if err != nil {
		return err
	}

	return execListAction(cmd, pick)
}

func execListAction(cmd *cobra.Command, pick runpicker.ListPickerResult) error {
	bin, err := os.Executable()
	if err != nil {
		return err
	}

	var args []string
	switch {
	case pick.Kind == runpicker.KindProfile && pick.Action == runpicker.ActionUp:
		args = []string{domain.CmdRun, domain.CmdUp, pick.Name}
	case pick.Kind == runpicker.KindProfile && pick.Action == runpicker.ActionDown:
		args = []string{domain.CmdRun, domain.CmdDown, pick.Name}
	case pick.Kind == runpicker.KindJob && pick.Action == runpicker.ActionStart:
		args = []string{domain.CmdRun, domain.CmdStart, pick.Name}
	case pick.Kind == runpicker.KindJob && pick.Action == runpicker.ActionStop:
		args = []string{domain.CmdRun, domain.CmdStop, pick.Name}
	case pick.Kind == runpicker.KindJob && pick.Action == runpicker.ActionLogs:
		args = []string{domain.CmdRun, domain.CmdLogs, pick.Name}
	default:
		return nil
	}

	c := exec.Command(bin, args...)
	c.Stdin = os.Stdin
	c.Stdout = os.Stdout
	c.Stderr = os.Stderr
	return c.Run()
}
