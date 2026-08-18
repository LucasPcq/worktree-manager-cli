package flow

import (
	"errors"
	"fmt"
	"strings"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/rules"
	"github.com/LucasPcq/wtm/internal/service/branch"
)

// The questions `wtm create` asks, and the recap it shows before creating anything.
// createFlow.run (create.go) is what acts on the answers.

// Step keys of the create session.
const (
	KeyCreateBranch       = "create.branch"
	KeyCreateSource       = "create.source"
	KeyCreateEnv          = "create.env"
	KeyCreateSourceUpdate = "create.source_update"
	KeyCreateRecap        = "create.recap"
)

// Answer values of the source-update and recap steps.
const (
	updateFastForward = "ff"
	updateKeep        = "keep"
	createConfirm     = "create"
)

// Step labels of the create session, shown by the host as it progresses.
const (
	labelCreateBranch       = "Branch name"
	labelCreateSource       = "Source branch"
	labelCreateEnv          = "Env strategy"
	labelCreateSourceUpdate = "Source update"
	labelCreateRecap        = "Confirm & create"
)

// session declares the create questions. A step whose value the request already
// carries is answered by the presets, so it is not asked — but it is still read
// back from the answers, which is why a flag never makes a line disappear from the
// recap.
//
// TEMPORARY DUPLICATION: these steps are also declared, in Bubbletea terms, by
// internal/tui/newwt (CreateSteps/ReadCreateResult/buildCreateRecap). `wtm extract`
// embeds that version as a sub-flow of its own combined wizard, so it cannot be
// removed until extract migrates to this package too (LUC-173, lot 4). Until then
// the two declarations must be changed together: same steps, same prose (the shared
// wording lives in internal/domain/constants.go), same recap. `wtm create` reads
// only the version below.
func (f *createFlow) session() Session {
	return Session{
		ErrLabel: domain.WizardErrLabel,
		Presets: NewAnswers(map[string]string{
			KeyCreateBranch: f.request.Branch,
			KeyCreateSource: f.request.From,
			KeyCreateEnv:    f.request.EnvFrom,
		}),
		Steps: []Step{
			f.branchStep(),
			f.sourceStep(),
			f.envStep(),
			f.sourceUpdateStep(),
			f.recapStep(),
		},
	}
}

func (f *createFlow) branchStep() Step {
	return Step{
		Kind:        StepText,
		Key:         KeyCreateBranch,
		Label:       labelCreateBranch,
		Title:       labelCreateBranch,
		Description: domain.CreateBranchStepDescription,
		Validate: func(value string) error {
			if strings.TrimSpace(value) == "" {
				return errors.New(domain.CreateBranchRequired)
			}
			return nil
		},
		// A branch name has no safe default: a run that cannot ask must refuse.
		Resolve: func(Answers) (Answer, error) {
			return Answer{}, errors.New(domain.CreateBranchRequiredUnattended)
		},
	}
}

func (f *createFlow) sourceStep() Step {
	return Step{
		Kind:        StepBranchSelect,
		Key:         KeyCreateSource,
		Label:       labelCreateSource,
		Title:       labelCreateSource,
		Description: domain.CreateSourceStepDescription,
		Branches:    f.candidates,
		Pinned:      f.ctx.Config.Project.Worktrees.BaseBranch,
		Refresh: func() []domain.BranchCandidate {
			return branch.Refresh(branch.ListParams{ProjectDir: f.ctx.ProjectDir})
		},
		Build: func(answers Answers) (StepContent, error) {
			// A branch that already exists keeps its commits, so the source is only
			// the parent recorded for `wtm sync`, never a start-point.
			description := domain.CreateSourceStepDescription
			if f.reusesBranch(answers) {
				description = domain.RecapParentRecordedForSync
			}
			return StepContent{Title: labelCreateSource, Description: description}, nil
		},
		Resolve: f.resolveSource,
		Flag:    domain.FlagFrom,
	}
}

