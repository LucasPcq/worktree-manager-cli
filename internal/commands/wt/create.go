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

func runCreate(cmd *cobra.Command, args []string) error {
	var branchName string
	if len(args) > 0 {
		branchName = args[0]
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

	// A branch that already exists locally is checked out as-is: the source is then
	// only the sync parent recorded in metadata, never a start-point. Inspected up
	// front (when the name is known) so the non-interactive path stops requiring a
	// source it would not use.
	target := domain.BranchTarget{}
	if branchName != "" {
		target = branch.Target(branch.BranchParams{ProjectDir: result.ProjectDir, Branch: branchName})
	}

	// A branch checked out elsewhere is knowable right here — failing before the
	// wizard (worktree.Create's guard is the chokepoint for the other callers)
	// saves the user a full interactive run that could only ever end in refusal.
	if target.State == domain.BranchTargetCheckedOut && !ifNotExists {
		return fmt.Errorf("%w: "+domain.BranchCheckedOutElsewhereFmt,
			domain.ErrWorktreeExists, branchName, target.WorktreePath, branchName)
	}

	if interactive {
		// Every interactive run goes through the wizard — including --from / --env-from,
		// which just skip the steps they fix. The source-update select and the final
		// recap are always present, so the create never fires without a confirmation.
		wizResult, wizErr := runCreateWizard(createWizardParams{
			ProjectDir:    result.ProjectDir,
			Config:        result.Config,
			EnvFromFlag:   envFromFlag,
			IncludeBranch: branchName == "",
			BranchName:    branchName,
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
		if branchName == "" {
			branchName = wizResult.BranchName
		}
		fromBranch = wizResult.FromBranch
		if envFromFlag == "" {
			envOverride = wizResult.EnvFromOverride
		}
		// Only the accepted fast-forward is executed here (its recovery prompt is a
		// legitimate post-execution standalone). Its subject is whichever branch the
		// worktree starts from — the wizard resolved that, the command just runs it.
		if wizResult.FastForwardBranch != "" &&
			!executeFastForwardSource(result.ProjectDir, wizResult.FastForwardBranch) {
			return nil
		}
		// The branch name only exists now on the wizard path, so the target is
		// inspected here to drive the recap and the reuse reporting.
		target = branch.Target(branch.BranchParams{ProjectDir: result.ProjectDir, Branch: branchName})
	} else {
		// Non-interactive (--yes / --output json): create straight from the flags.
		if branchName == "" {
			return fmt.Errorf("branch name is required without the interactive wizard (pass it as an argument)")
		}
		// An existing branch was created outside wtm, so nothing says what it was
		// branched off. Defaulting the parent would record a guess that `sync` and
		// `tree` then treat as fact, so it is a required selection here — the picker
		// that would otherwise ask cannot run.
		if rules.ParentMustBeExplicit(target.State) && fromFlag == "" {
			return fmt.Errorf(domain.ParentRequiredFmt, branchName, domain.FlagFrom)
		}
		// A branch git creates starts from the source, which defaults to the
		// configured base branch — mirroring the picker's default.
		if rules.SourceIsStartPoint(target.State) {
			if fromBranch == "" {
				fromBranch = result.Config.Project.Worktrees.BaseBranch
			}
			if fromBranch == "" {
				return fmt.Errorf("no source branch: pass --%s (no base branch configured)", domain.FlagFrom)
			}
			if !rules.BranchCandidateExists(branchCandidates(result.ProjectDir), fromBranch) {
				return fmt.Errorf("%w: %s", domain.ErrBranchNotFound, fromBranch)
			}
		}
		// --ff updates whichever branch the worktree starts from: the source for a
		// new branch, the target itself when it is reused.
		if ffFlag {
			fastForwardSourceIfBehind(result.ProjectDir, ffSubject(ffSubjectParams{
				Target:     target,
				FromBranch: fromBranch,
				Branch:     branchName,
			}))
		}
	}

	// A reused branch is checked out as-is: the source is not a start-point, so it
	// is passed as the recorded sync parent instead (git would ignore it anyway).
	startPoint := fromBranch
	if !rules.SourceIsStartPoint(target.State) {
		startPoint = ""
	}

	human := rules.IsHumanFormat(format)

	// Phase 1 — the silent creation (git worktree add + env copy) under a spinner.
	// Hooks are held back (SkipHooks) so they can stream in their own phase below
	// without fighting the spinner for the terminal.
	var createResult domain.CreateResult
	err = components.RunLoading(components.LoadingParams{
		Message: fmt.Sprintf("Creating worktree %s…", branchName),
		Animate: human,
		Work: func() error {
			var e error
			createResult, e = worktree.Create(domain.CreateParams{
				ProjectDir:      result.ProjectDir,
				StateDir:        result.StateDir,
				Branch:          branchName,
				FromBranch:      startPoint,
				SourceBranch:    fromBranch,
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
			Branch:       branchName,
			FromBranch:   fromBranch,
			Hooks:        result.Config.Project.Hooks.OnCreate,
		}); hookErr != nil {
			return hookErr
		}
	}

	if format == domain.OutputJSON {
		return output.WriteWorktreeCreateJSON(cmd.OutOrStdout(), createResult)
	}

	// A reused branch's own divergence from origin (behind/diverged) is the one
	// thing "Created worktree x on existing branch" would otherwise leave out —
	// the wizard already surfaces it via the fast-forward step, but a --yes run
	// has no wizard, so the conclusion states it explicitly.
	var reusedNote shared.ReusedBranchNoteResult
	if createResult.ExistingBranch {
		reusedNote = shared.ReusedBranchNote(shared.ReusedBranchNoteParams{
			Branch: branchName,
			Ahead:  createResult.OriginAhead,
			Behind: createResult.OriginBehind,
		})
	}

	// Phase 3 — the recap, with a jump-in `wtm go` step.
	output.Frame(cmd.OutOrStdout(), func() {
		output.FormatCreateResult(cmd.OutOrStdout(), output.CreateResultParams{
			Branch:        branchName,
			AlreadyExists: createResult.AlreadyExists,
			From:          fromBranch,
			EnvStrategy:   string(createResult.Metadata.EnvStrategy),
			Path: createDisplayPath(displayPathParams{
				Config:     result.Config,
				ProjectDir: result.ProjectDir,
				Path:       createResult.Path,
			}),
			ExistingBranch:    createResult.ExistingBranch,
			ReusedNote:        reusedNote.Text,
			ReusedNoteWarning: reusedNote.Warning,
			GoCommand:         fmt.Sprintf(domain.GoCommandFmt, branchName),
		})
	})

	return nil
}

// ffSubjectParams holds inputs for ffSubject.
type ffSubjectParams struct {
	Target     domain.BranchTarget
	FromBranch string
	Branch     string
}

// ffSubject names the branch --ff updates: the source for a branch git is about
// to create, and the worktree's own branch when it already exists locally — that
// is the ref the worktree will actually check out, so it is the only one whose
// freshness matters. The non-interactive counterpart of the wizard's step subject.
func ffSubject(params ffSubjectParams) string {
	if rules.SourceIsStartPoint(params.Target.State) {
		return params.FromBranch
	}
	return params.Branch
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
	// The branch/source pair barely changes across one wizard's steps and recap
	// re-renders (back-navigation replays them), so branch.Target — ~3 git
	// subprocesses each — is cached per branch name for the run rather than
	// re-inspected on every Build/recap call.
	target := memoizedTarget(params.ProjectDir)

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
		Target:         target,
		SourceUpdate: func(up newpicker.SourceUpdateParams) newpicker.SourceUpdatePrompt {
			return sourceUpdatePrompt(sourceUpdatePromptParams{ProjectDir: params.ProjectDir, Target: target, Update: up})
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

// memoizedTarget wraps branch.Target in a per-branch-name cache, valid for the
// lifetime of a single wizard run (the branch a run is deciding on cannot change
// underneath it).
func memoizedTarget(projectDir string) func(string) domain.BranchTarget {
	cache := make(map[string]domain.BranchTarget)
	return func(b string) domain.BranchTarget {
		if t, ok := cache[b]; ok {
			return t
		}
		t := branch.Target(branch.BranchParams{ProjectDir: projectDir, Branch: b})
		cache[b] = t
		return t
	}
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
// worktree actually starts from into the reconciliation the wizard (and the
// standalone --from path) shows: a fast-forward offer for a behind-only branch
// (declining proceeds as-is) or a heads-up for a diverged one (declining cancels).
//
// The subject is the target branch when it already exists locally — its commits
// are what the worktree checks out, and the source is then only the recorded sync
// parent — and the source branch otherwise. Only one subject per run, so the step
// and the recap can never contradict each other.
func sourceUpdatePrompt(p sourceUpdatePromptParams) newpicker.SourceUpdatePrompt {
	subject := p.Update.Source
	if p.Update.Branch != "" && p.Target(p.Update.Branch).State == domain.BranchTargetExisting {
		subject = p.Update.Branch
	}
	if subject == "" {
		return newpicker.SourceUpdatePrompt{Branch: subject, SkipReason: "no source to check"}
	}
	if rules.IsRemoteBranch(subject) {
		return newpicker.SourceUpdatePrompt{Branch: subject, SkipReason: "source is a remote branch"}
	}
	state, ab := branch.Divergence(branch.BranchParams{ProjectDir: p.ProjectDir, Branch: subject})
	if rules.ShouldOfferFastForward(state) {
		return newpicker.SourceUpdatePrompt{
			Branch: subject,
			Show:   true,
			Params: components.NewConfirmParams{
				Title:       fmt.Sprintf(domain.SourceFastForwardPrompt, subject, ab.Behind),
				Description: domain.SourceFastForwardDescription,
				DefaultYes:  true,
			},
		}
	}
	if state == domain.DivergenceDiverged {
		return newpicker.SourceUpdatePrompt{
			Branch: subject,
			Show:   true,
			Params: components.NewConfirmParams{
				Title:      fmt.Sprintf(domain.SourceDivergedPrompt, subject, ab.Ahead, ab.Behind),
				Warning:    domain.SourceDivergedWarning,
				DefaultYes: true,
			},
			AbortOnDecline: true,
			SkipReason:     "source diverged from origin — see recap",
		}
	}
	return newpicker.SourceUpdatePrompt{Branch: subject, SkipReason: "source already up to date"}
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

// branchCandidates lists the local and remote-tracking branches offered as
// worktree start-points, with remotes whose name already exists locally dropped
// and each local branch tagged with its divergence from origin.
func branchCandidates(projectDir string) []domain.BranchCandidate {
	return branch.Candidates(branch.ListParams{ProjectDir: projectDir})
}
