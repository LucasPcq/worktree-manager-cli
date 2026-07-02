package clean

import (
	"fmt"
	"os"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/rules"
	"github.com/LucasPcq/wtm/internal/styles"
	"github.com/LucasPcq/wtm/internal/tui/components"
)

// Delete-step choices.
const (
	deleteYes   = "yes"
	deleteForce = "force"
	deleteNo    = "no"
)

// Step names, used for construction and value extraction.
const (
	stepPicker   = "Worktree"
	stepDelete   = "Delete"
	stepReparent = "Reparent children"
)

// CheckFunc runs the pre-deletion safety check for a branch. It swallows errors
// into a zero result — doClean is the idempotent safety net for a worktree that
// vanished or is the parent.
type CheckFunc func(branch string) domain.CleanCheckResult

// ReparentPreviewFunc returns the children a clean of branch would orphan.
type ReparentPreviewFunc func(branch string) domain.CleanReparentPlan

// RunWizardParams holds inputs for the unified clean wizard.
type RunWizardParams struct {
	ProjectDir string
	// PreselectedBranch skips the picker (a branch was given as an argument); the
	// safety check is then supplied via PreCheck.
	PreselectedBranch string
	PreCheck          *domain.CleanCheckResult
	// Check runs the safety check asynchronously when the delete step is entered,
	// for a branch chosen in the picker (unused when PreselectedBranch is set).
	Check           CheckFunc
	ReparentPreview ReparentPreviewFunc
}

// RunResult is the wizard outcome.
type RunResult struct {
	Branch string
	Force  bool
	// ReparentAsked is true when the reparent step was shown; ReparentChildren then
	// holds the answer.
	ReparentAsked    bool
	ReparentChildren bool
}

// checkRequestMsg asks the message handler to run the safety check for a branch;
// checkDoneMsg carries its result back.
type (
	checkRequestMsg struct{ branch string }
	checkDoneMsg    struct{ check domain.CleanCheckResult }
)

// RunWizard runs the full clean flow in one wizard: worktree picker (when no
// branch was given) → delete confirmation → reparent confirmation. The safety
// check for a picked worktree queries the PR state over the network, so it runs
// asynchronously behind the shared loading spinner. Esc on the delete or reparent
// step goes back a step instead of aborting the whole clean. Returns
// domain.ErrUserAborted on Esc at the first step or the explicit "No, cancel".
func RunWizard(params RunWizardParams) (RunResult, error) {
	// The picker may be reached through a shell wrapper that captures stdout, so
	// force lipgloss to detect color against stderr (the TTY).
	styles.UseRendererOn(os.Stderr)

	hasPicker := params.PreselectedBranch == ""
	var steps []components.Step

	if hasPicker {
		items, err := pickableItems(params.ProjectDir)
		if err != nil {
			return RunResult{}, err
		}
		steps = append(steps, components.Step{
			Name: stepPicker,
			Model: components.NewSelectList(components.NewSelectListParams{
				Title:       "Select worktree to clean",
				Description: "The parent worktree cannot be cleaned",
				Items:       items,
			}),
			Summary: components.SelectSummary,
		})
	}

	deleteIdx := len(steps)
	if hasPicker {
		// Empty placeholder: Enter is a no-op until the async check replaces it with
		// the real options, so the user can't confirm before seeing the warnings.
		steps = append(steps, components.Step{
			Name:    stepDelete,
			Model:   deletePlaceholder(),
			Summary: components.SelectSummary,
			OnEnter: func(prev []components.Step) tea.Cmd {
				branch := pickerValue(prev)
				return func() tea.Msg { return checkRequestMsg{branch: branch} }
			},
		})
	} else {
		check := domain.CleanCheckResult{}
		if params.PreCheck != nil {
			check = *params.PreCheck
		}
		steps = append(steps, components.Step{
			Name:    stepDelete,
			Model:   deleteStep(check),
			Summary: components.SelectSummary,
		})
	}

	steps = append(steps, reparentStep(reparentStepParams{
		Preview:     params.ReparentPreview,
		Preselected: params.PreselectedBranch,
	}))

	final, err := components.RunWizard(components.RunWizardParams{
		Steps:    steps,
		Stderr:   true,
		ErrLabel: "clean wizard",
		OnMsg: func(w *components.WizardModel, msg tea.Msg) (tea.Cmd, bool) {
			switch m := msg.(type) {
			case checkRequestMsg:
				// Clear any options left from a previously-picked worktree so the
				// spinner isn't shown over stale buttons while the new check runs.
				w.UpdateStepModel(deleteIdx, func(any) any { return deletePlaceholder() })
				loadCmd := w.StartLoading("Checking worktree…")
				return tea.Batch(loadCmd, runCheckCmd(m.branch, params.Check)), true
			case checkDoneMsg:
				result := m.check
				w.UpdateStepModel(deleteIdx, func(any) any { return deleteStep(result) })
				w.SetLoading(false)
				return nil, true
			}
			return nil, false
		},
	})
	if err != nil {
		return RunResult{}, err
	}

	finalSteps := final.Steps()
	deleteSel, ok := finalSteps[deleteIdx].Model.(components.SelectListModel)
	if !ok {
		return RunResult{}, fmt.Errorf("unexpected model type")
	}
	if deleteSel.Value() == deleteNo {
		return RunResult{}, domain.ErrUserAborted
	}

	result := RunResult{
		Branch: params.PreselectedBranch,
		Force:  deleteSel.Value() == deleteForce,
	}
	if hasPicker {
		result.Branch = pickerValue(finalSteps)
	}

	reparentIdx := len(finalSteps) - 1
	if !final.Skipped(reparentIdx) {
		if rc, ok := finalSteps[reparentIdx].Model.(components.ConfirmModel); ok {
			result.ReparentAsked = true
			result.ReparentChildren = rc.Confirmed()
		}
	}
	return result, nil
}

