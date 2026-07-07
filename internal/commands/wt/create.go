package wt

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/LucasPcq/wtm/internal/commands/shared"
	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/output"
	"github.com/LucasPcq/wtm/internal/rules"
	"github.com/LucasPcq/wtm/internal/service/branch"
	"github.com/LucasPcq/wtm/internal/service/worktree"
	"github.com/LucasPcq/wtm/internal/tui/components"
	newpicker "github.com/LucasPcq/wtm/internal/tui/newwt"
)

// newCreateCmd creates the wtm create subcommand.
func newCreateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   domain.CmdCreate + " [branch]",
		Short: "Create a new worktree",
		Long:  "Create a git worktree with env provisioning, metadata, and hooks.\nWithout arguments, prompts for the branch name interactively.",
		Args:  cobra.MaximumNArgs(1),
		RunE:  runCreate,
	}

	cmd.Flags().String(domain.FlagFrom, "", "Source branch (skips interactive picker)")
	cmd.Flags().Bool(domain.FlagFF, false, "Fast-forward the source branch to origin before creating (non-interactive; skipped when it has diverged)")
	cmd.Flags().String(domain.FlagEnvFrom, "", "Override env strategy (example, main, parent)")
	cmd.Flags().Bool(domain.FlagIfNotExists, false, "Succeed silently if the worktree already exists (idempotent)")
	cmd.Flags().BoolP(domain.FlagYes, "y", false, "Skip all prompts; resolve every decision from flags and safe defaults (branch name required; source defaults to the base branch)")
	shared.AddOutputFlag(cmd)

	return cmd
}

