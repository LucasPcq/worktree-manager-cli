// Package config loads and validates wtm configuration from project and user-level TOML files.
package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
	"github.com/LucasPcq/wtm/internal/domain"
)

// LoadParams holds the inputs for loading configuration.
type LoadParams struct {
	ProjectDir string
}

// Load reads project and global config files, merges them, applies defaults,
// and validates the result. Returns ErrConfigNotFound if .wtm.toml is absent.
func Load(params LoadParams) (domain.Config, error) {
	projectPath := filepath.Join(params.ProjectDir, domain.ConfigFileName)

	project, err := loadProjectConfig(projectPath)
	if err != nil {
		return domain.Config{}, fmt.Errorf("project config: %w", err)
	}

	global, err := loadGlobalConfig()
	if err != nil {
		return domain.Config{}, fmt.Errorf("global config: %w", err)
	}

	cfg := merge(project, global)
	applyDefaults(&cfg)

	if err := Validate(cfg); err != nil {
		return domain.Config{}, err
	}

	return cfg, nil
}

// loadProjectConfig reads and parses the project .wtm.toml file.
// Returns ErrConfigNotFound if the file does not exist.
func loadProjectConfig(path string) (domain.ProjectConfig, error) {
	if _, err := os.Stat(path); errors.Is(err, os.ErrNotExist) {
		return domain.ProjectConfig{}, domain.ErrConfigNotFound
	}

	var raw rawProjectConfig
	if _, err := toml.DecodeFile(path, &raw); err != nil {
		return domain.ProjectConfig{}, fmt.Errorf("parse %s: %w", path, err)
	}

	hooks, err := decodeHooksConfig(raw.Hooks)
	if err != nil {
		return domain.ProjectConfig{}, fmt.Errorf("parse hooks: %w", err)
	}

	cfg := domain.ProjectConfig{
		Worktrees: domain.WorktreesConfig{
			BasePath:   raw.Worktrees.BasePath,
			BaseBranch: raw.Worktrees.BaseBranch,
		},
		Env: domain.EnvConfig{
			Strategy:  domain.EnvStrategy(raw.Env.Strategy),
			CopyFiles: raw.Env.CopyFiles,
		},
		Hooks: hooks,
		Github: domain.GithubConfig{
			AutoDraft:  raw.Github.AutoDraft,
			BaseBranch: raw.Github.BaseBranch,
		},
		Agents: domain.AgentsConfig{
			Default: domain.AgentType(raw.Agents.Default),
		},
		Integrations: domain.IntegrationsConfig{
			VSCodeProjectManager: raw.Integrations.VSCodeProjectManager,
			CursorProjectManager: raw.Integrations.CursorProjectManager,
		},
	}

	return cfg, nil
}

// rawProjectConfig is the intermediate TOML-decoded structure.
// Hooks are decoded as raw interfaces to support the string | {cmd,cwd} union.
type rawProjectConfig struct {
	Worktrees struct {
		BasePath   string `toml:"base_path"`
		BaseBranch string `toml:"base_branch"`
	} `toml:"worktrees"`
	Env struct {
		Strategy  string   `toml:"strategy"`
		CopyFiles []string `toml:"copy_files"`
	} `toml:"env"`
	Hooks rawHooksConfig `toml:"hooks"`
	Github struct {
		AutoDraft  bool   `toml:"auto_draft"`
		BaseBranch string `toml:"base_branch"`
	} `toml:"github"`
	Agents struct {
		Default string `toml:"default"`
	} `toml:"agents"`
	Integrations struct {
		VSCodeProjectManager bool `toml:"vscode_project_manager"`
		CursorProjectManager bool `toml:"cursor_project_manager"`
	} `toml:"integrations"`
}

type rawHooksConfig struct {
	OnCreate []interface{} `toml:"on_create"`
	OnFocus  []interface{} `toml:"on_focus"`
	OnBlur   []interface{} `toml:"on_blur"`
}

func decodeHooksConfig(raw rawHooksConfig) (domain.HooksConfig, error) {
	onCreate, err := unmarshalHookCommands(raw.OnCreate)
	if err != nil {
		return domain.HooksConfig{}, fmt.Errorf("on_create: %w", err)
	}

	onFocus, err := unmarshalHookCommands(raw.OnFocus)
	if err != nil {
		return domain.HooksConfig{}, fmt.Errorf("on_focus: %w", err)
	}

	onBlur, err := unmarshalHookCommands(raw.OnBlur)
	if err != nil {
		return domain.HooksConfig{}, fmt.Errorf("on_blur: %w", err)
	}

	return domain.HooksConfig{
		OnCreate: onCreate,
		OnFocus:  onFocus,
		OnBlur:   onBlur,
	}, nil
}

// loadGlobalConfig reads ~/.config/wtm/config.toml. Returns zero-value GlobalConfig
// if the file does not exist (global config is optional).
func loadGlobalConfig() (domain.GlobalConfig, error) {
	configDir, err := os.UserConfigDir()
	if err != nil {
		return domain.GlobalConfig{}, nil
	}

	path := filepath.Join(configDir, domain.GlobalConfigDir, domain.GlobalConfigFile)

	if _, err := os.Stat(path); errors.Is(err, os.ErrNotExist) {
		return domain.GlobalConfig{}, nil
	}

	var cfg domain.GlobalConfig
	if _, err := toml.DecodeFile(path, &cfg); err != nil {
		return domain.GlobalConfig{}, fmt.Errorf("parse %s: %w", path, err)
	}

	return cfg, nil
}

// merge combines project and global configs. Project values take priority.
// Global values fill in only when the project value is the zero value.
func merge(project domain.ProjectConfig, global domain.GlobalConfig) domain.Config {
	cfg := domain.Config{
		Project: project,
		Global:  global,
	}

	if cfg.Project.Agents.Default == "" && global.Agent != "" {
		cfg.Project.Agents.Default = domain.AgentType(global.Agent)
	}

	return cfg
}

// applyDefaults fills zero-value fields with sensible defaults.
func applyDefaults(cfg *domain.Config) {
	if cfg.Project.Worktrees.BasePath == "" {
		cfg.Project.Worktrees.BasePath = domain.DefaultBasePath
	}
	if cfg.Project.Worktrees.BaseBranch == "" {
		cfg.Project.Worktrees.BaseBranch = domain.DefaultBaseBranch
	}
	if cfg.Project.Env.Strategy == "" {
		cfg.Project.Env.Strategy = domain.DefaultEnvStrategy
	}
	if cfg.Project.Agents.Default == "" {
		cfg.Project.Agents.Default = domain.DefaultAgent
	}
	if cfg.Global.Shell == "" {
		cfg.Global.Shell = domain.DefaultShell
	}
	if cfg.Global.Agent == "" {
		cfg.Global.Agent = domain.AgentType(domain.DefaultAgent)
	}
}
