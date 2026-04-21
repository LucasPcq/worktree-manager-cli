package config

import (
	"errors"
	"os"
	"path/filepath"

	"github.com/LucasPcq/wtm/internal/domain"
)

// LoadRun reads and parses .wtm/run.toml from the project directory.
// Returns an empty config (no error) if the file does not exist. Unknown
// keys (typos like `[[profiles]]` instead of `[[profile]]`) surface as
// errors rather than being silently ignored.
func LoadRun(projectDir string) (domain.RunConfig, error) {
	path := filepath.Join(projectDir, domain.ProjectDirName, domain.RunFileName)

	if _, err := os.Stat(path); errors.Is(err, os.ErrNotExist) {
		return domain.RunConfig{}, nil
	}

	var cfg domain.RunConfig
	if err := decodeStrict(path, &cfg); err != nil {
		return domain.RunConfig{}, err
	}

	return cfg, nil
}
