package config

import (
	"strings"
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
)

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
	warnings, _ := ValidateRun(cfg)
	if len(warnings) == 0 {
		t.Fatal("expected warning for duplicate job name")
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
