package domain

import "testing"

func TestJobIsDetached(t *testing.T) {
	tests := []struct {
		name string
		job  JobConfig
		want bool
	}{
		{"service no stop", JobConfig{Kind: JobKindService, Cmd: "pnpm dev"}, false},
		{"service with stop", JobConfig{Kind: JobKindService, Cmd: "docker compose up -d", Stop: "docker compose down"}, true},
		{"task no stop", JobConfig{Kind: JobKindTask, Cmd: "pnpm migrate"}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.job.IsDetached(); got != tt.want {
				t.Errorf("IsDetached() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestRunConfigDefaultProfile(t *testing.T) {
	cfg := RunConfig{
		Profiles: []ProfileConfig{
			{Name: "back"},
			{Name: "full", Default: true},
			{Name: "front"},
		},
	}
	p, ok := cfg.DefaultProfile()
	if !ok || p.Name != "full" {
		t.Errorf("expected default profile 'full', got %q (ok=%v)", p.Name, ok)
	}
}

func TestRunConfigDefaultProfileFallback(t *testing.T) {
	cfg := RunConfig{
		Profiles: []ProfileConfig{{Name: "only"}},
	}
	p, ok := cfg.DefaultProfile()
	if !ok || p.Name != "only" {
		t.Errorf("expected fallback to first profile, got %q (ok=%v)", p.Name, ok)
	}
}

func TestProfileJobsPreservesOrder(t *testing.T) {
	cfg := RunConfig{
		Jobs: []JobConfig{
			{Name: "a", Kind: JobKindService, Cmd: "a"},
			{Name: "b", Kind: JobKindTask, Cmd: "b"},
			{Name: "c", Kind: JobKindService, Cmd: "c"},
		},
	}
	profile := ProfileConfig{Jobs: []string{"c", "a", "b"}}
	got := cfg.ProfileJobs(profile)
	if len(got) != 3 || got[0].Name != "c" || got[1].Name != "a" || got[2].Name != "b" {
		t.Errorf("expected order [c, a, b], got %v", jobNames(got))
	}
}

func jobNames(jobs []JobConfig) []string {
	out := make([]string, len(jobs))
	for i, j := range jobs {
		out[i] = j.Name
	}
	return out
}
