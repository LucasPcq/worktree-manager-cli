package rules

import (
	"path"
	"sort"

	"github.com/LucasPcq/wtm/internal/domain"
)

// TemplateTarget reports whether name (a base filename) is a known env template
// such as ".env.example" and, if so, returns its provisioning target base name
// (".env") and the matched suffix. It never matches multi-environment files like
// ".env.production" or ".env.production.example" — those are out of scope.
func TemplateTarget(name string) (target, suffix string, ok bool) {
	for _, s := range domain.EnvTemplateSuffixes {
		if name == domain.EnvFileName+s {
			return domain.EnvFileName, s, true
		}
	}
	return "", "", false
}

// TemplateCandidates returns the template paths that could provision target, in
// priority order (e.g. ".env" → [".env.example", ".env.dist", …]). Templates sit
// next to their target, so the suffix is appended to the full target path.
func TemplateCandidates(target string) []string {
	candidates := make([]string, 0, len(domain.EnvTemplateSuffixes))
	for _, s := range domain.EnvTemplateSuffixes {
		candidates = append(candidates, target+s)
	}
	return candidates
}

// IsLocalEnvFile reports whether name (a base filename) is the machine-local
// override ".env.local".
func IsLocalEnvFile(name string) bool {
	return name == domain.EnvLocalFileName
}

// IsEnvCandidate reports whether name (a base filename) is an env file wtm manages
// — the value file, the local override, or a known template. Multi-environment
// files (".env.production", …) are not candidates.
func IsEnvCandidate(name string) bool {
	_, _, _, ok := classifyEnvName(name)
	return ok
}

// EnvTargets extracts the flat target list from a structured file list, for
// provisioning consumers that copy value files by target name.
func EnvTargets(files []domain.EnvFile) []string {
	targets := make([]string, 0, len(files))
	for _, f := range files {
		targets = append(targets, f.Target)
	}
	return targets
}

// ClassifyEnvFiles turns a set of discovered relative paths into structured env
// files, grouping each value target with its committed template per directory.
// A ".env" and its ".env.example" collapse into one entry; a lone template still
// yields its target; ".env.local" is its own value entry flagged local. When
// several templates exist for one target the highest-priority suffix is pinned.
// The result is sorted by target for deterministic output.
func ClassifyEnvFiles(paths []string) []domain.EnvFile {
	type entry struct {
		file     domain.EnvFile
		tmplRank int
	}
	byTarget := map[string]*entry{}

	get := func(target string) *entry {
		e, ok := byTarget[target]
		if ok {
			return e
		}
		e = &entry{
			file:     domain.EnvFile{Target: target},
			tmplRank: len(domain.EnvTemplateSuffixes),
		}
		byTarget[target] = e
		return e
	}

	for _, p := range paths {
		dir := path.Dir(p)
		name := path.Base(p)

		targetBase, isTemplate, isLocal, ok := classifyEnvName(name)
		if !ok {
			continue
		}

		target := path.Join(dir, targetBase)

		if isLocal {
			get(target).file.Local = true
			continue
		}
		if !isTemplate {
			get(target)
			continue
		}

		_, suffix, _ := TemplateTarget(name)
		rank := suffixRank(suffix)
		e := get(target)
		if rank < e.tmplRank {
			e.tmplRank = rank
			e.file.Template = path.Join(dir, name)
		}
	}

	files := make([]domain.EnvFile, 0, len(byTarget))
	for _, e := range byTarget {
		files = append(files, e.file)
	}
	sort.Slice(files, func(i, j int) bool { return files[i].Target < files[j].Target })
	return files
}

// classifyEnvName maps a base filename to its value-target base name and role.
// ok=false means the file is not an env file wtm manages.
func classifyEnvName(name string) (targetBase string, template, local bool, ok bool) {
	if name == domain.EnvFileName {
		return domain.EnvFileName, false, false, true
	}
	if IsLocalEnvFile(name) {
		return domain.EnvLocalFileName, false, true, true
	}
	if _, _, isTemplate := TemplateTarget(name); isTemplate {
		return domain.EnvFileName, true, false, true
	}
	return "", false, false, false
}

// suffixRank returns the priority index of a template suffix, or the length of the
// suffix list when unknown (lowest priority).
func suffixRank(suffix string) int {
	for i, s := range domain.EnvTemplateSuffixes {
		if s == suffix {
			return i
		}
	}
	return len(domain.EnvTemplateSuffixes)
}
