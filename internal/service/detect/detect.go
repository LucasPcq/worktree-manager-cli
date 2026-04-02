// Package detect provides auto-detection functions for the init wizard.
package detect

import (
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/LucasPcq/wtm/internal/domain"
)

// BaseBranch detects the default branch via git symbolic-ref.
// Returns domain.DefaultBaseBranch if detection fails.
func BaseBranch(projectDir string) string {
	cmd := exec.Command("git", "symbolic-ref", "refs/remotes/origin/HEAD")
	cmd.Dir = projectDir
	out, err := cmd.Output()
	if err != nil {
		return domain.DefaultBaseBranch
	}

	ref := strings.TrimSpace(string(out))
	parts := strings.Split(ref, "/")
	if len(parts) == 0 {
		return domain.DefaultBaseBranch
	}

	return parts[len(parts)-1]
}

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

// PackageManager detects the package manager from lockfiles in the project directory.
// Checks in priority order: pnpm, npm, yarn, go, pip.
func PackageManager(projectDir string) domain.PackageManager {
	checks := []struct {
		lockfile string
		pm       domain.PackageManager
	}{
		{domain.LockfilePnpm, domain.PkgManagerPnpm},
		{domain.LockfileNpm, domain.PkgManagerNpm},
		{domain.LockfileYarn, domain.PkgManagerYarn},
		{domain.LockfileGo, domain.PkgManagerGo},
		{domain.LockfilePip, domain.PkgManagerPip},
	}

	for _, c := range checks {
		if fileExists(filepath.Join(projectDir, c.lockfile)) {
			return c.pm
		}
	}

	return domain.PkgManagerNone
}

// InstallCommand returns the default install command for a given package manager.
func InstallCommand(pm domain.PackageManager) string {
	switch pm {
	case domain.PkgManagerPnpm:
		return domain.InstallCmdPnpm
	case domain.PkgManagerNpm:
		return domain.InstallCmdNpm
	case domain.PkgManagerYarn:
		return domain.InstallCmdYarn
	case domain.PkgManagerGo:
		return domain.InstallCmdGo
	case domain.PkgManagerPip:
		return domain.InstallCmdPip
	default:
		return ""
	}
}

// GlobalConfigExists checks whether ~/.config/wtm/config.toml exists.
func GlobalConfigExists() bool {
	configDir, err := os.UserConfigDir()
	if err != nil {
		return false
	}
	return fileExists(filepath.Join(configDir, domain.GlobalConfigDir, domain.GlobalConfigFile))
}

// ProjectConfigExists checks whether .wtm.toml exists in the given directory.
func ProjectConfigExists(projectDir string) bool {
	return fileExists(filepath.Join(projectDir, domain.ConfigFileName))
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

// PnpmWorkspacePackages parses pnpm-workspace.yaml and resolves the package patterns
// to actual directories. Returns nil if pnpm-workspace.yaml doesn't exist.
func PnpmWorkspacePackages(projectDir string) []string {
	wsPath := filepath.Join(projectDir, "pnpm-workspace.yaml")
	data, err := os.ReadFile(wsPath)
	if err != nil {
		return nil
	}

	patterns := parsePnpmWorkspace(string(data))
	var packages []string

	for _, pattern := range patterns {
		matches, err := filepath.Glob(filepath.Join(projectDir, pattern))
		if err != nil {
			continue
		}
		for _, m := range matches {
			info, statErr := os.Stat(m)
			if statErr != nil || !info.IsDir() {
				continue
			}
			rel, relErr := filepath.Rel(projectDir, m)
			if relErr == nil {
				packages = append(packages, rel)
			}
		}
	}

	sort.Strings(packages)
	return packages
}

// parsePnpmWorkspace extracts package patterns from pnpm-workspace.yaml content.
// Handles the simple format: packages:\n  - "pattern"\n  - 'pattern'\n  - pattern
func parsePnpmWorkspace(content string) []string {
	var patterns []string
	inPackages := false

	for _, line := range strings.Split(content, "\n") {
		trimmed := strings.TrimSpace(line)

		if trimmed == "packages:" {
			inPackages = true
			continue
		}

		if inPackages {
			if !strings.HasPrefix(trimmed, "- ") {
				break
			}
			pattern := strings.TrimPrefix(trimmed, "- ")
			pattern = strings.Trim(pattern, "\"'")
			if pattern != "" && !strings.HasPrefix(pattern, "!") {
				patterns = append(patterns, pattern)
			}
		}
	}

	return patterns
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
