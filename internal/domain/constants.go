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

	// Granular exit codes let LLM agents branch precisely on failure cause.
	ExitCodeWorktreeExists  = 10 // a worktree or its path already exists
	ExitCodeBranchNotFound  = 11 // the requested branch does not exist locally
	ExitCodeConfigNotFound  = 12 // the repo has no wtm config (run `wtm init`)
	ExitCodeServiceNotFound = 14 // the referenced job is not declared in run.toml
	ExitCodeExtractConflict = 15 // selected changes do not apply cleanly onto the target worktree

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
	FlagFrom       = "from"
	FlagEnvFrom    = "env-from"
	FlagForce      = "force"
	FlagBase       = "base"
	FlagExclusive  = "exclusive"
	FlagParallel   = "parallel"
	FlagDetach     = "detach"
	FlagProfile    = "profile"
	FlagOutput     = "output"
	FlagYes        = "yes"
	FlagAll        = "all"
	FlagGlobal     = "global"
	FlagMerge      = "merge"
	FlagReplace    = "replace"
	FlagMine       = "mine"
	FlagReview     = "review"
	FlagCmd        = "cmd"
	FlagKind       = "kind"
	FlagStop       = "stop"
	FlagCwd        = "cwd"
	FlagJobs       = "jobs"
	FlagDefault    = "default"
	FlagTo         = "to"
	FlagKeep       = "keep"
	FlagFiles      = "files"
	FlagOnConflict = "on-conflict"

	// FlagReparentChildren opts in (non-interactively) to reparenting the orphaned
	// children of a cleaned worktree onto its grandparent. In interactive mode the
	// command proposes this with a recap and an explicit confirmation instead.
	FlagReparentChildren = "reparent-children"

	// On-conflict modes for `extract`.
	OnConflictAbort   = "abort"
	OnConflictResolve = "resolve"

	// init flags (non-interactive bootstrap).
	FlagIfNotExists    = "if-not-exists"
	FlagNonInteractive = "non-interactive"
	FlagShell          = "shell"
	FlagBasePath       = "base-path"
	FlagBaseBranch     = "base-branch"
	FlagEnvStrategy    = "env-strategy"
	FlagInstallCommand = "install-command"

	// init skip flags — opt out of optional config sections (non-interactive).
	FlagSkipEnv      = "skip-env"
	FlagSkipHooks    = "skip-hooks"
	FlagSkipServices = "skip-services"

	// init wizard section gate choices — whether to configure or skip a section.
	WizardChoiceConfigure = "configure"
	WizardChoiceSkip      = "skip"

	// SkipMarkerComment is the leading comment written into a config.toml section
	// the user skipped during init. The config template emits it (followed by
	// section-specific guidance) so a skipped section stays valid but inert.
	SkipMarkerComment = "# Skipped during init."

	// FlagOnly re-runs init for specific sections only (re-init / re-detect).
	FlagOnly = "only"

	// Init section identifiers — used by `wtm init --only <section>`.
	SectionEnv       = "env"
	SectionHooks     = "hooks"
	SectionServices  = "services"
	SectionWorktrees = "worktrees"

	// sync flags — cascade rebase of worktrees onto their recorded parent.
	FlagDryRun       = "dry-run"
	FlagPush         = "push"
	FlagNoPush       = "no-push"
	FlagKeepConflict = "keep-conflict"

	// prune flags — batch removal of finished worktrees.
	FlagMerged  = "merged"
	FlagClosed  = "closed"
	FlagGone    = "gone"
	FlagNoFetch = "no-fetch"

	// FlagValidate makes `config show` validate the config instead of printing it.
	FlagValidate = "validate"

	// Prune candidate reasons — the category that made a worktree prunable,
	// emitted in the prune result (JSON + text recap). All are GitHub/remote
	// truth: a merged PR (--merged), a PR closed without merging (--closed), or a
	// deleted upstream branch (--gone).
	PruneReasonPRMerged = "pr_merged"
	PruneReasonPRClosed = "pr_closed"
	PruneReasonGone     = "gone"

	// Prune skip reasons — why a matching worktree was not removed. The current
	// worktree is not among them: prune removes it (like clean) and redirects the
	// shell to the base repo afterwards. Dirty/Unpushed/OpenPR mirror clean's
	// unsafe-to-remove checks: they skip unless --force is passed.
	PruneSkipBase     = "base_branch"
	PruneSkipMain     = "main_worktree"
	PruneSkipDirty    = "dirty"
	PruneSkipUnpushed = "unpushed"
	PruneSkipOpenPR   = "open_pr"

	// Script classification keywords for package.json → run.toml mapping.
	// A script is classified as a long-running service when its name matches
	// one of these keywords exactly, as a prefix ("<kw>:"), or as a suffix (":<kw>").
	ScriptKeyDev   = "dev"
	ScriptKeyStart = "start"
	ScriptKeyServe = "serve"
	ScriptKeyWatch = "watch"

	// FlagWithPRs includes GitHub PR info in non-interactive worktree listings.
	// PRs are fetched lazily (streamed) in interactive mode, but a pipe/JSON
	// consumer can't stream, so the fetch is opt-in and blocking there.
	FlagWithPRs = "with-prs"

	// GHPRFields is the JSON field set passed to `gh pr list/view --json`. It
	// holds exactly what wtm consumes: PR identity, head/base branches, url, and
	// the fork flag (isCrossRepository).
	GHPRFields = "number,title,author,headRefName,baseRefName,url,isCrossRepository,isDraft"

	// GHPRFieldsWithState is the field set for the all-states PR listing used by
	// `wtm tree --with-prs`, which must surface merged/closed PRs (clean
	// candidates) — so it includes the PR state.
	GHPRFieldsWithState = "number,headRefName,url,state"

	// PR states, normalised to lowercase. PRInfo.State always holds one of these,
	// and output routes rendering on them. Centralised so a typo can't silently
	// degrade merged/closed display.
	PRStateOpen   = "open"
	PRStateMerged = "merged"
	PRStateClosed = "closed"

	// Checkout wizard badge texts: a PR whose branch already has a local
	// worktree ("linked") or that comes from a fork ("fork") is disabled.
	BadgeTextLinked = "linked"
	BadgeTextFork   = "fork"

	// BadgeTextRemote tags a remote-tracking branch (origin/*) offered as a
	// worktree start-point or parent in a branch picker.
	BadgeTextRemote = "remote"

	// Divergence badge glyphs: a local branch ahead of / behind its origin
	// counterpart is labelled with these in a branch picker (e.g. "↑2 ↓5").
	BadgeGlyphAhead  = "↑"
	BadgeGlyphBehind = "↓"

	// Status pill glyphs: the leading symbol of a worktree row's right-aligned
	// dirty/clean status pill.
	BadgeGlyphDirty = "⚠"
	BadgeGlyphClean = "✓"

	// KeyRefresh is the picker key that re-fetches origin and recomputes the
	// branch divergence badges.
	KeyRefresh = "r"

	// RemoteBranchPrefix is the short-name prefix of origin remote-tracking refs
	// ("origin/feature"). Used to strip/build remote refs and to detect whether a
	// picked start-point is remote.
	RemoteBranchPrefix = "origin/"

	// LoadingBranchesText labels the spinner shown while a branch picker fetches
	// origin to refresh its divergence badges.
	LoadingBranchesText = "Fetching branches…"

	// SummaryConfigDefault is the env-step summary shown when no explicit env
	// strategy is chosen and the project config default applies.
	SummaryConfigDefault = "config default"

	// Output format values for FlagOutput.
	OutputText = "text"
	OutputJSON = "json"
	// OutputMermaid renders a Mermaid flowchart (wtm tree only) — a diagram that
	// can be pasted into a PR or Notion as a shareable discussion artifact.
	OutputMermaid = "mermaid"

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

	// MetaFileName is the metadata file created per worktree inside
	// <state-dir>/worktrees/<branch>/.
	MetaFileName = "meta.json"

	// Cobra group IDs — one per section of the root --help output.
	CmdGroupWorktrees = "worktrees"
	CmdGroupNavigate  = "navigate"
	CmdGroupStack     = "stack"
	CmdGroupJobs      = "jobs"
	CmdGroupGitHub    = "github"
	CmdGroupSetup     = "setup"

	// Cobra group titles — the section headers rendered in the root --help output,
	// registered alongside their IDs so a rename touches one place.
	CmdGroupWorktreesTitle = "Worktrees:"
	CmdGroupNavigateTitle  = "Navigate:"
	CmdGroupStackTitle     = "Stacked branches:"
	CmdGroupJobsTitle      = "Dev jobs (experimental):"
	CmdGroupGitHubTitle    = "GitHub:"
	CmdGroupSetupTitle     = "Setup:"

	// CLI command names — used in Use: declarations and exec.Command(bin, …) call sites.
	// Centralised here so a rename is a single-file change with no silent breakage.
	CmdRun      = "run"
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
	CmdExtract  = "extract"
	CmdSync     = "sync"
	CmdRelocate = "relocate"
	CmdReparent = "reparent"
	CmdTree     = "tree"
	CmdPrune    = "prune"

	// MinWizardListHeight is the minimum number of rows reserved for a wizard
	// step's scrollable list. Completed-step summaries are bounded so they never
	// shrink the list below this, keeping the breadcrumb (which names the worktree
	// being acted on) on screen even after many steps. See LUC-85.
	MinWizardListHeight = 3

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
