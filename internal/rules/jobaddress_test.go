package rules

import (
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
)

func TestWorktreeJobAddressesOffsetsEveryPortAndPublishesTheNamedOnes(t *testing.T) {
	cfg := domain.RunConfig{Jobs: []domain.JobConfig{
		{Name: "web", Ports: map[string]int{"PORT": 3000}, URL: &domain.JobURLConfig{Port: "PORT"}},
		{Name: "worker", Ports: map[string]int{"METRICS": 9100}},
	}}

	addresses := WorktreeJobAddresses(WorktreeJobAddressesParams{
		Config: cfg, PortOffset: 10, Worktree: "feat-x", Project: "wtm", PublicPort: 8080,
	})

	if got := addresses["web"].Ports; len(got) != 1 || got[0] != 3010 {
		t.Errorf("web ports = %v, want the declared port plus the worktree's offset", got)
	}
	if addresses["web"].URL == "" {
		t.Error("a job declaring a url must carry one")
	}
	if got := addresses["worker"].Ports; len(got) != 1 || got[0] != 9110 {
		t.Errorf("worker ports = %v, want the offset applied to every declared port", got)
	}
	if addresses["worker"].URL != "" {
		t.Error("a job publishing no name has no url, and inventing one would lie")
	}
}

func TestWorktreeJobAddressesSortsPortsSoTheyNeverMoveBetweenTwoReads(t *testing.T) {
	cfg := domain.RunConfig{Jobs: []domain.JobConfig{
		{Name: "web", Ports: map[string]int{"B": 4000, "A": 3000}},
	}}

	for range 20 {
		got := WorktreeJobAddresses(WorktreeJobAddressesParams{Config: cfg})["web"].Ports
		if len(got) != 2 || got[0] != 3000 || got[1] != 4000 {
			t.Fatalf("ports = %v, want them sorted", got)
		}
	}
}

func TestWorktreeJobAddressesAnswersNothingForAProjectWithNoRun(t *testing.T) {
	if got := WorktreeJobAddresses(WorktreeJobAddressesParams{}); got != nil {
		t.Fatalf("addresses = %v, want none where nothing is declared", got)
	}
}
