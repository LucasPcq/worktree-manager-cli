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
	"github.com/LucasPcq/wtm/internal/infra"
	"github.com/LucasPcq/wtm/internal/output"
	"github.com/LucasPcq/wtm/internal/rules"
	"github.com/LucasPcq/wtm/internal/service/process"
	"github.com/LucasPcq/wtm/internal/service/worktree"
	cleanui "github.com/LucasPcq/wtm/internal/tui/clean"
	"github.com/LucasPcq/wtm/internal/tui/components"
)

// newCleanCmd creates the wtm clean subcommand.
func newCleanCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   domain.CmdClean + " [branch]",
		Short: "Remove a worktree and its local branch",
		Long:  "Remove a git worktree and delete the local branch. The remote branch is never touched.\nWithout arguments, shows an interactive picker.",
		Args:  cobra.MaximumNArgs(1),
		RunE:  runClean,
	}

	cmd.Flags().Bool(domain.FlagForce, false, "Lift safety refusals (dirty/unpushed/open-PR); still asks to confirm unless --yes")
	cmd.Flags().BoolP(domain.FlagYes, "y", false, "Skip all prompts; resolve every decision from flags and safe defaults (keeps safety checks unless --force)")
	cmd.Flags().Bool(domain.FlagReparentChildren, false, "Reparent orphaned child worktrees onto the grandparent (no prompt)")
	shared.AddOutputFlag(cmd)

	return cmd
}

func runClean(cmd *cobra.Command, args []string) error {
	force, _ := cmd.Flags().GetBool(domain.FlagForce)
	yes, _ := cmd.Flags().GetBool(domain.FlagYes)
	reparentFlag, _ := cmd.Flags().GetBool(domain.FlagReparentChildren)
	format, _ := cmd.Flags().GetString(domain.FlagOutput)

	if format == domain.OutputJSON && !yes {
		return domain.ErrCleanJSONNeedsYes
	}

	dir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get working directory: %w", err)
	}

	result, err := shared.LoadConfig(cmd, dir)
	if err != nil {
		return err
	}

	interactive := rules.IsHumanFormat(format)
	// canPrompt is the true prompt-capability gate: a human format on a real
	// terminal AND not --yes. Every interactive picker/prompt keys off it; --yes and
	// a piped/non-TTY run both take the prompt-free path (which errors when the branch
	// is missing rather than launching a wizard against a non-TTY stdin).
	canPrompt := interactive && term.IsTerminal(int(os.Stdin.Fd())) && !yes
	baseBranch := resolveBase("", result)

	// The full interactive path runs one wizard: picker → delete → reparent, with
	// the safety check loaded async behind the spinner. --force is the safety axis,
	// not a confirmation bypass: it still runs the wizard (which confirms) with the
	// refusals lifted. Only --yes (or JSON) takes the lighter, prompt-free path.
	if canPrompt {
		return runCleanWizard(cmd, cleanWizardParams{
			result:       result,
			args:         args,
			format:       format,
			baseBranch:   baseBranch,
			force:        force,
			reparentFlag: reparentFlag,
		})
	}

	// Prompt-free path (--yes or JSON): the worktree must be named explicitly — no
	// picker fallback. A missing branch errors naming the flag.
	if len(args) == 0 {
		return domain.ErrCleanBranchRequired
	}
	branch := args[0]

	cleanParams := cleanParamsFor(cleanParamsInput{result: result, baseBranch: baseBranch, branch: branch, force: force})
	reparentPlan := worktree.PlanCleanReparent(cleanParams)

	if yes && !force {
		if err := ensureSafeToClean(cleanParams, interactive); err != nil {
			return err
		}
	}

	// canPrompt is false here, so decideReparent resolves the reparent decision from
	// the flag or the safe default (orphan) — never a prompt under --yes.
	applyReparent, aborted := decideReparent(cmd, decideReparentParams{
		Plan:        reparentPlan,
		Flag:        reparentFlag,
		Interactive: canPrompt,
	})
	if aborted {
		output.Frame(cmd.OutOrStdout(), func() {
			output.Message(cmd.OutOrStdout(), "Aborted.")
		})
		return nil
	}
	return doClean(cmd, doCleanParams{
		Params:        cleanParams,
		Format:        format,
		ReparentPlan:  reparentPlan,
		ApplyReparent: applyReparent,
		CanPrompt:     false,
	})
}

