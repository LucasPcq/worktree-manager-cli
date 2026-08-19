package wt

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/LucasPcq/wtm/internal/commands/shared"
	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/flow"
	cleanflow "github.com/LucasPcq/wtm/internal/flow/clean"
	createflow "github.com/LucasPcq/wtm/internal/flow/create"
	pruneflow "github.com/LucasPcq/wtm/internal/flow/prune"
	reparentflow "github.com/LucasPcq/wtm/internal/flow/reparent"
	syncflow "github.com/LucasPcq/wtm/internal/flow/sync"
	"github.com/LucasPcq/wtm/internal/output"
	"github.com/LucasPcq/wtm/internal/rules"
	"github.com/LucasPcq/wtm/internal/tui/components"
	"github.com/LucasPcq/wtm/internal/tui/flowui"
)

// cliPresenter is the CLI half of flow.Presenter: the flow decides what happens,
// this decides how it reads.
type cliPresenter struct {
	cmd    *cobra.Command
	format string
	// human means the output is meant for a person: progress is animated and the
	// hook phase gets its title.
	human bool
}

func newPresenter(cmd *cobra.Command, format string) cliPresenter {
	return cliPresenter{cmd: cmd, format: format, human: rules.IsHumanFormat(format)}
}

func (p cliPresenter) Stage(params flow.StageParams) error {
	return components.RunLoading(components.LoadingParams{
		Message: params.Message,
		Animate: p.human,
		Work:    params.Work,
	})
}

func (p cliPresenter) HookPhase(params flow.HookPhaseParams) error {
	if p.human {
		output.HooksSection(p.cmd.ErrOrStderr(), params.Title)
	}
	return params.Run(p.cmd.ErrOrStderr())
}

func (p cliPresenter) Notice(notice flow.Notice) {
	if notice.Kind == flow.NoticeWarning {
		output.Frame(p.cmd.ErrOrStderr(), func() {
			output.Warning(p.cmd.ErrOrStderr(), notice.Text)
		})
		return
	}
	output.Frame(p.cmd.OutOrStdout(), func() {
		output.Message(p.cmd.OutOrStdout(), notice.Text)
	})
}

func (p cliPresenter) Status(notice flow.Notice) {
	if len(notice.Lines) > 0 {
		output.Callout(p.cmd.ErrOrStderr(), notice.Text, notice.Lines)
		return
	}
	switch notice.Kind {
	case flow.NoticeWarning:
		output.Warning(p.cmd.ErrOrStderr(), notice.Text)
	default:
		output.Success(p.cmd.ErrOrStderr(), notice.Text)
	}
}

type createPresenter struct {
	cliPresenter
	config shared.ConfigResult
}

func (p createPresenter) Created(outcome createflow.Outcome) error {
	if p.format == domain.OutputJSON {
		return output.WriteWorktreeCreateJSON(p.cmd.OutOrStdout(), outcome.Result)
	}

	// A reused branch's divergence from origin is the one thing "Created worktree x
	// on existing branch" would leave out, and a prompt-free run has no wizard to
	// have shown it.
	var reusedNote shared.ReusedBranchNoteResult
	if outcome.Result.ExistingBranch {
		reusedNote = shared.ReusedBranchNote(shared.ReusedBranchNoteParams{
			Branch: outcome.Branch,
			Ahead:  outcome.Result.OriginAhead,
			Behind: outcome.Result.OriginBehind,
		})
	}

	output.Frame(p.cmd.OutOrStdout(), func() {
		output.FormatCreateResult(p.cmd.OutOrStdout(), output.CreateResultParams{
			Branch:        outcome.Branch,
			AlreadyExists: outcome.Result.AlreadyExists,
			From:          outcome.FromBranch,
			EnvStrategy:   string(outcome.Result.Metadata.EnvStrategy),
			Path: createDisplayPath(displayPathParams{
				Config:     p.config.Config,
				ProjectDir: p.config.ProjectDir,
				Path:       outcome.Result.Path,
			}),
			ExistingBranch:    outcome.Result.ExistingBranch,
			ReusedNote:        reusedNote.Text,
			ReusedNoteWarning: reusedNote.Warning,
			GoCommand:         fmt.Sprintf(domain.GoCommandFmt, outcome.Branch),
		})
	})
	return nil
}

type cleanPresenter struct {
	cliPresenter
}

func (p cleanPresenter) Cleaned(outcome cleanflow.Outcome) error {
	if outcome.AlreadyAbsent {
		if p.format == domain.OutputJSON {
			return output.WriteWorktreeCleanJSON(p.cmd.OutOrStdout(), output.WriteWorktreeCleanJSONParams{
				Branch:        outcome.Branch,
				AlreadyAbsent: true,
			})
		}
		output.Frame(p.cmd.OutOrStdout(), func() {
			output.Message(p.cmd.OutOrStdout(), fmt.Sprintf(domain.CleanAlreadyAbsentFmt, outcome.Branch))
		})
		return nil
	}

	if p.format == domain.OutputJSON {
		return output.WriteWorktreeCleanJSON(p.cmd.OutOrStdout(), output.WriteWorktreeCleanJSONParams{
			Branch:           outcome.Branch,
			Path:             outcome.Path,
			Reparented:       outcome.Reparented,
			OrphanedChildren: outcome.OrphanedChildren,
		})
	}

	output.Frame(p.cmd.OutOrStdout(), func() {
		output.Success(p.cmd.OutOrStdout(), fmt.Sprintf(domain.CleanedFmt, outcome.Branch))
		for _, child := range outcome.Reparented {
			output.Success(p.cmd.OutOrStdout(), fmt.Sprintf(domain.CleanReparentedFmt, child.Branch, child.NewParent))
		}
		for _, child := range outcome.OrphanedChildren {
			output.Warning(p.cmd.OutOrStdout(), fmt.Sprintf(domain.CleanStillOrphanedFmt, child.Branch, child.OldParent))
		}
	})
	return nil
}

