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

	// StateDirName is the wtm state directory inside the git common dir
	// (i.e. <git-common-dir>/wtm/). Never committed — git ignores .git/.
	StateDirName = "wtm"

	// WorktreesSubdir is the subdirectory under the state dir that holds
	// per-worktree metadata: <state-dir>/worktrees/<encoded-branch>/.
	WorktreesSubdir = "worktrees"

	// ConfigFileName is the project-level config file name (inside <state-dir>/).
	ConfigFileName = "config.toml"

	// GlobalConfigDir is the subdirectory under ~/.config for wtm.
	GlobalConfigDir = "wtm"

	// GlobalConfigFile is the user-level config file name.
	GlobalConfigFile = "config.toml"

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

	// EnvGoFile is the environment variable used by the shell wrapper to pass the go-file path.
	EnvGoFile = "WTM_GO_FILE"

	// Flag names.
	FlagFrom      = "from"
	FlagEnvFrom   = "env-from"
	FlagForce     = "force"
	FlagTitle     = "title"
	FlagBase      = "base"
	FlagDraft     = "draft"
	FlagExclusive = "exclusive"
	FlagParallel  = "parallel"
	FlagDetach    = "detach"
	FlagProfile   = "profile"
	FlagOutput    = "output"
	FlagYes       = "yes"
	FlagAll       = "all"
	FlagGlobal    = "global"
	FlagMerge     = "merge"
	FlagReplace   = "replace"
	FlagMine      = "mine"
	FlagReview    = "review"
	FlagCmd       = "cmd"
	FlagKind      = "kind"
	FlagStop      = "stop"
	FlagCwd       = "cwd"
	FlagJobs      = "jobs"
	FlagDefault   = "default"

	// Script classification keywords for package.json → run.toml mapping.
	// A script is classified as a long-running service when its name matches
	// one of these keywords exactly, as a prefix ("<kw>:"), or as a suffix (":<kw>").
	ScriptKeyDev   = "dev"
	ScriptKeyStart = "start"
	ScriptKeyServe = "serve"
	ScriptKeyWatch = "watch"

	// Output format values for FlagOutput.
	OutputText = "text"
	OutputJSON = "json"

	// RunFileName is the run config file name (inside <state-dir>/).
	RunFileName = "run.toml"

	// SchemasDirName is the directory (inside <state-dir>/ or under the global
	// config dir) where `wtm schema dump` writes the JSON Schema files
	// that editors reference via the TOML `#:schema` directive.
	SchemasDirName = "schemas"

	// Job action result statuses emitted by `run *` JSON output.
	JobActionStarted = "started"
	JobActionStopped = "stopped"
	JobActionDone    = "done"
	JobActionError   = "error"
	JobActionAdded   = "added"
	JobActionRemoved = "removed"
	JobActionUpdated = "updated"

	// Metadata files created per worktree inside <state-dir>/worktrees/<branch>/.
	MetaFileName    = "meta.json"
	ContextFileName = "context.md"

	// CmdGroupCore is the Cobra group ID for core commands.
	CmdGroupCore = "core"

	// CmdGroupSetup is the Cobra group ID for setup commands.
	CmdGroupSetup = "setup"

	// CLI command names — used in Use: declarations and exec.Command(bin, …) call sites.
	// Centralised here so a rename is a single-file change with no silent breakage.
	CmdWt       = "wt"
	CmdRun      = "run"
	CmdPr       = "pr"
	CmdGo       = "go"
	CmdCreate   = "create"
	CmdClean    = "clean"
	CmdList     = "list"
	CmdSwitch   = "switch"
	CmdUp       = "up"
	CmdDown     = "down"
	CmdStart    = "start"
	CmdStop     = "stop"
	CmdLogs     = "logs"
	CmdPs       = "ps"
	CmdCheckout = "checkout"
	CmdExport   = "export"
	CmdImport   = "import"
	CmdJob      = "job"
	CmdProfile  = "profile"
	CmdAdd      = "add"
	CmdRm       = "rm"
	CmdEdit     = "edit"

	// DaemonSocketName is the Unix socket filename for the service daemon.
	DaemonSocketName = "wtm.sock"

	// DaemonIdleTimeoutSeconds is how long the daemon waits with no services before auto-exit.
	DaemonIdleTimeoutSeconds = 30

	// DaemonStartTimeoutSeconds is how long to wait for the daemon to start.
	DaemonStartTimeoutSeconds = 5

	// CtrlCByte is the ASCII code for Ctrl+C, used for PTY detach.
	CtrlCByte byte = 0x03

	// JobAlreadyRunningSuffix is the tail of the daemon error returned when a
	// job is started while already running. Callers match on it to treat a
	// repeat start (e.g. re-running `run up` while services are up) as a benign
	// no-op rather than a failure that aborts the profile.
	JobAlreadyRunningSuffix = "is already running"
)
