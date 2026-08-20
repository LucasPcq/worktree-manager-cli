package fastforward

import (
	"errors"
	"fmt"
	"strings"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/flow"
	"github.com/LucasPcq/wtm/internal/rules"
)

const (
	KeySelection = "fastforward.selection"
	KeyConfirm   = "fastforward.confirm"
)

const (
	confirmYes   = "fast-forward"
	confirmForce = "force"
)

const (
	labelSelection = "Worktrees"
	labelConfirm   = "Confirm"
)

func (f *fastForwardFlow) session() flow.Session {
	return flow.Session{
		ErrLabel: domain.FastForwardWizardErrLabel,
		Presets:  f.presetSelection(),
		Steps:    []flow.Step{f.selectionStep(), f.confirmStep()},
	}
}

// presetSelection settles the selection when args or --all already did, so the
// step is not asked but still reads back in the recap.
func (f *fastForwardFlow) presetSelection() flow.Answers {
	presets := flow.NewAnswers(nil)
	if f.request.All {
		return presets.WithValues(KeySelection, allBranches(f.statuses))
	}
	return presets.WithValues(KeySelection, f.selection)
}

func allBranches(statuses []domain.WorktreeStatus) []string {
	branches := make([]string, 0, len(statuses))
	for _, status := range statuses {
		branches = append(branches, status.Branch)
	}
	return branches
}

func (f *fastForwardFlow) selectionStep() flow.Step {
	return flow.Step{
		Kind:  flow.StepMultiSelect,
		Key:   KeySelection,
		Label: labelSelection,
		Build: func(flow.Answers) (flow.StepContent, error) {
			return flow.StepContent{
				Title:       domain.FastForwardSelectionTitle,
				Description: domain.MultiSelectHint,
				Options:     f.selectionOptions(),
			}, nil
		},
		ValidateSet: func(values []string) error {
			if len(values) == 0 {
				return errors.New(domain.FastForwardSelectAtLeastOne)
			}
			return nil
		},
		Resolve: func(flow.Answers) (flow.Answer, error) {
			return flow.Answer{}, fmt.Errorf(domain.FastForwardSelectionRequiredFmt,
				domain.FlagAll, domain.FlagYes, domain.FlagOutput, domain.OutputJSON)
		},
		Summarize: flow.SummarizeSet,
		Flag:      domain.FlagAll,
	}
}

// selectionOptions tags each worktree with its state against origin. The tags
// come from remote-tracking refs with no fetch, so they say what is known, not
// what is true: the confirm step fetches.
func (f *fastForwardFlow) selectionOptions() []flow.Option {
	prechecked := make(map[string]bool, len(f.request.Precheck))
	for _, name := range f.request.Precheck {
		prechecked[name] = true
	}

	options := make([]flow.Option, 0, len(f.statuses))
	for _, status := range f.statuses {
		tag, tone := originTag(status)
		options = append(options, flow.Option{
			Label:    status.Branch,
			Value:    status.Branch,
			Selected: prechecked[status.Branch],
			Tag:      tag,
			Tone:     tone,
		})
	}
	return options
}

func originTag(status domain.WorktreeStatus) (string, domain.Tone) {
	switch status.OriginState {
	case domain.DivergenceBehind:
		return rules.CommitCountLabel(status.OriginBehind) + " behind", domain.ToneWarning
	case domain.DivergenceDiverged:
		return domain.FastForwardLabelDiverged, domain.ToneWarning
	case domain.DivergenceUnknown:
		return domain.FastForwardLabelNoRemote, domain.ToneNeutral
	default:
		return domain.FastForwardLabelUpToDate, domain.ToneNeutral
	}
}

func (f *fastForwardFlow) confirmStep() flow.Step {
	return flow.Step{
		Kind:           flow.StepRecap,
		Key:            KeyConfirm,
		Label:          labelConfirm,
		Title:          domain.FastForwardConfirmTitle,
		LoadingMessage: domain.FastForwardCheckLoading,
		Load: func(answers flow.Answers) (flow.StepContent, error) {
			checks := f.checkAll(answers.Values(KeySelection))
			return flow.StepContent{
				Title:       domain.FastForwardConfirmTitle,
				Description: recap(checks),
				Options:     confirmOptions(checks),
				Blockers:    blockersOf(checks),
			}, nil
		},
		Resolve:   f.resolveConfirm,
		Summarize: func(flow.Answer) string { return domain.FastForwardOption },
	}
}

// resolveConfirm keeps the safety check while answering: --yes is the
// confirmation axis, not a licence to fast-forward something unsafe. Only
// --force lifts the refusal.
func (f *fastForwardFlow) resolveConfirm(answers flow.Answers) (flow.Answer, error) {
	if f.request.Force {
		return flow.Answer{Value: confirmForce}, nil
	}
	for _, check := range f.checkAll(answers.Values(KeySelection)) {
		blockers := rules.FastForwardBlockers(check)
		if len(blockers) == 0 {
			continue
		}
		return flow.Answer{}, fmt.Errorf(domain.FastForwardForceHintFmt,
			check.Branch, blockers[0].Label, domain.FlagForce)
	}
	return flow.Answer{Value: confirmYes}, nil
}

func (f *fastForwardFlow) checkAll(selected []string) []domain.FastForwardCheck {
	checks := make([]domain.FastForwardCheck, 0, len(selected))
	for _, name := range selected {
		checks = append(checks, f.check(name))
	}
	return checks
}

func confirmOptions(checks []domain.FastForwardCheck) []flow.Option {
	options := []flow.Option{{Label: domain.FastForwardOption, Value: confirmYes}}
	if len(blockersOf(checks)) == 0 {
		return options
	}
	return append(options,
		flow.Option{Separator: true},
		flow.Option{Label: domain.FastForwardAnywayOption, Value: confirmForce, Danger: true},
	)
}

func blockersOf(checks []domain.FastForwardCheck) []flow.Blocker {
	var blockers []flow.Blocker
	for _, check := range checks {
		for _, refusal := range rules.FastForwardBlockers(check) {
			blockers = append(blockers, flow.Blocker{
				Key:   check.Branch + ":" + refusal.Key,
				Label: check.Branch + ": " + refusal.Label,
			})
		}
	}
	return blockers
}

// recap names every selected branch, refusals included: a branch that silently
// vanished from a run the user asked for is worse than one that says why it
// stayed put.
func recap(checks []domain.FastForwardCheck) string {
	lines := make([]string, 0, len(checks))
	for _, check := range checks {
		lines = append(lines, recapLine(check))
	}
	return strings.Join(lines, "\n")
}

func recapLine(check domain.FastForwardCheck) string {
	if reason, refused := rules.FastForwardRefusal(check); refused {
		return reason
	}
	if !rules.FastForwardNeedsWork(check) {
		return fmt.Sprintf(domain.FastForwardUpToDateFmt, check.Branch)
	}
	return fmt.Sprintf(domain.FastForwardPlanFmt, check.Branch, check.Branch,
		rules.CommitCountLabel(check.Behind))
}
