package run

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
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

const composeWithPorts = `services:
  postgres:                    # la base
    image: postgres:16-alpine
    ports:
      - "5432:5432"
  cache:
    ports:
      - "${CACHE_PORT:-6379}:6379"
  edge:
    ports:
      - "3000-3005:3000-3005"
`

func writeCompose(t *testing.T, name, content string) string {
	t.Helper()
	projectDir := os.Getenv("WTM_PROJECT_DIR")
	if err := os.WriteFile(filepath.Join(projectDir, name), []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", name, err)
	}
	return filepath.Join(projectDir, name)
}

func dockerJobPorts(t *testing.T, cfg domain.RunConfig) map[string]int {
	t.Helper()
	for _, j := range cfg.Jobs {
		if strings.Contains(j.Cmd, "docker") {
			return j.Ports
		}
	}
	t.Fatalf("no docker job in %+v", cfg.Jobs)
	return nil
}

// TestRunInit_DeclaresTemplatedPortsAndWithholdsFrozenOnes verifies the
// detection only declares what it can actually isolate: a mapping already
// reading a variable is picked up, a literal one is reported and left out, and
// no project file is touched without --patch-compose.
func TestRunInit_DeclaresTemplatedPortsAndWithholdsFrozenOnes(t *testing.T) {
	stateDir := setupTestProject(t)
	composePath := writeCompose(t, "docker-compose.yml", composeWithPorts)

	stdout, _, err := runCmd(t, domain.CmdInit, "--"+domain.FlagNonInteractive)
	if err != nil {
		t.Fatalf("run init: %v", err)
	}

	cfg, err := config.LoadRun(stateDir)
	if err != nil {
		t.Fatalf("load run: %v", err)
	}
	ports := dockerJobPorts(t, cfg)
	if len(ports) != 1 || ports["CACHE_PORT"] != 6379 {
		t.Errorf("ports = %v, want only the templated CACHE_PORT=6379", ports)
	}

	after, err := os.ReadFile(composePath)
	if err != nil {
		t.Fatalf("read compose: %v", err)
	}
	if string(after) != composeWithPorts {
		t.Errorf("the compose file must be untouched without --%s:\n%s", domain.FlagPatchCompose, after)
	}

	for _, want := range []string{"5432:5432", "POSTGRES_PORT", "run job edit", "range"} {
		if !strings.Contains(stdout, want) {
			t.Errorf("the report must mention %q, got:\n%s", want, stdout)
		}
	}
}

// TestRunInit_PatchComposeTemplatizesAndDeclares verifies --patch-compose
// rewrites the literal mappings and declares the ports they now read, leaving
// the rest of the file byte for byte as it was.
func TestRunInit_PatchComposeTemplatizesAndDeclares(t *testing.T) {
	stateDir := setupTestProject(t)
	composePath := writeCompose(t, "docker-compose.yml", composeWithPorts)

	if _, _, err := runCmd(t, domain.CmdInit, "--"+domain.FlagNonInteractive, "--"+domain.FlagPatchCompose); err != nil {
		t.Fatalf("run init: %v", err)
	}

	after, err := os.ReadFile(composePath)
	if err != nil {
		t.Fatalf("read compose: %v", err)
	}
	got := string(after)
	if !strings.Contains(got, `- "${POSTGRES_PORT:-5432}:5432"`) {
		t.Errorf("the literal mapping was not templatized:\n%s", got)
	}
	for _, untouched := range []string{"  postgres:                    # la base", "image: postgres:16-alpine", `- "3000-3005:3000-3005"`} {
		if !strings.Contains(got, untouched) {
			t.Errorf("the rewrite must leave %q alone:\n%s", untouched, got)
		}
	}

	cfg, err := config.LoadRun(stateDir)
	if err != nil {
		t.Fatalf("load run: %v", err)
	}
	ports := dockerJobPorts(t, cfg)
	if ports["POSTGRES_PORT"] != 5432 || ports["CACHE_PORT"] != 6379 {
		t.Errorf("ports = %v, want POSTGRES_PORT=5432 and CACHE_PORT=6379", ports)
	}
}

// TestRunInit_BackfillsAJobConfiguredBeforePortsExisted verifies re-running
// init gives an existing compose job the ports it never declared, without
// touching anything else about it.
func TestRunInit_BackfillsAJobConfiguredBeforePortsExisted(t *testing.T) {
	stateDir := setupTestProject(t)
	writeCompose(t, "docker-compose.yml", composeWithPorts)
	writeRunTOML(t, stateDir, domain.RunConfig{
		Jobs: []domain.JobConfig{{
			Name: "docker-compose",
			Kind: domain.JobKindService,
			Cmd:  "docker compose -f docker-compose.yml up -d",
			Stop: "docker compose -f docker-compose.yml down --remove-orphans",
			Cwd:  ".",
		}},
	})

	if _, _, err := runCmd(t, domain.CmdInit, "--"+domain.FlagNonInteractive); err != nil {
		t.Fatalf("run init: %v", err)
	}

	cfg, err := config.LoadRun(stateDir)
	if err != nil {
		t.Fatalf("load run: %v", err)
	}
	if len(cfg.Jobs) != 1 {
		t.Fatalf("the existing job must not be duplicated, got %+v", cfg.Jobs)
	}
	job := cfg.Jobs[0]
	if job.Ports["CACHE_PORT"] != 6379 {
		t.Errorf("ports = %v, want the detected CACHE_PORT backfilled", job.Ports)
	}
	if job.Cmd != "docker compose -f docker-compose.yml up -d" || job.Cwd != "." {
		t.Errorf("backfill must touch nothing but ports, got %+v", job)
	}
}

