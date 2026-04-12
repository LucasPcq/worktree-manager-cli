package domain

// InitDetectionResult holds all auto-detected values for repo init.
type InitDetectionResult struct {
	BaseBranch         string
	EnvFiles           []string
	PackageManager     PackageManager
	InstallCommand     string
	DockerComposeFiles []string
	MonorepoPackages   []string
}

// InitGlobalAnswers holds the wizard answers for global config setup.
type InitGlobalAnswers struct {
	Agent AgentType
	Shell ShellType
}

// InitProjectAnswers holds the wizard answers for project config setup.
type InitProjectAnswers struct {
	BasePath        string
	BaseBranch      string
	EnvCopyFiles    []string
	EnvStrategy     EnvStrategy
	InstallCommand string
	OnCreateExtra  []HookCommand
	Agent          AgentType
	AgentOverride   bool
}
