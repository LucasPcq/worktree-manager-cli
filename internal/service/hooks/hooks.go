// Package hooks implements the execution engine for on_create hooks.
package hooks

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"sort"
	"strings"
	"time"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/rules"
)

// RunHooksParams holds inputs for executing lifecycle hooks.
type RunHooksParams struct {
	Hooks   []domain.HookCommand
	WorkDir string
	Vars    rules.TemplateVars
	// Env is what the hook learns about the worktree it runs in, the same
	// vocabulary a run job gets. A hook that tears down docker needs the
	// worktree's compose project as much as the job that brought it up.
	Env    map[string]string
	Output io.Writer // if nil, uses os.Stdout/Stderr (CLI mode). Set to capture output (TUI mode).
}

// RunHooks executes each hook command sequentially with template interpolation.
// Stops on first error unless the hook has ContinueOnError set.
func RunHooks(params RunHooksParams) error {
	output := params.Output
	if output == nil {
		output = os.Stderr
	}

	for _, hook := range params.Hooks {
		resolved := rules.ResolveTemplateVars(hook, params.Vars)
		err := runSingleHook(runSingleHookParams{
			Hook:       resolved,
			DefaultDir: params.WorkDir,
			Env:        params.Env,
			Output:     output,
		})
		if err == nil {
			continue
		}
		if resolved.ContinueOnError {
			continue
		}
		return err
	}

	return nil
}

type runSingleHookParams struct {
	Hook       domain.HookCommand
	DefaultDir string
	Env        map[string]string
	Output     io.Writer
}

func runSingleHook(params runSingleHookParams) error {
	hook := params.Hook
	output := params.Output

	parts := strings.Fields(hook.Cmd)
	if len(parts) == 0 {
		return nil
	}

	cmd := exec.Command(parts[0], parts[1:]...)

	if hook.Cwd != "" {
		cmd.Dir = hook.Cwd
	} else {
		cmd.Dir = params.DefaultDir
	}
	cmd.Env = hookEnv(params.Env)

	var stderr bytes.Buffer
	cmd.Stdout = output
	cmd.Stderr = &stderr

	// A hook prints in three visually separated beats — the command being run, its
	// live output, then the result line — so a streamed install is easy to read.
	fmt.Fprintln(output)
	fmt.Fprintf(output, "  → %s", hook.Cmd)
	if hook.Cwd != "" {
		fmt.Fprintf(output, " (cwd: %s)", hook.Cwd)
	}
	fmt.Fprintln(output)
	fmt.Fprintln(output)

	start := time.Now()
	err := cmd.Run()
	elapsed := time.Since(start)

	fmt.Fprintln(output)

	if err != nil {
		fmt.Fprintf(output, "  ✗ %s (%s)\n", hook.Cmd, formatDuration(elapsed))
		stderrStr := strings.TrimSpace(stderr.String())
		if stderrStr != "" {
			fmt.Fprintf(output, "    %s\n", stderrStr)
		}
		return fmt.Errorf("hook %q failed: %w", hook.Cmd, err)
	}

	fmt.Fprintf(output, "  ✓ %s (%s)\n", hook.Cmd, formatDuration(elapsed))
	return nil
}

func formatDuration(d time.Duration) string {
	if d < time.Second {
		return fmt.Sprintf("%dms", d.Milliseconds())
	}
	return fmt.Sprintf("%.1fs", d.Seconds())
}

// hookEnv layers the worktree's variables over the caller's environment. Nil
// leaves the process environment as it is, so a caller with nothing to say
// changes nothing.
func hookEnv(overrides map[string]string) []string {
	if len(overrides) == 0 {
		return nil
	}

	keys := make([]string, 0, len(overrides))
	for key := range overrides {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	env := os.Environ()
	for _, key := range keys {
		env = append(env, key+"="+overrides[key])
	}
	return env
}
