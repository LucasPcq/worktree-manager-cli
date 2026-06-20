package initcmd

import (
	"strings"
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
)

func TestParseSections(t *testing.T) {
	tests := []struct {
		name    string
		input   []string
		want    []string
		wantErr bool
	}{
		{name: "single", input: []string{"services"}, want: []string{"services"}},
		{name: "csv", input: []string{"env,services"}, want: []string{"env", "services"}},
		{name: "repeated", input: []string{"env", "hooks"}, want: []string{"env", "hooks"}},
		{name: "dedup", input: []string{"env", "env"}, want: []string{"env"}},
		{name: "trim", input: []string{" env , services "}, want: []string{"env", "services"}},
		{name: "unknown", input: []string{"nope"}, wantErr: true},
		{name: "mixed valid+invalid", input: []string{"env,bogus"}, wantErr: true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := parseSections(tc.input)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error for %v", tc.input)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if strings.Join(got, ",") != strings.Join(tc.want, ",") {
				t.Errorf("got %v, want %v", got, tc.want)
			}
		})
	}
}

func TestReinitHooksFromAnswers(t *testing.T) {
	answers := domain.InitProjectAnswers{
		InstallCommand: "pnpm install",
		OnCreateExtra:  []domain.HookCommand{{Cmd: "pnpm install", Cwd: "packages/api"}},
	}
	got := reinitHooks(answers)
	if len(got) != 2 {
		t.Fatalf("expected 2 hooks, got %d", len(got))
	}
	if got[0].Cmd != "pnpm install" || got[0].Cwd != "" {
		t.Errorf("first hook should be the bare install command, got %+v", got[0])
	}
	if got[1].Cwd != "packages/api" {
		t.Errorf("second hook should carry the workspace cwd, got %+v", got[1])
	}
}

func TestReinitHooksEmpty(t *testing.T) {
	if got := reinitHooks(domain.InitProjectAnswers{}); len(got) != 0 {
		t.Errorf("expected no hooks, got %v", got)
	}
}
