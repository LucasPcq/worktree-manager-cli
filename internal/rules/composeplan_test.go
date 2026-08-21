package rules

import (
	"reflect"
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
)

func scanWith(file string, bindings ...domain.ComposePortBinding) domain.ComposeScan {
	for i := range bindings {
		bindings[i].File = file
	}
	return domain.ComposeScan{File: file, Bindings: bindings}
}

func templated(service, name string, base int) domain.ComposePortBinding {
	return domain.ComposePortBinding{Service: service, Status: domain.ComposePortTemplated, Var: name, Base: base}
}

func frozen(service, name string, base int) domain.ComposePortBinding {
	return domain.ComposePortBinding{
		Service: service, Status: domain.ComposePortFrozen, Var: name, Base: base,
		Token: "x", Replacement: "y",
	}
}

func TestPlanComposePortsDeclaresTemplatedAndWithholdsFrozen(t *testing.T) {
	scans := map[string]domain.ComposeScan{
		"docker-compose.yml": scanWith("docker-compose.yml",
			templated("postgres", "DB_PORT", 5432),
			frozen("redis", "REDIS_PORT", 6379),
			domain.ComposePortBinding{Service: "edge", Status: domain.ComposePortUnsupported, Reason: "port range"},
		),
	}

	plan := PlanComposePorts(PlanComposePortsParams{Scans: scans, Files: []string{"docker-compose.yml"}})

	want := map[string]int{"DB_PORT": 5432}
	if !reflect.DeepEqual(plan.PortsByFile["docker-compose.yml"], want) {
		t.Errorf("declared %v, want only the templated one: %v", plan.PortsByFile["docker-compose.yml"], want)
	}
	if len(plan.Patches) != 0 {
		t.Errorf("nothing is rewritten without Patch, got %v", plan.Patches)
	}
	if len(plan.Withheld) != 2 {
		t.Fatalf("withheld %d, want the frozen one and the unsupported one", len(plan.Withheld))
	}
	if plan.Withheld[0].Status != domain.ComposePortFrozen || plan.Withheld[1].Status != domain.ComposePortUnsupported {
		t.Errorf("withheld = %+v", plan.Withheld)
	}
}

func TestPlanComposePortsDeclaresFrozenOnceAuthorized(t *testing.T) {
	scans := map[string]domain.ComposeScan{
		"docker-compose.yml": scanWith("docker-compose.yml",
			templated("postgres", "DB_PORT", 5432),
			frozen("redis", "REDIS_PORT", 6379),
		),
	}

	plan := PlanComposePorts(PlanComposePortsParams{Scans: scans, Files: []string{"docker-compose.yml"}, Patch: true})

	want := map[string]int{"DB_PORT": 5432, "REDIS_PORT": 6379}
	if !reflect.DeepEqual(plan.PortsByFile["docker-compose.yml"], want) {
		t.Errorf("declared %v, want %v", plan.PortsByFile["docker-compose.yml"], want)
	}
	if len(plan.Patches["docker-compose.yml"]) != 1 {
		t.Errorf("only the frozen mapping is rewritten, got %v", plan.Patches)
	}
	if len(plan.Withheld) != 0 {
		t.Errorf("nothing is withheld, got %+v", plan.Withheld)
	}
}

func TestPlanComposePortsIgnoresUnselectedFilesAndReportsUnreadableOnes(t *testing.T) {
	scans := map[string]domain.ComposeScan{
		"a.yml": scanWith("a.yml", templated("db", "DB_PORT", 5432)),
		"b.yml": scanWith("b.yml", templated("db", "OTHER_PORT", 5433)),
		"c.yml": {File: "c.yml", Err: "yaml: line 3: mapping values are not allowed"},
	}

	plan := PlanComposePorts(PlanComposePortsParams{Scans: scans, Files: []string{"a.yml", "c.yml"}})

	if _, present := plan.PortsByFile["b.yml"]; present {
		t.Error("a file the user did not select must contribute nothing")
	}
	if len(plan.Unreadable) != 1 || plan.Unreadable[0].File != "c.yml" {
		t.Errorf("unreadable = %+v, want c.yml", plan.Unreadable)
	}
}

