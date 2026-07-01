package inittui

import (
	"fmt"
	"strconv"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/tui/components"
)

// Step keys identify wizard steps so extraction is positional-independent —
// the same builders feed both the full init wizard and the targeted
// `wtm init --only <section>` re-init wizard.
const (
	stepBasePath     = "base_path"
	stepBaseBranch   = "base_branch"
	stepEnvGate      = "env_gate"
	stepEnvStrategy  = "env_strategy"
	stepEnvFiles     = "env_files"
	stepHooksGate    = "hooks_gate"
	stepHooks        = "hooks"
	stepServicesGate = "services_gate"
	stepDocker       = "docker"
	stepScripts      = "scripts"
)

// stepSet accumulates wizard steps and records each one's index by key.
type stepSet struct {
	steps []components.Step
	idx   map[string]int
}

func newStepSet() *stepSet {
	return &stepSet{idx: map[string]int{}}
}

func (s *stepSet) add(key string, step components.Step) {
	s.idx[key] = len(s.steps)
	s.steps = append(s.steps, step)
}

// at returns the index of a registered step, or -1 if it was not built.
func (s *stepSet) at(key string) int {
	if i, ok := s.idx[key]; ok {
		return i
	}
	return -1
}

// SectionPrefill carries the current on-disk config so a targeted re-init wizard
// pre-selects what's already configured. A nil prefill (full init) falls back to
// detection-driven defaults with no "current/new" tags.
type SectionPrefill struct {
	BaseBranch    string
	EnvStrategy   string
	EnvCopyFiles  map[string]bool
	OnCreate      []domain.HookCommand
	DockerFiles   map[string]bool
	ScriptIndices map[int]bool
}

// RunProjectWizard presents the full project init wizard pre-populated with
// detection results. Returns ErrUserAborted if the user aborts.
//
// The wizard is organised by concept. Each optional section (env, hooks,
// services) is introduced by a "gate" step that explains what the section does
// and offers Configure / Skip; its sub-steps auto-skip when the gate is skipped.
func RunProjectWizard(detection domain.InitDetectionResult) (domain.InitProjectAnswers, error) {
	s := newStepSet()

	s.add(stepBasePath, basePathStep())
	s.add(stepBaseBranch, baseBranchStep(detection))

	s.add(stepEnvGate, envGate(detection))
	addEnvSteps(s, detection, autoSkipWhenGateSkipped(s.at(stepEnvGate)), nil)

	s.add(stepHooksGate, hooksGate(detection))
	addHooksSteps(s, detection, autoSkipWhenGateSkipped(s.at(stepHooksGate)), nil)

	s.add(stepServicesGate, servicesGate(detection))
	addServicesSteps(s, detection, autoSkipWhenGateSkipped(s.at(stepServicesGate)), nil)

	final, err := runWizard(s.steps)
	if err != nil {
		return domain.InitProjectAnswers{}, err
	}
	return extractProjectAnswers(final, detection, s.idx), nil
}

// RunSectionWizard presents a targeted wizard for the requested sections only,
// without gates or core steps. Used by `wtm init --only <section>`. Returns a
// nil-ish answers set with no steps when nothing is configurable (e.g. services
// requested but none detected); callers handle the empty case.
func RunSectionWizard(sections []string, detection domain.InitDetectionResult, prefill *SectionPrefill) (domain.InitProjectAnswers, error) {
	s := newStepSet()
	for _, section := range sections {
		switch section {
		case domain.SectionWorktrees:
			addWorktreesSteps(s, detection, prefill)
		case domain.SectionEnv:
			addEnvSteps(s, detection, nil, prefill)
		case domain.SectionHooks:
			addHooksSteps(s, detection, nil, prefill)
		case domain.SectionServices:
			addServicesSteps(s, detection, nil, prefill)
		}
	}

	if len(s.steps) == 0 {
		return domain.InitProjectAnswers{}, nil
	}

	final, err := runWizard(s.steps)
	if err != nil {
		return domain.InitProjectAnswers{}, err
	}
	return extractProjectAnswers(final, detection, s.idx), nil
}

