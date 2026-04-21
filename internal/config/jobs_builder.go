package config

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/rules"
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
	pm := resolveRunnerPrefix(params.PackageManager)

	jobs := make([]domain.JobConfig, 0, len(params.Scripts))
	counts := map[string]int{}

	for _, s := range params.Scripts {
		base := scriptJobName(s)
		counts[base]++
		name := base
		if counts[base] > 1 {
			name = fmt.Sprintf("%s-%d", base, counts[base])
		}

		cwd := s.Workspace
		if cwd == "" {
			cwd = "."
		}

		jobs = append(jobs, domain.JobConfig{
			Name: name,
			Kind: rules.ClassifyScriptKind(s.Name),
			Cmd:  fmt.Sprintf("%s run %s", pm, s.Name),
			Cwd:  cwd,
		})
	}

	return domain.RunConfig{Jobs: jobs}
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
			Cmd:  fmt.Sprintf("%s -f %s up -d", composeCmd, f),
			Stop: fmt.Sprintf("%s -f %s down --remove-orphans", composeCmd, f),
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
