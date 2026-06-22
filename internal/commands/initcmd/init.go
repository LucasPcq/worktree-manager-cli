package initcmd

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/LucasPcq/wtm/internal/commands/shared"
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
	cmd := &cobra.Command{
		Use:   "init",
		Short: "Initialize wtm configuration",
		Long: "Interactive wizard to set up global config and project config in <git-common-dir>/wtm/config.toml.\n" +
			"Pass --non-interactive (or any config flag) to bootstrap from flags + auto-detection instead.\n" +
			"Use --only env|hooks|services to re-run init for specific sections and regenerate them cleanly\n" +
			"(run.toml jobs are regenerated while profiles are preserved).",
		RunE:  runInit,
	}

	cmd.Flags().Bool(domain.FlagNonInteractive, false, "Bootstrap from flags + auto-detection; never prompt")
	cmd.Flags().String(domain.FlagShell, "", "Global shell: zsh, bash, or fish")
	cmd.Flags().String(domain.FlagBasePath, "", "Worktree directory, relative to repo root")
	cmd.Flags().String(domain.FlagBaseBranch, "", "Default base branch for new worktrees")
	cmd.Flags().String(domain.FlagEnvStrategy, "", "Env provisioning strategy: example, main, or parent")
	cmd.Flags().String(domain.FlagInstallCommand, "", "Command to run after creating a worktree")
	cmd.Flags().Bool(domain.FlagSkipEnv, false, "Skip .env provisioning config")
	cmd.Flags().Bool(domain.FlagSkipHooks, false, "Skip on_create hooks config")
	cmd.Flags().Bool(domain.FlagSkipServices, false, "Skip service/task detection (docker, scripts)")
	cmd.Flags().StringSlice(domain.FlagOnly, nil, "Re-init only these sections (env, hooks, services); regenerates them cleanly")
	cmd.Flags().Bool(domain.FlagYes, false, "Skip the re-init confirmation prompt")

	return cmd
}

// initFlagged reports whether the user passed --non-interactive or any of the
// value flags, which switches init into the non-interactive, flag-driven path.
func initFlagged(cmd *cobra.Command) bool {
	for _, name := range []string{
		domain.FlagNonInteractive, domain.FlagShell,
		domain.FlagBasePath, domain.FlagBaseBranch, domain.FlagEnvStrategy,
		domain.FlagInstallCommand, domain.FlagSkipEnv, domain.FlagSkipHooks,
		domain.FlagSkipServices,
	} {
		if cmd.Flags().Changed(name) {
			return true
		}
	}
	return false
}

func runInit(cmd *cobra.Command, _ []string) error {
	dir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get working directory: %w", err)
	}

	flagged := initFlagged(cmd)

	if err := ensureGlobalConfig(cmd, flagged); err != nil {
		return err
	}

	stateDir, err := shared.StateDir(dir)
	if err != nil {
		return fmt.Errorf("wtm must be run inside a git repository: %w", err)
	}

	if only, _ := cmd.Flags().GetStringSlice(domain.FlagOnly); len(only) > 0 {
		sections, err := parseSections(only)
		if err != nil {
			return err
		}
		return runReinit(cmd, dir, stateDir, sections)
	}

	if detect.ProjectConfigExists(stateDir) {
		output.Blank(cmd.OutOrStdout())
		output.Message(cmd.OutOrStdout(), fmt.Sprintf("%s already exists.", filepath.Join(stateDir, domain.ConfigFileName)))
		output.Message(cmd.OutOrStdout(), "Reconfigure a section with `wtm init --only env|hooks|services|worktrees`, or edit by hand with `wtm config edit`.")
		output.Blank(cmd.OutOrStdout())
		return nil
	}

	return createProjectConfig(cmd, dir, stateDir, flagged)
}

