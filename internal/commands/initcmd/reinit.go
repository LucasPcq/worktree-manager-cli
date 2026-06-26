package initcmd

import (
	"errors"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/LucasPcq/wtm/internal/config"
	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/output"
	"github.com/LucasPcq/wtm/internal/rules"
	"github.com/LucasPcq/wtm/internal/service/detect"
	"github.com/LucasPcq/wtm/internal/tui/components"
	initwizard "github.com/LucasPcq/wtm/internal/tui/inittui"
)

// parseSections validates and de-duplicates the --only values (CSV or repeated),
// preserving order. Returns ExitCodeUsage-wrapped error on an unknown section.
func parseSections(raw []string) ([]string, error) {
	valid := map[string]bool{
		domain.SectionWorktrees: true,
		domain.SectionEnv:       true,
		domain.SectionHooks:     true,
		domain.SectionServices:  true,
	}
	seen := map[string]bool{}
	var sections []string
	for _, entry := range raw {
		for _, name := range strings.Split(entry, ",") {
			name = strings.TrimSpace(name)
			if name == "" {
				continue
			}
			if !valid[name] {
				return nil, fmt.Errorf("unknown section %q for --%s (valid: %s, %s, %s, %s)",
					name, domain.FlagOnly, domain.SectionWorktrees, domain.SectionEnv, domain.SectionHooks, domain.SectionServices)
			}
			if !seen[name] {
				seen[name] = true
				sections = append(sections, name)
			}
		}
	}
	return sections, nil
}

// runReinit re-runs init for the requested sections only, regenerating each
// section cleanly. config.toml sections preserve all untouched values; run.toml
// regenerates jobs while preserving profiles.
func runReinit(cmd *cobra.Command, dir, stateDir string, sections []string) error {
	if !detect.ProjectConfigExists(stateDir) {
		return fmt.Errorf("no wtm config found — run `wtm init` first: %w", domain.ErrConfigNotFound)
	}

	detection := detect.ProjectEnvironment(dir)

	nonInteractive, _ := cmd.Flags().GetBool(domain.FlagNonInteractive)

	var answers domain.InitProjectAnswers
	if nonInteractive {
		built, err := buildReinitAnswers(cmd, stateDir, detection)
		if err != nil {
			return err
		}
		answers = built
	} else {
		prefill, err := buildPrefill(stateDir, detection)
		if err != nil {
			return err
		}
		wizardAnswers, err := initwizard.RunSectionWizard(sections, detection, prefill)
		if errors.Is(err, domain.ErrUserAborted) {
			return nil
		}
		if err != nil {
			return err
		}
		answers = wizardAnswers
	}

	yes, _ := cmd.Flags().GetBool(domain.FlagYes)
	if !yes && !nonInteractive {
		confirmed, err := components.RunStandaloneConfirm(components.NewConfirm(components.NewConfirmParams{
			Title:       "Re-initialize " + strings.Join(sections, ", "),
			Description: "This regenerates the selected section(s) cleanly.",
			Warning:     reinitWarning(sections),
		}))
		if errors.Is(err, components.ErrAborted) || (err == nil && !confirmed) {
			output.Frame(cmd.OutOrStdout(), func() {
				output.Message(cmd.OutOrStdout(), "Cancelled.")
			})
			return nil
		}
		if err != nil {
			return err
		}
	}

	output.FrameStart(cmd.OutOrStdout())

	if contains(sections, domain.SectionServices) {
		if err := applyServicesReinit(cmd, stateDir, answers, detection.PackageManager); err != nil {
			return err
		}
	}
	if contains(sections, domain.SectionWorktrees) || contains(sections, domain.SectionEnv) || contains(sections, domain.SectionHooks) {
		if err := applyConfigReinit(cmd, stateDir, sections, answers); err != nil {
			return err
		}
	}

	output.FrameEnd(cmd.OutOrStdout())
	return nil
}

// buildPrefill snapshots the current config + run files so the interactive
// re-init wizard pre-selects what's already configured.
func buildPrefill(stateDir string, detection domain.InitDetectionResult) (*initwizard.SectionPrefill, error) {
	cfg, err := config.LoadProjectRaw(stateDir)
	if err != nil {
		return nil, fmt.Errorf("load project config: %w", err)
	}
	runCfg, err := config.LoadRun(stateDir)
	if err != nil {
		return nil, fmt.Errorf("load run config: %w", err)
	}

	return &initwizard.SectionPrefill{
		BaseBranch:    cfg.Worktrees.BaseBranch,
		EnvStrategy:   string(cfg.Env.Strategy),
		EnvCopyFiles:  toSet(cfg.Env.CopyFiles),
		OnCreate:      cfg.Hooks.OnCreate,
		DockerFiles:   rules.DockerFilesConfigured(runCfg, detection.DockerComposeFiles),
		ScriptIndices: rules.ScriptsConfigured(runCfg, detection.PackageScripts, detection.PackageManager),
	}, nil
}

