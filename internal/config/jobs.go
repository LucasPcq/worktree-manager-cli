package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
	"github.com/LucasPcq/wtm/internal/domain"
)

// LoadRun reads and parses .wtm/run.toml from the project directory.
// Returns an empty config (no error) if the file does not exist.
func LoadRun(projectDir string) (domain.RunConfig, error) {
	path := filepath.Join(projectDir, domain.ProjectDirName, domain.RunFileName)

	if _, err := os.Stat(path); errors.Is(err, os.ErrNotExist) {
		return domain.RunConfig{}, nil
	}

	var cfg domain.RunConfig
	if _, err := toml.DecodeFile(path, &cfg); err != nil {
		return domain.RunConfig{}, fmt.Errorf("parse %s: %w", path, err)
	}

	return cfg, nil
}

// ValidateRun checks for structural errors in the run config and returns
// warnings for ambiguous-but-not-fatal cases. A non-empty error slice means
// the config should be rejected.
func ValidateRun(cfg domain.RunConfig) (warnings []string, errs []string) {
	jobNames := map[string]bool{}
	for _, j := range cfg.Jobs {
		if j.Name == "" {
			errs = append(errs, "job with empty name")
			continue
		}
		if jobNames[j.Name] {
			warnings = append(warnings, fmt.Sprintf("duplicate job name %q — only the first definition is used", j.Name))
		}
		jobNames[j.Name] = true

		if j.Cmd == "" {
			errs = append(errs, fmt.Sprintf("job %q: cmd is required", j.Name))
		}

		switch j.Kind {
		case domain.JobKindService:
			// ok — stop is optional (presence ⇒ detached)
		case domain.JobKindTask:
			if j.Stop != "" {
				errs = append(errs, fmt.Sprintf("job %q: tasks cannot declare a stop command", j.Name))
			}
		case "":
			errs = append(errs, fmt.Sprintf("job %q: kind is required (service or task)", j.Name))
		default:
			errs = append(errs, fmt.Sprintf("job %q: unknown kind %q (expected service or task)", j.Name, j.Kind))
		}
	}

	seenProfiles := map[string]bool{}
	defaultCount := 0
	for _, p := range cfg.Profiles {
		if p.Name == "" {
			errs = append(errs, "profile with empty name")
			continue
		}
		if seenProfiles[p.Name] {
			warnings = append(warnings, fmt.Sprintf("duplicate profile name %q — only the first definition is used", p.Name))
		}
		seenProfiles[p.Name] = true

		for _, ref := range p.Jobs {
			if !jobNames[ref] {
				errs = append(errs, fmt.Sprintf("profile %q references unknown job %q", p.Name, ref))
			}
		}

		if p.Default {
			defaultCount++
		}
	}
	if defaultCount > 1 {
		warnings = append(warnings, fmt.Sprintf("%d profiles marked as default — only the first is used", defaultCount))
	}

	return warnings, errs
}
