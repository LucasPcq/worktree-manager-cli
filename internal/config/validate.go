package config

import "github.com/LucasPcq/wtm/internal/domain"

// Validate checks that all config values are within their allowed sets.
// Returns the first validation error found.
func Validate(cfg domain.Config) error {
	if err := validateEnvStrategy(cfg.Project.Env.Strategy); err != nil {
		return err
	}

	if err := validateAgentType(cfg.Project.Agents.Default); err != nil {
		return err
	}

	if err := validateShellType(cfg.Global.Shell); err != nil {
		return err
	}

	if err := validateAgentType(domain.AgentType(cfg.Global.Agent)); err != nil {
		return err
	}

	return nil
}

func validateEnvStrategy(s domain.EnvStrategy) error {
	switch s {
	case domain.EnvStrategyExample, domain.EnvStrategyMain, domain.EnvStrategyParent:
		return nil
	default:
		return domain.ErrInvalidEnvStrategy
	}
}

func validateShellType(s domain.ShellType) error {
	switch s {
	case domain.ShellZsh, domain.ShellBash, domain.ShellFish:
		return nil
	default:
		return domain.ErrInvalidShellType
	}
}

func validateAgentType(a domain.AgentType) error {
	switch a {
	case domain.AgentClaudeCode, domain.AgentCursor, domain.AgentNone:
		return nil
	default:
		return domain.ErrInvalidAgentType
	}
}
