// Package progress is the OUTPUT-feedback capability: running slow work while
// showing a spinner. It is deliberately separate from internal/prompter (user
// INPUT): a spinner animates a result, it never solicits the user. Keeping the two
// interfaces apart respects interface segregation — orchestration helpers depend on
// Runner, only genuine prompts depend on prompter.Prompter.
package progress

// Runner runs work while optionally showing a spinner.
type Runner interface {
	Run(params RunParams, work func() error) error
}

// RunParams configures Run.
type RunParams struct {
	Message string
	// Animate shows the spinner; callers pass false for machine output (JSON) so the
	// work runs silently.
	Animate bool
}
