package worktree

import (
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
)

func runAddressConfig() domain.RunConfig {
	return domain.RunConfig{Jobs: []domain.JobConfig{
		{Name: "web", Ports: map[string]int{"PORT": 3000}},
	}}
}

func TestRunAddressesForAnswersPerBranch(t *testing.T) {
	repo := newOrdinalRepo(t)
	repo.addWorktree(t, "feat/a")
	repo.addWorktree(t, "feat/b")

	addresses := RunAddressesFor(RunAddressesForParams{
		ProjectDir: repo.dir,
		StateDir:   repo.stateDir,
		RunConfig:  runAddressConfig(),
		Branches:   []string{"feat/a", "feat/b"},
	})

	if len(addresses) != 2 {
		t.Fatalf("branches = %d, want one entry per branch asked for", len(addresses))
	}
	a, b := addresses["feat/a"]["web"], addresses["feat/b"]["web"]
	if len(a.Ports) == 0 || len(b.Ports) == 0 {
		t.Fatalf("ports = %v / %v, want each branch its own", a.Ports, b.Ports)
	}
	if a.Ports[0] == b.Ports[0] {
		t.Errorf("both branches answer on %d, want the offset to separate them", a.Ports[0])
	}
}

func TestRunAddressesForSkipsABranchItCannotRead(t *testing.T) {
	repo := newOrdinalRepo(t)
	repo.addWorktree(t, "feat/a")

	addresses := RunAddressesFor(RunAddressesForParams{
		ProjectDir: repo.dir,
		StateDir:   repo.stateDir,
		RunConfig:  runAddressConfig(),
		Branches:   []string{"", "feat/a"},
	})

	if _, present := addresses[""]; present {
		t.Error("an unreadable branch got an entry, want it skipped rather than guessed")
	}
	if _, present := addresses["feat/a"]; !present {
		t.Error("one bad branch took the good one down with it")
	}
}

func TestRunAddressesForIsEmptyWithoutJobs(t *testing.T) {
	repo := newOrdinalRepo(t)
	repo.addWorktree(t, "feat/a")

	if got := RunAddressesFor(RunAddressesForParams{
		ProjectDir: repo.dir, StateDir: repo.stateDir, Branches: []string{"feat/a"},
	}); len(got) != 0 {
		t.Errorf("addresses = %v, want none: a project with no run module computes none", got)
	}
}
