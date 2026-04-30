package detect

import (
	"os"
	"path/filepath"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/infra"
)

// GlobalConfigExists checks whether ~/.config/wtm/config.toml exists.
func GlobalConfigExists() bool {
	configDir, err := os.UserConfigDir()
	if err != nil {
		return false
	}
	return infra.FileExists(filepath.Join(configDir, domain.GlobalConfigDir, domain.GlobalConfigFile))
}

// ProjectConfigExists checks whether config.toml exists in the given state dir.
func ProjectConfigExists(stateDir string) bool {
	return infra.FileExists(filepath.Join(stateDir, domain.ConfigFileName))
}