func runCreate(cmd *cobra.Command, args []string) error {
	var branch string
	if len(args) > 0 {
		branch = args[0]
	}
	fromFlag, _ := cmd.Flags().GetString(domain.FlagFrom)
	ffFlag, _ := cmd.Flags().GetBool(domain.FlagFF)
	envFromFlag, _ := cmd.Flags().GetString(domain.FlagEnvFrom)
	ifNotExists, _ := cmd.Flags().GetBool(domain.FlagIfNotExists)
	yes, _ := cmd.Flags().GetBool(domain.FlagYes)

	dir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get working directory: %w", err)
	}

	result, err := shared.LoadConfig(cmd, dir)
	if err != nil {
		return err
	}

	format, _ := cmd.Flags().GetString(domain.FlagOutput)

	if format == domain.OutputJSON && !yes {
		return fmt.Errorf("--output json requires --%s (prompts cannot run in JSON mode)", domain.FlagYes)
	}

	// The wizard needs a TTY and is skipped by --yes; a human-format run without a
	// terminal (piped/scripted) also falls back to the non-interactive path.
	interactive := rules.IsHumanFormat(format) && !yes && term.IsTerminal(int(os.Stdin.Fd()))

	fromBranch := fromFlag
	envOverride := envFromFlag

	// Validate an explicit --from up front, so both the interactive wizard and the
	// non-interactive path report a missing branch the same way.
	if fromFlag != "" && !rules.BranchCandidateExists(branchCandidates(result.ProjectDir), fromFlag) {
		return fmt.Errorf("%w: %s", domain.ErrBranchNotFound, fromFlag)
	}

	if interactive {
		// Every interactive run goes through the wizard — including --from / --env-from,
		// which just skip the steps they fix. The source-update select and the final
		// recap are always present, so the create never fires without a confirmation.
		wizResult, wizErr := runCreateWizard(createWizardParams{
			ProjectDir:    result.ProjectDir,
			Config:        result.Config,
			EnvFromFlag:   envFromFlag,
			IncludeBranch: branch == "",
			BranchName:    branch,
			Source:        fromFlag,
		})
		if errors.Is(wizErr, domain.ErrUserAborted) {
			output.Frame(cmd.OutOrStdout(), func() {
				output.Message(cmd.OutOrStdout(), "Aborted.")
			})
			return nil
		}
		if wizErr != nil {
			return wizErr
		}
		if branch == "" {
			branch = wizResult.BranchName
		}
		fromBranch = wizResult.FromBranch
		if envFromFlag == "" {
			envOverride = wizResult.EnvFromOverride
		}
		// Only the accepted fast-forward is executed here (its recovery prompt is a
		// legitimate post-execution standalone).
		if wizResult.FastForwardSource && !executeFastForwardSource(result.ProjectDir, fromBranch) {
			return nil
		}
	} else {
		// Non-interactive (--yes / --output json): create straight from the flags.
		// The source defaults to the configured base branch when --from is omitted,
		// mirroring the picker's default; --ff fast-forwards it first.
		if fromBranch == "" {
			fromBranch = result.Config.Project.Worktrees.BaseBranch
		}
		if fromBranch == "" {
			return fmt.Errorf("no source branch: pass --%s (no base branch configured)", domain.FlagFrom)
		}
		if !rules.BranchCandidateExists(branchCandidates(result.ProjectDir), fromBranch) {
			return fmt.Errorf("%w: %s", domain.ErrBranchNotFound, fromBranch)
		}
		if branch == "" {
			return fmt.Errorf("branch name is required without the interactive wizard (pass it as an argument)")
		}
		if ffFlag {
			fastForwardSourceIfBehind(result.ProjectDir, fromBranch)
		}
	}

	human := rules.IsHumanFormat(format)

	// Phase 1 — the silent creation (git worktree add + env copy) under a spinner.
	// Hooks are held back (SkipHooks) so they can stream in their own phase below
	// without fighting the spinner for the terminal.
	var createResult domain.CreateResult
	err = components.RunLoading(components.LoadingParams{
		Message: fmt.Sprintf("Creating worktree %s…", branch),
		Animate: human,
		Work: func() error {
			var e error
			createResult, e = worktree.Create(domain.CreateParams{
				ProjectDir:      result.ProjectDir,
				StateDir:        result.StateDir,
				Branch:          branch,
				FromBranch:      fromBranch,
				Config:          result.Config,
				EnvFromOverride: envOverride,
				IfNotExists:     ifNotExists,
				SkipHooks:       true,
			})
			return e
		},
	})
	if err != nil {
		return err
	}

	// Phase 2 — on_create hooks as a distinct, titled phase. Skipped when the
	// worktree already existed (its hooks already ran on first creation).
	if !createResult.AlreadyExists {
		if hookErr := shared.RunCreateHooksPhase(shared.CreateHooksPhaseParams{
			Cmd:          cmd,
			ShowHeader:   human,
			ProjectDir:   result.ProjectDir,
			WorktreePath: createResult.Path,
			Branch:       branch,
			FromBranch:   fromBranch,
			Hooks:        result.Config.Project.Hooks.OnCreate,
		}); hookErr != nil {
			return hookErr
		}
	}

	if format == domain.OutputJSON {
		return output.WriteWorktreeCreateJSON(cmd.OutOrStdout(), createResult)
	}

	// Phase 3 — the recap, with a jump-in `wtm go` step.
	output.Frame(cmd.OutOrStdout(), func() {
		output.FormatCreateResult(cmd.OutOrStdout(), output.CreateResultParams{
			Branch:        branch,
			AlreadyExists: createResult.AlreadyExists,
			From:          fromBranch,
			EnvStrategy:   string(createResult.Metadata.EnvStrategy),
			Path:          createDisplayPath(result.Config, createResult.Path),
			GoCommand:     fmt.Sprintf(domain.GoCommandFmt, branch),
		})
	})

	return nil
}

// createDisplayPath renders the new worktree as base_path/<name>, matching how
// worktrees are conceptually located, instead of a long absolute path.
func createDisplayPath(config domain.Config, path string) string {
	return filepath.Join(config.Project.Worktrees.BasePath, filepath.Base(path))
}

type createWizardParams struct {
	ProjectDir    string
	Config        domain.Config
	EnvFromFlag   string
	IncludeBranch bool
	// BranchName is the branch given as a positional arg (shown in the recap).
	BranchName string
	// Source (--from), when set, fixes the source branch and skips its picker.
	Source string
}

