package rules

import (
	"errors"
	"strings"
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
)

func TestValidateRelocateTarget(t *testing.T) {
	cases := []struct {
		name    string
		to      string
		wantErr bool
	}{
		{"empty means not provided", "", false},
		{"relative path", "../.worktrees", false},
		{"nested relative path", "worktrees/sub", false},
		{"whitespace only", "   ", true},
		{"absolute path", "/tmp/worktrees", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateRelocateTarget(tc.to)
			if tc.wantErr && !errors.Is(err, domain.ErrInvalidBasePath) {
				t.Fatalf("expected ErrInvalidBasePath for %q, got %v", tc.to, err)
			}
			if !tc.wantErr && err != nil {
				t.Fatalf("expected no error for %q, got %v", tc.to, err)
			}
		})
	}
}

// --- Validate tests (migrated from config/config_test.go) ---

func TestValidateInvalidEnvStrategy(t *testing.T) {
	cfg := validConfig()
	cfg.Project.Env.Strategy = "invalid"

	err := Validate(cfg)
	if !errors.Is(err, domain.ErrInvalidEnvStrategy) {
		t.Errorf("expected ErrInvalidEnvStrategy, got %v", err)
	}
}

func TestValidateInvalidShellType(t *testing.T) {
	cfg := validConfig()
	cfg.Global.Shell = "powershell"

	err := Validate(cfg)
	if !errors.Is(err, domain.ErrInvalidShellType) {
		t.Errorf("expected ErrInvalidShellType, got %v", err)
	}
}

func TestValidateInvalidAgentType(t *testing.T) {
	cfg := validConfig()
	cfg.Project.Agents.Default = "copilot"

	err := Validate(cfg)
	if !errors.Is(err, domain.ErrInvalidAgentType) {
		t.Errorf("expected ErrInvalidAgentType, got %v", err)
	}
}

func validConfig() domain.Config {
	return domain.Config{
		Project: domain.ProjectConfig{
			Worktrees: domain.WorktreesConfig{
				BasePath:   domain.DefaultBasePath,
				BaseBranch: domain.DefaultBaseBranch,
			},
			Env: domain.EnvConfig{
				Strategy: domain.DefaultEnvStrategy,
			},
			Agents: domain.AgentsConfig{
				Default: domain.DefaultAgent,
			},
		},
		Global: domain.GlobalConfig{
			Shell: domain.DefaultShell,
			Agent: domain.AgentType(domain.DefaultAgent),
		},
	}
}

// --- ValidateRun tests (migrated from config/jobs_test.go) ---

func TestValidateRunOK(t *testing.T) {
	cfg := domain.RunConfig{
		Jobs: []domain.JobConfig{
			{Name: "dev", Kind: domain.JobKindService, Cmd: "pnpm dev"},
			{Name: "migrate", Kind: domain.JobKindTask, Cmd: "pnpm migrate"},
		},
		Profiles: []domain.ProfileConfig{
			{Name: "full", Jobs: []string{"migrate", "dev"}, Default: true},
		},
	}
	warnings, errs := ValidateRun(cfg)
	if len(errs) > 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}
	if len(warnings) > 0 {
		t.Fatalf("unexpected warnings: %v", warnings)
	}
}

func TestValidateRunTaskWithStop(t *testing.T) {
	cfg := domain.RunConfig{
		Jobs: []domain.JobConfig{
			{Name: "migrate", Kind: domain.JobKindTask, Cmd: "pnpm migrate", Stop: "echo nope"},
		},
	}
	_, errs := ValidateRun(cfg)
	if len(errs) == 0 {
		t.Fatal("expected error for task with stop command")
	}
	if !strings.Contains(strings.Join(errs, " "), "tasks cannot declare a stop command") {
		t.Errorf("unexpected error message: %v", errs)
	}
}

func TestValidateRunMissingKind(t *testing.T) {
	cfg := domain.RunConfig{
		Jobs: []domain.JobConfig{
			{Name: "thing", Cmd: "echo hi"},
		},
	}
	_, errs := ValidateRun(cfg)
	if len(errs) == 0 {
		t.Fatal("expected error for missing kind")
	}
}

func TestValidateRunUnknownKind(t *testing.T) {
	cfg := domain.RunConfig{
		Jobs: []domain.JobConfig{
			{Name: "thing", Kind: "wat", Cmd: "echo hi"},
		},
	}
	_, errs := ValidateRun(cfg)
	if len(errs) == 0 {
		t.Fatal("expected error for unknown kind")
	}
}

func TestValidateRunProfileReferencesUnknownJob(t *testing.T) {
	cfg := domain.RunConfig{
		Jobs: []domain.JobConfig{
			{Name: "dev", Kind: domain.JobKindService, Cmd: "pnpm dev"},
		},
		Profiles: []domain.ProfileConfig{
			{Name: "full", Jobs: []string{"dev", "ghost"}},
		},
	}
	_, errs := ValidateRun(cfg)
	if len(errs) == 0 {
		t.Fatal("expected error for unknown job reference")
	}
}

func TestValidateRunDuplicateJob(t *testing.T) {
	cfg := domain.RunConfig{
		Jobs: []domain.JobConfig{
			{Name: "dev", Kind: domain.JobKindService, Cmd: "pnpm dev"},
			{Name: "dev", Kind: domain.JobKindService, Cmd: "echo dup"},
		},
	}
	_, errs := ValidateRun(cfg)
	if len(errs) == 0 {
		t.Fatal("expected error for duplicate job name")
	}
	if !strings.Contains(strings.Join(errs, " "), "duplicate job name") {
		t.Errorf("unexpected error: %v", errs)
	}
}

func TestValidateRunDuplicateProfile(t *testing.T) {
	cfg := domain.RunConfig{
		Jobs: []domain.JobConfig{
			{Name: "dev", Kind: domain.JobKindService, Cmd: "pnpm dev"},
		},
		Profiles: []domain.ProfileConfig{
			{Name: "full", Jobs: []string{"dev"}},
			{Name: "full", Jobs: []string{"dev"}},
		},
	}
	_, errs := ValidateRun(cfg)
	if len(errs) == 0 {
		t.Fatal("expected error for duplicate profile name")
	}
	if !strings.Contains(strings.Join(errs, " "), "duplicate profile name") {
		t.Errorf("unexpected error: %v", errs)
	}
}

func TestValidateRunMultipleDefaults(t *testing.T) {
	cfg := domain.RunConfig{
		Jobs: []domain.JobConfig{
			{Name: "dev", Kind: domain.JobKindService, Cmd: "pnpm dev"},
		},
		Profiles: []domain.ProfileConfig{
			{Name: "a", Jobs: []string{"dev"}, Default: true},
			{Name: "b", Jobs: []string{"dev"}, Default: true},
		},
	}
	_, errs := ValidateRun(cfg)
	if len(errs) == 0 {
		t.Fatal("expected error for multiple default profiles")
	}
	if !strings.Contains(strings.Join(errs, " "), "default") {
		t.Errorf("unexpected error: %v", errs)
	}
}