// TestRunInit_NeverWritesAConfigItsOwnLoaderRefuses verifies two compose files
// exposing the same host port produce a loadable run.toml: the second
// declaration is withdrawn and reported rather than written.
func TestRunInit_NeverWritesAConfigItsOwnLoaderRefuses(t *testing.T) {
	stateDir := setupTestProject(t)
	writeCompose(t, "docker-compose.yml", "services:\n  a:\n    ports:\n      - \"${A_PORT:-5432}:5432\"\n")
	writeCompose(t, "docker-compose.other.yml", "services:\n  b:\n    ports:\n      - \"${B_PORT:-5442}:5432\"\n")

	stdout, _, err := runCmd(t, domain.CmdInit, "--"+domain.FlagNonInteractive)
	if err != nil {
		t.Fatalf("run init: %v", err)
	}

	cfg, err := config.LoadRun(stateDir)
	if err != nil {
		t.Fatalf("the written config must load: %v", err)
	}

	// Both sides were inferred, so wtm has nothing to arbitrate on and withdraws
	// the pair rather than picking a winner.
	declared := 0
	for _, j := range cfg.Jobs {
		declared += len(j.Ports)
	}
	if declared != 0 {
		t.Errorf("two colliding detected bases must both be withdrawn, got %+v", cfg.Jobs)
	}
	for _, want := range []string{"withdrawn", "A_PORT", "B_PORT"} {
		if !strings.Contains(stdout, want) {
			t.Errorf("the report must name %q, got:\n%s", want, stdout)
		}
	}
}

// TestRunInit_BackfillsAJobWhoseNameWasChanged verifies a compose file already
// run by a job does not get a second one just because that job was renamed —
// the two would declare the same ports and both lose them to the collision
// check, leaving the file with no isolation at all.
func TestRunInit_BackfillsAJobWhoseNameWasChanged(t *testing.T) {
	stateDir := setupTestProject(t)
	writeCompose(t, "docker-compose.yml", composeWithPorts)
	writeRunTOML(t, stateDir, domain.RunConfig{
		Jobs: []domain.JobConfig{{
			Name: "docker",
			Kind: domain.JobKindService,
			Cmd:  "docker compose -f docker-compose.yml up -d",
			Stop: "docker compose -f docker-compose.yml down --remove-orphans",
			Cwd:  ".",
		}},
	})

	if _, _, err := runCmd(t, domain.CmdInit, "--"+domain.FlagNonInteractive); err != nil {
		t.Fatalf("run init: %v", err)
	}

	cfg, err := config.LoadRun(stateDir)
	if err != nil {
		t.Fatalf("load run: %v", err)
	}
	if len(cfg.Jobs) != 1 || cfg.Jobs[0].Name != "docker" {
		t.Fatalf("the renamed job must stay the only one, got %+v", cfg.Jobs)
	}
	if cfg.Jobs[0].Ports["CACHE_PORT"] != 6379 {
		t.Errorf("ports = %v, want the detected port backfilled onto it", cfg.Jobs[0].Ports)
	}
}

// TestRunInit_TwoComposeFilesOnTheSameBase is the case the spec names: the two
// declarations meet inside a single worktree, not some worktrees apart.
func TestRunInit_TwoComposeFilesOnTheSameBase(t *testing.T) {
	stateDir := setupTestProject(t)
	writeCompose(t, "docker-compose.yml", "services:\n  a:\n    ports:\n      - \"${A_PORT:-5432}:5432\"\n")
	writeCompose(t, "docker-compose.other.yml", "services:\n  b:\n    ports:\n      - \"${B_PORT:-5432}:5432\"\n")

	stdout, _, err := runCmd(t, domain.CmdInit, "--"+domain.FlagNonInteractive)
	if err != nil {
		t.Fatalf("run init: %v", err)
	}

	cfg, err := config.LoadRun(stateDir)
	if err != nil {
		t.Fatalf("the written config must load: %v", err)
	}
	for _, j := range cfg.Jobs {
		if len(j.Ports) != 0 {
			t.Errorf("job %q kept %v; two identical bases can never coexist", j.Name, j.Ports)
		}
	}
	if !strings.Contains(stdout, "withdrawn") {
		t.Errorf("the withdrawal must be reported, got:\n%s", stdout)
	}
}

// TestRunInit_PatchComposeIsIdempotent verifies a second run neither rewrites
// the file again nor duplicates anything.
func TestRunInit_PatchComposeIsIdempotent(t *testing.T) {
	stateDir := setupTestProject(t)
	composePath := writeCompose(t, "docker-compose.yml", composeWithPorts)

	flags := []string{domain.CmdInit, "--" + domain.FlagNonInteractive, "--" + domain.FlagPatchCompose}
	if _, _, err := runCmd(t, flags...); err != nil {
		t.Fatalf("first run: %v", err)
	}
	first, err := os.ReadFile(composePath)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	firstCfg, err := config.LoadRun(stateDir)
	if err != nil {
		t.Fatalf("load: %v", err)
	}

	if _, _, err := runCmd(t, flags...); err != nil {
		t.Fatalf("second run: %v", err)
	}
	second, err := os.ReadFile(composePath)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	secondCfg, err := config.LoadRun(stateDir)
	if err != nil {
		t.Fatalf("load: %v", err)
	}

	if string(first) != string(second) {
		t.Errorf("the compose file changed on the second run:\n%s", second)
	}
	if len(firstCfg.Jobs) != len(secondCfg.Jobs) {
		t.Errorf("jobs went from %d to %d", len(firstCfg.Jobs), len(secondCfg.Jobs))
	}
}
