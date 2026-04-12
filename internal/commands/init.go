package commands

import (
	"errors"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/LucasPcq/wtm/internal/config"
	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/output"
	"github.com/LucasPcq/wtm/internal/service/detect"
	initwizard "github.com/LucasPcq/wtm/internal/tui/init"
)

// NewInitCmd creates the wtm init command.
func NewInitCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "init",
		Short: "Initialize wtm configuration",
		Long:  "Interactive wizard to set up global config and project .wtm/config.toml.",
		RunE:  runInit,
	}
}

func runInit(cmd *cobra.Command, _ []string) error {
	dir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get working directory: %w", err)
	}

	if err := ensureGlobalConfig(cmd); err != nil {
		return err
	}

	if detect.ProjectConfigExists(dir) {
		output.Blank(cmd.OutOrStdout())
		output.Message(cmd.OutOrStdout(), ".wtm/config.toml already exists. Delete it or edit it manually to reconfigure.")
		output.Blank(cmd.OutOrStdout())
		return nil
	}

	return createProjectConfig(cmd, dir)
}

func ensureGlobalConfig(cmd *cobra.Command) error {
	if detect.GlobalConfigExists() {
		return nil
	}

	output.Message(cmd.OutOrStdout(), "No global config found. Let's set one up.")
	output.Blank(cmd.OutOrStdout())

	answers, err := initwizard.RunGlobalWizard()
	if errors.Is(err, domain.ErrUserAborted) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("global wizard: %w", err)
	}

	if err := config.WriteGlobal(answers); err != nil {
		return fmt.Errorf("write global config: %w", err)
	}

	output.Blank(cmd.OutOrStdout())
	output.Success(cmd.OutOrStdout(), "Global config saved.")
	output.Blank(cmd.OutOrStdout())
	output.Message(cmd.OutOrStdout(), domain.MsgShellInitHint)
	output.Blank(cmd.OutOrStdout())

	return nil
}

func createProjectConfig(cmd *cobra.Command, dir string) error {
	output.Blank(cmd.OutOrStdout())
	output.Intro(cmd.OutOrStdout(), "No .wtm/config.toml found. Let's initialize this project.")
	output.Blank(cmd.OutOrStdout())

	detection := buildDetectionResult(dir)

	answers, err := initwizard.RunProjectWizard(detection)
	if errors.Is(err, domain.ErrUserAborted) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("project wizard: %w", err)
	}

	if err := config.WriteProject(config.WriteProjectParams{
		ProjectDir: dir,
		Answers:    answers,
	}); err != nil {
		return fmt.Errorf("write project config: %w", err)
	}

	output.Success(cmd.OutOrStdout(), fmt.Sprintf("Created %s/%s", domain.ProjectDirName, domain.ConfigFileName))

	if len(answers.DockerComposeFiles) > 0 {
		servicesCfg := config.BuildDockerServices(answers.DockerComposeCmd, answers.DockerComposeFiles)
		err := config.WriteServices(config.WriteServicesParams{
			ProjectDir: dir,
			Config:     servicesCfg,
		})
		if errors.Is(err, config.ErrServicesFileExists) {
			output.Message(cmd.OutOrStdout(), fmt.Sprintf("%s/%s already exists — left untouched", domain.ProjectDirName, domain.ServicesFileName))
		} else if err != nil {
			return fmt.Errorf("write services config: %w", err)
		} else {
			output.Success(cmd.OutOrStdout(), fmt.Sprintf("Created %s/%s", domain.ProjectDirName, domain.ServicesFileName))
		}
	}

	output.Blank(cmd.OutOrStdout())
	return nil
}

func buildDetectionResult(dir string) domain.InitDetectionResult {
	pm := detect.PackageManager(dir)
	return domain.InitDetectionResult{
		BaseBranch:         detect.BaseBranch(dir),
		EnvFiles:           detect.EnvFiles(dir),
		PackageManager:     pm,
		InstallCommand:     detect.InstallCommand(pm),
		DockerComposeFiles: detect.DockerComposeFiles(dir),
		DockerComposeCmd:   detect.DockerComposeCommand(),
		MonorepoPackages:   detect.PnpmWorkspacePackages(dir),
	}
}

