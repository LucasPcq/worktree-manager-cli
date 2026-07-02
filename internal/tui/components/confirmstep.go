package components

// ConfirmStepParams configures a conditional confirmation step built by
// ConfirmStep.
type ConfirmStepParams struct {
	// Name labels the step in the breadcrumb and completed-step summaries.
	Name string
	// Decide runs each time the wizard enters this step. It inspects the
	// already-completed prior steps and returns whether the confirmation is
	// needed plus the prompt to show. When apply is false the step is
	// auto-skipped — hidden from the breadcrumb and reported via Skipped(i).
	Decide func(prev []Step) (apply bool, params NewConfirmParams)
	// YesLabel and NoLabel label the choice in the completed-step summary.
	// They default to "yes"/"no".
	YesLabel string
	NoLabel  string
}

// ConfirmStep builds a wizard step that shows a yes/no confirmation only when it
// is relevant, deriving both its relevance and its wording from the earlier
// answers via Decide. It packages the Build + AutoSkip pair the sync/relocate
// wizards use, so a confirmation lives inside the wizard — with breadcrumb and
// back navigation — instead of as an orphaned standalone prompt after it (where
// Esc would abort the whole operation). Decide runs once per entry in Build; its
// applies result is cached and read back by AutoSkip in the same advance pass.
func ConfirmStep(p ConfirmStepParams) Step {
	yes, no := p.YesLabel, p.NoLabel
	if yes == "" {
		yes = "yes"
	}
	if no == "" {
		no = "no"
	}

	// Cached across Build → AutoSkip within a single advance() pass: buildStep
	// runs Build (setting applies) immediately before AutoSkip is evaluated.
	applies := true
	build := func(prev []Step) any {
		apply, params := p.Decide(prev)
		applies = apply
		return NewConfirm(params)
	}

	return Step{
		Name:     p.Name,
		Model:    NewConfirm(NewConfirmParams{}),
		Build:    build,
		AutoSkip: func(WizardModel) bool { return !applies },
		Summary:  ConfirmSummary(yes, no),
	}
}
