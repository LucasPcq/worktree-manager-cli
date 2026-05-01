package inittui

import (
	"fmt"
	"strconv"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/tui/components"
)

// RunProjectWizard presents the project init wizard pre-populated with detection results.
// Returns ErrUserAborted if the user presses Esc at the first step.
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
		idxAgent            int
	)

	stepIdx := 0

	idxBasePath = stepIdx
	steps = append(steps, components.Step{
		Name: "Worktree directory",
		Model: components.NewTextInput(components.NewTextInputParams{
			Title:       "Worktree directory",
			Description: "Where to store worktrees (relative to repo root)",
			Placeholder: domain.DefaultBasePath,
		}),
		Summary: textInputSummary,
	})
	stepIdx++

	idxBaseBranch = stepIdx
	steps = append(steps, components.Step{
		Name: "Base branch",
		Model: components.NewTextInput(components.NewTextInputParams{
			Title:       "Base branch",
			Description: "Default branch for new worktrees",
			Placeholder: detection.BaseBranch,
		}),
		Summary: textInputSummary,
	})
	stepIdx++

	idxEnvStrategy = stepIdx
	steps = append(steps, components.Step{
		Name: "Env strategy",
		Model: components.NewSelectList(components.NewSelectListParams{
			Title:       "Env strategy",
			Description: "How to provision .env files in new worktrees",
			Items: []components.SelectItem{
				{Label: "example — copy .env.example → .env", Value: string(domain.EnvStrategyExample)},
				{Label: "main — copy .env from main worktree", Value: string(domain.EnvStrategyMain)},
				{Label: "parent — copy .env from source worktree", Value: string(domain.EnvStrategyParent)},
			},
		}),
		Summary: selectListSummary,
	})
	stepIdx++

	idxInstallCommand = stepIdx
	steps = append(steps, components.Step{
		Name: "Install command",
		Model: components.NewTextInput(components.NewTextInputParams{
			Title:       "Install command",
			Description: "Command to run after creating a worktree (leave empty to skip)",
			Placeholder: detection.InstallCommand,
		}),
		Summary: textInputSummary,
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
				Description: "Detected .env files — select which to copy into new worktrees",
				Items:       items,
			}),
			Summary: multiSelectSummary,
		})
		stepIdx++
	}

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
				Description: fmt.Sprintf("Run '%s' in selected packages on worktree creation", detection.InstallCommand),
				Items:       items,
			}),
			Summary: multiSelectSummary,
		})
		stepIdx++
	}

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
				Description: "Detected docker-compose files — selected files become service jobs in run.toml",
				Items:       items,
			}),
			Summary: multiSelectSummary,
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
				Description: "Selected scripts become jobs in run.toml (dev-style scripts → services, others → tasks)",
				Items:       items,
			}),
			Summary: packageScriptsSummary,
		})
		stepIdx++
	}

	idxAgent = stepIdx
	steps = append(steps, components.Step{
		Name: "Project AI agent",
		Model: components.NewSelectList(components.NewSelectListParams{
			Title:       "Project AI agent",
			Description: "Override global default, or inherit",
			Items: []components.SelectItem{
				{Label: "Inherit from global config", Value: "inherit"},
				{Label: "Claude Code", Value: string(domain.AgentClaudeCode)},
				{Label: "Cursor", Value: string(domain.AgentCursor)},
				{Label: "None", Value: string(domain.AgentNone)},
			},
		}),
		Summary: selectListSummary,
	})

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
		IdxAgent:            idxAgent,
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
	IdxAgent            int
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

	envStrategyModel, ok := finalSteps[p.IdxEnvStrategy].Model.(components.SelectListModel)
	if !ok {
		return domain.InitProjectAnswers{}, domain.ErrUserAborted
	}

	installCmdModel, ok := finalSteps[p.IdxInstallCommand].Model.(components.TextInputModel)
	if !ok {
		return domain.InitProjectAnswers{}, domain.ErrUserAborted
	}
	installCommand := installCmdModel.Value()
	if installCommand == "" {
		installCommand = p.Detection.InstallCommand
	}

	var envCopyFiles []string
	if p.IdxEnvFiles >= 0 {
		msModel, ok := finalSteps[p.IdxEnvFiles].Model.(components.MultiSelectModel)
		if !ok {
			return domain.InitProjectAnswers{}, domain.ErrUserAborted
		}
		envCopyFiles = msModel.Values()
	}

	answers := domain.InitProjectAnswers{
		BasePath:       basePath,
		BaseBranch:     baseBranch,
		EnvCopyFiles:   envCopyFiles,
		EnvStrategy:    domain.EnvStrategy(envStrategyModel.Value()),
		InstallCommand: installCommand,
	}

	if p.IdxDockerCompose >= 0 {
		msModel, ok := finalSteps[p.IdxDockerCompose].Model.(components.MultiSelectModel)
		if !ok {
			return domain.InitProjectAnswers{}, domain.ErrUserAborted
		}
		answers.DockerComposeFiles = msModel.Values()
		if len(answers.DockerComposeFiles) > 0 {
			answers.DockerComposeCmd = p.Detection.DockerComposeCmd
		}
	}

	if p.IdxMonorepoPackages >= 0 {
		msModel, ok := finalSteps[p.IdxMonorepoPackages].Model.(components.MultiSelectModel)
		if !ok {
			return domain.InitProjectAnswers{}, domain.ErrUserAborted
		}
		monorepoPackages := msModel.Values()
		if len(monorepoPackages) > 0 && installCommand != "" {
			for _, pkg := range monorepoPackages {
				answers.OnCreateExtra = append(answers.OnCreateExtra, domain.HookCommand{
					Cmd: installCommand,
					Cwd: pkg,
				})
			}
		}
	}

	if p.IdxPackageScripts >= 0 {
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

	agentModel, ok := finalSteps[p.IdxAgent].Model.(components.SelectListModel)
	if !ok {
		return domain.InitProjectAnswers{}, domain.ErrUserAborted
	}
	agentChoice := agentModel.Value()
	if agentChoice != "inherit" {
		answers.AgentOverride = true
		answers.Agent = domain.AgentType(agentChoice)
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