func toSet(values []string) map[string]bool {
	set := make(map[string]bool, len(values))
	for _, v := range values {
		set[v] = true
	}
	return set
}

// buildReinitAnswers resolves answers for the non-interactive path. Scalar
// values (base branch, env strategy, install command) keep their current config
// value unless a flag overrides them; the detected lists (env files, docker,
// scripts, monorepo) are regenerated from detection. NonInteractive is left
// false so an unresolved base branch falls back to a default rather than erroring.
func buildReinitAnswers(cmd *cobra.Command, stateDir string, detection domain.InitDetectionResult) (domain.InitProjectAnswers, error) {
	cfg, err := config.LoadProjectRaw(stateDir)
	if err != nil {
		return domain.InitProjectAnswers{}, fmt.Errorf("load project config: %w", err)
	}

	baseBranch, _ := cmd.Flags().GetString(domain.FlagBaseBranch)
	if baseBranch == "" {
		baseBranch = cfg.Worktrees.BaseBranch
	}
	envStrategy, _ := cmd.Flags().GetString(domain.FlagEnvStrategy)
	if envStrategy == "" {
		envStrategy = string(cfg.Env.Strategy)
	}
	installCommand, _ := cmd.Flags().GetString(domain.FlagInstallCommand)
	if installCommand == "" {
		installCommand = rules.InstallCommandFromHooks(cfg.Hooks.OnCreate)
	}

	return rules.BuildProjectAnswers(rules.InitProjectFlags{
		BaseBranch:     baseBranch,
		EnvStrategy:    envStrategy,
		InstallCommand: installCommand,
	}, detection)
}

// applyServicesReinit regenerates run.toml jobs from detection while preserving
// the existing profiles.
func applyServicesReinit(cmd *cobra.Command, stateDir string, answers domain.InitProjectAnswers, pm domain.PackageManager) error {
	existing, err := config.LoadRun(stateDir)
	if err != nil {
		return fmt.Errorf("load run config: %w", err)
	}

	built := rules.BuildInitRunConfig(answers, pm)
	newRun := domain.RunConfig{Jobs: built.Jobs, Profiles: existing.Profiles}

	if err := config.WriteRun(config.WriteRunParams{StateDir: stateDir, Config: newRun, Force: true}); err != nil {
		return fmt.Errorf("write run config: %w", err)
	}

	output.Success(cmd.OutOrStdout(), fmt.Sprintf("Regenerated run.toml: %d job(s), %d profile(s) preserved", len(newRun.Jobs), len(newRun.Profiles)))
	return nil
}

// applyConfigReinit rewrites config.toml, updating only the requested sections
// and preserving every other section's current values.
func applyConfigReinit(cmd *cobra.Command, stateDir string, sections []string, answers domain.InitProjectAnswers) error {
	cfg, err := config.LoadProjectRaw(stateDir)
	if err != nil {
		return fmt.Errorf("load project config: %w", err)
	}

	if contains(sections, domain.SectionWorktrees) {
		cfg.Worktrees.BaseBranch = answers.BaseBranch
	}
	if contains(sections, domain.SectionEnv) {
		cfg.Env.Strategy = answers.EnvStrategy
		cfg.Env.CopyFiles = answers.EnvCopyFiles
	}
	if contains(sections, domain.SectionHooks) {
		cfg.Hooks.OnCreate = answers.OnCreate
	}

	if err := config.WriteProjectConfig(config.WriteProjectConfigParams{StateDir: stateDir, Config: cfg}); err != nil {
		return fmt.Errorf("write project config: %w", err)
	}

	output.Success(cmd.OutOrStdout(), "Rewrote config.toml")
	return nil
}

// reinitWarning describes what each requested section's regeneration will touch.
func reinitWarning(sections []string) string {
	var lines []string
	for _, s := range sections {
		switch s {
		case domain.SectionWorktrees:
			lines = append(lines, "config.toml [worktrees] base_branch will be rewritten")
		case domain.SectionEnv:
			lines = append(lines, "config.toml [env] will be rewritten")
		case domain.SectionHooks:
			lines = append(lines, "config.toml [hooks] will be rewritten")
		case domain.SectionServices:
			lines = append(lines, "run.toml jobs will be regenerated (profiles preserved)")
		}
	}
	return strings.Join(lines, "; ")
}

func contains(list []string, want string) bool {
	for _, v := range list {
		if v == want {
			return true
		}
	}
	return false
}
