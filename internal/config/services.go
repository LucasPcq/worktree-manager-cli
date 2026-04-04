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

// ValidateServices checks for ambiguous config and returns warnings.
func ValidateServices(cfg domain.ServicesConfig) []string {
	var warnings []string

	// Check duplicate service names
	seenServices := map[string]bool{}
	for _, svc := range cfg.Services {
		if seenServices[svc.Name] {
			warnings = append(warnings, fmt.Sprintf("duplicate service name %q — only the first definition is used", svc.Name))
		}
		seenServices[svc.Name] = true
	}

	// Check duplicate profile names
	seenProfiles := map[string]bool{}
	for _, p := range cfg.Profiles {
		if seenProfiles[p.Name] {
			warnings = append(warnings, fmt.Sprintf("duplicate profile name %q — only the first definition is used", p.Name))
		}
		seenProfiles[p.Name] = true
	}

	// Check multiple default profiles
	defaultCount := 0
	for _, p := range cfg.Profiles {
		if p.Default {
			defaultCount++
		}
	}
	if defaultCount > 1 {
		warnings = append(warnings, fmt.Sprintf("%d profiles marked as default — only the first is used", defaultCount))
	}

	return warnings
}
