// Package hooks implements the execution engine for on_create, on_focus, and on_blur hooks.
package hooks

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/LucasPcq/wtm/internal/domain"
)

// RunHooksParams holds inputs for executing lifecycle hooks.
type RunHooksParams struct {
	Hooks   []domain.HookCommand
	WorkDir string
}

// RunHooks executes each hook command sequentially.
// Stdout and stderr are forwarded to the parent process.
// Returns immediately on the first error.
func RunHooks(params RunHooksParams) error {
	for _, hook := range params.Hooks {
		if err := runSingleHook(hook, params.WorkDir); err != nil {
			return err
		}
	}
	return nil
}

func runSingleHook(hook domain.HookCommand, defaultDir string) error {
	parts := strings.Fields(hook.Cmd)
	if len(parts) == 0 {
		return nil
	}

	cmd := exec.Command(parts[0], parts[1:]...)

	if hook.Cwd != "" {
		cmd.Dir = hook.Cwd
	} else {
		cmd.Dir = defaultDir
	}

	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	fmt.Fprintf(os.Stderr, "→ %s\n", hook.Cmd)

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("hook %q failed: %w", hook.Cmd, err)
	}

	return nil
}
