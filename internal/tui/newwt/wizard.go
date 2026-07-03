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
	stepBranchName = "Branch name"
	stepSourceName = "Source branch"
	stepEnvName    = "Env strategy"
	stepSourceUpd  = "Source update"
	stepConfirm    = "Confirm & create"
)

// Source-update choices and the recap action value.
const (
	sourceFastForward = "ff"
	sourceKeep        = "keep"
	createConfirm     = "create"
)

// SourceUpdatePrompt decides the conditional "reconcile source" step. In the
// wizard, only a fast-forward offer (Show && !AbortOnDecline) is an actionable
// choice; a diverged source (Show && AbortOnDecline) is surfaced as a ⚠ line in
// the recap instead of a gate. The standalone --from path still renders Params as
// a single confirm, so those fields are preserved.
type SourceUpdatePrompt struct {
	// Show controls whether the confirmation appears at all (standalone path) or a
	// fast-forward choice appears (wizard path).
	Show bool
	// Params is the prompt to render when Show is true.
	Params components.NewConfirmParams
	// AbortOnDecline treats a "No" as cancelling creation (a diverged source);
	// when false a "No" silently proceeds (declining a fast-forward offer).
	AbortOnDecline bool
	// SkipReason is shown in the wizard summaries when the source-update step is
	// skipped (no fast-forward offer): e.g. "source already up to date".
	SkipReason string
}

