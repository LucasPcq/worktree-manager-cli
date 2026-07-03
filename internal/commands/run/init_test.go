package run

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/LucasPcq/wtm/internal/config"
	"github.com/LucasPcq/wtm/internal/domain"
)

// TestRunUp_NotInitialized asserts a blocked run command fails with the opt-in
// guard (ErrRunNotInitialized) when run.toml declares no job/profile.
func TestRunUp_NotInitialized(t *testing.T) {
	setupTestProject(t)

	_, _, err := runCmd(t, domain.CmdUp)
	if err == nil {
		t.Fatal("expected error on uninitialized run module")
	}
	if !errors.Is(err, domain.ErrRunNotInitialized) {
		t.Errorf("expected ErrRunNotInitialized, got: %v", err)
	}
}

// TestRunInit_NonInteractiveAutoGenerates verifies `wtm run init --non-interactive`
// turns a detected docker-compose file into a run.toml job without prompting.
func TestRunInit_NonInteractiveAutoGenerates(t *testing.T) {
	stateDir := setupTestProject(t)
	projectDir := os.Getenv("WTM_PROJECT_DIR")
	if err := os.WriteFile(filepath.Join(projectDir, "docker-compose.yml"), []byte("services: {}\n"), 0o644); err != nil {
		t.Fatalf("write docker-compose: %v", err)
	}

	if _, _, err := runCmd(t, domain.CmdInit, "--"+domain.FlagNonInteractive); err != nil {
		t.Fatalf("run init: %v", err)
	}

	cfg, err := config.LoadRun(stateDir)
	if err != nil {
		t.Fatalf("load run: %v", err)
	}
	if len(cfg.Jobs) != 1 || cfg.Jobs[0].Kind != domain.JobKindService {
		t.Errorf("expected one docker service job, got %+v", cfg.Jobs)
	}
}

// TestRunInit_ReRunMergesAndPreservesProfiles verifies re-running init is
// additive: existing jobs/profiles are kept and detected jobs merge in without
// duplication.
func TestRunInit_ReRunMergesAndPreservesProfiles(t *testing.T) {
	stateDir := setupTestProject(t)
	writeRunTOML(t, stateDir, domain.RunConfig{
		Jobs: []domain.JobConfig{
			{Name: "api", Kind: domain.JobKindService, Cmd: "echo hi"},
		},
		Profiles: []domain.ProfileConfig{
			{Name: "dev", Jobs: []string{"api"}, Default: true},
		},
	})

	projectDir := os.Getenv("WTM_PROJECT_DIR")
	if err := os.WriteFile(filepath.Join(projectDir, "docker-compose.yml"), []byte("services: {}\n"), 0o644); err != nil {
		t.Fatalf("write docker-compose: %v", err)
	}

	if _, _, err := runCmd(t, domain.CmdInit, "--"+domain.FlagNonInteractive); err != nil {
		t.Fatalf("run init: %v", err)
	}

	cfg, err := config.LoadRun(stateDir)
	if err != nil {
		t.Fatalf("load run: %v", err)
	}

	// The pre-existing job survives and the detected docker job is added.
	if len(cfg.Jobs) != 2 {
		t.Errorf("expected 2 jobs after merge, got %+v", cfg.Jobs)
	}
	found := false
	for _, j := range cfg.Jobs {
		if j.Name == "api" {
			found = true
		}
	}
	if !found {
		t.Error("existing job 'api' was dropped by re-run")
	}
	// The profile is untouched.
	if len(cfg.Profiles) != 1 || cfg.Profiles[0].Name != "dev" || !cfg.Profiles[0].Default {
		t.Errorf("profile not preserved: %+v", cfg.Profiles)
	}
}