func dockerJob(name, file string, ports map[string]int) domain.JobConfig {
	return domain.JobConfig{
		Name:  name,
		Kind:  domain.JobKindService,
		Cmd:   "docker compose " + DockerComposeFileFlag(file) + "up -d",
		Stop:  "docker compose " + DockerComposeFileFlag(file) + "down --remove-orphans",
		Cwd:   ".",
		Ports: ports,
	}
}

func TestBackfillDockerPortsFillsAJobThatHasNone(t *testing.T) {
	cfg := domain.RunConfig{Jobs: []domain.JobConfig{
		dockerJob("docker-compose", "docker-compose.yml", nil),
		{Name: "web", Kind: domain.JobKindService, Cmd: "pnpm dev"},
	}}

	got := BackfillDockerPorts(BackfillDockerPortsParams{
		Config:      cfg,
		PortsByFile: map[string]map[string]int{"docker-compose.yml": {"DB_PORT": 5432, "REDIS_PORT": 6379}},
	})

	want := map[string]int{"DB_PORT": 5432, "REDIS_PORT": 6379}
	if !reflect.DeepEqual(got.Config.Jobs[0].Ports, want) {
		t.Errorf("ports = %v, want %v", got.Config.Jobs[0].Ports, want)
	}
	if got.Config.Jobs[1].Ports != nil {
		t.Errorf("a job that runs no compose file is untouched, got %v", got.Config.Jobs[1].Ports)
	}
	if !reflect.DeepEqual(got.Added["docker-compose"], want) {
		t.Errorf("added = %v, want %v", got.Added, want)
	}
}

func TestBackfillDockerPortsNeverOverwritesADeclaredPort(t *testing.T) {
	cfg := domain.RunConfig{Jobs: []domain.JobConfig{
		dockerJob("docker-compose", "docker-compose.yml", map[string]int{"DB_PORT": 15432}),
	}}

	got := BackfillDockerPorts(BackfillDockerPortsParams{
		Config:      cfg,
		PortsByFile: map[string]map[string]int{"docker-compose.yml": {"DB_PORT": 5432, "REDIS_PORT": 6379}},
	})

	if got.Config.Jobs[0].Ports["DB_PORT"] != 15432 {
		t.Errorf("DB_PORT = %d, want the hand-written 15432 kept", got.Config.Jobs[0].Ports["DB_PORT"])
	}
	if got.Config.Jobs[0].Ports["REDIS_PORT"] != 6379 {
		t.Error("the missing port must still be added")
	}
	if _, reported := got.Added["docker-compose"]["DB_PORT"]; reported {
		t.Error("a port that was already declared was not added")
	}
}

func TestBackfillDockerPortsLeavesTheInputConfigAlone(t *testing.T) {
	original := map[string]int{"DB_PORT": 5432}
	cfg := domain.RunConfig{Jobs: []domain.JobConfig{dockerJob("docker-compose", "docker-compose.yml", original)}}

	BackfillDockerPorts(BackfillDockerPortsParams{
		Config:      cfg,
		PortsByFile: map[string]map[string]int{"docker-compose.yml": {"REDIS_PORT": 6379}},
	})

	if !reflect.DeepEqual(original, map[string]int{"DB_PORT": 5432}) {
		t.Errorf("the caller's ports map was mutated: %v", original)
	}
}

