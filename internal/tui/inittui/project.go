package inittui

import (
	"fmt"
	"strconv"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/tui/components"
)

// RunProjectWizard presents the project init wizard pre-populated with detection results.
// Returns ErrUserAborted if the user presses Esc at the first step.
//
// The wizard is organised by concept. Each optional section (env, hooks,
// services) is introduced by a "gate" step that explains what the section does
// and offers Configure / Skip; its sub-steps auto-skip when the gate is skipped.
func RunProjectWizard(detection domain.InitDetectionResult) (domain.InitProjectAnswers, error) {
	var (
		steps               []components.Step
		idxBasePath         int
		idxBaseBranch       int
		idxEnvStrategy      int
		idxInstallCommand   int
		idxEnvFiles         = -1
		idxMonorepoPackages = -1
		idxDockerCompose    = -1
		idxPackageScripts   = -1
	)

	stepIdx := 0

	idxBasePath = stepIdx
	steps = append(steps, components.Step{
		Name: "Worktree directory",
		Model: components.NewTextInput(components.NewTextInputParams{
			Title:       "Worktree directory",
			Description: "wtm creates each worktree in this directory, relative to the repo root. Keeping it outside the repo avoids cluttering your main checkout.",
			Placeholder: domain.DefaultBasePath,
		}),
		Summary: textInputSummary,
		Callout: true,
	})
	stepIdx++

	idxBaseBranch = stepIdx
	steps = append(steps, components.Step{
		Name: "Base branch",
		Model: components.NewTextInput(components.NewTextInputParams{
			Title:       "Base branch",
			Description: "New worktrees branch off this base by default — usually your main development branch.",
			Placeholder: detection.BaseBranch,
		}),
		Summary: textInputSummary,
		Callout: true,
	})
	stepIdx++

	// ── Env section ──────────────────────────────────────────────────────────
	idxEnvGate := stepIdx
	steps = append(steps, sectionGate(sectionGateParams{
		Name: "Environment files",
		Description: "Each new worktree is a clean checkout — your local .env files aren't carried over. " +
			"wtm can copy them in automatically so the worktree runs without redoing your local setup.",
		Detected:         detectedEnv(detection),
		ConfigureLabel:   "Configure .env copying",
		SkipLabel:        "Skip — I'll handle .env myself",
		DefaultConfigure: len(detection.EnvFiles) > 0,
	}))
	stepIdx++
	skipWhenEnvGated := autoSkipWhenGateSkipped(idxEnvGate)

	idxEnvStrategy = stepIdx
	steps = append(steps, components.Step{
		Name: "Env strategy",
		Model: components.NewSelectList(components.NewSelectListParams{
			Title:       "Env strategy",
			Description: "How wtm provisions .env files in a new worktree: copy .env.example, copy from your main worktree, or from the worktree you branched from.",
			Items: []components.SelectItem{
				{Label: "example — copy .env.example → .env", Value: string(domain.EnvStrategyExample)},
				{Label: "main — copy .env from main worktree", Value: string(domain.EnvStrategyMain)},
				{Label: "parent — copy .env from source worktree", Value: string(domain.EnvStrategyParent)},
			},
		}),
		Summary:  selectListSummary,
		AutoSkip: skipWhenEnvGated,
		Callout:  true,
	})
	stepIdx++

	if len(detection.EnvFiles) > 0 {
		idxEnvFiles = stepIdx
		items := make([]components.MultiSelectItem, 0, len(detection.EnvFiles))
		for _, f := range detection.EnvFiles {
			items = append(items, components.MultiSelectItem{
				Label:    f,
				Value:    f,
				Selected: true,
			})
		}
		steps = append(steps, components.Step{
			Name: "Env files",
			Model: components.NewMultiSelect(components.NewMultiSelectParams{
				Title:       "Env files to copy",
				Description: "Which of the detected .env files wtm should copy into every new worktree.",
				Items:       items,
			}),
			Summary:  multiSelectSummary,
			AutoSkip: skipWhenEnvGated,
			Callout:  true,
		})
		stepIdx++
	}

	// ── Hooks section ────────────────────────────────────────────────────────
	idxHooksGate := stepIdx
	steps = append(steps, sectionGate(sectionGateParams{
		Name: "Post-create hooks",
		Description: "Run commands automatically right after a worktree is created — typically installing " +
			"dependencies — so it's ready to use immediately instead of needing manual steps.",
		Detected:         detectedHooks(detection),
		ConfigureLabel:   "Configure setup commands",
		SkipLabel:        "Skip — no automatic commands",
		DefaultConfigure: detection.InstallCommand != "",
	}))
	stepIdx++
	skipWhenHooksGated := autoSkipWhenGateSkipped(idxHooksGate)

	idxInstallCommand = stepIdx
	steps = append(steps, components.Step{
		Name: "Install command",
		Model: components.NewTextInput(components.NewTextInputParams{
			Title:       "Install command",
			Description: "Runs once right after a worktree is created — typically to install dependencies so it's ready to use.",
			Placeholder: detection.InstallCommand,
		}),
		Summary:  textInputSummary,
		AutoSkip: skipWhenHooksGated,
		Callout:  true,
	})
	stepIdx++

	if len(detection.MonorepoPackages) > 0 {
		idxMonorepoPackages = stepIdx
		items := make([]components.MultiSelectItem, 0, len(detection.MonorepoPackages))
		for _, pkg := range detection.MonorepoPackages {
			items = append(items, components.MultiSelectItem{
				Label:    pkg,
				Value:    pkg,
				Selected: true,
			})
		}
		steps = append(steps, components.Step{
			Name: "Monorepo packages",
			Model: components.NewMultiSelect(components.NewMultiSelectParams{
				Title:       "Monorepo packages",
				Description: fmt.Sprintf("Also run '%s' inside these packages on worktree creation (monorepo setups).", detection.InstallCommand),
				Items:       items,
			}),
			Summary:  multiSelectSummary,
			AutoSkip: skipWhenHooksGated,
			Callout:  true,
		})
		stepIdx++
	}

	// ── Services section ─────────────────────────────────────────────────────
	idxServicesGate := stepIdx
	steps = append(steps, sectionGate(sectionGateParams{
		Name: "Services & tasks",
		Description: "Let wtm run your dev stack per worktree — long-running services (databases, dev servers) " +
			"and one-off tasks — so you start/stop everything with `wtm run` instead of juggling terminals.",
		Detected:         detectedServices(detection),
		ConfigureLabel:   "Configure services & tasks",
		SkipLabel:        "Skip — no service management",
		DefaultConfigure: len(detection.DockerComposeFiles) > 0 || len(detection.PackageScripts) > 0,
	}))
	stepIdx++
	skipWhenServicesGated := autoSkipWhenGateSkipped(idxServicesGate)

	if len(detection.DockerComposeFiles) > 0 {
		idxDockerCompose = stepIdx
		items := make([]components.MultiSelectItem, 0, len(detection.DockerComposeFiles))
		for _, f := range detection.DockerComposeFiles {
			items = append(items, components.MultiSelectItem{
				Label:    f,
				Value:    f,
				Selected: true,
			})
		}
		steps = append(steps, components.Step{
			Name: "Docker services",
			Model: components.NewMultiSelect(components.NewMultiSelectParams{
				Title:       "Docker Compose services",
				Description: "Each selected docker-compose file becomes a service you can start and stop with `wtm run`.",
				Items:       items,
			}),
			Summary:  multiSelectSummary,
			AutoSkip: skipWhenServicesGated,
			Callout:  true,
		})
		stepIdx++
	}

	if len(detection.PackageScripts) > 0 {
		idxPackageScripts = stepIdx
		pm := string(detection.PackageManager)
		items := make([]components.MultiSelectItem, 0, len(detection.PackageScripts))
		for i, s := range detection.PackageScripts {
			scope := "root"
			if s.Workspace != "" {
				scope = s.Workspace
			}
			label := fmt.Sprintf("%s / %s — %s run %s", scope, s.Name, pm, s.Name)
			items = append(items, components.MultiSelectItem{
				Label:    label,
				Value:    fmt.Sprintf("%d", i),
				Selected: s.Kind == domain.JobKindService,
			})
		}
		steps = append(steps, components.Step{
			Name: "Package scripts",
			Model: components.NewMultiSelect(components.NewMultiSelectParams{
				Title:       "Package.json scripts",
				Description: "Each selected script becomes a job — dev-style scripts run as services, the rest as one-off tasks.",
				Items:       items,
			}),
			Summary:  packageScriptsSummary,
			AutoSkip: skipWhenServicesGated,
			Callout:  true,
		})
		stepIdx++
	}

	wiz := components.NewWizard(steps)
	finalModel, err := tea.NewProgram(wiz).Run()
	if err != nil {
		return domain.InitProjectAnswers{}, fmt.Errorf("project wizard: %w", err)
	}

	final, ok := finalModel.(components.WizardModel)
	if !ok {
		return domain.InitProjectAnswers{}, domain.ErrUserAborted
	}
	if final.Aborted() {
		return domain.InitProjectAnswers{}, domain.ErrUserAborted
	}

	return extractProjectAnswers(extractProjectParams{
		Final:               final,
		Detection:           detection,
		IdxBasePath:         idxBasePath,
		IdxBaseBranch:       idxBaseBranch,
		IdxEnvStrategy:      idxEnvStrategy,
		IdxInstallCommand:   idxInstallCommand,
		IdxEnvFiles:         idxEnvFiles,
		IdxMonorepoPackages: idxMonorepoPackages,
		IdxDockerCompose:    idxDockerCompose,
		IdxPackageScripts:   idxPackageScripts,
	})
}

