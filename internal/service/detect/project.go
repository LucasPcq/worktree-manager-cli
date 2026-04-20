package detect

import "github.com/LucasPcq/wtm/internal/domain"

// ProjectEnvironment runs all detection probes for a project directory and
// returns a fully-populated InitDetectionResult. Package script kinds are
// pre-classified so the wizard can drive selection without further logic.
func ProjectEnvironment(dir string) domain.InitDetectionResult {
	pm := PackageManager(dir)
	scripts := PackageJSONScripts(dir)
	for i := range scripts {
		scripts[i].Kind = domain.ClassifyScriptKind(scripts[i].Name)
	}
	return domain.InitDetectionResult{
		BaseBranch:         BaseBranch(dir),
		EnvFiles:           EnvFiles(dir),
		PackageManager:     pm,
		InstallCommand:     InstallCommand(pm),
		DockerComposeFiles: DockerComposeFiles(dir),
		DockerComposeCmd:   DockerComposeCommand(),
		MonorepoPackages:   PnpmWorkspacePackages(dir),
		PackageScripts:     scripts,
	}
}
