// Package hooks implements the execution engine for on_create, on_focus, and on_blur hooks.
package hooks

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/LucasPcq/wtm/internal/domain"
)

// TemplateVars holds the variables available for interpolation in hook commands.
type TemplateVars struct {
	Worktree   string
	Branch     string
	Root       string
	FromBranch string
}

// RunHooksParams holds inputs for executing lifecycle hooks.
type RunHooksParams struct {
	Hooks   []domain.HookCommand
	WorkDir string
	Vars    TemplateVars
	Output  io.Writer // if nil, uses os.Stdout/Stderr (CLI mode). Set to capture output (TUI mode).
}

// RunHooks executes each hook command sequentially with template interpolation.
// Stops on first error unless the hook has ContinueOnError set.
func RunHooks(params RunHooksParams) error {
	output := params.Output
	if output == nil {
		output = os.Stderr
	}

	for _, hook := range params.Hooks {
		resolved := resolveTemplateVars(hook, params.Vars)
		err := runSingleHook(resolved, params.WorkDir, output)
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

func resolveTemplateVars(hook domain.HookCommand, vars TemplateVars) domain.HookCommand {
	hook.Cmd = interpolate(hook.Cmd, vars)
	hook.Cwd = interpolate(hook.Cwd, vars)
	return hook
}

func interpolate(s string, vars TemplateVars) string {
	if s == "" {
		return s
	}
	r := strings.NewReplacer(
		"{{worktree}}", vars.Worktree,
		"{{branch}}", vars.Branch,
		"{{root}}", vars.Root,
		"{{from_branch}}", vars.FromBranch,
	)
	return r.Replace(s)
}

func runSingleHook(hook domain.HookCommand, defaultDir string, output io.Writer) error {
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

	var stderr bytes.Buffer
	cmd.Stdout = output
	cmd.Stderr = &stderr

	fmt.Fprintf(output, "  → %s", hook.Cmd)
	if hook.Cwd != "" {
		fmt.Fprintf(output, " (cwd: %s)", hook.Cwd)
	}
	fmt.Fprintln(output)

	start := time.Now()
	err := cmd.Run()
	elapsed := time.Since(start)

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
