package rules

import (
	"fmt"
	"time"

	"github.com/LucasPcq/wtm/internal/domain"
)

// IsRunInitialized reports whether the run module counts as initialized: its
// run.toml declares at least one job or profile. An absent run.toml loads as an
// empty RunConfig, so this doubles as the "run.toml missing" check. The
// not-initialized guard on run commands is built on this predicate.
func IsRunInitialized(cfg domain.RunConfig) bool {
	return len(cfg.Jobs) > 0 || len(cfg.Profiles) > 0
}

// IsDetached reports whether the job is a service with a stop command,
// meaning the launcher process exits after starting detached work
// (e.g. docker compose up -d).
func IsDetached(job domain.JobConfig) bool {
	return job.Kind == domain.JobKindService && job.Stop != ""
}

type JobUptimeParams struct {
	Job domain.JobInfo
	Now time.Time
}

// JobUptime reads how long a job has been up, and only answers for one that
// still is: on a job that stopped, StartedAt dates a run that is over, and
// letting it keep counting would read as still running. A start in the future
// (a clock stepped between the daemon and the reader) counts as zero rather
// than counting backwards.
func JobUptime(params JobUptimeParams) string {
	if params.Job.Status != domain.JobStatusRunning || params.Job.StartedAt.IsZero() {
		return ""
	}

	elapsed := max(params.Now.Sub(params.Job.StartedAt), 0)
	switch {
	case elapsed < time.Minute:
		return fmt.Sprintf(domain.JobUptimeSecFmt, int(elapsed.Seconds()))
	case elapsed < time.Hour:
		return fmt.Sprintf(domain.JobUptimeMinFmt, int(elapsed.Minutes()))
	case elapsed < 24*time.Hour:
		return fmt.Sprintf(domain.JobUptimeHourFmt, int(elapsed.Hours()), int(elapsed.Minutes())%60)
	default:
		return fmt.Sprintf(domain.JobUptimeDayFmt, int(elapsed.Hours())/24, int(elapsed.Hours())%24)
	}
}

// DefaultProfile returns the profile marked as default, or the first one.
func DefaultProfile(cfg domain.RunConfig) (domain.ProfileConfig, bool) {
	for _, p := range cfg.Profiles {
		if p.Default {
			return p, true
		}
	}
	if len(cfg.Profiles) > 0 {
		return cfg.Profiles[0], true
	}
	return domain.ProfileConfig{}, false
}

// FindProfile returns a profile by name.
func FindProfile(cfg domain.RunConfig, name string) (domain.ProfileConfig, bool) {
	for _, p := range cfg.Profiles {
		if p.Name == name {
			return p, true
		}
	}
	return domain.ProfileConfig{}, false
}

// FindJob returns a job by name.
func FindJob(cfg domain.RunConfig, name string) (domain.JobConfig, bool) {
	for _, j := range cfg.Jobs {
		if j.Name == name {
			return j, true
		}
	}
	return domain.JobConfig{}, false
}

// ProfileJobs returns the JobConfig list for a profile, preserving the
// declared order.
func ProfileJobs(cfg domain.RunConfig, profile domain.ProfileConfig) []domain.JobConfig {
	jobs := make([]domain.JobConfig, 0, len(profile.Jobs))
	for _, name := range profile.Jobs {
		if j, ok := FindJob(cfg, name); ok {
			jobs = append(jobs, j)
		}
	}
	return jobs
}

// FilterToProfile returns a new RunConfig containing only the named profile
// and the jobs it references. Returns an error if the profile is not found.
func FilterToProfile(cfg domain.RunConfig, name string) (domain.RunConfig, error) {
	p, ok := FindProfile(cfg, name)
	if !ok {
		return domain.RunConfig{}, fmt.Errorf("profile %q not found", name)
	}
	return domain.RunConfig{
		Jobs:     ProfileJobs(cfg, p),
		Profiles: []domain.ProfileConfig{p},
	}, nil
}

// FindExistingDefaultProfile returns the name of the profile currently marked
// default, excluding `exclude`. Returns "" if no other default exists.
func FindExistingDefaultProfile(cfg domain.RunConfig, exclude string) string {
	for _, p := range cfg.Profiles {
		if p.Default && p.Name != exclude {
			return p.Name
		}
	}
	return ""
}

// ApplyDefaultOverride returns a copy of cfg where every profile other than
// keepName has Default=false. Pure — input is not mutated. Use after a wizard
// run when the user (re)confirmed a new default to prevent ValidateRun from
// rejecting two defaults.
func ApplyDefaultOverride(cfg domain.RunConfig, keepName string) domain.RunConfig {
	out := domain.RunConfig{
		Jobs:     cfg.Jobs,
		Profiles: make([]domain.ProfileConfig, len(cfg.Profiles)),
	}
	copy(out.Profiles, cfg.Profiles)
	for i, p := range out.Profiles {
		if p.Default && p.Name != keepName {
			out.Profiles[i].Default = false
		}
	}
	return out
}

// MergeResult summarizes what happened during a MergeRunConfigs call.
type MergeResult struct {
	Added   []string
	Skipped []string
}

// MergeRunConfigs merges src into dst. Jobs and profiles in src that share a
// name with an existing entry in dst are skipped (names go into Skipped); new
// entries are appended (names go into Added). dst is never mutated.
func MergeRunConfigs(dst, src domain.RunConfig) (domain.RunConfig, MergeResult) {
	out := domain.RunConfig{
		Jobs:     make([]domain.JobConfig, len(dst.Jobs)),
		Profiles: make([]domain.ProfileConfig, len(dst.Profiles)),
	}
	copy(out.Jobs, dst.Jobs)
	copy(out.Profiles, dst.Profiles)

	existingJobs := make(map[string]bool, len(dst.Jobs))
	for _, j := range dst.Jobs {
		existingJobs[j.Name] = true
	}
	existingProfiles := make(map[string]bool, len(dst.Profiles))
	for _, p := range dst.Profiles {
		existingProfiles[p.Name] = true
	}

	var result MergeResult

	for _, j := range src.Jobs {
		if existingJobs[j.Name] {
			result.Skipped = append(result.Skipped, j.Name)
			continue
		}
		out.Jobs = append(out.Jobs, j)
		result.Added = append(result.Added, j.Name)
		existingJobs[j.Name] = true
	}

	for _, p := range src.Profiles {
		if existingProfiles[p.Name] {
			result.Skipped = append(result.Skipped, p.Name)
			continue
		}
		out.Profiles = append(out.Profiles, p)
		result.Added = append(result.Added, p.Name)
		existingProfiles[p.Name] = true
	}

	return out, result
}
