package rules

import (
	"fmt"

	"github.com/LucasPcq/wtm/internal/domain"
)

// InitGlobalFlags holds the raw --agent/--shell inputs for non-interactive init.
type InitGlobalFlags struct {
	Agent string
	Shell string
}

// InitProjectFlags holds the raw project-config inputs for non-interactive init.
// NonInteractive makes unresolved required values fail rather than silently
// falling back to a constant default. The Skip* flags opt out of optional
// sections, mirroring the wizard skip key.
type InitProjectFlags struct {
	BasePath       string
	BaseBranch     string
	EnvStrategy    string
	InstallCommand string
	NonInteractive bool
	SkipEnv        bool
	SkipHooks      bool
	SkipServices   bool
}

// BuildGlobalAnswers resolves global config from flags, falling back to the
// constant defaults. Returns ErrInvalidAgentType / ErrInvalidShellType when a
// provided value is not a known enum member.
func BuildGlobalAnswers(flags InitGlobalFlags) (domain.InitGlobalAnswers, error) {
	agent := domain.DefaultAgent
	if flags.Agent != "" {
		agent = domain.AgentType(flags.Agent)
	}
	if err := ValidateAgentType(agent); err != nil {
		return domain.InitGlobalAnswers{}, err
	}

	shell := domain.DefaultShell
	if flags.Shell != "" {
		shell = domain.ShellType(flags.Shell)
	}
	if err := ValidateShellType(shell); err != nil {
		return domain.InitGlobalAnswers{}, err
	}

	return domain.InitGlobalAnswers{Agent: agent, Shell: shell}, nil
}

// BuildProjectAnswers resolves project config from flags and auto-detection,
// mirroring the wizard defaults: flags win, then detection, then constants.
// Conditional multi-selects (env files, package scripts, docker compose,
// monorepo packages) take every detected value, matching the wizard's
// pre-selection. The project agent inherits the global default.
func BuildProjectAnswers(flags InitProjectFlags, detection domain.InitDetectionResult) (domain.InitProjectAnswers, error) {
	basePath := flags.BasePath
	if basePath == "" {
		basePath = domain.DefaultBasePath
	}

	baseBranch := flags.BaseBranch
	if baseBranch == "" {
		baseBranch = detection.BaseBranch
	}
	if baseBranch == "" {
		if flags.NonInteractive {
			return domain.InitProjectAnswers{}, fmt.Errorf("base branch could not be detected — pass --%s", domain.FlagBaseBranch)
		}
		baseBranch = domain.DefaultBaseBranch
	}

	answers := domain.InitProjectAnswers{
		BasePath:   basePath,
		BaseBranch: baseBranch,
	}

	if flags.SkipEnv {
		answers.SkipEnv = true
	} else {
		envStrategy := domain.DefaultEnvStrategy
		if flags.EnvStrategy != "" {
			envStrategy = domain.EnvStrategy(flags.EnvStrategy)
		}
		if err := ValidateEnvStrategy(envStrategy); err != nil {
			return domain.InitProjectAnswers{}, err
		}
		answers.EnvStrategy = envStrategy
		answers.EnvCopyFiles = detection.EnvFiles
	}

	if flags.SkipHooks {
		answers.SkipHooks = true
	} else {
		installCommand := flags.InstallCommand
		if installCommand == "" {
			installCommand = detection.InstallCommand
		}
		answers.InstallCommand = installCommand
		if installCommand != "" {
			for _, pkg := range detection.MonorepoPackages {
				answers.OnCreateExtra = append(answers.OnCreateExtra, domain.HookCommand{
					Cmd: installCommand,
					Cwd: pkg,
				})
			}
		}
	}

	if flags.SkipServices {
		answers.SkipServices = true
	} else {
		answers.SelectedPackageScripts = detection.PackageScripts
		if len(detection.DockerComposeFiles) > 0 {
			answers.DockerComposeFiles = detection.DockerComposeFiles
			answers.DockerComposeCmd = detection.DockerComposeCmd
		}
	}

	return answers, nil
}
