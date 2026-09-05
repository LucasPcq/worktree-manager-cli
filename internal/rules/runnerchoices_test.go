package rules

import (
	"strings"
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
)

// monorepoConfig is the shape the step exists for: two root scripts that fan
// out, and the app scripts they start.
func monorepoConfig() domain.RunConfig {
	return domain.RunConfig{Jobs: []domain.JobConfig{
		{Name: "docker-compose", Kind: domain.JobKindService, Cmd: "docker compose up -d", Cwd: ".", Ports: map[string]int{"DB_PORT": 5432}},
		{Name: "dev:crm", Kind: domain.JobKindService, Cmd: "pnpm run dev:crm", Cwd: "."},
		{Name: "dev:shop", Kind: domain.JobKindService, Cmd: "pnpm run dev:shop", Cwd: "."},
		{Name: "crm-web-dev", Kind: domain.JobKindService, Cmd: "pnpm run dev", Cwd: "apps/crm/web", Ports: map[string]int{"VITE_PORT": 5175}},
		{Name: "seed", Kind: domain.JobKindTask, Cmd: "pnpm run seed", Cwd: "apps/crm/api"},
	}}
}

func TestRunnerCandidatesAreTheRootServicesHoldingNoPort(t *testing.T) {
	names := RunnerCandidates(RunnerChoicesParams{Config: monorepoConfig(), ComposeJobs: []string{"docker-compose"}})

	if len(names) != 2 || names[0] != "dev:crm" || names[1] != "dev:shop" {
		t.Fatalf("got %+v", names)
	}
}

func TestRunnerChoicesOfferOneRowPerNestedService(t *testing.T) {
	choices := RunnerChoices(RunnerChoicesParams{Config: monorepoConfig(), ComposeJobs: []string{"docker-compose"}})

	if len(choices) != 1 || choices[0].Job != "crm-web-dev" {
		t.Fatalf("got %+v — a task binds nothing and a root service is a candidate, not a row", choices)
	}
	if choices[0].Runner != "" {
		t.Fatal("no relation is the default: which command fans out is the one thing wtm refuses to infer")
	}
	if len(choices[0].Options) != 3 || choices[0].Options[0] != "" {
		t.Fatalf("options = %+v, want none first then both candidates", choices[0].Options)
	}
}

func TestRunnerChoicesAreEmptyWithoutACandidate(t *testing.T) {
	cfg := domain.RunConfig{Jobs: []domain.JobConfig{
		{Name: "web", Kind: domain.JobKindService, Cmd: "vite", Cwd: "apps/web"},
	}}

	if choices := RunnerChoices(RunnerChoicesParams{Config: cfg}); len(choices) != 0 {
		t.Fatalf("got %+v", choices)
	}
}

func TestRunnerChoicesArePreFilledFromTheConfig(t *testing.T) {
	cfg := monorepoConfig()
	cfg.Jobs[1].Runs = []string{"crm-web-dev"}

	choices := RunnerChoices(RunnerChoicesParams{Config: cfg, ComposeJobs: []string{"docker-compose"}})
	if choices[0].Runner != "dev:crm" {
		t.Fatalf("a re-init shows what was settled: %+v", choices[0])
	}
}

func TestApplyRunnerChoicesWritesTheRelation(t *testing.T) {
	cfg := monorepoConfig()
	choices := RunnerChoices(RunnerChoicesParams{Config: cfg, ComposeJobs: []string{"docker-compose"}})
	choices[0].Runner = "dev:crm"

	out := ApplyRunnerChoices(ApplyRunnerChoicesParams{Config: cfg, Choices: choices})
	crm, _ := FindJob(out, "dev:crm")
	if len(crm.Runs) != 1 || crm.Runs[0] != "crm-web-dev" {
		t.Fatalf("got %+v", crm.Runs)
	}
	if _, errs := ValidateRun(out); len(errs) != 0 {
		t.Fatalf("the relation it writes must load back: %v", errs)
	}
}

func TestApplyRunnerChoicesWithdrawsWhatARowGaveBack(t *testing.T) {
	cfg := monorepoConfig()
	cfg.Jobs[1].Runs = []string{"crm-web-dev"}

	choices := RunnerChoices(RunnerChoicesParams{Config: cfg, ComposeJobs: []string{"docker-compose"}})
	choices[0].Runner = ""

	out := ApplyRunnerChoices(ApplyRunnerChoicesParams{Config: cfg, Choices: choices})
	if crm, _ := FindJob(out, "dev:crm"); len(crm.Runs) != 0 {
		t.Fatalf("a row set back to none withdraws the relation: %+v", crm.Runs)
	}
}

func TestApplyRunnerChoicesLeavesAJobTheStepNeverOffered(t *testing.T) {
	cfg := monorepoConfig()
	cfg.Jobs[0].Runs = []string{"crm-web-dev"}

	out := ApplyRunnerChoices(ApplyRunnerChoicesParams{Config: cfg, Choices: RunnerChoices(RunnerChoicesParams{Config: cfg, ComposeJobs: []string{"docker-compose"}})})
	if compose, _ := FindJob(out, "docker-compose"); len(compose.Runs) != 1 {
		t.Fatalf("the compose job was never a candidate, so the step does not speak for it: %+v", compose.Runs)
	}
}

