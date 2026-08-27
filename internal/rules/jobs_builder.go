package rules

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/LucasPcq/wtm/internal/domain"
)

// BuildScriptJobsParams holds the inputs for BuildScriptJobs.
type BuildScriptJobsParams struct {
	PackageManager domain.PackageManager
	Scripts        []domain.PackageScript
}

// BuildScriptJobs turns selected package.json scripts into RunConfig entries.
// The command is "<pm> run <scriptName>"; cwd is the workspace dir ("." for root);
// kind is derived from ClassifyScriptKind; no stop is set (services are PID-tracked).
// Job names are "<pkgName>-<scriptName>" for workspace scripts, "<scriptName>" for root.
// Duplicate base names are disambiguated with a counter suffix ("-2", "-3", …).
func BuildScriptJobs(params BuildScriptJobsParams) domain.RunConfig {
	jobs := make([]domain.JobConfig, 0, len(params.Scripts))
	counts := map[string]int{}

	for _, s := range params.Scripts {
		base := scriptJobName(s)
		counts[base]++
		name := base
		if counts[base] > 1 {
			name = fmt.Sprintf("%s-%d", base, counts[base])
		}

		jobs = append(jobs, domain.JobConfig{
			Name: name,
			Kind: scriptKind(s),
			Cmd:  ScriptJobCmd(params.PackageManager, s.Name),
			Cwd:  ScriptJobCwd(s.Workspace),
		})
	}

	return domain.RunConfig{Jobs: jobs}
}

// ScriptJobCmd is the run.toml command emitted for a package script:
// "<pm> run <name>". It is the single source of truth for that command shape,
// shared by BuildScriptJobs and the prefill matcher in prefill.go.
func ScriptJobCmd(pm domain.PackageManager, scriptName string) string {
	return fmt.Sprintf("%s run %s", resolveRunnerPrefix(pm), scriptName)
}

// ScriptJobCwd is the run.toml cwd emitted for a package script: its workspace
// dir, or "." for a root script.
func ScriptJobCwd(workspace string) string {
	if workspace == "" {
		return "."
	}
	return workspace
}

// DockerComposeFileFlag is the "-f <file> " fragment shared by the docker job
// commands BuildDockerJobs emits and the prefill matcher that detects them.
func DockerComposeFileFlag(file string) string {
	return "-f " + file + " "
}

// resolveRunnerPrefix returns the CLI prefix used in "X run <script>" commands.
// Falls back to "pnpm" for non-JS package managers.
func resolveRunnerPrefix(pm domain.PackageManager) string {
	switch pm {
	case domain.PkgManagerPnpm:
		return "pnpm"
	case domain.PkgManagerNpm:
		return "npm"
	case domain.PkgManagerYarn:
		return "yarn"
	default:
		return "pnpm"
	}
}

// scriptJobName returns the base job name for a package script:
// root scripts use the script name directly; workspace scripts prepend the package name.
// scriptKind prefers the kind the caller settled. The name is only a guess, and
// it is wrong in both directions: `preview` serves requests while `start` is
// production. A kind chosen in the wizard outranks it.
func scriptKind(s domain.PackageScript) domain.JobKind {
	if s.Kind != "" {
		return s.Kind
	}
	return ClassifyScriptKind(s.Name)
}

func scriptJobName(s domain.PackageScript) string {
	if s.Workspace == "" {
		return s.Name
	}
	return s.PkgName + "-" + s.Name
}

// BuildInitRunConfig assembles a RunConfig from the answers collected during
// wtm init, merging docker-compose jobs and selected package.json script jobs.
func BuildInitRunConfig(answers domain.InitProjectAnswers, pm domain.PackageManager) domain.RunConfig {
	runCfg := domain.RunConfig{}
	if len(answers.DockerComposeFiles) > 0 {
		runCfg = BuildDockerJobs(answers.DockerComposeCmd, answers.DockerComposeFiles)
	}
	if len(answers.SelectedPackageScripts) > 0 {
		scriptsCfg := BuildScriptJobs(BuildScriptJobsParams{
			PackageManager: pm,
			Scripts:        answers.SelectedPackageScripts,
		})
		runCfg, _ = MergeRunConfigs(runCfg, scriptsCfg)
	}
	// Scripts arrive alphabetically, which declares `dev` ahead of `migrate`.
	// The written file is read back as an order, so it has to be the one a run
	// can follow.
	runCfg.Jobs = TasksFirst(runCfg.Jobs)
	return runCfg
}

// BuildDockerJobs builds a RunConfig with one [[job]] entry per detected
// docker-compose file, using the provided compose command (e.g.
// "docker compose" or "docker-compose"). Each job is kind="service" with a
// stop command, meaning they run as detached services. No profile is emitted.
func BuildDockerJobs(composeCmd string, files []string) domain.RunConfig {
	jobs := make([]domain.JobConfig, 0, len(files))
	counts := map[string]int{}
	for _, f := range files {
		base := jobNameFromComposeFile(f)
		counts[base]++
		name := base
		if counts[base] > 1 {
			name = fmt.Sprintf("%s-%d", base, counts[base])
		}
		jobs = append(jobs, domain.JobConfig{
			Name: name,
			Kind: domain.JobKindService,
			Cmd:  fmt.Sprintf("%s %sup -d", composeCmd, DockerComposeFileFlag(f)),
			Stop: fmt.Sprintf("%s %sdown --remove-orphans", composeCmd, DockerComposeFileFlag(f)),
			Cwd:  ".",
		})
	}
	return domain.RunConfig{Jobs: jobs}
}

// jobNameFromComposeFile turns "docker-compose.dev.yml" into
// "docker-compose-dev", "docker-compose.yaml" into "docker-compose", and
// "docker-compose.prod.yaml" into "docker-compose-prod".
func jobNameFromComposeFile(path string) string {
	base := filepath.Base(path)
	base = strings.TrimSuffix(base, filepath.Ext(base))
	if base == "docker-compose" || base == "" {
		return "docker-compose"
	}
	base = strings.TrimPrefix(base, "docker-compose.")
	return "docker-compose-" + base
}
