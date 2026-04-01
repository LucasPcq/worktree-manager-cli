// Package detect provides auto-detection functions for the init wizard.
package detect

import (
	"os"
	"os/exec"
	"path/filepath"
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
// Excludes node_modules, .trees, .git, and vendor directories.
func EnvFiles(projectDir string) []string {
	var files []string

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
		if name == ".env" || name == ".env.example" || name == ".env.local" {
			rel, relErr := filepath.Rel(projectDir, path)
			if relErr != nil {
				return nil
			}
			files = append(files, rel)
		}

		return nil
	})

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

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
