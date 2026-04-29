package rules

import (
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
)

func TestJobIsDetached(t *testing.T) {
	tests := []struct {
		name string
		job  domain.JobConfig
		want bool
	}{
		{"service no stop", domain.JobConfig{Kind: domain.JobKindService, Cmd: "pnpm dev"}, false},
		{"service with stop", domain.JobConfig{Kind: domain.JobKindService, Cmd: "docker compose up -d", Stop: "docker compose down"}, true},
		{"task no stop", domain.JobConfig{Kind: domain.JobKindTask, Cmd: "pnpm migrate"}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsDetached(tt.job); got != tt.want {
				t.Errorf("IsDetached() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestRunConfigDefaultProfile(t *testing.T) {
	cfg := domain.RunConfig{
		Profiles: []domain.ProfileConfig{
			{Name: "back"},
			{Name: "full", Default: true},
			{Name: "front"},
		},
	}
	p, ok := DefaultProfile(cfg)
	if !ok || p.Name != "full" {
		t.Errorf("expected default profile 'full', got %q (ok=%v)", p.Name, ok)
	}
}

func TestRunConfigDefaultProfileFallback(t *testing.T) {
	cfg := domain.RunConfig{
		Profiles: []domain.ProfileConfig{{Name: "only"}},
	}
	p, ok := DefaultProfile(cfg)
	if !ok || p.Name != "only" {
		t.Errorf("expected fallback to first profile, got %q (ok=%v)", p.Name, ok)
	}
}

func TestProfileJobsPreservesOrder(t *testing.T) {
	cfg := domain.RunConfig{
		Jobs: []domain.JobConfig{
			{Name: "a", Kind: domain.JobKindService, Cmd: "a"},
			{Name: "b", Kind: domain.JobKindTask, Cmd: "b"},
			{Name: "c", Kind: domain.JobKindService, Cmd: "c"},
		},
	}
	profile := domain.ProfileConfig{Jobs: []string{"c", "a", "b"}}
	got := ProfileJobs(cfg, profile)
	if len(got) != 3 || got[0].Name != "c" || got[1].Name != "a" || got[2].Name != "b" {
		t.Errorf("expected order [c, a, b], got %v", jobNames(got))
	}
}

func jobNames(jobs []domain.JobConfig) []string {
	out := make([]string, len(jobs))
	for i, j := range jobs {
		out[i] = j.Name
	}
	return out
}
