// Package syncpicker renders a multi-select worktree picker for `wtm sync`,
// letting the user choose which worktrees to rebase (or the base, to refresh it)
// and how a conflicting rebase should be handled.
package syncpicker

import (
	"errors"
	"fmt"
	"os"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/rules"
	"github.com/LucasPcq/wtm/internal/styles"
	"github.com/LucasPcq/wtm/internal/tui/components"
)

// Conflict-mode choices for the second wizard step.
const (
	conflictModeNormal = "normal"
	conflictModeKeep   = "keep"
)

// syncConfirm is the recap step's action value.
const syncConfirm = "sync"

const (
	parentModeFF   = "fast-forward"
	parentModeKeep = "keep"
)

// Step names, used for construction and value extraction.
const (
	stepWorktrees = "Worktrees"
	stepConflict  = "On conflict"
	stepParents   = "Parent branches"
	stepConfirm   = "Confirm"
)

// PlanPreviewParams carries the in-progress selection to the plan-preview hook.
type PlanPreviewParams struct {
	Branches []string
}

// RunParams holds the inputs required to render the picker.
type RunParams struct {
	// Statuses populate the multi-select when no branches are preselected.
	Statuses []domain.WorktreeStatus
	// Preselected fixes the worktrees to sync (branch args or --all) and skips the
	// multi-select. nil → show the multi-select picker.
	Preselected []string
	// DefaultKeepConflict pre-selects the "keep conflicts in progress" choice on
	// the conflict-mode step (e.g. when --keep-conflict was passed on the CLI).
	DefaultKeepConflict bool
	// KeepConflict is the conflict mode used when the on-conflict step is omitted
	// (SkipConfirm), taken from --keep-conflict.
	KeepConflict bool
	// StaleParents returns, for the worktrees currently checked, the parents the
	// cascade would rebase onto without refreshing and that a fast-forward would
	// actually advance. It is injected by the command layer (like PlanPreview) and
	// must be local-only: it is called synchronously while the wizard builds the
	// step. A nil hook omits the parent question entirely.
	StaleParents func([]string) []domain.ParentUpdate
	// ParentPreset settles the parent question from a flag instead of asking it.
	// The step is still installed and still appears in the summaries, naming the
	// flag that answered it — a flag must never make a line disappear from the
	// recap. nil means ask.
	ParentPreset *bool
	// PlanPreview renders the cascade preview shown on the confirmation step. It
	// is injected by the command layer so the picker (TUI) needs no dependency on
	// the service/output packages. Given the checked branches it returns the
	// plain plan text and the number of rebase steps. A nil hook omits the
	// confirmation step (same effect as SkipConfirm).
	PlanPreview func(PlanPreviewParams) (string, int, error)
	// SkipConfirm omits the on-conflict and confirmation steps (e.g. --dry-run or
	// --yes): the wizard then only collects the selection (when needed).
	SkipConfirm bool
}

// RunResult is the picker outcome: the chosen worktrees, the conflict mode, and
// whether the user accepted the plan on the confirmation step.
type RunResult struct {
	Branches     []string
	KeepConflict bool
	// FastForwardParents is false when the question was never asked.
	FastForwardParents bool
	// Confirmed reports whether the user accepted the plan. It is true when the
	// confirmation step was elided (SkipConfirm or no PlanPreview).
	Confirmed bool
}

