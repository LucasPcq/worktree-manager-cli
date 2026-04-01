// Package worktree implements git worktree operations (create, list, remove, detect parent).
package worktree

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/infra"
	"github.com/LucasPcq/wtm/internal/service/env"
	"github.com/LucasPcq/wtm/internal/service/hooks"
)

// CreateParams holds all inputs needed to create a new worktree.
type CreateParams struct {
	ProjectDir      string
	Branch          string
	FromBranch      string
	Config          domain.Config
	EnvFromOverride string
}

// CreateResult holds the output of a successful worktree creation.
type CreateResult struct {
	Path     string
	Metadata domain.WorktreeMetadata
}

// Create orchestrates worktree creation: git worktree add, env copy, metadata, hooks.
func Create(params CreateParams) (CreateResult, error) {
	sanitized := sanitizeBranchName(params.Branch)
	wtPath := filepath.Join(params.ProjectDir, params.Config.Project.Worktrees.BasePath, sanitized)

	if _, err := os.Stat(wtPath); err == nil {
		return CreateResult{}, fmt.Errorf("%w: %s", domain.ErrWorktreePathExists, wtPath)
	}

	if err := infra.CreateWorktree(infra.CreateWorktreeParams{
		ProjectDir: params.ProjectDir,
		Path:       wtPath,
		Branch:     params.Branch,
		FromBranch: params.FromBranch,
	}); err != nil {
		return CreateResult{}, err
	}

	strategy := resolveEnvStrategy(params)

	mainPath, err := infra.FindMainWorktreePath(infra.FindMainWorktreeParams{
		ProjectDir: params.ProjectDir,
	})
	if err != nil {
		return CreateResult{}, fmt.Errorf("find main worktree: %w", err)
	}

	if len(params.Config.Project.Env.CopyFiles) > 0 {
		if err := env.CopyEnvFiles(env.CopyEnvFilesParams{
			Strategy:           strategy,
			CopyFiles:          params.Config.Project.Env.CopyFiles,
			TargetDir:          wtPath,
			MainWorktreePath:   mainPath,
			ParentWorktreePath: params.ProjectDir,
		}); err != nil {
			return CreateResult{}, fmt.Errorf("copy env files: %w", err)
		}
	}

	metadata := domain.WorktreeMetadata{
		SourceBranch: params.FromBranch,
		CreatedAt:    time.Now().UTC().Format(time.RFC3339),
		EnvStrategy:  strategy,
	}

	if err := writeMetadata(wtPath, metadata); err != nil {
		return CreateResult{}, err
	}

	if len(params.Config.Project.Hooks.OnCreate) > 0 {
		if err := hooks.RunHooks(hooks.RunHooksParams{
			Hooks:   params.Config.Project.Hooks.OnCreate,
			WorkDir: wtPath,
		}); err != nil {
			return CreateResult{}, fmt.Errorf("on_create hooks: %w", err)
		}
	}

	return CreateResult{
		Path:     wtPath,
		Metadata: metadata,
	}, nil
}

// SanitizeBranchName replaces slashes with dashes for use as a directory name.
func sanitizeBranchName(name string) string {
	return strings.ReplaceAll(name, "/", "-")
}

func resolveEnvStrategy(params CreateParams) domain.EnvStrategy {
	if params.EnvFromOverride != "" {
		return domain.EnvStrategy(params.EnvFromOverride)
	}
	return params.Config.Project.Env.Strategy
}

func writeMetadata(wtPath string, metadata domain.WorktreeMetadata) error {
	metaDir := filepath.Join(wtPath, domain.MetaDirName)
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

	contextPath := filepath.Join(metaDir, domain.ContextFileName)
	if err := os.WriteFile(contextPath, nil, 0o644); err != nil {
		return fmt.Errorf("write %s: %w", contextPath, err)
	}

	return nil
}
