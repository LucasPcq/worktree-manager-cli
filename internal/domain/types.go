package domain

// Worktree represents a git worktree managed by wtm.
type Worktree struct {
	Name   string
	Path   string
	Branch string
}

// HookEvent identifies when a hook should fire.
type HookEvent string

const (
	// HookOnCreate fires after a new worktree is created.
	HookOnCreate HookEvent = "on_create"

	// HookOnFocus fires when switching to a worktree.
	HookOnFocus HookEvent = "on_focus"

	// HookOnBlur fires when leaving a worktree.
	HookOnBlur HookEvent = "on_blur"
)

// EnvStrategy determines how .env files are provisioned in new worktrees.
type EnvStrategy string

const (
	// EnvStrategyExample copies .env.example.
	EnvStrategyExample EnvStrategy = "example"

	// EnvStrategyMain copies .env from the main worktree.
	EnvStrategyMain EnvStrategy = "main"

	// EnvStrategyParent copies .env from the parent worktree.
	EnvStrategyParent EnvStrategy = "parent"
)

// ShellType represents a supported shell for integration.
type ShellType string

const (
	ShellZsh  ShellType = "zsh"
	ShellBash ShellType = "bash"
	ShellFish ShellType = "fish"
)

// AgentType represents a supported AI agent.
type AgentType string

const (
	AgentClaudeCode AgentType = "claude-code"
	AgentCursor     AgentType = "cursor"
	AgentNone       AgentType = "none"
)

// HookCommand represents a hook entry — either a simple command string or a {cmd, cwd} pair.
type HookCommand struct {
	Cmd string `toml:"cmd"`
	Cwd string `toml:"cwd"`
}

// ProjectConfig maps to .wtm.toml (project-level configuration).
type ProjectConfig struct {
	Worktrees    WorktreesConfig    `toml:"worktrees"`
	Env          EnvConfig          `toml:"env"`
	Hooks        HooksConfig        `toml:"hooks"`
	Github       GithubConfig       `toml:"github"`
	Agents       AgentsConfig       `toml:"agents"`
	Integrations IntegrationsConfig `toml:"integrations"`
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
	OnFocus  []HookCommand `toml:"on_focus"`
	OnBlur   []HookCommand `toml:"on_blur"`
}

// GithubConfig controls GitHub integration behavior.
type GithubConfig struct {
	AutoDraft  bool   `toml:"auto_draft"`
	BaseBranch string `toml:"base_branch"`
}

// AgentsConfig controls the default AI agent.
type AgentsConfig struct {
	Default AgentType `toml:"default"`
}

// IntegrationsConfig toggles third-party editor integrations.
type IntegrationsConfig struct {
	VSCodeProjectManager bool `toml:"vscode_project_manager"`
	CursorProjectManager bool `toml:"cursor_project_manager"`
}

// GlobalConfig maps to ~/.config/wtm/config.toml (user-level configuration).
type GlobalConfig struct {
	Shell ShellType `toml:"shell"`
	Agent AgentType `toml:"agent"`
}

// Config is the merged, validated configuration used by the service layer.
type Config struct {
	Project ProjectConfig
	Global  GlobalConfig
}

// PackageManager represents a detected package manager.
type PackageManager string

const (
	PkgManagerPnpm PackageManager = "pnpm"
	PkgManagerNpm  PackageManager = "npm"
	PkgManagerYarn PackageManager = "yarn"
	PkgManagerGo   PackageManager = "go"
	PkgManagerPip  PackageManager = "pip"
	PkgManagerNone PackageManager = "none"
)

// InitDetectionResult holds all auto-detected values for repo init.
type InitDetectionResult struct {
	BaseBranch     string
	EnvFiles       []string
	PackageManager PackageManager
	InstallCommand string
}

// InitGlobalAnswers holds the wizard answers for global config setup.
type InitGlobalAnswers struct {
	Agent AgentType
	Shell ShellType
}

// InitProjectAnswers holds the wizard answers for project config setup.
type InitProjectAnswers struct {
	BasePath       string
	BaseBranch     string
	EnvCopyFiles   []string
	EnvStrategy    EnvStrategy
	InstallCommand string
	Agent          AgentType
	AgentOverride  bool
}
