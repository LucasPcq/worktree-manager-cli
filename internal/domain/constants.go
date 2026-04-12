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

	// ProjectDirName is the project-level wtm directory.
	ProjectDirName = ".wtm"

	// ConfigFileName is the project-level config file name (inside .wtm/).
	ConfigFileName = "config.toml"

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

	// MsgShellInitHint tells the user how to set up shell integration.
	MsgShellInitHint = "Add this to your shell config:\n\n  eval \"$(wtm shell-init)\""

	// Lockfile names for package manager detection.
	LockfilePnpm = "pnpm-lock.yaml"
	LockfileNpm  = "package-lock.json"
	LockfileYarn = "yarn.lock"
	LockfileGo   = "go.mod"
	LockfilePip  = "requirements.txt"

	// Install commands per package manager.
	InstallCommandPnpm = "pnpm install"
	InstallCommandNpm  = "npm install"
	InstallCommandYarn = "yarn install"
	InstallCommandGo   = "go mod download"
	InstallCommandPip  = "pip install -r requirements.txt"

	// Flag names.
	FlagFrom      = "from"
	FlagEnvFrom   = "env-from"
	FlagForce     = "force"
	FlagTitle     = "title"
	FlagBase      = "base"
	FlagDraft     = "draft"
	FlagExclusive = "exclusive"
	FlagParallel  = "parallel"
	FlagProfile   = "profile"

	// Metadata files created inside each worktree's .wtm/ directory.
	MetaFileName    = "meta.json"
	ContextFileName = "context.md"

	// CmdGroupCore is the Cobra group ID for core commands.
	CmdGroupCore = "core"

	// CmdGroupSetup is the Cobra group ID for setup commands.
	CmdGroupSetup = "setup"

	// DaemonSocketName is the Unix socket filename for the service daemon.
	DaemonSocketName = "wtm.sock"

	// DaemonIdleTimeoutSeconds is how long the daemon waits with no services before auto-exit.
	DaemonIdleTimeoutSeconds = 30

	// DaemonStartTimeoutSeconds is how long to wait for the daemon to start.
	DaemonStartTimeoutSeconds = 5

	// CtrlCByte is the ASCII code for Ctrl+C, used for PTY detach.
	CtrlCByte byte = 0x03

	// FeatureDashboard controls whether the interactive dashboard is available.
	FeatureDashboard = false
)