// runCheckCmd runs the safety check off the UI goroutine so the spinner animates.
func runCheckCmd(branch string, check CheckFunc) tea.Cmd {
	return func() tea.Msg { return checkDoneMsg{check: check(branch)} }
}

// deletePlaceholder is the empty delete step shown while the async safety check
// runs: with no items, Enter is a no-op, so the user cannot confirm before the
// real options (with any warnings) replace it.
func deletePlaceholder() components.SelectListModel {
	return components.NewSelectList(components.NewSelectListParams{Title: "Proceed with deletion?"})
}

// deleteStep builds the delete confirmation, recapping what will be removed and
// offering a danger "force" option only when the worktree is unsafe (dirty,
// unpushed commits, or an open PR).
func deleteStep(check domain.CleanCheckResult) components.SelectListModel {
	items := []components.SelectItem{{Label: "Yes, delete", Value: deleteYes}}
	if rules.HasWarnings(check) {
		items = append(items,
			components.SelectItem{Separator: true},
			components.SelectItem{Label: "Yes, force delete (bypass all checks)", Value: deleteForce, Danger: true},
		)
	}
	items = append(items,
		components.SelectItem{Separator: true},
		components.SelectItem{Label: "No, cancel", Value: deleteNo},
	)

	return components.NewSelectList(components.NewSelectListParams{
		Title:       "Proceed with deletion?",
		Description: deleteDescription(check),
		Items:       items,
	})
}

// deleteDescription recaps the warnings and what the delete will remove.
func deleteDescription(check domain.CleanCheckResult) string {
	var lines []string
	if check.IsDirty {
		lines = append(lines, "Worktree has uncommitted changes")
	}
	if check.UnpushedCommits > 0 {
		lines = append(lines, fmt.Sprintf("%d commit(s) not pushed to remote", check.UnpushedCommits))
	}
	if check.HasOpenPR {
		lines = append(lines, "Open PR: "+check.PRUrl)
	}
	if len(lines) > 0 {
		lines = append(lines, "")
	}
	lines = append(lines,
		"Will delete:",
		"  worktree  "+check.WorktreePath,
		"  branch    "+check.Branch,
	)
	return strings.Join(lines, "\n")
}

// reparentStepParams holds inputs for reparentStep.
type reparentStepParams struct {
	Preview     ReparentPreviewFunc
	Preselected string
}

// reparentStep builds the conditional reparent confirmation. It derives the
// orphaned children from the chosen branch and is skipped when the delete was
// cancelled or there are no children. Hosted in the wizard so Esc returns to the
// delete step instead of aborting the clean.
func reparentStep(params reparentStepParams) components.Step {
	return components.ConfirmStep(components.ConfirmStepParams{
		Name:     stepReparent,
		YesLabel: "reparent",
		NoLabel:  "leave orphaned",
		Decide: func(prev []components.Step) (bool, components.NewConfirmParams) {
			if deleteChoice(prev) == deleteNo {
				return false, components.NewConfirmParams{}
			}
			plan := params.Preview(branchFromPrev(prev, params.Preselected))
			if len(plan.Children) == 0 {
				return false, components.NewConfirmParams{}
			}
			return true, components.NewConfirmParams{
				Title:       fmt.Sprintf(domain.CleanReparentPrompt, len(plan.Children), plan.Grandparent),
				Description: reparentProposalText(plan),
				DefaultYes:  true,
			}
		},
	})
}

// pickerValue reads the branch chosen on the picker step.
func pickerValue(steps []components.Step) string {
	return stepValue(steps, stepPicker)
}

// deleteChoice reads the value chosen on the delete step.
func deleteChoice(steps []components.Step) string {
	return stepValue(steps, stepDelete)
}

// branchFromPrev resolves the branch being cleaned: the picker choice, else the
// preselected argument branch.
func branchFromPrev(steps []components.Step, preselected string) string {
	if v := stepValue(steps, stepPicker); v != "" {
		return v
	}
	return preselected
}

func stepValue(steps []components.Step, name string) string {
	for _, s := range steps {
		if s.Name != name {
			continue
		}
		if sl, ok := s.Model.(components.SelectListModel); ok {
			return sl.Value()
		}
	}
	return ""
}

// reparentProposalText lists the children a clean would orphan as the reparent
// confirmation's description (the wizard applies its own muted style).
func reparentProposalText(plan domain.CleanReparentPlan) string {
	lines := make([]string, 0, len(plan.Children)+1)
	lines = append(lines, domain.CleanReparentIntro)
	for _, c := range plan.Children {
		lines = append(lines, fmt.Sprintf("  %s: %s → %s", c.Branch, c.OldParent, c.NewParent))
	}
	return strings.Join(lines, "\n")
}
