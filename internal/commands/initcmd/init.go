package initcmd

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/LucasPcq/wtm/internal/config"
	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/output"
	"github.com/LucasPcq/wtm/internal/rules"
	"github.com/LucasPcq/wtm/internal/schemas"
	"github.com/LucasPcq/wtm/internal/service/detect"
	initwizard "github.com/LucasPcq/wtm/internal/tui/inittui"
)

// NewCmd creates the wtm init command.
func NewCmd() *cobra.Command {
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
	if err := dumpGlobalSchema(); err != nil {
		return err
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

	detection := detect.ProjectEnvironment(dir)

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

	if err := dumpProjectSchemas(dir); err != nil {
		return err
	}

	runCfg := rules.BuildInitRunConfig(answers, detection.PackageManager)
	if len(runCfg.Jobs) > 0 {
		err := config.WriteRun(config.WriteRunParams{
			ProjectDir: dir,
			Config:     runCfg,
		})
		if errors.Is(err, config.ErrRunFileExists) {
			output.Message(cmd.OutOrStdout(), fmt.Sprintf("%s/%s already exists — left untouched", domain.ProjectDirName, domain.RunFileName))
		} else if err != nil {
			return fmt.Errorf("write run config: %w", err)
		} else {
			output.Success(cmd.OutOrStdout(), fmt.Sprintf("Created %s/%s", domain.ProjectDirName, domain.RunFileName))
		}
	}

	output.Blank(cmd.OutOrStdout())
	return nil
}

// dumpGlobalSchema writes the global config schema next to ~/.config/wtm/
// config.toml so editors can resolve its `#:schema` directive.
func dumpGlobalSchema() error {
	configDir, err := os.UserConfigDir()
	if err != nil {
		return nil // best effort — global schema dump isn't critical
	}
	dir := filepath.Join(configDir, domain.GlobalConfigDir, domain.SchemasDirName)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create %s: %w", dir, err)
	}
	path := filepath.Join(dir, schemas.Global.Filename())
	if err := os.WriteFile(path, schemas.Global.Bytes(), 0o644); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}
	return nil
}

// dumpProjectSchemas extracts the bundled JSON Schemas into .wtm/schemas/
// alongside the project config files so editors (Taplo, etc.) can resolve
// the `#:schema ./schemas/...json` directive at the top of each TOML.
func dumpProjectSchemas(projectDir string) error {
	schemaDir := filepath.Join(projectDir, domain.ProjectDirName, domain.SchemasDirName)
	if err := os.MkdirAll(schemaDir, 0o755); err != nil {
		return fmt.Errorf("create %s: %w", schemaDir, err)
	}
	for _, s := range []schemas.Schema{schemas.Project, schemas.Run} {
		path := filepath.Join(schemaDir, s.Filename())
		if err := os.WriteFile(path, s.Bytes(), 0o644); err != nil {
			return fmt.Errorf("write %s: %w", path, err)
		}
	}
	return nil
}
