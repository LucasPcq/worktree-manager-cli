package rules

import (
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
)

func composeAnswers(files ...string) domain.InitProjectAnswers {
	return domain.InitProjectAnswers{
		DockerComposeFiles: files,
		DockerComposeCmd:   "docker compose",
		PatchCompose:       true,
	}
}

func frozenAt(file, service, name string, base int) domain.ComposePortBinding {
	return domain.ComposePortBinding{
		File: file, Service: service, Status: domain.ComposePortFrozen,
		Var: name, Base: base, Token: "x", Replacement: "y",
	}
}

// TestResolveComposePortsNeverRewritesForAPortItThenWithdraws is the ordering
// the whole resolver exists for: two bases a block apart lose their
// declarations, so neither mapping may be rewritten in the project's file.
func TestResolveComposePortsNeverRewritesForAPortItThenWithdraws(t *testing.T) {
	file := "docker-compose.yml"
	plan := PlanComposePorts(PlanComposePortsParams{
		Files: []string{file},
		Patch: true,
		Scans: map[string]domain.ComposeScan{file: {File: file, Bindings: []domain.ComposePortBinding{
			frozenAt(file, "storefront", "STOREFRONT_PORT", 3000),
			frozenAt(file, "admin", "ADMIN_PORT", 3010),
		}}},
	})

	got := ResolveComposePorts(ResolveComposePortsParams{
		Answers: composeAnswers(file),
		Plan:    plan,
	})

	if len(got.Dropped) != 2 {
		t.Fatalf("both bases must be withdrawn, got %+v", got.Dropped)
	}
	if len(got.Patches) != 0 {
		t.Errorf("no file may be rewritten for a port that is not declared, got %v", got.Patches)
	}
	if len(got.Written) != 0 {
		t.Errorf("written = %v, want nothing", got.Written)
	}
}

func TestResolveComposePortsRewritesOnlyTheSurvivingMapping(t *testing.T) {
	file := "docker-compose.yml"
	plan := PlanComposePorts(PlanComposePortsParams{
		Files: []string{file},
		Patch: true,
		Scans: map[string]domain.ComposeScan{file: {File: file, Bindings: []domain.ComposePortBinding{
			frozenAt(file, "postgres", "POSTGRES_PORT", 5432),
			frozenAt(file, "admin", "ADMIN_PORT", 3010),
		}}},
	})

	// A hand-written 3000 outranks the detected 3010 and takes it down with it.
	existing := domain.RunConfig{Jobs: []domain.JobConfig{
		{Name: "web", Kind: domain.JobKindService, Cmd: "pnpm dev", Ports: map[string]int{"PORT": 3000}},
	}}

	got := ResolveComposePorts(ResolveComposePortsParams{
		Answers:  composeAnswers(file),
		Existing: existing,
		Plan:     plan,
	})

	if len(got.Patches[file]) != 1 || got.Patches[file][0].Var != "POSTGRES_PORT" {
		t.Fatalf("only the surviving mapping is rewritten, got %+v", got.Patches[file])
	}
	if len(got.Dropped) != 1 || got.Dropped[0].Port.Name != "ADMIN_PORT" {
		t.Errorf("dropped = %+v", got.Dropped)
	}
}

func TestResolveComposePortsWithdrawsAFileThatChangedUnderIt(t *testing.T) {
	file := "docker-compose.yml"
	plan := PlanComposePorts(PlanComposePortsParams{
		Files: []string{file},
		Patch: true,
		Scans: map[string]domain.ComposeScan{file: {File: file, Bindings: []domain.ComposePortBinding{
			frozenAt(file, "postgres", "POSTGRES_PORT", 5432),
		}}},
	})

	got := ResolveComposePorts(ResolveComposePortsParams{
		Answers:      composeAnswers(file),
		Plan:         plan,
		Unverifiable: map[string]string{file: "the file changed since it was read"},
	})

	if len(got.Patches) != 0 || len(got.Written) != 0 {
		t.Errorf("a stale scan contributes nothing, got patches=%v written=%v", got.Patches, got.Written)
	}
	if got.Changed[file] == "" {
		t.Error("the reason must reach the report")
	}
	// The run still does its job: the compose file gets its runner.
	if len(got.Config.Jobs) != 1 {
		t.Errorf("jobs = %+v, want the run to carry on", got.Config.Jobs)
	}
}

// TestResolveComposePortsReportsAFileWithNoJob covers the silent case: a job
// name already taken by another file leaves the new one with no runner, so its
// ports have nowhere to go.
func TestResolveComposePortsReportsAFileWithNoJob(t *testing.T) {
	file := "docker-compose.yml"
	plan := PlanComposePorts(PlanComposePortsParams{
		Files: []string{file},
		Patch: true,
		Scans: map[string]domain.ComposeScan{file: {File: file, Bindings: []domain.ComposePortBinding{
			frozenAt(file, "postgres", "POSTGRES_PORT", 5432),
		}}},
	})

	existing := domain.RunConfig{Jobs: []domain.JobConfig{
		{Name: "docker-compose", Kind: domain.JobKindService, Cmd: "docker compose -f infra/docker-compose.yml up -d"},
	}}

	got := ResolveComposePorts(ResolveComposePortsParams{
		Answers:  composeAnswers(file),
		Existing: existing,
		Plan:     plan,
	})

	if len(got.Orphaned) != 1 || got.Orphaned[0] != file {
		t.Fatalf("orphaned = %v, want %s named", got.Orphaned, file)
	}
	if len(got.Patches) != 0 {
		t.Errorf("a file whose ports go nowhere must not be rewritten, got %v", got.Patches)
	}
	if len(got.Written) != 0 {
		t.Errorf("written = %v, want nothing", got.Written)
	}
}

func TestResolveComposePortsIsIdempotent(t *testing.T) {
	file := "docker-compose.yml"
	scans := map[string]domain.ComposeScan{file: {File: file, Bindings: []domain.ComposePortBinding{
		{File: file, Service: "postgres", Status: domain.ComposePortTemplated, Var: "DB_PORT", Base: 5432},
	}}}
	plan := PlanComposePorts(PlanComposePortsParams{Files: []string{file}, Patch: true, Scans: scans})

	first := ResolveComposePorts(ResolveComposePortsParams{Answers: composeAnswers(file), Plan: plan})
	second := ResolveComposePorts(ResolveComposePortsParams{
		Answers:  composeAnswers(file),
		Existing: first.Config,
		Plan:     PlanComposePorts(PlanComposePortsParams{Files: []string{file}, Patch: true, Scans: scans}),
	})

	if len(second.Config.Jobs) != len(first.Config.Jobs) {
		t.Errorf("a second run duplicated jobs: %d → %d", len(first.Config.Jobs), len(second.Config.Jobs))
	}
	if len(second.Written) != 0 {
		t.Errorf("a second run declares nothing new, got %v", second.Written)
	}
	if len(second.Patches) != 0 {
		t.Errorf("a second run rewrites nothing, got %v", second.Patches)
	}
}