// resolveSource answers the source without asking. An existing branch was created
// outside wtm, so nothing says what it was branched off: defaulting the parent
// would record a guess that `sync` and `tree` then treat as fact, so it is a
// required selection. A branch git creates starts from the configured base branch —
// mirroring the picker's pinned default.
func (f *createFlow) resolveSource(answers Answers) (Answer, error) {
	branchName := answers.Value(KeyCreateBranch)
	target := f.target(branchName)
	if rules.ParentMustBeExplicit(target.State) {
		return Answer{}, fmt.Errorf(domain.ParentRequiredFmt, branchName, domain.FlagFrom)
	}
	base := f.ctx.Config.Project.Worktrees.BaseBranch
	if base == "" {
		return Answer{}, fmt.Errorf(domain.CreateNoSourceFmt, domain.FlagFrom)
	}
	if !rules.BranchCandidateExists(f.candidates, base) {
		return Answer{}, fmt.Errorf("%w: %s", domain.ErrBranchNotFound, base)
	}
	return Answer{Value: base}, nil
}

func (f *createFlow) envStep() Step {
	strategy := string(f.ctx.Config.Project.Env.Strategy)
	return Step{
		Kind:        StepSelect,
		Key:         KeyCreateEnv,
		Label:       labelCreateEnv,
		Title:       labelCreateEnv,
		Description: domain.CreateEnvStepDescription,
		Options: []Option{
			{Label: fmt.Sprintf(domain.EnvOptionConfigDefaultFmt, strategy), Value: ""},
			{Label: domain.EnvOptionExample, Value: string(domain.EnvStrategyExample)},
			{Label: domain.EnvOptionMain, Value: string(domain.EnvStrategyMain)},
			{Label: domain.EnvOptionParent, Value: string(domain.EnvStrategyParent)},
		},
		// The configured strategy is the safe default.
		Resolve:   func(Answers) (Answer, error) { return Answer{Value: ""}, nil },
		Summarize: envSummary,
		Flag:      domain.FlagEnvFrom,
	}
}

// envSummary labels the chosen strategy, naming the empty (config default) choice
// explicitly rather than leaving it blank.
func envSummary(answer Answer) string {
	if answer.Value != "" {
		return answer.Value
	}
	return domain.EnvSummaryConfigDefault
}

// sourceUpdateStep is the conditional fast-forward decision. It applies only to a
// behind-only branch; a diverged one is not a gate here — it becomes a ⚠ line in
// the recap.
func (f *createFlow) sourceUpdateStep() Step {
	return Step{
		Kind:  StepSelect,
		Key:   KeyCreateSourceUpdate,
		Label: labelCreateSourceUpdate,
		Skip: func(answers Answers) (bool, string) {
			prompt := f.sourceUpdate(answers)
			if prompt.Show && !prompt.AbortOnDecline {
				return false, ""
			}
			return true, prompt.SkipReason
		},
		Build: func(answers Answers) (StepContent, error) {
			prompt := f.sourceUpdate(answers)
			return StepContent{
				Description: prompt.Description,
				Options: []Option{
					{Label: fmt.Sprintf(domain.SourceFastForwardOptionFmt, prompt.Branch), Value: updateFastForward},
					{Separator: true},
					{Label: domain.SourceKeepAsIsOption, Value: updateKeep},
				},
			}, nil
		},
		// --ff accepts the offer without asking; anything else keeps the branch as
		// it is — never a mutation the user did not request.
		Resolve: func(Answers) (Answer, error) {
			if f.request.FastForward {
				return Answer{Value: updateFastForward}, nil
			}
			return Answer{Value: updateKeep}, nil
		},
		Summarize: func(answer Answer) string {
			if answer.Value == updateFastForward {
				return domain.SourceUpdateSummaryFastForward
			}
			return domain.SourceUpdateSummaryKeep
		},
		Flag: domain.FlagFF,
	}
}

