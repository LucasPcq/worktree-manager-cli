// Package relocate renders the interactive wizard for `wtm relocate`: an opt-in
// base_path edit, a parent picker per adopted worktree, and a final recap + apply
// confirmation — all in a single wizard so the breadcrumb, step summaries, and back
// navigation work across the whole flow.
package relocate

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/tui/branchrefresh"
	"github.com/LucasPcq/wtm/internal/tui/components"
)

// relocateApply is the recap step's action value.
const relocateApply = "apply"

// Step names and base_path gate values.
const (
	stepBasePathGate  = "Base path"
	stepBasePathValue = "New base_path"
	stepApply         = "Apply"
	basePathKeep      = "keep"
	basePathChange    = "change"
)

// RunParams holds the inputs for the relocate wizard.
type RunParams struct {
	ProjectDir string
	Adoptions  []domain.RelocateStep    // worktrees needing a parent (rules.PlanAdoptions)
	Branches   []domain.BranchCandidate // candidate parent branches (resolved by the command)
	BaseBranch string
	// CurrentBasePath is the base_path in effect, pre-filled in the edit step and
	// returned unchanged when the user keeps it.
	CurrentBasePath string
	// EditBasePath includes the keep/change gate and value steps. False (e.g. when
	// --to already fixed base_path) drops them; the wizard then only picks parents.
	EditBasePath bool
	// HasWork reports whether the current base_path already has worktrees to move or
	// adopt. When false and the user keeps base_path (nothing changes), the recap is
	// auto-skipped so the wizard never asks to confirm a no-op.
	HasWork bool
	// ValidateBasePath validates a typed base_path. Provided by the command so the
	// TUI holds no decision logic (it forwards the rules validator).
	ValidateBasePath func(string) error
	// RecapText renders the recap body for the chosen base_path and adoption parents.
	// Provided by the command so the TUI holds no plan/format logic.
	RecapText func(basePath string, parents map[string]string) string
}

// RunResult is the outcome of the wizard. Aborted (Esc or "No, cancel") and
// Confirmed ("Yes, apply") are mutually exclusive; both false means the recap was
// auto-skipped because nothing would change — the command treats that as "nothing to
// do", not an abort.
type RunResult struct {
	Aborted   bool
	Confirmed bool
	BasePath  string
	Parents   map[string]string
}

// RunWizard drives the relocate flow: an opt-in base_path edit (a keep/change gate
// then a value step, auto-skipped when kept), one parent picker per adopted worktree,
// then a final "Apply" recap. It returns the chosen base_path, the adoption parents,
// and whether the user confirmed. An abort (Esc on the first step) or a "No" on the
// final step yields Confirmed=false.
func RunWizard(params RunParams) (RunResult, error) {
	holder := &params.Branches

	steps := make([]components.Step, 0, len(params.Adoptions)+3)
	if params.EditBasePath {
		steps = append(steps, basePathGateStep(params.CurrentBasePath))
		steps = append(steps, basePathValueStep(params))
	}
	for _, adoption := range params.Adoptions {
		steps = append(steps, parentStep(parentStepParams{
			Branch:     adoption.Branch,
			BaseBranch: params.BaseBranch,
			Holder:     holder,
		}))
	}
	steps = append(steps, recapStep(params))

	// The background branch refresh only feeds the parent pickers; skip it (and its
	// spinner) when there is nothing to adopt.
	wp := components.WizardParams{Steps: steps}
	if len(params.Adoptions) > 0 {
		wp.InitCmd = branchrefresh.Cmd(params.ProjectDir)
		wp.Loading = true
		wp.LoadingText = domain.LoadingBranchesText
		wp.OnMsg = branchrefresh.Handler(params.ProjectDir, holder)
	}
	finalModel, err := tea.NewProgram(components.NewWizardWithParams(wp), tea.WithOutput(os.Stderr)).Run()
	if err != nil {
		return RunResult{}, fmt.Errorf("relocate wizard: %w", err)
	}

	final, ok := finalModel.(components.WizardModel)
	if !ok || final.Aborted() {
		return RunResult{Aborted: true}, nil
	}

	done := final.Steps()
	result := RunResult{
		BasePath: resolveBasePath(done, params.CurrentBasePath),
		Parents:  extractParents(done, params.Adoptions),
	}
	// The recap is always the last step. When it was auto-skipped (nothing changes),
	// there is nothing to confirm — return neither Confirmed nor Aborted.
	if final.Skipped(len(done) - 1) {
		return result, nil
	}
	if !recapConfirmed(done) {
		return RunResult{Aborted: true}, nil
	}
	result.Confirmed = true
	return result, nil
}

