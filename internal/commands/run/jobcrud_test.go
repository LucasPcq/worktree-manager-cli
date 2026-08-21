package run

import (
	"encoding/json"
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/LucasPcq/wtm/internal/config"
	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/output"
)

func TestRunJobAdd_OK(t *testing.T) {
	stateDir := setupTestProject(t)

	stdout, _, err := runCmd(t,
		domain.CmdJob, domain.CmdAdd, "api",
		"--"+domain.FlagCmd, "go run ./cmd/api",
		"--"+domain.FlagKind, string(domain.JobKindService),
		"--"+domain.FlagOutput, domain.OutputJSON,
	)
	if err != nil {
		t.Fatalf("run job add: %v", err)
	}

	var result domain.JobActionResult
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("parse JSON: %v\noutput: %s", err, stdout)
	}
	if result.Name != "api" || result.Status != domain.JobActionAdded {
		t.Errorf("unexpected result: %+v", result)
	}

	cfg, err := config.LoadRun(stateDir)
	if err != nil {
		t.Fatalf("load run: %v", err)
	}
	if len(cfg.Jobs) != 1 || cfg.Jobs[0].Name != "api" {
		t.Errorf("expected one job 'api', got %+v", cfg.Jobs)
	}
}

func TestRunJobAdd_Duplicate(t *testing.T) {
	stateDir := setupTestProject(t)
	writeRunTOML(t, stateDir, domain.RunConfig{
		Jobs: []domain.JobConfig{
			{Name: "api", Kind: domain.JobKindService, Cmd: "echo hi"},
		},
	})

	_, _, err := runCmd(t,
		domain.CmdJob, domain.CmdAdd, "api",
		"--"+domain.FlagCmd, "echo dup",
	)
	if err == nil {
		t.Fatal("expected error for duplicate job name")
	}
	if !strings.Contains(err.Error(), "duplicate job name") {
		t.Errorf("expected duplicate error, got: %v", err)
	}
}

func TestRunJobRm_OK(t *testing.T) {
	stateDir := setupTestProject(t)
	writeRunTOML(t, stateDir, domain.RunConfig{
		Jobs: []domain.JobConfig{
			{Name: "api", Kind: domain.JobKindService, Cmd: "echo hi"},
		},
	})

	_, _, err := runCmd(t, domain.CmdJob, domain.CmdRm, "api")
	if err != nil {
		t.Fatalf("run job rm: %v", err)
	}

	cfg, err := config.LoadRun(stateDir)
	if err != nil {
		t.Fatalf("load run: %v", err)
	}
	if len(cfg.Jobs) != 0 {
		t.Errorf("expected zero jobs, got %+v", cfg.Jobs)
	}
}

func TestRunJobRm_NotFound(t *testing.T) {
	stateDir := setupTestProject(t)
	writeRunTOML(t, stateDir, domain.RunConfig{
		Jobs: []domain.JobConfig{
			{Name: "api", Kind: domain.JobKindService, Cmd: "echo hi"},
		},
	})

	_, _, err := runCmd(t, domain.CmdJob, domain.CmdRm, "ghost")
	if err == nil {
		t.Fatal("expected error for unknown job")
	}
	if !strings.Contains(err.Error(), "ghost") {
		t.Errorf("expected error to mention 'ghost', got: %v", err)
	}
}

func TestRunJobRm_Referenced(t *testing.T) {
	stateDir := setupTestProject(t)
	writeRunTOML(t, stateDir, domain.RunConfig{
		Jobs: []domain.JobConfig{
			{Name: "api", Kind: domain.JobKindService, Cmd: "echo hi"},
		},
		Profiles: []domain.ProfileConfig{
			{Name: "dev", Jobs: []string{"api"}},
		},
	})

	_, _, err := runCmd(t, domain.CmdJob, domain.CmdRm, "api")
	if err == nil {
		t.Fatal("expected error for referenced job")
	}
	if !strings.Contains(err.Error(), "referenced") || !strings.Contains(err.Error(), "dev") {
		t.Errorf("expected error to mention 'referenced' and 'dev', got: %v", err)
	}
}

