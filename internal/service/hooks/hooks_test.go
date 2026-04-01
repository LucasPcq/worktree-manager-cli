package hooks

import (
	"strings"
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
)

func TestRunHooksSuccess(t *testing.T) {
	err := RunHooks(RunHooksParams{
		Hooks: []domain.HookCommand{
			{Cmd: "echo hello"},
		},
		WorkDir: t.TempDir(),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRunHooksFailure(t *testing.T) {
	err := RunHooks(RunHooksParams{
		Hooks: []domain.HookCommand{
			{Cmd: "false"},
		},
		WorkDir: t.TempDir(),
	})
	if err == nil {
		t.Fatal("expected error from failing command")
	}
	if !strings.Contains(err.Error(), "false") {
		t.Errorf("error should mention the command: %v", err)
	}
}

func TestRunHooksStopsOnFirstError(t *testing.T) {
	err := RunHooks(RunHooksParams{
		Hooks: []domain.HookCommand{
			{Cmd: "false"},
			{Cmd: "echo should-not-run"},
		},
		WorkDir: t.TempDir(),
	})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestRunHooksEmptyList(t *testing.T) {
	err := RunHooks(RunHooksParams{
		Hooks:   nil,
		WorkDir: t.TempDir(),
	})
	if err != nil {
		t.Fatalf("empty hooks should be no-op: %v", err)
	}
}

func TestRunHooksCwdOverride(t *testing.T) {
	dir := t.TempDir()

	err := RunHooks(RunHooksParams{
		Hooks: []domain.HookCommand{
			{Cmd: "pwd", Cwd: dir},
		},
		WorkDir: "/tmp",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
