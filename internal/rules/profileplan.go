package rules

import (
	"path/filepath"
	"sort"
	"strings"

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
//
// Every job enters a profile, tasks included: a profile holding only the
// services would have `wtm run up` start them on a database no migration ever
// touched (LUC-208).
func ProposeProfiles(params ProposeProfilesParams) []domain.ProfileConfig {
	if len(params.Existing) > 0 {
		return params.Existing
	}
	if len(params.Config.Jobs) == 0 {
		return nil
	}

	var shared []domain.JobConfig
	var dirs []string
	byDir := map[string][]domain.JobConfig{}
	for _, job := range params.Config.Jobs {
		if IsSharedJob(job) {
			shared = append(shared, job)
			continue
		}
		if _, seen := byDir[job.Cwd]; !seen {
			dirs = append(dirs, job.Cwd)
		}
		byDir[job.Cwd] = append(byDir[job.Cwd], job)
	}

	global := domain.ProfileConfig{
		Name:    domain.ProfileAllName,
		Jobs:    JobNames(TasksFirst(params.Config.Jobs)),
		Default: true,
	}
	if len(dirs) <= 1 || len(dirs) > domain.ProfileProposalMaxPackages {
		return []domain.ProfileConfig{global}
	}

	sort.Strings(dirs)
	names := ProfileNamesForDirs(dirs)
	profiles := make([]domain.ProfileConfig, 0, len(dirs)+1)
	for _, dir := range dirs {
		combined := append(append([]domain.JobConfig{}, shared...), byDir[dir]...)
		profiles = append(profiles, domain.ProfileConfig{
			Name: names[dir],
			Jobs: JobNames(TasksFirst(combined)),
		})
	}
	return append(profiles, global)
}

// ProfileNamesForDirs names one profile per package directory, keyed by that
// directory. The base name alone merges apps/app-1/back with apps/app-2/back
// into a single profile carrying both applications' jobs, so a colliding name
// is widened one path segment at a time until it stands apart.
func ProfileNamesForDirs(dirs []string) map[string]string {
	names := make(map[string]string, len(dirs))
	for _, dir := range dirs {
		names[dir] = profileNameForDir(dir, dirs)
	}
	return names
}

func profileNameForDir(dir string, dirs []string) string {
	segments := dirSegments(dir)
	for take := 1; take < len(segments); take++ {
		suffix := segments[len(segments)-take:]
		if uniqueSuffix(suffix, dir, dirs) {
			return strings.Join(suffix, domain.ProfileNameSegmentSep)
		}
	}
	return strings.Join(segments, domain.ProfileNameSegmentSep)
}

func uniqueSuffix(suffix []string, owner string, dirs []string) bool {
	for _, other := range dirs {
		if other == owner {
			continue
		}
		if hasSuffixSegments(dirSegments(other), suffix) {
			return false
		}
	}
	return true
}

func hasSuffixSegments(segments, suffix []string) bool {
	if len(suffix) > len(segments) {
		return false
	}
	for i := range suffix {
		if segments[len(segments)-len(suffix)+i] != suffix[i] {
			return false
		}
	}
	return true
}

func dirSegments(dir string) []string {
	cleaned := filepath.ToSlash(filepath.Clean(dir))
	var segments []string
	for _, segment := range strings.Split(cleaned, "/") {
		if segment != "" && segment != "." {
			segments = append(segments, segment)
		}
	}
	if len(segments) == 0 {
		return []string{domain.ProfileAllName}
	}
	return segments
}

type ApplyInitAnswersParams struct {
	Config   domain.RunConfig
	Ports    []domain.PortEntry
	Profiles []domain.ProfileConfig
	Cmds     []domain.JobCmdFix
}

// ApplyInitAnswers folds the wizard's two composition steps back into the
// config about to be written. Neither is inferred: a port the user corrected
// and a split they composed outrank what detection proposed.
func ApplyInitAnswers(params ApplyInitAnswersParams) domain.RunConfig {
	cfg := params.Config
	cfg.Jobs = make([]domain.JobConfig, len(params.Config.Jobs))
	copy(cfg.Jobs, params.Config.Jobs)

	// A zero base is an answer too: the user was offered the port and declined
	// it. Writing it would declare a port nothing binds.
	for _, entry := range params.Ports {
		if entry.Base <= 0 {
			continue
		}
		for i, job := range cfg.Jobs {
			if job.Name != entry.Job {
				continue
			}
			cfg.Jobs[i].Ports = clonePorts(cfg.Jobs[i].Ports)
			cfg.Jobs[i].Ports[entry.Name] = entry.Base
		}
	}

	for _, fix := range params.Cmds {
		for i, job := range cfg.Jobs {
			if job.Name == fix.Job && fix.Cmd != "" {
				cfg.Jobs[i].Cmd = fix.Cmd
			}
		}
	}

	if len(params.Profiles) > 0 {
		cfg.Profiles = params.Profiles
	}
	return cfg
}
