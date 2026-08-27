package rules

import (
	"testing"
	"time"

	"github.com/LucasPcq/wtm/internal/domain"
)

func TestJobUptime(t *testing.T) {
	now := time.Date(2026, 8, 20, 12, 0, 0, 0, time.UTC)
	running := func(startedAt time.Time) domain.JobInfo {
		return domain.JobInfo{Name: "dev", Status: domain.JobStatusRunning, StartedAt: startedAt}
	}
	tests := []struct {
		name string
		job  domain.JobInfo
		want string
	}{
		{"42 s", running(now.Add(-42 * time.Second)), "42s"},
		{"une minute pile", running(now.Add(-time.Minute)), "1m"},
		{"5 min", running(now.Add(-5 * time.Minute)), "5m"},
		{"une heure pile", running(now.Add(-time.Hour)), "1h00m"},
		{"3 h 7", running(now.Add(-3*time.Hour - 7*time.Minute)), "3h07m"},
		{"2 j 5 h", running(now.Add(-53 * time.Hour)), "2d05h"},
		{"démarrage dans le futur", running(now.Add(time.Hour)), "0s"},
		{"jamais démarré", running(time.Time{}), ""},
		{"arrêté", domain.JobInfo{Status: domain.JobStatusStopped, StartedAt: now.Add(-time.Hour)}, ""},
		{"crashé", domain.JobInfo{Status: domain.JobStatusCrashed, StartedAt: now.Add(-time.Hour)}, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := JobUptime(JobUptimeParams{Job: tt.job, Now: now}); got != tt.want {
				t.Errorf("JobUptime() = %q, want %q", got, tt.want)
			}
		})
	}
}

// TestJobUptime_SaysNothingWithoutANow pins the degradation an unset Now would
// otherwise be: every row of the table reading 0s, i.e. every job just started.
func TestJobUptime_SaysNothingWithoutANow(t *testing.T) {
	job := domain.JobInfo{
		Name:      "dev",
		Status:    domain.JobStatusRunning,
		StartedAt: time.Date(2026, 8, 20, 11, 0, 0, 0, time.UTC),
	}
	if got := JobUptime(JobUptimeParams{Job: job}); got != "" {
		t.Errorf("JobUptime() = %q, want an empty column", got)
	}
}

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

func TestFindExistingDefaultProfile(t *testing.T) {
	cfg := domain.RunConfig{
		Profiles: []domain.ProfileConfig{
			{Name: "dev", Default: true},
			{Name: "ci"},
		},
	}

	if got := FindExistingDefaultProfile(cfg, ""); got != "dev" {
		t.Errorf("expected 'dev', got %q", got)
	}
	if got := FindExistingDefaultProfile(cfg, "dev"); got != "" {
		t.Errorf("expected empty when excluding 'dev', got %q", got)
	}
	if got := FindExistingDefaultProfile(domain.RunConfig{}, ""); got != "" {
		t.Errorf("expected empty for empty cfg, got %q", got)
	}
}

func TestApplyDefaultOverride(t *testing.T) {
	cfg := domain.RunConfig{
		Profiles: []domain.ProfileConfig{
			{Name: "dev", Default: true},
			{Name: "ci", Default: true},
			{Name: "staging"},
		},
	}

	out := ApplyDefaultOverride(cfg, "ci")

	for _, p := range out.Profiles {
		switch p.Name {
		case "ci":
			if !p.Default {
				t.Errorf("expected 'ci' to keep Default=true, got false")
			}
		case "dev":
			if p.Default {
				t.Errorf("expected 'dev' to be flipped to Default=false, got true")
			}
		case "staging":
			if p.Default {
				t.Errorf("expected 'staging' Default to remain false")
			}
		}
	}

	// Input must not be mutated.
	if !cfg.Profiles[0].Default {
		t.Errorf("ApplyDefaultOverride must not mutate input — original 'dev' was flipped")
	}
}

func TestApplyDefaultOverride_NoDefaultsToFlip(t *testing.T) {
	cfg := domain.RunConfig{
		Profiles: []domain.ProfileConfig{
			{Name: "dev", Default: true},
		},
	}

	out := ApplyDefaultOverride(cfg, "dev")

	if !out.Profiles[0].Default {
		t.Errorf("expected 'dev' to remain Default=true")
	}
}

func TestIsAlreadyRunning(t *testing.T) {
	tests := []struct {
		name    string
		message string
		want    bool
	}{
		{"refus du démon", "job api is already running", true},
		{"autre refus", "job api: exit status 1", false},
		{"message vide", "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsAlreadyRunning(tt.message); got != tt.want {
				t.Errorf("IsAlreadyRunning(%q) = %v, want %v", tt.message, got, tt.want)
			}
		})
	}
}

func TestMergeRunConfigsKeepsThePortOffsetBlock(t *testing.T) {
	dst := domain.RunConfig{PortOffsetBlock: 25, Jobs: []domain.JobConfig{{Name: "a", Cmd: "x"}}}
	src := domain.RunConfig{Jobs: []domain.JobConfig{{Name: "b", Cmd: "y"}}}

	got, _ := MergeRunConfigs(dst, src)
	if got.PortOffsetBlock != 25 {
		t.Errorf("block = %d; a project sets it precisely to keep its ports apart", got.PortOffsetBlock)
	}
}

// Sans profil, `run up` démarre tout ce qui est déclaré. Ne garder que les
// services supprimait en silence les migrations dont un service dépend, et le
// compteur d'étapes affichait [1/1] sans rien dire du job écarté (LUC-208).
func TestJobsWithoutProfileKeepsEveryDeclaredJob(t *testing.T) {
	cfg := domain.RunConfig{Jobs: []domain.JobConfig{
		{Name: "docker-compose", Kind: domain.JobKindService},
		{Name: "migrate", Kind: domain.JobKindTask},
		{Name: "dev", Kind: domain.JobKindService},
	}}

	got := JobsWithoutProfile(cfg)
	if len(got) != len(cfg.Jobs) {
		t.Fatalf("expected the %d declared jobs, got %d: %+v", len(cfg.Jobs), len(got), got)
	}
	for i, job := range got {
		if job.Name != cfg.Jobs[i].Name {
			t.Errorf("job %d = %s, want %s: l'ordre déclaré est l'ordre du run", i, job.Name, cfg.Jobs[i].Name)
		}
	}
}
