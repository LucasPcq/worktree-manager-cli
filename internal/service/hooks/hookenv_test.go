package hooks

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
)

func TestRunHooksInjectsWorktreeEnv(t *testing.T) {
	dir := t.TempDir()
	marker := filepath.Join(dir, "seen")

	// A script rather than an inline command: a hook's cmd is split on
	// whitespace, so a redirection has nowhere to live.
	script := filepath.Join(dir, "hook.sh")
	body := "#!/bin/sh\nprintenv " + domain.EnvComposeProjectName + " > " + marker + "\n"
	if err := os.WriteFile(script, []byte(body), 0o755); err != nil {
		t.Fatalf("write hook script: %v", err)
	}

	var out bytes.Buffer
	if err := RunHooks(RunHooksParams{
		Hooks:   []domain.HookCommand{{Cmd: script}},
		WorkDir: dir,
		Env:     map[string]string{domain.EnvComposeProjectName: "feat-x"},
		Output:  &out,
	}); err != nil {
		t.Fatalf("RunHooks: %v", err)
	}

	got, err := os.ReadFile(marker)
	if err != nil {
		t.Fatalf("hook left no marker: %v", err)
	}
	if strings.TrimSpace(string(got)) != "feat-x" {
		t.Errorf("hook saw %s=%q, want %q", domain.EnvComposeProjectName, strings.TrimSpace(string(got)), "feat-x")
	}
}

// A caller with nothing to say leaves the hook's environment as it was.
func TestRunHooksWithoutEnvKeepsProcessEnvironment(t *testing.T) {
	t.Setenv("WTM_HOOK_PROBE", "inherited")

	dir := t.TempDir()
	marker := filepath.Join(dir, "seen")
	script := filepath.Join(dir, "hook.sh")
	if err := os.WriteFile(script, []byte("#!/bin/sh\nprintenv WTM_HOOK_PROBE > "+marker+"\n"), 0o755); err != nil {
		t.Fatalf("write hook script: %v", err)
	}

	var out bytes.Buffer
	if err := RunHooks(RunHooksParams{
		Hooks:   []domain.HookCommand{{Cmd: script}},
		WorkDir: dir,
		Output:  &out,
	}); err != nil {
		t.Fatalf("RunHooks: %v", err)
	}

	got, err := os.ReadFile(marker)
	if err != nil {
		t.Fatalf("hook left no marker: %v", err)
	}
	if strings.TrimSpace(string(got)) != "inherited" {
		t.Errorf("hook saw %q, want %q", strings.TrimSpace(string(got)), "inherited")
	}
}
