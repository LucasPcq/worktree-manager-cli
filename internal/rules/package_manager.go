package rules

import "github.com/LucasPcq/wtm/internal/domain"

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
