package profilecmd

import (
	"errors"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/LucasPcq/wtm/internal/commands/shared"
	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/output"
	"github.com/LucasPcq/wtm/internal/rules"
	"github.com/LucasPcq/wtm/internal/service/runconfig"
	"github.com/LucasPcq/wtm/internal/tui/runpicker"
	"github.com/LucasPcq/wtm/internal/tui/runwizard"
)

func newEditCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   domain.CmdEdit + " [name]",
		Short: "Edit an existing profile via wizard",
		Long: "Edit a profile declared in <git-common-dir>/wtm/run.toml.\n\n" +
			"Without an argument, prompts to pick from the existing profiles. The wizard\n" +
			"is pre-filled with the current values; renaming is allowed and configuration\n" +
			"is re-validated on save.",
		Args: cobra.MaximumNArgs(1),
		RunE: runEdit,
	}
	shared.AddOutputFlag(cmd)
	return cmd
}

func runEdit(cmd *cobra.Command, args []string) error {
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

	if err := shared.RequireRunInitialized(cfg); err != nil {
		return err
	}

	var name string
	if len(args) > 0 {
		name = args[0]
	} else {
		picked, pickErr := runpicker.PickProfile(runpicker.PickProfileParams{Config: cfg, Title: "Edit which profile?"})
		if errors.Is(pickErr, domain.ErrUserAborted) {
			return nil
		}
		if pickErr != nil {
			return pickErr
		}
		name = picked
	}

	return runEditByName(editByNameParams{Cmd: cmd, Res: res, Config: cfg, Name: name})
}

// editByNameParams groups inputs for runEditByName.
type editByNameParams struct {
	Cmd    *cobra.Command
	Res    shared.ConfigResult
	Config domain.RunConfig
	Name   string
}

// runEditByName runs the edit wizard on the named profile, persists the
// change, and emits the result.
func runEditByName(params editByNameParams) error {
	current, exists := rules.FindProfile(params.Config, params.Name)
	if !exists {
		return fmt.Errorf("profile %q not found", params.Name)
	}

	updated, wizErr := runwizard.RunProfileWizard(runwizard.ProfileWizardParams{
		Existing:    params.Config,
		Initial:     current,
		ExcludeName: current.Name,
	})
	if errors.Is(wizErr, domain.ErrUserAborted) {
		return nil
	}
	if wizErr != nil {
		return wizErr
	}

	cfg := params.Config
	for i, p := range cfg.Profiles {
		if p.Name == current.Name {
			cfg.Profiles[i] = updated
			break
		}
	}
	if updated.Default {
		cfg = rules.ApplyDefaultOverride(cfg, updated.Name)
	}

	if err := runconfig.Save(runconfig.SaveParams{StateDir: params.Res.StateDir, Config: cfg}); err != nil {
		return err
	}

	format, _ := params.Cmd.Flags().GetString(domain.FlagOutput)
	if format == domain.OutputJSON {
		return output.WriteProfileResultJSON(params.Cmd.OutOrStdout(), output.ProfileActionResult{
			Name:   updated.Name,
			Status: domain.JobActionUpdated,
		})
	}

	output.Frame(params.Cmd.OutOrStdout(), func() {
		output.Success(params.Cmd.OutOrStdout(), fmt.Sprintf("Updated profile %q", updated.Name))
	})
	return nil
}
