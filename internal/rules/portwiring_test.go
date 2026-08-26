package rules

import (
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
)

func wiringConfig() domain.RunConfig {
	return domain.RunConfig{Jobs: []domain.JobConfig{
		{Name: "docker-compose", Kind: domain.JobKindService, Cmd: "docker compose up", Ports: map[string]int{"POSTGRES_PORT": 5432}},
		{Name: "api-dev", Kind: domain.JobKindService, Cmd: "pnpm dev", Ports: map[string]int{"PORT": 3001}},
		{Name: "web-dev", Kind: domain.JobKindService, Cmd: "vite --port ${WEB_PORT}", Ports: map[string]int{"WEB_PORT": 5173}},
		{Name: "seed", Kind: domain.JobKindTask, Cmd: "node seed.js", Ports: map[string]int{"PORT": 1}},
	}}
}

func TestJobsMissingPortRefNamesACommandThatIgnoresItsVariable(t *testing.T) {
	got := JobsMissingPortRef(JobsMissingPortRefParams{
		Config: wiringConfig(),
		Exempt: []string{"docker-compose"},
	})

	if len(got) != 1 {
		t.Fatalf("expected api-dev alone, got %+v", got)
	}
	if got[0].Job != "api-dev" {
		t.Errorf("job = %q, want api-dev", got[0].Job)
	}
	if len(got[0].Vars) != 1 || got[0].Vars[0] != "PORT" {
		t.Errorf("vars = %v, want [PORT]", got[0].Vars)
	}
}

func TestJobsMissingPortRefSpareAJobThatReferencesIt(t *testing.T) {
	for _, cmd := range []string{"vite --port ${WEB_PORT}", "vite --port $WEB_PORT", "PORT=$WEB_PORT vite"} {
		cfg := domain.RunConfig{Jobs: []domain.JobConfig{
			{Name: "web", Kind: domain.JobKindService, Cmd: cmd, Ports: map[string]int{"WEB_PORT": 5173}},
		}}
		if got := JobsMissingPortRef(JobsMissingPortRefParams{Config: cfg}); len(got) != 0 {
			t.Errorf("%q references the variable, yet was flagged: %+v", cmd, got)
		}
	}
}

func TestJobsMissingPortRefSparesAnExemptJob(t *testing.T) {
	// A compose stack reads its ports from the compose file, never from the
	// command that starts it.
	got := JobsMissingPortRef(JobsMissingPortRefParams{
		Config: wiringConfig(),
		Exempt: []string{"docker-compose", "api-dev"},
	})

	if len(got) != 0 {
		t.Errorf("every flagged job was exempt, got %+v", got)
	}
}

func TestJobsMissingPortRefIgnoresTasks(t *testing.T) {
	cfg := domain.RunConfig{Jobs: []domain.JobConfig{
		{Name: "seed", Kind: domain.JobKindTask, Cmd: "node seed.js", Ports: map[string]int{"PORT": 1}},
	}}

	if got := JobsMissingPortRef(JobsMissingPortRefParams{Config: cfg}); len(got) != 0 {
		t.Errorf("a task binds nothing, yet was flagged: %+v", got)
	}
}

func TestPortEntriesForOffersAServiceThatDeclaredNothing(t *testing.T) {
	cfg := domain.RunConfig{Jobs: []domain.JobConfig{
		{Name: "api-dev", Kind: domain.JobKindService, Ports: map[string]int{"PORT": 3001}},
		{Name: "web-dev", Kind: domain.JobKindService},
		{Name: "seed", Kind: domain.JobKindTask},
	}}

	got := PortEntriesFor(PortEntriesForParams{Config: cfg})

	if len(got) != 2 {
		t.Fatalf("every service must get a row, declared or not: %+v", got)
	}
	if got[1].Job != "web-dev" || got[1].Base != 0 {
		t.Errorf("the undeclared service = %+v, want web-dev with no base", got[1])
	}
	if got[1].Name != "WEB_DEV_PORT" {
		t.Errorf("name = %q, want one free of the PORT api-dev already carries", got[1].Name)
	}
}

func TestApplyInitAnswersWritesANewlyDeclaredPort(t *testing.T) {
	cfg := domain.RunConfig{Jobs: []domain.JobConfig{
		{Name: "web-dev", Kind: domain.JobKindService},
	}}

	got := ApplyInitAnswers(ApplyInitAnswersParams{
		Config: cfg,
		Ports:  []domain.PortEntry{{Job: "web-dev", Name: "PORT", Base: 5173}},
	})

	if got.Jobs[0].Ports["PORT"] != 5173 {
		t.Errorf("ports = %v, want the port the user declared", got.Jobs[0].Ports)
	}
}

