package jobcmd

import (
	"errors"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/LucasPcq/wtm/internal/commands/shared"
	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/output"
	"github.com/LucasPcq/wtm/internal/service/runconfig"
	"github.com/LucasPcq/wtm/internal/tui/runwizard"
)

func newAddCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   domain.CmdAdd + " [name]",
		Short: "Add a job to run.toml",
		Long: "Append a job to <git-common-dir>/wtm/run.toml.\n\n" +
			"Pass --cmd (and optionally --kind, --stop, --cwd) for non-interactive use.\n" +
			"Without --cmd, prompts interactively for each field.",
		Args: cobra.MaximumNArgs(1),
		RunE: runAdd,
	}
	cmd.Flags().String(domain.FlagCmd, "", "Command to run (skips wizard when set with name)")
	cmd.Flags().String(domain.FlagKind, string(domain.JobKindService), "Job kind: service or task")
	cmd.Flags().String(domain.FlagStop, "", "Stop command (services only)")
	cmd.Flags().String(domain.FlagCwd, "", "Working directory (relative to project root)")
	shared.AddOutputFlag(cmd)
	return cmd
}

func runAdd(cmd *cobra.Command, args []string) error {
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

	cmdFlag, _ := cmd.Flags().GetString(domain.FlagCmd)
	var name string
	if len(args) > 0 {
		name = args[0]
	}

	var newJob domain.JobConfig

	if name == "" || cmdFlag == "" {
		result, wizErr := runwizard.RunJobWizard(runwizard.JobWizardParams{
			Existing: cfg,
			Initial:  domain.JobConfig{Name: name},
		})
		if errors.Is(wizErr, domain.ErrUserAborted) {
			return nil
		}
		if wizErr != nil {
			return wizErr
		}
		newJob = result
	} else {
		kind, _ := cmd.Flags().GetString(domain.FlagKind)
		stop, _ := cmd.Flags().GetString(domain.FlagStop)
		cwd, _ := cmd.Flags().GetString(domain.FlagCwd)
		newJob = domain.JobConfig{
			Name: name,
			Kind: domain.JobKind(kind),
			Cmd:  cmdFlag,
			Stop: stop,
			Cwd:  cwd,
		}
	}

	cfg.Jobs = append(cfg.Jobs, newJob)
	if err := runconfig.Save(runconfig.SaveParams{StateDir: res.StateDir, Config: cfg}); err != nil {
		return err
	}

	format, _ := cmd.Flags().GetString(domain.FlagOutput)
	if format == domain.OutputJSON {
		return output.WriteJobResultJSON(cmd.OutOrStdout(), output.JobActionResult{
			Name:   newJob.Name,
			Status: domain.JobActionAdded,
		})
	}

	output.Blank(cmd.OutOrStdout())
	output.Success(cmd.OutOrStdout(), fmt.Sprintf("Added job %q", newJob.Name))
	output.Blank(cmd.OutOrStdout())
	return nil
}
