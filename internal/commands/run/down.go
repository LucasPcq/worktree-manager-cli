package run

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/LucasPcq/wtm/internal/commands/shared"
	"github.com/LucasPcq/wtm/internal/config"
	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/output"
	"github.com/LucasPcq/wtm/internal/rules"
	"github.com/LucasPcq/wtm/internal/service/process"
	"github.com/LucasPcq/wtm/internal/tui/components"
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
	shared.AddOutputFlag(cmd)
	cmd.Flags().Bool(domain.FlagAll, false, "Stop jobs across every worktree (bypasses per-worktree scoping)")
	return cmd
}

func runDown(cmd *cobra.Command, args []string) error {
	format, _ := cmd.Flags().GetString(domain.FlagOutput)
	all, _ := cmd.Flags().GetBool(domain.FlagAll)

	profileName, _ := cmd.Flags().GetString(domain.FlagProfile)

	if all && (len(args) > 0 || profileName != "") {
		return fmt.Errorf("--%s cannot be combined with a worktree or --%s", domain.FlagAll, domain.FlagProfile)
	}

	dir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get working directory: %w", err)
	}
	if err := shared.GuardRunInitialized(dir); err != nil {
		return err
	}

	projectDir, err := shared.ProjectRoot(dir)
	if err != nil {
		return err
	}
	stateDir, err := shared.StateDir(dir)
	if err != nil {
		return err
	}
	runCfg, err := config.LoadRun(stateDir)
	if err != nil {
		return fmt.Errorf("load run config: %w", err)
	}

	resolved, err := resolveInputs(inputsParams{
		Args:        args,
		Cwd:         dir,
		ProjectDir:  projectDir,
		Interactive: !all && isTTY() && rules.IsHumanFormat(format),
		Pick:        true,
		// No profile step: `run down` with no --profile means "stop everything in
		// this worktree", which is a safe default. Asking would force a choice the
		// command does not need and offers no "all" answer for.
		Second: secondAxis{Given: profileName},
	})
	if err != nil {
		return err
	}
	profileName = resolved.Second

	socketPath := process.SocketPath()

	if err := ensureDaemonForDown(cmd, ensureDownParams{SocketPath: socketPath, Dir: resolved.Dir, All: all}); err != nil {
		return err
	}
	if !process.IsDaemonRunning(socketPath) {
		if format == domain.OutputJSON {
			return output.WriteJobResultsJSON(cmd.OutOrStdout(), nil)
		}
		output.Frame(cmd.OutOrStdout(), func() {
			output.Message(cmd.OutOrStdout(), "No jobs running.")
		})
		return nil
	}

	client := process.NewClient(socketPath)

	if profileName != "" {
		profile, ok := rules.FindProfile(runCfg, profileName)
		if !ok {
			return fmt.Errorf("profile %q not found in config", profileName)
		}

		jobs := rules.ProfileJobs(runCfg, profile)
		results := make([]domain.JobActionResult, 0, len(jobs))
		if rules.IsHumanFormat(format) {
			output.FrameStart(cmd.OutOrStdout())
		}
		for _, job := range jobs {
			var resp process.Response
			sendErr := components.RunLoading(components.LoadingParams{
				Message: fmt.Sprintf("Stopping %s…", job.Name),
				Animate: rules.IsHumanFormat(format),
				Work: func() error {
					var e error
					resp, e = client.Send(process.Request{
						Action:  process.ActionStop,
						Name:    job.Name,
						WorkDir: resolved.Dir,
					})
					return e
				},
			})
			if sendErr != nil {
				results = append(results, domain.JobActionResult{Name: job.Name, Status: domain.JobActionError, Message: sendErr.Error()})
				if format != domain.OutputJSON {
					output.Error(cmd.ErrOrStderr(), fmt.Sprintf("%s: %v", job.Name, sendErr))
				}
				continue
			}
			if resp.Status == process.StatusError {
				results = append(results, domain.JobActionResult{Name: job.Name, Status: domain.JobActionError, Message: resp.Message})
				if format != domain.OutputJSON {
					output.Error(cmd.ErrOrStderr(), fmt.Sprintf("%s: %s", job.Name, resp.Message))
				}
				continue
			}
			results = append(results, domain.JobActionResult{Name: job.Name, Status: domain.JobActionStopped})
			if format != domain.OutputJSON {
				output.Success(cmd.OutOrStdout(), fmt.Sprintf("%s stopped", job.Name))
			}
		}
		if format == domain.OutputJSON {
			if err := output.WriteJobResultsJSON(cmd.OutOrStdout(), results); err != nil {
				return err
			}
		} else {
			output.FrameEnd(cmd.OutOrStdout())
		}
		// A job left standing is a failure, whichever surface reported it: every
		// run command exits non-zero on what it could not do (LUC-198). Both
		// surfaces have already named the jobs, so the error carries nothing more.
		if failedToStop(results) {
			return domain.ErrAborted
		}
		return nil
	}

	req := process.Request{Action: process.ActionStopAll}
	if !all {
		req.WorkDir = resolved.Dir
	}

	var resp process.Response
	stopErr := components.RunLoading(components.LoadingParams{
		Message: "Stopping jobs…",
		Animate: rules.IsHumanFormat(format),
		Work: func() error {
			var e error
			resp, e = client.Send(req)
			return e
		},
	})
	if stopErr != nil {
		return fmt.Errorf("stop all jobs: %w", stopErr)
	}
	if resp.Status == process.StatusError {
		return fmt.Errorf("stop all: %s", resp.Message)
	}

	if format == domain.OutputJSON {
		stopped := make([]domain.JobActionResult, 0, len(resp.Jobs))
		for _, job := range resp.Jobs {
			stopped = append(stopped, domain.JobActionResult{Name: job.Name, Status: domain.JobActionStopped})
		}
		return output.WriteJobResultsJSON(cmd.OutOrStdout(), stopped)
	}

	if len(resp.Jobs) == 0 {
		output.Frame(cmd.OutOrStdout(), func() {
			if all {
				output.Message(cmd.OutOrStdout(), "No jobs running.")
			} else {
				output.Message(cmd.OutOrStdout(), "No jobs running in this worktree.")
			}
		})
		return nil
	}
	output.FrameStart(cmd.OutOrStdout())
	for _, job := range resp.Jobs {
		output.Success(cmd.OutOrStdout(), fmt.Sprintf("%s stopped", job.Name))
	}
	output.FrameEnd(cmd.OutOrStdout())
	return nil
}

type ensureDownParams struct {
	SocketPath string
	Dir        string
	All        bool
}

// ensureDaemonForDown wakes a daemon when the index still holds jobs for what is
// being stopped. A daemon exits once no foreground job is left, so after a reboot
// — or simply half an hour later — nothing is listening while detached stacks are
// very much up, and `run down` is exactly the command that must reach them.
func ensureDaemonForDown(cmd *cobra.Command, params ensureDownParams) error {
	if process.IsDaemonRunning(params.SocketPath) {
		return nil
	}

	indexed := process.HasIndexedJobs(params.Dir)
	if params.All {
		indexed = process.HasAnyIndexedJob()
	}
	if !indexed {
		return nil
	}

	result, err := shared.LoadConfig(cmd, params.Dir)
	if err != nil {
		return err
	}
	return process.EnsureDaemon(process.DaemonParams{
		SocketPath: params.SocketPath,
		ProxyPort:  rules.ProxyPort(result.Config.Global),
	})
}

func failedToStop(results []domain.JobActionResult) bool {
	for _, result := range results {
		if result.Status == domain.JobActionError {
			return true
		}
	}
	return false
}