func runCreateWizard(params createWizardParams) (newpicker.WizardResult, error) {
	return newpicker.RunWizard(newpicker.WizardParams{
		ProjectDir:     params.ProjectDir,
		Branches:       branchCandidates(params.ProjectDir),
		DefaultBranch:  params.Config.Project.Worktrees.BaseBranch,
		ConfigStrategy: params.Config.Project.Env.Strategy,
		IncludeBranch:  params.IncludeBranch,
		BranchName:     params.BranchName,
		Source:         params.Source,
		IncludeEnv:     params.EnvFromFlag == "",
		EnvOverride:    params.EnvFromFlag,
		SourceUpdate: func(source string) newpicker.SourceUpdatePrompt {
			return sourceUpdatePrompt(params.ProjectDir, source)
		},
		EnvFallback: func(source, envStepValue string) (bool, components.NewConfirmParams) {
			override := params.EnvFromFlag
			if override == "" {
				override = envStepValue
			}
			return envFallbackPrompt(params.ProjectDir, params.Config, source, override)
		},
	})
}

// sourceUpdatePrompt classifies a source branch's divergence from origin into the
// reconciliation confirmation the wizard (and the standalone --from path) shows: a
// fast-forward offer for a behind-only branch (declining proceeds as-is) or a
// heads-up for a diverged branch (declining cancels). A remote or up-to-date
// source needs no confirmation. Shared so both paths phrase it identically.
func sourceUpdatePrompt(projectDir, source string) newpicker.SourceUpdatePrompt {
	if rules.IsRemoteBranch(source) {
		return newpicker.SourceUpdatePrompt{SkipReason: "source is a remote branch"}
	}
	state, ab := branch.Divergence(branch.BranchParams{ProjectDir: projectDir, Branch: source})
	if rules.ShouldOfferFastForward(state) {
		return newpicker.SourceUpdatePrompt{
			Show: true,
			Params: components.NewConfirmParams{
				Title:       fmt.Sprintf(domain.SourceFastForwardPrompt, source, ab.Behind),
				Description: domain.SourceFastForwardDescription,
				DefaultYes:  true,
			},
		}
	}
	if state == domain.DivergenceDiverged {
		return newpicker.SourceUpdatePrompt{
			Show: true,
			Params: components.NewConfirmParams{
				Title:      fmt.Sprintf(domain.SourceDivergedPrompt, source, ab.Ahead, ab.Behind),
				Warning:    domain.SourceDivergedWarning,
				DefaultYes: true,
			},
			AbortOnDecline: true,
			SkipReason:     "source diverged from origin — see recap",
		}
	}
	return newpicker.SourceUpdatePrompt{SkipReason: "source already up to date"}
}

// envFallbackPrompt decides the "env source" confirmation: whether the parent env
// strategy will fall back to copying .env from main, and the prompt to show.
func envFallbackPrompt(projectDir string, config domain.Config, source, override string) (bool, components.NewConfirmParams) {
	if !shared.EnvParentFallbackApplies(shared.EnvFallbackParams{
		ProjectDir:  projectDir,
		Source:      source,
		Config:      config,
		EnvOverride: override,
	}) {
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
		Message: fmt.Sprintf("Updating %s from origin…", source),
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

// fastForwardSourceIfBehind fast-forwards a behind-only source branch to origin
// without prompting — the non-interactive counterpart used on the --from path with
// --ff set. A diverged/dirty/remote source is left as-is (best effort), so any
// error is intentionally ignored.
func fastForwardSourceIfBehind(projectDir, source string) {
	_ = branch.FastForwardIfBehind(branch.BranchParams{ProjectDir: projectDir, Branch: source})
}

// maybeFastForwardSource reconciles a source branch passed via --from, where there
// is no wizard to host the confirmation — a single standalone prompt is the whole
// interaction (Esc cancels, which is correct). Returns false only when the user
// cancels creation (declining the diverged warning, or a failed fast-forward).
func maybeFastForwardSource(projectDir, source string) bool {
	prompt := sourceUpdatePrompt(projectDir, source)
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

// branchCandidates lists the local and remote-tracking branches offered as
// worktree start-points, with remotes whose name already exists locally dropped
// and each local branch tagged with its divergence from origin.
func branchCandidates(projectDir string) []domain.BranchCandidate {
	return branch.Candidates(branch.ListParams{ProjectDir: projectDir})
}
