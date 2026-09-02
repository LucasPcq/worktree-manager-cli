package run

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/LucasPcq/wtm/internal/commands/shared"
	"github.com/LucasPcq/wtm/internal/config"
	"github.com/LucasPcq/wtm/internal/domain"
	downflow "github.com/LucasPcq/wtm/internal/flow/run/down"
	"github.com/LucasPcq/wtm/internal/output"
)

// newDownCmd creates the wtm run down subcommand.
func newDownCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   domain.CmdDown + " [worktree]",
		Short: "Stop a worktree's running jobs",
		Long:  "Stop the jobs running in [worktree] — the current one when omitted, picked interactively when there is a terminal.\nWith --profile, stops only that profile's jobs.\nJobs running in other worktrees are never touched.",
		Args:  cobra.MaximumNArgs(1),
		RunE:  runDown,
	}
	shared.AddProfileFlag(cmd, "Stop only this profile's jobs")
	shared.AddYesFlag(cmd, "Skip all prompts; stops what the worktree has running")
	shared.AddOutputFlag(cmd)
	cmd.Flags().Bool(domain.FlagAll, false, "Stop jobs across every worktree (bypasses per-worktree scoping)")
	return cmd
}

func runDown(cmd *cobra.Command, args []string) error {
	format, _ := cmd.Flags().GetString(domain.FlagOutput)
	yes, _ := cmd.Flags().GetBool(domain.FlagYes)
	all, _ := cmd.Flags().GetBool(domain.FlagAll)
	profile, _ := cmd.Flags().GetString(domain.FlagProfile)

	// --all is a different question, not a wider answer to this one: it takes
	// neither a worktree nor a profile.
	if all && (len(args) > 0 || profile != "") {
		return fmt.Errorf("--%s cannot be combined with a worktree or --%s", domain.FlagAll, domain.FlagProfile)
	}

	dir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get working directory: %w", err)
	}
	if err := shared.GuardRunInitialized(dir); err != nil {
		return err
	}

	result, err := shared.LoadConfig(cmd, dir)
	if err != nil {
		return err
	}
	runCfg, err := config.LoadRun(result.StateDir)
	if err != nil {
		return fmt.Errorf("load run config: %w", err)
	}

	outcome, err := downflow.Run(downflow.Params{
		Context: shared.FlowContext(result),
		Request: downflow.Request{
			Worktree: firstArg(args),
			Cwd:      dir,
			Profile:  profile,
			All:      all,
			Config:   runCfg,
		},
		Prompter: shared.FlowPrompter(shared.FlowPrompterParams{
			Interactive: !all && shared.Interactive(shared.UnattendedParams{TTY: isTTY(), Format: format, Yes: yes}),
			Stderr:      true,
		}),
		Presenter: downPresenter{CLIPresenter: shared.NewPresenter(cmd, format)},
	})
	if err != nil {
		return err
	}
	if outcome.Aborted || outcome.Failed() {
		return domain.ErrAborted
	}
	return nil
}

// downPresenter reports what the worktree had running. A job left standing is
// named on stderr and turned into a non-zero exit by the runner; both surfaces
// have already listed the jobs, so the error carries nothing more (LUC-198).
type downPresenter struct {
	shared.CLIPresenter
}

func (p downPresenter) Downed(outcome downflow.Outcome) error {
	if p.Format == domain.OutputJSON {
		return output.WriteJobResultsJSON(p.Cmd.OutOrStdout(), outcome.Results)
	}

	out, errOut := p.Cmd.OutOrStdout(), p.Cmd.ErrOrStderr()
	if outcome.NoDaemon || len(outcome.Results) == 0 {
		output.Frame(out, func() { output.Message(out, p.nothingRunning(outcome)) })
		return nil
	}

	output.FrameStart(out)
	for _, result := range outcome.Results {
		if result.Status == domain.JobActionError {
			output.Error(errOut, fmt.Sprintf("%s: %s", result.Name, result.Message))
			continue
		}
		output.Success(out, fmt.Sprintf(domain.RunStoppedFmt, result.Name))
	}
	output.FrameEnd(out)
	return nil
}

func (p downPresenter) nothingRunning(outcome downflow.Outcome) string {
	if outcome.All {
		return domain.RunNoJobsRunning
	}
	return domain.RunNoJobsHere
}
