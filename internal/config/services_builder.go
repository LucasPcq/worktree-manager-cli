package config

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/LucasPcq/wtm/internal/domain"
)

// BuildDockerServices builds a ServicesConfig with one [[services]] entry per
// detected docker-compose file, using the provided compose command invocation
// (e.g. "docker compose" or "docker-compose"). No profile is emitted.
func BuildDockerServices(composeCmd string, files []string) domain.ServicesConfig {
	services := make([]domain.ServiceConfig, 0, len(files))
	counts := map[string]int{}
	for _, f := range files {
		base := serviceNameFromComposeFile(f)
		counts[base]++
		name := base
		if counts[base] > 1 {
			name = fmt.Sprintf("%s-%d", base, counts[base])
		}
		services = append(services, domain.ServiceConfig{
			Name: name,
			Cmd:  fmt.Sprintf("%s -f %s up -d", composeCmd, f),
			Stop: fmt.Sprintf("%s -f %s down --remove-orphans", composeCmd, f),
			Cwd:  ".",
		})
	}
	return domain.ServicesConfig{Services: services}
}

// serviceNameFromComposeFile turns "docker-compose.dev.yml" into
// "docker-compose-dev", "docker-compose.yaml" into "docker-compose", and
// "docker-compose.prod.yaml" into "docker-compose-prod".
func serviceNameFromComposeFile(path string) string {
	base := filepath.Base(path)
	base = strings.TrimSuffix(base, filepath.Ext(base))
	if base == "docker-compose" || base == "" {
		return "docker-compose"
	}
	base = strings.TrimPrefix(base, "docker-compose.")
	return "docker-compose-" + base
}
