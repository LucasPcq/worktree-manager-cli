package init

import (
	"errors"
	"fmt"

	"github.com/charmbracelet/huh"

	"github.com/LucasPcq/wtm/internal/domain"
)

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

	envOptions := buildEnvOptions(detection.EnvFiles)

	groups := []*huh.Group{
		huh.NewGroup(
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
		),
		buildEnvGroup(envOptions, &envCopyFiles, &envStrategy),
		huh.NewGroup(
			huh.NewInput().
				Title("Install command").
				Description("Command to run after creating a worktree (leave empty to skip)").
				Placeholder(detection.InstallCommand).
				Value(&installCommand),
		),
	}

	var dockerComposeFile string
	var enableDockerHooks bool
	if len(detection.DockerComposeFiles) > 0 {
		groups = append(groups, buildDockerGroup(detection.DockerComposeFiles, &dockerComposeFile, &enableDockerHooks))
	}

	var monorepoPackages []string
	if len(detection.MonorepoPackages) > 0 {
		groups = append(groups, buildMonorepoGroup(detection.MonorepoPackages, detection.InstallCommand, &monorepoPackages))
	}

	groups = append(groups, huh.NewGroup(
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
	))

	form := huh.NewForm(groups...)

	if err := form.Run(); err != nil {
		if errors.Is(err, huh.ErrUserAborted) {
			return domain.InitProjectAnswers{}, domain.ErrUserAborted
		}
		return domain.InitProjectAnswers{}, err
	}

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

	if enableDockerHooks && dockerComposeFile != "" {
		focusCmd, blurCmd := dockerHookCommands(dockerComposeFile)
		answers.OnFocusCommands = []string{focusCmd}
		answers.OnBlurCommands = []string{blurCmd}
	}

	if len(monorepoPackages) > 0 && installCommand != "" {
		for _, pkg := range monorepoPackages {
			answers.OnCreateExtra = append(answers.OnCreateExtra, domain.HookCommand{
				Cmd: installCommand,
				Cwd: pkg,
			})
		}
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

func buildDockerGroup(files []string, selectedFile *string, enableHooks *bool) *huh.Group {
	fields := []huh.Field{}

	if len(files) > 1 {
		options := make([]huh.Option[string], 0, len(files))
		for _, f := range files {
			options = append(options, huh.NewOption(f, f))
		}
		fields = append(fields,
			huh.NewSelect[string]().
				Title("Docker Compose file").
				Description("Multiple docker-compose files detected — select one").
				Options(options...).
				Value(selectedFile),
		)
	} else {
		*selectedFile = files[0]
	}

	fields = append(fields,
		huh.NewConfirm().
			Title("Docker hooks").
			Description("Automatically start/stop Docker when switching worktrees?").
			Value(enableHooks),
	)

	return huh.NewGroup(fields...)
}

func buildMonorepoGroup(packages []string, installCmd string, selectedPackages *[]string) *huh.Group {
	options := make([]huh.Option[string], 0, len(packages))
	for _, pkg := range packages {
		options = append(options, huh.NewOption(pkg, pkg).Selected(true))
	}

	return huh.NewGroup(
		huh.NewMultiSelect[string]().
			Title("Monorepo packages").
			Description(fmt.Sprintf("Run '%s' in selected packages on worktree creation", installCmd)).
			Options(options...).
			Value(selectedPackages),
	)
}

func dockerHookCommands(composeFile string) (string, string) {
	if composeFile == "docker-compose.yml" || composeFile == "docker-compose.yaml" {
		return "docker-compose up -d", "docker-compose down --remove-orphans"
	}
	return fmt.Sprintf("docker-compose -f %s up -d", composeFile),
		fmt.Sprintf("docker-compose -f %s down --remove-orphans", composeFile)
}
