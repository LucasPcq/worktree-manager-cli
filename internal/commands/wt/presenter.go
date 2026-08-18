package wt

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/LucasPcq/wtm/internal/commands/shared"
	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/flow"
	"github.com/LucasPcq/wtm/internal/output"
	"github.com/LucasPcq/wtm/internal/rules"
	"github.com/LucasPcq/wtm/internal/tui/components"
	"github.com/LucasPcq/wtm/internal/tui/flowui"
)

// cliPresenter renders a flow's phases on a command's streams. It is the CLI half
// of flow.Presenter: the flow decides what happens, this decides how it reads.
type cliPresenter struct {
	cmd    *cobra.Command
	format string
	// human reports that the output is meant for a person: progress is animated and
	// the hook phase gets its title. A machine format keeps both silent.
	human bool
}

// newPresenter builds the presenter for a command run.
func newPresenter(cmd *cobra.Command, format string) cliPresenter {
	return cliPresenter{cmd: cmd, format: format, human: rules.IsHumanFormat(format)}
}

// Stage runs one unit of work under a spinner.
func (p cliPresenter) Stage(params flow.StageParams) error {
	return components.RunLoading(components.LoadingParams{
		Message: params.Message,
		Animate: p.human,
		Work:    params.Work,
	})
}

// HookPhase titles the hook section and streams the hooks straight to stderr, so
// their output appears as it is produced.
func (p cliPresenter) HookPhase(params flow.HookPhaseParams) error {
	if p.human {
		output.HooksSection(p.cmd.ErrOrStderr(), params.Title)
	}
	return params.Run(p.cmd.ErrOrStderr())
}

// Notice frames a message that concludes the run.
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

// Status prints one line inside an ongoing phase, on stderr so a machine payload
// stays clean.
func (p cliPresenter) Status(notice flow.Notice) {
	switch notice.Kind {
	case flow.NoticeWarning:
		output.Warning(p.cmd.ErrOrStderr(), notice.Text)
	default:
		output.Success(p.cmd.ErrOrStderr(), notice.Text)
	}
}

// createPresenter renders `wtm create`.
type createPresenter struct {
	cliPresenter
	config shared.ConfigResult
}

// Created writes the machine payload, or the recap with its jump-in `wtm go` step.
func (p createPresenter) Created(outcome flow.CreateOutcome) error {
	if p.format == domain.OutputJSON {
		return output.WriteWorktreeCreateJSON(p.cmd.OutOrStdout(), outcome.Result)
	}

	// A reused branch's own divergence from origin is the one thing "Created worktree
	// x on existing branch" would otherwise leave out — the wizard surfaces it on the
	// fast-forward step, but a prompt-free run has no wizard, so the conclusion says
	// it explicitly.
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

// cleanPresenter renders `wtm clean`.
type cleanPresenter struct {
	cliPresenter
}

// Cleaned writes the machine payload, or the recap of what was removed and what
// happened to the children. Removing what is already gone is a success.
func (p cleanPresenter) Cleaned(outcome flow.CleanOutcome) error {
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

// flowContext hands the flow the resolved environment: it cannot load the config
// itself (that reads cobra flags and the environment).
func flowContext(config shared.ConfigResult) flow.Context {
	return flow.Context{
		ProjectDir: config.ProjectDir,
		StateDir:   config.StateDir,
		Config:     config.Config,
	}
}

// flowPrompterParams holds inputs for flowPrompter.
type flowPrompterParams struct {
	// Interactive is the prompt-capability gate: a human format, on a terminal, and
	// not bypassed by --yes.
	Interactive bool
	// Stderr renders the wizard on stderr, for a command whose stdout the shell
	// wrapper consumes.
	Stderr bool
}

// flowPrompter picks who answers the flow's questions: the wizard, or nobody —
// in which case every value comes from a flag or a documented safe default, and a
// required selection without one refuses the run.
func flowPrompter(params flowPrompterParams) flow.Prompter {
	if !params.Interactive {
		return flow.Unattended{}
	}
	return flowui.New(flowui.Params{Stderr: params.Stderr})
}
