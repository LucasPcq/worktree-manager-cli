package rules

import (
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
)

func TestEffectivePortOffsetBlock(t *testing.T) {
	tests := []struct {
		name string
		cfg  domain.RunConfig
		want int
	}{
		{"unset falls back to the default", domain.RunConfig{}, domain.PortOffsetBlock},
		{"zero is not a block of nothing", domain.RunConfig{PortOffsetBlock: 0}, domain.PortOffsetBlock},
		{"declared wins", domain.RunConfig{PortOffsetBlock: 100}, 100},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := EffectivePortOffsetBlock(tt.cfg); got != tt.want {
				t.Errorf("got %d, want %d", got, tt.want)
			}
		})
	}
}

func TestJobPorts(t *testing.T) {
	ports := map[string]int{"PORT": 3000, "DB_PORT": 5432}

	main := JobPorts(JobPortsParams{Ports: ports, PortOffset: 0})
	if main["PORT"] != 3000 || main["DB_PORT"] != 5432 {
		t.Errorf("main checkout must keep the declared bases, got %v", main)
	}

	second := JobPorts(JobPortsParams{Ports: ports, PortOffset: 10})
	if second["PORT"] != 3010 || second["DB_PORT"] != 5442 {
		t.Errorf("got %v, want PORT=3010 DB_PORT=5442", second)
	}
}

func TestJobPortsNoDeclaration(t *testing.T) {
	if got := JobPorts(JobPortsParams{PortOffset: 10}); got != nil {
		t.Errorf("a job declaring nothing gets nothing, got %v", got)
	}
	if got := JobPortEnv(JobPortsParams{PortOffset: 10}); got != nil {
		t.Errorf("a job declaring nothing gets nothing, got %v", got)
	}
}

func TestJobPortEnv(t *testing.T) {
	env := JobPortEnv(JobPortsParams{Ports: map[string]int{"PORT": 3000}, PortOffset: 30})
	if env["PORT"] != "3030" {
		t.Errorf("got %v, want PORT=3030", env)
	}
}

func TestIsEnvVarName(t *testing.T) {
	valid := []string{"PORT", "_PORT", "DB_PORT2", "port"}
	for _, name := range valid {
		if !IsEnvVarName(name) {
			t.Errorf("%q should be a valid name", name)
		}
	}
	invalid := []string{"", "2PORT", "DB-PORT", "DB PORT", "DB.PORT"}
	for _, name := range invalid {
		if IsEnvVarName(name) {
			t.Errorf("%q should be rejected", name)
		}
	}
}

func jobWithPorts(name string, ports map[string]int) domain.JobConfig {
	return domain.JobConfig{Name: name, Kind: domain.JobKindService, Cmd: "run", Ports: ports}
}

func TestPortCollisions(t *testing.T) {
	tests := []struct {
		name string
		cfg  domain.RunConfig
		want int
	}{
		{
			// The question this ticket set out to answer: an offset is uniform, so
			// neighbouring bases keep their spacing instead of overlapping.
			name: "three neighbouring databases are fine",
			cfg:  domain.RunConfig{Jobs: []domain.JobConfig{jobWithPorts("db", map[string]int{"A": 5434, "B": 5435, "C": 5436})}},
			want: 0,
		},
		{
			name: "a gap of exactly one block collides on the next worktree",
			cfg:  domain.RunConfig{Jobs: []domain.JobConfig{jobWithPorts("web", map[string]int{"PORT": 3000, "ADMIN": 3010})}},
			want: 1,
		},
		{
			name: "far apart on the same residue is arithmetic, not a conflict",
			cfg:  domain.RunConfig{Jobs: []domain.JobConfig{jobWithPorts("web", map[string]int{"PORT": 3000, "ALT": 8080})}},
			want: 0,
		},
		{
			name: "the same base twice collides inside one worktree",
			cfg: domain.RunConfig{Jobs: []domain.JobConfig{
				jobWithPorts("web", map[string]int{"PORT": 3000}),
				jobWithPorts("api", map[string]int{"PORT": 3000}),
			}},
			want: 1,
		},
		{
			name: "a wider block clears a gap the default rejects",
			cfg: domain.RunConfig{
				PortOffsetBlock: 100,
				Jobs:            []domain.JobConfig{jobWithPorts("web", map[string]int{"PORT": 3000, "ADMIN": 3010})},
			},
			want: 0,
		},
		{
			name: "a wider block creates gaps the default cleared",
			cfg: domain.RunConfig{
				PortOffsetBlock: 100,
				Jobs:            []domain.JobConfig{jobWithPorts("web", map[string]int{"PORT": 3000, "ADMIN": 3100})},
			},
			want: 1,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := PortCollisions(tt.cfg); len(got) != tt.want {
				t.Errorf("got %d collisions %v, want %d", len(got), got, tt.want)
			}
		})
	}
}

func TestPortCollisionsHorizon(t *testing.T) {
	block := domain.PortOffsetBlock
	inside := domain.RunConfig{Jobs: []domain.JobConfig{
		jobWithPorts("web", map[string]int{"A": 3000, "B": 3000 + block*domain.PortCollisionHorizon}),
	}}
	if got := PortCollisions(inside); len(got) != 1 {
		t.Errorf("a pair meeting at the horizon must be reported, got %v", got)
	}

	outside := domain.RunConfig{Jobs: []domain.JobConfig{
		jobWithPorts("web", map[string]int{"A": 3000, "B": 3000 + block*(domain.PortCollisionHorizon+1)}),
	}}
	if got := PortCollisions(outside); len(got) != 0 {
		t.Errorf("a pair meeting past the horizon must be left alone, got %v", got)
	}
}

func TestPortCollisionNamesBothSides(t *testing.T) {
	cfg := domain.RunConfig{Jobs: []domain.JobConfig{
		jobWithPorts("web", map[string]int{"PORT": 3000}),
		jobWithPorts("api", map[string]int{"ADMIN": 3010}),
	}}
	collisions := PortCollisions(cfg)
	if len(collisions) != 1 {
		t.Fatalf("got %d collisions, want 1", len(collisions))
	}
	c := collisions[0]
	if c.A.Job != "web" || c.A.Name != "PORT" || c.B.Job != "api" || c.B.Name != "ADMIN" {
		t.Errorf("both sides must be named, got %+v", c)
	}
	if c.Worktrees != 1 {
		t.Errorf("got %d worktrees apart, want 1", c.Worktrees)
	}
}

func TestPortDeclarationsAreStable(t *testing.T) {
	cfg := domain.RunConfig{Jobs: []domain.JobConfig{
		jobWithPorts("web", map[string]int{"PORT": 3000, "ADMIN": 9000, "DEBUG": 9229}),
		jobWithPorts("api", map[string]int{"API_PORT": 8080}),
	}}
	want := []PortDeclaration{
		{Job: "web", Name: "ADMIN", Base: 9000},
		{Job: "web", Name: "DEBUG", Base: 9229},
		{Job: "web", Name: "PORT", Base: 3000},
		{Job: "api", Name: "API_PORT", Base: 8080},
	}
	for range 20 {
		got := PortDeclarations(cfg)
		if len(got) != len(want) {
			t.Fatalf("got %d declarations, want %d", len(got), len(want))
		}
		for i := range want {
			if got[i] != want[i] {
				t.Fatalf("declaration %d: got %+v, want %+v", i, got[i], want[i])
			}
		}
	}
}
