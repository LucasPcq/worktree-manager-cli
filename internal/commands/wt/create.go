package wt

import (
	"errors"
	"fmt"
	"os"

	"github.com/spf13/cobra"

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
	cmd.Flags().String(domain.FlagEnvFrom, "", "Override env strategy (example, main, parent)")
	cmd.Flags().Bool(domain.FlagIfNotExists, false, "Succeed silently if the worktree already exists (idempotent)")
	shared.AddOutputFlag(cmd)

	return cmd
}

func runCreate(cmd *cobra.Command, args []string) error {
	var branch string
	if len(args) > 0 {
		branch = args[0]
	}
	fromFlag, _ := cmd.Flags().GetString(domain.FlagFrom)
	envFromFlag, _ := cmd.Flags().GetString(domain.FlagEnvFrom)
	ifNotExists, _ := cmd.Flags().GetBool(domain.FlagIfNotExists)

	dir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get working directory: %w", err)
	}

	result, err := shared.LoadConfig(cmd, dir)
	if err != nil {
		return err
	}

	format, _ := cmd.Flags().GetString(domain.FlagOutput)
	interactive := rules.IsHumanFormat(format)

	fromBranch := fromFlag
	envOverride := envFromFlag

	if fromFlag == "" {
		wizResult, wizErr := runCreateWizard(createWizardParams{
			ProjectDir:    result.ProjectDir,
			Config:        result.Config,
			IncludeBranch: branch == "",
		})
		if errors.Is(wizErr, domain.ErrUserAborted) {
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
		if interactive && !maybeFastForwardSource(result.ProjectDir, fromBranch) {
			return nil
		}
		if interactive && !shared.ConfirmEnvParentFallback(shared.EnvFallbackParams{
			ProjectDir:  result.ProjectDir,
			Source:      fromBranch,
			Config:      result.Config,
			EnvOverride: envOverride,
		}) {
			return nil
		}
	} else {
		if !rules.BranchCandidateExists(branchCandidates(result.ProjectDir), fromFlag) {
			return fmt.Errorf("%w: %s", domain.ErrBranchNotFound, fromFlag)
		}
	}

	// on_create hooks stream their own output, so only show a spinner for the
	// silent path (git worktree add + env copy) on the human-facing run.
	hasHooks := len(result.Config.Project.Hooks.OnCreate) > 0
	showSpinner := rules.IsHumanFormat(format) && !hasHooks

	var createResult domain.CreateResult
	err = components.RunLoading(components.LoadingParams{
		Message: "Creating worktree…",
		Animate: showSpinner,
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
			})
			return e
		},
	})
	if err != nil {
		return err
	}

	if format == domain.OutputJSON {
		return output.WriteWorktreeCreateJSON(cmd.OutOrStdout(), createResult)
	}

	output.Frame(cmd.OutOrStdout(), func() {
		if createResult.AlreadyExists {
			output.Success(cmd.OutOrStdout(), fmt.Sprintf("Worktree %s already exists at %s", branch, createResult.Path))
		} else {
			output.Success(cmd.OutOrStdout(), fmt.Sprintf("Created worktree %s at %s", branch, createResult.Path))
		}
	})

	return nil
}

type createWizardParams struct {
	ProjectDir    string
	Config        domain.Config
	IncludeBranch bool
}

func runCreateWizard(params createWizardParams) (newpicker.WizardResult, error) {
	return newpicker.RunWizard(newpicker.WizardParams{
		ProjectDir:     params.ProjectDir,
		Branches:       branchCandidates(params.ProjectDir),
		DefaultBranch:  params.Config.Project.Worktrees.BaseBranch,
		ConfigStrategy: params.Config.Project.Env.Strategy,
		IncludeBranch:  params.IncludeBranch,
	})
}

// maybeFastForwardSource reconciles a source branch that has drifted from origin
// before the worktree is created. A behind-only branch is offered a fast-forward
// (so the worktree starts up to date while the source stays local); a diverged
// branch — which cannot be fast-forwarded — is used as-is after an explicit
// heads-up about the reconciliation it will need later. It returns false only when
// the user cancels creation (a failed fast-forward, or declining the diverged
// warning) — there is no silent fallback to a stale base.
func maybeFastForwardSource(projectDir, source string) bool {
	if rules.IsRemoteBranch(source) {
		return true
	}
	state, ab := branch.Divergence(branch.BranchParams{ProjectDir: projectDir, Branch: source})
	if rules.ShouldOfferFastForward(state) {
		return fastForwardStaleSource(projectDir, source, ab.Behind)
	}
	if state == domain.DivergenceDiverged {
		return confirmDivergedProceed(source, ab)
	}
	return true
}

// fastForwardStaleSource prompts for and applies a fast-forward of a behind-only
// source branch. Returns false only if the fast-forward fails and the user then
// declines to create from the stale local branch.
func fastForwardStaleSource(projectDir, source string, behind int) bool {
	if !confirmFastForward(source, behind) {
		return true
	}

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
	return confirmProceedStale(source, behind, ffErr)
}

func confirmDivergedProceed(source string, ab domain.AheadBehind) bool {
	confirmed, _ := components.RunStandaloneConfirm(components.NewConfirm(components.NewConfirmParams{
		Title:      fmt.Sprintf("%s has diverged from origin (%d ahead, %d behind) — create the worktree from it anyway?", source, ab.Ahead, ab.Behind),
		Warning:    "It can't be fast-forwarded. The worktree starts from your local branch, missing commits that are on origin — you may have to rebase or resolve conflicts later.",
		DefaultYes: true,
	}))
	return confirmed
}

func confirmFastForward(source string, behind int) bool {
	confirmed, _ := components.RunStandaloneConfirm(components.NewConfirm(components.NewConfirmParams{
		Title:       fmt.Sprintf("%s is %d commit(s) behind origin — fast-forward it before creating?", source, behind),
		Description: "Updates your local branch to origin so the new worktree starts up to date. Skipped if its worktree has uncommitted changes.",
		DefaultYes:  true,
	}))
	return confirmed
}

func confirmProceedStale(source string, behind int, cause error) bool {
	confirmed, _ := components.RunStandaloneConfirm(components.NewConfirm(components.NewConfirmParams{
		Title:      fmt.Sprintf("Create the worktree from local %s anyway? (behind origin by %d)", source, behind),
		Warning:    fmt.Sprintf("Couldn't fast-forward: %v", cause),
		DefaultYes: false,
	}))
	return confirmed
}

// branchCandidates lists the local and remote-tracking branches offered as
// worktree start-points, with remotes whose name already exists locally dropped
// and each local branch tagged with its divergence from origin.
func branchCandidates(projectDir string) []domain.BranchCandidate {
	return branch.Candidates(branch.ListParams{ProjectDir: projectDir})
}