// cleanWizardParams holds inputs for runCleanWizard.
type cleanWizardParams struct {
	result       shared.ConfigResult
	args         []string
	format       string
	baseBranch   string
	force        bool
	reparentFlag bool
}

// runCleanWizard drives the unified interactive clean: the worktree picker (unless
// a branch was given), the delete confirmation, and the reparent confirmation, all
// in one wizard so Esc steps back instead of aborting. The safety check for a
// picked worktree runs asynchronously inside the wizard (it queries the PR state);
// for an explicit branch it is pre-flighted here so an absent/parent worktree is
// reported without prompting.
func runCleanWizard(cmd *cobra.Command, p cleanWizardParams) error {
	argBranch := ""
	if len(p.args) > 0 {
		argBranch = p.args[0]
	}

	var preCheck *domain.CleanCheckResult
	if argBranch != "" {
		check, handled, err := precheckClean(cmd, precheckParams{result: p.result, baseBranch: p.baseBranch, branch: argBranch})
		if err != nil || handled {
			return err
		}
		preCheck = &check
	}

	res, err := cleanui.RunWizard(cleanui.RunWizardParams{
		ProjectDir:        p.result.ProjectDir,
		PreselectedBranch: argBranch,
		PreCheck:          preCheck,
		Force:             p.force,
		ReparentChildren:  p.reparentFlag,
		Check: func(branch string) domain.CleanCheckResult {
			check, checkErr := worktree.Check(cleanParamsFor(cleanParamsInput{result: p.result, baseBranch: p.baseBranch, branch: branch}))
			if checkErr != nil {
				return domain.CleanCheckResult{Branch: branch}
			}
			return check
		},
		ReparentPreview: func(branch string) domain.CleanReparentPlan {
			return worktree.PlanCleanReparent(cleanParamsFor(cleanParamsInput{result: p.result, baseBranch: p.baseBranch, branch: branch}))
		},
	})
	if errors.Is(err, domain.ErrUserAborted) {
		output.Frame(cmd.OutOrStdout(), func() {
			output.Message(cmd.OutOrStdout(), "Aborted.")
		})
		return nil
	}
	if err != nil {
		return err
	}

	cleanParams := cleanParamsFor(cleanParamsInput{result: p.result, baseBranch: p.baseBranch, branch: res.Branch, force: res.Force})
	reparentPlan := worktree.PlanCleanReparent(cleanParams)
	applyReparent := len(reparentPlan.Children) > 0 && res.ReparentAsked && res.ReparentChildren
	return doClean(cmd, doCleanParams{
		Params:        cleanParams,
		Format:        p.format,
		ReparentPlan:  reparentPlan,
		ApplyReparent: applyReparent,
		CanPrompt:     true,
	})
}

// cleanParamsInput holds inputs for cleanParamsFor.
type cleanParamsInput struct {
	result     shared.ConfigResult
	baseBranch string
	branch     string
	force      bool
}

// cleanParamsFor assembles the CleanParams for a branch.
func cleanParamsFor(in cleanParamsInput) domain.CleanParams {
	return domain.CleanParams{
		ProjectDir: in.result.ProjectDir,
		StateDir:   in.result.StateDir,
		Branch:     in.branch,
		Force:      in.force,
		BaseBranch: in.baseBranch,
		Config:     in.result.Config,
	}
}

// precheckParams holds inputs for precheckClean.
type precheckParams struct {
	result     shared.ConfigResult
	baseBranch string
	branch     string
}