// runWizard runs the bubbletea program for the given steps and returns the
// final model, mapping abort/quit to ErrUserAborted.
func runWizard(steps []components.Step) (components.WizardModel, error) {
	wiz := components.NewWizard(steps)
	finalModel, err := tea.NewProgram(wiz).Run()
	if err != nil {
		return components.WizardModel{}, fmt.Errorf("wizard: %w", err)
	}
	final, ok := finalModel.(components.WizardModel)
	if !ok || final.Aborted() {
		return components.WizardModel{}, domain.ErrUserAborted
	}
	return final, nil
}

// ── Step builders ───────────────────────────────────────────────────────────

func basePathStep() components.Step {
	return components.Step{
		Name: "Worktree directory",
		Model: components.NewTextInput(components.NewTextInputParams{
			Title:       "Worktree directory",
			Description: "wtm creates each worktree in this directory, relative to the repo root. Keeping it outside the repo avoids cluttering your main checkout.",
			Placeholder: domain.DefaultBasePath,
		}),
		Summary: textInputSummary,
		Callout: true,
	}
}

func baseBranchStep(detection domain.InitDetectionResult) components.Step {
	return components.Step{
		Name: "Base branch",
		Model: components.NewSelectList(components.NewSelectListParams{
			Title:       "Base branch",
			Description: "New worktrees branch off this base by default — usually your main development branch.",
			Items:       baseBranchItems(detection.Branches, detection.BaseBranch),
		}),
		Summary: selectListSummary,
		Callout: true,
	}
}

// baseBranchItems builds the base-branch picker rows with the detected default
// pinned first and remote-tracking branches grouped after a separator.
func baseBranchItems(branches []domain.BranchCandidate, detected string) []components.SelectItem {
	return components.BranchItems(components.BranchItemsParams{
		Candidates:   branches,
		Pinned:       detected,
		PinnedSuffix: " (detected)",
	})
}

// addWorktreesSteps adds the editable worktrees step (base branch only) for a
// targeted re-init. base_path is intentionally not editable here — changing it
// would orphan existing worktrees (tracked separately).
func addWorktreesSteps(s *stepSet, detection domain.InitDetectionResult, prefill *SectionPrefill) {
	pinned := detection.BaseBranch
	if prefill != nil && prefill.BaseBranch != "" {
		pinned = prefill.BaseBranch
	}
	s.add(stepBaseBranch, components.Step{
		Name: "Base branch",
		Model: components.NewSelectList(components.NewSelectListParams{
			Title:       "Base branch",
			Description: "Default branch new worktrees are created from. Changing it only affects future worktrees.",
			Items:       baseBranchItems(detection.Branches, pinned),
		}),
		Summary: selectListSummary,
		Callout: true,
	})
}

func envGate(detection domain.InitDetectionResult) components.Step {
	return sectionGate(sectionGateParams{
		Name: "Environment files",
		Description: "Each new worktree is a clean checkout — your local .env files aren't carried over. " +
			"wtm can copy them in automatically so the worktree runs without redoing your local setup.",
		Detected:       detectedEnv(detection),
		ConfigureLabel: "Configure .env copying",
		SkipLabel:      "Skip — I'll handle .env myself",
	})
}

func addEnvSteps(s *stepSet, detection domain.InitDetectionResult, autoSkip func(components.WizardModel) bool, prefill *SectionPrefill) {
	strategyItems := []components.SelectItem{
		{Label: "example — copy .env.example → .env", Value: string(domain.EnvStrategyExample)},
		{Label: "main — copy .env from main worktree", Value: string(domain.EnvStrategyMain)},
		{Label: "parent — copy .env from source worktree", Value: string(domain.EnvStrategyParent)},
	}
	if prefill != nil {
		strategyItems = moveToFront(strategyItems, prefill.EnvStrategy)
	}
	s.add(stepEnvStrategy, components.Step{
		Name: "Env strategy",
		Model: components.NewSelectList(components.NewSelectListParams{
			Title:       "Env strategy",
			Description: "How wtm provisions .env files in a new worktree: copy .env.example, copy from your main worktree, or from the worktree you branched from.",
			Items:       strategyItems,
		}),
		Summary:  selectListSummary,
		AutoSkip: autoSkip,
		Callout:  true,
	})

	if len(detection.EnvFiles) == 0 {
		return
	}
	items := make([]components.MultiSelectItem, 0, len(detection.EnvFiles))
	for _, f := range detection.EnvFiles {
		selected := prefillSelected(prefill, prefill != nil && prefill.EnvCopyFiles[f], true)
		items = append(items, components.MultiSelectItem{Label: f, Value: f, Selected: selected})
	}
	s.add(stepEnvFiles, components.Step{
		Name: "Env files",
		Model: components.NewMultiSelect(components.NewMultiSelectParams{
			Title:       "Env files to copy",
			Description: "Which of the detected .env files wtm should copy into every new worktree.",
			Items:       items,
		}),
		Summary:  multiSelectSummary,
		AutoSkip: autoSkip,
		Callout:  true,
	})
}

