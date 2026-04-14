package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
)

func TestLoadRunRejectsTypoedSection(t *testing.T) {
	dir := t.TempDir()
	wtmDir := filepath.Join(dir, domain.ProjectDirName)
	if err := os.MkdirAll(wtmDir, 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(wtmDir, domain.RunFileName)
	body := `
[[job]]
name = "dev"
kind = "service"
cmd = "pnpm dev"

[[profiles]]
name = "full"
jobs = ["dev"]
`
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := LoadRun(dir)
	if err == nil {
		t.Fatal("expected error for typoed [[profiles]] section")
	}
	if !strings.Contains(err.Error(), "profiles") {
		t.Errorf("expected error to mention 'profiles', got: %v", err)
	}
}

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

func TestBuildDockerJobsKind(t *testing.T) {
	cfg := BuildDockerJobs("docker compose", []string{"docker-compose.yml"})
	if len(cfg.Jobs) != 1 {
		t.Fatalf("expected 1 job, got %d", len(cfg.Jobs))
	}
	j := cfg.Jobs[0]
	if j.Kind != domain.JobKindService {
		t.Errorf("expected Kind=service, got %q", j.Kind)
	}
	if j.Stop == "" {
		t.Error("expected stop command on docker job")
	}
	if !j.IsDetached() {
		t.Error("expected docker job to be detached")
	}
}
