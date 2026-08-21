package worktree

import (
	"fmt"
	"strings"

	"github.com/LucasPcq/wtm/internal/config"
	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/rules"
	envsvc "github.com/LucasPcq/wtm/internal/service/env"
)

type ResolveEnvPortsParams struct {
	ProjectDir   string
	StateDir     string
	Branch       string
	WorktreePath string
	// EnvFiles are the value targets .wtm.toml configures. A link may only name
	// one of them, and this is the only place both files are in hand — LoadRun
	// validates what run.toml can answer for alone and never sees .wtm.toml.
	EnvFiles []domain.EnvFile
}

// ResolveEnvPorts gathers what a worktree needs to reconcile the ports written
// into its .env files: the links and bases run.toml declares, and the offset its
// ordinal binds on. A project declaring no link resolves to zero links, which
// every caller treats as nothing to do.
func ResolveEnvPorts(params ResolveEnvPortsParams) (envsvc.EnvPortsParams, error) {
	cfg, err := config.LoadRun(params.StateDir)
	if err != nil {
		return envsvc.EnvPortsParams{}, err
	}
	if len(cfg.EnvPorts) == 0 {
		return envsvc.EnvPortsParams{}, nil
	}

	if errs := rules.ValidateEnvPortTargets(cfg.EnvPorts, params.EnvFiles); len(errs) > 0 {
		return envsvc.EnvPortsParams{}, fmt.Errorf("invalid run config: %s", strings.Join(errs, "; "))
	}

	ordinal, err := EnsureOrdinal(WorktreeRef{
		ProjectDir: params.ProjectDir,
		StateDir:   params.StateDir,
		Branch:     params.Branch,
	})
	if err != nil {
		return envsvc.EnvPortsParams{}, err
	}

	block := rules.EffectivePortOffsetBlock(cfg)
	return envsvc.EnvPortsParams{
		WorktreePath: params.WorktreePath,
		Links:        cfg.EnvPorts,
		Bases:        rules.EnvPortBases(cfg),
		Offset:       ordinal * block,
		Block:        block,
	}, nil
}
