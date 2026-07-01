// Package relocate renders the interactive wizard for `wtm relocate`: a parent
// picker per adopted worktree followed by a final recap + apply confirmation.
package relocate

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/tui/branchrefresh"
	"github.com/LucasPcq/wtm/internal/tui/components"
)

// RunParams holds the inputs for the relocate wizard.
type RunParams struct {
	ProjectDir string
	Plan       domain.RelocatePlan
	Adoptions  []domain.RelocateStep    // worktrees needing a parent (rules.PlanAdoptions)
	Branches   []domain.BranchCandidate // candidate parent branches (resolved by the command)
	BaseBranch string
}

// RunResult is the outcome of the wizard.
type RunResult struct {
	Confirmed bool
	Parents   map[string]string
}

// RunWizard drives the relocate confirmation flow: one parent picker per adopted
// worktree, then a final "Apply" step recapping the resolved actions. It returns
// the chosen parents and whether the user confirmed. An abort (Esc on the first
// step) or a "No" on the final step yields Confirmed=false.
func RunWizard(params RunParams) (RunResult, error) {
	holder := &params.Branches

	steps := make([]components.Step, 0, len(params.Adoptions)+1)
	for _, adoption := range params.Adoptions {
		steps = append(steps, parentStep(parentStepParams{
			Branch:     adoption.Branch,
			BaseBranch: params.BaseBranch,
			Holder:     holder,
		}))
	}
	steps = append(steps, applyStep(params))

	wiz := components.NewWizardWithParams(components.WizardParams{
		Steps:       steps,
		InitCmd:     branchrefresh.Cmd(params.ProjectDir),
		Loading:     true,
		LoadingText: domain.LoadingBranchesText,
		OnMsg:       branchrefresh.Handler(params.ProjectDir, holder),
	})
	finalModel, err := tea.NewProgram(wiz, tea.WithOutput(os.Stderr)).Run()
	if err != nil {
		return RunResult{}, fmt.Errorf("relocate wizard: %w", err)
	}

	final, ok := finalModel.(components.WizardModel)
	if !ok || final.Aborted() {
		return RunResult{Confirmed: false}, nil
	}

	done := final.Steps()
	return RunResult{
		Confirmed: lastStepConfirmed(done),
		Parents:   extractParents(done, params.Adoptions),
	}, nil
}

type parentStepParams struct {
	Branch     string
	BaseBranch string
	Holder     *[]domain.BranchCandidate
}

func parentStep(params parentStepParams) components.Step {
	build := func() any {
		return components.NewSelectList(components.NewSelectListParams{
			Title:       "Parent for " + params.Branch,
			Description: "Recorded as the worktree's parent so `wtm sync` can rebase it.",
			Items: components.BranchItems(components.BranchItemsParams{
				Candidates:   *params.Holder,
				Pinned:       params.BaseBranch,
				PinnedSuffix: "  (default)",
				Exclude:      params.Branch,
			}),
		})
	}

	return components.Step{
		Name:       "Parent for " + params.Branch,
		Model:      build(),
		Build:      func([]components.Step) any { return build() },
		CanRefresh: true,
		Summary:    components.SelectSummary,
	}
}

func applyStep(params RunParams) components.Step {
	step := components.Step{Name: "Apply"}
	if len(params.Adoptions) == 0 {
		// No pickers precede this step, so the wizard never calls Build (it skips
		// index 0). Build the recap directly from the plan.
		step.Model = newApplyConfirm(params.Plan, nil)
		return step
	}
	step.Build = func(prev []components.Step) any {
		return newApplyConfirm(params.Plan, extractParents(prev, params.Adoptions))
	}
	step.Model = newApplyConfirm(params.Plan, nil)
	return step
}

func newApplyConfirm(plan domain.RelocatePlan, parents map[string]string) components.ConfirmModel {
	return components.NewConfirm(components.NewConfirmParams{
		Title:       "Apply these changes?",
		Description: buildRecap(plan, parents),
		DefaultYes:  true,
	})
}

// buildRecap renders the resolved actions (with chosen parents) as a grouped,
// plain-text block shown above the final Yes/No.
func buildRecap(plan domain.RelocatePlan, parents map[string]string) string {
	var apply, skipped, blocked []string
	for _, step := range plan.Steps {
		switch step.Status {
		case domain.RelocateStatusMove, domain.RelocateStatusAdopt:
			apply = append(apply, recapApplyLine(plan.BasePath, step, parents))
		case domain.RelocateStatusSkippedDirty:
			skipped = append(skipped, step.Branch+" — uncommitted changes")
		case domain.RelocateStatusSkippedLocked:
			skipped = append(skipped, step.Branch+" — locked")
		case domain.RelocateStatusBlockedDest:
			blocked = append(blocked, step.Branch+" — target path occupied")
		}
	}

	var b strings.Builder
	writeRecapGroup(&b, "To apply:", apply)
	writeRecapGroup(&b, "Skipped:", skipped)
	writeRecapGroup(&b, "Blocked:", blocked)
	return strings.TrimRight(b.String(), "\n")
}

func writeRecapGroup(b *strings.Builder, title string, lines []string) {
	if len(lines) == 0 {
		return
	}
	if b.Len() > 0 {
		b.WriteString("\n")
	}
	b.WriteString(title)
	b.WriteString("\n")
	for _, line := range lines {
		b.WriteString("  " + line + "\n")
	}
}

func recapApplyLine(basePath string, step domain.RelocateStep, parents map[string]string) string {
	if step.Status == domain.RelocateStatusAdopt {
		return fmt.Sprintf("%s  adopt in place (parent: %s)", step.Branch, resolveParent(step, parents))
	}
	target := filepath.Join(basePath, filepath.Base(step.ToPath))
	if step.Adopt {
		return fmt.Sprintf("%s → %s (adopt, parent: %s)", step.Branch, target, resolveParent(step, parents))
	}
	return fmt.Sprintf("%s → %s", step.Branch, target)
}

func resolveParent(step domain.RelocateStep, parents map[string]string) string {
	if parent, ok := parents[step.Branch]; ok && parent != "" {
		return parent
	}
	return step.Parent
}

func extractParents(steps []components.Step, adoptions []domain.RelocateStep) map[string]string {
	parents := make(map[string]string, len(adoptions))
	for i, adoption := range adoptions {
		if i >= len(steps) {
			break
		}
		if sl, ok := steps[i].Model.(components.SelectListModel); ok {
			parents[adoption.Branch] = sl.Value()
		}
	}
	return parents
}

func lastStepConfirmed(steps []components.Step) bool {
	if len(steps) == 0 {
		return false
	}
	cm, ok := steps[len(steps)-1].Model.(components.ConfirmModel)
	return ok && cm.Confirmed()
}