func hooksGate(detection domain.InitDetectionResult) components.Step {
	return sectionGate(sectionGateParams{
		Name: "Post-create hooks",
		Description: "Run commands automatically right after a worktree is created — typically installing " +
			"dependencies — so it's ready to use immediately instead of needing manual steps.",
		Detected:       detectedHooks(detection),
		ConfigureLabel: "Configure setup commands",
		SkipLabel:      "Skip — no automatic commands",
	})
}

func addHooksSteps(s *stepSet, detection domain.InitDetectionResult, autoSkip func(components.WizardModel) bool, prefill *SectionPrefill) {
	var hooks []domain.HookCommand
	if prefill != nil {
		hooks = prefill.OnCreate
	} else if detection.InstallCommand != "" {
		hooks = append(hooks, domain.HookCommand{Cmd: detection.InstallCommand})
		for _, pkg := range detection.MonorepoPackages {
			hooks = append(hooks, domain.HookCommand{Cmd: detection.InstallCommand, Cwd: pkg})
		}
	}

	s.add(stepHooks, components.Step{
		Name: "Post-create hooks",
		Model: components.NewHookList(components.NewHookListParams{
			Title:       "Post-create hooks",
			Description: "Commands run after creating a worktree. Add, edit, remove or reorder them — then select Done.",
			Hooks:       hooks,
		}),
		Summary:  hookListSummary,
		AutoSkip: autoSkip,
		Callout:  true,
	})
}

func servicesGate(detection domain.InitDetectionResult) components.Step {
	return sectionGate(sectionGateParams{
		Name: "Services & tasks",
		Description: "Let wtm run your dev stack per worktree — long-running services (databases, dev servers) " +
			"and one-off tasks — so you start/stop everything with `wtm run` instead of juggling terminals.",
		Detected:       detectedServices(detection),
		ConfigureLabel: "Configure services & tasks",
		SkipLabel:      "Skip — no service management",
	})
}

func addServicesSteps(s *stepSet, detection domain.InitDetectionResult, autoSkip func(components.WizardModel) bool, prefill *SectionPrefill) {
	if len(detection.DockerComposeFiles) > 0 {
		items := make([]components.MultiSelectItem, 0, len(detection.DockerComposeFiles))
		for _, f := range detection.DockerComposeFiles {
			selected := prefillSelected(prefill, prefill != nil && prefill.DockerFiles[f], true)
			items = append(items, components.MultiSelectItem{Label: f, Value: f, Selected: selected})
		}
		s.add(stepDocker, components.Step{
			Name: "Docker services",
			Model: components.NewMultiSelect(components.NewMultiSelectParams{
				Title:       "Docker Compose services",
				Description: "Each selected docker-compose file becomes a service you can start and stop with `wtm run`.",
				Items:       items,
			}),
			Summary:  multiSelectSummary,
			AutoSkip: autoSkip,
			Callout:  true,
		})
	}

	if len(detection.PackageScripts) > 0 {
		pm := string(detection.PackageManager)
		items := make([]components.MultiSelectItem, 0, len(detection.PackageScripts))
		for i, script := range detection.PackageScripts {
			scope := "root"
			if script.Workspace != "" {
				scope = script.Workspace
			}
			label := fmt.Sprintf("%s / %s — %s run %s", scope, script.Name, pm, script.Name)
			selected := prefillSelected(prefill, prefill != nil && prefill.ScriptIndices[i], script.Kind == domain.JobKindService)
			items = append(items, components.MultiSelectItem{
				Label:    label,
				Value:    strconv.Itoa(i),
				Selected: selected,
			})
		}
		s.add(stepScripts, components.Step{
			Name: "Package scripts",
			Model: components.NewMultiSelect(components.NewMultiSelectParams{
				Title:       "Package.json scripts",
				Description: "Each selected script becomes a job — dev-style scripts run as services, the rest as one-off tasks.",
				Items:       items,
			}),
			Summary:  packageScriptsSummary,
			AutoSkip: autoSkip,
			Callout:  true,
		})
	}
}

