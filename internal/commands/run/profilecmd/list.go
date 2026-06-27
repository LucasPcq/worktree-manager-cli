package profilecmd

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
		Short: "List profiles from run.toml",
		Long: "List profiles declared in <git-common-dir>/wtm/run.toml.\n\n" +
			"In a TTY, opens an interactive picker. Selecting a profile offers Edit or Remove.\n" +
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
	res, err := shared.LoadConfig(cmd, wd)
	if err != nil {
		return err
	}
	cfg, err := runconfig.Load(res.StateDir)
	if err != nil {
		return fmt.Errorf("load run.toml: %w", err)
	}

	format, _ := cmd.Flags().GetString(domain.FlagOutput)
	if format == domain.OutputJSON {
		return output.WriteProfilesJSON(cmd.OutOrStdout(), cfg.Profiles)
	}

	if !term.IsTerminal(int(os.Stdin.Fd())) {
		output.Frame(cmd.OutOrStdout(), func() {
			fmt.Fprint(cmd.OutOrStdout(), output.FormatRunConfig(domain.RunConfig{Profiles: cfg.Profiles}))
		})
		return nil
	}

	pick, err := runpicker.PickProfileThenAction(cfg)
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
		return runRmByName(rmByNameParams{Cmd: cmd, Res: res, Config: cfg, Name: pick.Name})
	}
	return nil
}
