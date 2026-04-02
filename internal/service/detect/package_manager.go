package detect

import (
	"path/filepath"

	"github.com/LucasPcq/wtm/internal/domain"
)

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

	for _, check := range checks {
		if fileExists(filepath.Join(projectDir, check.lockfile)) {
			return check.pm
		}
	}

	return domain.PkgManagerNone
}

// InstallCommand returns the default install command for a given package manager.
func InstallCommand(pm domain.PackageManager) string {
	switch pm {
	case domain.PkgManagerPnpm:
		return domain.InstallCommandPnpm
	case domain.PkgManagerNpm:
		return domain.InstallCommandNpm
	case domain.PkgManagerYarn:
		return domain.InstallCommandYarn
	case domain.PkgManagerGo:
		return domain.InstallCommandGo
	case domain.PkgManagerPip:
		return domain.InstallCommandPip
	default:
		return ""
	}
}
