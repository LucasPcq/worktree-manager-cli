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
	"github.com/LucasPcq/wtm/internal/rules"
	"github.com/LucasPcq/wtm/internal/service/runconfig"
	"github.com/LucasPcq/wtm/internal/tui/runpicker"
	"github.com/LucasPcq/wtm/internal/tui/runwizard"
)

func newEditCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   domain.CmdEdit + " [name]",
		Short: "Edit an existing profile",
		Long: "Edit a profile declared in <git-common-dir>/wtm/run.toml.\n\n" +
			"Pass --name, --jobs or --default for non-interactive use: a flag left out\n" +
			"keeps the field as it is. --jobs replaces the whole list — its order is the\n" +
			"start order, so it is given in full — and --default=false takes the default\n" +
			"away without handing it to another profile.\n\n" +
			"With no such flag, the wizard opens pre-filled with the current values, and\n" +
			"without an argument it prompts to pick from the existing profiles.",
		Args: cobra.MaximumNArgs(1),
		RunE: runEdit,
	}
	cmd.Flags().String(domain.FlagName, "", "Rename the profile")
	cmd.Flags().StringSlice(domain.FlagJobs, nil, "Comma-separated existing job names, in start order (replaces the list)")
	cmd.Flags().Bool(domain.FlagDefault, false, "Mark this profile as the default (--default=false takes it away)")
	shared.AddOutputFlag(cmd)
	return cmd
}

// profilePatchFromFlags reads the edit flags as a patch: only what the user
// actually passed, so an absent flag can be told from an explicit value.
func profilePatchFromFlags(cmd *cobra.Command) rules.ProfilePatch {
	var patch rules.ProfilePatch
	if cmd.Flags().Changed(domain.FlagName) {
		name, _ := cmd.Flags().GetString(domain.FlagName)
		patch.Name = &name
	}
	if cmd.Flags().Changed(domain.FlagJobs) {
		jobs, _ := cmd.Flags().GetStringSlice(domain.FlagJobs)
		patch.Jobs = jobs
	}
	if cmd.Flags().Changed(domain.FlagDefault) {
		isDefault, _ := cmd.Flags().GetBool(domain.FlagDefault)
		patch.Default = &isDefault
	}
	return patch
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

	format, _ := cmd.Flags().GetString(domain.FlagOutput)
	interactive := rules.IsHumanFormat(format) && term.IsTerminal(int(os.Stdin.Fd()))

	var name string
	switch {
	case len(args) > 0:
		name = args[0]
	case !interactive:
		return fmt.Errorf("edit needs the profile to edit as an argument when there is no terminal to pick one in")
	default:
		picked, pickErr := runpicker.PickProfile(runpicker.PickProfileParams{Config: cfg, Title: "Edit which profile?"})
		if errors.Is(pickErr, domain.ErrUserAborted) {
			return nil
		}
		if pickErr != nil {
			return pickErr
		}
		name = picked
	}

	return runEditByName(editByNameParams{
		Cmd:         cmd,
		Res:         res,
		Config:      cfg,
		Name:        name,
		Patch:       profilePatchFromFlags(cmd),
		Interactive: interactive,
	})
}

// editByNameParams groups inputs for runEditByName. An empty Patch opens the
// wizard; otherwise the flags alone decide, without a question.
type editByNameParams struct {
	Cmd         *cobra.Command
	Res         shared.ConfigResult
	Config      domain.RunConfig
	Name        string
	Patch       rules.ProfilePatch
	Interactive bool
}

// runEditByName applies the patch (or runs the wizard) on the named profile,
// persists the change, and emits the result.
func runEditByName(params editByNameParams) error {
	current, exists := rules.FindProfile(params.Config, params.Name)
	if !exists {
		return fmt.Errorf("profile %q not found", params.Name)
	}

	updated, err := editedProfile(params, current)
	if errors.Is(err, domain.ErrUserAborted) {
		return nil
	}
	if err != nil {
		return err
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
		output.Update(params.Cmd.OutOrStdout(), fmt.Sprintf("Updated profile %q", updated.Name))
	})
	return nil
}

func editedProfile(params editByNameParams, current domain.ProfileConfig) (domain.ProfileConfig, error) {
	if params.Patch.Empty() {
		if !params.Interactive {
			return domain.ProfileConfig{}, fmt.Errorf("edit has nothing to change — pass --%s, --%s or --%s",
				domain.FlagName, domain.FlagJobs, domain.FlagDefault)
		}
		return runwizard.RunProfileWizard(runwizard.ProfileWizardParams{
			Existing:    params.Config,
			Initial:     current,
			ExcludeName: current.Name,
		})
	}
	return rules.ApplyProfilePatch(rules.ApplyProfilePatchParams{Current: current, Patch: params.Patch})
}
