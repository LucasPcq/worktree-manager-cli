package shared

import (
	"errors"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/spf13/cobra"

	"github.com/LucasPcq/wtm/internal/config"
	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/infra"
	"github.com/LucasPcq/wtm/internal/output"
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

// StartSpinner displays a spinner with a message and returns a stop function.
func StartSpinner(w io.Writer, message string) func() {
	fmt.Fprintln(w)
	done := make(chan struct{})
	stopped := make(chan struct{})
	go func() {
		frames := []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}
		i := 0
		for {
			select {
			case <-done:
				clearLen := len(message) + len(output.Indent) + 4
				fmt.Fprintf(w, "\r%-*s\r", clearLen, "")
				close(stopped)
				return
			default:
				fmt.Fprintf(w, "\r%s%s %s", output.Indent, frames[i%len(frames)], message)
				i++
				time.Sleep(80 * time.Millisecond)
			}
		}
	}()
	return func() {
		close(done)
		<-stopped
	}
}

// ShowGHBanner prints a bordered hint when the GitHub CLI is unavailable, so the
// user knows why PR badges and PR-related actions are missing. Callers choose
// the writer: stdout for human-facing listings, stderr for commands whose
// stdout is consumed by tooling (e.g. `wtm resolve`).
func ShowGHBanner(w io.Writer, conn domain.GHConnection) {
	switch conn {
	case domain.GHConnectionNotInstalled:
		output.Callout(w, "GitHub CLI not found", []string{
			"Install it to see PRs linked to your worktrees:",
			"https://cli.github.com",
		})
	case domain.GHConnectionNotAuthenticated:
		output.Callout(w, "GitHub not connected", []string{
			"Connect to see PRs linked to your worktrees:",
			"run `gh auth login`",
		})
	}
}