func TestRunJobRm_ReferencedForce(t *testing.T) {
	stateDir := setupTestProject(t)
	writeRunTOML(t, stateDir, domain.RunConfig{
		Jobs: []domain.JobConfig{
			{Name: "api", Kind: domain.JobKindService, Cmd: "echo hi"},
			{Name: "build", Kind: domain.JobKindTask, Cmd: "echo build"},
		},
		Profiles: []domain.ProfileConfig{
			{Name: "dev", Jobs: []string{"api", "build"}},
		},
	})

	_, _, err := runCmd(t, domain.CmdJob, domain.CmdRm, "api", "--"+domain.FlagForce)
	if err != nil {
		t.Fatalf("run job rm --force: %v", err)
	}

	cfg, err := config.LoadRun(stateDir)
	if err != nil {
		t.Fatalf("load run: %v", err)
	}
	if len(cfg.Jobs) != 1 || cfg.Jobs[0].Name != "build" {
		t.Errorf("expected only 'build' remaining, got %+v", cfg.Jobs)
	}
	if len(cfg.Profiles) != 1 || slices.Contains(cfg.Profiles[0].Jobs, "api") {
		t.Errorf("expected 'api' stripped from profile 'dev', got %+v", cfg.Profiles)
	}
}

func TestRunProfileAdd_OK(t *testing.T) {
	stateDir := setupTestProject(t)
	writeRunTOML(t, stateDir, domain.RunConfig{
		Jobs: []domain.JobConfig{
			{Name: "api", Kind: domain.JobKindService, Cmd: "echo hi"},
		},
	})

	stdout, _, err := runCmd(t,
		domain.CmdProfile, domain.CmdAdd, "dev",
		"--"+domain.FlagJobs, "api",
		"--"+domain.FlagDefault,
		"--"+domain.FlagOutput, domain.OutputJSON,
	)
	if err != nil {
		t.Fatalf("run profile add: %v", err)
	}

	var result output.ProfileActionResult
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("parse JSON: %v\noutput: %s", err, stdout)
	}
	if result.Name != "dev" || result.Status != domain.JobActionAdded {
		t.Errorf("unexpected result: %+v", result)
	}

	cfg, err := config.LoadRun(stateDir)
	if err != nil {
		t.Fatalf("load run: %v", err)
	}
	if len(cfg.Profiles) != 1 || cfg.Profiles[0].Name != "dev" || !cfg.Profiles[0].Default {
		t.Errorf("expected default profile 'dev', got %+v", cfg.Profiles)
	}
}

func TestRunProfileAdd_UnknownJob(t *testing.T) {
	setupTestProject(t)

	_, _, err := runCmd(t,
		domain.CmdProfile, domain.CmdAdd, "dev",
		"--"+domain.FlagJobs, "ghost",
	)
	if err == nil {
		t.Fatal("expected error for unknown job ref")
	}
	if !strings.Contains(err.Error(), "unknown job") {
		t.Errorf("expected 'unknown job' error, got: %v", err)
	}
}

func TestRunProfileAdd_Duplicate(t *testing.T) {
	stateDir := setupTestProject(t)
	writeRunTOML(t, stateDir, domain.RunConfig{
		Jobs: []domain.JobConfig{
			{Name: "api", Kind: domain.JobKindService, Cmd: "echo hi"},
		},
		Profiles: []domain.ProfileConfig{
			{Name: "dev", Jobs: []string{"api"}},
		},
	})

	_, _, err := runCmd(t,
		domain.CmdProfile, domain.CmdAdd, "dev",
		"--"+domain.FlagJobs, "api",
	)
	if err == nil {
		t.Fatal("expected error for duplicate profile name")
	}
	if !strings.Contains(err.Error(), "duplicate profile name") {
		t.Errorf("expected duplicate error, got: %v", err)
	}
}

