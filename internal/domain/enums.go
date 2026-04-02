package domain

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
