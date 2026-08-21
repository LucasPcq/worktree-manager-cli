package detect

import (
	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/rules"
	"github.com/LucasPcq/wtm/internal/service/compose"
)

// ProjectEnvironment runs all detection probes for a project directory and
// returns a fully-populated InitDetectionResult. Package script kinds are
// pre-classified so the wizard can drive selection without further logic.
func ProjectEnvironment(dir string) domain.InitDetectionResult {
	pm := PackageManager(dir)
	composeFiles := DockerComposeFiles(dir)
	scripts := PackageJSONScripts(dir)
	for i := range scripts {
		scripts[i].Kind = rules.ClassifyScriptKind(scripts[i].Name)
	}
	return domain.InitDetectionResult{
		BaseBranch:         BaseBranch(dir),
		Branches:           Branches(dir),
		EnvFiles:           EnvFiles(dir),
		PackageManager:     pm,
		InstallCommand:     rules.InstallCommand(pm),
		DockerComposeFiles: composeFiles,
		DockerComposeCmd:   DockerComposeCommand(),
		ComposeScans:       compose.ScanAll(dir, composeFiles),
		MonorepoPackages:   PnpmWorkspacePackages(dir),
		PackageScripts:     scripts,
	}
}
