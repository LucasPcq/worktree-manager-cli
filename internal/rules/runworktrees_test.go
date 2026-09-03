package rules

import (
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
)

// The addresses a surface caches are read off the ports and the published url,
// so a port edited without a rename is a change like any other.
func TestSameRunJobsSeesAPortMoveWithNoRename(t *testing.T) {
	before := domain.RunConfig{Jobs: []domain.JobConfig{{Name: "web", Ports: map[string]int{"PORT": 3000}}}}
	after := domain.RunConfig{Jobs: []domain.JobConfig{{Name: "web", Ports: map[string]int{"PORT": 4000}}}}

	if SameRunJobs(before, after) {
		t.Fatal("a job binding another port is not the same job to a surface showing its address")
	}
	if !SameRunJobs(before, before) {
		t.Fatal("the same config read twice is the same config")
	}
}

func TestSameRunJobsSeesAPublishedURLAppearAndVanish(t *testing.T) {
	plain := domain.RunConfig{Jobs: []domain.JobConfig{{Name: "web"}}}
	published := domain.RunConfig{Jobs: []domain.JobConfig{{Name: "web", URL: &domain.JobURLConfig{Port: "PORT"}}}}

	if SameRunJobs(plain, published) || SameRunJobs(published, plain) {
		t.Fatal("a job that starts publishing a name changes what the panel shows")
	}
	if !SameRunJobs(published, published) {
		t.Fatal("the same published job is the same job")
	}
}

func TestBranchesWithJobsUpNamesOnlyWhatRuns(t *testing.T) {
	branches := BranchesWithJobsUp(BranchesWithJobsUpParams{
		Jobs: []domain.JobInfo{
			{Name: "web", Status: domain.JobStatusRunning, WorkDir: "/wt/b"},
			{Name: "api", Status: domain.JobStatusRunning, WorkDir: "/wt/a"},
			{Name: "old", Status: domain.JobStatusStopped, WorkDir: "/wt/idle"},
		},
		Statuses: []domain.WorktreeStatus{
			{Branch: "feat/b", Path: "/wt/b"},
			{Branch: "feat/idle", Path: "/wt/idle"},
			{Branch: "feat/a", Path: "/wt/a"},
			{Branch: "", Path: "/wt/detached"},
		},
	})

	if len(branches) != 2 {
		t.Fatalf("branches = %v, want only the two running something", branches)
	}
	if branches[0] != "feat/a" || branches[1] != "feat/b" {
		t.Errorf("branches = %v, want them sorted so two polls ask for the same thing", branches)
	}
}
