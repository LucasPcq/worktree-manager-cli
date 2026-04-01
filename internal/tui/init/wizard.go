// Package init builds interactive huh forms for the wtm init wizard.
package init

import (
	"errors"

	"github.com/charmbracelet/huh"

	"github.com/LucasPcq/wtm/internal/domain"
)

// RunGlobalWizard presents the global config setup form.
// Returns ErrUserAborted if the user presses Ctrl+C.
func RunGlobalWizard() (domain.InitGlobalAnswers, error) {
	var agent string
	var shell string

	form := huh.NewForm(
		huh.NewGroup(
			huh.NewSelect[string]().
				Title("Default AI agent").
				Description("Which agent do you use for development?").
				Options(
					huh.NewOption("Claude Code", string(domain.AgentClaudeCode)),
					huh.NewOption("Cursor", string(domain.AgentCursor)),
					huh.NewOption("None", string(domain.AgentNone)),
				).
				Value(&agent),

			huh.NewSelect[string]().
				Title("Shell").
				Description("Your primary shell (for shell-init integration)").
				Options(
					huh.NewOption("zsh", string(domain.ShellZsh)),
					huh.NewOption("bash", string(domain.ShellBash)),
					huh.NewOption("fish", string(domain.ShellFish)),
				).
				Value(&shell),
		),
	)

	if err := form.Run(); err != nil {
		if errors.Is(err, huh.ErrUserAborted) {
			return domain.InitGlobalAnswers{}, domain.ErrUserAborted
		}
		return domain.InitGlobalAnswers{}, err
	}

	return domain.InitGlobalAnswers{
		Agent: domain.AgentType(agent),
		Shell: domain.ShellType(shell),
	}, nil
}

// RunProjectWizard presents the project init form pre-populated with detection results.
// Returns ErrUserAborted if the user presses Ctrl+C.
func RunProjectWizard(detection domain.InitDetectionResult) (domain.InitProjectAnswers, error) {
	var (
		basePath       string
		baseBranch     string
		envCopyFiles   []string
		envStrategy    string
		installCommand string
		agentChoice    string
	)

	// Build env file options with pre-selection
	envOptions := buildEnvOptions(detection.EnvFiles)

	// Build groups
	worktreeGroup := huh.NewGroup(
		huh.NewInput().
			Title("Worktree directory").
			Description("Where to store worktrees (relative to repo root)").
			Placeholder(domain.DefaultBasePath).
			Value(&basePath),

		huh.NewInput().
			Title("Base branch").
			Description("Default branch for new worktrees").
			Placeholder(detection.BaseBranch).
			Value(&baseBranch),
	)

	envGroup := buildEnvGroup(envOptions, &envCopyFiles, &envStrategy)

	hooksGroup := huh.NewGroup(
		huh.NewInput().
			Title("Install command").
			Description("Command to run after creating a worktree (leave empty to skip)").
			Placeholder(detection.InstallCommand).
			Value(&installCommand),
	)

	agentGroup := huh.NewGroup(
		huh.NewSelect[string]().
			Title("Project AI agent").
			Description("Override global default, or inherit").
			Options(
				huh.NewOption("Inherit from global config", "inherit"),
				huh.NewOption("Claude Code", string(domain.AgentClaudeCode)),
				huh.NewOption("Cursor", string(domain.AgentCursor)),
				huh.NewOption("None", string(domain.AgentNone)),
			).
			Value(&agentChoice),
	)

	form := huh.NewForm(worktreeGroup, envGroup, hooksGroup, agentGroup)

	if err := form.Run(); err != nil {
		if errors.Is(err, huh.ErrUserAborted) {
			return domain.InitProjectAnswers{}, domain.ErrUserAborted
		}
		return domain.InitProjectAnswers{}, err
	}

	// Apply defaults for empty inputs
	if basePath == "" {
		basePath = domain.DefaultBasePath
	}
	if baseBranch == "" {
		baseBranch = detection.BaseBranch
	}
	if installCommand == "" {
		installCommand = detection.InstallCommand
	}

	answers := domain.InitProjectAnswers{
		BasePath:       basePath,
		BaseBranch:     baseBranch,
		EnvCopyFiles:   envCopyFiles,
		EnvStrategy:    domain.EnvStrategy(envStrategy),
		InstallCommand: installCommand,
	}

	if agentChoice != "inherit" {
		answers.AgentOverride = true
		answers.Agent = domain.AgentType(agentChoice)
	}

	return answers, nil
}

func buildEnvOptions(files []string) []huh.Option[string] {
	options := make([]huh.Option[string], 0, len(files))
	for _, f := range files {
		options = append(options, huh.NewOption(f, f).Selected(true))
	}
	return options
}

func buildEnvGroup(envOptions []huh.Option[string], envCopyFiles *[]string, envStrategy *string) *huh.Group {
	fields := []huh.Field{
		huh.NewSelect[string]().
			Title("Env strategy").
			Description("How to provision .env files in new worktrees").
			Options(
				huh.NewOption("example — copy .env.example → .env", string(domain.EnvStrategyExample)),
				huh.NewOption("main — copy .env from main worktree", string(domain.EnvStrategyMain)),
				huh.NewOption("parent — copy .env from source worktree", string(domain.EnvStrategyParent)),
			).
			Value(envStrategy),
	}

	if len(envOptions) > 0 {
		fields = append(fields,
			huh.NewMultiSelect[string]().
				Title("Env files to copy").
				Description("Detected .env files — select which to copy into new worktrees").
				Options(envOptions...).
				Value(envCopyFiles),
		)
	}

	return huh.NewGroup(fields...)
}
