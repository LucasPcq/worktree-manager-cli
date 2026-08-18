package flow

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/rules"
	"github.com/LucasPcq/wtm/internal/service/process"
	"github.com/LucasPcq/wtm/internal/service/shell"
	"github.com/LucasPcq/wtm/internal/service/worktree"
)

// CleanRequest is what the surface already knows about a clean run. Force is the
// safety axis — a business input the service consumes — and stays here; the
// confirmation axis is carried by which Prompter is installed.
type CleanRequest struct {
	// Branch is the worktree to remove, given as a positional argument.
	Branch string
	// Force is --force: lift the safety refusals (dirty, unpushed, open PR).
	Force bool
	// ReparentChildren is --reparent-children: move orphaned children onto the
	// grandparent without asking.
	ReparentChildren bool
	// BaseBranch is the fallback parent for orphaned children when the removed
	// worktree has no recorded parent of its own.
	BaseBranch string
}

// CleanOutcome is the result of a clean run.
type CleanOutcome struct {
	Branch string
	Path   string
	// AlreadyAbsent reports the idempotent no-op: there was nothing to remove.
	AlreadyAbsent bool
	// Reparented are the children moved onto the grandparent, OrphanedChildren
	// those left pointing at the removed parent.
	Reparented       []domain.ReparentResult
	OrphanedChildren []domain.ReparentResult
	// Aborted reports that the user cancelled; nothing was removed.
	Aborted bool
}

// CleanPresenter renders a clean run.
type CleanPresenter interface {
	Presenter
	// Cleaned renders the conclusion: what was removed and what happened to the
	// children, or the machine payload of it.
	Cleaned(CleanOutcome) error
}

// CleanParams holds inputs for Clean.
type CleanParams struct {
	Context   Context
	Request   CleanRequest
	Prompter  Prompter
	Presenter CleanPresenter
}

// Clean is the whole `wtm clean` déroulé: pick and check the worktree, decide what
// happens to its children, stop its services, run its hooks, remove it, and
// conclude. Removing what is already gone is a success, so a retry is safe.
func Clean(params CleanParams) (CleanOutcome, error) {
	f := &cleanFlow{
		ctx:       params.Context,
		request:   params.Request,
		prompter:  params.Prompter,
		presenter: params.Presenter,
		checks:    make(map[string]checkResult),
	}
	return f.run()
}

// checkResult caches one safety check: it queries the PR state over the network,
// and the delete recap, the safety refusal and the reparent line all read it.
type checkResult struct {
	check domain.CleanCheckResult
	err   error
}

type cleanFlow struct {
	ctx       Context
	request   CleanRequest
	prompter  Prompter
	presenter CleanPresenter
	checks    map[string]checkResult
}

func (f *cleanFlow) run() (CleanOutcome, error) {
	// A branch given up front is checked before anything is asked, so an absent or
	// parent worktree is reported without a single question. Only an interactive run
	// pre-flights: the prompt-free path lets the removal itself report both
	// idempotently (which is what its machine payload says).
	if f.request.Branch != "" && f.prompter.Interactive() {
		_, err := f.checkStaged(f.request.Branch)
		if handled, outcome, hErr := f.handleCheckError(f.request.Branch, err); handled {
			return outcome, hErr
		}
	}

	answers, err := f.prompter.Ask(f.session())
	if errors.Is(err, domain.ErrUserAborted) {
		f.presenter.Notice(AbortedNotice)
		return CleanOutcome{Aborted: true}, nil
	}
	if err != nil {
		return CleanOutcome{}, err
	}

	branchName := answers.Value(KeyCleanWorktree)
	// --force lifts the refusals for the whole run; the delete step's own force row
	// lifts them for this one.
	force := f.request.Force || answers.Value(KeyCleanDelete) == deleteForce

	plan := f.reparentPlan(branchName)
	applyReparent := len(plan.Children) > 0 && answers.Value(KeyCleanReparent) == reparentYes

	return f.remove(removeParams{
		Params:        f.cleanParams(branchName, force),
		ReparentPlan:  plan,
		ApplyReparent: applyReparent,
	})
}

// handleCheckError maps the two idempotent outcomes of the safety check — the
// worktree is gone, or it is the parent — to a concluded run.
func (f *cleanFlow) handleCheckError(branchName string, err error) (bool, CleanOutcome, error) {
	if errors.Is(err, domain.ErrWorktreeNotFound) {
		outcome := CleanOutcome{Branch: branchName, AlreadyAbsent: true}
		return true, outcome, f.presenter.Cleaned(outcome)
	}
	if errors.Is(err, domain.ErrCannotCleanParent) {
		f.presenter.Notice(Notice{Kind: NoticeWarning, Text: domain.CleanCannotCleanParent})
		return true, CleanOutcome{Branch: branchName}, nil
	}
	if err != nil {
		return true, CleanOutcome{}, err
	}
	return false, CleanOutcome{}, nil
}

