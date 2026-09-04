package up

import (
	"errors"
	"path/filepath"
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/flow"
	"github.com/LucasPcq/wtm/internal/flow/run/target"
	"github.com/LucasPcq/wtm/internal/testutil/flowtest"
	"github.com/LucasPcq/wtm/internal/testutil/gittest"
)

// A flag that contradicts itself is not a decision to default: --exclusive
// stops all but one, and the run was told to bring up two.
func TestExclusiveIsRefusedOnSeveralWorktrees(t *testing.T) {
	repo := gittest.InitRepo(t)
	second := filepath.Join(t.TempDir(), "feature")
	gittest.Git(t, repo, "worktree", "add", "-b", "feature", second)

	_, err := Run(Params{
		Context:   flow.Context{ProjectDir: repo},
		Request:   Request{Worktrees: []string{"main", "feature"}, Cwd: repo, Exclusive: true},
		Prompter:  flow.Unattended{},
		Presenter: presenterOnly{&flowtest.Recorder{}},
	})

	if !errors.Is(err, domain.ErrExclusiveMultiWorktree) {
		t.Fatalf("err = %v, want the contradiction refused", err)
	}
}

func TestSeveralProfilesStartTheirUnionOnce(t *testing.T) {
	cfg := domain.RunConfig{
		Jobs: []domain.JobConfig{
			{Name: "db", Kind: domain.JobKindService},
			{Name: "web", Kind: domain.JobKindService},
			{Name: "api", Kind: domain.JobKindService},
		},
		Profiles: []domain.ProfileConfig{
			{Name: "front", Jobs: []string{"db", "web"}},
			{Name: "back", Jobs: []string{"db", "api"}},
		},
	}
	f := &upFlow{request: Request{Config: cfg}}

	got, err := f.resolveProfile(flow.NewAnswers(nil).WithValues(target.KeyProfile, []string{"front", "back"}))
	if err != nil {
		t.Fatalf("resolveProfile: %v", err)
	}

	var names []string
	for _, job := range got.Jobs {
		names = append(names, job.Name)
	}
	if len(names) != 3 {
		t.Fatalf("jobs = %v, want db started once and web, api after it", names)
	}
	if names[0] != "db" || names[1] != "web" || names[2] != "api" {
		t.Errorf("jobs = %v, want the profiles' order preserved", names)
	}
	if got.Name != "front, back" {
		t.Errorf("Name = %q, want both profiles named", got.Name)
	}
}

func TestOneProfileIsUnchangedBySeveralProfileSupport(t *testing.T) {
	cfg := domain.RunConfig{
		Jobs:     []domain.JobConfig{{Name: "web", Kind: domain.JobKindService}},
		Profiles: []domain.ProfileConfig{{Name: "front", Jobs: []string{"web"}}},
	}
	f := &upFlow{request: Request{Config: cfg}}

	got, err := f.resolveProfile(flow.NewAnswers(nil).WithValues(target.KeyProfile, []string{"front"}))
	if err != nil {
		t.Fatalf("resolveProfile: %v", err)
	}
	if got.Name != "front" || len(got.Jobs) != 1 {
		t.Errorf("got %+v, want the single-profile shape untouched", got)
	}
}
