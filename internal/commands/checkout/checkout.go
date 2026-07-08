// Package checkout implements the wtm checkout command: create a worktree from
// an existing GitHub pull request.
package checkout

import (
	"errors"
	"fmt"
	"os"
	"strconv"

	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/LucasPcq/wtm/internal/commands/shared"
	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/infra"
	"github.com/LucasPcq/wtm/internal/output"
	"github.com/LucasPcq/wtm/internal/rules"
	"github.com/LucasPcq/wtm/internal/service/branch"
	ghservice "github.com/LucasPcq/wtm/internal/service/github"
	"github.com/LucasPcq/wtm/internal/service/worktree"
	checkoutwizard "github.com/LucasPcq/wtm/internal/tui/checkout"
	"github.com/LucasPcq/wtm/internal/tui/components"
	"github.com/LucasPcq/wtm/internal/tui/envconfirm"
)

// NewCmd creates the wtm checkout command.
func NewCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   domain.CmdCheckout + " [number]",
		Short: "Create a worktree from an existing pull request",
		Long:  "Create a worktree from a pull request.\nWithout arguments, shows an interactive picker of open PRs.",
		Args:  cobra.MaximumNArgs(1),
		RunE:  runCheckout,
	}

	cmd.Flags().Bool(domain.FlagReview, false, "Show only PRs where you are requested as reviewer")
	cmd.Flags().Bool(domain.FlagMine, false, "Show only your PRs")
	cmd.Flags().String(domain.FlagFrom, "", "Parent branch for sync (defaults to the PR base branch)")
	cmd.Flags().String(domain.FlagEnvFrom, "", "Override env strategy (example, main, parent)")
	cmd.Flags().BoolP(domain.FlagYes, "y", false, "Skip all prompts; resolve every decision from flags and safe defaults (PR number required)")
	shared.AddOutputFlag(cmd)

	return cmd
}

func runCheckout(cmd *cobra.Command, args []string) error {
	dir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get working directory: %w", err)
	}

	result, err := shared.LoadConfig(cmd, dir)
	if err != nil {
		return err
	}

	format, _ := cmd.Flags().GetString(domain.FlagOutput)
	fromOverride, _ := cmd.Flags().GetString(domain.FlagFrom)
	envOverride, _ := cmd.Flags().GetString(domain.FlagEnvFrom)
	yes, _ := cmd.Flags().GetBool(domain.FlagYes)

	if format == domain.OutputJSON && !yes {
		return fmt.Errorf("--output json requires --%s (prompts cannot run in JSON mode)", domain.FlagYes)
	}

	// --yes runs prompt-free: parent defaults to the PR base branch, env to the
	// config strategy, and the env-fallback confirmation is skipped. It behaves like
	// a non-interactive run (a PR number is required) while still framing human output.
	opts := checkoutOptions{
		jsonMode:     format == domain.OutputJSON,
		interactive:  rules.IsHumanFormat(format) && term.IsTerminal(int(os.Stdin.Fd())) && !yes,
		fromOverride: fromOverride,
		envOverride:  envOverride,
	}

	if len(args) == 1 {
		number, parseErr := strconv.Atoi(args[0])
		if parseErr != nil || number <= 0 {
			return fmt.Errorf("invalid PR number %q", args[0])
		}
		return checkoutByNumber(cmd, result, number, opts)
	}

	return checkoutInteractive(cmd, result, opts)
}

// checkoutOptions carries the resolved invocation mode and overrides.
type checkoutOptions struct {
	jsonMode     bool
	interactive  bool
	fromOverride string
	envOverride  string
}

