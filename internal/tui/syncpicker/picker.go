// Package syncpicker renders a multi-select worktree picker for `wtm sync`,
// letting the user choose which worktrees to rebase (or the base, to refresh it)
// and how a conflicting rebase should be handled.
package syncpicker

import (
	"errors"
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/styles"
	"github.com/LucasPcq/wtm/internal/tui/components"
)

// Conflict-mode choices for the second wizard step.
const (
	conflictModeNormal = "normal"
	conflictModeKeep   = "keep"
)

// PlanPreviewParams carries the in-progress selection to the plan-preview hook.
type PlanPreviewParams struct {
	Branches []string
}

// RunParams holds the inputs required to render the picker.
type RunParams struct {
	Statuses []domain.WorktreeStatus
	// DefaultKeepConflict pre-selects the "keep conflicts in progress" choice on
	// the conflict-mode step (e.g. when --keep-conflict was passed on the CLI).
	DefaultKeepConflict bool
	// BaseBranch is the branch the cascade rebases onto; shown in the conflict
	// step's compact counter.
	BaseBranch string
	// PlanPreview renders the cascade preview shown on the confirmation step. It
	// is injected by the command layer so the picker (TUI) needs no dependency on
	// the service/output packages. Given the checked branches it returns the
	// plain plan text and the number of rebase steps. A nil hook omits the
	// confirmation step (same effect as SkipConfirm).
	PlanPreview func(PlanPreviewParams) (string, int, error)
	// SkipConfirm omits the confirmation step (e.g. --dry-run or --yes), matching
	// the non-interactive flow where no confirmation is shown before acting.
	SkipConfirm bool
}

// RunResult is the picker outcome: the chosen worktrees, the conflict mode, and
// whether the user accepted the plan on the confirmation step.
type RunResult struct {
	Branches     []string
	KeepConflict bool
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

	items := make([]components.MultiSelectItem, 0, len(params.Statuses))
	for _, status := range params.Statuses {
		items = append(items, components.MultiSelectItem{
			Label: label(status),
			Value: status.Branch,
		})
	}

	ms := components.NewMultiSelect(components.NewMultiSelectParams{
		Title:       "Select worktrees to sync",
		Description: "Space to toggle, a to select all, / to filter, enter to confirm, esc to cancel.",
		Items:       items,
		Validate: func(selected []string) error {
			if len(selected) == 0 {
				return errors.New("select at least one worktree")
			}
			return nil
		},
	})

	steps := []components.Step{
		{Name: "Worktrees", Model: ms},
		{
			Name:  "On conflict",
			Model: conflictModeStep(conflictModeStepParams{DefaultKeep: params.DefaultKeepConflict, BaseBranch: params.BaseBranch}),
			Build: func(prev []components.Step) any {
				return conflictModeStep(conflictModeStepParams{Prev: prev, DefaultKeep: params.DefaultKeepConflict, BaseBranch: params.BaseBranch})
			},
		},
	}

	showConfirm := !params.SkipConfirm && params.PlanPreview != nil
	var previewErr error
	confirmIdx := len(steps)
	if showConfirm {
		// The plan preview walks every selected worktree's history, so it can take a
		// moment on a large tree. Compute it asynchronously when the step is entered
		// (OnEnter) behind the loading spinner, rather than blocking the render in a
		// Build hook. The placeholder confirm's Enter is guarded while loading so the
		// user cannot accept a plan they have not seen yet.
		steps = append(steps, components.Step{
			Name:  "Confirm",
			Model: planPlaceholder(),
			OnEnter: func(prev []components.Step) tea.Cmd {
				branches := selectedBranches(prev)
				keep := keepConflictFromPrev(prev)
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
	msModel, ok := finalSteps[0].Model.(components.MultiSelectModel)
	if !ok {
		return RunResult{}, errors.New("unexpected model type")
	}
	modeModel, ok := finalSteps[1].Model.(components.SelectListModel)
	if !ok {
		return RunResult{}, errors.New("unexpected model type")
	}

	result := RunResult{
		Branches:     msModel.Values(),
		KeepConflict: modeModel.Value() == conflictModeKeep,
		Confirmed:    true,
	}
	if showConfirm {
		confirmModel, ok := finalSteps[2].Model.(components.ConfirmModel)
		if !ok {
			return RunResult{}, errors.New("unexpected model type")
		}
		result.Confirmed = confirmModel.Confirmed()
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

// planPlaceholder is the confirm step shown while the plan preview computes: its
// Enter is guarded by the loading check in OnMsg, and it carries no stale plan.
func planPlaceholder() components.ConfirmModel {
	return components.NewConfirm(components.NewConfirmParams{
		DefaultYes:  true,
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

// conflictModeStepParams holds inputs for conflictModeStep.
type conflictModeStepParams struct {
	Prev        []components.Step
	DefaultKeep bool
	BaseBranch  string
}

// conflictModeStep builds the conflict-handling choice. The description shows a
// compact counter of the worktrees selected in the prior step so the decision is
// made in context without re-listing them (the full list is shown on the
// confirmation step); the "keep" option leads (and is highlighted, being the
// danger row) when DefaultKeep is set so a prior --keep-conflict is pre-selected.
func conflictModeStep(params conflictModeStepParams) components.SelectListModel {
	desc := "Choose what happens when a rebase hits a conflict."
	if selected := selectedBranches(params.Prev); len(selected) > 0 {
		desc = syncCounter(len(selected), params.BaseBranch) + "\n\n" + desc
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

// selectedBranches reads the worktrees checked in the first (multi-select) step.
func selectedBranches(prev []components.Step) []string {
	if len(prev) == 0 {
		return nil
	}
	ms, ok := prev[0].Model.(components.MultiSelectModel)
	if !ok {
		return nil
	}
	return ms.Values()
}

// keepConflictFromPrev reads whether the conflict-mode step (the second step)
// chose to keep conflicts in progress.
func keepConflictFromPrev(prev []components.Step) bool {
	if len(prev) < 2 {
		return false
	}
	mode, ok := prev[1].Model.(components.SelectListModel)
	if !ok {
		return false
	}
	return mode.Value() == conflictModeKeep
}

// syncCounter renders the compact "About to sync N worktree(s)…" line shown on
// the conflict step, naming the base when known so the target is unambiguous.
func syncCounter(count int, base string) string {
	if base == "" {
		return fmt.Sprintf("About to sync %d worktree(s).", count)
	}
	return fmt.Sprintf("About to sync %d worktree(s) onto %s.", count, base)
}

// confirmStepParams holds inputs for confirmStep.
type confirmStepParams struct {
	PlanText     string
	Count        int
	KeepConflict bool
}

// confirmStep builds the plan-confirmation step (the wizard's final step). The
// description carries the plain plan preview followed by the confirmation
// question; the danger warning is shown when conflicts are kept in progress. The
// wizard renders the description in its muted style, so the plan text must be
// plain (see output.SprintSyncPlan).
func confirmStep(params confirmStepParams) components.ConfirmModel {
	question := fmt.Sprintf(domain.SyncConfirmPrompt, params.Count)
	desc := question
	if params.PlanText != "" {
		desc = params.PlanText + "\n\n" + question
	}
	warning := ""
	if params.KeepConflict {
		warning = domain.SyncKeepConflictWarning
	}
	return components.NewConfirm(components.NewConfirmParams{
		Description: desc,
		Warning:     warning,
		DefaultYes:  true,
	})
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
