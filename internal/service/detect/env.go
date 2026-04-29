package detect

import (
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/LucasPcq/wtm/internal/infra"
)

// EnvFiles scans the project directory recursively for .env and .env.example files.
// Returns deduplicated target names (e.g. ".env", not ".env.example") since copy_files
// lists the target names and the env strategy determines the source.
// Excludes node_modules, .trees, .git, and vendor directories.
func EnvFiles(projectDir string) []string {
	seen := map[string]bool{}

	skipDirs := map[string]bool{
		"node_modules": true,
		".trees":       true,
		".git":         true,
		"vendor":       true,
		"dist":         true,
	}

	_ = filepath.Walk(projectDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}

		if info.IsDir() {
			if skipDirs[info.Name()] {
				return filepath.SkipDir
			}
			return nil
		}

		name := info.Name()
		if name != ".env" && name != ".env.example" && name != ".env.local" {
			return nil
		}

		rel, relErr := filepath.Rel(projectDir, path)
		if relErr != nil {
			return nil
		}

		// Normalize to target name: strip .example suffix
		target := strings.TrimSuffix(rel, ".example")
		seen[target] = true

		return nil
	})

	files := make([]string, 0, len(seen))
	for f := range seen {
		files = append(files, f)
	}
	sort.Strings(files)

	return files
}

// DockerComposeFiles scans for docker-compose*.yml and docker-compose*.yaml files.
func DockerComposeFiles(projectDir string) []string {
	var files []string

	patterns := []string{"docker-compose*.yml", "docker-compose*.yaml"}
	for _, pattern := range patterns {
		matches, err := filepath.Glob(filepath.Join(projectDir, pattern))
		if err != nil {
			continue
		}
		for _, m := range matches {
			rel, relErr := filepath.Rel(projectDir, m)
			if relErr == nil {
				files = append(files, rel)
			}
		}
	}

	sort.Strings(files)
	return files
}

// DockerComposeCommand returns the docker-compose invocation available on the host.
func DockerComposeCommand() string {
	return infra.DockerComposeCommand()
}
