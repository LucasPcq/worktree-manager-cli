package rules

import (
	"errors"
	"strings"
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
)

func TestValidateEnvFiles(t *testing.T) {
	cases := []struct {
		name    string
		files   []domain.EnvFile
		wantErr error
	}{
		{"valid pair", []domain.EnvFile{{Target: ".env", Template: ".env.example"}}, nil},
		{"valid local", []domain.EnvFile{{Target: ".env.local", Local: true}}, nil},
		{"empty target", []domain.EnvFile{{Target: ""}}, domain.ErrEnvFileNoTarget},
		{"duplicate target", []domain.EnvFile{{Target: ".env"}, {Target: ".env"}}, domain.ErrEnvFileDuplicateTarget},
		{"bad template", []domain.EnvFile{{Target: ".env", Template: ".env.bogus"}}, domain.ErrEnvFileBadTemplate},
		{"template mismatched target", []domain.EnvFile{{Target: ".env", Template: "apps/.env.example"}}, domain.ErrEnvFileBadTemplate},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateEnvFiles(tc.files)
			if tc.wantErr == nil && err != nil {
				t.Fatalf("expected no error, got %v", err)
			}
			if tc.wantErr != nil && !errors.Is(err, tc.wantErr) {
				t.Fatalf("expected %v, got %v", tc.wantErr, err)
			}
		})
	}
}

func TestValidateSudoDeletePath(t *testing.T) {
	const home = "/home/dev"
	const project = "/home/dev/repo"

	cases := []struct {
		name    string
		path    string
		wantErr bool
	}{
		{"safe worktree under project", "/home/dev/repo/.worktrees/feat", false},
		{"safe sibling worktree", "/home/dev/worktrees/feat", false},
		{"filesystem root", "/", true},
		{"home dir", home, true},
		{"project root", project, true},
		{"ancestor of project", "/home/dev", true},
		{"relative path", "repo/.worktrees/feat", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateSudoDeletePath(SudoDeletePathParams{
				Path:       tc.path,
				HomeDir:    home,
				ProjectDir: project,
			})
			if tc.wantErr && !errors.Is(err, domain.ErrUnsafeSudoDeletePath) {
				t.Fatalf("expected ErrUnsafeSudoDeletePath for %q, got %v", tc.path, err)
			}
			if !tc.wantErr && err != nil {
				t.Fatalf("expected no error for %q, got %v", tc.path, err)
			}
		})
	}
}

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
		},
		Global: domain.GlobalConfig{
			Shell: domain.DefaultShell,
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