func TestRunProfileRm_OK(t *testing.T) {
	stateDir := setupTestProject(t)
	writeRunTOML(t, stateDir, domain.RunConfig{
		Jobs: []domain.JobConfig{
			{Name: "api", Kind: domain.JobKindService, Cmd: "echo hi"},
		},
		Profiles: []domain.ProfileConfig{
			{Name: "dev", Jobs: []string{"api"}},
		},
	})

	_, _, err := runCmd(t, domain.CmdProfile, domain.CmdRm, "dev")
	if err != nil {
		t.Fatalf("run profile rm: %v", err)
	}

	cfg, err := config.LoadRun(stateDir)
	if err != nil {
		t.Fatalf("load run: %v", err)
	}
	if len(cfg.Profiles) != 0 {
		t.Errorf("expected zero profiles, got %+v", cfg.Profiles)
	}
	if len(cfg.Jobs) != 1 {
		t.Errorf("jobs should be untouched, got %+v", cfg.Jobs)
	}
}

func TestRunProfileRm_NotFound(t *testing.T) {
	stateDir := setupTestProject(t)
	writeRunTOML(t, stateDir, domain.RunConfig{
		Jobs: []domain.JobConfig{
			{Name: "api", Kind: domain.JobKindService, Cmd: "echo hi"},
		},
		Profiles: []domain.ProfileConfig{
			{Name: "dev", Jobs: []string{"api"}},
		},
	})

	_, _, err := runCmd(t, domain.CmdProfile, domain.CmdRm, "ghost")
	if err == nil {
		t.Fatal("expected error for unknown profile")
	}
	if !strings.Contains(err.Error(), "ghost") {
		t.Errorf("expected error to mention 'ghost', got: %v", err)
	}
}

func TestRunJobAdd_TaskWithStop(t *testing.T) {
	setupTestProject(t)

	_, _, err := runCmd(t,
		domain.CmdJob, domain.CmdAdd, "build",
		"--"+domain.FlagCmd, "make build",
		"--"+domain.FlagKind, string(domain.JobKindTask),
		"--"+domain.FlagStop, "echo nope",
	)
	if err == nil {
		t.Fatal("expected error: tasks cannot have stop")
	}
	if !strings.Contains(err.Error(), "tasks cannot declare a stop command") {
		t.Errorf("expected stop-on-task error, got: %v", err)
	}
}

func TestRunJobAdd_InvalidKind(t *testing.T) {
	setupTestProject(t)

	_, _, err := runCmd(t,
		domain.CmdJob, domain.CmdAdd, "weird",
		"--"+domain.FlagCmd, "echo hi",
		"--"+domain.FlagKind, "daemon",
	)
	if err == nil {
		t.Fatal("expected error for unknown kind")
	}
	if !strings.Contains(err.Error(), "unknown kind") {
		t.Errorf("expected 'unknown kind' error, got: %v", err)
	}
}

func TestRunJobRm_JSONOutput(t *testing.T) {
	stateDir := setupTestProject(t)
	writeRunTOML(t, stateDir, domain.RunConfig{
		Jobs: []domain.JobConfig{
			{Name: "api", Kind: domain.JobKindService, Cmd: "echo hi"},
		},
	})

	stdout, _, err := runCmd(t,
		domain.CmdJob, domain.CmdRm, "api",
		"--"+domain.FlagOutput, domain.OutputJSON,
	)
	if err != nil {
		t.Fatalf("run job rm --output json: %v", err)
	}

	var result domain.JobActionResult
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("parse JSON: %v\noutput: %s", err, stdout)
	}
	if result.Name != "api" || result.Status != domain.JobActionRemoved {
		t.Errorf("unexpected result: %+v", result)
	}
}

func TestRunJobEdit_NotFound(t *testing.T) {
	stateDir := setupTestProject(t)
	writeRunTOML(t, stateDir, domain.RunConfig{
		Jobs: []domain.JobConfig{
			{Name: "api", Kind: domain.JobKindService, Cmd: "echo hi"},
		},
	})

	_, _, err := runCmd(t, domain.CmdJob, domain.CmdEdit, "ghost")
	if err == nil {
		t.Fatal("expected error for unknown job")
	}
	if !strings.Contains(err.Error(), "ghost") {
		t.Errorf("expected error to mention 'ghost', got: %v", err)
	}
}