// precheckClean runs the safety check for an explicit branch, reporting the
// idempotent absent/parent outcomes. handled=true means the command already
// produced its output and should return.
func precheckClean(cmd *cobra.Command, p precheckParams) (domain.CleanCheckResult, bool, error) {
	params := cleanParamsFor(cleanParamsInput{result: p.result, baseBranch: p.baseBranch, branch: p.branch})
	var check domain.CleanCheckResult
	err := components.RunLoading(components.LoadingParams{
		Message: "Checking worktree…",
		Animate: true,
		Work:    func() error { var e error; check, e = worktree.Check(params); return e },
	})
	if errors.Is(err, domain.ErrWorktreeNotFound) {
		output.Frame(cmd.OutOrStdout(), func() {
			output.Message(cmd.OutOrStdout(), fmt.Sprintf("Worktree %s already absent — nothing to clean", p.branch))
		})
		return domain.CleanCheckResult{}, true, nil
	}
	if errors.Is(err, domain.ErrCannotCleanParent) {
		output.Frame(cmd.ErrOrStderr(), func() {
			output.Warning(cmd.ErrOrStderr(), "Cannot clean the parent worktree.")
		})
		return domain.CleanCheckResult{}, true, nil
	}
	if err != nil {
		return domain.CleanCheckResult{}, false, err
	}
	return check, false, nil
}

// decideReparentParams holds inputs for decideReparent.
type decideReparentParams struct {
	Plan        domain.CleanReparentPlan
	Flag        bool
	Interactive bool
}

// decideReparent resolves whether the cleaned worktree's orphaned children should
// be reparented onto the grandparent, for the non-wizard paths (--force / --yes).
// With no children it is a no-op. The --reparent-children flag forces reparenting;
// otherwise the user is asked with a single-step prompt (--yes / --force), and
// non-interactively without the flag the children are left orphaned. The second
// return reports an Esc on the prompt, which aborts the clean.
func decideReparent(cmd *cobra.Command, params decideReparentParams) (apply bool, abort bool) {
	if len(params.Plan.Children) == 0 {
		return false, false
	}
	if params.Flag {
		return true, false
	}
	if !params.Interactive {
		return false, false
	}
	return confirmReparent(cmd, params.Plan)
}

// confirmReparent shows the proposed reparenting (a bold section, separated from
// the "Will delete" recap by a blank line) and asks. Yes → reparent; No → delete
// but leave the children orphaned; Esc → abort the whole clean.
func confirmReparent(cmd *cobra.Command, plan domain.CleanReparentPlan) (apply bool, abort bool) {
	output.FormatReparentProposal(cmd.ErrOrStderr(), plan)

	cm := components.NewConfirm(components.NewConfirmParams{
		Title:      fmt.Sprintf(domain.CleanReparentPrompt, len(plan.Children), plan.Grandparent),
		DefaultYes: true,
	})
	confirmed, err := components.RunStandaloneConfirm(cm)
	if err != nil {
		// Esc (or a program failure) aborts the whole operation rather than
		// silently deleting the parent and orphaning its children.
		return false, true
	}
	return confirmed, false
}

// ensureSafeToClean runs the pre-deletion check for the `--yes` path (skip the
// prompt but keep safety) and refuses when the worktree is dirty, has unpushed
// commits, or has an open PR — directing the user to `--force` to override. A
// worktree that is absent or is the parent is left to doClean, which handles both
// idempotently.
func ensureSafeToClean(params domain.CleanParams, interactive bool) error {
	var check domain.CleanCheckResult
	err := components.RunLoading(components.LoadingParams{
		Message: "Checking worktree…",
		Animate: interactive,
		Work:    func() error { var e error; check, e = worktree.Check(params); return e },
	})
	if err != nil {
		// Absent / parent / other errors are handled by doClean; don't block here.
		return nil
	}
	if reason, unsafe := cleanUnsafeReason(check); unsafe {
		return fmt.Errorf(domain.CleanForceHintFmt, params.Branch, reason)
	}
	return nil
}

// cleanUnsafeReason reports why a worktree is unsafe to remove without --force.
func cleanUnsafeReason(check domain.CleanCheckResult) (string, bool) {
	if check.IsDirty {
		return "has uncommitted changes", true
	}
	if check.UnpushedCommits > 0 {
		return fmt.Sprintf("has %d unpushed commit(s)", check.UnpushedCommits), true
	}
	if check.HasOpenPR {
		return "has an open pull request", true
	}
	return "", false
}