// removeParams holds inputs for remove.
type removeParams struct {
	Params        domain.CleanParams
	ReparentPlan  domain.CleanReparentPlan
	ApplyReparent bool
}

// remove is the execution: stop the services, run the hooks, remove the worktree,
// follow the user out of a directory that no longer exists, and settle the children.
func (f *cleanFlow) remove(p removeParams) (CleanOutcome, error) {
	params := p.Params
	worktreePath := ""
	if wt, err := worktree.FindByBranch(worktree.FindByBranchParams{
		ProjectDir: params.ProjectDir,
		Branch:     params.Branch,
	}); err == nil {
		worktreePath = wt.Path
	}

	// Decided before the removal, while the paths still resolve.
	cwd, _ := os.Getwd()
	insideRemoved := worktreePath != "" && cwd != "" &&
		rules.IsPathWithin(resolveSymlinks(worktreePath), resolveSymlinks(cwd))

	f.stopServices(params.Branch)

	// on_clean hooks run as their own phase before the removal, so they don't fight
	// the removal progress for the terminal; the service then skips them.
	params.SkipHooks = true
	if hookErr := f.runHooks(worktreePath, params.Branch); hookErr != nil {
		return CleanOutcome{}, hookErr
	}

	err := f.presenter.Stage(StageParams{
		Message: fmt.Sprintf(domain.CleanLoadingFmt, params.Branch),
		Work:    func() error { return worktree.Clean(params) },
	})
	if errors.Is(err, domain.ErrWorktreeNotFound) {
		outcome := CleanOutcome{Branch: params.Branch, AlreadyAbsent: true}
		return outcome, f.presenter.Cleaned(outcome)
	}
	if errors.Is(err, domain.ErrCannotCleanParent) {
		f.presenter.Notice(Notice{Kind: NoticeWarning, Text: domain.CleanCannotCleanParent})
		return CleanOutcome{Branch: params.Branch}, nil
	}
	if errors.Is(err, domain.ErrWorktreeRemoveFailed) {
		recovered, rErr := f.recoverRemoveFailure(recoverParams{
			Params: params,
			Path:   worktreePath,
			Cause:  err,
		})
		if rErr != nil {
			return CleanOutcome{}, rErr
		}
		if !recovered {
			return CleanOutcome{}, err
		}
		// Recovered: rejoin the normal post-removal flow so the redirect, the
		// children and the recap all still happen.
		err = nil
	}
	if err != nil {
		return CleanOutcome{}, err
	}

	if insideRemoved {
		shell.RequestCd(params.ProjectDir)
	}

	reparented, reparentErr := f.applyReparent(p.ReparentPlan, p.ApplyReparent)
	if reparentErr != nil {
		return CleanOutcome{}, reparentErr
	}

	outcome := CleanOutcome{
		Branch:           params.Branch,
		Path:             worktreePath,
		Reparented:       reparented,
		OrphanedChildren: orphanedChildren(p.ReparentPlan, p.ApplyReparent),
	}
	return outcome, f.presenter.Cleaned(outcome)
}

// stopServices asks the daemon to stop the jobs running in the worktree about to
// disappear. A no-op when no daemon is running or the worktree is unknown.
func (f *cleanFlow) stopServices(branchName string) {
	socket := process.SocketPath()
	if !process.IsDaemonRunning(socket) {
		return
	}
	wt, err := worktree.FindByBranch(worktree.FindByBranchParams{
		ProjectDir: f.ctx.ProjectDir,
		Branch:     branchName,
	})
	if err != nil {
		return
	}
	if process.StopWorktreeJobs(process.NewClient(socket), wt.Path) {
		f.presenter.Status(Notice{
			Kind: NoticeSuccess,
			Text: fmt.Sprintf(domain.CleanStoppedServicesFmt, branchName),
		})
	}
}

