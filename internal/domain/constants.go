// Package domain defines shared types, constants, and errors for the wtm CLI.
package domain

import "time"

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
	ExitCodeWorktreeExists    = 10 // a worktree or its path already exists
	ExitCodeBranchNotFound    = 11 // the requested branch does not exist locally
	ExitCodeConfigNotFound    = 12 // the repo has no wtm config (run `wtm init`)
	ExitCodeServiceNotFound   = 14 // the referenced job is not declared in run.toml
	ExitCodeExtractConflict   = 15 // selected changes do not apply cleanly onto the target worktree
	ExitCodeRunNotInitialized = 16 // the run module is not initialized (run `wtm run init`)

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

	// EnvFileName is the canonical env value file — the copy target of any template.
	EnvFileName = ".env"

	// EnvLocalFileName is the machine-local override. Detected as a value file
	// (worktrees share a machine, so a local value is legitimately shareable) and
	// flagged local for informational display — never excluded from detection.
	EnvLocalFileName = ".env.local"

	// Committed-schema template suffixes recognized on a `.env` file. `.example`
	// is the dominant ecosystem convention; the rest are real but minority.
	EnvTemplateSuffixExample  = ".example"
	EnvTemplateSuffixDist     = ".dist"
	EnvTemplateSuffixSample   = ".sample"
	EnvTemplateSuffixTemplate = ".template"
	EnvTemplateSuffixTmpl     = ".tmpl"

	// `wtm env` value-source display labels — the single source a worktree's env
	// values come from, named by its strategy and shown in the drift report header.
	// `example` reads placeholders from the template only; `main` reads the main
	// worktree; `parent` reads the parent worktree only (main solely when the parent
	// has no local worktree at all).
	EnvSourceLabelTemplate = "template"
	EnvSourceLabelMain     = "main"
	EnvSourceLabelParent   = "parent worktree"
	// EnvSourceLabelNone is shown when the strategy's value source has no .env file at
	// all (and no fallback either) — nothing to sync from, so keys come from the
	// template as placeholders. Typical on a fresh project before any .env is created.
	EnvSourceLabelNone = "template (no .env to sync from)"

	// Tokens recognized by the .env content parser (internal/rules/env_parse.go).
	EnvCommentPrefix = "#"
	EnvExportPrefix  = "export "
	EnvAssign        = "="
	EnvQuoteDouble   = '"'
	EnvQuoteSingle   = '\''

	// DefaultShell is the default shell for integration.
	DefaultShell = ShellZsh

	// ShellInitCommand is the single source of truth for the shell-integration
	// eval line; both the multi-line hint and the init recap bullet compose it.
	ShellInitCommand = "eval \"$(wtm shell-init)\""

	// MsgShellInitHint tells the user how to set up shell integration.
	MsgShellInitHint = "Add this to your shell config:\n\n  " + ShellInitCommand

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

	// Workspace declaration files — their presence means one root install already
	// covers every package in the repo, so no per-package install is seeded.
	WorkspaceFilePnpm  = "pnpm-workspace.yaml"
	WorkspaceFileGo    = "go.work"
	WorkspaceFileTurbo = "turbo.json"
	WorkspaceFileNx    = "nx.json"
	WorkspaceFileLerna = "lerna.json"

	// PackageJSONFile is the npm manifest, used for script discovery and as the
	// marker that a globbed workspace pattern matched a real package.
	PackageJSONFile = "package.json"

	// MonorepoScanMaxDepth caps how deep sub-project discovery and globstar
	// workspace patterns descend below the project root (1 = direct children).
	MonorepoScanMaxDepth = 3

	// Directory names excluded from every project scan.
	ScanSkipNodeModules = "node_modules"
	ScanSkipTrees       = ".trees"
	ScanSkipGit         = ".git"
	ScanSkipVendor      = "vendor"
	ScanSkipDist        = "dist"

	// EnvGoFile is the environment variable used by the shell wrapper to pass the go-file path.
	EnvGoFile = "WTM_GO_FILE"

	// Flag names.
	FlagFrom       = "from"
	FlagFF         = "ff"
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

	// `wtm env` flags. FlagFrom (source override), FlagOnConflict (keep/overwrite),
	// FlagYes and FlagOutput are shared with other commands. FlagMode selects
	// add/refresh, FlagCheck is the read-only drift report, FlagPrune drops orphans.
	FlagMode  = "mode"
	FlagCheck = "check"
	FlagPrune = "prune"

	// FlagReparentChildren opts in (non-interactively) to reparenting the orphaned
	// children of a cleaned worktree onto its grandparent. In interactive mode the
	// command proposes this with a recap and an explicit confirmation instead.
	FlagReparentChildren = "reparent-children"

	// On-conflict modes for `extract`.
	OnConflictAbort   = "abort"
	OnConflictResolve = "resolve"

	// Status codes of the XY field of `git status --porcelain`.
	PorcelainUntracked  = "??"
	PorcelainIgnored    = "!!"
	PorcelainUnmodified = ' '
	PorcelainRename     = "R"
	PorcelainCopy       = "C"
	PorcelainDeleted    = "D"

	// PorcelainFieldSep separates the records of `git status --porcelain -z`.
	PorcelainFieldSep = "\x00"

	// PorcelainPathOffset is where the path starts in a record: the two-character
	// XY status field plus its trailing space.
	PorcelainPathOffset = 3

	// init flags (non-interactive bootstrap).
	FlagIfNotExists    = "if-not-exists"
	FlagNonInteractive = "non-interactive"
	FlagShell          = "shell"
	FlagBasePath       = "base-path"
	FlagBaseBranch     = "base-branch"
	FlagEnvStrategy    = "env-strategy"
	FlagInstallCommand = "install-command"
	FlagCleanCommand   = "clean-command"

	// init skip flags — opt out of optional config sections (non-interactive).
	FlagSkipEnv   = "skip-env"
	FlagSkipHooks = "skip-hooks"
	FlagSkipClean = "skip-clean"

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
	FlagFFParents    = "ff-parents"
	FlagNoFFParents  = "no-ff-parents"

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
	// holds exactly what wtm consumes: PR identity, head/base branches, url,
	// and the fork flag (isCrossRepository).
	GHPRFields = "number,title,author,headRefName,baseRefName,url,isCrossRepository,isDraft"

	// GHPRFieldsWithChecks adds the review decision and the CI status-check
	// rollup. Only the dashboard's REVIEW section renders them, and
	// statusCheckRollup is resolved per pull request, so every other surface
	// stays on the narrow set rather than paying for a field it never shows.
	GHPRFieldsWithChecks = GHPRFields + ",reviewDecision,statusCheckRollup"

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

	// GHCheckConclusion* are the `conclusion` values a statusCheckRollup entry
	// carries once it has finished running (a modern Checks API CheckRun). A
	// NEUTRAL or SKIPPED check counts as passed — the least-wrong of three
	// buckets, disclosed here rather than accidental: it can conflate "ran and
	// mattered" with "never ran at all". CANCELLED and TIMED_OUT block the PR
	// the same way FAILURE does. ACTION_REQUIRED is not a failure: the workflow
	// needs authorization to run (typically a first run from a fork awaiting
	// approval) — it has not run and broken, it has not started, so it reads
	// as pending, not ✗.
	GHCheckConclusionSuccess        = "SUCCESS"
	GHCheckConclusionFailure        = "FAILURE"
	GHCheckConclusionNeutral        = "NEUTRAL"
	GHCheckConclusionSkipped        = "SKIPPED"
	GHCheckConclusionCancelled      = "CANCELLED"
	GHCheckConclusionTimedOut       = "TIMED_OUT"
	GHCheckConclusionActionRequired = "ACTION_REQUIRED"

	// GHCheckState* are the `state` values a legacy StatusContext rollup entry
	// carries — the Status API, used by integrations that predate the Checks
	// API (CircleCI, Travis, and similar). A rollup mixes both shapes; an
	// entry with no `conclusion` is not necessarily still running, it may be
	// reporting through `state` instead.
	GHCheckStateSuccess = "SUCCESS"
	GHCheckStateError   = "ERROR"
	GHCheckStateFailure = "FAILURE"
	GHCheckStatePending = "PENDING"

	// GHReviewDecision* are the raw `reviewDecision` values `gh` returns.
	GHReviewDecisionApproved         = "APPROVED"
	GHReviewDecisionChangesRequested = "CHANGES_REQUESTED"
	GHReviewDecisionReviewRequired   = "REVIEW_REQUIRED"

	// Checkout wizard badge texts: a PR whose branch already has a local
	// worktree ("linked") or that comes from a fork ("fork") is disabled.
	BadgeTextLinked = "linked"
	BadgeTextFork   = "fork"

	// BadgeTextRemote tags a remote-tracking branch (origin/*) offered as a
	// worktree start-point or parent in a branch picker.
	BadgeTextRemote = "remote"

	// BadgeTextBase and BadgeTextOrigin prefix a worktree's commit badges to name
	// the referential the arrows count against: "base ↑N" = commits ahead of the
	// parent/base branch; "origin ↑a ↓b" = divergence from origin/<branch>. Same
	// glyphs, two referentials — the labels disambiguate them.
	BadgeTextBase   = "base"
	BadgeTextOrigin = "origin"

	// Divergence badge glyphs: a local branch ahead of / behind its origin
	// counterpart is labelled with these in a branch picker (e.g. "↑2 ↓5").
	BadgeGlyphAhead  = "↑"
	BadgeGlyphBehind = "↓"

	// Divergence state labels: the string form of a DivergenceState emitted in
	// JSON output (worktree origin divergence) so agents can branch on it.
	DivergenceLabelUpToDate = "up-to-date"
	DivergenceLabelBehind   = "behind"
	DivergenceLabelAhead    = "ahead"
	DivergenceLabelDiverged = "diverged"

	// Status pill glyphs: the leading symbol of a worktree row's right-aligned
	// dirty/clean status pill.
	BadgeGlyphDirty = "⚠"
	BadgeGlyphClean = "✓"

	// WorktreeActiveTag marks the worktree the shell is currently inside, in
	// both `wtm list`'s text output and its interactive picker — one wording,
	// reused rather than restated.
	WorktreeActiveTag = "● active"

	// SummaryNone stands in for a set answer the user left empty, in a wizard
	// breadcrumb that must still show the step was reached.
	SummaryNone = "none"

	// Tree badge texts: what a node's status annotations read as, shared by the
	// ASCII tree, the Mermaid export and the dashboard's Tree tab.
	TreeBadgeVirtualText   = "(no worktree)"
	TreeBadgeRebasingText  = "⚠ rebasing"
	TreeBadgeDirtyText     = "⚠ dirty"
	TreeBadgeNeedsSyncText = "⚠ needs sync"
	TreeBadgeCycleText     = "⚠ cycle"

	// Tree connector glyphs: the gutter a flattened forest row carries. They are
	// part of the shape rules.FlattenForest decides, not of one renderer's style,
	// which is why both the CLI tree and the dashboard draw from the same values.
	TreeConnectorBranch = "├─ "
	TreeConnectorLast   = "└─ "
	TreeGutterPipe      = "│  "
	TreeGutterBlank     = "   "

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

	// LoadingWorktreesText labels the spinner shown while a worktree list fetches
	// origin to refresh its divergence badges.
	LoadingWorktreesText = "Fetching worktrees…"

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

	// ExperimentalRunNotice is the single source of truth for the "run is
	// experimental" wording. Reused by the run-init output, the not-initialized
	// guard, and the mention printed at the end of `wtm init`, so the caveat
	// stays consistent everywhere the run module surfaces.
	ExperimentalRunNotice = "`wtm run` is experimental — the workflow is still stabilizing and commands may change."

	// MsgRunInitHint points users at the dedicated command that configures the
	// run module, printed at the end of `wtm init` (which no longer configures
	// services itself).
	MsgRunInitHint = "Run services per worktree ? Configure them with `wtm run init` (experimental)."

	// MsgRelocateHint points users at `wtm relocate` to adopt/align worktrees that
	// existed before wtm. Printed unconditionally at the end of `wtm init` — we do
	// not probe for pre-existing worktrees, the hint is cheap and always relevant.
	MsgRelocateHint = "Worktrees created before wtm ? Adopt and align them with `wtm relocate`."

	// Init recap (LUC-125): labels and copy for the framed end-of-init recap
	// (accent-bar box + pill title) that summarizes the written config and lists
	// the next steps. RecapWidth is the fixed render width shared with `relocate`.
	RecapWidth = 80

	InitRecapTitleGlobal  = "Global config ready"
	InitRecapTitleProject = "Project ready"
	InitRecapNextSteps    = "Next steps"

	InitRecapLabelShell       = "shell"
	InitRecapLabelBasePath    = "base_path"
	InitRecapLabelBaseBranch  = "base_branch"
	InitRecapLabelEnvStrategy = "env_strategy"
	InitRecapLabelOnCreate    = "on_create"
	InitRecapLabelOnClean     = "on_clean"

	InitRecapValueSkipped         = "skipped"
	InitRecapValueSkippedTemplate = "skipped (template)"
	InitRecapHookMoreFmt          = "%s  (+%d more)"

	// Hook phase titles: shown as a bold section header above the streamed hook
	// output, so create/clean read as distinct phases instead of loose lines.
	HooksTitleOnCreate = "Running on_create hooks"
	HooksTitleOnClean  = "Running on_clean hooks"

	// create result recap labels (aligned "label   value" rows). "from" names the
	// start-point of a newly created branch; "parent" replaces it when an existing
	// local branch is reused, where the source is only the recorded sync parent.
	CreateRecapLabelFrom   = "from"
	CreateRecapLabelParent = "parent"
	CreateRecapLabelEnv    = "env"
	CreateRecapLabelPath   = "path"

	// GoCommandFmt builds the jump-in command shown by every worktree-creating
	// command (create, extract, checkout): `wtm go <branch>`.
	GoCommandFmt = "wtm go %s"

	// Next-step bullets printed in the recap. Each is a single line so the
	// "Next steps" block stays flat (no cascading indentation).
	InitNextStepShell    = "Add to your shell config:  " + ShellInitCommand
	InitNextStepCreate   = "wtm create <branch>  — create a worktree to get started"
	InitNextStepRelocate = "wtm relocate  — adopt & align pre-existing worktrees"
	InitNextStepRunInit  = "wtm run init  — (experimental) configure per-worktree services"

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
	CmdInit     = "init"
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
	CmdEnv      = "env"
	// CmdFastForwardAlias keeps the frequent gesture short to type.
	CmdFastForward      = "fast-forward"
	CmdFastForwardAlias = "ff"

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

	// SyncConfirmPrompt is the confirmation question shown before running a sync
	// cascade, formatted with the number of worktrees to rebase. Shared by the
	// interactive picker's confirmation step and the non-picker confirm prompt.
	SyncConfirmPrompt = "Rebase %d worktree(s) onto their parents?"

	// SyncConfirmBaseFmt replaces it when the run rebases nothing: counting the
	// worktrees of a base-only refresh announces a cascade that will not happen.
	SyncConfirmBaseFmt = "Fetch %s and fast-forward it to its remote?"

	// SyncPlanHeader titles the cascade preview. It carries no branch: the plan's
	// own lines name every branch involved, and a header that gains and loses a
	// suffix reads as two different sections.
	SyncPlanHeader = "Sync plan"

	// SyncKeepConflictWarning is the consequence line on the sync confirmation when
	// conflicts are kept. It breaks itself in two: the surfaces truncate a row that
	// does not fit rather than wrapping it, so a single long line loses its tail to
	// an ellipsis — and the tail is the part naming the way out.
	SyncKeepConflictWarning = "⚠ Conflicting rebases are left in their worktree.\n" +
		"  Resolve each with git rebase --continue."

	// SyncPushPrompt is the push confirmation question shown after a successful
	// cascade, formatted with the number of pushable branches.
	SyncPushPrompt = "Push %d rebased branch(es) to origin?"

	// SyncPushWarning clarifies what pushing does and that declining skips it:
	// No or Esc leaves the rebased branches local.
	SyncPushWarning = "Force-pushes the rebased branches with --force-with-lease. " +
		"No or Esc skips the push — branches stay local."

	// SyncPushOption and SyncKeepLocalOption name the two outcomes of the push
	// question rather than answering it yes or no: force-pushing and keeping the
	// branches local read nothing alike.
	SyncPushOption      = "Push to origin"
	SyncKeepLocalOption = "Keep local"

	// SyncRebasing and SyncPushing head the two phases of a cascade.
	SyncRebasing = "Rebasing worktrees…"
	SyncPushing  = "Pushing to origin…"

	// SyncPlanComputing is the loading message shown while the sync plan preview is
	// computed asynchronously on entering the confirmation step.
	SyncPlanComputing = "Computing sync plan…"

	// SyncParentScanning is the loading message shown while the parents outside the
	// cascade are checked against their remote.
	SyncParentScanning = "Checking parent branches…"

	// SyncParentDescription explains why the parent question is asked at all, above
	// the two choices: these branches have no step of their own, so nothing in the
	// cascade refreshes them.
	SyncParentDescription = "These parents have no step of their own, so nothing else refreshes them.\n" +
		"Choose whether to bring them up to date first."

	SyncWizardErrLabel = "sync wizard"
	// SyncSelectionTitle heads the worktree multi-select; SyncConflictTitle and
	// SyncParentsTitle head the two decisions that follow.
	SyncSelectionTitle   = "Select worktrees to sync"
	SyncConflictTitle    = "On conflict"
	SyncParentsTitle     = "Parent branches"
	SyncConfirmTitle     = "Confirm"
	SyncSelectAtLeastOne = "select at least one worktree"
	// SyncConflictNormal and SyncConflictKeep stay short enough to sit on one line
	// in a narrow terminal: an option that wraps loses its shape, and what each one
	// entails is spelled out by SyncConflictIntro right above them.
	SyncConflictNormal = "Abort the rebase"
	SyncConflictKeep   = "Keep the conflict"
	SyncConflictIntro  = "Choose what happens when a rebase hits a conflict.\n" +
		"Aborting rewinds it and leaves the worktree clean.\n" +
		"Keeping it leaves the rebase there for you to resolve."
	// SyncCounterFmt names no branch: each worktree is rebased onto its own
	// recorded parent, which the base only coincides with at the first level.
	SyncCounterFmt            = "About to sync %d worktree(s) onto their parent."
	SyncConflictNormalSummary = "sync normally"
	SyncConflictKeepSummary   = "keep conflicts in progress"
	SyncParentFFOption        = "Fast-forward them first"
	SyncParentKeepOption      = "Leave them as they are"
	SyncParentFFSummary       = "fast-forward"
	SyncParentKeepSummary     = "leave as they are"
	SyncParentLineFmt         = "%s is %s behind %s%s — %s rebase onto it."
	SyncConfirmOption         = "Yes, sync"
	SyncNothingToSync         = "No worktrees to sync."
	// SyncNoRebaseStep and SyncNoStaleParent are why a decision was never put to
	// the user: nothing is rebased, or no parent is behind its remote.
	SyncNoRebaseStep  = "nothing to rebase"
	SyncNoStaleParent = "no parent behind its remote"
	// SyncDryRunNoQuestion is why a preview asks neither of the two decisions that
	// only matter once a rebase actually runs.
	SyncDryRunNoQuestion = "dry run — nothing is rebased"
	// SyncSelectionRequiredFmt refuses a run that can neither pick nor be told what
	// to sync. Verbs: --all, --yes, --output, json.
	SyncSelectionRequiredFmt = "specify one or more worktrees, or pass --%s (no interactive picker under --%s, without a terminal, or in --%s %s mode)"
	SyncNeedsTerminal        = "sync needs a terminal to confirm the cascade; pass --yes to run unattended or --dry-run to preview"
	// SyncKeepConflictHintFmt tells the user where a kept conflict was left.
	// Verbs: branch, worktree path.
	SyncKeepConflictHintFmt = "%s left mid-rebase in %s — run `git rebase --continue` or `git rebase --abort` there"
	// SyncTag* name what the cascade would skip.
	SyncTagDirty    = "dirty"
	SyncTagRebasing = "rebasing"
	// SyncLabel* say in a few words what became of one branch, for a surface with
	// one line per step rather than a paragraph (rules.SyncStatusLabel).
	SyncLabelSynced           = "rebased onto its parent"
	SyncLabelUpToDate         = "already up to date"
	SyncLabelSkippedDirty     = "skipped — uncommitted changes"
	SyncLabelSkippedAncestor  = "skipped — an ancestor was not synced"
	SyncLabelDiverged         = "skipped — diverged from origin"
	SyncLabelRebaseInProgress = "skipped — a rebase is already in progress"
	SyncLabelConflict         = "conflict"
	SyncLabelUnknownParent    = "skipped — no recorded parent"
	SyncLabelError            = "failed"
	// SyncLabelConflictKept, SyncLabelConflictAborted and SyncLabelErrorFmt say what
	// the bare status cannot: which of the two conflict modes ran — and so whether
	// there is anything left to clean up — and why a step failed (cause).
	SyncLabelConflictKept    = "conflict — rebase left in progress"
	SyncLabelConflictAborted = "conflict — rebase aborted, worktree left clean"
	SyncLabelErrorFmt        = "failed — %s"
	// SyncBaseLabel* and SyncParentLabel* do the same for the two branches a
	// cascade moves without rebasing them (rules.SyncBaseLabel,
	// rules.SyncParentStatusLabel).
	SyncBaseLabelFastForwarded   = "fast-forwarded from origin"
	SyncBaseLabelUpToDate        = "already up to date"
	SyncParentLabelFastForwarded = "fast-forwarded from origin"
	SyncParentLabelBehind        = "left behind origin"
	SyncParentLabelDiverged      = "diverged from origin — left untouched"
	SyncParentLabelFFFailed      = "could not be fast-forwarded"
	// SyncPlanFailedFmt heads the recap of a cascade that could not be planned
	// (e.g. a cycle in the parent chain).
	SyncPlanFailedFmt = "Failed to build sync plan: %w"

	// FastForward* are the fast-forward flow's labels, shared by both surfaces.
	FastForwardWizardErrLabel   = "fast-forward wizard"
	FastForwardSelectionTitle   = "Select worktrees to fast-forward"
	FastForwardConfirmTitle     = "Fast-forward from origin"
	FastForwardCheckLoading     = "Checking branches against origin"
	FastForwardStage            = "Fast-forwarding"
	FastForwardOption           = "Yes, fast-forward"
	FastForwardAnywayOption     = "Fast-forward anyway"
	FastForwardSelectAtLeastOne = "select at least one worktree"
	FastForwardNothingToDo      = "No worktrees to fast-forward."
	FastForwardWarnDirty        = "uncommitted changes."
	FastForwardBlockerDirty     = "dirty"
	// FastForwardSelectionRequiredFmt names the flag a run with no picker is
	// missing, rather than falling back to one. Verbs: --all, --yes, --output, json.
	FastForwardSelectionRequiredFmt = "specify one or more worktrees, or pass --%s (no interactive picker under --%s, without a terminal, or in --%s %s mode)"
	// FastForwardForceHintFmt refuses a dirty worktree by naming the flag that
	// lifts it: --yes is the confirmation axis and never lifts a refusal.
	// Verbs: branch, reason, --force.
	FastForwardForceHintFmt = "%s has %s Use --%s to fast-forward anyway"
	// FastForwardLabel* say in a few words what became of one branch
	// (rules.FastForwardStatusLabel).
	FastForwardLabelUpToDate = "already up to date"
	FastForwardLabelAdvanced = "fast-forwarded from origin"
	FastForwardLabelDiverged = "diverged"
	FastForwardLabelNoRemote = "no origin counterpart"
	FastForwardLabelFailed   = "failed"
	// FastForwardDivergedHintFmt names the gesture that does handle a divergence:
	// this one never rewrites a branch carrying local commits. Verbs: branch,
	// ahead, behind, branch.
	FastForwardDivergedHintFmt = "%s has diverged from origin (%d ahead, %d behind) — run `wtm sync %s`"
	FastForwardNoRemoteFmt     = "%s has no origin counterpart"
	// FastForwardPlanFmt, FastForwardUpToDateFmt, FastForwardResultFmt and
	// FastForwardFailedFmt are the per-branch lines: what will happen, and what did.
	// The arrow points at the branch that moves, the way the sync plan renders
	// "branch ← parent": here the branch receives what origin already carries.
	FastForwardPlanFmt     = "%s ← origin/%s (%s)"
	FastForwardUpToDateFmt = "%s is already up to date"
	FastForwardResultFmt   = "%s: %s"
	FastForwardFailedFmt   = "%s: failed — %s"
	FastForwardHeader      = "Fast-forward"

	// Source-reconciliation and env-fallback prompts shared by the create and
	// extract flows — used both by the in-wizard confirmation steps and the
	// standalone confirms on the non-interactive --from path. Format verbs:
	// %s source branch, %d commit counts.

	// SourceFastForwardPrompt offers to fast-forward a behind-only source branch
	// to origin before creating the worktree (source, behind).
	SourceFastForwardPrompt = "%s is %d commit(s) behind origin — fast-forward it before creating?"
	// SourceFastForwardDescription explains what the fast-forward does. Declining
	// keeps the source as-is rather than aborting.
	SourceFastForwardDescription = "Updates your local branch to origin so the new worktree starts up to date. " +
		"Skipped if its worktree has uncommitted changes."
	// SourceDivergedPrompt warns that a diverged source can't be fast-forwarded and
	// asks whether to create from it anyway (source, ahead, behind).
	SourceDivergedPrompt = "%s has diverged from origin (%d ahead, %d behind) — create the worktree from it anyway?"
	// SourceDivergedWarning explains the consequence of a diverged source.
	SourceDivergedWarning = "It can't be fast-forwarded. The worktree starts from your local branch, missing commits " +
		"that are on origin — you may have to rebase or resolve conflicts later."
	// SourceProceedStalePrompt asks whether to create from a stale local source
	// after a fast-forward failed (source, behind).
	SourceProceedStalePrompt = "Create the worktree from local %s anyway? (behind origin by %d)"
	// SourceProceedStaleWarning reports why the fast-forward failed (cause).
	SourceProceedStaleWarning = "Couldn't fast-forward: %v"
	// SourceUpdateSkip* explain why a run offers no source reconciliation.
	SourceUpdateSkipNoSource = "no source to check"
	SourceUpdateSkipRemote   = "source is a remote branch"
	SourceUpdateSkipUpToDate = "source already up to date"
	SourceUpdateSkipDiverged = "source diverged from origin — see recap"
	// SourceFastForwardOptionFmt labels the fast-forward choice on the
	// source-update step (subject).
	SourceFastForwardOptionFmt = "Fast-forward %s to origin"
	// SourceFastForwardLoadingFmt is the spinner message while a fast-forward
	// runs, in the wizard's confirmation step or checkout's reuse reconciliation
	// (subject).
	SourceFastForwardLoadingFmt = "Updating %s from origin…"
	// RecapUpdateFastForward is the recap line naming an accepted fast-forward,
	// shared by create's and extract's combined recaps (subject).
	RecapUpdateFastForward = "Update:  fast-forward %s to origin"
	// RecapParentRecordedForSync explains, on the source-update step, that a
	// reused branch's source is recorded for `wtm sync` rather than being a
	// git start-point.
	RecapParentRecordedForSync     = "Parent recorded for `wtm sync` — the branch already exists and keeps its commits"
	SourceKeepAsIsOption           = "Keep it as-is"
	SourceUpdateSummaryFastForward = "fast-forward to origin"
	SourceUpdateSummaryKeep        = "keep as-is"

	// FlowStepRequired*Fmt refuse a step that has no safe default and cannot be
	// asked (step label, flag name).
	FlowStepRequiredFmt     = "%s is required and cannot be asked in this mode"
	FlowStepRequiredFlagFmt = "%s is required and cannot be asked in this mode: pass --%s"

	// The create flow (internal/flow/create): step prose, option labels, recap
	// fields and refusals. Format verbs: %s branch, %s env strategy, %s flag name.
	CreateLoadingFmt               = "Creating worktree %s…"
	CreateBranchStepDescription    = "Name for the new worktree branch"
	CreateBranchRequired           = "branch name is required"
	CreateBranchRequiredUnattended = "branch name is required without the interactive wizard (pass it as an argument)"
	CreateSourceStepDescription    = "Branch to base the new worktree on"
	CreateEnvStepDescription       = "How to provision .env files in the new worktree"
	CreateNoSourceFmt              = "no source branch: pass --%s (no base branch configured)"
	CreateRecapConfirmOption       = "Yes, create worktree"
	EnvOptionConfigDefaultFmt      = "Use config default (%s)"
	EnvOptionExample               = "example — copy .env.example → .env"
	EnvOptionMain                  = "main — copy .env from main worktree"
	EnvOptionParent                = "parent — copy .env from source worktree"
	// EnvSummaryConfigDefault names the empty env choice rather than leaving a
	// recap line blank.
	EnvSummaryConfigDefault = "config default"

	// RecapField* are the aligned labels of the create recap body.
	RecapFieldBranch       = "Branch:  "
	RecapFieldSource       = "Source:  "
	RecapFieldParent       = "Parent:  "
	RecapFieldEnv          = "Env:     "
	RecapFastForwardSuffix = " (fast-forward to origin)"
	WarningPrefix          = "⚠ "
	WizardErrLabel         = "wizard"

	// Existing-branch reuse, shared by create, extract and checkout: a worktree
	// created on a local branch that already exists checks it out as-is instead of
	// branching off the source. Format verbs: %s branch, %s worktree path, %d
	// commit counts.

	// BranchCheckedOutElsewhereFmt reports that the branch cannot be checked out
	// because another worktree already holds it (branch, path, branch). Phrased to
	// read on from the ErrWorktreeExists sentinel it is wrapped in.
	BranchCheckedOutElsewhereFmt = "%s is checked out at %s — run `wtm go %s` to jump in"
	// BranchReusedSuffix marks the branch line of a recap when the worktree checks
	// out an existing local branch instead of creating one.
	BranchReusedSuffix = " (existing local branch — reused)"
	// BranchReusedHeadline is the create conclusion for a reused branch (branch).
	BranchReusedHeadline = "Created worktree %s on existing branch"
	// BranchReusedNote states that the worktree checked out an existing local
	// branch rather than origin's (branch).
	BranchReusedNote = "Reused your existing local branch %s."
	// BranchReusedBehindWarning warns that the reused branch is behind origin
	// (branch, behind).
	BranchReusedBehindWarning = "%s is %d commit(s) behind origin — the worktree starts from your local branch."
	// BranchReusedDivergedWarning warns that the reused branch has diverged from
	// origin (branch, ahead, behind).
	BranchReusedDivergedWarning = "%s has diverged from origin (%d ahead, %d behind) — the worktree starts from your " +
		"local branch, missing commits that are on origin."
	// ParentRequiredFmt errors when a create/extract target names a branch that
	// already exists locally but the caller gave no --from: its parent was set
	// outside wtm, so nothing says what it was branched off, and defaulting would
	// record a guess `wtm sync` then treats as fact (branch, flag name).
	ParentRequiredFmt = "%s already exists locally: pass --%s to record its parent branch " +
		"(it can't be inferred, and `wtm sync` needs it)"

	// EnvParentFallbackPrompt warns, before creating, that the "parent" env
	// strategy will source .env from main because the source has no local worktree
	// (source).
	EnvParentFallbackPrompt = "%s has no local worktree — copy .env from the main worktree instead of the parent?"
	// EnvParentFallbackWarning explains why the fallback happens.
	EnvParentFallbackWarning = "The \"parent\" env strategy needs the source branch checked out to copy its .env; " +
		"without a worktree it comes from main."

	// PruneReparentPrompt is the confirmation shown when a prune leaves child
	// worktrees that can be reparented onto their grandparent (count). Hosted as a
	// step of the prune picker so declining goes back rather than aborting.
	PruneReparentPrompt = "Reparent %d child worktree(s) onto their grandparent?"
	// PruneReparentIntro precedes the list of children a prune would otherwise
	// orphan, shown in the reparent confirmation.
	PruneReparentIntro = "These children would otherwise be left orphaned:"

	// CleanReparentPrompt is the confirmation shown when cleaning a worktree that
	// has children which can be reparented onto their grandparent (count,
	// grandparent). Hosted as a step of the clean confirm wizard.
	CleanReparentPrompt = "Reparent %d child worktree(s) onto %s?"
	// CleanReparentIntro precedes the list of children a clean would otherwise
	// orphan, shown in the reparent confirmation.
	CleanReparentIntro = "These children would otherwise be left orphaned:"

	// CleanForceHintFmt is the refusal shown when a worktree is unsafe to remove
	// without --force (branch, reason).
	CleanForceHintFmt = "worktree %s %s; pass --force to remove it anyway"
	// The clean flow (internal/flow/clean): step prose, option labels, recap body,
	// refusals and progress messages. Format verbs: %s branch, %s path, %d counts.
	CleanPickerTitle        = "Select worktree to clean"
	CleanPickerDescription  = "The parent worktree cannot be cleaned"
	CleanNothingToClean     = "no worktrees to clean (only the parent worktree exists)"
	CleanNoOrphanedChildren = "no orphaned children"
	CleanReparentOptionFmt  = "Reparent onto %s (%d)"
	CleanOrphanOption       = "Leave orphaned"
	CleanReparentSummary    = "reparent"
	CleanOrphanSummary      = "leave orphaned"
	CleanReparentChildFmt   = "  • %s will rebase onto %s instead of %s"
	CleanDeleteTitle        = "Proceed with deletion?"
	CleanDeleteOption       = "Yes, delete"
	CleanForceDeleteOption  = "Yes, force delete (bypass all checks)"
	CleanWarnDirty          = "Worktree has uncommitted changes"
	CleanWarnUnpushedFmt    = "%d commit(s) not pushed to remote"
	CleanWarnOpenPR         = "Open PR: "
	CleanWillDelete         = "Will delete:"
	CleanWillDeleteWorktree = "  worktree  "
	CleanWillDeleteBranch   = "  branch    "
	CleanRecapReparentFmt   = "Then reparent %d child worktree(s) onto %s."
	CleanRecapOrphanFmt     = "Then leave %d child worktree(s) orphaned."
	// CleanBlockerDirty, CleanBlockerUnpushed and CleanBlockerOpenPR key the
	// removal refusals a surface lists one by one (rules.CleanBlockers).
	CleanBlockerDirty    = "dirty"
	CleanBlockerUnpushed = "unpushed"
	CleanBlockerOpenPR   = "open_pr"

	CleanUnsafeDirty        = "has uncommitted changes"
	CleanUnsafeUnpushedFmt  = "has %d unpushed commit(s)"
	CleanUnsafeOpenPR       = "has an open pull request"
	CleanCheckLoading       = "Checking worktree…"
	CleanLoadingFmt         = "Removing worktree %s…"
	CleanCannotCleanParent  = "Cannot clean the parent worktree."
	CleanAlreadyAbsentFmt   = "Worktree %s already absent — nothing to clean"
	CleanedFmt              = "Cleaned worktree and branch %s"
	CleanReparentedFmt      = "Reparented %s onto %s"
	CleanStillOrphanedFmt   = "%s still points at the removed parent %s — reparent it with `wtm reparent`"
	CleanStoppedServicesFmt = "Stopped services on %s"
	CleanRemovalFailedFmt   = "Removal failed: %s"
	CleanWizardErrLabel     = "clean wizard"
	// CleanSudoConfirmFmt is the confirmation title for the privileged `sudo rm -rf`
	// removal fallback (worktree path).
	CleanSudoConfirmFmt = "Force-delete %s with `sudo rm -rf`? (you may be prompted for your password)"

	// PruneLabel* are the short phrases a reason or a skip reads as, shared by
	// every surface that shows one.
	PruneLabelPRMerged  = "PR merged"
	PruneLabelPRClosed  = "PR closed"
	PruneLabelGone      = "remote branch gone"
	PruneLabelBase      = "base branch"
	PruneLabelMain      = "main worktree"
	PruneLabelDirty     = "dirty — pass --force"
	PruneLabelUnpushed  = "unpushed commits — pass --force"
	PruneLabelOpenPR    = "open PR — pass --force"
	PruneSkippedFmt     = "Skipped %s (%s)"
	PruneWizardErrLabel = "prune wizard"
	PruneSelectionTitle = "Select worktrees to prune"
	PruneConfirmTitle   = "Confirm"
	// PruneScanning and PruneFetchAndScanning distinguish the two costs of the
	// planning phase: the second one also hits the network (gh, and the fetch
	// gone-detection runs first).
	PruneScanning          = "Scanning worktrees…"
	PruneFetchAndScanning  = "Fetching remotes and scanning worktrees…"
	PruneRemoving          = "Pruning worktrees…"
	PruneNothingToPrune    = "Nothing to prune."
	PruneConfirmOption     = "Yes, prune"
	PruneForceOption       = "Yes, force prune (bypass safety checks)"
	PruneNothingSelected   = "No worktrees selected — nothing will be pruned."
	PruneWillPruneFmt      = "Will prune %d worktree(s): %s"
	PruneReparentOptionFmt = "Reparent onto grandparent (%d)"
	PruneOrphanOption      = "Leave orphaned"
	PruneReparentChildFmt  = "  • %s will rebase onto %s instead of %s"
	PruneRecapReparentFmt  = "Then reparent %d child worktree(s) onto their grandparent."
	PruneRecapOrphanFmt    = "Then leave %d child worktree(s) orphaned."
	PruneReparentSummary   = "reparent onto grandparent"
	PruneOrphanSummary     = "leave orphaned"
	PruneNoChildren        = "no children to reparent"
	// PruneTag* label a candidate in the picker with what made it prunable, or
	// with the refusal standing in the way of removing it.
	PruneTagMerged   = "merged"
	PruneTagClosed   = "closed"
	PruneTagGone     = "gone"
	PruneTagDirty    = "dirty"
	PruneTagUnpushed = "unpushed"
	PruneTagOpenPR   = "open PR"
	// PruneJSONNeedsYes refuses a JSON run that would have to prompt.
	PruneJSONNeedsYes = "--output json requires --yes or --dry-run (the selection prompt cannot run in JSON mode)"
	// PruneNeedsTerminal refuses a run that can neither prompt nor resolve.
	PruneNeedsTerminal = "prune needs a terminal to confirm; pass --yes to run non-interactively"

	ReparentWizardErrLabel       = "reparent wizard"
	ReparentWorktreesTitle       = "Select worktrees to reparent"
	ReparentWorktreesDescription = "Choose the worktrees whose parent you want to change"
	ReparentSelectAtLeastOne     = "select at least one worktree"
	ReparentNoWorktrees          = "no worktrees to reparent"
	ReparentParentTitle          = "Select the new parent"
	// ReparentParentSingleFmt names what one worktree is rebased onto today, and
	// ReparentParentNoneFmt stands in when its recorded parent is gone (merged then
	// cleaned) — surfaced as text, never as a badge that would silently vanish.
	ReparentParentSingleFmt = "%s is currently rebased onto %s"
	ReparentParentNoneFmt   = "%s has no recorded parent"
	ReparentParentBatchFmt  = "New parent for the %d selected worktrees"
	// ReparentParentExcluded explains why a worktree in the selection, and anything
	// that would close a parent cycle, is missing from the candidate list.
	ReparentParentExcluded = "Branches that would close a parent cycle are not listed"
	ReparentRecapLabel     = "Confirm & reparent"
	ReparentRecapOption    = "Yes, reparent"
	ReparentRecapWorktrees = "Worktrees:   "
	ReparentRecapNewParent = "New parent:  "
	ReparentRecapWorktree  = "Worktree:    "
	ReparentRecapParent    = "Parent:      "
	// ReparentRecapMoveFmt reads old → new.
	ReparentRecapMoveFmt = "%s  →  %s"
	// ReparentNoParent stands where a recap would name the current parent of a
	// worktree that has none recorded.
	ReparentNoParent     = "(none)"
	ReparentStageMessage = "Recording the new parent…"
	ReparentedFmt        = "Reparented %s: %s → %s"
	// ReparentSyncHintFmt and ReparentSyncHintBare tell the user how the recorded
	// change is applied: reparent only rewrites metadata, the rebase is `wtm sync`.
	ReparentSyncHintFmt  = "Run `wtm sync %s` to rebase onto the new parent."
	ReparentSyncHintBare = "Run `wtm sync` to rebase the reparented worktrees onto their new parent."

	AbortedMessage = "Aborted."
	// WizardCancelLabel is the constant final option on every wizard recap step —
	// the single explicit cancellation point (alongside Esc on the first step).
	// Kept identical across commands so "No, cancel" always reads and sits the same.
	WizardCancelLabel = "No, cancel"
	// WizardCancelValue is the sentinel carried by the WizardCancelLabel row; the
	// command layer maps a chosen WizardCancelValue to ErrUserAborted.
	WizardCancelValue = "__wtm_wizard_cancel__"
	// WizardRecapTitle heads the final recap step, marking it visually as the
	// synthesis and action point.
	WizardRecapTitle = "Review & confirm"

	// MultiSelectHint is the shared footer hint for multi-select wizard steps, kept
	// identical across commands (sync, prune, extract) so the controls always read
	// the same.
	MultiSelectHint = "Space to toggle, a to select all, / to filter, enter to confirm, esc to cancel."

	// PinnedSuffixDefault, PinnedSuffixBase, and PinnedSuffixDetected label the
	// pinned first row of a branch picker, sharing one leading-space convention.
	PinnedSuffixDefault  = " (default)"
	PinnedSuffixBase     = " (base)"
	PinnedSuffixDetected = " (detected)"

	// OpKind* names a running flow in a surface that schedules several of them.
	OpKindCreate   = "create"
	OpKindClean    = "clean"
	OpKindReparent = "reparent"
	OpKindPrune    = "prune"
	OpKindSync     = "sync"
	// OpKindFastForward names a run that advances branches to their origin
	// counterpart and nothing else.
	OpKindFastForward = "fast-forward"

	// CmdUI is the full-screen dashboard command.
	CmdUI = "ui"

	// FetchStaleAfter est l'âge au-delà duquel les refs origin sont annoncées
	// périmées dans le header. En dessous, rien ne s'affiche : un marqueur
	// permanent ne signale plus rien.
	FetchStaleAfter = 24 * time.Hour

	AgeJustNow = "just now"
	AgeMinFmt  = "%d min ago"
	AgeHourFmt = "%d h ago"
	AgeDayFmt  = "%d d ago"
	AgeWeekFmt = "%d w ago"

	// DashboardNarrowWidth is the terminal width under which the dashboard drops
	// the side-by-side detail panel for a list-only view, detail on a key.
	DashboardNarrowWidth = 100
	// DashboardPollSeconds paces the local-git poll. `gh` is never polled: PRs load
	// once asynchronously and refresh only on KeyRefresh.
	DashboardPollSeconds = 3
	// DashboardDetailCommits is the number of commits requested for ACTIVITY.
	// DashboardDetailChanges is CHANGES' equivalent fixed cap. Both are fixed
	// maximums, not a budget split with the leftover height: a list either
	// fits its cap or folds the rest into "… N more", regardless of the other
	// list's state or how much panel height happens to be free.
	DashboardDetailCommits = 5
	DashboardDetailChanges = 5
	// DashboardDetailDebounce delays a detail load so a fast walk through the
	// list does not fire one git log per row crossed.
	DashboardDetailDebounce = 150 * time.Millisecond
	// DashboardSpinnerGrace is the delay before a loading marker appears: below
	// it, the data arrives before the marker would.
	DashboardSpinnerGrace = 200 * time.Millisecond

	DashboardRefreshing   = "refreshing"
	DashboardLoadingField = "loading…"

	// DashboardAnimationCap is the hard ceiling every dashboard animation is
	// checked against: nothing the surface draws on its own may run longer.
	DashboardAnimationCap = 400 * time.Millisecond
	// DashboardTabSlide is how long the active tab's rule takes to slide to its
	// new position. DashboardRowFlash is how long a just-created worktree's row
	// stays lit before it fades back into the ordinary selected look.
	DashboardTabSlide = 200 * time.Millisecond
	DashboardRowFlash = 400 * time.Millisecond
	// DashboardAnimFrame paces the redraw ticks a bounded animation schedules
	// while it runs — cheap enough to be invisible, coarse enough that it is
	// never mistaken for real work.
	DashboardAnimFrame = 50 * time.Millisecond

	DashboardListWidthPercent = 40
	DashboardMinListWidth     = 24
	DashboardMinDetailWidth   = 32
	DashboardOutputBodyHeight = 8
	// DashboardChromeHeight is what a panel spends on chrome: its two border rows
	// plus its title row. DashboardTitleGap is the blank line under that title —
	// a panel whose body starts against its title reads as one block.
	DashboardChromeHeight = 3
	DashboardTitleGap     = 1

	// DashboardHeaderCompactHeight is the fallback header — the context line
	// (where you are: repo, base branch, active worktree), the wordmark and
	// its tabs, then the rule that underlines the active one — shown below
	// DashboardHeaderTallThreshold rows. No rule separates the first two: the
	// tab rule underneath already does that job.
	//
	// DashboardHeaderTallHeight is the six-row signature block instead: the
	// drawn wordmark's three rows (each carrying one piece of the same
	// context the compact header packs onto one line), a blank line, the tab
	// bar, and the rule. DashboardHeaderTallThreshold is the terminal height
	// above which it shows — six rows of chrome on a 24-row terminal is a
	// quarter of the screen. The choice between the two is made once, in
	// rules.DashboardHeaderHeight; nothing else reads these two heights
	// directly.
	DashboardHeaderCompactHeight = 3
	DashboardHeaderTallHeight    = 6
	DashboardHeaderTallThreshold = 30

	// DashboardRowHeight is how many lines one worktree takes — its name, then
	// what its state amounts to — and DashboardRowGap the blank line between two.
	DashboardRowHeight = 2
	DashboardRowGap    = 1
	// DashboardTreeRowHeight is a node and the line under it: the spacer carries
	// the gutter on down, so the rows breathe without the tree coming apart.
	DashboardTreeRowHeight = 2

	// DashboardMsgBuffer sizes the channel the running flows post on. It only has
	// to absorb a burst of hook output between two frames.
	DashboardMsgBuffer = 256

	// DashboardModalChrome is what a modal box spends on its two border rows.
	DashboardModalChrome       = 2
	DashboardModalWidthPercent = 60
	DashboardModalMinWidth     = 40
	DashboardModalMaxWidth     = 88

	// DashboardWordmark names the product in the compact fallback header.
	// DashboardWordmarkGap is the fixed gap between the drawn wordmark block
	// (DashboardWordmarkLines, below) and the context text beside it in the
	// tall header, so every row's text starts on the same column regardless
	// of that row's own content.
	DashboardWordmark    = "wtm"
	DashboardWordmarkGap = "   "
	// DashboardContextSep joins the header context line's segments (repo, base,
	// active worktree). DashboardFetchedFmt and DashboardBaseFmt are its
	// individual segments; DashboardActiveGlyph marks the active worktree.
	// DashboardNeverFetched is its own wording rather than an empty age: a
	// repository that has never fetched is the most stale case there is, and
	// saying nothing about its age would read as "fetched recently".
	DashboardContextSep = " · "
	DashboardFetchedFmt = "fetched %s"
	// DashboardVersionFmt renders the running version, always shown.
	// DashboardUpgradeFmt is appended to it when a newer release is known — it
	// carries the command because a badge that only says a version exists leaves
	// the reader to guess what to do with it.
	DashboardVersionFmt      = "v%s"
	DashboardUpgradeFmt      = "→ %s · run wtm upgrade"
	DashboardNeverFetched    = "never fetched"
	DashboardActiveGlyph     = "●"
	DashboardBaseFmt         = "base %s"
	DashboardCountFmt        = "%d worktrees"
	DashboardCountOneFmt     = "%d worktree"
	DashboardTreeCountFmt    = "%d nodes"
	DashboardTreeCountOneFmt = "%d node"
	// DashboardRuleGlyph carries the header rule, DashboardActiveRuleGlyph the
	// heavier segment under the active tab.
	DashboardRuleGlyph       = "─"
	DashboardActiveRuleGlyph = "━"

	DashboardTabWorktrees = "Worktrees"
	DashboardTabTree      = "Tree"
	DashboardListTitle    = "Worktrees"
	DashboardTreeTitle    = "Worktree tree"
	DashboardDetailTitle  = "Detail"
	DashboardOutputTitle  = "Output"

	// DashboardEmptyList is shown when the list loaded but came back with
	// nothing — in a valid repository the main worktree is always present, so
	// this means the listing itself did not go as expected. Neutral wording on
	// purpose: naming an action ("press n…") here would be confident advice in
	// the one state where the surface does not know what is going on.
	DashboardEmptyList   = "No worktrees found."
	DashboardEmptyTree   = "No worktrees to lay out."
	DashboardLoadingTree = "Building the tree…"
	// DashboardTreeVirtual marks a node standing in for a parent branch that has
	// no worktree, so a row nothing can be done to says why.
	DashboardTreeVirtual = "no worktree"
	// DashboardTreeNodeGlyph marks a node that has a worktree and
	// DashboardTreeVirtualGlyph one standing in for a branch without one, so the
	// two read apart before their badges are read at all.
	DashboardTreeNodeGlyph    = "●"
	DashboardTreeVirtualGlyph = "○"
	// The Tree tab's badge formats: a PR number, a one-sided divergence count
	// ("base ↑3", "origin ↓2") and a two-sided one ("origin ↑1 ↓4").
	DashboardTreePRFmt       = "PR #%d"
	DashboardTreeAheadFmt    = "%s %s%d"
	DashboardTreeDivergedFmt = "%s %s%d %s%d"
	// DashboardEmptySelection and DashboardEmptyOutput name their next action:
	// both states are genuinely reachable and persistent, unlike
	// DashboardEmptyList above.
	DashboardEmptySelection = "Select a worktree to see what's in it."
	DashboardEmptyOutput    = "Output from create, clean and sync runs appears here."
	// DashboardCreatedFormat renders a worktree's creation date in local time.
	DashboardCreatedFormat = "2006-01-02 15:04"

	// DashboardPRFmt renders a pull request as number, title and state.
	DashboardPRFmt = "#%d %s (%s)"

	DashboardLabelPath    = "Path"
	DashboardLabelParent  = "Parent"
	DashboardLabelRebase  = "Rebase"
	DashboardLabelCreated = "Created"

	DashboardRebaseInProgress = "in progress"
	DashboardUpToDate         = "up to date"
	DashboardUnknownParent    = "unknown"

	DashboardHelpWide = "↑↓ select · n new · m actions · a bulk · tab view · o output · r refresh · ? help · q quit"
	// DashboardHelpTree drops "n new": the Tree tab lays out what exists, and a new
	// worktree is created from the list it would appear in.
	DashboardHelpTree   = "↑↓ select · m actions · a bulk · tab view · o output · r refresh · ? help · q quit"
	DashboardHelpNarrow = "↑↓ select · enter detail · n new · m actions · a bulk · o output · r refresh · ? help · q quit"
	DashboardHelpDetail = "esc back · ↑↓ select · o output · r refresh · q quit"

	// DashboardHelpTitle heads the key/mouse reference overlay. Every clickable
	// zone is listed there with its keyboard equivalent.
	DashboardHelpTitle = "Keys & mouse"

	// DashboardHelpSection* name the reference's four groups. The mouse is a
	// group of its own rather than a suffix on every key: it is the same
	// information, and folded into the prose it doubled the width of every row.
	DashboardHelpSectionNav   = "NAV"
	DashboardHelpSectionAct   = "ACT"
	DashboardHelpSectionMouse = "MOUSE"
	DashboardHelpSectionView  = "VIEW"

	DashboardHelpKeysSelect      = "↑↓  j k"
	DashboardHelpKeysEnds        = "g  G"
	DashboardHelpKeysPage        = "pgup pgdown"
	DashboardHelpKeysTab         = "tab shift+tab"
	DashboardHelpKeysOpenDetail  = "enter → l"
	DashboardHelpKeysCloseDetail = "esc ← h"
	DashboardHelpKeysOutputMove  = "shift+↑↓"
	DashboardHelpKeysClick       = "click"
	DashboardHelpKeysRightClick  = "right-click"
	DashboardHelpKeysWheel       = "wheel"

	DashboardHelpTextSelect      = "select a worktree"
	DashboardHelpTextEnds        = "first · last"
	DashboardHelpTextPage        = "page the list"
	DashboardHelpTextTab         = "switch view"
	DashboardHelpTextOpenDetail  = "open the detail"
	DashboardHelpTextCloseDetail = "close the detail"
	DashboardHelpTextNew         = "new worktree"
	DashboardHelpTextMenu        = "actions on this worktree"
	DashboardHelpTextActions     = "actions on several"
	DashboardHelpTextFastForward = "fast-forward from origin"
	DashboardHelpTextOpenPR      = "open the pull request"
	DashboardHelpTextOutput      = "fold/unfold the output"
	DashboardHelpTextOutputMove  = "scroll the output"
	DashboardHelpTextRefresh     = "refresh worktrees / PRs"
	DashboardHelpTextClick       = "select · activate"
	DashboardHelpTextRightClick  = "actions on a row"
	DashboardHelpTextWheel       = "scroll list / output"

	// DashboardHelpHint closes the overlay, DashboardHelpHintScroll replaces it
	// when the reference is taller than the screen and has to be scrolled.
	DashboardHelpHint       = "? or esc  close     q  quit"
	DashboardHelpHintScroll = "↑↓  scroll     ? or esc  close     q  quit"

	// DashboardHelpKeyGap separates the key column from the text it names, and
	// DashboardHelpColumnGap the two columns of the wide layout.
	DashboardHelpKeyGap    = 2
	DashboardHelpColumnGap = 4
	// DashboardHelpChrome is what the overlay spends around its bands: the title
	// and its rule, then the rule and the hint closing it. DashboardHelpFrame is
	// what the box itself spends: two border columns and two padding columns.
	DashboardHelpChrome = 4
	DashboardHelpFrame  = 4

	// DashboardAddLabel is the list panel's header button, KeyNew its keyboard
	// equivalent. The long form is used wherever the panel is wide enough.
	DashboardAddLabel     = "+ New"
	DashboardAddLabelLong = "+ New worktree"

	// DashboardMetaFromPrefix, DashboardMetaSeparator and DashboardMetaNothing
	// make up the second line of a worktree row.
	DashboardMetaFromPrefix = "from "
	DashboardMetaSeparator  = " · "
	DashboardMetaNothing    = "—"
	// DashboardMenuTitle heads the per-worktree context menu and
	// DashboardMenuReparent and DashboardMenuDelete are its entries. The context
	// menu acts on the worktree it was opened from, so reparent is offered here
	// for that one alone; changing several at once is `wtm reparent`.
	DashboardMenuReparent = "Change parent"
	// DashboardMenuReparentBatch is the same change over a selection the user
	// makes inside the run, which is why it lives in the global menu and not on a
	// row: a context menu hangs off one worktree.
	DashboardMenuReparentBatch = "Reparent worktrees"
	// DashboardMenuPrune removes every finished worktree at once. There is no
	// preview entry beside it: the recap lists what goes, and closing the modal
	// removes nothing.
	DashboardMenuPrune  = "Prune finished worktrees"
	DashboardMenuDelete = "Delete worktree"
	// DashboardMenuSync leads the row menu: the Tree tab is where a worktree whose
	// parent moved is flagged, so it is where the rebase has to be reachable from.
	// It arrives with the row and its descendants checked; the selection stays the
	// user's to change.
	DashboardMenuSync = "Sync this worktree"
	// DashboardMenuFastForward leads every row menu, base row included: it is the
	// least destructive action a row offers, and the most frequent.
	DashboardMenuFastForward = "Fast-forward from origin"
	// DashboardMenuFastForwardAll is the same gesture over a selection the user
	// makes inside the run, next to the batch sync and reparent.
	DashboardMenuFastForwardAll = "Fast-forward worktrees"
	// DashboardMenuSyncAll rebases every worktree at once. It arrives with the ones
	// a cascade would skip left unchecked — they stay listed, with the tag saying
	// why.
	DashboardMenuSyncAll = "Sync worktrees"
	// DashboardMenuEmpty stands in for the actions of a worktree that has none.
	DashboardMenuEmpty = "No actions available"
	// DashboardMenuChrome is what the menu box spends on its borders and padding.
	DashboardMenuChrome = 4

	DashboardCreateTitle        = "New worktree"
	DashboardDeleteTitle        = "Delete worktree"
	DashboardReparentTitle      = "Change parent"
	DashboardReparentBatchTitle = "Reparent worktrees"
	DashboardPruneTitle         = "Prune finished worktrees"
	DashboardSyncTitle          = "Sync worktrees"
	// DashboardSyncRowTitle heads the same run started from a row: a modal that
	// renamed the entry the user just picked reads as a different action.
	DashboardSyncRowTitle = "Sync this worktree"
	// DashboardFastForwardTitle heads the run started from a row, and
	// DashboardFastForwardAllTitle the one started from the global menu.
	DashboardFastForwardTitle    = "Fast-forward from origin"
	DashboardFastForwardAllTitle = "Fast-forward worktrees"
	// DashboardSync*Fmt report a finished cascade in the output panel, one line per
	// branch it touched. Verbs: branch, then what became of it.
	DashboardSyncStepFmt   = "%s — %s"
	DashboardSyncParentFmt = "%s (parent) — %s"
	DashboardSyncBaseFmt   = "%s (base) — %s"
	// DashboardSyncPushedFmt reports a branch the run published (branch).
	DashboardSyncPushedFmt = "↑ %s pushed to origin"
	// DashboardActionsLabel is the header button that opens the global menu, and
	// DashboardActionsTitle heads that menu. KeyActions is its keyboard way in.
	DashboardActionsLabel = "⋯ Actions"
	DashboardActionsShort = "⋯"
	DashboardActionsTitle = "Actions"

	// DashboardBlockersTitle heads the refusals a removal must have lifted, one by
	// one — the dashboard never offers a blanket bypass.
	DashboardBlockersTitle = "Lift every refusal to enable the deletion:"
	DashboardBlockedSuffix = " — lift the refusals above"
	DashboardConfirmLabel  = "Confirm"
	// DashboardButtonFmt brackets an action so it reads as one without needing a
	// block of color behind it.
	DashboardButtonFmt = "[ %s ]"

	DashboardGlyphChoiceOn  = "◉"
	DashboardGlyphChoiceOff = "○"
	DashboardGlyphCheckOn   = "[x]"
	DashboardGlyphCheckOff  = "[ ]"

	DashboardModalPreparing  = "Preparing…"
	DashboardStepperHint     = "↑↓ move · / filter · enter confirm · esc back"
	DashboardStepperTextHint = "enter confirm · esc back"
	// DashboardStepperMultiHint is the multi-select footer, worded like the CLI
	// wizard's so the same controls read the same on both surfaces.
	DashboardStepperMultiHint = "↑↓ move · space toggle · a all · / filter · enter confirm · esc back"
	DashboardStepperRowsHint  = "↑↓ move · enter confirm · esc back"
	DashboardFormHint         = "↑↓ move · space toggle · enter confirm · esc cancel"
	DashboardConfirmHint      = "↑↓ move · enter confirm · esc cancel"

	// DashboardUnsupportedStepFmt refuses a step kind no modal can draw, rather
	// than guessing a widget for it (step key, kind).
	DashboardUnsupportedStepFmt = "dashboard: step %q has no renderer for kind %d"

	// DashboardBusyFmt refuses an action on a worktree a background run still
	// holds (branch, running operation).
	DashboardBusyFmt = "%s is busy: a %s is still running on it"
	// DashboardBusyCaptionFmt is the same fact under a menu entry (operation).
	DashboardBusyCaptionFmt = "a %s is running on it"
	// DashboardBlockedByFmt refuses to start anything while a blocking run owns
	// the dashboard (running operation).
	DashboardBlockedByFmt = "A %s is running — wait for it to finish"
	// DashboardStartedFmt, DashboardFinishedFmt and DashboardFailedFmt bracket a
	// run in the output panel (operation, target).
	DashboardStartedFmt  = "▸ %s %s"
	DashboardFinishedFmt = "✓ %s %s"
	DashboardFailedFmt   = "✗ %s: %s"
	// DashboardPrivilegedHintFmt names the way out of a removal the dashboard
	// cannot finish: sudo prompts on the terminal the dashboard is holding.
	DashboardPrivilegedHintFmt = "  run `wtm clean %s --force` in a terminal to remove it with sudo"
	// DashboardOperationLabel names a failed run in the output panel when the
	// failure is the run itself rather than one of its phases.
	DashboardOperationLabel = "operation"
	// DashboardOpenPRLabel names a failed browser launch for the REVIEW
	// section's PR line, in the same "✗ <label>: <err>" form.
	DashboardOpenPRLabel = "open PR"

	// KeyNew opens the new-worktree wizard, the keyboard equivalent of the list
	// header's add button.
	KeyNew = "n"
	// KeyMenu opens the selected worktree's context menu. It is not a shortcut for
	// the right click but its equal: terminals that hand the right button to a
	// paste action never deliver it.
	KeyMenu = "m"
	// KeyActions opens the header's global menu, the one that acts on a selection
	// made inside the run rather than on the row under the cursor.
	KeyActions = "a"
	// KeyToggleOutput folds and unfolds the bottom output panel, the keyboard
	// equivalent of clicking its header.
	KeyToggleOutput = "o"
	// KeyHelp toggles the key reference overlay.
	// KeyOpenPR opens the selected worktree's pull request in a browser. The
	// detail panel's PR line is also clickable; the key is what makes the action
	// reachable without a mouse and over a plain ssh terminal.
	KeyOpenPR = "p"
	// KeyFastForward advances the selected worktree's branch to its origin
	// counterpart, the keyboard way into the row menu's first entry.
	KeyFastForward = "f"

	KeyHelp = "?"
	// KeyQuit leaves the dashboard. Esc does not: it only closes what is open, so
	// a persistent dashboard is never left by accident.
	KeyQuit = "q"

	// GitLogFieldSep separates fields in `git log --format`. The pipe cannot
	// serve: a commit subject may contain one. The ASCII unit separator can't
	// appear in ordinary commit text, so it can.
	GitLogFieldSep = "\x1f"
	// GitLogFormat feeds `git log --format`: short SHA, subject, author name,
	// strict ISO 8601 date, separated by GitLogFieldSep.
	GitLogFormat = "%h" + GitLogFieldSep + "%s" + GitLogFieldSep + "%an" + GitLogFieldSep + "%cI"
	// GitLogFieldCount is the number of fields GitLogFormat produces.
	GitLogFieldCount = 4
	// FetchHeadFileName is the file whose mtime marks the last successful fetch.
	FetchHeadFileName = "FETCH_HEAD"

	// DetailSection* names the detail panel's four conditional sections. A
	// section is emitted only when it has something to say, so its position
	// varies between worktrees — its rank in DetailSectionDropOrder never does.
	DetailSectionReview   = "REVIEW"
	DetailSectionChanges  = "CHANGES"
	DetailSectionActivity = "ACTIVITY"
	DetailSectionLinks    = "LINKS"

	DetailYouAreHere = "● you are here"
	DetailMoreFmt    = "…  %d more"
	// DetailBlockedFmt names a safety refusal, not an impossibility: these
	// worktrees can be deleted with --force, so the line says what unlocks
	// deletion instead of claiming it cannot happen. The parent worktree is a
	// different category — it is never deletable at all — and never renders
	// this line (rules.CleanBlockers returns nothing for it).
	DetailBlockedFmt = "⚠ deletion requires --force — %s"

	// Chip* build the vital strip. ChipBaseFmt/ChipOrigin*Fmt reuse
	// BadgeGlyphAhead/BadgeGlyphBehind so the arrow glyph is defined once.
	ChipClean             = "clean"
	ChipDirty             = "dirty"
	ChipRebasing          = "rebasing"
	ChipBaseFmt           = "base " + BadgeGlyphAhead + "%d"
	ChipActiveFmt         = "active %s"
	ChipOriginAheadFmt    = "origin " + BadgeGlyphAhead + "%d"
	ChipOriginBehindFmt   = "origin " + BadgeGlyphBehind + "%d"
	ChipOriginDivergedFmt = "origin " + BadgeGlyphAhead + "%d " + BadgeGlyphBehind + "%d"

	// DetailSectionChrome is what one section spends beyond its body lines: a
	// blank separator row before its title, the title row itself, and a blank
	// row under it. Verified against the spec §6 mockup
	// (docs/superpowers/specs/2026-08-19-wtm-ui-identity-design.md, lines
	// 130-134): REVIEW spans 5 rows for 2 body lines, so chrome is 5-2=3, not 2.
	// There is no DetailFixedRows: DetailSections reserves exactly what REVIEW
	// and LINKS actually cost, computed via sectionsHeight, instead of a
	// constant that had to secretly agree with every section builder.
	DetailSectionChrome = 3

	// DetailFieldFmt renders a LINKS field as a padded label followed by its
	// value ("Parent    main"). DetailListSep joins a list of names into one
	// field's value.
	DetailFieldFmt = "%-10s%s"
	DetailListSep  = ", "

	// DetailListIndent prefixes every body line of every detail section
	// (CHANGES, ACTIVITY, LINKS, REVIEW's PR header and checks line), so no
	// section looks misaligned against its neighbours. DetailFileFmt renders
	// one changed file as its glyph and its path, DetailUntrackedGlyph stands
	// in for the raw "??" porcelain code. DetailCommitFmt renders one commit
	// as its short SHA and subject.
	DetailListIndent     = "  "
	DetailFileFmt        = "%s  %s"
	DetailUntrackedGlyph = "?"
	DetailCommitFmt      = "%s  %s"

	// DetailReviewHeaderFmt renders a PR as its number, title and state.
	DetailReviewHeaderFmt = "#%d  %s  %s"

	// DetailChecks*Glyph mark a status-check's outcome on the REVIEW checks
	// line. DetailChecksFmt renders the passed/failed pair, DetailChecksPendingFmt
	// appends the pending count only when there is one. DetailReviewDecisionFmt
	// renders the human-readable review decision; the DetailReviewDecision*
	// labels are what a raw GHReviewDecision* enum value maps to.
	DetailChecksPassedGlyph  = "✓"
	DetailChecksFailedGlyph  = "✗"
	DetailChecksPendingGlyph = "⧗"
	DetailChecksFmt          = "checks " + DetailChecksPassedGlyph + " %d  " + DetailChecksFailedGlyph + " %d"
	DetailChecksPendingFmt   = "  " + DetailChecksPendingGlyph + " %d"
	DetailReviewDecisionFmt  = "review  %s"

	DetailReviewDecisionApproved         = "approved"
	DetailReviewDecisionChangesRequested = "changes requested"
	DetailReviewDecisionReviewRequired   = "review required"

	// DashboardLabelChildren and DashboardLabelEnv extend the LINKS field
	// labels (DashboardLabelParent, DashboardLabelCreated, DashboardLabelPath
	// already exist for the old WORKTREE section).
	DashboardLabelChildren = "Children"
	DashboardLabelEnv      = "Env"

	// Changes*Fmt render the CHANGES section's summary line, one fragment per
	// porcelain category plus the diff volume, each omitted when zero.
	// ChangesDeletionGlyph is the minus sign, distinct from an ASCII hyphen so a
	// deletion count never reads as a negative number.
	ChangesModifiedFmt   = "%d modified"
	ChangesUntrackedFmt  = "%d untracked"
	ChangesStagedFmt     = "%d staged"
	ChangesDeletionGlyph = "−"
	ChangesDiffStatFmt   = "+%d " + ChangesDeletionGlyph + "%d"

	// ActivityFilesChangedFmt renders ACTIVITY's title-row file count
	// alongside its diff volume — the committed-diff counterpart to CHANGES'
	// porcelain breakdown. `git diff --shortstat` only reports one aggregate
	// count, not a per-status split, so this is the one fragment ACTIVITY has
	// to offer where CHANGES has three.
	ActivityFilesChangedFmt = "%d files changed"

	// Env* render the LINKS "Env" field's drift summary, one fragment per
	// category, joined with DashboardMetaSeparator when several apply.
	EnvMissingFmt     = "%d keys missing"
	EnvConflictingFmt = "%d conflicting"
	EnvOrphanFmt      = "%d orphan"

	// DashboardNotConfigured names a legitimate absence (no env files declared),
	// never presented as a success, and stays glyph-free — nothing is wrong.
	// DashboardUnavailableFmt names a family that failed to read, naming why —
	// it never goes silently empty, and carries the warning glyph the
	// legitimate-absence case must not have: that contrast is the point.
	DashboardNotConfigured  = "not configured"
	DashboardUnavailableFmt = "⚠ unavailable — %s"
)

