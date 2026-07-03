package rules

import (
	"fmt"

	"github.com/LucasPcq/wtm/internal/domain"
)

// InitGlobalFlags holds the raw --shell input for non-interactive init.
type InitGlobalFlags struct {
	Shell string
}

// InitProjectFlags holds the raw project-config inputs for non-interactive init.
// NonInteractive makes unresolved required values fail rather than silently
// falling back to a constant default. The Skip* flags opt out of optional
// sections, mirroring the wizard skip key. Services are no longer part of the
// global init — they are configured by the dedicated `wtm run init` command.
type InitProjectFlags struct {
	BasePath       string
	BaseBranch     string
	EnvStrategy    string
	InstallCommand string
	NonInteractive bool
	SkipEnv        bool
	SkipHooks      bool
}

// BuildGlobalAnswers resolves global config from flags, falling back to the
// constant defaults. Returns ErrInvalidShellType when a provided value is not a
// known enum member.
func BuildGlobalAnswers(flags InitGlobalFlags) (domain.InitGlobalAnswers, error) {
	shell := domain.DefaultShell
	if flags.Shell != "" {
		shell = domain.ShellType(flags.Shell)
	}
	if err := ValidateShellType(shell); err != nil {
		return domain.InitGlobalAnswers{}, err
	}

	return domain.InitGlobalAnswers{Shell: shell}, nil
}

// BuildProjectAnswers resolves project config from flags and auto-detection,
// mirroring the wizard defaults: flags win, then detection, then constants.
// Conditional multi-selects (env files, package scripts, docker compose,
// monorepo packages) take every detected value, matching the wizard's
// pre-selection.
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
		if installCommand != "" {
			answers.OnCreate = append(answers.OnCreate, domain.HookCommand{Cmd: installCommand})
			for _, pkg := range detection.MonorepoPackages {
				answers.OnCreate = append(answers.OnCreate, domain.HookCommand{Cmd: installCommand, Cwd: pkg})
			}
		}
	}

	return answers, nil
}

// AutoServicesAnswers builds the services portion of InitProjectAnswers from
// detection alone — every detected docker-compose file and package script — for
// the non-interactive `wtm run init` path. The base config fields are left
// zero-valued: only the services fields feed BuildInitRunConfig.
func AutoServicesAnswers(detection domain.InitDetectionResult) domain.InitProjectAnswers {
	answers := domain.InitProjectAnswers{
		SelectedPackageScripts: detection.PackageScripts,
	}
	if len(detection.DockerComposeFiles) > 0 {
		answers.DockerComposeFiles = detection.DockerComposeFiles
		answers.DockerComposeCmd = detection.DockerComposeCmd
	}
	return answers
}
