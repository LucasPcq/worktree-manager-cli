package rules

import (
	"fmt"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/LucasPcq/wtm/internal/domain"
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

type ClassifyInstallParams struct {
	ExecPath     string
	ResolvedPath string
	GoBinDir     string
	Version      string
}

// ClassifyInstall decides how the running binary was installed. Order matters:
// a `make install` build lands in GoBinDir and would otherwise be sent to fetch
// a published release over the user's own build.
func ClassifyInstall(params ClassifyInstallParams) domain.InstallMethod {
	if NormalizeVersion(params.Version) == domain.Version {
		return domain.InstallSource
	}

	// Homebrew installs <prefix>/bin/wtm as a symlink into the Cellar, so the
	// resolved path is what carries the evidence.
	if isBrewCellarPath(params.ResolvedPath) {
		return domain.InstallHomebrew
	}

	if params.GoBinDir != "" && filepath.Dir(params.ResolvedPath) == filepath.Clean(params.GoBinDir) {
		return domain.InstallGoInstall
	}

	return domain.InstallStandalone
}

const brewCellarSegment = "Cellar"

// isBrewCellarPath matches the full Homebrew layout —
// <prefix>/Cellar/<formula>/<version>/bin/<binary> — rather than the bare
// "Cellar" segment, which a user directory of that name would also carry.
func isBrewCellarPath(path string) bool {
	parts := strings.Split(filepath.ToSlash(path), "/")
	if len(parts) < 5 || parts[len(parts)-2] != "bin" {
		return false
	}

	for i, part := range parts[:len(parts)-3] {
		if part == brewCellarSegment && i < len(parts)-3 {
			return true
		}
	}

	return false
}

type ReleaseAssetNameParams struct {
	Version string
	GOOS    string
	GOARCH  string
}

// ReleaseAssetName builds the goreleaser archive name for a platform. Tags carry
// a leading "v"; asset filenames do not.
func ReleaseAssetName(params ReleaseAssetNameParams) string {
	return fmt.Sprintf("%s_%s_%s_%s.tar.gz", domain.AppName, NormalizeVersion(params.Version), params.GOOS, params.GOARCH)
}

// UpgradeCommandFor names the exact command that updates this install, so every
// message can tell the user what to run rather than that an update exists.
func UpgradeCommandFor(method domain.InstallMethod) string {
	switch method {
	case domain.InstallHomebrew:
		return "brew upgrade " + domain.BrewFormula
	case domain.InstallGoInstall:
		return "go install " + domain.ModulePath + "@latest"
	case domain.InstallSource:
		return "git pull && make install"
	default:
		return domain.AppName + " " + domain.CmdUpgrade
	}
}