func (f *cleanFlow) runHooks(worktreePath, branchName string) error {
	hooks := f.ctx.Config.Project.Hooks.OnClean
	if len(hooks) == 0 || worktreePath == "" {
		return nil
	}
	return f.presenter.HookPhase(HookPhaseParams{
		Title: domain.HooksTitleOnClean,
		Run: func(sink io.Writer) error {
			return worktree.RunCleanHooks(domain.CleanHooksParams{
				ProjectDir:   f.ctx.ProjectDir,
				WorktreePath: worktreePath,
				Branch:       branchName,
				Hooks:        hooks,
				Output:       sink,
			})
		},
	})
}

// recoverParams holds inputs for recoverRemoveFailure.
type recoverParams struct {
	Params domain.CleanParams
	Path   string
	Cause  error
}

// recoverRemoveFailure offers the privileged removal when `git worktree remove`
// failed on files the current user cannot delete (typically root-owned files left
// by a container). It reports recovered=true only when the privileged deletion
// succeeded, so the caller resumes the normal post-removal flow. Nobody to ask, an
// unknown path, or a decline leaves the original failure to the caller.
func (f *cleanFlow) recoverRemoveFailure(p recoverParams) (bool, error) {
	if !f.prompter.Interactive() || p.Path == "" {
		return false, nil
	}

	f.presenter.Status(Notice{
		Kind: NoticeWarning,
		Text: fmt.Sprintf(domain.CleanRemovalFailedFmt, p.Cause),
	})

	confirmed, err := f.prompter.Confirm(ConfirmParams{
		Title:      fmt.Sprintf(domain.CleanSudoConfirmFmt, p.Path),
		DefaultYes: false,
	})
	if err != nil || !confirmed {
		return false, nil
	}

	if forceErr := worktree.ForceClean(domain.ForceCleanParams{
		ProjectDir: p.Params.ProjectDir,
		Path:       p.Path,
		Branch:     p.Params.Branch,
		Force:      p.Params.Force,
	}); forceErr != nil {
		return false, forceErr
	}
	return true, nil
}

// applyReparent moves the orphaned children onto the grandparent when authorized.
func (f *cleanFlow) applyReparent(plan domain.CleanReparentPlan, apply bool) ([]domain.ReparentResult, error) {
	if !apply || len(plan.Children) == 0 {
		return nil, nil
	}
	return worktree.ApplyReparentChildren(worktree.ApplyReparentChildrenParams{
		Plan:     plan,
		StateDir: f.ctx.StateDir,
	})
}

// orphanedChildren are the children left dangling because reparenting was not
// authorized.
func orphanedChildren(plan domain.CleanReparentPlan, apply bool) []domain.ReparentResult {
	if apply || len(plan.Children) == 0 {
		return nil
	}
	return plan.Children
}

// cleanParams assembles the service inputs for a branch.
func (f *cleanFlow) cleanParams(branchName string, force bool) domain.CleanParams {
	return domain.CleanParams{
		ProjectDir: f.ctx.ProjectDir,
		StateDir:   f.ctx.StateDir,
		Branch:     branchName,
		Force:      force,
		BaseBranch: f.request.BaseBranch,
		Config:     f.ctx.Config,
	}
}

// reparentPlan lists the children a removal would orphan. Force plays no part in
// it, so the plan is the same whichever way the safety axis went.
func (f *cleanFlow) reparentPlan(branchName string) domain.CleanReparentPlan {
	return worktree.PlanCleanReparent(f.cleanParams(branchName, false))
}

// checkStaged runs the safety check under a progress indicator, once per branch.
func (f *cleanFlow) checkStaged(branchName string) (domain.CleanCheckResult, error) {
	if cached, ok := f.checks[branchName]; ok {
		return cached.check, cached.err
	}
	var result checkResult
	if err := f.presenter.Stage(StageParams{
		Message: domain.CleanCheckLoading,
		Work: func() error {
			result.check, result.err = worktree.Check(f.cleanParams(branchName, false))
			return nil
		},
	}); err != nil {
		return domain.CleanCheckResult{}, err
	}
	f.checks[branchName] = result
	return result.check, result.err
}

// checkCached runs the safety check without a progress indicator: the host that
// loads a step already shows one of its own.
func (f *cleanFlow) checkCached(branchName string) (domain.CleanCheckResult, error) {
	if cached, ok := f.checks[branchName]; ok {
		return cached.check, cached.err
	}
	var result checkResult
	result.check, result.err = worktree.Check(f.cleanParams(branchName, false))
	f.checks[branchName] = result
	return result.check, result.err
}

// resolveSymlinks returns the canonical path, falling back to the input when it
// cannot be resolved (the path may no longer exist).
func resolveSymlinks(path string) string {
	if resolved, err := filepath.EvalSymlinks(path); err == nil {
		return resolved
	}
	return path
}
