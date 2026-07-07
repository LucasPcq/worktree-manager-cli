package jobcmd

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
		Short: "Edit an existing job via wizard",
		Long: "Edit a job declared in <git-common-dir>/wtm/run.toml.\n\n" +
			"Without an argument, prompts to pick from the existing jobs. The wizard\n" +
			"is pre-filled with the current values; renaming is allowed and references\n" +
			"in profiles will be checked against the new name on save.",
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
		picked, pickErr := runpicker.PickJob(runpicker.PickJobParams{Config: cfg, Title: "Edit which job?"})
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

// runEditByName runs the edit wizard on the named job, persists the change,
// and emits the result. Callable from list once the user picked an action —
// keeps a single source of truth for the edit flow.
func runEditByName(params editByNameParams) error {
	current, exists := rules.FindJob(params.Config, params.Name)
	if !exists {
		return fmt.Errorf("job %q not found", params.Name)
	}

	updated, wizErr := runwizard.RunJobWizard(runwizard.JobWizardParams{
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
	for i, j := range cfg.Jobs {
		if j.Name == current.Name {
			cfg.Jobs[i] = updated
			break
		}
	}

	if err := runconfig.Save(runconfig.SaveParams{StateDir: params.Res.StateDir, Config: cfg}); err != nil {
		return err
	}

	format, _ := params.Cmd.Flags().GetString(domain.FlagOutput)
	if format == domain.OutputJSON {
		return output.WriteJobResultJSON(params.Cmd.OutOrStdout(), output.JobActionResult{
			Name:   updated.Name,
			Status: domain.JobActionUpdated,
		})
	}

	output.Frame(params.Cmd.OutOrStdout(), func() {
		output.Update(params.Cmd.OutOrStdout(), fmt.Sprintf("Updated job %q", updated.Name))
	})
	return nil
}