func TestRunProfileAdd_DefaultOverride(t *testing.T) {
	stateDir := setupTestProject(t)
	writeRunTOML(t, stateDir, domain.RunConfig{
		Jobs: []domain.JobConfig{
			{Name: "api", Kind: domain.JobKindService, Cmd: "echo hi"},
		},
		Profiles: []domain.ProfileConfig{
			{Name: "dev", Jobs: []string{"api"}, Default: true},
		},
	})

	_, _, err := runCmd(t,
		domain.CmdProfile, domain.CmdAdd, "ci",
		"--"+domain.FlagJobs, "api",
		"--"+domain.FlagDefault,
	)
	if err != nil {
		t.Fatalf("run profile add --default with existing default: %v", err)
	}

	cfg, err := config.LoadRun(stateDir)
	if err != nil {
		t.Fatalf("load run: %v", err)
	}
	if len(cfg.Profiles) != 2 {
		t.Fatalf("expected 2 profiles, got %d", len(cfg.Profiles))
	}
	for _, p := range cfg.Profiles {
		switch p.Name {
		case "dev":
			if p.Default {
				t.Errorf("expected old default 'dev' to be unset, but Default=true")
			}
		case "ci":
			if !p.Default {
				t.Errorf("expected new 'ci' to be default, but Default=false")
			}
		default:
			t.Errorf("unexpected profile %q", p.Name)
		}
	}
}

func TestRunProfileRm_JSONOutput(t *testing.T) {
	stateDir := setupTestProject(t)
	writeRunTOML(t, stateDir, domain.RunConfig{
		Jobs: []domain.JobConfig{
			{Name: "api", Kind: domain.JobKindService, Cmd: "echo hi"},
		},
		Profiles: []domain.ProfileConfig{
			{Name: "dev", Jobs: []string{"api"}},
		},
	})

	stdout, _, err := runCmd(t,
		domain.CmdProfile, domain.CmdRm, "dev",
		"--"+domain.FlagOutput, domain.OutputJSON,
	)
	if err != nil {
		t.Fatalf("run profile rm --output json: %v", err)
	}

	var result output.ProfileActionResult
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("parse JSON: %v\noutput: %s", err, stdout)
	}
	if result.Name != "dev" || result.Status != domain.JobActionRemoved {
		t.Errorf("unexpected result: %+v", result)
	}
}

func TestRunProfileEdit_NotFound(t *testing.T) {
	stateDir := setupTestProject(t)
	writeRunTOML(t, stateDir, domain.RunConfig{
		Jobs: []domain.JobConfig{
			{Name: "api", Kind: domain.JobKindService, Cmd: "echo hi"},
		},
		Profiles: []domain.ProfileConfig{
			{Name: "dev", Jobs: []string{"api"}},
		},
	})

	_, _, err := runCmd(t, domain.CmdProfile, domain.CmdEdit, "ghost")
	if err == nil {
		t.Fatal("expected error for unknown profile")
	}
	if !strings.Contains(err.Error(), "ghost") {
		t.Errorf("expected error to mention 'ghost', got: %v", err)
	}
}

func TestRunJobList_JSONOutput(t *testing.T) {
	stateDir := setupTestProject(t)
	writeRunTOML(t, stateDir, domain.RunConfig{
		Jobs: []domain.JobConfig{
			{Name: "api", Kind: domain.JobKindService, Cmd: "echo hi"},
			{Name: "build", Kind: domain.JobKindTask, Cmd: "make"},
		},
	})

	stdout, _, err := runCmd(t,
		domain.CmdJob, domain.CmdList,
		"--"+domain.FlagOutput, domain.OutputJSON,
	)
	if err != nil {
		t.Fatalf("run job list --output json: %v", err)
	}

	var jobs []domain.JobConfig
	if err := json.Unmarshal([]byte(stdout), &jobs); err != nil {
		t.Fatalf("parse JSON: %v\noutput: %s", err, stdout)
	}
	if len(jobs) != 2 || jobs[0].Name != "api" || jobs[1].Name != "build" {
		t.Errorf("unexpected jobs: %+v", jobs)
	}
}

