package rules

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/LucasPcq/wtm/internal/domain"
)

// Validate checks that all config values are within their allowed sets.
func Validate(cfg domain.Config) error {
	if err := ValidateEnvStrategy(cfg.Project.Env.Strategy); err != nil {
		return err
	}
	if err := ValidateEnvFiles(cfg.Project.Env.Files); err != nil {
		return err
	}
	if err := ValidateShellType(cfg.Global.Shell); err != nil {
		return err
	}
	return nil
}

// ValidateEnvFiles rejects entries with an empty target, duplicate targets, or a
// template that is not a recognized template of its target.
func ValidateEnvFiles(files []domain.EnvFile) error {
	seen := make(map[string]bool, len(files))
	for _, f := range files {
		if f.Target == "" {
			return domain.ErrEnvFileNoTarget
		}
		if seen[f.Target] {
			return fmt.Errorf("%w: %s", domain.ErrEnvFileDuplicateTarget, f.Target)
		}
		seen[f.Target] = true

		if f.Template != "" && !isTemplateOf(f.Target, f.Template) {
			return fmt.Errorf("%w: %s is not a template of %s", domain.ErrEnvFileBadTemplate, f.Template, f.Target)
		}
	}
	return nil
}

func isTemplateOf(target, template string) bool {
	for _, c := range TemplateCandidates(target) {
		if c == template {
			return true
		}
	}
	return false
}

// ValidateEnvStrategy returns ErrInvalidEnvStrategy if s is not a known value.
func ValidateEnvStrategy(s domain.EnvStrategy) error {
	switch s {
	case domain.EnvStrategyExample, domain.EnvStrategyMain, domain.EnvStrategyParent:
		return nil
	default:
		return domain.ErrInvalidEnvStrategy
	}
}

// ValidateShellType returns ErrInvalidShellType if s is not a known value.
func ValidateShellType(s domain.ShellType) error {
	switch s {
	case domain.ShellZsh, domain.ShellBash, domain.ShellFish:
		return nil
	default:
		return domain.ErrInvalidShellType
	}
}

// ValidateRelocateTarget checks the --to value for `relocate`. An empty
// string means the flag was not provided (relocate then uses the current
// base_path) and is allowed. A non-empty value must be a repo-relative path:
// whitespace-only and absolute paths are rejected.
func ValidateRelocateTarget(to string) error {
	if to == "" {
		return nil
	}
	if strings.TrimSpace(to) == "" || filepath.IsAbs(to) {
		return domain.ErrInvalidBasePath
	}
	return nil
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
			errs = append(errs, fmt.Sprintf("duplicate job name %q — names must be unique across the file", j.Name))
		}
		jobNames[j.Name] = true

		if j.Cmd == "" {
			errs = append(errs, fmt.Sprintf("job %q: cmd is required", j.Name))
		}

		switch j.Kind {
		case domain.JobKindService:
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
			errs = append(errs, fmt.Sprintf("duplicate profile name %q — names must be unique across the file", p.Name))
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
		errs = append(errs, fmt.Sprintf("%d profiles marked as default — only one profile can be the default", defaultCount))
	}

	return warnings, errs
}
