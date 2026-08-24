package rules

import (
	"path/filepath"
	"sort"

	"github.com/LucasPcq/wtm/internal/domain"
)

// IsSharedJob says whether a job serves every profile. A job whose cwd is the
// repository root — a compose stack, a root script — is infrastructure the
// packages sit on, so starting one package alone still needs it.
func IsSharedJob(job domain.JobConfig) bool {
	return job.Cwd == "" || job.Cwd == domain.ProfileRootCwd
}

type ProposeProfilesParams struct {
	Config domain.RunConfig
	// Existing is the split already in run.toml. Non-empty wins whole: a
	// composition the user made by hand is an answer, not a starting point to
	// infer over.
	Existing []domain.ProfileConfig
}

// ProposeProfiles suggests a split for the wizard to edit. It decides nothing
// final — the grouping is an intention, and two repos with the same directory
// shape can want opposite groupings.
func ProposeProfiles(params ProposeProfilesParams) []domain.ProfileConfig {
	if len(params.Existing) > 0 {
		return params.Existing
	}

	var shared, all []string
	byPackage := map[string][]string{}
	for _, job := range params.Config.Jobs {
		if job.Kind != domain.JobKindService {
			continue
		}
		all = append(all, job.Name)
		if IsSharedJob(job) {
			shared = append(shared, job.Name)
			continue
		}
		pkg := filepath.Base(job.Cwd)
		byPackage[pkg] = append(byPackage[pkg], job.Name)
	}

	if len(all) == 0 {
		return nil
	}

	global := domain.ProfileConfig{Name: domain.ProfileAllName, Jobs: all, Default: true}
	if len(byPackage) <= 1 {
		return []domain.ProfileConfig{global}
	}

	packages := make([]string, 0, len(byPackage))
	for pkg := range byPackage {
		packages = append(packages, pkg)
	}
	sort.Strings(packages)

	profiles := make([]domain.ProfileConfig, 0, len(packages)+1)
	for _, pkg := range packages {
		profiles = append(profiles, domain.ProfileConfig{
			Name: pkg,
			Jobs: append(append([]string{}, shared...), byPackage[pkg]...),
		})
	}
	return append(profiles, global)
}