// ── Extraction ──────────────────────────────────────────────────────────────

// extractProjectAnswers reads the answers from the final wizard model using the
// step-key→index map, tolerating absent steps (targeted re-init builds a subset).
func extractProjectAnswers(final components.WizardModel, detection domain.InitDetectionResult, idx map[string]int) domain.InitProjectAnswers {
	steps := final.Steps()
	answers := domain.InitProjectAnswers{}

	at := func(key string) int {
		if i, ok := idx[key]; ok {
			return i
		}
		return -1
	}

	if i := at(stepBasePath); i >= 0 {
		if m, ok := steps[i].Model.(components.TextInputModel); ok {
			answers.BasePath = m.Value()
		}
	}
	if answers.BasePath == "" {
		answers.BasePath = domain.DefaultBasePath
	}

	if i := at(stepBaseBranch); i >= 0 {
		if m, ok := steps[i].Model.(components.SelectListModel); ok {
			answers.BaseBranch = m.Value()
		}
	}
	if answers.BaseBranch == "" {
		answers.BaseBranch = detection.BaseBranch
	}

	// Env section.
	if i := at(stepEnvStrategy); i >= 0 {
		if final.Skipped(i) {
			answers.SkipEnv = true
		} else if m, ok := steps[i].Model.(components.SelectListModel); ok {
			answers.EnvStrategy = domain.EnvStrategy(m.Value())
			if fi := at(stepEnvFiles); fi >= 0 && !final.Skipped(fi) {
				if fm, ok := steps[fi].Model.(components.MultiSelectModel); ok {
					answers.EnvCopyFiles = fm.Values()
				}
			}
		}
	}

	// Hooks section.
	if i := at(stepHooks); i >= 0 {
		if final.Skipped(i) {
			answers.SkipHooks = true
		} else if m, ok := steps[i].Model.(components.HookListModel); ok {
			answers.OnCreate = m.Hooks()
		}
	}

	// Services section.
	if i := at(stepDocker); i >= 0 && !final.Skipped(i) {
		if m, ok := steps[i].Model.(components.MultiSelectModel); ok {
			answers.DockerComposeFiles = m.Values()
			if len(answers.DockerComposeFiles) > 0 {
				answers.DockerComposeCmd = detection.DockerComposeCmd
			}
		}
	}
	if i := at(stepScripts); i >= 0 && !final.Skipped(i) {
		if m, ok := steps[i].Model.(components.MultiSelectModel); ok {
			for _, idxStr := range m.Values() {
				n, err := strconv.Atoi(idxStr)
				if err != nil || n < 0 || n >= len(detection.PackageScripts) {
					continue
				}
				answers.SelectedPackageScripts = append(answers.SelectedPackageScripts, detection.PackageScripts[n])
			}
		}
	}

	return answers
}

// ── Summaries & gate helpers ────────────────────────────────────────────────

func textInputSummary(model any) string {
	ti, ok := model.(components.TextInputModel)
	if !ok {
		return ""
	}
	v := ti.Value()
	if v != "" {
		return v
	}
	if p := ti.Placeholder(); p != "" {
		return p + " (default)"
	}
	return "(empty)"
}

func selectListSummary(model any) string {
	sl, ok := model.(components.SelectListModel)
	if !ok {
		return ""
	}
	return sl.Value()
}

