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
	"github.com/LucasPcq/wtm/internal/rules"
	"github.com/LucasPcq/wtm/internal/service/runconfig"
	"github.com/LucasPcq/wtm/internal/tui/runpicker"
	"github.com/LucasPcq/wtm/internal/tui/runwizard"
)

func newEditCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   domain.CmdEdit + " [name]",
		Short: "Edit an existing job",
		Long: "Edit a job declared in <git-common-dir>/wtm/run.toml.\n\n" +
			"Pass any of --name, --cmd, --kind, --stop, --cwd, --port, --port-clear,\n" +
			"--url-port or --url-host for non-interactive use: a flag left out keeps the\n" +
			"field as it is, and passing an empty string clears it (--stop '' drops the\n" +
			"stop command, --url-port '' withdraws the published name).\n\n" +
			"--port merges into the ports the job already declares, so one entry can be\n" +
			"changed without rewriting the others; --port-clear empties the table.\n" +
			"--name also rewrites the references to this job in every profile.\n\n" +
			"With no such flag, the wizard opens pre-filled with the current values, and\n" +
			"without an argument it prompts to pick from the existing jobs.",
		Args: cobra.MaximumNArgs(1),
		RunE: runEdit,
	}
	cmd.Flags().String(domain.FlagName, "", "Rename the job, updating the profiles that reference it")
	cmd.Flags().String(domain.FlagCmd, "", "Command to run, as a /bin/sh line")
	cmd.Flags().String(domain.FlagKind, "", "Job kind: service or task")
	cmd.Flags().String(domain.FlagStop, "", "Stop command, as a /bin/sh line (pass '' to drop it)")
	cmd.Flags().String(domain.FlagCwd, "", "Working directory relative to project root (pass '' to drop it)")
	cmd.Flags().StringArray(domain.FlagPort, nil, "Base port as NAME=PORT, repeatable — merged into the declared ports")
	cmd.Flags().Bool(domain.FlagPortClear, false, "Drop every port this job declares")
	cmd.Flags().String(domain.FlagURLPort, "", "Publish this declared port under a name (pass '' to withdraw the url)")
	cmd.Flags().String(domain.FlagURLHost, "", "Host segment to publish under (pass '' to fall back to the job's name)")
	shared.AddOutputFlag(cmd)
	return cmd
}

// jobPatchFromFlags reads the edit flags as a patch: only what the user
// actually passed, so an absent flag can be told from an explicit empty value.
func jobPatchFromFlags(cmd *cobra.Command) (rules.JobPatch, error) {
	patch := rules.JobPatch{ClearPorts: changedBool(cmd, domain.FlagPortClear)}
	for _, field := range []struct {
		flag string
		into **string
	}{
		{domain.FlagName, &patch.Name},
		{domain.FlagCmd, &patch.Cmd},
		{domain.FlagKind, &patch.Kind},
		{domain.FlagStop, &patch.Stop},
		{domain.FlagCwd, &patch.Cwd},
		{domain.FlagURLPort, &patch.URLPort},
		{domain.FlagURLHost, &patch.URLHost},
	} {
		if !cmd.Flags().Changed(field.flag) {
			continue
		}
		value, _ := cmd.Flags().GetString(field.flag)
		*field.into = &value
	}

	portFlags, _ := cmd.Flags().GetStringArray(domain.FlagPort)
	ports, err := rules.ParsePorts(portFlags)
	if err != nil {
		return rules.JobPatch{}, err
	}
	patch.Ports = ports

	return patch, nil
}

func changedBool(cmd *cobra.Command, flag string) bool {
	if !cmd.Flags().Changed(flag) {
		return false
	}
	value, _ := cmd.Flags().GetBool(flag)
	return value
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

	patch, err := jobPatchFromFlags(cmd)
	if err != nil {
		return err
	}

	format, _ := cmd.Flags().GetString(domain.FlagOutput)
	interactive := rules.IsHumanFormat(format) && term.IsTerminal(int(os.Stdin.Fd()))

	var name string
	switch {
	case len(args) > 0:
		name = args[0]
	case !interactive:
		return fmt.Errorf("edit needs the job to edit as an argument when there is no terminal to pick one in")
	default:
		picked, pickErr := runpicker.PickJob(runpicker.PickJobParams{Config: cfg, Title: "Edit which job?"})
		if errors.Is(pickErr, domain.ErrUserAborted) {
			return nil
		}
		if pickErr != nil {
			return pickErr
		}
		name = picked
	}

	return runEditByName(editByNameParams{Cmd: cmd, Res: res, Config: cfg, Name: name, Patch: patch, Interactive: interactive})
}

// editByNameParams groups inputs for runEditByName. An empty Patch opens the
// wizard; otherwise the flags alone decide, without a question.
type editByNameParams struct {
	Cmd         *cobra.Command
	Res         shared.ConfigResult
	Config      domain.RunConfig
	Name        string
	Patch       rules.JobPatch
	Interactive bool
}

// runEditByName applies the patch (or runs the wizard) on the named job,
// persists the change, and emits the result. Callable from list once the user
// picked an action — keeps a single source of truth for the edit flow.
func runEditByName(params editByNameParams) error {
	current, exists := rules.FindJob(params.Config, params.Name)
	if !exists {
		return fmt.Errorf("job %q not found", params.Name)
	}

	updated, err := editedJob(params, current)
	if errors.Is(err, domain.ErrUserAborted) {
		return nil
	}
	if err != nil {
		return err
	}

	cfg := rules.RenameJobRefs(params.Config, current.Name, updated.Name)
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
		return output.WriteJobResultJSON(params.Cmd.OutOrStdout(), domain.JobActionResult{
			Name:   updated.Name,
			Status: domain.JobActionUpdated,
		})
	}

	output.Frame(params.Cmd.OutOrStdout(), func() {
		output.Update(params.Cmd.OutOrStdout(), fmt.Sprintf("Updated job %q", updated.Name))
	})
	return nil
}

func editedJob(params editByNameParams, current domain.JobConfig) (domain.JobConfig, error) {
	if params.Patch.Empty() {
		if !params.Interactive {
			return domain.JobConfig{}, fmt.Errorf("edit has nothing to change — pass --%s, --%s, --%s, --%s, --%s, --%s, --%s, --%s or --%s",
				domain.FlagName, domain.FlagCmd, domain.FlagKind, domain.FlagStop, domain.FlagCwd,
				domain.FlagPort, domain.FlagPortClear, domain.FlagURLPort, domain.FlagURLHost)
		}
		return runwizard.RunJobWizard(runwizard.JobWizardParams{
			Existing:    params.Config,
			Initial:     current,
			ExcludeName: current.Name,
		})
	}
	return rules.ApplyJobPatch(rules.ApplyJobPatchParams{Current: current, Patch: params.Patch})
}
