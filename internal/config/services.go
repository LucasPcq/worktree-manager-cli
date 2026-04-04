package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
	"github.com/LucasPcq/wtm/internal/domain"
)

// LoadServices reads and parses .wtm.services.toml from the project directory.
// Returns an empty config (no error) if the file does not exist.
func LoadServices(projectDir string) (domain.ServicesConfig, error) {
	path := filepath.Join(projectDir, domain.ProjectDirName, domain.ServicesFileName)

	if _, err := os.Stat(path); errors.Is(err, os.ErrNotExist) {
		return domain.ServicesConfig{}, nil
	}

	var cfg domain.ServicesConfig
	if _, err := toml.DecodeFile(path, &cfg); err != nil {
		return domain.ServicesConfig{}, fmt.Errorf("parse %s: %w", path, err)
	}

	return cfg, nil
}
