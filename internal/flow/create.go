package flow

import (
	"errors"
	"fmt"
	"io"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/rules"
	"github.com/LucasPcq/wtm/internal/service/branch"
	"github.com/LucasPcq/wtm/internal/service/worktree"
)

// CreateRequest is what the surface already knows about a create run. It holds no
// --yes and no output format: the confirmation axis is carried by which Prompter is
// installed, and the format by the surface.
type CreateRequest struct {
	// Branch is the worktree's branch, given as a positional argument.
	Branch string
	// From is --from: the start-point of a new branch, or the parent recorded for
	// `wtm sync` when the branch already exists locally.
	From string
	// EnvFrom is --env-from: an env strategy overriding the configured one.
	EnvFrom string
	// FastForward is --ff: update the branch the worktree starts from, when it is a
	// clean fast-forward away from origin.
	FastForward bool
	// IfNotExists is --if-not-exists: an already-existing worktree is a success.
	IfNotExists bool
}

// CreateOutcome is the result of a create run.
type CreateOutcome struct {
	Result     domain.CreateResult
	Branch     string
	FromBranch string
	// Aborted reports that the user cancelled; nothing was created.
	Aborted bool
}

// CreatePresenter renders a create run.
type CreatePresenter interface {
	Presenter
	// Created renders the conclusion: the recap of what was created, or the machine
	// payload of it.
	Created(CreateOutcome) error
}

// CreateParams holds inputs for Create.
type CreateParams struct {
	Context   Context
	Request   CreateRequest
	Prompter  Prompter
	Presenter CreatePresenter
}

// Create is the whole `wtm create` déroulé: validate what is knowable up front,
// ask what is missing, apply an accepted fast-forward, create the worktree, run its
// hooks, and conclude.
func Create(params CreateParams) (CreateOutcome, error) {
	f := &createFlow{
		ctx:        params.Context,
		request:    params.Request,
		prompter:   params.Prompter,
		presenter:  params.Presenter,
		candidates: BranchCandidates(params.Context.ProjectDir),
		target:     MemoizedTarget(params.Context.ProjectDir),
	}
	return f.run()
}

type createFlow struct {
	ctx        Context
	request    CreateRequest
	prompter   Prompter
	presenter  CreatePresenter
	candidates []domain.BranchCandidate
	target     func(string) domain.BranchTarget
}