// doCleanParams holds inputs for doClean.
type doCleanParams struct {
	Params        domain.CleanParams
	Format        string
	ReparentPlan  domain.CleanReparentPlan
	ApplyReparent bool
	// CanPrompt reports whether the run may show an interactive prompt (a human
	// format on a TTY, not --yes). It gates the sudo-rm removal-failure fallback.
	CanPrompt bool
}

func doClean(cmd *cobra.Command, p doCleanParams) error {
	params := p.Params
	format := p.Format

	wtPath := ""
	if wt, err := infra.FindWorktreeByBranch(infra.FindWorktreeByBranchParams{
		ProjectDir: params.ProjectDir,
		Branch:     params.Branch,
	}); err == nil {
		wtPath = wt.Path
	}

	// Decide before removal (paths must still exist to resolve symlinks).
	cwd, _ := os.Getwd()
	insideRemoved := wtPath != "" && cwd != "" && rules.IsPathWithin(resolveSymlinks(wtPath), resolveSymlinks(cwd))

	stopWorktreeServices(cmd, params.ProjectDir, params.Branch)

	// Run on_clean hooks as a distinct, titled phase before removal, so they don't
	// fight the removal spinner for the terminal; Clean itself then skips them.
	params.SkipHooks = true
	human := rules.IsHumanFormat(format)
	if hookErr := shared.RunCleanHooksPhase(shared.CleanHooksPhaseParams{
		Cmd:          cmd,
		ShowHeader:   human,
		ProjectDir:   params.ProjectDir,
		WorktreePath: wtPath,
		Branch:       params.Branch,
		Hooks:        params.Config.Project.Hooks.OnClean,
	}); hookErr != nil {
		return hookErr
	}

	err := components.RunLoading(components.LoadingParams{
		Message: fmt.Sprintf("Removing worktree %s…", params.Branch),
		Animate: human,
		Work:    func() error { return worktree.Clean(params) },
	})
	if errors.Is(err, domain.ErrWorktreeNotFound) {
		// Idempotent: cleaning an absent worktree is a no-op success so agents
		// can safely retry.
		if format == domain.OutputJSON {
			return output.WriteWorktreeCleanJSON(cmd.OutOrStdout(), output.WriteWorktreeCleanJSONParams{
				Branch:        params.Branch,
				AlreadyAbsent: true,
			})
		}
		output.Frame(cmd.OutOrStdout(), func() {
			output.Message(cmd.OutOrStdout(), fmt.Sprintf("Worktree %s already absent — nothing to clean", params.Branch))
		})
		return nil
	}
	if errors.Is(err, domain.ErrCannotCleanParent) {
		output.Frame(cmd.ErrOrStderr(), func() {
			output.Warning(cmd.ErrOrStderr(), "Cannot clean the parent worktree.")
		})
		return nil
	}
	if errors.Is(err, domain.ErrWorktreeRemoveFailed) {
		recovered, rerr := recoverRemoveFailure(cmd, recoverRemoveParams{
			CanPrompt: p.CanPrompt,
			Params:    params,
			WtPath:    wtPath,
			CleanErr:  err,
		})
		if rerr != nil {
			return rerr
		}
		if !recovered {
			return err
		}
		// Recovered via sudo rm: rejoin the normal post-clean flow so redirect,
		// child reparenting, and the success output all still run.
		err = nil
	}
	if err != nil {
		return err
	}

	reparentPlan := p.ReparentPlan
	applyReparent := p.ApplyReparent

	if insideRemoved {
		redirectToBase(params.ProjectDir)
	}

	reparented, reparentErr := applyChildReparent(params.StateDir, reparentPlan, applyReparent)
	if reparentErr != nil {
		return reparentErr
	}

	if format == domain.OutputJSON {
		return output.WriteWorktreeCleanJSON(cmd.OutOrStdout(), output.WriteWorktreeCleanJSONParams{
			Branch:           params.Branch,
			Path:             wtPath,
			Reparented:       reparented,
			OrphanedChildren: orphanedChildren(reparentPlan, applyReparent),
		})
	}

	output.Frame(cmd.OutOrStdout(), func() {
		output.Success(cmd.OutOrStdout(), fmt.Sprintf("Cleaned worktree and branch %s", params.Branch))
		for _, child := range reparented {
			output.Success(cmd.OutOrStdout(), fmt.Sprintf("Reparented %s onto %s", child.Branch, child.NewParent))
		}
		for _, child := range orphanedChildren(reparentPlan, applyReparent) {
			output.Warning(cmd.OutOrStdout(), fmt.Sprintf("%s still points at the removed parent %s — reparent it with `wtm reparent`", child.Branch, child.OldParent))
		}
	})
	return nil
}

