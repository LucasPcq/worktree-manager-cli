package config

import (
	"errors"
	"os"
	"path/filepath"

	"github.com/LucasPcq/wtm/internal/domain"
)

// LoadRun reads and parses run.toml from the state directory.
// Returns an empty config (no error) if the file does not exist. Unknown
// keys (typos like `[[profiles]]` instead of `[[profile]]`) surface as
// errors rather than being silently ignored.
func LoadRun(stateDir string) (domain.RunConfig, error) {
	path := filepath.Join(stateDir, domain.RunFileName)

	if _, err := os.Stat(path); errors.Is(err, os.ErrNotExist) {
		return domain.RunConfig{}, nil
	}

	var cfg domain.RunConfig
	if err := decodeStrict(path, &cfg); err != nil {
		return domain.RunConfig{}, err
	}

	return cfg, nil
}
