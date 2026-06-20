package rules

import (
	"fmt"
	"strings"

	"github.com/LucasPcq/wtm/internal/domain"
)

// InstallCommandFromHooks returns the first bare on_create hook (no cwd) — the
// conventional install command that `wtm init` writes first. Returns "" if none.
func InstallCommandFromHooks(onCreate []domain.HookCommand) string {
	for _, h := range onCreate {
		if h.Cwd == "" {
			return h.Cmd
		}
	}
	return ""
}

// MonorepoCwdsFromHooks returns the set of cwds referenced by per-package hooks.
func MonorepoCwdsFromHooks(onCreate []domain.HookCommand) map[string]bool {
	cwds := map[string]bool{}
	for _, h := range onCreate {
		if h.Cwd != "" {
			cwds[h.Cwd] = true
		}
	}
	return cwds
}

// DockerFilesConfigured returns the set of detected docker-compose files that
// already back a job in run, matched on the "-f <file> " fragment that
// BuildDockerJobs emits.
func DockerFilesConfigured(run domain.RunConfig, files []string) map[string]bool {
	configured := map[string]bool{}
	for _, f := range files {
		needle := "-f " + f + " "
		for _, job := range run.Jobs {
			if strings.Contains(job.Cmd, needle) {
				configured[f] = true
				break
			}
		}
	}
	return configured
}

// ScriptsConfigured returns the set of detected script indices that already back
// a job in run, matched on the "<pm> run <name>" command and cwd that
// BuildScriptJobs emits.
func ScriptsConfigured(run domain.RunConfig, scripts []domain.PackageScript, pm domain.PackageManager) map[int]bool {
	prefix := resolveRunnerPrefix(pm)
	configured := map[int]bool{}
	for i, s := range scripts {
		cmd := fmt.Sprintf("%s run %s", prefix, s.Name)
		cwd := s.Workspace
		if cwd == "" {
			cwd = "."
		}
		for _, job := range run.Jobs {
			if job.Cmd == cmd && job.Cwd == cwd {
				configured[i] = true
				break
			}
		}
	}
	return configured
}
