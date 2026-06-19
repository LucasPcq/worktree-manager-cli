package profilecmd

import (
	"errors"
	"fmt"
	"os"
	"slices"

	"github.com/spf13/cobra"

	"github.com/LucasPcq/wtm/internal/commands/shared"
	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/output"
	"github.com/LucasPcq/wtm/internal/rules"
	"github.com/LucasPcq/wtm/internal/service/runconfig"
	"github.com/LucasPcq/wtm/internal/tui/runpicker"
)

func newRmCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   domain.CmdRm + " [name]",
		Short: "Remove a profile from run.toml",
		Long: "Remove a profile from <git-common-dir>/wtm/run.toml.\n\n" +
			"Without an argument, prompts to pick from the existing profiles.\n" +
			"Jobs referenced by the profile are left untouched.",
		Args: cobra.MaximumNArgs(1),
		RunE: runRm,
	}
	shared.AddOutputFlag(cmd)
	return cmd
}

func runRm(cmd *cobra.Command, args []string) error {
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

	var name string
	if len(args) > 0 {
		name = args[0]
	} else {
		picked, pickErr := runpicker.PickProfile(runpicker.PickProfileParams{Config: cfg, Title: "Remove which profile?"})
		if errors.Is(pickErr, domain.ErrUserAborted) {
			return nil
		}
		if pickErr != nil {
			return pickErr
		}
		name = picked
	}

	return runRmByName(rmByNameParams{Cmd: cmd, Res: res, Config: cfg, Name: name})
}

// rmByNameParams groups inputs for runRmByName.
type rmByNameParams struct {
	Cmd    *cobra.Command
	Res    shared.ConfigResult
	Config domain.RunConfig
	Name   string
}

// runRmByName removes the named profile from cfg and persists it.
func runRmByName(params rmByNameParams) error {
	if _, exists := rules.FindProfile(params.Config, params.Name); !exists {
		return fmt.Errorf("profile %q not found", params.Name)
	}

	cfg := params.Config
	cfg.Profiles = slices.DeleteFunc(cfg.Profiles, func(p domain.ProfileConfig) bool { return p.Name == params.Name })

	if err := runconfig.Save(runconfig.SaveParams{StateDir: params.Res.StateDir, Config: cfg}); err != nil {
		return err
	}

	format, _ := params.Cmd.Flags().GetString(domain.FlagOutput)
	if format == domain.OutputJSON {
		return output.WriteProfileResultJSON(params.Cmd.OutOrStdout(), output.ProfileActionResult{
			Name:   params.Name,
			Status: domain.JobActionRemoved,
		})
	}

	output.Blank(params.Cmd.OutOrStdout())
	output.Success(params.Cmd.OutOrStdout(), fmt.Sprintf("Removed profile %q", params.Name))
	output.Blank(params.Cmd.OutOrStdout())
	return nil
}
