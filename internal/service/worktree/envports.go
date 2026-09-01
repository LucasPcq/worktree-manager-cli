package worktree

import (
	"fmt"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/LucasPcq/wtm/internal/config"
	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/rules"
	envsvc "github.com/LucasPcq/wtm/internal/service/env"
	"github.com/LucasPcq/wtm/internal/service/process"
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
	// Global carries the machine's [proxy] table. The project says whether it
	// wants addresses; this says whether the machine can serve them.
	Global domain.GlobalConfig
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

	// BranchEnv rather than EnsureOrdinal: it settles the offset and the worktree
	// label in one place, so a .env and the route a job answers under can never
	// disagree on which worktree they belong to.
	env, err := BranchEnv(WorktreeRef{
		ProjectDir: params.ProjectDir,
		StateDir:   params.StateDir,
		Branch:     params.Branch,
	})
	if err != nil {
		return envsvc.EnvPortsParams{}, err
	}
	offset, err := strconv.Atoi(env[domain.EnvPortOffset])
	if err != nil {
		return envsvc.EnvPortsParams{}, fmt.Errorf("resolve port offset: %w", err)
	}

	return envsvc.EnvPortsParams{
		WorktreePath: params.WorktreePath,
		Links:        cfg.EnvPorts,
		Bases:        rules.EnvPortBases(cfg),
		Offset:       offset,
		Block:        rules.EffectivePortOffsetBlock(cfg),
		Origins: rules.OriginContext{
			Addressing: rules.EffectiveAddressing(cfg),
			Jobs:       jobsByName(cfg),
			Worktree:   env[domain.EnvWorktree],
			Project:    filepath.Base(params.ProjectDir),
			PublicPort: process.PublicProxyPort(rules.ProxyPort(params.Global)),
		},
	}, nil
}

func jobsByName(cfg domain.RunConfig) map[string]domain.JobConfig {
	byName := make(map[string]domain.JobConfig, len(cfg.Jobs))
	for _, job := range cfg.Jobs {
		byName[job.Name] = job
	}
	return byName
}
