package rules

import (
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
)

func TestMergeRunConfigsEmpty(t *testing.T) {
	dst := domain.RunConfig{}
	src := domain.RunConfig{
		Jobs: []domain.JobConfig{
			{Name: "dev", Kind: domain.JobKindService, Cmd: "pnpm dev"},
		},
	}

	out, result := MergeRunConfigs(dst, src)

	if len(out.Jobs) != 1 {
		t.Fatalf("expected 1 job, got %d", len(out.Jobs))
	}
	if len(result.Added) != 1 || result.Added[0] != "dev" {
		t.Errorf("expected Added=[dev], got %v", result.Added)
	}
	if len(result.Skipped) != 0 {
		t.Errorf("expected no skipped, got %v", result.Skipped)
	}
}

func TestMergeRunConfigsDuplicateSkipped(t *testing.T) {
	existing := domain.JobConfig{Name: "dev", Kind: domain.JobKindService, Cmd: "pnpm dev"}
	dst := domain.RunConfig{Jobs: []domain.JobConfig{existing}}
	src := domain.RunConfig{
		Jobs: []domain.JobConfig{
			{Name: "dev", Kind: domain.JobKindService, Cmd: "vite"},      // duplicate
			{Name: "build", Kind: domain.JobKindTask, Cmd: "pnpm build"}, // new
		},
	}

	out, result := MergeRunConfigs(dst, src)

	if len(out.Jobs) != 2 {
		t.Fatalf("expected 2 jobs, got %d", len(out.Jobs))
	}
	if out.Jobs[0].Cmd != "pnpm dev" {
		t.Errorf("expected existing cmd preserved, got %q", out.Jobs[0].Cmd)
	}
	if len(result.Added) != 1 || result.Added[0] != "build" {
		t.Errorf("expected Added=[build], got %v", result.Added)
	}
	if len(result.Skipped) != 1 || result.Skipped[0] != "dev" {
		t.Errorf("expected Skipped=[dev], got %v", result.Skipped)
	}
}

func TestMergeRunConfigsDstUnmutated(t *testing.T) {
	dst := domain.RunConfig{
		Jobs: []domain.JobConfig{
			{Name: "a", Kind: domain.JobKindService, Cmd: "echo a"},
		},
	}
	src := domain.RunConfig{
		Jobs: []domain.JobConfig{
			{Name: "b", Kind: domain.JobKindTask, Cmd: "echo b"},
		},
	}

	origLen := len(dst.Jobs)
	MergeRunConfigs(dst, src)

	if len(dst.Jobs) != origLen {
		t.Errorf("dst was mutated: expected %d jobs, got %d", origLen, len(dst.Jobs))
	}
}

func TestMergeRunConfigsProfiles(t *testing.T) {
	dst := domain.RunConfig{
		Jobs: []domain.JobConfig{
			{Name: "dev", Kind: domain.JobKindService, Cmd: "pnpm dev"},
		},
		Profiles: []domain.ProfileConfig{
			{Name: "full", Jobs: []string{"dev"}, Default: true},
		},
	}
	src := domain.RunConfig{
		Jobs: []domain.JobConfig{
			{Name: "build", Kind: domain.JobKindTask, Cmd: "pnpm build"},
		},
		Profiles: []domain.ProfileConfig{
			{Name: "full", Jobs: []string{"build"}}, // duplicate
			{Name: "ci", Jobs: []string{"build"}},   // new
		},
	}

	out, result := MergeRunConfigs(dst, src)

	if len(out.Profiles) != 2 {
		t.Fatalf("expected 2 profiles, got %d", len(out.Profiles))
	}
	if out.Profiles[0].Name != "full" || !out.Profiles[0].Default {
		t.Errorf("original profile mutated: %+v", out.Profiles[0])
	}

	var skippedProfiles, addedProfiles int
	for _, s := range result.Skipped {
		if s == "full" {
			skippedProfiles++
		}
	}
	for _, a := range result.Added {
		if a == "ci" {
			addedProfiles++
		}
	}
	if skippedProfiles != 1 {
		t.Errorf("expected full to be skipped, result: %+v", result)
	}
	if addedProfiles != 1 {
		t.Errorf("expected ci to be added, result: %+v", result)
	}
}