// checkoutByNumber handles `wtm checkout <number>`. The loading box owns its own
// spacing; the persistent top padding comes from the framed result.
func checkoutByNumber(cmd *cobra.Command, result shared.ConfigResult, number int, opts checkoutOptions) error {
	var p domain.PRInfo
	err := components.RunLoading(components.LoadingParams{
		Message: "Fetching PR…",
		Animate: !opts.jsonMode,
		Work: func() error {
			var e error
			p, e = ghservice.GetPRDetail(ghservice.GetPRDetailParams{
				ProjectDir: result.ProjectDir,
				Number:     number,
			})
			return e
		},
	})
	if err != nil {
		return fmt.Errorf("fetch PR: %w", err)
	}

	localBranches, err := infra.ListLocalBranches(infra.ListBranchesParams{ProjectDir: result.ProjectDir})
	if err != nil {
		return fmt.Errorf("list local branches: %w", err)
	}
	if err := rules.ValidatePRForCheckout(p, localBranches); err != nil {
		return err
	}
	parentBranches := parentBranchCandidates(result.ProjectDir)

	parent, env, aborted, wizardRan, err := resolveParentAndEnv(resolveParams{
		result:         result,
		pr:             p,
		parentBranches: parentBranches,
		opts:           opts,
	})
	if err != nil || aborted {
		return err
	}

	// When the wizard ran it already hosted the env-fallback confirmation; only
	// the no-wizard interactive path (both --from and --env-from given) still needs
	// the standalone confirm.
	return createFromPR(cmd, result, createFromPRParams{pr: p, parent: parent, env: env, jsonMode: opts.jsonMode, interactive: opts.interactive, envConfirmed: wizardRan})
}

// checkoutInteractive handles `wtm checkout` with no number: it renders the
// multi-step wizard instantly and streams open PRs in asynchronously.
func checkoutInteractive(cmd *cobra.Command, result shared.ConfigResult, opts checkoutOptions) error {
	if !opts.interactive {
		return fmt.Errorf("PR number required without an interactive terminal (or when --yes is set)")
	}

	localBranches, err := infra.ListLocalBranches(infra.ListBranchesParams{ProjectDir: result.ProjectDir})
	if err != nil {
		return fmt.Errorf("list local branches: %w", err)
	}
	parentBranches := parentBranchCandidates(result.ProjectDir)

	dir := result.ProjectDir
	review, _ := cmd.Flags().GetBool(domain.FlagReview)
	mine, _ := cmd.Flags().GetBool(domain.FlagMine)
	filter := rules.PRFilterFor(rules.PRFilterParams{Review: review, Mine: mine})

	res, err := checkoutwizard.RunWizard(checkoutwizard.WizardParams{
		ProjectDir:       dir,
		PRLoader:         func() ([]domain.PRInfo, domain.GHConnection) { return shared.LoadPRsFiltered(dir, filter) },
		WorktreeBranches: worktreeBranches(dir),
		ParentBranches:   parentBranches,
		ConfigStrategy:   result.Config.Project.Env.Strategy,
		IncludeParent:    opts.fromOverride == "",
		IncludeEnv:       opts.envOverride == "",
		FromOverride:     opts.fromOverride,
		EnvOverride:      opts.envOverride,
		EnvFallback:      envconfirm.Decider(dir, result.Config),
	})
	if err != nil {
		if errors.Is(err, domain.ErrUserAborted) {
			return nil
		}
		return err
	}

	p := res.PR
	if p.Number == 0 {
		return nil
	}
	if err := rules.ValidatePRForCheckout(p, localBranches); err != nil {
		return err
	}

	parent := rules.FirstNonEmpty(opts.fromOverride, res.FromBranch, p.BaseBranch)
	env := rules.FirstNonEmpty(opts.envOverride, res.EnvFromOverride)

	// The env-fallback confirmation ran inside the wizard.
	return createFromPR(cmd, result, createFromPRParams{pr: p, parent: parent, env: env, jsonMode: opts.jsonMode, interactive: opts.interactive, envConfirmed: true})
}

// resolveParams holds inputs for resolving parent/env for a known PR.
type resolveParams struct {
	result         shared.ConfigResult
	pr             domain.PRInfo
	parentBranches []domain.BranchCandidate
	opts           checkoutOptions
}

