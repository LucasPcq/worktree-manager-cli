package relocate

import "github.com/LucasPcq/wtm/internal/tui/components"

// Step names and gate values for the base_path prompt.
const (
	basePathGateStep  = "Change base_path"
	basePathValueStep = "New base_path"
	basePathKeep      = "keep"
	basePathChange    = "change"
)

// BasePathPromptParams holds inputs for PromptBasePath. Validate is provided by the
// command so the TUI stays free of decision logic (it forwards the rules validator).
type BasePathPromptParams struct {
	Current  string
	Validate func(string) error
}

// BasePathResult is the outcome of the base_path prompt: the resolved path and
// whether the user actually changed it.
type BasePathResult struct {
	BasePath string
	Changed  bool
}

// PromptBasePath asks whether to change base_path and, when the user opts in,
// collects the new value (pre-filled with the current one). The value step is
// auto-skipped when the user keeps the current path, so the common "just adopt or
// align" case is a single Enter. Esc aborts (domain.ErrUserAborted, surfaced by
// RunWizard).
func PromptBasePath(params BasePathPromptParams) (BasePathResult, error) {
	gate := components.Step{
		Name: basePathGateStep,
		Model: components.NewSelectList(components.NewSelectListParams{
			Title:       "Change base_path?",
			Description: "Worktrees live under " + params.Current + ". Keep it, or set a new location to move them to.",
			Items: []components.SelectItem{
				{Label: "Keep " + params.Current, Value: basePathKeep},
				{Label: "Change it", Value: basePathChange},
			},
		}),
		Summary: components.SelectSummary,
	}

	valueStep := components.Step{
		Name:     basePathValueStep,
		Model:    newBasePathInput(params),
		Build:    func([]components.Step) any { return newBasePathInput(params) },
		AutoSkip: func(w components.WizardModel) bool { return gateChoice(w.Steps()) != basePathChange },
		Summary:  components.TextSummary,
	}

	final, err := components.RunWizard(components.RunWizardParams{
		Steps:    []components.Step{gate, valueStep},
		Stderr:   true,
		ErrLabel: "base_path prompt",
	})
	if err != nil {
		return BasePathResult{}, err
	}

	done := final.Steps()
	if gateChoice(done) != basePathChange {
		return BasePathResult{BasePath: params.Current}, nil
	}
	newBase := textValue(done, basePathValueStep)
	if newBase == "" || newBase == params.Current {
		return BasePathResult{BasePath: params.Current}, nil
	}
	return BasePathResult{BasePath: newBase, Changed: true}, nil
}

func newBasePathInput(params BasePathPromptParams) components.TextInputModel {
	return components.NewTextInput(components.NewTextInputParams{
		Title:       "New base_path",
		Description: "Relative to the repo root (e.g. ../.trees). Existing worktrees move here.",
		Placeholder: params.Current,
		Default:     params.Current,
		Validate:    params.Validate,
	})
}

func gateChoice(steps []components.Step) string {
	for _, s := range steps {
		if s.Name != basePathGateStep {
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
