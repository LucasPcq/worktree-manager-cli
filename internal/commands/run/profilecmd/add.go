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
	"github.com/LucasPcq/wtm/internal/tui/runwizard"
)

func newAddCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   domain.CmdAdd + " [name]",
		Short: "Add a profile to run.toml",
		Long: "Append a profile to <git-common-dir>/wtm/run.toml.\n\n" +
			"Pass --jobs (comma-separated existing job names) for non-interactive use.\n" +
			"Without --jobs, prompts interactively (multiselect over existing jobs).",
		Args: cobra.MaximumNArgs(1),
		RunE: runAdd,
	}
	cmd.Flags().StringSlice(domain.FlagJobs, nil, "Comma-separated existing job names (skips wizard when set with name)")
	cmd.Flags().Bool(domain.FlagDefault, false, "Mark this profile as the default")
	shared.AddOutputFlag(cmd)
	return cmd
}

func runAdd(cmd *cobra.Command, args []string) error {
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

	jobsFlag, _ := cmd.Flags().GetStringSlice(domain.FlagJobs)
	defaultFlag, _ := cmd.Flags().GetBool(domain.FlagDefault)

	var name string
	if len(args) > 0 {
		name = args[0]
	}

	var newProfile domain.ProfileConfig

	if name == "" || len(jobsFlag) == 0 {
		result, wizErr := runwizard.RunProfileWizard(runwizard.ProfileWizardParams{
			Existing: cfg,
			Initial:  domain.ProfileConfig{Name: name, Default: defaultFlag},
		})
		if errors.Is(wizErr, domain.ErrUserAborted) {
			return nil
		}
		if wizErr != nil {
			return wizErr
		}
		newProfile = result
	} else {
		newProfile = domain.ProfileConfig{
			Name:    name,
			Jobs:    jobsFlag,
			Default: defaultFlag,
		}
	}

	cfg.Profiles = append(cfg.Profiles, newProfile)
	if newProfile.Default {
		cfg = rules.ApplyDefaultOverride(cfg, newProfile.Name)
	}
	if err := runconfig.Save(runconfig.SaveParams{StateDir: res.StateDir, Config: cfg}); err != nil {
		return err
	}

	format, _ := cmd.Flags().GetString(domain.FlagOutput)
	if format == domain.OutputJSON {
		return output.WriteProfileResultJSON(cmd.OutOrStdout(), output.ProfileActionResult{
			Name:   newProfile.Name,
			Status: domain.JobActionAdded,
		})
	}

	output.Frame(cmd.OutOrStdout(), func() {
		output.Success(cmd.OutOrStdout(), fmt.Sprintf("Added profile %q", newProfile.Name))
	})
	return nil
}
