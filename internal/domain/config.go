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
	Worktrees WorktreesConfig `toml:"worktrees"`
	Env       EnvConfig       `toml:"env"`
	Hooks     HooksConfig     `toml:"hooks"`
	Github    GithubConfig    `toml:"github"`
}

// WorktreesConfig controls worktree creation defaults.
type WorktreesConfig struct {
	BasePath   string `toml:"base_path"`
	BaseBranch string `toml:"base_branch"`
}

// EnvConfig controls .env file provisioning.
type EnvConfig struct {
	Strategy  EnvStrategy `toml:"strategy"`
	CopyFiles []string    `toml:"copy_files"`
}

// HooksConfig defines lifecycle hooks as lists of commands.
type HooksConfig struct {
	OnCreate []HookCommand `toml:"on_create"`
}

// GithubConfig controls GitHub integration behavior.
type GithubConfig struct {
	AutoDraft bool `toml:"auto_draft"`
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
