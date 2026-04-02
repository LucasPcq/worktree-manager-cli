package detect

import (
	"os"
	"path/filepath"

	"github.com/LucasPcq/wtm/internal/domain"
)

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