// Run shows the sync picker — a worktree multi-select, a conflict-handling
// choice, and (unless SkipConfirm) a plan confirmation — and returns the checked
// branches, the conflict mode, and whether the plan was accepted. Returns
// domain.ErrUserAborted when the user cancels (Esc on the first step).
func Run(params RunParams) (RunResult, error) {
	// The picker may be reached through a shell wrapper that captures stdout, so
	// force lipgloss to detect color against stderr (the TTY).
	styles.UseRendererOn(os.Stderr)

	hasSelect := params.Preselected == nil

	// Set when the parent step is installed; it reports whether that step was
	// actually shown rather than auto-skipped.
	var parentShown *bool

	var steps []components.Step
	if hasSelect {
		steps = append(steps, selectStep(params.Statuses))
	}

	showConfirm := !params.SkipConfirm && params.PlanPreview != nil
	if showConfirm {
		// The on-conflict choice always precedes the recap, so the recap is never the
		// first step (its async plan preview is fired from OnEnter, which the wizard
		// does not run for step 0).
		steps = append(steps, conflictStep(conflictStepParams{
			HasSelect:   hasSelect,
			Preselected: params.Preselected,
			DefaultKeep: params.DefaultKeepConflict,
		}))
		if params.StaleParents != nil {
			step, shown := parentStep(parentStepParams{
				Preselected: params.Preselected,
				Stale:       params.StaleParents,
				Preset:      params.ParentPreset,
			})
			parentShown = shown
			steps = append(steps, step)
		}
	}

	// Nothing to ask (preselected worktrees + confirmation skipped): treat as an
	// accepted plan so the command syncs straight away.
	if len(steps) == 0 {
		return RunResult{Branches: params.Preselected, KeepConflict: params.KeepConflict, Confirmed: true}, nil
	}

	var previewErr error
	confirmIdx := len(steps)
	if showConfirm {
		// The plan preview walks every selected worktree's history, so it can take a
		// moment on a large tree. Compute it asynchronously when the step is entered
		// (OnEnter) behind the loading spinner, rather than blocking the render in a
		// Build hook. The placeholder confirm's Enter is guarded while loading so the
		// user cannot accept a plan they have not seen yet.
		steps = append(steps, components.Step{
			Name:  stepConfirm,
			Model: planPlaceholder(),
			Recap: true,
			OnEnter: func(prev []components.Step) tea.Cmd {
				branches := resolveBranches(prev, params.Preselected)
				keep := resolveKeep(prev, params.KeepConflict)
				return func() tea.Msg { return planRequestMsg{branches: branches, keepConflict: keep} }
			},
		})
	}

	final, err := components.RunWizard(components.RunWizardParams{
		Steps:    steps,
		Stderr:   true,
		ErrLabel: "sync picker",
		OnMsg: func(w *components.WizardModel, msg tea.Msg) (tea.Cmd, bool) {
			switch m := msg.(type) {
			case planRequestMsg:
				// Clear the plan left from a previous selection so the spinner isn't
				// shown over a stale preview while the new plan computes.
				w.UpdateStepModel(confirmIdx, func(any) any { return planPlaceholder() })
				loadCmd := w.StartLoading(domain.SyncPlanComputing)
				return tea.Batch(loadCmd, runPlanCmd(m, params.PlanPreview)), true
			case planDoneMsg:
				previewErr = m.err
				text := m.text
				if m.err != nil {
					text = "Failed to build sync plan: " + m.err.Error()
				}
				w.UpdateStepModel(confirmIdx, func(any) any {
					return confirmStep(confirmStepParams{PlanText: text, Count: m.count, KeepConflict: m.keepConflict})
				})
				w.SetLoading(false)
				return nil, true
			}
			// Block a premature Enter while the plan is still computing.
			if w.Loading() {
				if key, ok := msg.(tea.KeyMsg); ok && key.String() == "enter" {
					return nil, true
				}
			}
			return nil, false
		},
	})
	if err != nil {
		return RunResult{}, err
	}
	if previewErr != nil {
		return RunResult{}, previewErr
	}

	finalSteps := final.Steps()

	branches := params.Preselected
	if hasSelect {
		ms, ok := finalSteps[0].Model.(components.MultiSelectModel)
		if !ok {
			return RunResult{}, errors.New("unexpected model type")
		}
		branches = ms.Values()
	}

	result := RunResult{
		Branches:     branches,
		KeepConflict: resolveKeep(finalSteps, params.KeepConflict),
		FastForwardParents: resolveParentAnswer(resolveParentAnswerParams{
			Steps:  finalSteps,
			Shown:  parentShown,
			Preset: params.ParentPreset,
		}),
		Confirmed: true,
	}
	if showConfirm {
		if v, ok := stepSelectValue(finalSteps, stepConfirm); ok {
			result.Confirmed = v == syncConfirm
		}
	}
	return result, nil
}

// planRequestMsg asks the message handler to compute the sync plan for the
// current selection; planDoneMsg carries the rendered plan back.
type (
	planRequestMsg struct {
		branches     []string
		keepConflict bool
	}
	planDoneMsg struct {
		text         string
		count        int
		keepConflict bool
		err          error
	}
)

