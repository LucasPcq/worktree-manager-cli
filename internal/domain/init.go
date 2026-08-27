package domain

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

// PackageScript is one script entry discovered via package.json, possibly
// inside a pnpm workspace package.
type PackageScript struct {
	Name      string  // script name, e.g. "dev"
	Cmd       string  // raw script value from package.json
	Workspace string  // relative dir of the package, "" for root
	PkgName   string  // from package.json "name" field, npm scope stripped
	Kind      JobKind // pre-classified kind; used to drive default selection in the wizard
}

// InitDetectionResult holds all auto-detected values for repo init.
type InitDetectionResult struct {
	BaseBranch         string
	Branches           []BranchCandidate
	EnvFiles           []EnvFile
	PackageManager     PackageManager
	InstallCommand     string
	DockerComposeFiles []string
	DockerComposeCmd   string
	// ComposeScans holds each detected file's port mappings, keyed by the same
	// relative path as DockerComposeFiles.
	ComposeScans     map[string]ComposeScan
	MonorepoPackages []string
	PackageScripts   []PackageScript
}

// InitGlobalAnswers holds the wizard answers for global config setup.
type InitGlobalAnswers struct {
	Shell ShellType
}

// RecapField is one aligned "label   value" row of a framed command recap (the
// init config summary, the create result fields, etc.).
type RecapField struct {
	Label string
	Value string
}

// InitProjectAnswers holds the wizard answers for project config setup.
// The Skip* flags record sections the user opted out of (via the wizard skip
// key or the --skip-* flags); they drive whether each section is written as
// active config or left commented as a template.
type InitProjectAnswers struct {
	BasePath           string
	BaseBranch         string
	EnvFiles           []EnvFile
	EnvStrategy        EnvStrategy
	OnCreate           []HookCommand
	OnClean            []HookCommand
	DockerComposeFiles []string
	DockerComposeCmd   string
	// PatchCompose authorizes rewriting the selected compose files so their
	// frozen host ports and the names they pin absolutely read a variable. One
	// axis, not two: accepting half of them still leaves two worktrees unable to
	// run at once. Never inferred — it is the wizard's answer, or the
	// --patch-compose flag.
	PatchCompose           bool
	SelectedPackageScripts []PackageScript
	// Ports is what the wizard settled for the detected ports, and Profiles the
	// split `run up` will offer. Empty Profiles means the run could not ask: the
	// proposal is then taken as answered, since a profile is what makes `run up`
	// start something rather than everything.
	Ports    []PortEntry
	Profiles []ProfileConfig
	// Cmds is the commands the wizard amended so they read the port wtm injects.
	Cmds []JobCmdFix
	// URLs names the jobs the wizard left checked in the URLs step, and
	// URLsAsked says the step ran at all: unchecking every job withdraws every
	// url, where a run that never asked leaves the proposal standing.
	URLs      []string
	URLsAsked bool
	// LinkEnv is what the wizard settled for the .env keys holding a declared
	// port; EnvLinksAsked says the question was put at all, so a run that asked
	// and got "no" is not mistaken for one that never asked.
	LinkEnv       bool
	EnvLinksAsked bool
	SkipEnv       bool
	SkipHooks     bool
	SkipClean     bool
}
