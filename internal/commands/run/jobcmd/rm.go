package jobcmd

import (
	"errors"
	"fmt"
	"os"
	"strings"

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
		Short: "Remove a job from run.toml",
		Long: "Remove a job from <git-common-dir>/wtm/run.toml.\n\n" +
			"Without an argument, prompts to pick from the existing jobs.\n" +
			"Fails if the job is referenced by any profile, unless --force is given\n" +
			"(in which case the references are stripped from those profiles too).",
		Args: cobra.MaximumNArgs(1),
		RunE: runRm,
	}
	cmd.Flags().Bool(domain.FlagForce, false, "Also strip references from profiles that use this job")
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

	if err := shared.RequireRunInitialized(cfg); err != nil {
		return err
	}

	var name string
	if len(args) > 0 {
		name = args[0]
	} else {
		picked, pickErr := runpicker.PickJob(runpicker.PickJobParams{Config: cfg, Title: "Remove which job?"})
		if errors.Is(pickErr, domain.ErrUserAborted) {
			return nil
		}
		if pickErr != nil {
			return pickErr
		}
		name = picked
	}

	force, _ := cmd.Flags().GetBool(domain.FlagForce)
	return runRmByName(rmByNameParams{Cmd: cmd, Res: res, Config: cfg, Name: name, Force: force})
}

// rmByNameParams groups inputs for runRmByName.
type rmByNameParams struct {
	Cmd    *cobra.Command
	Res    shared.ConfigResult
	Config domain.RunConfig
	Name   string
	Force  bool
}

// runRmByName removes the named job from cfg, optionally stripping references
// in profiles when force=true. Callable from list once the user picked an
// action — keeps a single source of truth for the rm flow.
func runRmByName(params rmByNameParams) error {
	if _, exists := rules.FindJob(params.Config, params.Name); !exists {
		return fmt.Errorf("job %q not found", params.Name)
	}

	cfg, effect := rules.RemoveJob(params.Config, params.Name)
	if len(effect.Profiles) > 0 && !params.Force {
		return fmt.Errorf("job %q is referenced by profile(s): %s — pass --force to strip those references",
			params.Name, strings.Join(effect.Profiles, ", "))
	}

	if err := runconfig.Save(runconfig.SaveParams{StateDir: params.Res.StateDir, Config: cfg}); err != nil {
		return err
	}

	format, _ := params.Cmd.Flags().GetString(domain.FlagOutput)
	if format == domain.OutputJSON {
		return output.WriteJobResultJSON(params.Cmd.OutOrStdout(), domain.JobActionResult{
			Name:   params.Name,
			Status: domain.JobActionRemoved,
		})
	}

	output.Frame(params.Cmd.OutOrStdout(), func() {
		output.Success(params.Cmd.OutOrStdout(), fmt.Sprintf("Removed job %q", params.Name))
		if len(effect.Profiles) > 0 {
			output.Message(params.Cmd.OutOrStdout(), fmt.Sprintf(domain.JobRemovedProfilesFmt, strings.Join(effect.Profiles, ", ")))
		}
		if len(effect.EmptiedProfiles) > 0 {
			output.Message(params.Cmd.OutOrStdout(), fmt.Sprintf(domain.JobRemovedEmptiedFmt, strings.Join(effect.EmptiedProfiles, ", ")))
		}
		if len(effect.EnvPorts) > 0 {
			output.Message(params.Cmd.OutOrStdout(), fmt.Sprintf(domain.JobRemovedEnvPortsFmt, strings.Join(effect.EnvPorts, ", ")))
		}
	})
	return nil
}
