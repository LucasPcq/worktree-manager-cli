package jobcmd

import (
	"errors"
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/LucasPcq/wtm/internal/commands/shared"
	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/output"
	"github.com/LucasPcq/wtm/internal/service/runconfig"
	"github.com/LucasPcq/wtm/internal/tui/runpicker"
)

func newListCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   domain.CmdList,
		Short: "List jobs from run.toml",
		Long: "List jobs declared in <git-common-dir>/wtm/run.toml.\n\n" +
			"In a TTY, opens an interactive picker. Selecting a job offers Edit or Remove.\n" +
			"Use --output json (or pipe stdout) for a non-interactive listing.",
		RunE: runList,
	}
	shared.AddOutputFlag(cmd)
	return cmd
}

func runList(cmd *cobra.Command, _ []string) error {
	wd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get working directory: %w", err)
	}
	res, ok := shared.LoadConfig(cmd, wd)
	if !ok {
		return nil
	}
	cfg, err := runconfig.Load(res.StateDir)
	if err != nil {
		return fmt.Errorf("load run.toml: %w", err)
	}

	format, _ := cmd.Flags().GetString(domain.FlagOutput)
	if format == domain.OutputJSON {
		return output.WriteJobsJSON(cmd.OutOrStdout(), cfg.Jobs)
	}

	if !term.IsTerminal(int(os.Stdin.Fd())) {
		fmt.Fprint(cmd.OutOrStdout(), output.FormatRunConfig(domain.RunConfig{Jobs: cfg.Jobs}))
		return nil
	}

	pick, err := runpicker.PickJobThenAction(cfg)
	if errors.Is(err, domain.ErrUserAborted) {
		return nil
	}
	if err != nil {
		return err
	}

	switch pick.Action {
	case runpicker.ActionEdit:
		return runEditByName(editByNameParams{Cmd: cmd, Res: res, Config: cfg, Name: pick.Name})
	case runpicker.ActionRm:
		return runRmByName(rmByNameParams{Cmd: cmd, Res: res, Config: cfg, Name: pick.Name, Force: false})
	}
	return nil
}
