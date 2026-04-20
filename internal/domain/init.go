package domain

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
	EnvFiles           []string
	PackageManager     PackageManager
	InstallCommand     string
	DockerComposeFiles []string
	DockerComposeCmd   string
	MonorepoPackages   []string
	PackageScripts     []PackageScript
}

// InitGlobalAnswers holds the wizard answers for global config setup.
type InitGlobalAnswers struct {
	Agent AgentType
	Shell ShellType
}

// InitProjectAnswers holds the wizard answers for project config setup.
type InitProjectAnswers struct {
	BasePath              string
	BaseBranch            string
	EnvCopyFiles          []string
	EnvStrategy           EnvStrategy
	InstallCommand        string
	OnCreateExtra         []HookCommand
	Agent                 AgentType
	AgentOverride         bool
	DockerComposeFiles    []string
	DockerComposeCmd      string
	SelectedPackageScripts []PackageScript
}
