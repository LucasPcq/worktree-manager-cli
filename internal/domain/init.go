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
	MonorepoPackages   []string
	PackageScripts     []PackageScript
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
	BasePath               string
	BaseBranch             string
	EnvFiles               []EnvFile
	EnvStrategy            EnvStrategy
	OnCreate               []HookCommand
	OnClean                []HookCommand
	DockerComposeFiles     []string
	DockerComposeCmd       string
	SelectedPackageScripts []PackageScript
	SkipEnv                bool
	SkipHooks              bool
	SkipClean              bool
}
