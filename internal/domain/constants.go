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

	// EnvCredentialsSeparator ends the userinfo part of a URL. A value is elided
	// there before display: what precedes it is a password, what follows is the
	// host and the port the reader is actually looking for.
	EnvCredentialsSeparator = "@"
	// EnvValueDisplayWidth caps an elided .env value in the port table.
	EnvValueDisplayWidth = 44
	// Ellipsis marks a value the display cut short.
	Ellipsis = "\u2026"

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

	// EnvGoFile is the environment variable used by the shell wrapper to pass the go-file path.
	EnvGoFile = "WTM_GO_FILE"

	// Worktree-scoped variables injected into every job and lifecycle hook, so
	// two worktrees running the same services never share a resource.
	EnvWorktree           = "WTM_WORKTREE"
	EnvBranch             = "WTM_BRANCH"
	EnvOrdinal            = "WTM_ORDINAL"
	EnvPortOffset         = "WTM_PORT_OFFSET"
	EnvComposeProjectName = "COMPOSE_PROJECT_NAME"
	// EnvProject is the repository's slug, as the hostname and the compose
	// project name both derive from it.
	EnvProject = "WTM_PROJECT"

	// MainWorktreeOrdinal is never persisted: the main worktree has no meta.json,
	// so 0 in a linked worktree's metadata means "not allocated yet".
	MainWorktreeOrdinal = 0

	// PortOffsetBlock is the default spacing between two worktrees' ports, used
	// when run.toml does not set one.
	PortOffsetBlock = 10

	// PortProbeTimeout is how long `run up` waits for a declared port to answer
	// before reporting it silent. Generous on purpose: the poll returns as soon
	// as every port has answered, so only a failure ever spends it whole.
	PortProbeTimeout = 15 * time.Second
	// PortProbeInterval paces the poll, PortProbeDialTimeout bounds one dial —
	// a loopback port either answers at once or is not there.
	PortProbeInterval    = 250 * time.Millisecond
	PortProbeDialTimeout = 300 * time.Millisecond

	// PortMin and PortMax bound a declared base port.
	PortMin = 1
	PortMax = 65535

	// HostLabelMaxLen is the DNS limit on a single label of a hostname.
	HostLabelMaxLen = 63
	// HostLabelFallback names a segment whose source slugified to nothing.
	HostLabelFallback = "wtm"

	// DirectURLFmt is a job's URL without the proxy: its own port on the loopback.
	DirectURLFmt = "http://localhost:%d"

	// ProxyTLD is the special-use TLD every wtm route lives under (RFC 6761).
	ProxyTLD = "localhost"
	// ProxyDefaultPort is what the run proxy listens on when the config says nothing.
	ProxyDefaultPort = 4000
	// ProxyURLFmt is a job's named URL.
	ProxyURLFmt = "http://%s:%d"
	// ProxyLoopbackFmt is an address on the loopback: what the server binds, and
	// what a route targets. Never every interface.
	ProxyLoopbackFmt = "127.0.0.1:%d"
	// ProxyScheme is what the proxy dials a job with: the job listens on plain
	// HTTP on the loopback, whatever the browser used to reach the proxy.
	ProxyScheme = "http"

	// The two pages the proxy serves itself, in plain text: service/ may not
	// import lipgloss, and a browser landing here needs the fact, not a style.
	ProxyUnknownHostFmt  = "wtm: no job is published under %s\n\n"
	ProxyKnownRoutesHead = "Routes wtm is currently serving:\n"
	ProxyRouteLineFmt    = "  %s  ->  %s (job %s, worktree %s)\n"
	ProxyNoRoutesLine    = "  (none — start a job that declares a url)\n"
	ProxySilentTargetFmt = "wtm: job %s is published under %s but nothing answers on %s\n"

	// GOOS* name the platforms whose URL opener differs; everything else uses
	// the freedesktop one.
	GOOSDarwin  = "darwin"
	GOOSWindows = "windows"

	// Opener* are the commands that hand a URL to the desktop.
	OpenerDarwin     = "open"
	OpenerWindows    = "rundll32"
	OpenerWindowsArg = "url.dll,FileProtocolHandler"
	OpenerUnix       = "xdg-open"

	// ComposePortVarSuffix ends every variable name wtm introduces in a compose
	// file, and ComposeTemplatedPortFmt is the host side it writes: the default
	// keeps `docker compose up` working on its own, without wtm.
	ComposePortVarSuffix    = "_PORT"
	ComposeTemplatedPortFmt = "${%s:-%d}"

	// ComposeVarNamePrefix fronts a service name starting with a digit, which
	// cannot open an environment variable name.
	ComposeVarNamePrefix = "S"

	// The docker-compose keys wtm reads.
	ComposeServicesKey      = "services"
	ComposePortsKey         = "ports"
	ComposePublishedKey     = "published"
	ComposeTargetKey        = "target"
	ComposeContainerNameKey = "container_name"
	ComposeVolumesKey       = "volumes"
	ComposeNetworksKey      = "networks"
	ComposeNameKey          = "name"
	ComposeExternalKey      = "external"

	// ComposeIsolatedNameFmt is what an absolute name becomes: the compose
	// project fronts it, so it moves with the worktree. The default keeps
	// `docker compose up` working on its own, without wtm, and reproduces the
	// name the file used to pin.
	ComposeIsolatedNameFmt = "${" + EnvComposeProjectName + ":-%s}-%s"
	// ComposeIsolatedBareNameFmt covers the name that is already exactly the
	// project's: there is no suffix left to keep it apart from.
	ComposeIsolatedBareNameFmt = "${" + EnvComposeProjectName + ":-%s}"

	// The reasons an absolute name is left alone.
	ComposeNameReasonAnchor     = "the name carries a YAML anchor — rewriting it would move every site aliasing it"
	ComposeNameReasonAlias      = "the name is a YAML alias — templating it would change every anchor site"
	ComposeNameReasonUnreadable = "wtm could not read this name back from the file, so it will not rewrite it"
	ComposeNameReasonNoProject  = "wtm could not name the repository, so it has nothing to front the name with"

	// The reasons a compose port mapping is left alone.
	ComposePortReasonNoHost     = "no host port to shift — Docker picks one at random"
	ComposePortReasonRange      = "port range mappings cannot be shifted as a block"
	ComposePortReasonNotAPort   = "%q is not a port number"
	ComposePortReasonOutOfRange = "port %d is outside %d-%d"
	ComposePortReasonBadVarName = "%q is not a valid environment variable name"
	ComposePortReasonAlias      = "the ports list is a YAML alias — templating it would change every anchor site"
	ComposePortReasonUnreadable = "wtm could not read this mapping back from the file, so it will not rewrite it"
	ComposePortReasonNoDefault  = "%s has no default — wtm cannot tell which port it stands for"
	ComposePortReasonSharedVar  = "%s is declared with two different bases in this file — wtm cannot tell which one wins"
	ComposePortReasonSharedJob  = "%s is declared with two different bases across the files job %q stacks — wtm cannot tell which one wins"
	ComposePortReasonAnchor     = "the ports list carries a YAML anchor — rewriting it would move every service aliasing it"

	// ComposeFixDefaultFmt suggests the default to add to an explicit template.
	// The container port is a guess the reader confirms, not a value wtm writes.
	ComposeFixDefaultFmt = "add a default, e.g. %s"

	// The section titles of the compose port report.
	ComposePatchedTitle  = "Compose ports templatized"
	ComposePortsTitle    = "Ports declared"
	ComposeWithheldTitle = "Ports left alone"
	ComposeDroppedTitle  = "Ports withdrawn — they could not coexist"
	// ComposeFixIndentFmt indents the geste under the port it belongs to.
	ComposeFixIndentFmt = "  %s"

	// The section titles of the absolute-name report.
	ComposeNamesPatchedTitle  = "Compose names scoped to the worktree"
	ComposeNamesWithheldTitle = "Names left alone"
	// ComposeNamesVolumeWarning follows a renamed volume: the isolation is the
	// point, but the data already written does not travel into it, and a reader
	// who is not told reads the empty volume as data loss.
	ComposeNamesVolumeWarning = "a volume that pinned its name gains one per worktree, each starting empty — the\n" +
		"data already written stays in the volume under the old name"

	// ComposePatchLineFmt is one rewrite as both the wizard and the recap show
	// it: where it is, and what it becomes.
	ComposePatchLineFmt = "%s · %s   %s → %s"
	// ComposeNameLineFmt is one absolute-name rewrite, and ComposeNameKindFmt
	// qualifies the owner with what it is — a service and a volume may share a
	// name without sharing a line.
	ComposeNameLineFmt = "%s · %s %s   %s → %s"
	// ComposeNameWithheldFmt names an absolute name wtm refuses to rewrite, and
	// ComposeNameCollidesFmt is the detail a name withheld for want of
	// authorization carries — there is no refusal to report, only the effect.
	ComposeNameWithheldFmt = "%s · %s %s   %s"
	ComposeNameCollidesFmt = "%s is the same name in every worktree"
	// ComposeFrozenLineFmt names a host port left literal, and ComposeFixLineFmt
	// the mapping to write instead. ComposeFixCmdFmt is the declaration that
	// follows once the file reads a variable.
	ComposeFrozenLineFmt  = "%s · %s   %s binds the same port in every worktree"
	ComposeUnsupportedFmt = "%s · %s   %s"
	ComposeFixLineFmt     = "write %s"
	ComposeFixCmdFmt      = "then: wtm run job edit %s --port %s=%d"
	ComposeFixNoJobFmt    = "then declare it with `wtm run job edit <job> --port %s=%d`"
	ComposeDroppedLineFmt = "%s (job %q, base %d) is dropped — it meets %s (job %q, base %d) %d worktree(s) on"
	ComposeUnreadableFmt  = "%s could not be read: %s"

	// ComposePatchMovedFmt aborts a patch whose target token is no longer where
	// the scan found it.
	ComposePatchMovedFmt = "%s:%d: expected %s but found %s — the file changed since it was read"
	// ComposePatchOutOfRangeFmt aborts a patch pointing past the end of the file.
	ComposePatchOutOfRangeFmt = "%s:%d: line is past the end of the file"
	// ComposePatchPartialFmt names the files already rewritten when a later one
	// fails to write, so the user knows the tree is not as it was.
	ComposePatchPartialFmt = "already rewritten: %s"
	ComposeReadFileFmt     = "read %s: %w"
	ComposeWriteFileFmt    = "write %s: %w"

	// The port probe report: what `run up` observed on the ports a job declared.
	// It states what was seen, never what is at fault — wtm can tell that
	// nothing answers, not why.
	PortProbeTitle     = "Ports declared but not bound"
	PortProbeSilentFmt = "%s · nothing is listening on %s=%d"
	PortProbeBaseFmt   = "  but %d is listening — the base port"
	PortProbeBaseHint  = "  the command ran, but the variable did not reach it"

	// PortProbeHostV4 and PortProbeHostV6 are both dialed: a service bound to
	// ::1 only would otherwise read as silent.
	PortProbeHostV4 = "127.0.0.1"
	PortProbeHostV6 = "::1"

	// FlagNoProbe skips the port check for one invocation.
	FlagNoProbe = "no-probe"

	// The .env port report ([[env_port]] links resolved for one worktree).
	EnvPortsTitle       = "Env ports"
	EnvPortOffsetPrefix = "offset +"
	// EnvPortOffsetNoteFmt orients the reader of the confirmation, whose title
	// already says what is being asked.
	EnvPortOffsetNoteFmt = "This worktree's offset is +%d — the ports below move with it."
	// EnvPortTableRowFmt aligns key, port name, the port move, and the value the
	// key lands on — the only column that can be long, and the only one elided.
	EnvPortTableRowFmt = "%s  %s  %s  %s"
	// EnvPortFileRuleFmt opens a file's group; EnvPortRuleRune fills it out to
	// the table's width.
	EnvPortFileRuleFmt = "── %s "
	EnvPortRuleRune    = "─"
	// The port table's header. Read across, a row says "this key follows that
	// port, which moves from here to there, and becomes this".
	EnvPortHeaderKey     = "KEY"
	EnvPortHeaderFollows = "FOLLOWS"
	EnvPortHeaderPort    = "PORT"
	EnvPortHeaderBecomes = "BECOMES"
	EnvPortMoveFmt       = "%d → %d"

	// EnvPortAnomaliesTitle heads the links wtm reports instead of applying.
	EnvPortAnomaliesTitle = "Env ports left alone"
	EnvPortAnomalyRowFmt  = "%s  %s  %s"
	// The reasons a link is left alone. Guessing which number of a URL is the
	// port can corrupt it, so each of these reports rather than acts. The port
	// they concern is its own column, so no reason repeats it.
	EnvPortReasonMissingKey   = "no such key in this file"
	EnvPortReasonAmbiguousFmt = "%d appears more than once"
	EnvPortReasonNotFoundFmt  = "no %d to shift in the value"
	EnvPortsConfirmPrompt     = "Update these .env values to this worktree's ports?"

	// The trailing verdict of `wtm env`.
	EnvCheckDriftMessage = "Read-only check — run `wtm env` to reconcile."
	// EnvFileInSyncMessage closes a file block with nothing to do.
	// EnvFileKeysInSyncMessage replaces it when the port pass below still moves
	// a value in that same file — "in sync" there would contradict the next
	// section about the very same file.
	EnvFileInSyncMessage     = "in sync — nothing to reconcile"
	EnvFileKeysInSyncMessage = "keys in sync — port values below"

	// The detail column of a file block's key rows.
	EnvKeyRowGap         = "  "
	EnvDetailWouldAddFmt = "would be added from %s"
	EnvDetailAddedFmt    = "added from %s"
	EnvDetailToAddFmt    = "to add from %s"
	EnvDetailConflictFmt = "conflict — local %s vs %s %s"
	EnvDetailMissingFmt  = "needs a value — placeholder %s"
	EnvDetailOrphan      = "orphan — in no source"
	EnvEmptyValueLabel   = "(empty)"
	// The glyphs a file block's rows are marked with. One rune each, so the
	// key column stays aligned whatever a row's status is.
	EnvKeyGlyphAdd       = "+"
	EnvKeyGlyphAttention = "!"
	EnvKeyGlyphOrphan    = "−"
	// EnvFileHeaderFmt heads a file block; EnvFileSourceFmt is its muted half.
	EnvFileHeaderFmt   = "%s   %s"
	EnvFileSourceFmt   = "strategy: %s  ·  source: %s"
	EnvFieldWorktree   = "Worktree"
	EnvFieldMode       = "Mode"
	EnvModeCheckSuffix = "  ·  read-only check"
	// The two ways to apply on the `wtm env` recap. The second exists so the
	// port pass is proposed, as `wtm create` proposes it, and never imposed.
	EnvApplyActionLabel       = "Yes, apply"
	EnvApplyWithoutPortsLabel = "Apply, but leave the port values alone"
	// EnvPortsLeftAloneFmt replaces the table when the pass was declined.
	EnvPortsLeftAloneFmt       = "Env ports left alone — %d value(s) still on the base ports"
	EnvCheckCleanMessage       = "No drift."
	EnvNothingWrittenMessage   = "No changes written."
	EnvReconciledFmt           = "Reconciled %d file(s)."
	EnvPortsShiftedFmt         = "Shifted %d port value(s)."
	EnvReconciledAndShiftedFmt = "Reconciled %d file(s) and shifted %d port value(s)."

	// The [[env_port]] detection of `wtm run init`.
	// EnvPortLinkFmt is one link as the prompt and the recap both show it:
	// "<file> · <key>   follows POSTGRES_PORT (5432)".
	EnvPortLinkFmt          = "%s   follows %s (%d)"
	EnvPortJobSeparator     = "."
	EnvPortLinkSeparator    = " · "
	EnvPortsLinkedTitle     = "Env keys now following a port"
	EnvLinkStepName         = "Env keys"
	RecapStepName           = "Review"
	RecapNotAsked           = "not asked"
	RecapJobLineFmt         = "%s   %s"
	RecapRowIndent          = "  "
	RecapPortFmt            = "%s %d"
	RecapPortSep            = " · "
	RecapNoPort             = "⚠ no port declared"
	RecapTask               = "task"
	RecapDefaultSuffix      = "   (default)"
	RecapJobsTitle          = "Jobs"
	RecapProfilesTitle      = "Profiles"
	RecapAnswersTitle       = "Answers"
	RecapStepIntro          = "This is what `wtm run init` is about to write."
	RecapWriteLabel         = "Write run.toml"
	RecapWriteValue         = "write"
	RecapIgnoredPortWarnFmt = "⚠ %s: the command never mentions the port it is given — leave it only if it\n  reads the variable on its own."
	RecapUndeclaredWarnFmt  = "⚠ %s will bind the same port in every worktree — go back to Ports to declare one."
	EnvPortLinkConfirm      = "Link these keys so each worktree gets its own ports?"
	EnvPortLinkDescription  = "wtm rewrites the port inside each value when a worktree is created or reconciled. The rest of the value is left alone."

	// The wizard step that asks to make the selected compose files per-worktree.
	// Ports and absolute names are one question: accepting half of them still
	// leaves two worktrees unable to run at once, so there is no useful answer
	// that takes one without the other.
	ComposePatchStepName  = "Templatize compose"
	ComposePatchStepYes   = "rewrite"
	ComposePatchStepNo    = "leave as is"
	ComposePatchStepTitle = "Make these compose files per-worktree?"
	ComposePatchStepIntro = "As written, these files bind or name the same thing in every worktree, so a\n" +
		"second one cannot run alongside the first. wtm can rewrite them — every default\n" +
		"keeps `docker compose up` working on its own, without wtm:"
	// ComposePatchStepPortsLead and ComposePatchStepNamesLead head each half of
	// the step, so the reader can tell a bind from a name at a glance.
	ComposePatchStepPortsLead = "Host ports written as literals — every worktree would bind the same one:"
	ComposePatchStepNamesLead = "Names the Docker daemon resolves, which COMPOSE_PROJECT_NAME never reaches:"
	ComposePatchStepEpilogue  = "Declining leaves the files untouched; wtm then declares no port for them, and two\n" +
		"worktrees cannot run these services at once."

	// ComposeChangedTitle and ComposeOrphanTitle head the two report sections
	// that say why a file contributed nothing.
	ComposeChangedTitle = "Compose files skipped — they changed while wtm was reading them"
	ComposeOrphanTitle  = "Compose files with no job to carry their ports"
	ComposeOrphanFmt    = "%s · no job in run.toml runs this file, so its ports were not declared"

	// EnvPortKeyName and EnvPortKeySuffix are the whole convention wtm reads a
	// dev server's port by: a key named PORT, or one ending in _PORT.
	EnvPortKeyName   = "PORT"
	EnvPortKeySuffix = "_PORT"

	// The .env port report. Unlike a compose mapping, a declared port only
	// isolates the job if its command actually reads the variable — which wtm
	// does not know and does not guess, so the notice asks.
	EnvPortsDetectedTitle   = "Ports detected from .env"
	EnvPortDetectedLineFmt  = "%s · %s=%d (%s)"
	PortIsolationTitle      = "These jobs will bind the same port in every worktree"
	PortIsolationLineFmt    = "%s   %s"
	PortIsolationNoPort     = "no port declared"
	PortIsolationIgnoresFmt = "declares %s, but its command never mentions it"
	PortIsolationHint       = "Declare a port, or reference it in the command: `wtm run job edit <job>`"
	EnvPortUnreadable       = "%s could not be read: %s"

	// PortCollisionHorizon is how many worktrees a declared layout is checked
	// against. Two base ports collide when they differ by a multiple of the
	// block, but the multiple says how many worktrees it takes to get there:
	// 3000 and 8080 are both 0 mod 10 and would only meet on the 508th, which is
	// no reason to reject a config.
	PortCollisionHorizon = 20

	// OrdinalLockFileName guards allocation: the scan and the write that follows
	// it must be one step, or two worktrees started at once claim one number.
	OrdinalLockFileName = "ordinal.lock"

	ComposeProjectFallback = "wtm"

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
	FlagReplace    = "replace"
	FlagMine       = "mine"
	FlagReview     = "review"
	FlagCmd        = "cmd"
	FlagKind       = "kind"
	FlagStop       = "stop"
	FlagCwd        = "cwd"
	FlagPort       = "port"
	FlagJobs       = "jobs"
	FlagDefault    = "default"
	FlagTo         = "to"
	FlagKeep       = "keep"
	FlagFiles      = "files"
	FlagOnConflict = "on-conflict"
	// FlagRaw asks for a job's own port rather than the name the proxy serves it
	// under: an address every OS resolves and no proxy has to be up for.
	FlagRaw = "raw"

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
	FlagPatchCompose   = "patch-compose"
	FlagLinkEnv        = "link-env"
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

	// The wizard step settling the kind of a script checked outside the dev ones.
	// The description spells both kinds out: arriving here having checked
	// "build", nothing in the word "type" says which one a build is.
	ScriptKindStepName  = "Job type"
	ScriptKindStepTitle = "How should each of these run?"
	ScriptKindStepDesc  = "Each script you checked becomes a job. Its type is what `wtm run up` does with it.\n" +
		"\n" +
		"  ● service   started, then left running in the background\n" +
		"              a dev server, a watcher, an API — something with no natural end\n" +
		"              → run up hands the terminal back, the job keeps going\n" +
		"\n" +
		"  ● task      run to completion before the profile carries on\n" +
		"              a build, a migration, a seed — something that finishes\n" +
		"              → a non-zero exit aborts the run\n" +
		"\n" +
		"wtm guessed from the name. Correct what it got wrong."

	// The job-type picker: one row per job, both kinds shown, the current one filled.
	KindListEntryFmt   = "%s — %s"
	KindListRadiosFmt  = "%s task   %s service"
	KindRadioOn        = "●"
	KindRadioOff       = "○"
	KindListHelp       = "  ↑↓ navigate • ←→ set type • enter confirm"
	KindListSummaryFmt = "%d services, %d tasks"
	KindListGap        = 2

	// Why a wizard step was never put. An auto-skipped step leaves this line in
	// the recap: a step that vanishes silently while the counter jumps over it
	// reads as a bug.
	SkipReasonNoScriptChecked = "no script checked"
	SkipReasonKindsSettled    = "every checked script is a dev server"
	SkipReasonNoService       = "no long-running service to configure"
	SkipReasonNoJob           = "no job to group into a profile"
	SkipReasonNoPortDetected  = "no port detected for the jobs you kept"
	SkipReasonCommandsRead    = "every command already reads the port it is given"
	SkipReasonNoEnvKeyFollows = "no .env key holds a declared port"

	// The wizard step selecting which package scripts become jobs.
	ScriptsStepName  = "Package scripts"
	ScriptsStepTitle = "Package.json scripts"
	ScriptsStepDesc  = "Only what you check becomes a job. Dev scripts are checked for you; check\n" +
		"anything else you want `wtm run` to start or run."
	ScriptScopeRoot = "root"
	// PortNameDefault is the variable an undeclared service is offered under: the
	// one a process reads without being told to.
	PortNameDefault    = "PORT"
	ScriptLabelFmt     = "%s / %s"
	ScriptItemLabelFmt = "%s / %s — %s run %s"

	// The profile editor: its keys, its rows and its help bar.
	ProfileListStepName   = "Profiles"
	ProfileListKeyRename  = "r"
	ProfileListKeyMerge   = "f"
	ProfileListKeyRemove  = "d"
	ProfileListKeyNew     = "n"
	ProfileListHelp       = "  ↑↓ navigate • r rename • f merge • d remove • n new • enter select"
	ProfileListNamingHelp = "  enter save • esc cancel"
	ProfileListMergeHint  = "  f on another profile to merge it into %q • esc cancel"
	ProfileListMarkPrefix = "→ "
	ProfileListDoneRow    = "✓ Done"
	// ProfileListJobsIndent aligns the wrapped job list of the row under the
	// cursor: an unselected row is truncated, and moving onto it is how the
	// whole list is read.
	ProfileListJobsIndent    = "    "
	ProfileListEllipsis      = "…"
	ProfileListJobSep        = ", "
	ProfileListDefaultSuffix = "  (default)"
	ProfileListNameRequired  = "a profile needs a name"
	ProfileListNameTakenFmt  = "a profile named %q already exists"
	ProfileListSummaryFmt    = "%d profiles"
	ProfileStepTitle         = "What should `wtm run up` start?"
	ProfileStepDesc          = "A profile is a set of jobs started together. Jobs at the repository root —\n" +
		"a compose stack — join every profile, so starting one package alone still\n" +
		"brings its infrastructure up."

	// The step amending a command that never mentions the port wtm injects for it.
	CmdListStepName  = "Commands"
	CmdListStepTitle = "These commands ignore the port wtm gives them"
	CmdListStepDesc  = "wtm injects the variable into the job's environment. It does not touch the command.\n" +
		"\n" +
		"Each job below declares a port whose variable appears nowhere in its command, so\n" +
		"the command will bind whatever it binds today — the same port in every worktree.\n" +
		"\n" +
		"Reference the variable (`--port ${PORT}`), or leave it as it is if the command\n" +
		"already reads it from the environment on its own."
	CmdListEntryFmt = "%s · %s   %s"
	// CmdListEditFmt keeps the variable on the row being edited: it is what the
	// user has to type, and hiding it behind the input leaves nothing to go on.
	CmdListEditFmt    = "%s · reference ${%s} →  %s"
	CmdListVarSep     = ", "
	CmdListReferenced = "✓"
	CmdListDoneRow    = "✓ Done"
	CmdListHelp       = "  ↑↓ navigate • enter edit • esc back"
	CmdListEditHelp   = "  reference the variable in the command • enter save • esc cancel"
	CmdListEmptyErr   = "a job needs a command"
	CmdListSummaryFmt = "%d of %d now reference their port"
	CmdListCharLimit  = 512
	CmdListMinWidth   = 30
	CmdListWidthInset = 20

	// The wizard step reviewing the ports detection pre-filled.
	PortListStepName  = "Ports"
	PortListStepTitle = "One port per service, so two worktrees never collide"
	PortListStepDesc  = "Each port below is injected into its job under the variable's name, shifted by\n" +
		"the worktree's offset — 3001 on main, 3011 on the next worktree.\n" +
		"\n" +
		"A service with no port declared binds the same one everywhere: the second\n" +
		"worktree to start it will fail. Detection filled in what it could find in your\n" +
		"compose files and .env; declare the rest, or leave one blank to accept it."
	PortListEntryFmt = "%s · %s = %d"
	// PortListUndeclaredFmt renders a service nothing was detected for. It reads
	// as a gap to fill rather than as port zero, and the warning is what says
	// why filling it matters.
	PortListUndeclaredFmt        = "%s · %s = —"
	PortListUndeclared           = "not declared — every worktree binds the same port"
	PortListEditFmt              = "%s · %s = %s"
	PortListDoneRow              = "✓ Done"
	PortListHelp                 = "  ↑↓ navigate • enter select • esc back"
	PortListEditHelp             = "  type a port • enter save • esc cancel"
	PortListSummaryFmt           = "%d ports"
	PortListSummaryUndeclaredFmt = "%d ports, %d service(s) left undeclared"
	// PortListRangeErrFmt refuses a value outside the usable range rather than
	// overwriting a detected port that works.
	PortListRangeErrFmt = "%q is not a port between %d and %d"

	// ProfileAllName is the profile gathering every service the init retained.
	// In a single-package repo it is the only one: one profile per package plus
	// a global one collapse into each other, which is what keeps the rule free
	// of a special case.
	ProfileAllName = "all"
	// ProfileRootCwd is the cwd of a job serving every profile — a compose
	// stack, a root script. Sitting at the root is what makes a job shared.
	ProfileRootCwd = "."
	// ProfileProposalMaxPackages is how many package directories the init may
	// split into one profile each. Past it the split stops being a proposal a
	// reader can edit and becomes a list to scroll, so only ProfileAllName is
	// offered and the user splits what they actually want.
	ProfileProposalMaxPackages = 6
	// ProfileNameSegmentSep joins the path segments a colliding profile name is
	// widened with: apps/app-1/back and apps/app-2/back both end in "back".
	ProfileNameSegmentSep = "-"

	// PackageJSONName is the manifest whose presence makes a directory a package.
	PackageJSONName = "package.json"
	// PnpmWorkspaceName is pnpm's workspace declaration, read alongside the
	// `workspaces` field npm, yarn and bun put in the root manifest.
	PnpmWorkspaceName = "pnpm-workspace.yaml"
	// WorkspaceScanMaxDepth bounds how deep below the project root a workspace
	// package is looked for. A workspace pattern can say "**", so the walk needs
	// a floor of its own rather than one the pattern happens to impose.
	WorkspaceScanMaxDepth = 6

	// Script classification keywords for package.json → run.toml mapping.
	// A script is classified as a long-running service when its name matches
	// one of these keywords exactly, as a prefix ("<kw>:"), or as a suffix (":<kw>").
	ScriptKeyDev = "dev"
	// ScriptPreselectKey is the only script name fragment `run init` checks by
	// default. Deliberately blunt: reading the command to guess whether
	// `vite preview` serves requests would rebuild a per-tool heuristic, and
	// maintaining one is what the port probe refused to do for turbo.
	ScriptPreselectKey = ScriptKeyDev
	ScriptKeyStart     = "start"
	ScriptKeyServe     = "serve"
	ScriptKeyWatch     = "watch"

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
	CmdURL      = "url"
	CmdOpen     = "open"
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

	// JobLogsDirName is the directory under the state dir holding the persisted
	// job logs, one subdirectory per worktree:
	// <state-dir>/logs/<worktree>/<job>.log.
	JobLogsDirName = "logs"

	// JobLogFileExt is the extension of a job's log file.
	JobLogFileExt = ".log"

	// JobLogMaxBytes is the size at which a job log rotates and JobLogMaxFiles
	// how many files are kept for that job — the active one plus its backups,
	// <job>.log.1 and <job>.log.2. Retention is deliberately not configurable.
	JobLogMaxBytes = 5 << 20
	JobLogMaxFiles = 3

	// JobLogMaxPendingBytes caps the unterminated tail the sanitizer carries
	// between two chunks. A job that redraws one line forever without ever
	// emitting a newline (a progress bar) would otherwise hold — and re-scan —
	// its whole output; past this many bytes the tail is journaled as it stands.
	JobLogMaxPendingBytes = 64 << 10

	// JobLogTailLines is how much history a surface reads back when it does not
	// say: enough to fill a scrollback, short enough to stay one read.
	JobLogTailLines = 1000

	// JobStreamChunkBytes sizes one read from an attached job, and
	// JobStreamQueueChunks how many reads may wait ahead of a subscriber before
	// it holds the stream back. A job redrawing a full screen writes tens of
	// kilobytes at once, which is what these are sized for.
	JobStreamChunkBytes  = 32 << 10
	JobStreamQueueChunks = 256

	// JobLogTimestampLayout and JobLogSeparator format one persisted log line:
	// an RFC3339 instant, then the sanitized text. Fixed-width prefix so a
	// reader (or a grep) can split every line at the same column.
	JobLogTimestampLayout = time.RFC3339
	JobLogSeparator       = "  "

	// JobUptime*Fmt render how long a job has been up, coarsening as it ages so
	// the column stays narrow: 42s, 5m, 3h07m, 2d05h.
	JobUptimeSecFmt  = "%ds"
	JobUptimeMinFmt  = "%dm"
	JobUptimeHourFmt = "%dh%02dm"
	JobUptimeDayFmt  = "%dd%02dh"

	// JobPaneDefaultCols and JobPaneDefaultRows size a job's terminal emulator
	// until the surface knows how much room it can give it.
	JobPaneDefaultCols = 80
	JobPaneDefaultRows = 24

	// JobPaneScrollbackLines is how far back a job pane can scroll. A scrollback
	// line keeps one styled cell per written column and a cell is 112 bytes, so
	// this is what a pane costs. Measured saturated at this depth: 9.9 MiB for a
	// 120-column pane printing 40-character lines, 27.4 MiB for a 200-column one
	// printing full-width lines — 59 to 165 MiB for a six-job profile, where
	// 2000 lines measured 49 MiB per pane and 294 MiB for the six. Twenty to
	// ninety times the raw bytes it holds, against 5 MB x 3 kept on disk per job,
	// which is why this stops at the depth JobLogTailLines reads back: the
	// sanitized log file, not the emulator, is where the history lives.
	JobPaneScrollbackLines = 1000

	// JobPaneScrollbackBurstFactor is how much more than that a pane lets its
	// emulator hold while someone reads back through the history — the room the
	// pane needs to count the lines each write pushes, which a buffer that is
	// evicting no longer reports. It is given back at the live tail.
	JobPaneScrollbackBurstFactor = 2

	// ShellBin is the interpreter every command written in a config file runs
	// through — a job's cmd and stop, and every lifecycle hook. POSIX sh rather
	// than the user's own shell: a run.toml shared across a team must behave the
	// same on every machine.
	ShellBin = "/bin/sh"

	// ShellCommandFlag runs the line that follows it, ShellSyntaxCheckFlag parses
	// it without running anything.
	ShellCommandFlag     = "-c"
	ShellSyntaxCheckFlag = "-n"

	// JobAlreadyRunningSuffix is the tail of the daemon error returned when a
	// job is started while already running. Callers match on it to treat a
	// repeat start (e.g. re-running `run up` while services are up) as a benign
	// no-op rather than a failure that aborts the profile.
	JobAlreadyRunningSuffix = "is already running"

	// RunViewSidebarWidth is the job list's column budget beside a pane, and
	// RunViewSidebarMinPaneCols what the pane must keep for the list to be shown
	// at all: below it the list is dropped rather than squeezing the output both
	// of them exist to show.
	RunViewSidebarWidth       = 26
	RunViewSidebarMinPaneCols = 40

	// RunViewMinBodyRows is the height the body keeps before a notice band is
	// allowed to take rows from it.
	RunViewMinBodyRows = 3

	// RunViewMinPanelCols and RunViewMinPanelRows are the smallest a bordered
	// panel can be and still hold a cell of what it frames.
	RunViewMinPanelCols = 3
	RunViewMinPanelRows = 3

	// RunViewBorderWidth is what a panel's border costs it in columns, and
	// RunViewPanelChrome what the border and the title row together cost it in
	// rows.
	RunViewBorderWidth = 2
	RunViewPanelChrome = 3

	// RunViewMsgBuffer sizes the channel the stream readers post on, and
	// RunViewPollSeconds how often the job list is re-read from the daemon.
	RunViewMsgBuffer   = 64
	RunViewPollSeconds = 2

	// RunViewRenderFPS throttles the redraw of a pane being written to. Writing
	// a chunk into the emulator costs a fraction of rendering the grid, so the
	// bytes are taken as they come and only the drawing is paced.
	RunViewRenderFPS = 30

	// RunViewScrollLines is how far one scroll key moves through a pane's
	// history; a page moves by the pane's own height.
	RunViewScrollLines = 3

	// RunViewJobsTitle heads the job list and RunViewEmptyMessage stands in for
	// it when the worktree declares none.
	RunViewJobsTitle     = "JOBS"
	RunViewEmptyMessage  = "No jobs declared. Add them to run.toml with `wtm run init`."
	RunViewNoMatchFmt    = "No job matches %q."
	RunViewFilterPrompt  = "filter: "
	RunViewFilterHintFmt = "filter %q · esc clears"

	// RunViewSeparator joins two things said on one row of the run view.
	RunViewSeparator = " · "

	// RunViewCursorMark points at the job whose pane is on screen, and
	// RunViewMark* are the status marks in front of every job's name.
	RunViewCursorMark  = "▸"
	RunViewMarkRunning = "●"
	RunViewMarkStopped = "○"
	RunViewMarkCrashed = "✗"

	// RunViewPaneWaiting and RunViewPaneNoHistory stand in
	// for a pane with nothing in it yet, and RunViewPane*Label say where what is
	// in it came from: the job itself, or the log file it left behind.
	RunViewPaneWaiting      = "Waiting for output…"
	RunViewPaneNoHistory    = "No output recorded for this job."
	RunViewPaneHistoryLabel = "history"
	RunViewPaneLiveLabel    = "live"
	RunViewPaneScrollFmt    = "scrolled %d lines back"

	// RunViewHeaderTitle names the view and RunViewRunningFmt counts what it is
	// showing.
	RunViewHeaderTitle = "wtm run"
	// RunViewHeaderProfileFmt names the profile the run brought up, next to the
	// view's own title: several profiles share this screen and nothing else on
	// it says which one was started.
	RunViewHeaderProfileFmt = "wtm run · %s"
	RunViewRunningFmt       = "%d/%d running"

	// RunViewHelpBrowse and RunViewHelpFilter are the footer's key reminders,
	// one per mode the keyboard can be in.
	RunViewHelpBrowse = "↑↓ job · / filter · pgup/pgdn scroll · enter focus · o open · r refresh · q detach"
	RunViewHelpFilter = "type to filter · enter apply · esc clear"

	// RunViewFocusKey passes every keystroke to the job. Taking them back needs
	// a key no child application can claim, and no single one is free: a
	// terminal cannot even tell shift+enter from enter. So the exit is a
	// repeat — the first press still reaches the job, a second one within
	// RunViewFocusExitWindow means the reader, not the job.
	RunViewFocusKey     = "enter"
	RunViewFocusExitKey = "esc"

	// RunViewFocusHintFmt replaces the footer's key reminders while a job has the
	// keyboard, and RunViewFocusLabel marks the pane that holds it.
	RunViewFocusHintFmt = "focus %s — every key goes to the job · %s twice gives them back"
	RunViewFocusLabel   = "focus"

	// RunViewFocusExitWindow is how close the second exit key has to land to
	// read as a way out rather than as two keystrokes meant for the job.
	RunViewFocusExitWindow = 600 * time.Millisecond

	// RunViewNotAttachableFmt is why focus is refused: there is no live stream
	// behind the pane, only what the log file kept.
	RunViewNotAttachableFmt = "%s has no live stream to type into."

	// RunViewStepFmt reports where a profile's start sequence stands, and
	// RunViewMarkStarting / RunViewMarkDone mark the job it is on in the list.
	RunViewStepFmt      = "starting %d/%d · %s"
	RunViewMarkStarting = "◌"
	RunViewMarkDone     = "✓"

	// RunViewAbortTitle heads the report of a profile that gave up, and
	// RunViewAbort*Fmt are the three things it has to say: what failed and where,
	// what was left running, and what was never reached.
	RunViewAbortTitle         = "Profile aborted"
	RunViewAbortFailedFmt     = "failed at step %d/%d: %s — %s"
	RunViewAbortRunningFmt    = "left running: %s"
	RunViewAbortNotStartedFmt = "not started: %s"
	RunViewAbortDismiss       = "esc dismisses this report"

	// RunViewRecapTitle heads the recap printed once the screen is given back,
	// and RunViewRecap*Fmt are its lines: what is running, what ran, what did
	// not, and the two commands that act on any of it.
	RunViewRecapTitle         = "Jobs"
	RunViewRecapProfileFmt    = "Profile:      %s"
	RunViewRecapRunningFmt    = "Running:      %s"
	RunViewRecapCompletedFmt  = "Completed:    %s"
	RunViewRecapFailedFmt     = "Failed:       %s"
	RunViewRecapNotStartedFmt = "Not started:  %s"
	RunViewRecapNoneRunning   = "No job left running."
	RunViewRecapLogsHint      = "wtm run logs  — reopen this view"
	RunViewRecapDownHint      = "wtm run down  — stop the jobs"
	// RunViewRecapListSep joins the jobs named on one recap line.
	RunViewRecapListSep = ", "

	// RunStream* are how the line surface reports a profile's start sequence when
	// there is no screen to take over: one step announced, one line per job once
	// the daemon has answered for it, then the two commands that act on what is
	// left running.
	RunStreamProfileFmt = "Profile %s"
	RunStreamStepFmt    = "[%d/%d] %s"
	RunStreamStartedFmt = "%s started"
	// RunPortsSuffixFmt qualifies a name with the ports behind it — the line
	// announcing a started job, and the recap of what a job gained.
	// RunPortEntryFmt is one of those ports.
	RunPortsSuffixFmt = "%s · %s"
	RunPortEntryFmt   = "%s=%d"
	// RunURLPickerTitle heads the picker `run open` offers when several jobs
	// publish and the run is interactive enough to ask.
	RunURLPickerTitle = "Which job to open"

	// RunURLSuffixSep sets a job's URL apart from the line announcing it, far
	// enough that a terminal-detected link does not swallow the ports before it.
	RunURLSuffixSep     = "   "
	RunStreamAlreadyFmt = "%s already running"
	RunStreamDoneFmt    = "%s done"
	RunStreamNextHint   = "wtm run logs to attach · wtm run down to stop"

	// RunAbort* report the partial state a profile that gave up left behind, on
	// the surface that has no room to draw it: where it stopped, what nothing
	// tore down, what it never reached, and the way out.
	RunAbortStepFmt         = "Profile aborted at step %d/%d (%s)."
	RunAbortRunningLabel    = "Left running:"
	RunAbortNotStartedLabel = "Not started: "
	RunAbortHint            = "fix and re-run `wtm run up` · `wtm run down` to stop everything"

	// RunLogsPrefixFmt marks which job a line came from when several of them
	// share one stream, RunLogsHistoryHint says a job that is not running is
	// being read back from its log file, and RunLogsNoJobs stands in for a
	// worktree with nothing to read.
	RunLogsPrefixFmt   = "[%s]"
	RunLogsHistoryHint = "%s is not running — reading back its log file."
	RunLogsNoJobs      = "No running jobs in this worktree."

	// RunDaemonConnecting, RunStartingFmt and RunTaskRunningFmt are what a run
	// command says while it waits on the daemon.
	RunDaemonConnecting = "Connecting to daemon…"
	RunStartingFmt      = "Starting %s…"
	RunTaskRunningFmt   = "Running task %s"

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
	DashboardContextSep      = " · "
	DashboardFetchedFmt      = "fetched %s"
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
	DashboardLabelCreated = "Created"

	DashboardHelpWide = "↑↓ select · n new · m actions · a bulk · tab view · o output · r refresh · ? help · q quit"
	// DashboardHelpTree drops "n new": the Tree tab lays out what exists, and a new
	// worktree is created from the list it would appear in.
	DashboardHelpTree   = "↑↓ select · m actions · a bulk · tab view · o output · r refresh · ? help · q quit"
	DashboardHelpNarrow = "↑↓ select · enter detail · n new · m actions · a bulk · o output · r refresh · ? help · q quit"
	DashboardHelpDetail = "esc back · ↑↓ select · o output · r refresh · q quit"

	// DashboardHelpTitle heads the key/mouse reference overlay. Every clickable
	// zone is listed there with its keyboard equivalent.
	DashboardHelpTitle = "Keys & mouse"

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
	// DashboardMenuRefreshBase is the only entry the base row offers: it hangs off
	// nothing, so there is no rebase to run on it — only its own fast-forward.
	DashboardMenuRefreshBase = "Refresh base branch"
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
	DashboardSyncRowTitle     = "Sync this worktree"
	DashboardRefreshBaseTitle = "Refresh base branch"
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

	// KeyOpenURL opens the selected job's URL in a browser.
	KeyOpenURL = "o"

	KeyHelp = "?"
	// KeyQuit leaves the dashboard. Esc does not: it only closes what is open, so
	// a persistent dashboard is never left by accident.
	KeyQuit = "q"

	// EscapePrefix is the leading escape a terminal sends for an alt-modified
	// key, and KeyCtrlPrefix how a control combination is named.
	EscapePrefix  = "\x1b"
	KeyCtrlPrefix = "ctrl+"

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
// WorktreeScopedEnv names the variables that describe one worktree and must
// never be inherited. The run daemon is global and outlives the command that
// forked it, so its own environment holds whichever worktree started it; a job
// gets these from its request or not at all.
var WorktreeScopedEnv = []string{
	EnvWorktree,
	EnvBranch,
	EnvOrdinal,
	EnvPortOffset,
	EnvComposeProjectName,
}

var DashboardWordmarkLines = [3]string{
	`╻ ╻ ╺┳╸ ┏┳┓`,
	`┃╻┃  ┃  ┃┃┃`,
	`┗┻┛  ╹  ╹ ╹`,
}
