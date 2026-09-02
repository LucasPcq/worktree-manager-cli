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

// LoadConfig resolves the main worktree + state dir and loads config.toml from
// the state dir. On failure it returns an error for the caller to propagate so
// the top-level handler can pick the right exit code (e.g. ExitCodeConfigNotFound
// when the repo is uninitialized); it does not print anything itself.
func LoadConfig(cmd *cobra.Command, dir string) (ConfigResult, error) {
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

// AddOutputFlag registers the standard --output flag on cmd.
func AddOutputFlag(cmd *cobra.Command) {
	cmd.Flags().String(domain.FlagOutput, domain.OutputText, "Output format: text or json")
}

// AddJobFlag and AddProfileFlag register the run module's second axis. The
// worktree is the positional subject there, as everywhere else in the CLI, so
// the job or profile is named by a flag the way --to and --from are.
func AddJobFlag(cmd *cobra.Command, usage string) {
	cmd.Flags().String(domain.FlagJob, "", usage)
}

func AddProfileFlag(cmd *cobra.Command, usage string) {
	cmd.Flags().String(domain.FlagProfile, "", usage)
}

// AddYesFlag adds the confirmation axis. It is the only thing that turns prompts
// off; --force, where a command has one, is the safety axis and implies nothing
// here.
func AddYesFlag(cmd *cobra.Command, usage string) {
	cmd.Flags().BoolP(domain.FlagYes, "y", false, usage)
}

// Unattended folds --yes into the prompt-capability gate: a human format, on a
// terminal, and not bypassed.
type UnattendedParams struct {
	TTY    bool
	Format string
	Yes    bool
}

func Interactive(params UnattendedParams) bool {
	return params.TTY && rules.IsHumanFormat(params.Format) && !params.Yes
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