// EnvTemplateSuffixes are the committed-schema template suffixes recognized on a
// `.env` file, in priority order — the first match is pinned when several templates
// exist for the same target (e.g. `.env.example` wins over `.env.dist`).
var EnvTemplateSuffixes = []string{
	EnvTemplateSuffixExample,
	EnvTemplateSuffixDist,
	EnvTemplateSuffixSample,
	EnvTemplateSuffixTemplate,
	EnvTemplateSuffixTmpl,
}

// DashboardWordmarkLines is the drawn wordmark's three rows, the tall
// header's permanent top-left anchor — the letter spacing (one blank column
// between W, T and M) is deliberate: run together the glyphs are harder to
// read.
var DashboardWordmarkLines = [3]string{
	`╻ ╻ ╺┳╸ ┏┳┓`,
	`┃╻┃  ┃  ┃┃┃`,
	`┗┻┛  ╹  ╹ ╹`,
}

// Self-update: the release source, the install methods it can act on, and the
// policy knobs of the passive update check.
const (
	RepoOwner = "LucasPcq"
	RepoName  = "wtm"

	ModulePath  = "github.com/LucasPcq/wtm"
	BrewFormula = "LucasPcq/tap/wtm"

	ReleaseAPIBase = "https://api.github.com/repos/" + RepoOwner + "/" + RepoName + "/releases"

	// ChecksumsFileName is the SHA256 manifest goreleaser publishes with every release.
	ChecksumsFileName = "checksums.txt"

	// GlobalStateFile holds wtm-written state next to the user config. Kept separate
	// from config.toml: the CLI must never rewrite a file the user hand-edits.
	GlobalStateFile = "state.json"

	UpdateCheckTTL     = 24 * time.Hour
	UpdateCheckTimeout = 2 * time.Second
	UpdateNoticeWait   = 300 * time.Millisecond
	DownloadTimeout    = 60 * time.Second

	EnvNoUpdateCheck = "WTM_NO_UPDATE_CHECK"
	EnvCI            = "CI"
	EnvGitHubActions = "GITHUB_ACTIONS"

	// CmdUpgrade is the self-update command name. The five that follow already
	// exist as literals in their command files and are centralized here because
	// the update-check exclusion list needs to name them.
	CmdUpgrade    = "upgrade"
	CmdShellInit  = "shell-init"
	CmdResolve    = "resolve"
	CmdDaemon     = "daemon"
	CmdCompletion = "completion"
	CmdSchema     = "schema"

	// CmdShellComp and CmdShellCompNoDesc mirror cobra.ShellCompRequestCmd and
	// ShellCompNoDescRequestCmd, the hidden commands a shell invokes on Tab.
	CmdShellComp       = "__complete"
	CmdShellCompNoDesc = "__completeNoDesc"

	// FlagCheck (the read-only report) is shared with `wtm env`.
	FlagVersionPin = "version"

	// ExitCodeUpgradeUnsupported marks an upgrade that cannot proceed on this
	// install: built from source, or the binary is not writable.
	ExitCodeUpgradeUnsupported = 17

	// UpgradeConfirmPrompt keeps a space before the question mark, unlike every
	// other prompt here: it ends on a version number, and "0.26.1?" reads as part
	// of the number rather than as a question.
	UpgradeConfirmPrompt = "Update %s %s → %s ?"

	UpgradeJSONNeedsYes   = "--output json requires --yes or --check (the confirmation prompt cannot run in JSON mode)"
	UpgradeSourceHint     = "this binary was built from source — run `git pull && make install` instead"
	UpgradePinUnsupported = "--version only applies to a standalone binary; pin the version through your package manager instead"
)
