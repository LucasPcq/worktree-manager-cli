package rules_test

import (
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/rules"
)

func TestDiagnosePortProbes(t *testing.T) {
	tests := []struct {
		name       string
		resolved   map[string]int
		listening  map[int]bool
		offset     int
		wantStatus map[string]domain.PortProbeStatus
		wantBase   map[string]int
	}{
		{
			name:       "un port qui répond est vert",
			resolved:   map[string]int{"API_PORT": 3011},
			listening:  map[int]bool{3011: true},
			offset:     10,
			wantStatus: map[string]domain.PortProbeStatus{"API_PORT": domain.PortListening},
		},
		{
			name:       "un port muet est signalé",
			resolved:   map[string]int{"API_PORT": 3011},
			listening:  map[int]bool{},
			offset:     10,
			wantStatus: map[string]domain.PortProbeStatus{"API_PORT": domain.PortSilent},
		},
		{
			name:       "un port muet dont la base répond porte l'indice",
			resolved:   map[string]int{"WEB_PORT": 5183},
			listening:  map[int]bool{5173: true},
			offset:     10,
			wantStatus: map[string]domain.PortProbeStatus{"WEB_PORT": domain.PortSilent},
			wantBase:   map[string]int{"WEB_PORT": 5173},
		},
		{
			name:      "à offset 0 la base est le port résolu, donc aucun indice",
			resolved:  map[string]int{"WEB_PORT": 5173},
			listening: map[int]bool{},
			offset:    0,
			// Sur le checkout principal base et résolu se confondent : dire « la
			// base écoute » serait dire « le port écoute », ce qui est faux.
			wantStatus: map[string]domain.PortProbeStatus{"WEB_PORT": domain.PortSilent},
			wantBase:   map[string]int{},
		},
		{
			name:       "aucun port déclaré ne produit aucun verdict",
			resolved:   nil,
			listening:  map[int]bool{},
			offset:     10,
			wantStatus: map[string]domain.PortProbeStatus{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			probes := rules.DiagnosePortProbes(rules.DiagnosePortProbesParams{
				Job:       "web-dev",
				Resolved:  tt.resolved,
				Listening: tt.listening,
				Offset:    tt.offset,
			})

			if len(probes) != len(tt.wantStatus) {
				t.Fatalf("got %d probes, want %d: %+v", len(probes), len(tt.wantStatus), probes)
			}
			for _, p := range probes {
				if p.Job != "web-dev" {
					t.Errorf("probe %s lost its job: %q", p.Name, p.Job)
				}
				if want := tt.wantStatus[p.Name]; p.Status != want {
					t.Errorf("%s status = %s, want %s", p.Name, p.Status, want)
				}
				if want, expected := tt.wantBase[p.Name], tt.wantBase != nil; expected && p.BaseListening != want {
					t.Errorf("%s BaseListening = %d, want %d", p.Name, p.BaseListening, want)
				}
			}
		})
	}
}

func TestDiagnosePortProbesIsOrdered(t *testing.T) {
	// Two runs of the same input must report in the same order, or the recap
	// reshuffles between invocations for no reason.
	params := rules.DiagnosePortProbesParams{
		Job:       "docker-compose",
		Resolved:  map[string]int{"REDIS_PORT": 6389, "POSTGRES_PORT": 5442, "API_PORT": 3011},
		Listening: map[int]bool{},
		Offset:    10,
	}

	first := rules.DiagnosePortProbes(params)
	for range 5 {
		again := rules.DiagnosePortProbes(params)
		for i := range first {
			if first[i].Name != again[i].Name {
				t.Fatalf("order drifted: %s vs %s", first[i].Name, again[i].Name)
			}
		}
	}
	if first[0].Name != "API_PORT" {
		t.Errorf("expected alphabetical order, got %s first", first[0].Name)
	}
}

func TestPortsToProbe(t *testing.T) {
	t.Run("un service qui déclare des ports est sondé", func(t *testing.T) {
		if !rules.ShouldProbeJob(domain.JobKindService, map[string]int{"PORT": 3000}) {
			t.Error("a service declaring ports must be probed")
		}
	})
	t.Run("une task n'écoute pas", func(t *testing.T) {
		if rules.ShouldProbeJob(domain.JobKindTask, map[string]int{"PORT": 3000}) {
			t.Error("a task must never be probed")
		}
	})
	t.Run("un service sans port déclaré n'a rien à vérifier", func(t *testing.T) {
		if rules.ShouldProbeJob(domain.JobKindService, nil) {
			t.Error("a service declaring no port must not be probed")
		}
	})
}

