package wt

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/LucasPcq/wtm/internal/commands/shared"
	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/flow"
	"github.com/LucasPcq/wtm/internal/rules"
	"github.com/LucasPcq/wtm/internal/service/branch"
	"github.com/LucasPcq/wtm/internal/tui/components"
	newpicker "github.com/LucasPcq/wtm/internal/tui/newwt"
)

// newCreateCmd creates the wtm create subcommand.
func newCreateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   domain.CmdCreate + " [branch]",
		Short: "Create a new worktree",
		Long: "Create a git worktree with env provisioning, metadata, and hooks.\n" +
			"A branch that already exists locally is checked out as-is, keeping its commits.\n" +
			"Its parent can't be inferred, so --from then names the branch recorded for\n" +
			"`wtm sync` — asked in the wizard, required without it.\n" +
			"Without arguments, prompts for the branch name interactively.",
		Args: cobra.MaximumNArgs(1),
		RunE: runCreate,
	}

	// No backquotes in this usage string: cobra reads backquoted text as the flag's
	// value placeholder, which would render as "--from wtm sync" instead of string.
	cmd.Flags().String(domain.FlagFrom, "", "Source branch to start from — or, when the branch already exists locally, the parent to record for wtm sync (required there without the wizard)")
	cmd.Flags().Bool(domain.FlagFF, false, "Fast-forward to origin before creating — the source branch, or the branch itself when it already exists locally (non-interactive; skipped when it has diverged)")
	cmd.Flags().String(domain.FlagEnvFrom, "", "Override env strategy (example, main, parent)")
	cmd.Flags().Bool(domain.FlagIfNotExists, false, "Succeed silently if the worktree already exists (idempotent)")
	cmd.Flags().BoolP(domain.FlagYes, "y", false, "Skip all prompts; resolve every decision from flags and safe defaults (branch name required; source defaults to the base branch for a new branch, and --from is required for one that already exists)")
	shared.AddOutputFlag(cmd)

	return cmd
}

// runCreate wires the flags into the create flow: it decides who may be asked and
// where the output goes, and owns no part of the déroulé itself (internal/flow).
func runCreate(cmd *cobra.Command, args []string) error {
	branchName := ""
	if len(args) > 0 {
		branchName = args[0]
	}
	fromFlag, _ := cmd.Flags().GetString(domain.FlagFrom)
	ffFlag, _ := cmd.Flags().GetBool(domain.FlagFF)
	envFromFlag, _ := cmd.Flags().GetString(domain.FlagEnvFrom)
	ifNotExists, _ := cmd.Flags().GetBool(domain.FlagIfNotExists)
	yes, _ := cmd.Flags().GetBool(domain.FlagYes)
	format, _ := cmd.Flags().GetString(domain.FlagOutput)

	if format == domain.OutputJSON && !yes {
		return fmt.Errorf("--output json requires --%s (prompts cannot run in JSON mode)", domain.FlagYes)
	}

	dir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get working directory: %w", err)
	}

	config, err := shared.LoadConfig(cmd, dir)
	if err != nil {
		return err
	}

	// The wizard needs a TTY and is skipped by --yes; a human-format run without a
	// terminal (piped/scripted) also takes the prompt-free path.
	interactive := rules.IsHumanFormat(format) && !yes && term.IsTerminal(int(os.Stdin.Fd()))

	presenter := newPresenter(cmd, format)
	_, err = flow.Create(flow.CreateParams{
		Context: flowContext(config),
		Request: flow.CreateRequest{
			Branch:      branchName,
			From:        fromFlag,
			EnvFrom:     envFromFlag,
			FastForward: ffFlag,
			IfNotExists: ifNotExists,
		},
		Prompter:  flowPrompter(flowPrompterParams{Interactive: interactive}),
		Presenter: createPresenter{cliPresenter: presenter, config: config},
	})
	return err
}

// displayPathParams holds inputs for createDisplayPath.
type displayPathParams struct {
	Config     domain.Config
	ProjectDir string
	Path       string
}

// createDisplayPath renders the new worktree as base_path/<name>, matching how
// worktrees are conceptually located, instead of a long absolute path. A worktree
// that already existed may sit elsewhere (adopted, or holding the branch outside
// base_path), and recomposing it there would name a directory that does not exist
// — those keep their real path.
func createDisplayPath(params displayPathParams) string {
	base := params.Config.Project.Worktrees.BasePath
	expected := filepath.Join(params.ProjectDir, base, filepath.Base(params.Path))
	if filepath.Clean(expected) != filepath.Clean(params.Path) {
		return params.Path
	}
	return filepath.Join(base, filepath.Base(params.Path))
}

// The helpers below are the create decisions `wtm extract` still reaches for: it
// embeds create's wizard as a sub-flow of its own combined recap, in Bubbletea
// terms, so it cannot yet call internal/flow. Each one is a thin adapter over the
// flow decision — the logic lives in one place — and they go away with extract's
// own migration (LUC-173, lot 4).