func TestPruneCollidingPortsDropsTheDetectedSide(t *testing.T) {
	cfg := domain.RunConfig{Jobs: []domain.JobConfig{
		{Name: "web", Cmd: "pnpm dev", Ports: map[string]int{"PORT": 3000}},
		{Name: "docker", Cmd: "docker compose up -d", Ports: map[string]int{"ADMIN_PORT": 3010}},
	}}

	got := PruneCollidingPorts(PruneCollidingPortsParams{
		Config:   cfg,
		Detected: map[string]map[string]int{"docker": {"ADMIN_PORT": 3010}},
	})

	if got.Config.Jobs[0].Ports["PORT"] != 3000 {
		t.Error("the hand-written declaration must survive")
	}
	if got.Config.Jobs[1].Ports != nil {
		t.Errorf("the detected declaration must be dropped, got %v", got.Config.Jobs[1].Ports)
	}
	if len(got.Dropped) != 1 || got.Dropped[0].Port.Name != "ADMIN_PORT" || got.Dropped[0].Against.Name != "PORT" {
		t.Fatalf("dropped = %+v", got.Dropped)
	}
	if got.Dropped[0].Worktrees != 1 {
		t.Errorf("worktrees apart = %d, want 1", got.Dropped[0].Worktrees)
	}
}

func TestPruneCollidingPortsDropsBothWhenBothAreDetected(t *testing.T) {
	cfg := domain.RunConfig{Jobs: []domain.JobConfig{
		{Name: "a", Cmd: "x", Ports: map[string]int{"A_PORT": 5432}},
		{Name: "b", Cmd: "y", Ports: map[string]int{"B_PORT": 5432}},
	}}

	got := PruneCollidingPorts(PruneCollidingPortsParams{
		Config: cfg,
		Detected: map[string]map[string]int{
			"a": {"A_PORT": 5432},
			"b": {"B_PORT": 5432},
		},
	})

	if got.Config.Jobs[0].Ports != nil || got.Config.Jobs[1].Ports != nil {
		t.Errorf("two detected bases that cannot coexist are both dropped, got %+v", got.Config.Jobs)
	}
	if len(got.Dropped) != 2 {
		t.Errorf("both must be reported, got %+v", got.Dropped)
	}
}

func TestPruneCollidingPortsLeavesAValidLayoutAlone(t *testing.T) {
	cfg := domain.RunConfig{Jobs: []domain.JobConfig{
		{Name: "db", Cmd: "x", Ports: map[string]int{"P1": 5434, "P2": 5435, "P3": 5436}},
	}}

	got := PruneCollidingPorts(PruneCollidingPortsParams{
		Config:   cfg,
		Detected: map[string]map[string]int{"db": {"P1": 5434, "P2": 5435, "P3": 5436}},
	})

	if len(got.Dropped) != 0 {
		t.Errorf("bases one apart never meet, got %+v", got.Dropped)
	}
	if len(got.Config.Jobs[0].Ports) != 3 {
		t.Errorf("ports = %v", got.Config.Jobs[0].Ports)
	}
}

func TestPruneCollidingPortsNeverDropsAHandWrittenPair(t *testing.T) {
	cfg := domain.RunConfig{Jobs: []domain.JobConfig{
		{Name: "a", Cmd: "x", Ports: map[string]int{"A_PORT": 3000}},
		{Name: "b", Cmd: "y", Ports: map[string]int{"B_PORT": 3010}},
	}}

	got := PruneCollidingPorts(PruneCollidingPortsParams{Config: cfg})

	if len(got.Dropped) != 0 {
		t.Errorf("wtm never edits a conflict the user wrote themselves, got %+v", got.Dropped)
	}
}

func TestPruneCollidingPortsProducesALoadableConfig(t *testing.T) {
	cfg := domain.RunConfig{Jobs: []domain.JobConfig{
		{Name: "a", Kind: domain.JobKindService, Cmd: "x", Ports: map[string]int{"A_PORT": 5432}},
		{Name: "b", Kind: domain.JobKindService, Cmd: "y", Ports: map[string]int{"B_PORT": 5442}},
	}}

	if errs := ValidateRunPorts(cfg); len(errs) == 0 {
		t.Fatal("fixture must be a config ValidateRunPorts rejects")
	}

	got := PruneCollidingPorts(PruneCollidingPortsParams{
		Config:   cfg,
		Detected: map[string]map[string]int{"b": {"B_PORT": 5442}},
	})

	if errs := ValidateRunPorts(got.Config); len(errs) != 0 {
		t.Errorf("the pruned config must load, got %v", errs)
	}
}
