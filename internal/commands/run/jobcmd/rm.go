package jobcmd

import (
	"github.com/spf13/cobra"

	"github.com/LucasPcq/wtm/internal/commands/run/runctx"
	"github.com/LucasPcq/wtm/internal/commands/shared"
	"github.com/LucasPcq/wtm/internal/domain"
	jobflow "github.com/LucasPcq/wtm/internal/flow/run/job"
)

func newRmCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   domain.CmdRm + " [name]",
		Short: "Remove a job from run.toml",
		Long: "Remove a job from <git-common-dir>/wtm/run.toml.\n\n" +
			"Without an argument, prompts to pick from the existing jobs; under --yes the\n" +
			"argument is required.\n" +
			"Fails if the job is referenced by any profile, unless --force is given\n" +
			"(in which case the references are stripped from those profiles too).",
		Args: cobra.MaximumNArgs(1),
		RunE: runRm,
	}
	cmd.Flags().Bool(domain.FlagForce, false, "Also strip references from profiles that use this job")
	shared.AddYesFlag(cmd, "Skip the picker; [name] is then required")
	shared.AddOutputFlag(cmd)
	return cmd
}

func runRm(cmd *cobra.Command, args []string) error {
	ctx, err := runctx.Open(runctx.OpenParams{Cmd: cmd})
	if err != nil {
		return err
	}

	force, _ := cmd.Flags().GetBool(domain.FlagForce)
	outcome, err := jobflow.Remove(jobflow.RemoveParams{
		Context: ctx.FlowContext(),
		Request: jobflow.RemoveRequest{
			Name:   runctx.FirstArg(args),
			Force:  force,
			Config: ctx.Run,
		},
		Prompter:  ctx.Prompter(ctx.Interactive),
		Presenter: presenter{CLIPresenter: ctx.CLI(cmd)},
	})
	if err != nil {
		return err
	}
	if outcome.Aborted {
		return domain.ErrAborted
	}
	return nil
}
