package rules

import (
	"strconv"
	"strings"
)

// NormalizeVersion strips the leading "v" that release tags carry and asset
// filenames do not.
func NormalizeVersion(v string) string {
	return strings.TrimPrefix(strings.TrimSpace(v), "v")
}

type parsedVersion struct {
	fields     [3]int
	prerelease string
}

func parseVersion(v string) (parsedVersion, bool) {
	core, pre, _ := strings.Cut(NormalizeVersion(v), "-")
	if core == "" {
		return parsedVersion{}, false
	}

	parts := strings.Split(core, ".")
	if len(parts) > 3 {
		return parsedVersion{}, false
	}

	out := parsedVersion{prerelease: pre}
	for i, part := range parts {
		n, err := strconv.Atoi(part)
		if err != nil || n < 0 {
			return parsedVersion{}, false
		}
		out.fields[i] = n
	}

	return out, true
}

// IsNewerVersion reports whether latest supersedes current. An unparseable
// version on either side reports false: the notifier stays silent rather than
// guessing.
func IsNewerVersion(current string, latest string) bool {
	cur, ok := parseVersion(current)
	if !ok {
		return false
	}

	next, ok := parseVersion(latest)
	if !ok {
		return false
	}

	for i := range cur.fields {
		if next.fields[i] != cur.fields[i] {
			return next.fields[i] > cur.fields[i]
		}
	}

	if next.prerelease == cur.prerelease {
		return false
	}
	if cur.prerelease != "" && next.prerelease == "" {
		return true
	}
	if cur.prerelease == "" {
		return false
	}

	return next.prerelease > cur.prerelease
}