type prunePresenter struct {
	cliPresenter
}

// Pruned renders the three shapes a prune run concludes in: nothing matched, a
// dry-run preview, or a result. The frame goes on exactly once per branch, and
// never in JSON.
func (p prunePresenter) Pruned(outcome pruneflow.Outcome) error {
	if outcome.Empty {
		if p.format == domain.OutputJSON {
			return output.WritePruneResultJSON(p.cmd.OutOrStdout(), domain.PruneResult{})
		}
		output.Frame(p.cmd.OutOrStdout(), func() {
			output.Message(p.cmd.OutOrStdout(), domain.PruneNothingToPrune)
		})
		return nil
	}

	if p.format == domain.OutputJSON {
		return output.WritePruneResultJSON(p.cmd.OutOrStdout(), outcome.Result)
	}

	if outcome.Result.DryRun {
		output.Frame(p.cmd.OutOrStdout(), func() {
			output.FormatPrunePlan(p.cmd.OutOrStdout(), outcome.Plan)
		})
		return nil
	}

	output.Frame(p.cmd.OutOrStdout(), func() {
		output.FormatPruneResult(p.cmd.OutOrStdout(), outcome.Result)
	})
	return nil
}

type syncPresenter struct {
	cliPresenter
}

// Planned prints the cascade a run that could not ask never saw in a recap. It
// opens the frame on stderr, where the plan has always been written.
func (p syncPresenter) Planned(plan domain.SyncPlan) {
	if !p.human {
		return
	}
	output.FrameStart(p.cmd.ErrOrStderr())
	output.FormatSyncPlan(p.cmd.ErrOrStderr(), plan)
}

// Rebased is the recap the user reads BEFORE being asked to push. Its single
// leading blank separates the plan/spinner section (stderr) from the recap.
func (p syncPresenter) Rebased(result domain.SyncResult) {
	if !p.human {
		return
	}
	output.Blank(p.cmd.OutOrStdout())
	output.FormatSyncResult(p.cmd.OutOrStdout(), result)
}

func (p syncPresenter) Synced(outcome syncflow.Outcome) error {
	if p.format == domain.OutputJSON {
		return output.WriteSyncResultJSON(p.cmd.OutOrStdout(), outcome.Result)
	}
	if outcome.Empty {
		output.Frame(p.cmd.OutOrStdout(), func() {
			output.Message(p.cmd.OutOrStdout(), domain.SyncNothingToSync)
		})
		return nil
	}
	output.FormatSyncPushSummary(p.cmd.OutOrStdout(), outcome.Result.Steps)
	output.FrameEnd(p.cmd.OutOrStdout())
	return nil
}

type reparentPresenter struct {
	cliPresenter
}

func (p reparentPresenter) Reparented(outcome reparentflow.Outcome) error {
	if p.format == domain.OutputJSON {
		return output.WriteReparentJSON(p.cmd.OutOrStdout(), outcome.Results)
	}

	output.Frame(p.cmd.OutOrStdout(), func() {
		for _, result := range outcome.Results {
			output.Success(p.cmd.OutOrStdout(), fmt.Sprintf(domain.ReparentedFmt, result.Branch, result.OldParent, result.NewParent))
		}
		output.Message(p.cmd.OutOrStdout(), reparentSyncHint(outcome.Results))
	})
	return nil
}

// reparentSyncHint tells the user how to apply the recorded change. A single
// worktree names it in the suggested command; several point at a bare `wtm sync`.
func reparentSyncHint(results []domain.ReparentResult) string {
	if len(results) == 1 {
		return fmt.Sprintf(domain.ReparentSyncHintFmt, results[0].Branch)
	}
	return domain.ReparentSyncHintBare
}

// flowContext: the flow cannot load the config itself, which reads cobra flags.
func flowContext(config shared.ConfigResult) flow.Context {
	return flow.Context{
		ProjectDir: config.ProjectDir,
		StateDir:   config.StateDir,
		Config:     config.Config,
	}
}

type flowPrompterParams struct {
	// Interactive is the prompt-capability gate: a human format, on a terminal, and
	// not bypassed by --yes.
	Interactive bool
	Stderr      bool
}

func flowPrompter(params flowPrompterParams) flow.Prompter {
	if !params.Interactive {
		return flow.Unattended{}
	}
	return flowui.New(flowui.Params{Stderr: params.Stderr})
}
