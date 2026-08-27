package run

import (
	"errors"
	"strconv"
	"strings"
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
)

func published(name string, base int, host string) domain.JobConfig {
	return domain.JobConfig{
		Name:  name,
		Kind:  domain.JobKindService,
		Cmd:   "pnpm dev --port ${PORT}",
		Ports: map[string]int{"PORT": base},
		URL:   &domain.JobURLConfig{Port: "PORT", Host: host},
	}
}

func TestRunURLPrintsTheOnlyURL(t *testing.T) {
	stateDir := setupTestProject(t)
	writeRunTOML(t, stateDir, domain.RunConfig{Jobs: []domain.JobConfig{
		published("web", 3000, ""),
		{Name: "db", Kind: domain.JobKindService, Cmd: "docker compose up", Ports: map[string]int{"PG_PORT": 5432}},
	}})
	fakeTTY(t, false)

	stdout, _, err := runCmd(t, domain.CmdURL)
	if err != nil {
		t.Fatalf("run url: %v", err)
	}
	// The port carries the worktree's offset, so the line is asserted by shape:
	// what matters here is that stdout holds the URL and nothing else.
	if !strings.HasPrefix(stdout, "http://localhost:") || strings.Count(stdout, "\n") != 1 {
		t.Errorf("stdout = %q, want the bare URL and nothing else", stdout)
	}
}

func TestRunURLNamesTheJobWhenAmbiguous(t *testing.T) {
	stateDir := setupTestProject(t)
	writeRunTOML(t, stateDir, domain.RunConfig{Jobs: []domain.JobConfig{
		published("web", 3000, ""),
		published("api", 4000, ""),
	}})
	fakeTTY(t, false)

	_, _, err := runCmd(t, domain.CmdURL)
	if !errors.Is(err, domain.ErrJobAmbiguous) {
		t.Fatalf("err = %v, want ErrJobAmbiguous — a machine surface never falls back to a picker", err)
	}
}

func TestRunURLNamedJobWins(t *testing.T) {
	stateDir := setupTestProject(t)
	writeRunTOML(t, stateDir, domain.RunConfig{Jobs: []domain.JobConfig{
		published("web", 3000, ""),
		published("api", 4000, ""),
	}})
	fakeTTY(t, false)

	web, _, err := runCmd(t, domain.CmdURL, "web")
	if err != nil {
		t.Fatalf("run url web: %v", err)
	}
	api, _, err := runCmd(t, domain.CmdURL, "api")
	if err != nil {
		t.Fatalf("run url api: %v", err)
	}

	// Both carry the same worktree offset, so the gap between them is the gap
	// between the bases they declared: the named job's own port, not the first.
	if portOf(t, api)-portOf(t, web) != 1000 {
		t.Errorf("web = %q, api = %q — api must answer on its own declared port", web, api)
	}
}

func portOf(t *testing.T, url string) int {
	t.Helper()
	_, port, found := strings.Cut(strings.TrimSpace(url), "http://localhost:")
	if !found {
		t.Fatalf("url %q is not a direct address", url)
	}
	n, err := strconv.Atoi(port)
	if err != nil {
		t.Fatalf("port of %q: %v", url, err)
	}
	return n
}

func TestRunURLUnknownJobNamesTheOnesThatPublish(t *testing.T) {
	stateDir := setupTestProject(t)
	writeRunTOML(t, stateDir, domain.RunConfig{Jobs: []domain.JobConfig{published("web", 3000, "")}})
	fakeTTY(t, false)

	_, _, err := runCmd(t, domain.CmdURL, "nope")
	if err == nil || !strings.Contains(err.Error(), "web") {
		t.Fatalf("err = %v, want one naming the jobs that do publish", err)
	}
}