type extractProjectParams struct {
	Final               components.WizardModel
	Detection           domain.InitDetectionResult
	IdxBasePath         int
	IdxBaseBranch       int
	IdxEnvStrategy      int
	IdxInstallCommand   int
	IdxEnvFiles         int
	IdxMonorepoPackages int
	IdxDockerCompose    int
	IdxPackageScripts   int
}

func extractProjectAnswers(p extractProjectParams) (domain.InitProjectAnswers, error) {
	finalSteps := p.Final.Steps()

	basePathModel, ok := finalSteps[p.IdxBasePath].Model.(components.TextInputModel)
	if !ok {
		return domain.InitProjectAnswers{}, domain.ErrUserAborted
	}
	basePath := basePathModel.Value()
	if basePath == "" {
		basePath = domain.DefaultBasePath
	}

	baseBranchModel, ok := finalSteps[p.IdxBaseBranch].Model.(components.TextInputModel)
	if !ok {
		return domain.InitProjectAnswers{}, domain.ErrUserAborted
	}
	baseBranch := baseBranchModel.Value()
	if baseBranch == "" {
		baseBranch = p.Detection.BaseBranch
	}

	answers := domain.InitProjectAnswers{
		BasePath:   basePath,
		BaseBranch: baseBranch,
	}

	// Env section — skipping the strategy step opts the whole section out.
	if p.Final.Skipped(p.IdxEnvStrategy) {
		answers.SkipEnv = true
	} else {
		envStrategyModel, ok := finalSteps[p.IdxEnvStrategy].Model.(components.SelectListModel)
		if !ok {
			return domain.InitProjectAnswers{}, domain.ErrUserAborted
		}
		answers.EnvStrategy = domain.EnvStrategy(envStrategyModel.Value())
		if p.IdxEnvFiles >= 0 && !p.Final.Skipped(p.IdxEnvFiles) {
			msModel, ok := finalSteps[p.IdxEnvFiles].Model.(components.MultiSelectModel)
			if !ok {
				return domain.InitProjectAnswers{}, domain.ErrUserAborted
			}
			answers.EnvCopyFiles = msModel.Values()
		}
	}

	// Hooks section — skipping the install step opts the whole section out.
	if p.Final.Skipped(p.IdxInstallCommand) {
		answers.SkipHooks = true
	} else {
		installCmdModel, ok := finalSteps[p.IdxInstallCommand].Model.(components.TextInputModel)
		if !ok {
			return domain.InitProjectAnswers{}, domain.ErrUserAborted
		}
		installCommand := installCmdModel.Value()
		if installCommand == "" {
			installCommand = p.Detection.InstallCommand
		}
		answers.InstallCommand = installCommand
		if p.IdxMonorepoPackages >= 0 && !p.Final.Skipped(p.IdxMonorepoPackages) && installCommand != "" {
			msModel, ok := finalSteps[p.IdxMonorepoPackages].Model.(components.MultiSelectModel)
			if !ok {
				return domain.InitProjectAnswers{}, domain.ErrUserAborted
			}
			for _, pkg := range msModel.Values() {
				answers.OnCreateExtra = append(answers.OnCreateExtra, domain.HookCommand{
					Cmd: installCommand,
					Cwd: pkg,
				})
			}
		}
	}

	if p.IdxDockerCompose >= 0 && !p.Final.Skipped(p.IdxDockerCompose) {
		msModel, ok := finalSteps[p.IdxDockerCompose].Model.(components.MultiSelectModel)
		if !ok {
			return domain.InitProjectAnswers{}, domain.ErrUserAborted
		}
		answers.DockerComposeFiles = msModel.Values()
		if len(answers.DockerComposeFiles) > 0 {
			answers.DockerComposeCmd = p.Detection.DockerComposeCmd
		}
	}

	if p.IdxPackageScripts >= 0 && !p.Final.Skipped(p.IdxPackageScripts) {
		msModel, ok := finalSteps[p.IdxPackageScripts].Model.(components.MultiSelectModel)
		if !ok {
			return domain.InitProjectAnswers{}, domain.ErrUserAborted
		}
		for _, idxStr := range msModel.Values() {
			idx, parseErr := strconv.Atoi(idxStr)
			if parseErr != nil || idx < 0 || idx >= len(p.Detection.PackageScripts) {
				continue
			}
			answers.SelectedPackageScripts = append(answers.SelectedPackageScripts, p.Detection.PackageScripts[idx])
		}
	}

	return answers, nil
}

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
	vals := ms.Values()
	return fmt.Sprintf("%d files selected", len(vals))
}

func packageScriptsSummary(model any) string {
	ms, ok := model.(components.MultiSelectModel)
	if !ok {
		return ""
	}
	n := len(ms.Values())
	return fmt.Sprintf("%d scripts selected", n)
}

// sectionGateParams holds the inputs for a section gate step.
type sectionGateParams struct {
	Name             string
	Description      string
	Detected         string
	ConfigureLabel   string
	SkipLabel        string
	DefaultConfigure bool
}

// sectionGate builds the intro/gate step for an optional section. The recommended
// option (driven by detection) is listed first so it is highlighted by default.
// The explanation is rendered as a callout above the Configure/Skip choice.
func sectionGate(p sectionGateParams) components.Step {
	configure := components.SelectItem{Label: p.ConfigureLabel, Value: domain.WizardChoiceConfigure}
	skip := components.SelectItem{Label: p.SkipLabel, Value: domain.WizardChoiceSkip}

	items := []components.SelectItem{skip, configure}
	if p.DefaultConfigure {
		items = []components.SelectItem{configure, skip}
	}

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