func TestApplyInitAnswersLeavesAPortTheUserDeclined(t *testing.T) {
	cfg := domain.RunConfig{Jobs: []domain.JobConfig{
		{Name: "web-dev", Kind: domain.JobKindService},
	}}

	got := ApplyInitAnswers(ApplyInitAnswersParams{
		Config: cfg,
		Ports:  []domain.PortEntry{{Job: "web-dev", Name: "PORT", Base: 0}},
	})

	if len(got.Jobs[0].Ports) != 0 {
		t.Errorf("ports = %v, want none — an empty answer declares nothing", got.Jobs[0].Ports)
	}
}

func TestPortEntriesForAvoidsAVariableAnotherJobAlreadyUses(t *testing.T) {
	// Two jobs may each carry their own PORT — their environments are separate.
	// But the env-file linking flattens variables to one base, so proposing a
	// name already taken would make a .env key follow the wrong port.
	cfg := domain.RunConfig{Jobs: []domain.JobConfig{
		{Name: "api-dev", Kind: domain.JobKindService, Ports: map[string]int{"PORT": 3001}},
		{Name: "web-dev", Kind: domain.JobKindService},
	}}

	got := PortEntriesFor(PortEntriesForParams{Config: cfg})

	if got[1].Name == "PORT" {
		t.Errorf("web-dev was offered %q, already taken by api-dev", got[1].Name)
	}
	if got[1].Name != "WEB_DEV_PORT" {
		t.Errorf("name = %q, want one derived from the job", got[1].Name)
	}
}

func TestPortEntriesForKeepsThePlainNameWhenItIsFree(t *testing.T) {
	cfg := domain.RunConfig{Jobs: []domain.JobConfig{
		{Name: "web-dev", Kind: domain.JobKindService},
	}}

	if got := PortEntriesFor(PortEntriesForParams{Config: cfg})[0].Name; got != domain.PortNameDefault {
		t.Errorf("name = %q, want the plain default when nothing takes it", got)
	}
}

func TestPortEntriesForSeparatesTwoUndeclaredServices(t *testing.T) {
	cfg := domain.RunConfig{Jobs: []domain.JobConfig{
		{Name: "web-dev", Kind: domain.JobKindService},
		{Name: "admin-dev", Kind: domain.JobKindService},
	}}

	got := PortEntriesFor(PortEntriesForParams{Config: cfg})

	if got[0].Name == got[1].Name {
		t.Errorf("both services were offered %q", got[0].Name)
	}
}

func TestPortEntriesForOffersAListeningPortToAServiceThatOnlyDeclaredADependency(t *testing.T) {
	// DB_PORT is the port of something the job talks to, not the one it binds.
	// wtm cannot tell the direction apart, so it stops short of concluding and
	// offers the row the user alone can fill.
	cfg := domain.RunConfig{Jobs: []domain.JobConfig{
		{Name: "api-dev", Kind: domain.JobKindService, Cwd: "apps/api", Ports: map[string]int{"DB_PORT": 9999}},
	}}

	got := PortEntriesFor(PortEntriesForParams{Config: cfg})

	if len(got) != 2 {
		t.Fatalf("expected DB_PORT plus an offered row, got %+v", got)
	}
	if got[1].Name != domain.PortNameDefault || got[1].Base != 0 {
		t.Errorf("offered row = %+v, want an empty PORT", got[1])
	}
}

func TestPortEntriesForAsksNothingMoreOfAServiceThatDeclaredItsPort(t *testing.T) {
	cfg := domain.RunConfig{Jobs: []domain.JobConfig{
		{Name: "api-dev", Kind: domain.JobKindService, Cwd: "apps/api", Ports: map[string]int{"PORT": 3001, "DB_PORT": 9999}},
	}}

	if got := PortEntriesFor(PortEntriesForParams{Config: cfg}); len(got) != 2 {
		t.Errorf("PORT is declared, nothing more to ask: %+v", got)
	}
}

func TestPortEntriesForTrustsAComposeJobsDeclaration(t *testing.T) {
	// A compose file's `ports:` list is the whole story: nothing is missing from
	// it, so offering another row would ask about a port that does not exist.
	cfg := domain.RunConfig{Jobs: []domain.JobConfig{
		{Name: "docker-compose", Kind: domain.JobKindService, Ports: map[string]int{"POSTGRES_PORT": 5432}},
	}}

	got := PortEntriesFor(PortEntriesForParams{Config: cfg, ComposeJobs: []string{"docker-compose"}})

	if len(got) != 1 {
		t.Errorf("a compose job needs no extra row: %+v", got)
	}
}