// recoverRemoveParams holds inputs for recoverRemoveFailure.
type recoverRemoveParams struct {
	CanPrompt bool
	Params    domain.CleanParams
	WtPath    string
	CleanErr  error
}

// recoverRemoveFailure offers the interactive `sudo rm -rf` fallback when
// `git worktree remove` failed on files it cannot delete as the current user
// (typically root-owned files left by Docker). It returns recovered=true only
// when the privileged deletion succeeded, so the caller resumes the normal
// post-clean flow (reparent, redirect, success output). A non-interactive run,
// an unknown worktree path, or a declined prompt returns recovered=false, leaving
// the caller to surface the original error.
func recoverRemoveFailure(cmd *cobra.Command, p recoverRemoveParams) (bool, error) {
	if !p.CanPrompt || p.WtPath == "" {
		return false, nil
	}

	output.Warning(cmd.ErrOrStderr(), fmt.Sprintf("Removal failed: %s", p.CleanErr))

	confirm := components.NewConfirm(components.NewConfirmParams{
		Title:      fmt.Sprintf(domain.CleanSudoConfirmFmt, p.WtPath),
		DefaultYes: false,
	})
	confirmed, err := components.RunStandaloneConfirm(confirm)
	if err != nil || !confirmed {
		return false, nil
	}

	if forceErr := worktree.ForceClean(domain.ForceCleanParams{
		ProjectDir: p.Params.ProjectDir,
		Path:       p.WtPath,
		Branch:     p.Params.Branch,
		Force:      p.Params.Force,
	}); forceErr != nil {
		return false, forceErr
	}

	return true, nil
}

// applyChildReparent reparents the orphaned children when authorized, otherwise
// returns nil so the caller can report them as still-orphaned.
func applyChildReparent(stateDir string, plan domain.CleanReparentPlan, apply bool) ([]domain.ReparentResult, error) {
	if !apply || len(plan.Children) == 0 {
		return nil, nil
	}
	return worktree.ApplyReparentChildren(worktree.ApplyReparentChildrenParams{Plan: plan, StateDir: stateDir})
}

// orphanedChildren returns the children left dangling because reparenting was not
// authorized.
func orphanedChildren(plan domain.CleanReparentPlan, apply bool) []domain.ReparentResult {
	if apply || len(plan.Children) == 0 {
		return nil
	}
	return plan.Children
}

// redirectToBase asks the shell wrapper to cd into the base repo, avoiding a
// stale "ghost" directory after removing the worktree we were sitting in. It
// relies on the WTM_GO_FILE bridge (see wtm shell-init); a no-op without it.
func redirectToBase(baseDir string) {
	goFile := os.Getenv(domain.EnvGoFile)
	if goFile == "" {
		return
	}
	_ = os.WriteFile(goFile, []byte(baseDir), 0o644)
}

// resolveSymlinks returns the canonical path, falling back to the input when it
// cannot be resolved (e.g. the path no longer exists).
func resolveSymlinks(path string) string {
	if resolved, err := filepath.EvalSymlinks(path); err == nil {
		return resolved
	}
	return path
}

func stopWorktreeServices(cmd *cobra.Command, projectDir string, branch string) {
	socketPath := process.SocketPath()
	if !process.IsDaemonRunning(socketPath) {
		return
	}

	wt, err := infra.FindWorktreeByBranch(infra.FindWorktreeByBranchParams{
		ProjectDir: projectDir,
		Branch:     branch,
	})
	if err != nil {
		return
	}

	client := process.NewClient(socketPath)
	if process.StopWorktreeJobs(client, wt.Path) {
		output.Success(cmd.ErrOrStderr(), fmt.Sprintf("Stopped services on %s", branch))
	}
}