func (f *createFlow) run() (CreateOutcome, error) {
	// An explicit --from is validated up front, so every path reports a missing
	// branch the same way.
	if f.request.From != "" && !rules.BranchCandidateExists(f.candidates, f.request.From) {
		return CreateOutcome{}, fmt.Errorf("%w: %s", domain.ErrBranchNotFound, f.request.From)
	}

	// A branch checked out elsewhere is knowable right here — failing before any
	// question (worktree.Create's guard is the chokepoint for the other callers)
	// saves the user a full interactive run that could only ever end in refusal.
	if f.request.Branch != "" {
		if target := f.target(f.request.Branch); target.State == domain.BranchTargetCheckedOut && !f.request.IfNotExists {
			return CreateOutcome{}, fmt.Errorf("%w: "+domain.BranchCheckedOutElsewhereFmt,
				domain.ErrWorktreeExists, f.request.Branch, target.WorktreePath, f.request.Branch)
		}
	}

	answers, err := f.prompter.Ask(f.session())
	if errors.Is(err, domain.ErrUserAborted) {
		f.presenter.Notice(AbortedNotice)
		return CreateOutcome{Aborted: true}, nil
	}
	if err != nil {
		return CreateOutcome{}, err
	}

	branchName := answers.Value(KeyCreateBranch)
	fromBranch := answers.Value(KeyCreateSource)

	// Only an accepted fast-forward runs here: its subject is whichever branch the
	// worktree starts from, which the source-update step already resolved.
	if answers.Value(KeyCreateSourceUpdate) == updateFastForward {
		subject := f.sourceUpdate(answers).Branch
		proceed, ffErr := f.applyFastForward(subject)
		if ffErr != nil {
			return CreateOutcome{}, ffErr
		}
		if !proceed {
			return CreateOutcome{Aborted: true}, nil
		}
	}

	// The branch name may only exist now, so the target is re-inspected to drive the
	// start-point decision and the reuse reporting.
	target := f.target(branchName)

	// A reused branch is checked out as-is: the source is not a start-point, so it
	// is recorded as the sync parent instead (git would ignore it anyway).
	startPoint := fromBranch
	if !rules.SourceIsStartPoint(target.State) {
		startPoint = ""
	}

	// Phase 1 — the silent creation (git worktree add + env copy). Hooks are held
	// back (SkipHooks) so they stream in their own phase below.
	var result domain.CreateResult
	err = f.presenter.Stage(StageParams{
		Message: fmt.Sprintf(domain.CreateLoadingFmt, branchName),
		Work: func() error {
			var createErr error
			result, createErr = worktree.Create(domain.CreateParams{
				ProjectDir:      f.ctx.ProjectDir,
				StateDir:        f.ctx.StateDir,
				Branch:          branchName,
				FromBranch:      startPoint,
				SourceBranch:    fromBranch,
				Config:          f.ctx.Config,
				EnvFromOverride: answers.Value(KeyCreateEnv),
				IfNotExists:     f.request.IfNotExists,
				SkipHooks:       true,
			})
			return createErr
		},
	})
	if err != nil {
		return CreateOutcome{}, err
	}

	// Phase 2 — on_create hooks as a distinct phase. Skipped when the worktree
	// already existed (its hooks ran on first creation).
	if !result.AlreadyExists {
		if hookErr := f.runHooks(result.Path, branchName, fromBranch); hookErr != nil {
			return CreateOutcome{}, hookErr
		}
	}

	outcome := CreateOutcome{Result: result, Branch: branchName, FromBranch: fromBranch}
	return outcome, f.presenter.Created(outcome)
}

func (f *createFlow) runHooks(worktreePath, branchName, fromBranch string) error {
	hooks := f.ctx.Config.Project.Hooks.OnCreate
	if len(hooks) == 0 {
		return nil
	}
	return f.presenter.HookPhase(HookPhaseParams{
		Title: domain.HooksTitleOnCreate,
		Run: func(sink io.Writer) error {
			return worktree.RunCreateHooks(domain.CreateHooksParams{
				ProjectDir:   f.ctx.ProjectDir,
				WorktreePath: worktreePath,
				Branch:       branchName,
				FromBranch:   fromBranch,
				Hooks:        hooks,
				Output:       sink,
			})
		},
	})
}

// applyFastForward executes the accepted fast-forward and reports whether the run
// may continue.
func (f *createFlow) applyFastForward(subject string) (bool, error) {
	params := branch.BranchParams{ProjectDir: f.ctx.ProjectDir, Branch: subject}

	// Without anyone to answer, --ff is best effort: a branch that cannot be cleanly
	// fast-forwarded (diverged, dirty, fetch failure) is left as-is and creation
	// proceeds from it — the non-interactive form of "keep it as-is".
	if !f.prompter.Interactive() {
		_ = branch.FastForwardIfBehind(params)
		return true, nil
	}

	ffErr := f.presenter.Stage(StageParams{
		Message: fmt.Sprintf(domain.SourceFastForwardLoadingFmt, subject),
		Work:    func() error { return branch.FastForwardToOrigin(params) },
	})
	if ffErr == nil {
		return true, nil
	}

	// The fast-forward failing is a runtime outcome, so asking to proceed from the
	// stale branch is a legitimate post-execution question.
	_, ab := branch.Divergence(params)
	proceed, confirmErr := f.prompter.Confirm(ConfirmParams{
		Title:      fmt.Sprintf(domain.SourceProceedStalePrompt, subject, ab.Behind),
		Warning:    fmt.Sprintf(domain.SourceProceedStaleWarning, ffErr),
		DefaultYes: false,
	})
	if confirmErr != nil {
		return false, nil
	}
	return proceed, nil
}
