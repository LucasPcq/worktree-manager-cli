package shared

import (
	"errors"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/LucasPcq/wtm/internal/config"
	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/infra"
	"github.com/LucasPcq/wtm/internal/rules"
	"github.com/LucasPcq/wtm/internal/service/detect"
)

// ConfigResult holds the loaded config along with the resolved paths every
// command needs: the main worktree (for git ops & BasePath resolution) and
// the state dir (for wtm config / run / metadata files).
type ConfigResult struct {
	Config     domain.Config
	ProjectDir string
	StateDir   string
}

// ProjectRoot returns the main worktree path. Works from any worktree —
// resolves back to the parent repo. WTM_PROJECT_DIR overrides git resolution;
// useful in tests and CI.
func ProjectRoot(dir string) (string, error) {
	if override := os.Getenv("WTM_PROJECT_DIR"); override != "" {
		return override, nil
	}
	mainPath, err := infra.FindMainWorktreePath(infra.FindMainWorktreeParams{
		ProjectDir: dir,
	})
	if err != nil {
		return "", fmt.Errorf("find project root: %w", err)
	}
	return mainPath, nil
}

// LoadConfigDir resolves the main worktree + state dir and loads config.toml from
// the state dir. On failure it returns an error for the caller to propagate so
// the top-level handler can pick the right exit code (e.g. ExitCodeConfigNotFound
// when the repo is uninitialized); it does not print anything itself. This is the
// cobra-free entry point the Factory's Config closure builds on.
func LoadConfigDir(dir string) (ConfigResult, error) {
	root, err := ProjectRoot(dir)
	if err != nil {
		return ConfigResult{}, err
	}

	stateDir, err := StateDir(dir)
	if err != nil {
		return ConfigResult{}, err
	}

	cfg, err := config.Load(config.LoadParams{StateDir: stateDir})
	if errors.Is(err, domain.ErrConfigNotFound) {
		return ConfigResult{}, fmt.Errorf("no wtm config found — run `wtm init` first: %w", domain.ErrConfigNotFound)
	}
	if err != nil {
		return ConfigResult{}, fmt.Errorf("loading config: %w", err)
	}

	return ConfigResult{Config: cfg, ProjectDir: root, StateDir: stateDir}, nil
}

// LoadConfig is the legacy cobra-taking wrapper around LoadConfigDir, kept so the
// existing (pre-Factory) commands compile unchanged. cmd is unused. New Factory-based
// commands should depend on LoadConfigDir (via Factory.Config) instead.
func LoadConfig(cmd *cobra.Command, dir string) (ConfigResult, error) {
	return LoadConfigDir(dir)
}

// ResolveBase resolves the base branch a worktree operation should fall back to,
// in precedence order: an explicit override, then the project config, then git
// auto-detection. The resolution lives here (command-support layer) so the pure
// service layer only ever receives an already-resolved base branch — never the
// "how was this chosen" logic.
func ResolveBase(override string, cfg ConfigResult) string {
	if override != "" {
		return override
	}
	if cfg.Config.Project.Worktrees.BaseBranch != "" {
		return cfg.Config.Project.Worktrees.BaseBranch
	}
	return detect.BaseBranch(cfg.ProjectDir)
}

// AddOutputFlag registers the standard --output flag on cmd.
func AddOutputFlag(cmd *cobra.Command) {
	cmd.Flags().String(domain.FlagOutput, domain.OutputText, "Output format: text or json")
}

// RequireRunInitialized enforces the run-module opt-in guard: the module counts
// as initialized once run.toml declares at least one job or profile. Blocked run
// commands call this after loading run.toml; the creation paths (run init,
// run job/profile add, run import) skip it. On failure it returns
// ErrRunNotInitialized (wrapped, with the experimental notice on a second line)
// so the top-level handler prints the pedagogical message and picks the
// dedicated exit code; it does not print anything itself.
func RequireRunInitialized(cfg domain.RunConfig) error {
	if rules.IsRunInitialized(cfg) {
		return nil
	}
	return fmt.Errorf("%w\n%s", domain.ErrRunNotInitialized, domain.ExperimentalRunNotice)
}

// GuardRunInitialized resolves the state dir, loads run.toml, and enforces the
// run-module guard in a single call — for commands that don't otherwise need the
// loaded config in hand (ps, down, logs).
func GuardRunInitialized(dir string) error {
	stateDir, err := StateDir(dir)
	if err != nil {
		return err
	}
	cfg, err := config.LoadRun(stateDir)
	if err != nil {
		return fmt.Errorf("load run config: %w", err)
	}
	return RequireRunInitialized(cfg)
}
