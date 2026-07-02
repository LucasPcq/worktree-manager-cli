// Package newwt builds interactive wizards for the wtm new command.
package newwt

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/tui/branchrefresh"
	"github.com/LucasPcq/wtm/internal/tui/components"
)

// Step names, shared between construction and value extraction so prior-step
// answers can be located by name rather than by a fragile positional offset.
const (
	stepBranchName  = "Branch name"
	stepSourceName  = "Source branch"
	stepEnvName     = "Env strategy"
	stepSourceUpd   = "Source update"
	stepEnvFallback = "Env source"
)

// SourceUpdatePrompt decides the conditional "reconcile source" confirmation.
type SourceUpdatePrompt struct {
	// Show controls whether the confirmation appears at all.
	Show bool
	// Params is the prompt to render when Show is true.
	Params components.NewConfirmParams
	// AbortOnDecline treats a "No" as cancelling creation (a diverged source);
	// when false a "No" silently proceeds (declining a fast-forward offer).
	AbortOnDecline bool
}

// WizardParams holds inputs for the interactive new worktree wizard.
type WizardParams struct {
	ProjectDir     string
	Branches       []domain.BranchCandidate
	DefaultBranch  string
	ConfigStrategy domain.EnvStrategy
	IncludeBranch  bool
	// SourceUpdate, when set, adds a conditional confirmation after the env step:
	// given the chosen source branch it decides whether to offer a fast-forward
	// (behind-only) or warn about a diverged branch. Injected by the command layer
	// so the TUI stays free of git logic.
	SourceUpdate func(source string) SourceUpdatePrompt
	// EnvFallback, when set, adds a conditional confirmation: given the chosen
	// source and env override it decides whether to warn that the "parent" env
	// strategy will fall back to copying .env from main.
	EnvFallback func(source, envOverride string) (show bool, params components.NewConfirmParams)
}

// WizardResult holds the answers from the interactive wizard.
type WizardResult struct {
	BranchName      string
	FromBranch      string
	EnvFromOverride string
	// FastForwardSource is true when the user accepted a fast-forward offer for a
	// behind-only source; the command fast-forwards it before creating.
	FastForwardSource bool
}

// RunWizard displays the interactive wizard for wtm new.
// When IncludeBranch is true, the first step prompts for a branch name.
// Returns ErrUserAborted on Ctrl+C or Esc at the first step, or when the user
// declines a hard confirmation (diverged source, or the env fallback).
func RunWizard(params WizardParams) (WizardResult, error) {
	var steps []components.Step

	if params.IncludeBranch {
		branchInput := components.NewTextInput(components.NewTextInputParams{
			Title:       stepBranchName,
			Description: "Name for the new worktree branch",
			Validate: func(s string) error {
				if strings.TrimSpace(s) == "" {
					return fmt.Errorf("branch name is required")
				}
				return nil
			},
		})
		steps = append(steps, components.Step{
			Name:    stepBranchName,
			Model:   branchInput,
			Summary: components.TextSummary,
		})
	}

	holder := &params.Branches
	sourceList := func() any {
		return components.NewSelectList(components.NewSelectListParams{
			Title:       stepSourceName,
			Description: "Branch to base the new worktree on",
			Items:       buildBranchItems(*holder, params.DefaultBranch),
		})
	}

	steps = append(steps, components.Step{
		Name:       stepSourceName,
		Model:      sourceList(),
		Build:      func([]components.Step) any { return sourceList() },
		CanRefresh: true,
		Summary:    components.SelectSummary,
	})

	envItems := buildEnvItems(params.ConfigStrategy)
	envList := components.NewSelectList(components.NewSelectListParams{
		Title:       stepEnvName,
		Description: "How to provision .env files in the new worktree",
		Items:       envItems,
	})

	steps = append(steps, components.Step{
		Name:    stepEnvName,
		Model:   envList,
		Summary: envStrategySummary,
	})

	// A confirmation that lives inside the wizard: Esc goes back to the env step
	// instead of aborting the whole creation, and it is hidden from the breadcrumb
	// when it does not apply. srcPrompt captures the last decision so the caller
	// can tell a fast-forward offer (decline = proceed) from a diverged warning
	// (decline = abort).
	var srcPrompt SourceUpdatePrompt
	if params.SourceUpdate != nil {
		steps = append(steps, components.ConfirmStep(components.ConfirmStepParams{
			Name: stepSourceUpd,
			Decide: func(prev []components.Step) (bool, components.NewConfirmParams) {
				p := params.SourceUpdate(stepValueByName(prev, stepSourceName))
				srcPrompt = p
				return p.Show, p.Params
			},
		}))
	}

	if params.EnvFallback != nil {
		steps = append(steps, components.ConfirmStep(components.ConfirmStepParams{
			Name: stepEnvFallback,
			Decide: func(prev []components.Step) (bool, components.NewConfirmParams) {
				return params.EnvFallback(
					stepValueByName(prev, stepSourceName),
					stepValueByName(prev, stepEnvName),
				)
			},
		}))
	}

	wiz := components.NewWizardWithParams(components.WizardParams{
		Steps:       steps,
		InitCmd:     branchrefresh.Cmd(params.ProjectDir),
		Loading:     true,
		LoadingText: domain.LoadingBranchesText,
		OnMsg:       branchrefresh.Handler(params.ProjectDir, holder),
	})
	p := tea.NewProgram(wiz)

	finalModel, err := p.Run()
	if err != nil {
		return WizardResult{}, fmt.Errorf("wizard: %w", err)
	}

	final, ok := finalModel.(components.WizardModel)
	if !ok || final.Aborted() {
		return WizardResult{}, domain.ErrUserAborted
	}

	return extractResult(final, srcPrompt)
}

