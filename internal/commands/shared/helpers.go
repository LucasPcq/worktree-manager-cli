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

// LoadConfig resolves the main worktree + state dir and loads config.toml
// from the state dir. Returns the config and resolved paths, or prints an
// error and returns false.
func LoadConfig(cmd *cobra.Command, dir string) (ConfigResult, bool) {
	root, err := ProjectRoot(dir)
	if err != nil {
		output.Error(cmd.ErrOrStderr(), err.Error())
		return ConfigResult{}, false
	}

	stateDir, err := StateDir(dir)
	if err != nil {
		output.Error(cmd.ErrOrStderr(), err.Error())
		return ConfigResult{}, false
	}

	cfg, err := config.Load(config.LoadParams{StateDir: stateDir})
	if errors.Is(err, domain.ErrConfigNotFound) {
		output.Warning(cmd.ErrOrStderr(), "No wtm config found. Run `wtm init` first.")
		return ConfigResult{}, false
	}
	if err != nil {
		output.Error(cmd.ErrOrStderr(), fmt.Sprintf("Loading config: %v", err))
		return ConfigResult{}, false
	}

	return ConfigResult{Config: cfg, ProjectDir: root, StateDir: stateDir}, true
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
