package rules

import (
	"fmt"

	"github.com/LucasPcq/wtm/internal/domain"
)

type ConcurrencyParams struct {
	// Exclusive and Parallel are the flags, which override the config for one run.
	Exclusive bool
	Parallel  bool
	Config    domain.Concurrency
	// OthersRunning says another worktree has jobs up. Without one there is
	// nothing to stop, so there is nothing to decide either.
	OthersRunning bool
}

// ConcurrencyDecision is what a run does about the other worktrees, and whether
// anybody still has to be asked. Value is always usable: Ask only says that a
// fully interactive run may offer the choice, never that it must.
type ConcurrencyDecision struct {
	Value domain.Concurrency
	Ask   bool
}

// DecideConcurrency resolves the flags, then the config, then falls back to
// leaving the other worktrees alone. Parallel is the safe default in the sense
// the bypass model means: it is the answer that stops nothing.
func DecideConcurrency(params ConcurrencyParams) ConcurrencyDecision {
	if !params.OthersRunning {
		return ConcurrencyDecision{Value: domain.ConcurrencyParallel}
	}
	if params.Exclusive {
		return ConcurrencyDecision{Value: domain.ConcurrencyExclusive}
	}
	if params.Parallel {
		return ConcurrencyDecision{Value: domain.ConcurrencyParallel}
	}
	if params.Config == domain.ConcurrencyExclusive || params.Config == domain.ConcurrencyParallel {
		return ConcurrencyDecision{Value: params.Config}
	}
	return ConcurrencyDecision{Value: domain.ConcurrencyParallel, Ask: true}
}

// ValidateConcurrency refuses a value that is neither of the two. A typo would
// otherwise read as "not answered yet" and bring the question back for good.
func ValidateConcurrency(cfg domain.RunConfig) []string {
	switch cfg.Concurrency {
	case "", domain.ConcurrencyParallel, domain.ConcurrencyExclusive:
		return nil
	default:
		return []string{fmt.Sprintf("unknown concurrency %q (expected %q or %q)",
			cfg.Concurrency, domain.ConcurrencyParallel, domain.ConcurrencyExclusive)}
	}
}