// planPlaceholder is the recap step shown while the plan preview computes: it has
// no options (so Enter is a no-op) and carries no stale plan; the loading check in
// OnMsg also guards Enter.
func planPlaceholder() components.SelectListModel {
	return components.NewSelectList(components.NewSelectListParams{
		Description: domain.SyncPlanComputing,
	})
}

// runPlanCmd computes the plan preview off the UI goroutine so the spinner
// animates while it runs.
func runPlanCmd(req planRequestMsg, preview func(PlanPreviewParams) (string, int, error)) tea.Cmd {
	return func() tea.Msg {
		text, count, err := preview(PlanPreviewParams{Branches: req.branches})
		return planDoneMsg{text: text, count: count, keepConflict: req.keepConflict, err: err}
	}
}

// selectStep builds the worktree multi-select (shown only when no branches were
// preselected via args or --all).
func selectStep(statuses []domain.WorktreeStatus) components.Step {
	items := make([]components.MultiSelectItem, 0, len(statuses))
	for _, status := range statuses {
		items = append(items, components.MultiSelectItem{
			Label: label(status),
			Value: status.Branch,
		})
	}
	return components.Step{
		Name: stepWorktrees,
		Model: components.NewMultiSelect(components.NewMultiSelectParams{
			Title:       "Select worktrees to sync",
			Description: domain.MultiSelectHint,
			Items:       items,
			Validate: func(selected []string) error {
				if len(selected) == 0 {
					return errors.New("select at least one worktree")
				}
				return nil
			},
		}),
		Summary: worktreesSummary,
	}
}

// worktreesSummary lists the selected worktrees under the breadcrumb, capping the
// list at 5 names with a "+N" tail so a large selection does not overflow the line.
func worktreesSummary(model any) string {
	ms, ok := model.(components.MultiSelectModel)
	if !ok {
		return ""
	}
	vals := ms.Values()
	if len(vals) == 0 {
		return "none"
	}
	const maxNames = 5
	if len(vals) <= maxNames {
		return strings.Join(vals, ", ")
	}
	return strings.Join(vals[:maxNames], ", ") + fmt.Sprintf(" +%d", len(vals)-maxNames)
}

// conflictSummary labels the on-conflict choice in the completed-step summaries.
func conflictSummary(model any) string {
	sl, ok := model.(components.SelectListModel)
	if !ok {
		return ""
	}
	if sl.Value() == conflictModeKeep {
		return "keep conflicts in progress"
	}
	return "sync normally"
}

// conflictStepParams holds inputs for conflictStep.
type conflictStepParams struct {
	// HasSelect reports whether a multi-select precedes this step. When false this
	// step is first, so its model is built concretely from Preselected (the wizard
	// never runs Build for step 0).
	HasSelect   bool
	Preselected []string
	DefaultKeep bool
}

// conflictStep builds the on-conflict decision step, deriving its worktree counter
// from the multi-select selection (or the preselected branches when it is first).
func conflictStep(p conflictStepParams) components.Step {
	step := components.Step{Name: stepConflict, Summary: conflictSummary}
	if p.HasSelect {
		step.Model = conflictModeStep(conflictModeStepParams{DefaultKeep: p.DefaultKeep})
		step.Build = func(prev []components.Step) any {
			return conflictModeStep(conflictModeStepParams{
				Selected:    resolveBranches(prev, p.Preselected),
				DefaultKeep: p.DefaultKeep,
			})
		}
		return step
	}
	step.Model = conflictModeStep(conflictModeStepParams{
		Selected:    p.Preselected,
		DefaultKeep: p.DefaultKeep,
	})
	return step
}

// conflictModeStepParams holds inputs for conflictModeStep.
type conflictModeStepParams struct {
	Selected    []string
	DefaultKeep bool
}

