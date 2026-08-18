package flow

import (
	"fmt"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/rules"
	"github.com/LucasPcq/wtm/internal/service/branch"
	"github.com/LucasPcq/wtm/internal/service/worktree"
)

// BranchCandidates lists the local and remote-tracking branches offered as
// worktree start-points, each local one tagged with its divergence from origin.
func BranchCandidates(projectDir string) []domain.BranchCandidate {
	return branch.Candidates(branch.ListParams{ProjectDir: projectDir})
}

// MemoizedTarget wraps branch.Target in a per-branch-name cache valid for the
// lifetime of one run: classifying a branch costs ~3 git subprocesses, and a step,
// a recap and a warning all classify the same name. The branch a run is deciding
// on cannot change underneath it.
func MemoizedTarget(projectDir string) func(string) domain.BranchTarget {
	cache := make(map[string]domain.BranchTarget)
	return func(name string) domain.BranchTarget {
		if name == "" {
			return domain.BranchTarget{}
		}
		if target, ok := cache[name]; ok {
			return target
		}
		target := branch.Target(branch.BranchParams{ProjectDir: projectDir, Branch: name})
		cache[name] = target
		return target
	}
}

// SourceUpdatePrompt is the reconciliation a run offers for the branch the
// worktree actually starts from: a fast-forward offer for a behind-only branch
// (declining proceeds as-is), or a warning for a diverged one (declining cancels).
type SourceUpdatePrompt struct {
	// Branch names the branch an accepted fast-forward applies to.
	Branch string
	// Show reports that there is something to reconcile at all.
	Show bool
	// Title, Description and Warning word the offer.
	Title       string
	Description string
	Warning     string
	// AbortOnDecline treats a "no" as cancelling the operation (a diverged branch);
	// when false a "no" simply proceeds (declining a fast-forward).
	AbortOnDecline bool
	// SkipReason explains, when Show is false, why nothing is offered.
	SkipReason string
}

// SourceUpdateParams holds inputs for SourceUpdate.
type SourceUpdateParams struct {
	ProjectDir string
	// Target classifies a branch name; pass a MemoizedTarget so a run does not hit
	// git again on every re-render.
	Target func(string) domain.BranchTarget
	// Branch is the worktree's own branch, Source its start-point / parent.
	Branch string
	Source string
}

// SourceUpdate classifies the divergence from origin of whichever branch the
// worktree actually starts from.
//
// The subject is the target branch when it already exists locally — its commits
// are what the worktree checks out, and the source is then only the recorded sync
// parent — and the source branch otherwise. One subject per run, so a step and a
// recap can never contradict each other.
func SourceUpdate(params SourceUpdateParams) SourceUpdatePrompt {
	subject := params.Source
	if params.Target != nil && params.Branch != "" &&
		params.Target(params.Branch).State == domain.BranchTargetExisting {
		subject = params.Branch
	}
	if subject == "" {
		return SourceUpdatePrompt{SkipReason: domain.SourceUpdateSkipNoSource}
	}
	if rules.IsRemoteBranch(subject) {
		return SourceUpdatePrompt{Branch: subject, SkipReason: domain.SourceUpdateSkipRemote}
	}

	state, ab := branch.Divergence(branch.BranchParams{ProjectDir: params.ProjectDir, Branch: subject})
	if rules.ShouldOfferFastForward(state) {
		return SourceUpdatePrompt{
			Branch:      subject,
			Show:        true,
			Title:       fmt.Sprintf(domain.SourceFastForwardPrompt, subject, ab.Behind),
			Description: domain.SourceFastForwardDescription,
		}
	}
	if state == domain.DivergenceDiverged {
		return SourceUpdatePrompt{
			Branch:         subject,
			Show:           true,
			Title:          fmt.Sprintf(domain.SourceDivergedPrompt, subject, ab.Ahead, ab.Behind),
			Warning:        domain.SourceDivergedWarning,
			AbortOnDecline: true,
			SkipReason:     domain.SourceUpdateSkipDiverged,
		}
	}
	return SourceUpdatePrompt{Branch: subject, SkipReason: domain.SourceUpdateSkipUpToDate}
}

// EnvFallbackParams holds inputs for EnvParentFallback.
type EnvFallbackParams struct {
	ProjectDir  string
	Source      string
	Config      domain.Config
	EnvOverride string
}

// EnvParentFallback reports whether the "parent" env strategy will silently source
// .env from the main worktree because the source branch has no local worktree, and
// the warning that says so.
func EnvParentFallback(params EnvFallbackParams) (bool, string) {
	if params.Source == "" {
		return false, ""
	}
	applies := worktree.EnvParentFallsBackToMain(worktree.EnvFallbackParams{
		ProjectDir:  params.ProjectDir,
		Source:      params.Source,
		Config:      params.Config,
		EnvOverride: params.EnvOverride,
	})
	if !applies {
		return false, ""
	}
	return true, domain.EnvParentFallbackWarning
}

// FastForwardSubjectParams holds inputs for FastForwardSubject.
type FastForwardSubjectParams struct {
	Target     domain.BranchTarget
	FromBranch string
	Branch     string
}

// FastForwardSubject names the branch a fast-forward updates: the source for a
// branch git is about to create, and the worktree's own branch when it already
// exists locally — that is the ref the worktree actually checks out, so it is the
// only one whose freshness matters.
func FastForwardSubject(params FastForwardSubjectParams) string {
	if rules.SourceIsStartPoint(params.Target.State) {
		return params.FromBranch
	}
	return params.Branch
}