func TestRunProfileList_JSONOutput(t *testing.T) {
	stateDir := setupTestProject(t)
	writeRunTOML(t, stateDir, domain.RunConfig{
		Jobs: []domain.JobConfig{
			{Name: "api", Kind: domain.JobKindService, Cmd: "echo hi"},
		},
		Profiles: []domain.ProfileConfig{
			{Name: "dev", Jobs: []string{"api"}, Default: true},
		},
	})

	stdout, _, err := runCmd(t,
		domain.CmdProfile, domain.CmdList,
		"--"+domain.FlagOutput, domain.OutputJSON,
	)
	if err != nil {
		t.Fatalf("run profile list --output json: %v", err)
	}

	var profiles []domain.ProfileConfig
	if err := json.Unmarshal([]byte(stdout), &profiles); err != nil {
		t.Fatalf("parse JSON: %v\noutput: %s", err, stdout)
	}
	if len(profiles) != 1 || profiles[0].Name != "dev" || !profiles[0].Default {
		t.Errorf("unexpected profiles: %+v", profiles)
	}
}

// TestRunJobList_NotInitialized verifies the opt-in guard: on an uninitialized
// run module (no run.toml jobs/profiles), a blocked command like `job list`
// fails with the pedagogical ErrRunNotInitialized rather than emitting output.
func TestRunJobList_NotInitialized(t *testing.T) {
	setupTestProject(t)

	_, _, err := runCmd(t,
		domain.CmdJob, domain.CmdList,
		"--"+domain.FlagOutput, domain.OutputJSON,
	)
	if err == nil {
		t.Fatal("expected error on uninitialized run module")
	}
	if !errors.Is(err, domain.ErrRunNotInitialized) {
		t.Errorf("expected ErrRunNotInitialized, got: %v", err)
	}
}

func TestRunJobAdd_WithPorts(t *testing.T) {
	stateDir := setupTestProject(t)

	if _, _, err := runCmd(t,
		domain.CmdJob, domain.CmdAdd, "web",
		"--"+domain.FlagCmd, "pnpm dev",
		"--"+domain.FlagKind, string(domain.JobKindService),
		"--"+domain.FlagPort, "PORT=3000",
		"--"+domain.FlagPort, "ADMIN=9000",
		"--"+domain.FlagOutput, domain.OutputJSON,
	); err != nil {
		t.Fatalf("run job add: %v", err)
	}

	// Read back through the loader: the ports must survive the encode/decode
	// round trip, not merely reach the config in memory.
	cfg, err := config.LoadRun(stateDir)
	if err != nil {
		t.Fatalf("load run: %v", err)
	}
	ports := cfg.Jobs[0].Ports
	if ports["PORT"] != 3000 || ports["ADMIN"] != 9000 {
		t.Errorf("got %v, want PORT=3000 ADMIN=9000", ports)
	}
}

func TestRunJobAdd_RejectsMalformedPort(t *testing.T) {
	setupTestProject(t)

	_, _, err := runCmd(t,
		domain.CmdJob, domain.CmdAdd, "web",
		"--"+domain.FlagCmd, "pnpm dev",
		"--"+domain.FlagPort, "3000",
	)
	if err == nil {
		t.Fatal("expected a malformed --port to be refused")
	}
	if !strings.Contains(err.Error(), "NAME=PORT") {
		t.Errorf("the error should say the expected form, got: %v", err)
	}
}

// A base that collides with one already declared is caught on save, with the
// message that names both sides.
func TestRunJobAdd_RejectsCollidingPort(t *testing.T) {
	setupTestProject(t)

	if _, _, err := runCmd(t,
		domain.CmdJob, domain.CmdAdd, "web",
		"--"+domain.FlagCmd, "pnpm dev",
		"--"+domain.FlagPort, "PORT=3000",
	); err != nil {
		t.Fatalf("run job add: %v", err)
	}

	_, _, err := runCmd(t,
		domain.CmdJob, domain.CmdAdd, "api",
		"--"+domain.FlagCmd, "pnpm api",
		"--"+domain.FlagPort, "API_PORT=3010",
	)
	if err == nil {
		t.Fatal("expected the colliding base to be refused")
	}
	if !strings.Contains(err.Error(), "PORT") || !strings.Contains(err.Error(), "API_PORT") {
		t.Errorf("the error should name both sides, got: %v", err)
	}
}