// conflictModeStep builds the conflict-handling choice. The description shows a
// compact counter of the selected worktrees so the decision is made in context
// without re-listing them (the full list is shown on the confirmation step); the
// "keep" option leads (and is highlighted, being the danger row) when DefaultKeep
// is set so a prior --keep-conflict is pre-selected.
func conflictModeStep(params conflictModeStepParams) components.SelectListModel {
	desc := "Choose what happens when a rebase hits a conflict."
	if len(params.Selected) > 0 {
		desc = syncCounter(len(params.Selected)) + "\n\n" + desc
	}

	normalItem := components.SelectItem{
		Label: "Sync normally — abort & keep worktrees clean on conflict",
		Value: conflictModeNormal,
	}
	keepItem := components.SelectItem{
		Label:  "Keep conflicts in progress — leave the rebase in its worktree for manual resolution",
		Value:  conflictModeKeep,
		Danger: true,
	}

	items := []components.SelectItem{normalItem, keepItem}
	if params.DefaultKeep {
		items = []components.SelectItem{keepItem, normalItem}
	}

	return components.NewSelectList(components.NewSelectListParams{
		Title:       "On conflict",
		Description: desc,
		Items:       items,
	})
}

type parentStepParams struct {
	Preselected []string
	Stale       func([]string) []domain.ParentUpdate
	// Preset settles the answer from a flag; the step is then shown as a skipped
	// summary line rather than asked.
	Preset *bool
}

// parentStep lives inside the wizard rather than after it, so it gets a
// breadcrumb and back navigation, and auto-skips when the selection leaves no
// stale parent. The returned pointer reports whether it was shown: a skipped step
// keeps the model Build left behind, whose first option would otherwise read back
// as an answer nobody gave.
func parentStep(p parentStepParams) (components.Step, *bool) {
	// Cached across Build → AutoSkip/SkipReason within one advance pass, like
	// components.ConfirmStep does: buildStep runs Build immediately before AutoSkip.
	relevant := false
	build := func(prev []components.Step) any {
		stale := p.Stale(resolveBranches(prev, p.Preselected))
		relevant = len(stale) > 0
		return parentModeStep(stale)
	}

	// A preset never asks, but it is still listed: a run that will advance a parent
	// must say so even when a flag decided it.
	asked := func() bool { return relevant && p.Preset == nil }

	return components.Step{
		Name:       stepParents,
		Model:      parentModeStep(nil),
		Build:      build,
		AutoSkip:   func(components.WizardModel) bool { return !asked() },
		SkipReason: func() string { return presetReason(relevant, p.Preset) },
		Summary:    parentSummary,
	}, &relevant
}

// presetReason names the outcome and the flag responsible. Empty when there was
// nothing to decide, which keeps the step hidden instead of listing a non-question.
func presetReason(relevant bool, preset *bool) string {
	if !relevant || preset == nil {
		return ""
	}
	if *preset {
		return "fast-forward (--" + domain.FlagFFParents + ")"
	}
	return "left as they are (--" + domain.FlagNoFFParents + ")"
}

type resolveParentAnswerParams struct {
	Steps  []components.Step
	Shown  *bool
	Preset *bool
}

// resolveParentAnswer reads the parent decision: the preset when a flag settled
// it, the step's own choice when it was asked, and false when nothing was stale.
func resolveParentAnswer(params resolveParentAnswerParams) bool {
	if params.Shown == nil || !*params.Shown {
		return false
	}
	if params.Preset != nil {
		return *params.Preset
	}
	return resolveParentFF(params.Steps)
}

// parentModeStep builds the parent-refresh choice. Like the on-conflict step it
// leads with the concrete situation — which parent is how far behind, and which
// worktrees rebase onto it — then offers two explicit outcomes. Refreshing leads:
// it cannot lose commits, and a stale parent is the outcome being warned about.
func parentModeStep(stale []domain.ParentUpdate) components.SelectListModel {
	desc := domain.SyncParentDescription
	if len(stale) > 0 {
		desc = parentLines(stale) + "\n\n" + desc
	}

	return components.NewSelectList(components.NewSelectListParams{
		Title:       stepParents,
		Description: desc,
		Items: []components.SelectItem{
			{
				Label: "Fast-forward them — rebase onto the up-to-date parent",
				Value: parentModeFF,
			},
			{
				Label: "Leave them as they are — rebase onto the parent as it stands today",
				Value: parentModeKeep,
			},
		},
	})
}