func ensureGlobalConfig(cmd *cobra.Command, flagged bool) error {
	if detect.GlobalConfigExists() {
		return nil
	}

	answers, err := resolveGlobalAnswers(cmd, flagged)
	if errors.Is(err, domain.ErrUserAborted) {
		return nil
	}
	if err != nil {
		return err
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

// resolveGlobalAnswers builds the global config either from flags (non-interactive)
// or the interactive wizard.
func resolveGlobalAnswers(cmd *cobra.Command, flagged bool) (domain.InitGlobalAnswers, error) {
	if flagged {
		shell, _ := cmd.Flags().GetString(domain.FlagShell)
		return rules.BuildGlobalAnswers(rules.InitGlobalFlags{Shell: shell})
	}

	output.Message(cmd.OutOrStdout(), "No global config found. Let's set one up.")

	answers, err := initwizard.RunGlobalWizard()
	if err != nil {
		if errors.Is(err, domain.ErrUserAborted) {
			return domain.InitGlobalAnswers{}, err
		}
		return domain.InitGlobalAnswers{}, fmt.Errorf("global wizard: %w", err)
	}
	return answers, nil
}

// resolveProjectAnswers builds the project config either from flags + detection
// (non-interactive) or the interactive wizard.
func resolveProjectAnswers(cmd *cobra.Command, flagged bool, detection domain.InitDetectionResult) (domain.InitProjectAnswers, error) {
	if flagged {
		nonInteractive, _ := cmd.Flags().GetBool(domain.FlagNonInteractive)
		basePath, _ := cmd.Flags().GetString(domain.FlagBasePath)
		baseBranch, _ := cmd.Flags().GetString(domain.FlagBaseBranch)
		envStrategy, _ := cmd.Flags().GetString(domain.FlagEnvStrategy)
		installCommand, _ := cmd.Flags().GetString(domain.FlagInstallCommand)
		skipEnv, _ := cmd.Flags().GetBool(domain.FlagSkipEnv)
		skipHooks, _ := cmd.Flags().GetBool(domain.FlagSkipHooks)
		skipServices, _ := cmd.Flags().GetBool(domain.FlagSkipServices)
		return rules.BuildProjectAnswers(rules.InitProjectFlags{
			BasePath:       basePath,
			BaseBranch:     baseBranch,
			EnvStrategy:    envStrategy,
			InstallCommand: installCommand,
			NonInteractive: nonInteractive,
			SkipEnv:        skipEnv,
			SkipHooks:      skipHooks,
			SkipServices:   skipServices,
		}, detection)
	}

	answers, err := initwizard.RunProjectWizard(detection)
	if err != nil {
		if errors.Is(err, domain.ErrUserAborted) {
			return domain.InitProjectAnswers{}, err
		}
		return domain.InitProjectAnswers{}, fmt.Errorf("project wizard: %w", err)
	}
	return answers, nil
}

func createProjectConfig(cmd *cobra.Command, dir, stateDir string, flagged bool) error {
	if !flagged {
		output.Blank(cmd.OutOrStdout())
		output.Intro(cmd.OutOrStdout(), "No wtm config found for this repo. Let's initialize it.")
	}

	stop := shared.StartSpinner(cmd.ErrOrStderr(), "Detecting project settings…")
	detection := detect.ProjectEnvironment(dir)
	stop()

	answers, err := resolveProjectAnswers(cmd, flagged, detection)
	if errors.Is(err, domain.ErrUserAborted) {
		return nil
	}
	if err != nil {
		return err
	}

	if err := config.WriteProject(config.WriteProjectParams{
		StateDir: stateDir,
		Answers:  answers,
	}); err != nil {
		return fmt.Errorf("write project config: %w", err)
	}

	output.Success(cmd.OutOrStdout(), fmt.Sprintf("Created %s", filepath.Join(stateDir, domain.ConfigFileName)))

	if err := dumpProjectSchemas(stateDir); err != nil {
		return err
	}

	runCfg := rules.BuildInitRunConfig(answers, detection.PackageManager)
	runPath := filepath.Join(stateDir, domain.RunFileName)
	var runErr error
	if len(runCfg.Jobs) > 0 {
		runErr = config.WriteRun(config.WriteRunParams{
			StateDir: stateDir,
			Config:   runCfg,
		})
	} else {
		runErr = config.WriteRunTemplate(config.WriteRunParams{StateDir: stateDir})
	}
	if errors.Is(runErr, config.ErrRunFileExists) {
		output.Message(cmd.OutOrStdout(), fmt.Sprintf("%s already exists — left untouched", runPath))
	} else if runErr != nil {
		return fmt.Errorf("write run config: %w", runErr)
	} else {
		output.Success(cmd.OutOrStdout(), fmt.Sprintf("Created %s", runPath))
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

// dumpProjectSchemas extracts the bundled JSON Schemas into <state-dir>/schemas/
// alongside the project config files so editors (Taplo, etc.) can resolve
// the `#:schema ./schemas/...json` directive at the top of each TOML.
func dumpProjectSchemas(stateDir string) error {
	schemaDir := filepath.Join(stateDir, domain.SchemasDirName)
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