// resolveParentAndEnv determines the parent branch and env strategy for a known
// PR, running the wizard for whatever the flags left unset (interactive only).
// wizardRan reports whether the wizard ran — and thus already hosted the
// env-fallback confirmation.
func resolveParentAndEnv(params resolveParams) (parent, env string, aborted, wizardRan bool, err error) {
	parent = params.opts.fromOverride
	env = params.opts.envOverride

	needParent := parent == ""
	needEnv := env == ""

	if params.opts.interactive && (needParent || needEnv) {
		wizardRan = true
		pr := params.pr
		res, runErr := checkoutwizard.RunWizard(checkoutwizard.WizardParams{
			ProjectDir:     params.result.ProjectDir,
			Preselected:    &pr,
			ParentBranches: params.parentBranches,
			ConfigStrategy: params.result.Config.Project.Env.Strategy,
			IncludeParent:  needParent,
			IncludeEnv:     needEnv,
			FromOverride:   params.opts.fromOverride,
			EnvOverride:    params.opts.envOverride,
			EnvFallback:    envconfirm.Decider(params.result.ProjectDir, params.result.Config),
		})
		if runErr != nil {
			if errors.Is(runErr, domain.ErrUserAborted) {
				return "", "", true, wizardRan, nil
			}
			return "", "", false, wizardRan, runErr
		}
		if needParent {
			parent = res.FromBranch
		}
		if needEnv {
			env = res.EnvFromOverride
		}
	}

	if parent == "" {
		parent = params.pr.BaseBranch
	}
	return parent, env, false, wizardRan, nil
}

// createFromPRParams holds inputs for the final worktree creation step.
type createFromPRParams struct {
	pr          domain.PRInfo
	parent      string
	env         string
	jsonMode    bool
	interactive bool
	// envConfirmed is true when the env-fallback confirmation already ran inside
	// the wizard; only the no-wizard interactive path confirms it here.
	envConfirmed bool
}

func createFromPR(cmd *cobra.Command, result shared.ConfigResult, params createFromPRParams) error {
	p := params.pr

	if params.interactive && !params.envConfirmed {
		if show, cp := envconfirm.Decider(result.ProjectDir, result.Config)(params.parent, params.env); show {
			if confirmed, _ := components.RunStandaloneConfirm(components.NewConfirm(cp)); !confirmed {
				return nil
			}
		}
	}

	fetchErr := components.RunLoading(components.LoadingParams{
		Message: "Fetching branch from origin…",
		Animate: !params.jsonMode,
		Work: func() error {
			return infra.FetchBranch(infra.FetchBranchParams{
				ProjectDir: result.ProjectDir,
				Branch:     p.Branch,
			})
		},
	})
	if fetchErr != nil {
		return fetchErr
	}

	if !params.jsonMode {
		output.Loading(cmd.ErrOrStderr(), fmt.Sprintf("Creating worktree %s…", p.Branch))
	}
	createResult, err := worktree.Create(domain.CreateParams{
		ProjectDir:      result.ProjectDir,
		StateDir:        result.StateDir,
		Branch:          p.Branch,
		FromBranch:      domain.RemoteBranchPrefix + p.Branch,
		SourceBranch:    params.parent,
		Config:          result.Config,
		EnvFromOverride: params.env,
	})
	if err != nil {
		return err
	}

	if params.jsonMode {
		return output.WritePRCheckoutJSON(cmd.OutOrStdout(), output.PRCheckoutJSON{
			Number: p.Number,
			Branch: p.Branch,
			Path:   createResult.Path,
			Author: p.Author,
			URL:    p.URL,
			Draft:  p.Draft,
		})
	}

	output.Frame(cmd.OutOrStdout(), func() {
		output.Success(cmd.OutOrStdout(), fmt.Sprintf("Checked out PR #%d (%s) at %s", p.Number, p.Branch, createResult.Path))
		output.InfoLine(cmd.OutOrStdout(), "cd", fmt.Sprintf("wtm go %s", p.Branch))
	})
	return nil
}

func worktreeBranches(projectDir string) []string {
	wts, err := infra.ListWorktrees(infra.ListWorktreesParams{ProjectDir: projectDir})
	if err != nil {
		return nil
	}
	branches := make([]string, 0, len(wts))
	for _, wt := range wts {
		branches = append(branches, wt.Branch)
	}
	return branches
}

// parentBranchCandidates lists the local and origin remote-tracking branches,
// tagged with their divergence, as the parent-branch options for the checkout
// wizard. Best effort: a listing failure yields an empty set rather than an error.
func parentBranchCandidates(projectDir string) []domain.BranchCandidate {
	return branch.Candidates(branch.ListParams{ProjectDir: projectDir})
}