// WizardParams holds inputs for the interactive new worktree wizard.
type WizardParams struct {
	ProjectDir     string
	Branches       []domain.BranchCandidate
	DefaultBranch  string
	ConfigStrategy domain.EnvStrategy
	IncludeBranch  bool
	// BranchName is the branch name fixed by a positional arg (when IncludeBranch is
	// false); shown in the recap so it stays complete even without a branch step.
	BranchName string
	// Source, when non-empty (--from), fixes the source branch and skips the source
	// picker. The source-update fast-forward is then computed up front.
	Source string
	// IncludeEnv shows the env-strategy step; false when --env-from fixes it, in
	// which case EnvOverride carries the chosen strategy.
	IncludeEnv  bool
	EnvOverride string
	// SourceUpdate, when set, decides the source-update step: given the source
	// branch it offers a fast-forward (behind-only) or reports a diverged branch as
	// a ⚠ recap line. Injected by the command layer so the TUI stays free of git logic.
	SourceUpdate func(source string) SourceUpdatePrompt
	// EnvFallback, when set, decides whether the "parent" env strategy will fall
	// back to copying .env from main — surfaced as a ⚠ recap line.
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

// CreateFlow is the create sub-flow (branch/source/source-update steps) ready to
// embed in a host wizard — e.g. `wtm extract` inserts it before its own recap so
// creating the target worktree and extracting share one combined recap. It also
// carries the source picker's background branch-refresh wiring, which the host
// plugs into its wizard.
type CreateFlow struct {
	Steps       []components.Step
	InitCmd     tea.Cmd
	OnMsg       components.WizardMsgHandler
	LoadingText string
}

// CreateSteps builds the create sub-flow steps WITHOUT the final recap, so a host
// wizard can compose them with its own steps and show one combined recap. When
// enabled is non-nil each step auto-skips while enabled(steps) is false (e.g. the
// extract target is an existing worktree, not "create new"). A nil enabled keeps
// every step active (the standalone `wtm create`).
func CreateSteps(params WizardParams, enabled func(steps []components.Step) bool) CreateFlow {
	holder := &params.Branches
	var steps []components.Step

	if params.IncludeBranch {
		steps = append(steps, gateStep(branchStep(), enabled))
	}
	if params.Source == "" {
		steps = append(steps, gateStep(sourceStep(holder, params.DefaultBranch), enabled))
	}
	if params.IncludeEnv {
		steps = append(steps, gateStep(envStep(params.ConfigStrategy), enabled))
	}
	// The source-update fast-forward: a ChoiceStep when the source is picked, else
	// (fixed by --from) a concrete step added only when a fast-forward is offered —
	// a ChoiceStep cannot be a first step. A diverged source is not a gate here; it
	// becomes a ⚠ line in the recap.
	if params.SourceUpdate != nil {
		if params.Source == "" {
			steps = append(steps, gateStep(sourceUpdateStep(params.SourceUpdate), enabled))
		} else if p := params.SourceUpdate(params.Source); p.Show && !p.AbortOnDecline {
			steps = append(steps, gateStep(sourceUpdateConcreteStep(p), enabled))
		}
	}

	flow := CreateFlow{Steps: steps}
	if params.Source == "" {
		flow.InitCmd = branchrefresh.Cmd(params.ProjectDir)
		flow.OnMsg = branchrefresh.Handler(params.ProjectDir, holder)
		flow.LoadingText = domain.LoadingBranchesText
	}
	return flow
}

// gateStep makes a step auto-skip while enabled(steps) is false, composing with any
// AutoSkip the step already has (so a ChoiceStep still skips on its own logic when
// enabled). A nil enabled leaves the step untouched.
func gateStep(step components.Step, enabled func(steps []components.Step) bool) components.Step {
	if enabled == nil {
		return step
	}
	inner := step.AutoSkip
	step.AutoSkip = func(w components.WizardModel) bool {
		if !enabled(w.Steps()) {
			return true
		}
		return inner != nil && inner(w)
	}
	return step
}

// ReadCreateResult reads the create answers from a (host or standalone) wizard's
// steps by name. It does not interpret a cancel — the host's recap owns that.
func ReadCreateResult(steps []components.Step, params WizardParams) WizardResult {
	result := WizardResult{
		FromBranch:        resolveSource(steps, params),
		EnvFromOverride:   resolveEnv(steps, params),
		FastForwardSource: stepValueByName(steps, stepSourceUpd) == sourceFastForward,
	}
	if child, ok := stepModelByName(steps, stepBranchName).(components.TextInputModel); ok {
		result.BranchName = child.Value()
	}
	return result
}

func branchStep() components.Step {
	return components.Step{
		Name: stepBranchName,
		Model: components.NewTextInput(components.NewTextInputParams{
			Title:       stepBranchName,
			Description: "Name for the new worktree branch",
			Validate: func(s string) error {
				if strings.TrimSpace(s) == "" {
					return fmt.Errorf("branch name is required")
				}
				return nil
			},
		}),
		Summary: components.TextSummary,
	}
}

func sourceStep(holder *[]domain.BranchCandidate, defaultBranch string) components.Step {
	sourceList := func() any {
		return components.NewSelectList(components.NewSelectListParams{
			Title:       stepSourceName,
			Description: "Branch to base the new worktree on",
			Items:       buildBranchItems(*holder, defaultBranch),
		})
	}
	return components.Step{
		Name:       stepSourceName,
		Model:      sourceList(),
		Build:      func([]components.Step) any { return sourceList() },
		CanRefresh: true,
		Summary:    components.SelectSummary,
	}
}

func envStep(strategy domain.EnvStrategy) components.Step {
	return components.Step{
		Name: stepEnvName,
		Model: components.NewSelectList(components.NewSelectListParams{
			Title:       stepEnvName,
			Description: "How to provision .env files in the new worktree",
			Items:       buildEnvItems(strategy),
		}),
		Summary: envStrategySummary,
	}
}

// RunWizard displays the interactive wizard for wtm new.
// When IncludeBranch is true, the first step prompts for a branch name.
// Returns ErrUserAborted on Ctrl+C or Esc at the first step, or when the user
// declines a hard confirmation (diverged source, or the env fallback).
func RunWizard(params WizardParams) (WizardResult, error) {
	flow := CreateSteps(params, nil)

	// The final recap: always last, recaps the selections with ⚠ lines for a
	// diverged source and the env fallback, and offers "Yes, create worktree" then
	// the constant "No, cancel" — the single cancellation point.
	steps := append(flow.Steps, components.RecapStep(components.RecapStepParams{
		Name: stepConfirm,
		Build: func(prev []components.Step) components.RecapContent {
			return components.RecapContent{
				Description: buildCreateRecap(prev, params),
				Actions: []components.SelectItem{
					{Label: "Yes, create worktree", Value: createConfirm},
				},
			}
		},
	}))

	wp := components.WizardParams{
		Steps:       steps,
		InitCmd:     flow.InitCmd,
		OnMsg:       flow.OnMsg,
		LoadingText: flow.LoadingText,
		Loading:     flow.InitCmd != nil,
	}
	finalModel, err := tea.NewProgram(components.NewWizardWithParams(wp)).Run()
	if err != nil {
		return WizardResult{}, fmt.Errorf("wizard: %w", err)
	}

	final, ok := finalModel.(components.WizardModel)
	if !ok || final.Aborted() {
		return WizardResult{}, domain.ErrUserAborted
	}
	return extractResult(final, params)
}

// sourceUpdateItems are the fast-forward-vs-keep options, shared by the ChoiceStep
// (picked source) and concrete (fixed source) variants.
func sourceUpdateItems() []components.SelectItem {
	return []components.SelectItem{
		{Label: "Fast-forward the source to origin", Value: sourceFastForward},
		{Separator: true},
		{Label: "Keep it as-is", Value: sourceKeep},
	}
}

// sourceUpdateConcreteStep builds an always-shown fast-forward select for the
// --from path, where the source (and thus its divergence) is known up front. It
// avoids ChoiceStep because it may be the wizard's first step.
func sourceUpdateConcreteStep(p SourceUpdatePrompt) components.Step {
	return components.Step{
		Name: stepSourceUpd,
		Model: components.NewSelectList(components.NewSelectListParams{
			Description: p.Params.Description,
			Items:       sourceUpdateItems(),
		}),
		Summary: sourceUpdateSummary,
	}
}

// sourceUpdateStep builds the optional fast-forward select. It applies only for a
// behind-only source (a fast-forward offer); a diverged or up-to-date source
// auto-skips with a reason (the diverged warning re-appears in the recap).
func sourceUpdateStep(decide func(source string) SourceUpdatePrompt) components.Step {
	return components.ChoiceStep(components.ChoiceStepParams{
		Name:    stepSourceUpd,
		Summary: sourceUpdateSummary,
		Decide: func(prev []components.Step) (bool, string, components.NewSelectListParams) {
			p := decide(stepValueByName(prev, stepSourceName))
			if p.Show && !p.AbortOnDecline {
				return true, "", components.NewSelectListParams{
					Description: p.Params.Description,
					Items:       sourceUpdateItems(),
				}
			}
			return false, p.SkipReason, components.NewSelectListParams{}
		},
	})
}

// sourceUpdateSummary labels the fast-forward choice in the completed-step summaries.
func sourceUpdateSummary(model any) string {
	sl, ok := model.(components.SelectListModel)
	if !ok {
		return ""
	}
	if sl.Value() == sourceFastForward {
		return "fast-forward source"
	}
	return "keep source as-is"
}

// resolveSource returns the source branch: the --from preset, else the picked one.
func resolveSource(steps []components.Step, params WizardParams) string {
	if params.Source != "" {
		return params.Source
	}
	return stepValueByName(steps, stepSourceName)
}

// resolveEnv returns the env override: the --env-from preset, else the picked one.
func resolveEnv(steps []components.Step, params WizardParams) string {
	if !params.IncludeEnv {
		return params.EnvOverride
	}
	return stepValueByName(steps, stepEnvName)
}

// buildCreateRecap recaps the selections with ⚠ lines for a diverged source and
// the env fallback, using the same deciders the steps did.
func buildCreateRecap(prev []components.Step, params WizardParams) string {
	source := resolveSource(prev, params)
	env := resolveEnv(prev, params)
	envLabel := env
	if envLabel == "" {
		envLabel = "config default"
	}

	branchName := params.BranchName
	if params.IncludeBranch {
		branchName = textValueByName(prev, stepBranchName)
	}

	sourceLabel := source
	if stepValueByName(prev, stepSourceUpd) == sourceFastForward {
		sourceLabel += " (fast-forward to origin)"
	}

	var lines []string
	if branchName != "" {
		lines = append(lines, "Branch:  "+branchName)
	}
	lines = append(lines,
		"Source:  "+sourceLabel,
		"Env:     "+envLabel,
	)

	if warnings := CreateWarnings(prev, params); len(warnings) > 0 {
		lines = append(lines, "")
		lines = append(lines, warnings...)
	}
	return strings.Join(lines, "\n")
}

// CreateWarnings returns the ⚠ recap lines for the create sub-flow — a diverged
// source and the env fallback — from the same deciders the steps used. Exposed so
// a host wizard (extract) can fold them into its combined recap.
func CreateWarnings(steps []components.Step, params WizardParams) []string {
	source := resolveSource(steps, params)
	env := resolveEnv(steps, params)
	var warnings []string
	if params.SourceUpdate != nil {
		if p := params.SourceUpdate(source); p.Show && p.AbortOnDecline && p.Params.Warning != "" {
			warnings = append(warnings, "⚠ "+p.Params.Warning)
		}
	}
	if params.EnvFallback != nil {
		if show, ep := params.EnvFallback(source, env); show && ep.Warning != "" {
			warnings = append(warnings, "⚠ "+ep.Warning)
		}
	}
	return warnings
}

// extractResult reads the wizard answers, translating the recap's "No, cancel"
// into ErrUserAborted.
func extractResult(final components.WizardModel, params WizardParams) (WizardResult, error) {
	steps := final.Steps()
	if stepValueByName(steps, stepConfirm) == domain.WizardCancelValue {
		return WizardResult{}, domain.ErrUserAborted
	}
	return ReadCreateResult(steps, params), nil
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

func textValueByName(steps []components.Step, name string) string {
	if ti, ok := stepModelByName(steps, name).(components.TextInputModel); ok {
		return ti.Value()
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
		PinnedSuffix: domain.PinnedSuffixDefault,
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
