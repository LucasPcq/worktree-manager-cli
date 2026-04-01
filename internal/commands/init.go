package commands

import (
	"errors"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/LucasPcq/wtm/internal/config"
	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/service/detect"
	initwizard "github.com/LucasPcq/wtm/internal/tui/init"
)

// NewInitCmd creates the wtm init command.
func NewInitCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "init",
		Short: "Initialize wtm configuration",
		Long:  "Interactive wizard to set up global config and project .wtm.toml.",
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

	return ensureProjectConfig(cmd, dir)
}

func ensureGlobalConfig(cmd *cobra.Command) error {
	if detect.GlobalConfigExists() {
		return nil
	}

	fmt.Fprintln(cmd.OutOrStdout(), "No global config found. Let's set one up.")
	fmt.Fprintln(cmd.OutOrStdout())

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

	fmt.Fprintln(cmd.OutOrStdout(), "\n✓ Global config saved.")
	fmt.Fprintln(cmd.OutOrStdout())
	fmt.Fprintln(cmd.OutOrStdout(), domain.MsgShellInitHint)
	fmt.Fprintln(cmd.OutOrStdout())

	return nil
}

func ensureProjectConfig(cmd *cobra.Command, dir string) error {
	if detect.ProjectConfigExists(dir) {
		fmt.Fprintln(cmd.OutOrStdout(), domain.MsgConfigExists)
		return nil
	}

	fmt.Fprintln(cmd.OutOrStdout(), "No .wtm.toml found. Let's initialize this project.")
	fmt.Fprintln(cmd.OutOrStdout())

	baseBranch := detect.BaseBranch(dir)
	envFiles := detect.EnvFiles(dir)
	pm := detect.PackageManager(dir)
	installCmd := detect.InstallCommand(pm)

	detection := domain.InitDetectionResult{
		BaseBranch:     baseBranch,
		EnvFiles:       envFiles,
		PackageManager: pm,
		InstallCommand: installCmd,
	}

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

	fmt.Fprintf(cmd.OutOrStdout(), "\n✓ Created %s\n", domain.ConfigFileName)

	return nil
}
