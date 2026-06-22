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
	if err := ValidateAgentType(cfg.Project.Agents.Default); err != nil {
		return err
	}
	if err := ValidateShellType(cfg.Global.Shell); err != nil {
		return err
	}
	if err := ValidateAgentType(domain.AgentType(cfg.Global.Agent)); err != nil {
		return err
	}
	return nil
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

// ValidateAgentType returns ErrInvalidAgentType if a is not a known value.
func ValidateAgentType(a domain.AgentType) error {
	switch a {
	case domain.AgentClaudeCode, domain.AgentCursor, domain.AgentNone:
		return nil
	default:
		return domain.ErrInvalidAgentType
	}
}

// ValidateRelocateTarget checks the --to value for `wt relocate`. An empty
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
