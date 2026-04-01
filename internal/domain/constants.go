// Package domain defines shared types, constants, and errors for the wtm CLI.
package domain

const (
	// AppName is the canonical name of the CLI binary.
	AppName = "wtm"

	// Version is the current release version, overridden at build time via ldflags.
	Version = "dev"

	// ExitCodeOK indicates successful execution.
	ExitCodeOK = 0

	// ExitCodeError indicates a generic runtime error.
	ExitCodeError = 1

	// ExitCodeUsage indicates invalid usage or bad input.
	ExitCodeUsage = 2

	// ConfigFileName is the project-level config file name.
	ConfigFileName = ".wtm.toml"

	// GlobalConfigDir is the subdirectory under ~/.config for wtm.
	GlobalConfigDir = "wtm"

	// GlobalConfigFile is the user-level config file name.
	GlobalConfigFile = "config.toml"

	// StateFileName is the global state file name.
	StateFileName = "state.json"

	// DefaultBasePath is the default directory for worktrees, relative to project root.
	// One level up so worktrees are created outside the main repo directory.
	DefaultBasePath = "../.trees"

	// DefaultBaseBranch is the default base branch for new worktrees.
	DefaultBaseBranch = "main"

	// DefaultEnvStrategy is the default .env provisioning strategy.
	DefaultEnvStrategy = EnvStrategyExample

	// DefaultAgent is the default AI agent.
	DefaultAgent = AgentClaudeCode

	// DefaultShell is the default shell for integration.
	DefaultShell = ShellZsh

	// MsgConfigExists is shown when .wtm.toml already exists during init.
	MsgConfigExists = ".wtm.toml already exists. Re-edit mode coming in a future release."

	// MsgShellInitHint tells the user how to set up shell integration.
	MsgShellInitHint = "Add this to your shell config:\n\n  eval \"$(wtm shell-init)\""

	// Lockfile names for package manager detection.
	LockfilePnpm = "pnpm-lock.yaml"
	LockfileNpm  = "package-lock.json"
	LockfileYarn = "yarn.lock"
	LockfileGo   = "go.mod"
	LockfilePip  = "requirements.txt"

	// Install commands per package manager.
	InstallCmdPnpm = "pnpm install"
	InstallCmdNpm  = "npm install"
	InstallCmdYarn = "yarn install"
	InstallCmdGo   = "go mod download"
	InstallCmdPip  = "pip install -r requirements.txt"

	// Flag names for the new command.
	FlagFrom    = "from"
	FlagEnvFrom = "env-from"

	// Metadata directory and files created inside each worktree.
	MetaDirName     = ".wtm"
	MetaFileName    = "meta.json"
	ContextFileName = "context.md"
)