func TestPortEntriesForFollowsTheRelationTheStepJustSettled(t *testing.T) {
	cfg := monorepoConfig()
	choices := RunnerChoices(RunnerChoicesParams{Config: cfg, ComposeJobs: []string{"docker-compose"}})
	choices[0].Runner = "dev:crm"

	entries := PortEntriesFor(PortEntriesForParams{
		Config:      ApplyRunnerChoices(ApplyRunnerChoicesParams{Config: cfg, Choices: choices}),
		ComposeJobs: []string{"docker-compose"},
	})

	for _, entry := range entries {
		if entry.Job == "dev:crm" && !entry.BindsNone {
			t.Fatalf("a runner is not a service that forgot a port: %+v", entry)
		}
		if entry.Job == "dev:shop" && entry.BindsNone {
			t.Fatalf("dev:shop runs nothing, so its row is still a question: %+v", entry)
		}
	}
}

func TestApplyInitAnswersWritesTheAddressingTheStepSettled(t *testing.T) {
	cfg := domain.RunConfig{Addressing: domain.AddressingNames}

	out := ApplyInitAnswers(ApplyInitAnswersParams{
		Config:          cfg,
		Addressing:      domain.AddressingPorts,
		AddressingAsked: true,
	})
	if out.Addressing != domain.AddressingPorts {
		t.Fatalf("got %q", out.Addressing)
	}
}

func TestApplyInitAnswersLeavesTheAddressingAStepNeverAskedAbout(t *testing.T) {
	cfg := domain.RunConfig{Addressing: domain.AddressingPorts}

	if out := ApplyInitAnswers(ApplyInitAnswersParams{Config: cfg}); out.Addressing != domain.AddressingPorts {
		t.Fatalf("got %q — a run that never asked must not write the default over the file", out.Addressing)
	}
}

func TestAddressingChoicesOpenOnTheModeTheProjectIsOn(t *testing.T) {
	if got := AddressingChoices(domain.AddressingNames); got[0] != domain.AddressingNames {
		t.Fatalf("got %+v", got)
	}
	if got := AddressingChoices(domain.AddressingPorts); got[0] != domain.AddressingPorts {
		t.Fatalf("got %+v", got)
	}
}

func TestAddressingStepSaysWhatNamedUrlsCost(t *testing.T) {
	if !strings.Contains(domain.AddressingStepDesc, "`wtm run`") {
		t.Fatal("the step must say that named urls answer only while wtm runs the job")
	}
}

func TestAnyJobPublishesANameReadsTheConfigAsItWillBeWritten(t *testing.T) {
	bare := domain.RunConfig{Jobs: []domain.JobConfig{{Name: "web", Kind: domain.JobKindService}}}
	if AnyJobPublishesAName(bare) {
		t.Fatal("no job publishes a name, so the mode changes nothing")
	}

	published := domain.RunConfig{Jobs: []domain.JobConfig{{Name: "web", Kind: domain.JobKindService, URL: &domain.JobURLConfig{Port: "PORT"}}}}
	if !AnyJobPublishesAName(published) {
		t.Fatal("a published job is addressed by the mode")
	}
}

func TestScriptsStepDescriptionWarnsOnlyWhereTheTrapExists(t *testing.T) {
	flat := []domain.PackageScript{{Name: "dev"}}
	if strings.Contains(ScriptsStepDescription(flat), domain.MonorepoRootHint) {
		t.Fatal("a single-package repo has no root-versus-app choice to make")
	}

	monorepo := append(flat, domain.PackageScript{Name: "dev", Workspace: "apps/web"})
	if !strings.Contains(ScriptsStepDescription(monorepo), domain.MonorepoRootHint) {
		t.Fatal("the trap is sprung here and the step must say so")
	}

	nestedOnly := []domain.PackageScript{{Name: "dev", Workspace: "apps/web"}}
	if strings.Contains(ScriptsStepDescription(nestedOnly), domain.MonorepoRootHint) {
		t.Fatal("no root script, nothing to warn about")
	}
}

func TestApplyRunnerChoicesKeepsAChildTheStepNeverOffered(t *testing.T) {
	cfg := monorepoConfig()
	// `seed` is a task: the step never rows it, so it never speaks for it.
	cfg.Jobs[1].Runs = []string{"seed"}

	choices := RunnerChoices(RunnerChoicesParams{Config: cfg, ComposeJobs: []string{"docker-compose"}})
	choices[0].Runner = "dev:crm"

	out := ApplyRunnerChoices(ApplyRunnerChoicesParams{Config: cfg, Choices: choices})
	crm, _ := FindJob(out, "dev:crm")
	if len(crm.Runs) != 2 {
		t.Fatalf("got %+v, want the hand-written child kept alongside the answered one", crm.Runs)
	}
	if crm.Runs[0] != "seed" {
		t.Fatalf("got %+v", crm.Runs)
	}
}
