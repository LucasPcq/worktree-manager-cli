// Package worktree implements git worktree operations (create, list, remove, detect parent).
package worktree

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/infra"
	"github.com/LucasPcq/wtm/internal/rules"
	"github.com/LucasPcq/wtm/internal/service/env"
	"github.com/LucasPcq/wtm/internal/service/hooks"
)

// Create orchestrates worktree creation: git worktree add, env copy, metadata, hooks.
func Create(params domain.CreateParams) (domain.CreateResult, error) {
	sanitized := rules.SanitizeBranchName(params.Branch)
	worktreePath := filepath.Join(params.ProjectDir, params.Config.Project.Worktrees.BasePath, sanitized)

	if _, err := os.Stat(worktreePath); err == nil {
		// Idempotent path: with --if-not-exists, an existing worktree is a
		// no-op success so agents can safely retry.
		if params.IfNotExists {
			return domain.CreateResult{
				Branch:        params.Branch,
				Path:          worktreePath,
				AlreadyExists: true,
			}, nil
		}
		return domain.CreateResult{}, fmt.Errorf("%w: %s", domain.ErrWorktreePathExists, worktreePath)
	}

	if err := infra.CreateWorktree(infra.CreateWorktreeParams{
		ProjectDir: params.ProjectDir,
		Path:       worktreePath,
		Branch:     params.Branch,
		FromBranch: params.FromBranch,
	}); err != nil {
		return domain.CreateResult{}, err
	}

	strategy := rules.ResolveEnvStrategy(params.Config.Project.Env.Strategy, params.EnvFromOverride)

	mainPath, err := infra.FindMainWorktreePath(infra.FindMainWorktreeParams{
		ProjectDir: params.ProjectDir,
	})
	if err != nil {
		return domain.CreateResult{}, fmt.Errorf("find main worktree: %w", err)
	}

	sourceBranch := params.SourceBranch
	if sourceBranch == "" {
		sourceBranch = params.FromBranch
	}

	if len(params.Config.Project.Env.CopyFiles) > 0 {
		copyErr := env.CopyEnvFiles(env.CopyEnvFilesParams{
			Strategy:           strategy,
			CopyFiles:          params.Config.Project.Env.CopyFiles,
			TargetDir:          worktreePath,
			MainWorktreePath:   mainPath,
			ParentWorktreePath: parentWorktreePath(params.ProjectDir, sourceBranch),
		})
		if copyErr != nil {
			return domain.CreateResult{}, fmt.Errorf("copy env files: %w", copyErr)
		}
	}

	metadata := domain.WorktreeMetadata{
		SourceBranch: sourceBranch,
		CreatedAt:    time.Now().UTC().Format(time.RFC3339),
		EnvStrategy:  strategy,
	}

	metaDir := rules.WorktreeMetaDir(params.StateDir, params.Branch)
	if err := writeMetadata(metaDir, metadata); err != nil {
		return domain.CreateResult{}, err
	}

	if len(params.Config.Project.Hooks.OnCreate) > 0 {
		hookErr := hooks.RunHooks(hooks.RunHooksParams{
			Hooks:   params.Config.Project.Hooks.OnCreate,
			WorkDir: worktreePath,
			Vars: rules.TemplateVars{
				Worktree:   worktreePath,
				Branch:     params.Branch,
				Root:       mainPath,
				FromBranch: params.FromBranch,
			},
		})
		if hookErr != nil {
			return domain.CreateResult{}, fmt.Errorf("on_create hooks: %w", hookErr)
		}
	}

	return domain.CreateResult{
		Branch:   params.Branch,
		Path:     worktreePath,
		Metadata: metadata,
	}, nil
}

// parentWorktreePath resolves the on-disk worktree of the parent branch, used by
// the env "parent" strategy to copy .env from the worktree the new one was
// branched off. Returns "" when the parent has no local worktree (e.g. a remote
// start-point like origin/x), letting env provisioning fall back to the main
// worktree instead of copying from the wrong directory.
func parentWorktreePath(projectDir, parentBranch string) string {
	wt, err := infra.FindWorktreeByBranch(infra.FindWorktreeByBranchParams{
		ProjectDir: projectDir,
		Branch:     parentBranch,
	})
	if err != nil {
		return ""
	}
	return wt.Path
}

func writeMetadata(metaDir string, metadata domain.WorktreeMetadata) error {
	if err := os.MkdirAll(metaDir, 0o755); err != nil {
		return fmt.Errorf("create %s: %w", metaDir, err)
	}

	data, err := json.MarshalIndent(metadata, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal metadata: %w", err)
	}

	metaPath := filepath.Join(metaDir, domain.MetaFileName)
	if err := os.WriteFile(metaPath, data, 0o644); err != nil {
		return fmt.Errorf("write %s: %w", metaPath, err)
	}

	return nil
}