// memoizedTarget caches branch classification for the lifetime of one run.
func memoizedTarget(projectDir string) func(string) domain.BranchTarget {
	return flow.MemoizedTarget(projectDir)
}

// branchCandidates lists the branches offered as worktree start-points.
func branchCandidates(projectDir string) []domain.BranchCandidate {
	return flow.BranchCandidates(projectDir)
}

// ffSubjectParams holds inputs for ffSubject.
type ffSubjectParams struct {
	Target     domain.BranchTarget
	FromBranch string
	Branch     string
}

// ffSubject names the branch a fast-forward updates.
func ffSubject(params ffSubjectParams) string {
	return flow.FastForwardSubject(flow.FastForwardSubjectParams{
		Target:     params.Target,
		FromBranch: params.FromBranch,
		Branch:     params.Branch,
	})
}

// sourceUpdatePromptParams holds inputs for sourceUpdatePrompt.
type sourceUpdatePromptParams struct {
	ProjectDir string
	// Target classifies a branch name; callers with a per-run cache (the wizard)
	// pass it in rather than hitting git on every re-render.
	Target func(string) domain.BranchTarget
	Update newpicker.SourceUpdateParams
}

// sourceUpdatePrompt classifies the divergence from origin of whichever branch the
// worktree starts from, in the terms the embedded create wizard speaks.
func sourceUpdatePrompt(p sourceUpdatePromptParams) newpicker.SourceUpdatePrompt {
	prompt := flow.SourceUpdate(flow.SourceUpdateParams{
		ProjectDir: p.ProjectDir,
		Target:     p.Target,
		Branch:     p.Update.Branch,
		Source:     p.Update.Source,
	})
	return newpicker.SourceUpdatePrompt{
		Branch: prompt.Branch,
		Show:   prompt.Show,
		Params: components.NewConfirmParams{
			Title:       prompt.Title,
			Description: prompt.Description,
			Warning:     prompt.Warning,
			DefaultYes:  true,
		},
		AbortOnDecline: prompt.AbortOnDecline,
		SkipReason:     prompt.SkipReason,
	}
}

// envFallbackPrompt decides the "env source" confirmation: whether the parent env
// strategy will fall back to copying .env from main, and the prompt to show.
func envFallbackPrompt(projectDir string, config domain.Config, source, override string) (bool, components.NewConfirmParams) {
	show, _ := flow.EnvParentFallback(flow.EnvFallbackParams{
		ProjectDir:  projectDir,
		Source:      source,
		Config:      config,
		EnvOverride: override,
	})
	if !show {
		return false, components.NewConfirmParams{}
	}
	return true, shared.EnvParentFallbackConfirm(source)
}

// executeFastForwardSource fast-forwards an accepted behind-only source branch.
// The fast-forward failing is a runtime outcome, so its recovery prompt ("create
// from the stale branch anyway?") is a legitimate post-execution standalone.
// Returns false only when that recovery is declined.
func executeFastForwardSource(projectDir, source string) bool {
	ffErr := components.RunLoading(components.LoadingParams{
		Message: fmt.Sprintf(domain.SourceFastForwardLoadingFmt, source),
		Animate: true,
		Work: func() error {
			return branch.FastForwardToOrigin(branch.BranchParams{ProjectDir: projectDir, Branch: source})
		},
	})
	if ffErr == nil {
		return true
	}
	_, ab := branch.Divergence(branch.BranchParams{ProjectDir: projectDir, Branch: source})
	confirmed, _ := components.RunStandaloneConfirm(components.NewConfirm(components.NewConfirmParams{
		Title:      fmt.Sprintf(domain.SourceProceedStalePrompt, source, ab.Behind),
		Warning:    fmt.Sprintf(domain.SourceProceedStaleWarning, ffErr),
		DefaultYes: false,
	}))
	return confirmed
}

// maybeFastForwardSource reconciles a source branch passed via --from, where there
// is no wizard to host the confirmation — a single standalone prompt is the whole
// interaction (Esc cancels, which is correct). Returns false only when the user
// cancels creation (declining the diverged warning, or a failed fast-forward).
func maybeFastForwardSource(projectDir, source string) bool {
	prompt := sourceUpdatePrompt(sourceUpdatePromptParams{
		ProjectDir: projectDir,
		Target:     memoizedTarget(projectDir),
		Update:     newpicker.SourceUpdateParams{Source: source},
	})
	if !prompt.Show {
		return true
	}
	confirmed, _ := components.RunStandaloneConfirm(components.NewConfirm(prompt.Params))
	if prompt.AbortOnDecline {
		return confirmed
	}
	if !confirmed {
		return true
	}
	return executeFastForwardSource(projectDir, source)
}