// extractResult reads the wizard answers, translating a declined hard
// confirmation (diverged source, or env fallback) into ErrUserAborted.
func extractResult(final components.WizardModel, srcPrompt SourceUpdatePrompt) (WizardResult, error) {
	steps := final.Steps()
	var result WizardResult

	if child, ok := stepModelByName(steps, stepBranchName).(components.TextInputModel); ok {
		result.BranchName = child.Value()
	}
	if child, ok := stepModelByName(steps, stepSourceName).(components.SelectListModel); ok {
		result.FromBranch = child.Value()
	}
	if child, ok := stepModelByName(steps, stepEnvName).(components.SelectListModel); ok {
		result.EnvFromOverride = child.Value()
	}

	if applied, confirmed := confirmAnswer(final, stepSourceUpd); applied {
		if srcPrompt.AbortOnDecline && !confirmed {
			return WizardResult{}, domain.ErrUserAborted
		}
		if !srcPrompt.AbortOnDecline && confirmed {
			result.FastForwardSource = true
		}
	}

	if applied, confirmed := confirmAnswer(final, stepEnvFallback); applied && !confirmed {
		return WizardResult{}, domain.ErrUserAborted
	}

	return result, nil
}

// confirmAnswer reports whether a named ConfirmStep applied (was not auto-skipped)
// and, if so, the user's yes/no answer.
func confirmAnswer(final components.WizardModel, name string) (applied, confirmed bool) {
	steps := final.Steps()
	idx := stepIndexByName(steps, name)
	if idx < 0 || final.Skipped(idx) {
		return false, false
	}
	child, ok := steps[idx].Model.(components.ConfirmModel)
	if !ok {
		return false, false
	}
	return true, child.Confirmed()
}

func stepIndexByName(steps []components.Step, name string) int {
	for i := range steps {
		if steps[i].Name == name {
			return i
		}
	}
	return -1
}

func stepModelByName(steps []components.Step, name string) any {
	if i := stepIndexByName(steps, name); i >= 0 {
		return steps[i].Model
	}
	return nil
}

func stepValueByName(steps []components.Step, name string) string {
	if sl, ok := stepModelByName(steps, name).(components.SelectListModel); ok {
		return sl.Value()
	}
	return ""
}

// envStrategySummary renders the chosen env strategy, labelling the empty
// (config default) choice explicitly.
func envStrategySummary(model any) string {
	child, ok := model.(components.SelectListModel)
	if !ok {
		return ""
	}
	if v := child.Value(); v != "" {
		return v
	}
	return "config default"
}

func buildBranchItems(branches []domain.BranchCandidate, defaultBranch string) []components.SelectItem {
	pinned := ""
	for _, b := range branches {
		if b.Name == defaultBranch {
			pinned = defaultBranch
			break
		}
	}
	return components.BranchItems(components.BranchItemsParams{
		Candidates:   branches,
		Pinned:       pinned,
		PinnedSuffix: " (default)",
	})
}

func buildEnvItems(strategy domain.EnvStrategy) []components.SelectItem {
	return []components.SelectItem{
		{Label: "Use config default (" + string(strategy) + ")", Value: ""},
		{Label: "example — copy .env.example → .env", Value: string(domain.EnvStrategyExample)},
		{Label: "main — copy .env from main worktree", Value: string(domain.EnvStrategyMain)},
		{Label: "parent — copy .env from source worktree", Value: string(domain.EnvStrategyParent)},
	}
}
