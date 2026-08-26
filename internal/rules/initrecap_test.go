package rules

import (
	"strings"
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
)

func recapConfig() domain.RunConfig {
	return domain.RunConfig{
		Jobs: []domain.JobConfig{
			{Name: "docker-compose", Kind: domain.JobKindService, Ports: map[string]int{"POSTGRES_PORT": 5432, "REDIS_PORT": 6379}},
			{Name: "api-dev", Kind: domain.JobKindService, Cwd: "apps/api", Ports: map[string]int{"PORT": 3001}},
			{Name: "web-dev", Kind: domain.JobKindService, Cwd: "apps/web"},
			{Name: "seed", Kind: domain.JobKindTask, Cwd: "apps/api"},
		},
		Profiles: []domain.ProfileConfig{
			{Name: "api", Jobs: []string{"docker-compose", "api-dev"}},
			{Name: "all", Jobs: []string{"docker-compose", "api-dev", "web-dev"}, Default: true},
		},
	}
}

func TestRecapJobLinesShowsEachJobsPorts(t *testing.T) {
	got := strings.Join(RecapJobLines(recapConfig()), "\n")

	for _, want := range []string{"POSTGRES_PORT 5432", "REDIS_PORT 6379", "PORT 3001"} {
		if !strings.Contains(got, want) {
			t.Errorf("the recap must show the port, not a count — missing %q:\n%s", want, got)
		}
	}
}

func TestRecapJobLinesFlagsAServiceWithNoPort(t *testing.T) {
	got := strings.Join(RecapJobLines(recapConfig()), "\n")

	if !strings.Contains(got, domain.RecapNoPort) {
		t.Errorf("web-dev declares no port and the recap must say so:\n%s", got)
	}
}

func TestRecapJobLinesNamesATaskAsOne(t *testing.T) {
	got := strings.Join(RecapJobLines(recapConfig()), "\n")

	if !strings.Contains(got, domain.RecapTask) {
		t.Errorf("a task binds nothing — it must not read as a service missing a port:\n%s", got)
	}
}

func TestRecapProfileLinesShowsTheJobsAndTheDefault(t *testing.T) {
	got := strings.Join(RecapProfileLines(recapConfig()), "\n")

	if !strings.Contains(got, "docker-compose, api-dev") {
		t.Errorf("a profile must list its jobs:\n%s", got)
	}
	if !strings.Contains(got, domain.RecapDefaultSuffix) {
		t.Errorf("the default profile must be marked:\n%s", got)
	}
}

func TestRecapLinesAlignOnTheLongestName(t *testing.T) {
	lines := RecapJobLines(recapConfig())

	at := strings.Index(lines[0], "POSTGRES_PORT")
	for _, line := range lines[1:] {
		if fields := strings.Fields(line); len(fields) > 1 && strings.Index(line, fields[1]) != at {
			t.Errorf("column drifted:\n%s", strings.Join(lines, "\n"))
			break
		}
	}
}
