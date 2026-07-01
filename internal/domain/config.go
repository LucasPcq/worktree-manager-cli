package domain

// ShellType represents a supported shell for integration.
type ShellType string

const (
	ShellZsh  ShellType = "zsh"
	ShellBash ShellType = "bash"
	ShellFish ShellType = "fish"
)

// ProjectConfig maps to .wtm.toml (project-level configuration).
type ProjectConfig struct {
	Worktrees WorktreesConfig `toml:"worktrees" json:"worktrees"`
	Env       EnvConfig       `toml:"env" json:"env"`
	Hooks     HooksConfig     `toml:"hooks" json:"hooks"`
}

// WorktreesConfig controls worktree creation defaults.
type WorktreesConfig struct {
	BasePath   string `toml:"base_path" json:"base_path"`
	BaseBranch string `toml:"base_branch" json:"base_branch"`
}

// EnvConfig controls .env file provisioning.
type EnvConfig struct {
	Strategy  EnvStrategy `toml:"strategy" json:"strategy"`
	CopyFiles []string    `toml:"copy_files" json:"copy_files"`
}

// HooksConfig defines lifecycle hooks as lists of commands.
type HooksConfig struct {
	OnCreate []HookCommand `toml:"on_create" json:"on_create"`
}

// GlobalConfig maps to ~/.config/wtm/config.toml (user-level configuration).
type GlobalConfig struct {
	Shell ShellType `toml:"shell"`
}

// Config is the merged, validated configuration used by the service layer.
type Config struct {
	Project ProjectConfig
	Global  GlobalConfig
}
