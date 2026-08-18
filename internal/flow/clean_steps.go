package flow

import (
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/rules"
	"github.com/LucasPcq/wtm/internal/service/worktree"
)

// The questions `wtm clean` asks, and the confirmation it shows before removing
// anything. cleanFlow.run and cleanFlow.remove (clean.go) act on the answers.

// Step keys of the clean session.
const (
	KeyCleanWorktree = "clean.worktree"
	KeyCleanReparent = "clean.reparent"
	KeyCleanDelete   = "clean.delete"
)

// Answer values of the reparent and delete steps.
const (
	reparentYes = "reparent"
	reparentNo  = "orphan"
	deleteYes   = "yes"
	deleteForce = "force"
)

// Step labels of the clean session.
const (
	labelCleanWorktree = "Worktree"
	labelCleanReparent = "Reparent children"
	labelCleanDelete   = "Delete"
)

// session declares the clean questions, in the order that makes the delete recap
// the last, unconditional action point: pick the worktree, decide about its
// children, then confirm.
func (f *cleanFlow) session() Session {
	return Session{
		ErrLabel: domain.CleanWizardErrLabel,
		Presets: NewAnswers(map[string]string{
			KeyCleanWorktree: f.request.Branch,
			KeyCleanReparent: f.presetReparent(),
		}),
		Steps: []Step{
			f.worktreeStep(),
			f.reparentStep(),
			f.deleteStep(),
		},
	}
}

func (f *cleanFlow) worktreeStep() Step {
	return Step{
		Kind:        StepSelect,
		Key:         KeyCleanWorktree,
		Label:       labelCleanWorktree,
		Title:       domain.CleanPickerTitle,
		Description: domain.CleanPickerDescription,
		Build: func(Answers) (StepContent, error) {
			options, err := f.cleanableOptions()
			if err != nil {
				return StepContent{}, err
			}
			return StepContent{
				Title:       domain.CleanPickerTitle,
				Description: domain.CleanPickerDescription,
				Options:     options,
			}, nil
		},
		// A worktree has no safe default: a run that cannot ask must name the target.
		Resolve: func(Answers) (Answer, error) { return Answer{}, domain.ErrCleanBranchRequired },
	}
}

// cleanableOptions lists every worktree but the main one, which cannot be cleaned,
// sorted by branch. Refuses when there is nothing to offer rather than showing an
// empty picker.
func (f *cleanFlow) cleanableOptions() ([]Option, error) {
	worktrees, err := worktree.ListAll(worktree.ListAllParams{ProjectDir: f.ctx.ProjectDir})
	if err != nil {
		return nil, fmt.Errorf("list worktrees: %w", err)
	}
	options := make([]Option, 0, len(worktrees))
	for _, wt := range worktrees {
		if wt.IsMain {
			continue
		}
		options = append(options, Option{Label: wt.Branch, Value: wt.Branch})
	}
	sort.Slice(options, func(i, j int) bool { return options[i].Value < options[j].Value })
	if len(options) == 0 {
		return nil, errors.New(domain.CleanNothingToClean)
	}
	return options, nil
}

// reparentStep decides what happens to the children the removal would orphan. It
// precedes the delete recap, so every option merely advances: going back returns to
// the picker, never cancelling.
func (f *cleanFlow) reparentStep() Step {
	return Step{
		Kind:  StepSelect,
		Key:   KeyCleanReparent,
		Label: labelCleanReparent,
		Skip: func(answers Answers) (bool, string) {
			if len(f.reparentPlan(answers.Value(KeyCleanWorktree)).Children) == 0 {
				return true, domain.CleanNoOrphanedChildren
			}
			return false, ""
		},
		Build: func(answers Answers) (StepContent, error) {
			plan := f.reparentPlan(answers.Value(KeyCleanWorktree))
			return StepContent{
				Description: reparentProposal(plan),
				Options: []Option{
					{Label: fmt.Sprintf(domain.CleanReparentOptionFmt, plan.Grandparent, len(plan.Children)), Value: reparentYes},
					{Separator: true},
					{Label: domain.CleanOrphanOption, Value: reparentNo},
				},
			}, nil
		},
		// Reparenting is opt-in: the safe default leaves the children where they are,
		// and `wtm reparent` can still move them afterwards. --reparent-children
		// answers the step through the presets, so it is never asked either way.
		Resolve: func(Answers) (Answer, error) { return Answer{Value: reparentNo}, nil },
		Summarize: func(answer Answer) string {
			if answer.Value == reparentYes {
				return domain.CleanReparentSummary
			}
			return domain.CleanOrphanSummary
		},
		Flag: domain.FlagReparentChildren,
	}
}

