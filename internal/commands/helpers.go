package commands

import (
	"errors"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/LucasPcq/wtm/internal/config"
	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/infra"
)

// configResult holds the loaded config and the resolved project root.
type configResult struct {
	Config     domain.Config
	ProjectDir string
}

// projectRoot returns the main worktree path (where .wtm/config.toml lives).
// Works from any worktree — resolves back to the parent repo.
func projectRoot(dir string) (string, error) {
	mainPath, err := infra.FindMainWorktreePath(infra.FindMainWorktreeParams{
		ProjectDir: dir,
	})
	if err != nil {
		return "", fmt.Errorf("find project root: %w", err)
	}
	return mainPath, nil
}

// loadConfig resolves the project root and loads .wtm/config.toml from there.
// Returns the config and root path, or prints an error and returns false.
func loadConfig(cmd *cobra.Command, dir string) (configResult, bool) {
	root, err := projectRoot(dir)
	if err != nil {
		fmt.Fprintf(cmd.ErrOrStderr(), "Error: %v\n", err)
		return configResult{}, false
	}

	cfg, err := config.Load(config.LoadParams{ProjectDir: root})
	if errors.Is(err, domain.ErrConfigNotFound) {
		fmt.Fprintln(cmd.ErrOrStderr(), "No .wtm/config.toml found. Run `wtm init` first.")
		return configResult{}, false
	}
	if err != nil {
		fmt.Fprintf(cmd.ErrOrStderr(), "Error loading config: %v\n", err)
		return configResult{}, false
	}

	return configResult{Config: cfg, ProjectDir: root}, true
}