func (f *createFlow) recapStep() Step {
	return Step{
		Kind:  StepRecap,
		Key:   KeyCreateRecap,
		Label: labelCreateRecap,
		Build: func(answers Answers) (StepContent, error) {
			return StepContent{
				Description: f.recap(answers),
				Options:     []Option{{Label: domain.CreateRecapConfirmOption, Value: createConfirm}},
			}, nil
		},
		Resolve: func(Answers) (Answer, error) { return Answer{Value: createConfirm}, nil },
	}
}

// sourceUpdate classifies the branch the worktree starts from, from the answers so
// far. Called by the step, the recap and the execution — always the same subject.
func (f *createFlow) sourceUpdate(answers Answers) SourceUpdatePrompt {
	return SourceUpdate(SourceUpdateParams{
		ProjectDir: f.ctx.ProjectDir,
		Target:     f.target,
		Branch:     answers.Value(KeyCreateBranch),
		Source:     answers.Value(KeyCreateSource),
	})
}

// reusesBranch reports whether the worktree will check out an existing local
// branch rather than create one.
func (f *createFlow) reusesBranch(answers Answers) bool {
	return f.target(answers.Value(KeyCreateBranch)).State == domain.BranchTargetExisting
}

// recap renders the review body: the selections, plus a ⚠ line for anything the
// user could not otherwise see. Every line falls back to the value the request
// resolved, so a flag never makes a line disappear.
func (f *createFlow) recap(answers Answers) string {
	source := answers.Value(KeyCreateSource)
	branchName := answers.Value(KeyCreateBranch)
	reused := f.reusesBranch(answers)

	envLabel := answers.Value(KeyCreateEnv)
	if envLabel == "" {
		envLabel = domain.EnvSummaryConfigDefault
	}

	branchLabel := branchName
	if reused {
		branchLabel += domain.BranchReusedSuffix
	}

	// The source line is a start-point for a new branch and only the recorded sync
	// parent for a reused one; the fast-forward annotation follows its subject.
	sourceField := domain.RecapFieldSource
	if reused {
		sourceField = domain.RecapFieldParent
	}
	ffBranch := ""
	if answers.Value(KeyCreateSourceUpdate) == updateFastForward {
		ffBranch = f.sourceUpdate(answers).Branch
	}
	sourceLabel := source
	if ffBranch != "" && ffBranch == source {
		sourceLabel += domain.RecapFastForwardSuffix
	}

	var lines []string
	if branchLabel != "" {
		lines = append(lines, domain.RecapFieldBranch+branchLabel)
	}
	lines = append(lines, sourceField+sourceLabel, domain.RecapFieldEnv+envLabel)
	if ffBranch != "" && ffBranch != source {
		lines = append(lines, fmt.Sprintf(domain.RecapUpdateFastForward, ffBranch))
	}

	if warnings := f.warnings(answers); len(warnings) > 0 {
		lines = append(lines, "")
		lines = append(lines, warnings...)
	}
	return strings.Join(lines, "\n")
}

// warnings are the ⚠ recap lines: a diverged source (which cannot be
// fast-forwarded) and the parent env strategy silently falling back to main.
func (f *createFlow) warnings(answers Answers) []string {
	var warnings []string
	if prompt := f.sourceUpdate(answers); prompt.Show && prompt.AbortOnDecline && prompt.Warning != "" {
		warnings = append(warnings, domain.WarningPrefix+prompt.Warning)
	}
	if show, warning := EnvParentFallback(EnvFallbackParams{
		ProjectDir:  f.ctx.ProjectDir,
		Source:      answers.Value(KeyCreateSource),
		Config:      f.ctx.Config,
		EnvOverride: answers.Value(KeyCreateEnv),
	}); show {
		warnings = append(warnings, domain.WarningPrefix+warning)
	}
	return warnings
}