// reparentProposal lists the children a removal would orphan, and where they would
// go instead.
func reparentProposal(plan domain.CleanReparentPlan) string {
	lines := make([]string, 0, len(plan.Children)+1)
	lines = append(lines, domain.CleanReparentIntro)
	for _, child := range plan.Children {
		lines = append(lines, fmt.Sprintf(domain.CleanReparentChildFmt, child.Branch, child.NewParent, child.OldParent))
	}
	return strings.Join(lines, "\n")
}

// deleteStep is the final recap: it states what will be removed, surfaces every
// safety warning, and offers the force row only when there is something to force.
func (f *cleanFlow) deleteStep() Step {
	content := func(answers Answers) (StepContent, error) {
		check, _ := f.checkCached(answers.Value(KeyCleanWorktree))
		return StepContent{
			Title:       domain.CleanDeleteTitle,
			Description: deleteRecap(check, f.reparentLine(answers)),
			Options:     deleteOptions(check),
		}, nil
	}

	step := Step{
		Kind:           StepRecap,
		Key:            KeyCleanDelete,
		Label:          labelCleanDelete,
		Title:          domain.CleanDeleteTitle,
		LoadingMessage: domain.CleanCheckLoading,
		Resolve:        f.resolveDelete,
	}
	// A branch given up front was already checked, so its recap needs no I/O and is
	// built as the step is entered. A picked one is checked then and there — the
	// check queries the PR state over the network, so it loads off the render path.
	if f.request.Branch != "" {
		step.Build = content
		return step
	}
	step.Load = content
	return step
}

// resolveDelete answers the confirmation without asking — and keeps the safety
// check while doing it: --yes is the confirmation axis, not a licence to remove
// something unsafe. Only --force lifts the refusal.
func (f *cleanFlow) resolveDelete(answers Answers) (Answer, error) {
	if f.request.Force {
		return Answer{Value: deleteYes}, nil
	}
	check, err := f.checkStaged(answers.Value(KeyCleanWorktree))
	if err != nil {
		// Absent, parent, or unreadable: the removal itself reports all three
		// idempotently, so nothing is blocked here.
		return Answer{Value: deleteYes}, nil
	}
	if reason, unsafe := rules.CleanUnsafeReason(check); unsafe {
		return Answer{}, fmt.Errorf(domain.CleanForceHintFmt, check.Branch, reason)
	}
	return Answer{Value: deleteYes}, nil
}

// deleteOptions offers the plain removal, plus a danger row bypassing the checks
// when the worktree is unsafe to remove.
func deleteOptions(check domain.CleanCheckResult) []Option {
	options := []Option{{Label: domain.CleanDeleteOption, Value: deleteYes}}
	if rules.HasWarnings(check) {
		options = append(options,
			Option{Separator: true},
			Option{Label: domain.CleanForceDeleteOption, Value: deleteForce, Danger: true},
		)
	}
	return options
}

// deleteRecap states the warnings, what will be removed, and the reparent decision.
func deleteRecap(check domain.CleanCheckResult, reparentLine string) string {
	var lines []string
	if check.IsDirty {
		lines = append(lines, domain.CleanWarnDirty)
	}
	if check.UnpushedCommits > 0 {
		lines = append(lines, fmt.Sprintf(domain.CleanWarnUnpushedFmt, check.UnpushedCommits))
	}
	if check.HasOpenPR {
		lines = append(lines, domain.CleanWarnOpenPR+check.PRUrl)
	}
	if len(lines) > 0 {
		lines = append(lines, "")
	}
	lines = append(lines,
		domain.CleanWillDelete,
		domain.CleanWillDeleteWorktree+check.WorktreePath,
		domain.CleanWillDeleteBranch+check.Branch,
	)
	if reparentLine != "" {
		lines = append(lines, "", reparentLine)
	}
	return strings.Join(lines, "\n")
}

// reparentLine summarizes the reparent decision for the delete recap, or "" when
// the worktree has no children to orphan.
func (f *cleanFlow) reparentLine(answers Answers) string {
	branchName := answers.Value(KeyCleanWorktree)
	if branchName == "" {
		return ""
	}
	plan := f.reparentPlan(branchName)
	if len(plan.Children) == 0 {
		return ""
	}
	if answers.Value(KeyCleanReparent) == reparentYes {
		return fmt.Sprintf(domain.CleanRecapReparentFmt, len(plan.Children), plan.Grandparent)
	}
	return fmt.Sprintf(domain.CleanRecapOrphanFmt, len(plan.Children))
}

// presetReparent answers the reparent step from --reparent-children, so the
// decision is not asked and every reader sees the same answer.
func (f *cleanFlow) presetReparent() string {
	if f.request.ReparentChildren {
		return reparentYes
	}
	return ""
}
