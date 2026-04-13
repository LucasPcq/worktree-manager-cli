package config

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/LucasPcq/wtm/internal/domain"
)

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
