package rules

import (
	"fmt"

	"github.com/LucasPcq/wtm/internal/domain"
)

// IsDetached reports whether the job is a service with a stop command,
// meaning the launcher process exits after starting detached work
// (e.g. docker compose up -d).
func IsDetached(job domain.JobConfig) bool {
	return job.Kind == domain.JobKindService && job.Stop != ""
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