// basePathGateStep asks whether to change base_path; keeping it auto-skips the value
// step so the common "just adopt or align" run is a single Enter.
func basePathGateStep(current string) components.Step {
	return components.Step{
		Name: stepBasePathGate,
		Model: components.NewSelectList(components.NewSelectListParams{
			Title:       "Change base_path?",
			Description: "Worktrees live under " + current + ". Keep it, or set a new location to move them all to.",
			Items: []components.SelectItem{
				{Label: "Keep " + current, Value: basePathKeep},
				{Label: "Change it", Value: basePathChange},
			},
		}),
		Summary: components.SelectSummary,
	}
}

func basePathValueStep(params RunParams) components.Step {
	build := func() any {
		return components.NewTextInput(components.NewTextInputParams{
			Title:       "New base_path",
			Description: "Relative to the repo root (e.g. ../.trees). Existing worktrees move here.",
			Placeholder: params.CurrentBasePath,
			Default:     params.CurrentBasePath,
			Validate:    params.ValidateBasePath,
		})
	}
	return components.Step{
		Name:     stepBasePathValue,
		Model:    build(),
		Build:    func([]components.Step) any { return build() },
		AutoSkip: func(w components.WizardModel) bool { return gateChoice(w.Steps()) != basePathChange },
		Summary:  components.TextSummary,
	}
}

func recapStep(params RunParams) components.Step {
	step := components.RecapStep(components.RecapStepParams{
		Name: stepApply,
		Build: func(prev []components.Step) components.RecapContent {
			basePath := resolveBasePath(prev, params.CurrentBasePath)
			parents := extractParents(prev, params.Adoptions)
			return components.RecapContent{
				Description: params.RecapText(basePath, parents),
				Actions:     []components.SelectItem{{Label: "Yes, apply", Value: relocateApply}},
			}
		},
	})
	// Skip the confirmation when nothing would change: base_path kept and no worktree
	// to move or adopt. Never reached as the first step (the gate always precedes it
	// when EditBasePath), so AutoSkip is evaluated.
	step.AutoSkip = func(w components.WizardModel) bool { return !willChange(w.Steps(), params) }
	return step
}

// willChange reports whether applying would do anything: the base_path is being set to
// a new value, or there is already worktree work under the current base_path.
func willChange(steps []components.Step, params RunParams) bool {
	if gateChoice(steps) == basePathChange {
		if v := textValue(steps, stepBasePathValue); v != "" && v != params.CurrentBasePath {
			return true
		}
	}
	return params.HasWork
}

// resolveBasePath returns the base_path the user settled on: the edited value when
// the gate was set to "change" (and a non-empty value entered), else the current one.
func resolveBasePath(steps []components.Step, current string) string {
	if gateChoice(steps) != basePathChange {
		return current
	}
	if v := textValue(steps, stepBasePathValue); v != "" {
		return v
	}
	return current
}

func gateChoice(steps []components.Step) string {
	for _, s := range steps {
		if s.Name != stepBasePathGate {
			continue
		}
		if sl, ok := s.Model.(components.SelectListModel); ok {
			return sl.Value()
		}
	}
	return ""
}

func textValue(steps []components.Step, name string) string {
	for _, s := range steps {
		if s.Name != name {
			continue
		}
		if ti, ok := s.Model.(components.TextInputModel); ok {
			return ti.Value()
		}
	}
	return ""
}

func recapConfirmed(steps []components.Step) bool {
	for _, s := range steps {
		if s.Name != stepApply {
			continue
		}
		if sl, ok := s.Model.(components.SelectListModel); ok {
			return sl.Value() == relocateApply
		}
	}
	return false
}

type parentStepParams struct {
	Branch     string
	BaseBranch string
	Holder     *[]domain.BranchCandidate
}

func parentStep(params parentStepParams) components.Step {
	desc := params.Branch + " was created outside wtm, so it has no recorded parent. " +
		"Pick the branch `wtm sync` should rebase it onto — its files stay where they are. " +
		"The full set of moves and adoptions is recapped on the final step."
	return components.ParentStep(components.ParentStepParams{
		Name:         "Parent for " + params.Branch,
		Title:        "Parent for " + params.Branch,
		Holder:       params.Holder,
		Pinned:       params.BaseBranch,
		PinnedSuffix: domain.PinnedSuffixDefault,
		Callout:      true,
		Resolve: func([]components.Step) components.ParentStepContext {
			return components.ParentStepContext{Excludes: []string{params.Branch}, Description: desc}
		},
	})
}

func extractParents(steps []components.Step, adoptions []domain.RelocateStep) map[string]string {
	parents := make(map[string]string, len(adoptions))
	for _, adoption := range adoptions {
		if v := parentValue(steps, "Parent for "+adoption.Branch); v != "" {
			parents[adoption.Branch] = v
		}
	}
	return parents
}

func parentValue(steps []components.Step, name string) string {
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