func parentLines(parents []domain.ParentUpdate) string {
	lines := make([]string, 0, len(parents))
	for _, parent := range parents {
		lines = append(lines, fmt.Sprintf("%s is %s behind %s%s — %s rebase onto it.",
			parent.Branch, rules.CommitCountLabel(parent.Behind),
			domain.RemoteBranchPrefix, parent.Branch,
			strings.Join(parent.Children, ", ")))
	}
	return strings.Join(lines, "\n")
}

func parentSummary(model any) string {
	sl, ok := model.(components.SelectListModel)
	if !ok {
		return ""
	}
	if sl.Value() == parentModeFF {
		return "fast-forward"
	}
	return "leave as they are"
}

func resolveParentFF(steps []components.Step) bool {
	sl, ok := stepModelByName(steps, stepParents).(components.SelectListModel)
	return ok && sl.Value() == parentModeFF
}

// resolveBranches reads the worktrees to sync: the multi-select selection when it
// is present, else the preselected branches (args / --all).
func resolveBranches(steps []components.Step, preselected []string) []string {
	if ms, ok := stepModelByName(steps, stepWorktrees).(components.MultiSelectModel); ok {
		return ms.Values()
	}
	return preselected
}

// resolveKeep reads whether conflicts are kept in progress: the on-conflict step's
// choice when present, else the fallback (from --keep-conflict).
func resolveKeep(steps []components.Step, fallback bool) bool {
	if sl, ok := stepModelByName(steps, stepConflict).(components.SelectListModel); ok {
		return sl.Value() == conflictModeKeep
	}
	return fallback
}

// stepModelByName returns the model of the named step, or nil when absent.
func stepModelByName(steps []components.Step, name string) any {
	for _, s := range steps {
		if s.Name == name {
			return s.Model
		}
	}
	return nil
}

// stepSelectValue reads the chosen value of the named SelectList step.
func stepSelectValue(steps []components.Step, name string) (string, bool) {
	if sl, ok := stepModelByName(steps, name).(components.SelectListModel); ok {
		return sl.Value(), true
	}
	return "", false
}

// syncCounter renders the compact "About to sync N worktree(s)…" line shown on
// the conflict step. It names no branch: each worktree is rebased onto its own
// recorded parent, which the base branch only coincides with at the first level.
// The base is shown where it is exact, in the plan recap's header.
func syncCounter(count int) string {
	return fmt.Sprintf("About to sync %d worktree(s) onto their parent.", count)
}

// confirmStepParams holds inputs for confirmStep.
type confirmStepParams struct {
	PlanText     string
	Count        int
	KeepConflict bool
}

// confirmStep builds the plan-confirmation recap (the wizard's final step). The
// description carries the plain plan preview followed by the confirmation
// question, with the keep-conflict danger folded in as a ⚠ line, then offers
// "Yes, sync" and the constant "No, cancel". The plan text must be plain (see
// output.SprintSyncPlan).
func confirmStep(params confirmStepParams) components.SelectListModel {
	return components.NewSelectList(components.NewSelectListParams{
		Description: confirmDescription(params),
		Items: []components.SelectItem{
			{Label: "Yes, sync", Value: syncConfirm},
			{Separator: true},
			{Label: domain.WizardCancelLabel, Value: domain.WizardCancelValue},
		},
	})
}

// confirmDescription builds the recap body: the plan preview, the confirmation
// question, and — when conflicts are kept in progress — the danger ⚠ line.
func confirmDescription(params confirmStepParams) string {
	question := fmt.Sprintf(domain.SyncConfirmPrompt, params.Count)
	desc := question
	if params.PlanText != "" {
		desc = params.PlanText + "\n\n" + question
	}
	if params.KeepConflict {
		desc += "\n\n⚠ " + domain.SyncKeepConflictWarning
	}
	return desc
}

// label decorates a worktree branch with a short suffix so the user can tell the
// base apart and spot worktrees that sync will skip.
func label(status domain.WorktreeStatus) string {
	switch {
	case status.IsParent:
		return status.Branch + " (base)"
	case status.RebaseInProgress:
		return status.Branch + " (rebasing)"
	case status.IsDirty:
		return status.Branch + " (dirty)"
	default:
		return status.Branch
	}
}
