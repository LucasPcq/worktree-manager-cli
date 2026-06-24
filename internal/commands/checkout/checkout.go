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
	ghservice "github.com/LucasPcq/wtm/internal/service/github"
	"github.com/LucasPcq/wtm/internal/service/worktree"
	checkoutwizard "github.com/LucasPcq/wtm/internal/tui/checkout"
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

	opts := checkoutOptions{
		jsonMode:     format == domain.OutputJSON,
		interactive:  format != domain.OutputJSON && term.IsTerminal(int(os.Stdin.Fd())),
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

// checkoutByNumber handles `wtm checkout <number>`. The "Fetching PR…" spinner
// gives both network feedback and the top padding before any wizard step.
func checkoutByNumber(cmd *cobra.Command, result shared.ConfigResult, number int, opts checkoutOptions) error {
	stop := func() {}
	if !opts.jsonMode {
		stop = shared.StartSpinner(cmd.ErrOrStderr(), "Fetching PR…")
	}
	p, err := ghservice.GetPRDetail(ghservice.GetPRDetailParams{
		ProjectDir: result.ProjectDir,
		Number:     number,
	})
	stop()
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

	parent, env, aborted, err := resolveParentAndEnv(resolveParams{
		result:        result,
		pr:            p,
		localBranches: localBranches,
		opts:          opts,
	})
	if err != nil || aborted {
		return err
	}

	return createFromPR(cmd, result, createFromPRParams{pr: p, parent: parent, env: env, jsonMode: opts.jsonMode})
}

// checkoutInteractive handles `wtm checkout` with no number: it renders the
// multi-step wizard instantly and streams open PRs in asynchronously.
func checkoutInteractive(cmd *cobra.Command, result shared.ConfigResult, opts checkoutOptions) error {
	if !opts.interactive {
		return fmt.Errorf("PR number required in non-interactive mode")
	}

	localBranches, err := infra.ListLocalBranches(infra.ListBranchesParams{ProjectDir: result.ProjectDir})
	if err != nil {
		return fmt.Errorf("list local branches: %w", err)
	}

	dir := result.ProjectDir
	review, _ := cmd.Flags().GetBool(domain.FlagReview)
	mine, _ := cmd.Flags().GetBool(domain.FlagMine)
	filter := rules.PRFilterFor(rules.PRFilterParams{Review: review, Mine: mine})

	// Top padding between the prompt and the wizard (the by-number path gets this
	// from its spinner instead).
	output.Blank(cmd.OutOrStdout())

	res, err := checkoutwizard.RunWizard(checkoutwizard.WizardParams{
		PRLoader:         func() ([]domain.PRInfo, domain.GHConnection) { return shared.LoadPRsFiltered(dir, filter) },
		WorktreeBranches: worktreeBranches(dir),
		LocalBranches:    localBranches,
		ConfigStrategy:   result.Config.Project.Env.Strategy,
		IncludeParent:    opts.fromOverride == "",
		IncludeEnv:       opts.envOverride == "",
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

	return createFromPR(cmd, result, createFromPRParams{pr: p, parent: parent, env: env, jsonMode: opts.jsonMode})
}

// resolveParams holds inputs for resolving parent/env for a known PR.
type resolveParams struct {
	result        shared.ConfigResult
	pr            domain.PRInfo
	localBranches []string
	opts          checkoutOptions
}

// resolveParentAndEnv determines the parent branch and env strategy for a known
// PR, running the wizard for whatever the flags left unset (interactive only).
func resolveParentAndEnv(params resolveParams) (parent, env string, aborted bool, err error) {
	parent = params.opts.fromOverride
	env = params.opts.envOverride

	needParent := parent == ""
	needEnv := env == ""

	if params.opts.interactive && (needParent || needEnv) {
		pr := params.pr
		res, runErr := checkoutwizard.RunWizard(checkoutwizard.WizardParams{
			Preselected:    &pr,
			LocalBranches:  params.localBranches,
			ConfigStrategy: params.result.Config.Project.Env.Strategy,
			IncludeParent:  needParent,
			IncludeEnv:     needEnv,
		})
		if runErr != nil {
			if errors.Is(runErr, domain.ErrUserAborted) {
				return "", "", true, nil
			}
			return "", "", false, runErr
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
	return parent, env, false, nil
}

// createFromPRParams holds inputs for the final worktree creation step.
type createFromPRParams struct {
	pr       domain.PRInfo
	parent   string
	env      string
	jsonMode bool
}

func createFromPR(cmd *cobra.Command, result shared.ConfigResult, params createFromPRParams) error {
	p := params.pr

	stopFetch := func() {}
	if !params.jsonMode {
		stopFetch = shared.StartSpinner(cmd.ErrOrStderr(), "Fetching branch from origin…")
	}
	fetchErr := infra.FetchBranch(infra.FetchBranchParams{
		ProjectDir: result.ProjectDir,
		Branch:     p.Branch,
	})
	stopFetch()
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
		FromBranch:      "origin/" + p.Branch,
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
		})
	}

	output.Blank(cmd.OutOrStdout())
	output.Success(cmd.OutOrStdout(), fmt.Sprintf("Checked out PR #%d (%s) at %s", p.Number, p.Branch, createResult.Path))
	output.InfoLine(cmd.OutOrStdout(), "cd", fmt.Sprintf("wtm go %s", p.Branch))
	output.Blank(cmd.OutOrStdout())
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
