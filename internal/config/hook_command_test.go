package config

import (
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
)

func TestUnmarshalHookCommand_PlainString(t *testing.T) {
	got, err := unmarshalHookCommand("echo hello")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	want := domain.HookCommand{Cmd: "echo hello", Cwd: "", ContinueOnError: false}
	if got != want {
		t.Errorf("got %+v, want %+v", got, want)
	}
}

func TestUnmarshalHookCommand_TableWithCmdAndCwd(t *testing.T) {
	raw := map[string]interface{}{
		"cmd": "npm install",
		"cwd": "/tmp/project",
	}

	got, err := unmarshalHookCommand(raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	want := domain.HookCommand{Cmd: "npm install", Cwd: "/tmp/project", ContinueOnError: false}
	if got != want {
		t.Errorf("got %+v, want %+v", got, want)
	}
}

func TestUnmarshalHookCommand_TableWithContinueOnError(t *testing.T) {
	raw := map[string]interface{}{
		"cmd":               "make lint",
		"cwd":               ".",
		"continue_on_error": true,
	}

	got, err := unmarshalHookCommand(raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	want := domain.HookCommand{Cmd: "make lint", Cwd: ".", ContinueOnError: true}
	if got != want {
		t.Errorf("got %+v, want %+v", got, want)
	}
}

func TestUnmarshalHookCommand_InvalidType(t *testing.T) {
	_, err := unmarshalHookCommand(42)
	if err == nil {
		t.Fatal("expected error for invalid type, got nil")
	}
}

func TestUnmarshalHookCommand_TableMissingCmd(t *testing.T) {
	raw := map[string]interface{}{
		"cwd": "/tmp",
	}

	_, err := unmarshalHookCommand(raw)
	if err == nil {
		t.Fatal("expected error for missing cmd field, got nil")
	}
}

func TestUnmarshalHookCommands_MixedArray(t *testing.T) {
	raw := []interface{}{
		"echo first",
		map[string]interface{}{
			"cmd": "echo second",
			"cwd": "/tmp",
		},
	}

	got, err := unmarshalHookCommands(raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(got) != 2 {
		t.Fatalf("expected 2 hooks, got %d", len(got))
	}

	wantFirst := domain.HookCommand{Cmd: "echo first"}
	if got[0] != wantFirst {
		t.Errorf("hook[0]: got %+v, want %+v", got[0], wantFirst)
	}

	wantSecond := domain.HookCommand{Cmd: "echo second", Cwd: "/tmp"}
	if got[1] != wantSecond {
		t.Errorf("hook[1]: got %+v, want %+v", got[1], wantSecond)
	}
}

func TestUnmarshalHookCommands_EmptyArray(t *testing.T) {
	got, err := unmarshalHookCommands([]interface{}{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(got) != 0 {
		t.Errorf("expected empty slice, got %d elements", len(got))
	}
}

func TestDecodeHooksConfig_AllThreeHookTypes(t *testing.T) {
	raw := rawHooksConfig{
		OnCreate: []interface{}{
			"npm install",
			map[string]interface{}{"cmd": "cp .env.example .env", "cwd": "."},
		},
		OnFocus: []interface{}{
			"echo focused",
		},
		OnBlur: []interface{}{
			map[string]interface{}{"cmd": "cleanup", "continue_on_error": true},
		},
	}

	got, err := decodeHooksConfig(raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(got.OnCreate) != 2 {
		t.Errorf("OnCreate: expected 2 hooks, got %d", len(got.OnCreate))
	}
	if len(got.OnFocus) != 1 {
		t.Errorf("OnFocus: expected 1 hook, got %d", len(got.OnFocus))
	}
	if len(got.OnBlur) != 1 {
		t.Errorf("OnBlur: expected 1 hook, got %d", len(got.OnBlur))
	}

	if got.OnCreate[0].Cmd != "npm install" {
		t.Errorf("OnCreate[0].Cmd = %q, want %q", got.OnCreate[0].Cmd, "npm install")
	}
	if got.OnCreate[1].Cwd != "." {
		t.Errorf("OnCreate[1].Cwd = %q, want %q", got.OnCreate[1].Cwd, ".")
	}
	if got.OnFocus[0].Cmd != "echo focused" {
		t.Errorf("OnFocus[0].Cmd = %q, want %q", got.OnFocus[0].Cmd, "echo focused")
	}
	if !got.OnBlur[0].ContinueOnError {
		t.Error("OnBlur[0].ContinueOnError = false, want true")
	}
}