func multiSelectSummary(model any) string {
	ms, ok := model.(components.MultiSelectModel)
	if !ok {
		return ""
	}
	return fmt.Sprintf("%d files selected", len(ms.Values()))
}

func packageScriptsSummary(model any) string {
	ms, ok := model.(components.MultiSelectModel)
	if !ok {
		return ""
	}
	return fmt.Sprintf("%d scripts selected", len(ms.Values()))
}

func hookListSummary(model any) string {
	hl, ok := model.(components.HookListModel)
	if !ok {
		return ""
	}
	return fmt.Sprintf("%d command(s)", len(hl.Hooks()))
}

// sectionGateParams holds the inputs for a section gate step.
type sectionGateParams struct {
	Name           string
	Description    string
	Detected       string
	ConfigureLabel string
	SkipLabel      string
}

// sectionGate builds the intro/gate step for an optional section. Configure is
// always listed first (and thus highlighted by default) so the Configure/Skip
// order stays identical across every gate. The explanation is rendered as a
// callout above the choice.
func sectionGate(p sectionGateParams) components.Step {
	configure := components.SelectItem{Label: p.ConfigureLabel, Value: domain.WizardChoiceConfigure}
	skip := components.SelectItem{Label: p.SkipLabel, Value: domain.WizardChoiceSkip}

	items := []components.SelectItem{configure, skip}

	note := ""
	if p.Detected != "" {
		note = "Detected: " + p.Detected
	}

	return components.Step{
		Name: p.Name,
		Model: components.NewSelectList(components.NewSelectListParams{
			Title:       p.Name,
			Description: p.Description,
			Items:       items,
		}),
		Summary:     gateSummary,
		Callout:     true,
		CalloutNote: note,
	}
}

// gateSummary renders a section gate's chosen state in the breadcrumb.
func gateSummary(model any) string {
	sl, ok := model.(components.SelectListModel)
	if !ok {
		return ""
	}
	if sl.Value() == domain.WizardChoiceSkip {
		return "skipped"
	}
	return "configured"
}

// autoSkipWhenGateSkipped returns an AutoSkip predicate that skips a sub-step
// when its section gate (at gateIdx) is set to "skip".
func autoSkipWhenGateSkipped(gateIdx int) func(components.WizardModel) bool {
	return func(w components.WizardModel) bool {
		return gateValue(w, gateIdx) == domain.WizardChoiceSkip
	}
}

// gateValue reads the selected value of the gate SelectList at idx.
func gateValue(w components.WizardModel, idx int) string {
	steps := w.Steps()
	if idx < 0 || idx >= len(steps) {
		return ""
	}
	sl, ok := steps[idx].Model.(components.SelectListModel)
	if !ok {
		return ""
	}
	return sl.Value()
}

// prefillSelected decides a multiselect item's checked state. Without a prefill
// (full init) it uses the detection default; with a prefill it checks the items
// already present in the current config.
func prefillSelected(prefill *SectionPrefill, configured, fullInitDefault bool) bool {
	if prefill == nil {
		return fullInitDefault
	}
	return configured
}

// moveToFront returns items with the entry matching value placed first, so a
// SelectList highlights the currently-configured choice by default.
func moveToFront(items []components.SelectItem, value string) []components.SelectItem {
	if value == "" {
		return items
	}
	for i, item := range items {
		if item.Value == value {
			reordered := []components.SelectItem{item}
			reordered = append(reordered, items[:i]...)
			reordered = append(reordered, items[i+1:]...)
			return reordered
		}
	}
	return items
}

func detectedEnv(d domain.InitDetectionResult) string {
	return strings.Join(d.EnvFiles, ", ")
}

func detectedHooks(d domain.InitDetectionResult) string {
	return d.InstallCommand
}

func detectedServices(d domain.InitDetectionResult) string {
	var parts []string
	if n := len(d.DockerComposeFiles); n > 0 {
		parts = append(parts, fmt.Sprintf("%d docker-compose file(s)", n))
	}
	if n := len(d.PackageScripts); n > 0 {
		parts = append(parts, fmt.Sprintf("%d script(s)", n))
	}
	return strings.Join(parts, ", ")
}
