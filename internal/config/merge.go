package config

import "github.com/LucasPcq/wtm/internal/domain"

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