func TestPortProbeLines(t *testing.T) {
	lines := rules.PortProbeLines([]domain.PortProbe{
		{Job: "web-dev", Name: "WEB_PORT", Port: 5183, Status: domain.PortSilent, BaseListening: 5173},
	})
	if len(lines) == 0 {
		t.Fatal("a silent port must produce at least one line")
	}
	joined := ""
	for _, l := range lines {
		joined += l + "\n"
	}
	for _, want := range []string{"WEB_PORT", "5183", "5173"} {
		if !contains(joined, want) {
			t.Errorf("lines are missing %q:\n%s", want, joined)
		}
	}
}

func TestPortProbeLinesSaysNothingWhenEverythingListens(t *testing.T) {
	lines := rules.PortProbeLines([]domain.PortProbe{
		{Job: "api-dev", Name: "API_PORT", Port: 3011, Status: domain.PortListening},
	})
	if len(lines) != 0 {
		t.Errorf("a healthy probe has nothing to report, got %v", lines)
	}
}

func TestServicesWithoutPorts(t *testing.T) {
	cfg := domain.RunConfig{Jobs: []domain.JobConfig{
		{Name: "api-dev", Kind: domain.JobKindService, Ports: map[string]int{"API_PORT": 3001}},
		{Name: "web-dev", Kind: domain.JobKindService},
		{Name: "lint", Kind: domain.JobKindTask},
	}}

	got := rules.ServicesWithoutPorts(cfg)
	if len(got) != 1 || got[0] != "web-dev" {
		t.Errorf("ServicesWithoutPorts = %v, want [web-dev]", got)
	}
}

func TestServicesWithoutPortsIgnoresTasks(t *testing.T) {
	// Une task n'écoute pas : l'absence de port n'y est pas une lacune.
	cfg := domain.RunConfig{Jobs: []domain.JobConfig{{Name: "seed", Kind: domain.JobKindTask}}}
	if got := rules.ServicesWithoutPorts(cfg); len(got) != 0 {
		t.Errorf("ServicesWithoutPorts = %v, want empty", got)
	}
}

func TestPortEntriesFor(t *testing.T) {
	cfg := domain.RunConfig{Jobs: []domain.JobConfig{
		{Name: "docker-compose", Kind: domain.JobKindService, Ports: map[string]int{"REDIS_PORT": 6379, "POSTGRES_PORT": 5432}},
		{Name: "web-dev", Kind: domain.JobKindService},
		{Name: "seed", Kind: domain.JobKindTask, Ports: map[string]int{"SEED_PORT": 9000}},
	}}

	got := rules.PortEntriesFor(rules.PortEntriesForParams{Config: cfg, ComposeJobs: []string{"docker-compose"}})

	if len(got) != 3 {
		t.Fatalf("expected the 2 compose ports and the undeclared service, got %+v", got)
	}
	// Ordre stable : le job dans l'ordre déclaré, les ports par nom.
	if got[0].Name != "POSTGRES_PORT" || got[1].Name != "REDIS_PORT" {
		t.Errorf("order = %s, %s", got[0].Name, got[1].Name)
	}
	if got[0].Job != "docker-compose" || got[0].Base != 5432 {
		t.Errorf("entry = %+v", got[0])
	}
}

func TestPortEntriesForSkipsATask(t *testing.T) {
	// Une task n'écoute pas : rien à décaler, donc rien à demander.
	cfg := domain.RunConfig{Jobs: []domain.JobConfig{
		{Name: "lint", Kind: domain.JobKindTask},
		{Name: "seed", Kind: domain.JobKindTask, Ports: map[string]int{"SEED_PORT": 9000}},
	}}
	if got := rules.PortEntriesFor(rules.PortEntriesForParams{Config: cfg}); len(got) != 0 {
		t.Errorf("PortEntriesFor = %+v, want empty", got)
	}
}
