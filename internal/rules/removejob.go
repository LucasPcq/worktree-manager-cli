package rules

import "github.com/LucasPcq/wtm/internal/domain"

// RemoveJobEffect is what a removal touched beyond the job itself, so both
// callers report the same thing.
type RemoveJobEffect struct {
	Removed         bool
	Profiles        []string
	EmptiedProfiles []string
	EnvPorts        []string
}

// RemoveJob takes a job out of a config along with everything that named it. A
// profile left with no job goes too: one that starts nothing is not a choice
// `run up` can offer.
func RemoveJob(cfg domain.RunConfig, name string) (domain.RunConfig, RemoveJobEffect) {
	var effect RemoveJobEffect

	out := cfg
	out.Jobs = make([]domain.JobConfig, 0, len(cfg.Jobs))
	for _, job := range cfg.Jobs {
		if job.Name == name {
			effect.Removed = true
			continue
		}
		out.Jobs = append(out.Jobs, job)
	}
	if !effect.Removed {
		return cfg, effect
	}

	out.Profiles = make([]domain.ProfileConfig, 0, len(cfg.Profiles))
	for _, profile := range cfg.Profiles {
		jobs := make([]string, 0, len(profile.Jobs))
		referenced := false
		for _, job := range profile.Jobs {
			if job == name {
				referenced = true
				continue
			}
			jobs = append(jobs, job)
		}
		if !referenced {
			out.Profiles = append(out.Profiles, profile)
			continue
		}
		effect.Profiles = append(effect.Profiles, profile.Name)
		if len(jobs) == 0 {
			effect.EmptiedProfiles = append(effect.EmptiedProfiles, profile.Name)
			continue
		}
		profile.Jobs = jobs
		out.Profiles = append(out.Profiles, profile)
	}

	out.EnvPorts = make([]domain.EnvPortLink, 0, len(cfg.EnvPorts))
	for _, link := range cfg.EnvPorts {
		if link.Job == name {
			effect.EnvPorts = append(effect.EnvPorts, link.Key)
			continue
		}
		out.EnvPorts = append(out.EnvPorts, link)
	}

	return out, effect
}
