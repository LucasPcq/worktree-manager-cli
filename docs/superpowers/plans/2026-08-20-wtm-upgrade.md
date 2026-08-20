# `wtm upgrade` + Update Notifier Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Ship `wtm upgrade` — one command that brings any install method to the latest release — plus a passive 24h notifier that tells users a new version exists.

**Architecture:** Every decision is a pure function in `internal/rules/` taking facts as data; `internal/service/selfupdate/` collects those facts and performs the I/O (HTTP, binary replacement, package-manager delegation); `internal/output/` renders; `internal/commands/upgrade/` wires flags. The notifier is a goroutine started in `cmd/root.go`'s `PersistentPreRun` and drained in `Execute`.

**Tech Stack:** Go 1.26.1, cobra, bubbletea (confirm prompt only), stdlib `net/http` / `archive/tar` / `compress/gzip` / `crypto/sha256`. No new module dependencies.

**Spec:** `docs/superpowers/specs/2026-08-20-wtm-upgrade-design.md`

## Global Constraints

- **No new dependencies.** `go.mod` must be unchanged at the end of this plan. `gh` is an optional runtime dependency of wtm and must not be used here.
- **Layer rules (CLAUDE.md §9).** `internal/rules/` imports stdlib + `internal/domain` only, zero I/O. `internal/service/` never imports cobra, bubbletea, or lipgloss. `internal/output/` and `internal/tui/` contain zero decision logic. `internal/commands/` contains zero business logic.
- **Struct params (CLAUDE.md §2).** Any function taking 2+ related inputs takes a single named-field struct.
- **Centralized constants (CLAUDE.md §5).** Every string key, flag name, env var name, URL, and exit code is a named constant in `internal/domain/constants.go`.
- **Near-zero comments (CLAUDE.md §8).** Comment only what the code cannot carry: a why, an ordering constraint, an invariant. No comment restating a signature.
- **Early returns (CLAUDE.md §6), comma-ok assertions (§7), `:=` for values that never change (§1).**
- **Version source.** The real version lives in `cmd.version` (set by the `-X github.com/LucasPcq/wtm/cmd.version` ldflag). `domain.Version` is `"dev"` in every build. No package under `internal/` may read `domain.Version` for update logic — the version is threaded in as an explicit parameter from `cmd/root.go`.
- **Release facts.** Repo is `LucasPcq/wtm`. Assets are `wtm_<version-without-v>_<goos>_<goarch>.tar.gz` plus `checksums.txt`. Platforms: darwin/linux, amd64/arm64. Tags carry a leading `v`; asset filenames do not.
- **Tests.** `make test` runs `go test ./... -race -count=1`. Table-driven tests, one `t.Run` per case.
- **Never invoke `sudo`.** Report `ErrUpgradeNotWritable` and name the command instead.
- **The notifier never becomes an error.** Any failure in the check path is swallowed.

---

## File Structure

**Created:**
- `internal/domain/upgrade.go` — `InstallMethod`, `ReleaseInfo`, `ReleaseAsset`, `UpdateState`, `UpgradeAction`, `UpgradeResult`
- `internal/rules/upgrade.go` + `_test.go` — `IsNewerVersion`, `ClassifyInstall`, `ReleaseAssetName`, `UpgradeCommandFor`, `ShouldCheckUpdate`
- `internal/service/selfupdate/release.go` + `_test.go` — GitHub Releases HTTP client
- `internal/service/selfupdate/detect.go` + `_test.go` — install-method fact collection
- `internal/service/selfupdate/replace.go` + `_test.go` — download, verify, atomic replace
- `internal/service/selfupdate/delegate.go` — brew / go install execution
- `internal/service/selfupdate/state.go` + `_test.go` — `~/.config/wtm/state.json`
- `internal/service/selfupdate/notify.go` — async check orchestration
- `internal/output/upgrade.go` + `_test.go` — human + JSON rendering
- `internal/commands/upgrade/upgrade.go` — flag wiring

**Modified:**
- `internal/domain/constants.go` — constants block
- `internal/domain/errors.go` — sentinel errors
- `internal/domain/config.go` — `UpdateConfig` on `GlobalConfig`
- `internal/rules/exitcode.go` + `_test.go` — new exit code mapping
- `cmd/root.go` — register `upgrade`, wire the notifier
- `internal/schemas/global.schema.json` — `update.check` key + `$id` fix
- `internal/schemas/{project,run}.schema.json` — `$id` fix
- `internal/config/wtm.toml.tmpl` — commented `[update]` block
- `.goreleaser.yaml`, `README.md` — stale repo links
- `internal/commands/agents/assets/using-wtm.skill.md` — new command surface
- `docs/` — regenerated

---

### Task 1: Repo rename cleanup + domain foundations

The release URL becomes load-bearing in Task 6, so the stale `worktree-manager-cli` references get fixed first.

**Files:**
- Modify: `.goreleaser.yaml:43`, `README.md:31`, `README.md:34`
- Modify: `internal/schemas/global.schema.json:3`, `internal/schemas/project.schema.json:3`, `internal/schemas/run.schema.json:3`
- Create: `internal/domain/upgrade.go`
- Modify: `internal/domain/constants.go`, `internal/domain/errors.go`
- Modify: `internal/rules/exitcode.go`
- Test: `internal/rules/exitcode_test.go`

**Interfaces:**
- Consumes: nothing
- Produces: `domain.InstallMethod` (+ its four values), `domain.ReleaseInfo`, `domain.ReleaseAsset`, `domain.UpdateState`, `domain.UpgradeAction` (+ four values), `domain.UpgradeResult`, the constants block, `domain.ErrUpgradeFromSource`, `domain.ErrUpgradeNotWritable`, `domain.ErrChecksumMismatch`, `domain.ErrReleaseAssetMissing`, `domain.ErrReleaseNotFound`

- [ ] **Step 1: Fix the stale repo links**

```bash
sed -i '' 's|https://github.com/LucasPcq/worktree-manager-cli|https://github.com/LucasPcq/wtm|' .goreleaser.yaml README.md
sed -i '' 's|LucasPcq/worktree-manager-cli/main/schemas|LucasPcq/wtm/main/schemas|' internal/schemas/global.schema.json internal/schemas/project.schema.json internal/schemas/run.schema.json
sed -i '' 's|worktree-manager-cli_\*_darwin_arm64.tar.gz|wtm_*_darwin_arm64.tar.gz|' README.md
```

Then verify nothing is left outside test fixtures:

```bash
grep -rn "worktree-manager-cli" --include="*.go" --include="*.md" --include="*.yaml" --include="*.json" . | grep -v "_test.go" | grep -v "^./docs/wtm_"
```

Expected: only `.claude/settings.json` (an OTEL attribute, cosmetic, leave it).

- [ ] **Step 2: Create the domain types**

Create `internal/domain/upgrade.go`:

```go
package domain

import "time"

type InstallMethod string

const (
	InstallHomebrew   InstallMethod = "homebrew"
	InstallGoInstall  InstallMethod = "go-install"
	InstallStandalone InstallMethod = "standalone"
	InstallSource     InstallMethod = "source"
)

type UpgradeAction string

const (
	UpgradeActionReplaced  UpgradeAction = "replaced"
	UpgradeActionDelegated UpgradeAction = "delegated"
	UpgradeActionNone      UpgradeAction = "none"
	UpgradeActionChecked   UpgradeAction = "checked"
)

type ReleaseAsset struct {
	Name string
	URL  string
}

type ReleaseInfo struct {
	Version string
	Tag     string
	URL     string
	Assets  []ReleaseAsset
}

type UpdateState struct {
	CheckedAt     time.Time `json:"checked_at"`
	LatestVersion string    `json:"latest_version"`
}

type UpgradeResult struct {
	Installed string        `json:"installed"`
	Latest    string        `json:"latest"`
	UpToDate  bool          `json:"up_to_date"`
	Method    InstallMethod `json:"method"`
	Action    UpgradeAction `json:"action"`
}
```

- [ ] **Step 3: Add the constants**

Append to the `const` block in `internal/domain/constants.go`:

```go
	// RepoOwner and RepoName address the GitHub repository releases are published to.
	RepoOwner = "LucasPcq"
	RepoName  = "wtm"

	// ModulePath is the Go module path, used by the go-install upgrade path.
	ModulePath = "github.com/LucasPcq/wtm"

	// BrewFormula is the fully qualified tap formula, used by the Homebrew upgrade path.
	BrewFormula = "LucasPcq/tap/wtm"

	// ReleaseAPIBase is the GitHub Releases REST endpoint for this repo.
	ReleaseAPIBase = "https://api.github.com/repos/" + RepoOwner + "/" + RepoName + "/releases"

	// ChecksumsFileName is the SHA256 manifest goreleaser publishes with every release.
	ChecksumsFileName = "checksums.txt"

	// GlobalStateFile holds wtm-written state next to the user config. Kept separate
	// from config.toml: the CLI must never rewrite a file the user hand-edits.
	GlobalStateFile = "state.json"

	// UpdateCheckTTL throttles the passive notifier.
	UpdateCheckTTL = 24 * time.Hour

	// UpdateCheckTimeout caps the notifier's HTTP request; UpdateNoticeWait caps how
	// long Execute waits for its result before giving up on printing a notice.
	UpdateCheckTimeout = 2 * time.Second
	UpdateNoticeWait   = 300 * time.Millisecond

	// DownloadTimeout caps the release download in wtm upgrade.
	DownloadTimeout = 60 * time.Second

	// EnvNoUpdateCheck disables the passive notifier when set to any value.
	EnvNoUpdateCheck = "WTM_NO_UPDATE_CHECK"

	// EnvCI and EnvGitHubActions mark a non-interactive automation run.
	EnvCI             = "CI"
	EnvGitHubActions  = "GITHUB_ACTIONS"

	// CmdUpgrade is the self-update command name. The four that follow already
	// exist as literals in their command files and are centralized here because
	// the notifier's exclusion list needs to name them.
	CmdUpgrade    = "upgrade"
	CmdShellInit  = "shell-init"
	CmdResolve    = "resolve"
	CmdDaemon     = "daemon"
	CmdCompletion = "completion"
	CmdSchema     = "schema"

	// FlagCheck reports upgrade availability without changing anything.
	FlagCheck = "check"

	// FlagVersionPin selects an explicit release instead of the latest.
	FlagVersionPin = "version"

	// ExitCodeUpgradeUnsupported marks an upgrade that cannot proceed on this
	// install: built from source, or the binary is not writable.
	ExitCodeUpgradeUnsupported = 17

	// UpgradeJSONNeedsYes refuses a JSON run that would have to prompt.
	UpgradeJSONNeedsYes = "--output json requires --yes or --check (the confirmation prompt cannot run in JSON mode)"

	// UpgradeSourceHint tells a from-source user how to update.
	UpgradeSourceHint = "this binary was built from source — run `git pull && make install` instead"

	// UpgradePinUnsupported refuses --version on a package-manager install.
	UpgradePinUnsupported = "--version only applies to a standalone binary; pin the version through your package manager instead"
```

`time` is already imported by `constants.go`.

- [ ] **Step 4: Add the sentinel errors**

Append to the `var` block in `internal/domain/errors.go`:

```go
	// ErrUpgradeFromSource is returned when wtm upgrade runs on a binary built
	// from source, where no published release corresponds to what is installed.
	ErrUpgradeFromSource = errors.New(UpgradeSourceHint)

	// ErrUpgradeNotWritable is returned when the running binary cannot be replaced
	// because its directory is not writable by the current user.
	ErrUpgradeNotWritable = errors.New("cannot write to the wtm binary — re-run with sudo")

	// ErrChecksumMismatch is returned when a downloaded release archive does not
	// match the SHA256 published in checksums.txt. Nothing is written.
	ErrChecksumMismatch = errors.New("downloaded archive failed checksum verification")

	// ErrReleaseAssetMissing is returned when the release carries no archive for
	// the running platform.
	ErrReleaseAssetMissing = errors.New("no release asset for this platform")

	// ErrReleaseNotFound is returned when the requested release tag does not exist.
	ErrReleaseNotFound = errors.New("release not found")
```

- [ ] **Step 5: Write the failing exit-code test**

Add to `internal/rules/exitcode_test.go`, following the existing table style in that file:

```go
func TestExitCodeUpgradeErrors(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want int
	}{
		{"from source", domain.ErrUpgradeFromSource, domain.ExitCodeUpgradeUnsupported},
		{"not writable", domain.ErrUpgradeNotWritable, domain.ExitCodeUpgradeUnsupported},
		{"wrapped not writable", fmt.Errorf("upgrade: %w", domain.ErrUpgradeNotWritable), domain.ExitCodeUpgradeUnsupported},
		{"checksum mismatch stays generic", domain.ErrChecksumMismatch, domain.ExitCodeError},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := rules.ExitCode(tc.err); got != tc.want {
				t.Fatalf("ExitCode(%v) = %d, want %d", tc.err, got, tc.want)
			}
		})
	}
}
```

Match the existing file's package clause and imports (add `fmt` if absent).

- [ ] **Step 6: Run the test to verify it fails**

Run: `go test ./internal/rules/ -run TestExitCodeUpgradeErrors -v`
Expected: FAIL — `ExitCode(...) = 1, want 17`.

- [ ] **Step 7: Map the new exit code**

In `internal/rules/exitcode.go`, add before the `default` branch:

```go
	case errors.Is(err, domain.ErrUpgradeFromSource), errors.Is(err, domain.ErrUpgradeNotWritable):
		return domain.ExitCodeUpgradeUnsupported
```

- [ ] **Step 8: Run the tests**

Run: `go test ./internal/rules/ ./internal/domain/ -race -count=1`
Expected: PASS

- [ ] **Step 9: Commit**

```bash
git add .goreleaser.yaml README.md internal/schemas internal/domain internal/rules
git commit -m "feat(upgrade): domain types, constants and exit code; fix stale repo links"
```

---

### Task 2: `rules.IsNewerVersion`

**Files:**
- Create: `internal/rules/upgrade.go`
- Test: `internal/rules/upgrade_test.go`

**Interfaces:**
- Consumes: `domain.Version` semantics only
- Produces: `rules.IsNewerVersion(current, latest string) bool`, `rules.NormalizeVersion(v string) string`

- [ ] **Step 1: Write the failing test**

Create `internal/rules/upgrade_test.go`:

```go
package rules_test

import (
	"testing"

	"github.com/LucasPcq/wtm/internal/rules"
)

func TestIsNewerVersion(t *testing.T) {
	cases := []struct {
		name    string
		current string
		latest  string
		want    bool
	}{
		{"patch bump", "0.26.1", "0.26.2", true},
		{"minor bump", "0.26.1", "0.27.0", true},
		{"major bump", "0.26.1", "1.0.0", true},
		{"equal", "0.26.1", "0.26.1", false},
		{"older", "0.27.0", "0.26.1", false},
		{"leading v on latest", "0.26.1", "v0.27.0", true},
		{"leading v on both", "v0.26.1", "v0.26.1", false},
		{"numeric not lexical", "0.9.0", "0.10.0", true},
		{"prerelease below its release", "1.0.0-rc1", "1.0.0", true},
		{"release above its prerelease", "1.0.0", "1.0.0-rc1", false},
		{"prerelease ordering", "1.0.0-rc1", "1.0.0-rc2", true},
		{"dev never notifies", "dev", "0.27.0", false},
		{"garbage current", "banana", "0.27.0", false},
		{"garbage latest", "0.26.1", "banana", false},
		{"empty latest", "0.26.1", "", false},
		{"short version", "1.0", "1.0.1", true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := rules.IsNewerVersion(tc.current, tc.latest); got != tc.want {
				t.Fatalf("IsNewerVersion(%q, %q) = %v, want %v", tc.current, tc.latest, got, tc.want)
			}
		})
	}
}

func TestNormalizeVersion(t *testing.T) {
	cases := []struct{ in, want string }{
		{"v0.26.1", "0.26.1"},
		{"0.26.1", "0.26.1"},
		{"", ""},
	}

	for _, tc := range cases {
		t.Run(tc.in, func(t *testing.T) {
			if got := rules.NormalizeVersion(tc.in); got != tc.want {
				t.Fatalf("NormalizeVersion(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `go test ./internal/rules/ -run "TestIsNewerVersion|TestNormalizeVersion" -v`
Expected: FAIL — `undefined: rules.IsNewerVersion`.

- [ ] **Step 3: Implement**

Create `internal/rules/upgrade.go`:

```go
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
	parts := strings.Split(core, ".")
	if len(parts) == 0 || len(parts) > 3 || core == "" {
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
```

- [ ] **Step 4: Run the tests**

Run: `go test ./internal/rules/ -run "TestIsNewerVersion|TestNormalizeVersion" -v`
Expected: PASS (16 + 3 subtests)

- [ ] **Step 5: Commit**

```bash
git add internal/rules/upgrade.go internal/rules/upgrade_test.go
git commit -m "feat(upgrade): version comparison rule"
```

---

### Task 3: `rules.ClassifyInstall`

**Files:**
- Modify: `internal/rules/upgrade.go`
- Test: `internal/rules/upgrade_test.go`

**Interfaces:**
- Consumes: `domain.InstallMethod` (Task 1)
- Produces: `rules.ClassifyInstallParams{ExecPath, ResolvedPath, GoBinDir, Version string}`, `rules.ClassifyInstall(params ClassifyInstallParams) domain.InstallMethod`

- [ ] **Step 1: Write the failing test**

Append to `internal/rules/upgrade_test.go`:

```go
func TestClassifyInstall(t *testing.T) {
	cases := []struct {
		name   string
		params rules.ClassifyInstallParams
		want   domain.InstallMethod
	}{
		{
			name: "homebrew arm mac",
			params: rules.ClassifyInstallParams{
				ExecPath:     "/opt/homebrew/bin/wtm",
				ResolvedPath: "/opt/homebrew/Cellar/wtm/0.26.1/bin/wtm",
				GoBinDir:     "/Users/x/go/bin",
				Version:      "0.26.1",
			},
			want: domain.InstallHomebrew,
		},
		{
			name: "homebrew intel mac",
			params: rules.ClassifyInstallParams{
				ExecPath:     "/usr/local/bin/wtm",
				ResolvedPath: "/usr/local/Cellar/wtm/0.26.1/bin/wtm",
				GoBinDir:     "/Users/x/go/bin",
				Version:      "0.26.1",
			},
			want: domain.InstallHomebrew,
		},
		{
			name: "linuxbrew",
			params: rules.ClassifyInstallParams{
				ExecPath:     "/home/x/.linuxbrew/bin/wtm",
				ResolvedPath: "/home/linuxbrew/.linuxbrew/Cellar/wtm/0.26.1/bin/wtm",
				GoBinDir:     "/home/x/go/bin",
				Version:      "0.26.1",
			},
			want: domain.InstallHomebrew,
		},
		{
			name: "go install",
			params: rules.ClassifyInstallParams{
				ExecPath:     "/Users/x/go/bin/wtm",
				ResolvedPath: "/Users/x/go/bin/wtm",
				GoBinDir:     "/Users/x/go/bin",
				Version:      "0.26.1",
			},
			want: domain.InstallGoInstall,
		},
		{
			name: "standalone in usr local bin",
			params: rules.ClassifyInstallParams{
				ExecPath:     "/usr/local/bin/wtm",
				ResolvedPath: "/usr/local/bin/wtm",
				GoBinDir:     "/Users/x/go/bin",
				Version:      "0.26.1",
			},
			want: domain.InstallStandalone,
		},
		{
			name: "user directory literally named Cellar is not brew",
			params: rules.ClassifyInstallParams{
				ExecPath:     "/Users/x/Cellar/wtm",
				ResolvedPath: "/Users/x/Cellar/wtm",
				GoBinDir:     "/Users/x/go/bin",
				Version:      "0.26.1",
			},
			want: domain.InstallStandalone,
		},
		{
			name: "dev build in go bin is source, not go-install",
			params: rules.ClassifyInstallParams{
				ExecPath:     "/Users/x/go/bin/wtm",
				ResolvedPath: "/Users/x/go/bin/wtm",
				GoBinDir:     "/Users/x/go/bin",
				Version:      "dev",
			},
			want: domain.InstallSource,
		},
		{
			name: "empty go bin dir does not swallow everything",
			params: rules.ClassifyInstallParams{
				ExecPath:     "/usr/local/bin/wtm",
				ResolvedPath: "/usr/local/bin/wtm",
				GoBinDir:     "",
				Version:      "0.26.1",
			},
			want: domain.InstallStandalone,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := rules.ClassifyInstall(tc.params); got != tc.want {
				t.Fatalf("ClassifyInstall(%+v) = %q, want %q", tc.params, got, tc.want)
			}
		})
	}
}
```

Add `"github.com/LucasPcq/wtm/internal/domain"` to the test file's imports.

- [ ] **Step 2: Run the test to verify it fails**

Run: `go test ./internal/rules/ -run TestClassifyInstall -v`
Expected: FAIL — `undefined: rules.ClassifyInstall`.

- [ ] **Step 3: Implement**

Append to `internal/rules/upgrade.go` (add `"path/filepath"` and the domain import):

```go
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

	if hasPathSegment(params.ResolvedPath, brewCellarSegment) {
		return domain.InstallHomebrew
	}

	if params.GoBinDir != "" && filepath.Dir(params.ResolvedPath) == filepath.Clean(params.GoBinDir) {
		return domain.InstallGoInstall
	}

	return domain.InstallStandalone
}

const brewCellarSegment = "Cellar"

func hasPathSegment(path string, segment string) bool {
	for _, part := range strings.Split(filepath.ToSlash(path), "/") {
		if part == segment {
			return true
		}
	}
	return false
}
```

The `Cellar` check runs on the *resolved* path because Homebrew installs `/opt/homebrew/bin/wtm` as a symlink into the Cellar; matching a whole path segment (not a substring) is what keeps `/Users/x/Cellar/wtm` out.

- [ ] **Step 4: Run the tests**

Run: `go test ./internal/rules/ -run TestClassifyInstall -v`
Expected: PASS (8 subtests)

- [ ] **Step 5: Commit**

```bash
git add internal/rules/upgrade.go internal/rules/upgrade_test.go
git commit -m "feat(upgrade): install-method classification rule"
```

---

### Task 4: `rules.ReleaseAssetName` and `rules.UpgradeCommandFor`

**Files:**
- Modify: `internal/rules/upgrade.go`
- Test: `internal/rules/upgrade_test.go`

**Interfaces:**
- Consumes: `domain.InstallMethod`, `domain.BrewFormula`, `domain.ModulePath`
- Produces: `rules.ReleaseAssetName(params ReleaseAssetNameParams) string` with `ReleaseAssetNameParams{Version, GOOS, GOARCH string}`; `rules.UpgradeCommandFor(method domain.InstallMethod) string`

- [ ] **Step 1: Write the failing test**

Append to `internal/rules/upgrade_test.go`:

```go
func TestReleaseAssetName(t *testing.T) {
	cases := []struct {
		name   string
		params rules.ReleaseAssetNameParams
		want   string
	}{
		{"darwin arm64", rules.ReleaseAssetNameParams{Version: "0.26.1", GOOS: "darwin", GOARCH: "arm64"}, "wtm_0.26.1_darwin_arm64.tar.gz"},
		{"linux amd64", rules.ReleaseAssetNameParams{Version: "0.26.1", GOOS: "linux", GOARCH: "amd64"}, "wtm_0.26.1_linux_amd64.tar.gz"},
		{"tag with leading v is normalized", rules.ReleaseAssetNameParams{Version: "v0.26.1", GOOS: "darwin", GOARCH: "amd64"}, "wtm_0.26.1_darwin_amd64.tar.gz"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := rules.ReleaseAssetName(tc.params); got != tc.want {
				t.Fatalf("ReleaseAssetName(%+v) = %q, want %q", tc.params, got, tc.want)
			}
		})
	}
}

func TestUpgradeCommandFor(t *testing.T) {
	cases := []struct {
		method domain.InstallMethod
		want   string
	}{
		{domain.InstallHomebrew, "brew upgrade LucasPcq/tap/wtm"},
		{domain.InstallGoInstall, "go install github.com/LucasPcq/wtm@latest"},
		{domain.InstallStandalone, "wtm upgrade"},
		{domain.InstallSource, "git pull && make install"},
	}

	for _, tc := range cases {
		t.Run(string(tc.method), func(t *testing.T) {
			if got := rules.UpgradeCommandFor(tc.method); got != tc.want {
				t.Fatalf("UpgradeCommandFor(%q) = %q, want %q", tc.method, got, tc.want)
			}
		})
	}
}
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `go test ./internal/rules/ -run "TestReleaseAssetName|TestUpgradeCommandFor" -v`
Expected: FAIL — `undefined: rules.ReleaseAssetName`.

- [ ] **Step 3: Implement**

Append to `internal/rules/upgrade.go` (add `"fmt"`):

```go
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
// message can tell the user what to run instead of that an update exists.
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
```

- [ ] **Step 4: Run the tests**

Run: `go test ./internal/rules/ -race -count=1`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/rules/upgrade.go internal/rules/upgrade_test.go
git commit -m "feat(upgrade): release asset naming and per-method upgrade command"
```

---

### Task 5: `rules.ShouldCheckUpdate`

The whole suppression policy of the notifier, in one pure function.

**Files:**
- Modify: `internal/rules/upgrade.go`
- Test: `internal/rules/upgrade_test.go`

**Interfaces:**
- Consumes: `rules.IsHumanFormat` (existing), `domain.UpdateCheckTTL`
- Produces: `rules.ShouldCheckUpdateParams{Version, Format, Command string; StderrIsTTY, CIEnv, OptOutEnv bool; ConfigCheck *bool; CheckedAt, Now time.Time}`, `rules.ShouldCheckUpdate(params ShouldCheckUpdateParams) bool`

- [ ] **Step 1: Write the failing test**

Append to `internal/rules/upgrade_test.go` (add `"time"` to imports):

```go
func baseCheckParams() rules.ShouldCheckUpdateParams {
	now := time.Date(2026, 8, 20, 12, 0, 0, 0, time.UTC)
	return rules.ShouldCheckUpdateParams{
		Version:     "0.26.1",
		Format:      domain.OutputText,
		Command:     "list",
		StderrIsTTY: true,
		CheckedAt:   now.Add(-48 * time.Hour),
		Now:         now,
	}
}

func TestShouldCheckUpdate(t *testing.T) {
	cases := []struct {
		name   string
		mutate func(*rules.ShouldCheckUpdateParams)
		want   bool
	}{
		{"nominal interactive run", func(p *rules.ShouldCheckUpdateParams) {}, true},
		{"dev build never checks", func(p *rules.ShouldCheckUpdateParams) { p.Version = "dev" }, false},
		{"json output", func(p *rules.ShouldCheckUpdateParams) { p.Format = domain.OutputJSON }, false},
		{"no tty", func(p *rules.ShouldCheckUpdateParams) { p.StderrIsTTY = false }, false},
		{"ci", func(p *rules.ShouldCheckUpdateParams) { p.CIEnv = true }, false},
		{"env opt out", func(p *rules.ShouldCheckUpdateParams) { p.OptOutEnv = true }, false},
		{"config opt out", func(p *rules.ShouldCheckUpdateParams) { p.ConfigCheck = boolPtr(false) }, false},
		{"config opt in is not an override of ci", func(p *rules.ShouldCheckUpdateParams) {
			p.ConfigCheck = boolPtr(true)
			p.CIEnv = true
		}, false},
		{"shell-init excluded", func(p *rules.ShouldCheckUpdateParams) { p.Command = domain.CmdShellInit }, false},
		{"resolve excluded", func(p *rules.ShouldCheckUpdateParams) { p.Command = domain.CmdResolve }, false},
		{"upgrade excluded", func(p *rules.ShouldCheckUpdateParams) { p.Command = domain.CmdUpgrade }, false},
		{"daemon excluded", func(p *rules.ShouldCheckUpdateParams) { p.Command = domain.CmdDaemon }, false},
		{"completion excluded", func(p *rules.ShouldCheckUpdateParams) { p.Command = domain.CmdCompletion }, false},
		{"schema excluded", func(p *rules.ShouldCheckUpdateParams) { p.Command = domain.CmdSchema }, false},
		{"inside ttl", func(p *rules.ShouldCheckUpdateParams) { p.CheckedAt = p.Now.Add(-1 * time.Hour) }, false},
		{"exactly at ttl", func(p *rules.ShouldCheckUpdateParams) { p.CheckedAt = p.Now.Add(-domain.UpdateCheckTTL) }, true},
		{"never checked", func(p *rules.ShouldCheckUpdateParams) { p.CheckedAt = time.Time{} }, true},
		{"clock skew: checked in the future", func(p *rules.ShouldCheckUpdateParams) { p.CheckedAt = p.Now.Add(2 * time.Hour) }, false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			params := baseCheckParams()
			tc.mutate(&params)
			if got := rules.ShouldCheckUpdate(params); got != tc.want {
				t.Fatalf("ShouldCheckUpdate(%+v) = %v, want %v", params, got, tc.want)
			}
		})
	}
}

func boolPtr(b bool) *bool { return &b }
```

`boolPtr` does not exist in `internal/rules/` today — declare it as shown. `domain.CmdShellInit`, `domain.CmdResolve`, `domain.CmdDaemon`, `domain.CmdCompletion` and `domain.CmdSchema` were added in Task 1.

- [ ] **Step 2: Run the test to verify it fails**

Run: `go test ./internal/rules/ -run TestShouldCheckUpdate -v`
Expected: FAIL — `undefined: rules.ShouldCheckUpdate`.

- [ ] **Step 3: Implement**

Append to `internal/rules/upgrade.go` (add `"time"`):

```go
type ShouldCheckUpdateParams struct {
	Version     string
	Format      string
	Command     string
	StderrIsTTY bool
	CIEnv       bool
	OptOutEnv   bool
	ConfigCheck *bool
	CheckedAt   time.Time
	Now         time.Time
}

// updateCheckExcluded lists commands whose output is consumed by something other
// than a human: shell-init and resolve are eval'd by the shell, where a stray
// byte breaks the caller.
var updateCheckExcluded = map[string]bool{
	domain.CmdShellInit: true,
	domain.CmdResolve:   true,
	domain.CmdUpgrade:   true,
	domain.CmdDaemon:     true,
	domain.CmdCompletion: true,
	domain.CmdSchema:     true,
}

func ShouldCheckUpdate(params ShouldCheckUpdateParams) bool {
	if NormalizeVersion(params.Version) == domain.Version {
		return false
	}
	if !IsHumanFormat(params.Format) {
		return false
	}
	if !params.StderrIsTTY {
		return false
	}
	if params.CIEnv || params.OptOutEnv {
		return false
	}
	if params.ConfigCheck != nil && !*params.ConfigCheck {
		return false
	}
	if updateCheckExcluded[params.Command] {
		return false
	}
	if params.CheckedAt.IsZero() {
		return true
	}

	elapsed := params.Now.Sub(params.CheckedAt)
	return elapsed >= domain.UpdateCheckTTL
}
```

A `CheckedAt` in the future yields a negative `elapsed`, which fails the `>=` and suppresses the check — the intended behavior under clock skew.

- [ ] **Step 4: Run the tests**

Run: `go test ./internal/rules/ -race -count=1`
Expected: PASS (18 subtests for this function)

- [ ] **Step 5: Commit**

```bash
git add internal/rules/upgrade.go internal/rules/upgrade_test.go internal/domain/constants.go
git commit -m "feat(upgrade): update-check suppression rule"
```

---

### Task 6: `selfupdate.FetchRelease`

**Files:**
- Create: `internal/service/selfupdate/release.go`
- Test: `internal/service/selfupdate/release_test.go`

**Interfaces:**
- Consumes: `domain.ReleaseInfo`, `domain.ReleaseAsset`, `domain.ErrReleaseNotFound`, `domain.ReleaseAPIBase`, `rules.NormalizeVersion`
- Produces: `selfupdate.FetchReleaseParams{BaseURL, Tag, UserAgent string; Timeout time.Duration}`, `selfupdate.FetchRelease(params FetchReleaseParams) (domain.ReleaseInfo, error)`

- [ ] **Step 1: Write the failing test**

Create `internal/service/selfupdate/release_test.go`:

```go
package selfupdate_test

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/service/selfupdate"
)

const releaseJSON = `{
  "tag_name": "v0.27.0",
  "html_url": "https://github.com/LucasPcq/wtm/releases/tag/v0.27.0",
  "assets": [
    {"name": "checksums.txt", "browser_download_url": "https://example.test/checksums.txt"},
    {"name": "wtm_0.27.0_darwin_arm64.tar.gz", "browser_download_url": "https://example.test/wtm.tar.gz"}
  ]
}`

func TestFetchReleaseLatest(t *testing.T) {
	var gotPath, gotUA string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath, gotUA = r.URL.Path, r.Header.Get("User-Agent")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(releaseJSON))
	}))
	defer srv.Close()

	info, err := selfupdate.FetchRelease(selfupdate.FetchReleaseParams{
		BaseURL:   srv.URL,
		UserAgent: "wtm/0.26.1",
		Timeout:   2 * time.Second,
	})
	if err != nil {
		t.Fatalf("FetchRelease: %v", err)
	}

	if gotPath != "/latest" {
		t.Fatalf("path = %q, want /latest", gotPath)
	}
	if gotUA != "wtm/0.26.1" {
		t.Fatalf("user-agent = %q, want wtm/0.26.1", gotUA)
	}
	if info.Version != "0.27.0" {
		t.Fatalf("Version = %q, want 0.27.0 (leading v stripped)", info.Version)
	}
	if info.Tag != "v0.27.0" {
		t.Fatalf("Tag = %q, want v0.27.0", info.Tag)
	}
	if len(info.Assets) != 2 {
		t.Fatalf("Assets = %d, want 2", len(info.Assets))
	}
}

func TestFetchReleaseByTag(t *testing.T) {
	var gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		_, _ = w.Write([]byte(releaseJSON))
	}))
	defer srv.Close()

	if _, err := selfupdate.FetchRelease(selfupdate.FetchReleaseParams{
		BaseURL: srv.URL,
		Tag:     "0.25.0",
		Timeout: 2 * time.Second,
	}); err != nil {
		t.Fatalf("FetchRelease: %v", err)
	}

	if gotPath != "/tags/v0.25.0" {
		t.Fatalf("path = %q, want /tags/v0.25.0 (leading v added back)", gotPath)
	}
}

func TestFetchReleaseNotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	_, err := selfupdate.FetchRelease(selfupdate.FetchReleaseParams{BaseURL: srv.URL, Timeout: 2 * time.Second})
	if !errors.Is(err, domain.ErrReleaseNotFound) {
		t.Fatalf("err = %v, want ErrReleaseNotFound", err)
	}
}

func TestFetchReleaseRateLimited(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	defer srv.Close()

	if _, err := selfupdate.FetchRelease(selfupdate.FetchReleaseParams{BaseURL: srv.URL, Timeout: 2 * time.Second}); err == nil {
		t.Fatal("want an error on 403, got nil")
	}
}

func TestFetchReleaseMalformedJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("not json"))
	}))
	defer srv.Close()

	if _, err := selfupdate.FetchRelease(selfupdate.FetchReleaseParams{BaseURL: srv.URL, Timeout: 2 * time.Second}); err == nil {
		t.Fatal("want an error on malformed JSON, got nil")
	}
}
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `go test ./internal/service/selfupdate/ -v`
Expected: FAIL — the package does not exist.

- [ ] **Step 3: Implement**

Create `internal/service/selfupdate/release.go`:

```go
// Package selfupdate resolves how wtm was installed and brings it to the latest
// published release.
package selfupdate

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/rules"
)

type FetchReleaseParams struct {
	BaseURL   string
	Tag       string
	UserAgent string
	Timeout   time.Duration
}

type releasePayload struct {
	TagName string `json:"tag_name"`
	HTMLURL string `json:"html_url"`
	Assets  []struct {
		Name string `json:"name"`
		URL  string `json:"browser_download_url"`
	} `json:"assets"`
}

func FetchRelease(params FetchReleaseParams) (domain.ReleaseInfo, error) {
	base := params.BaseURL
	if base == "" {
		base = domain.ReleaseAPIBase
	}

	url := base + "/latest"
	if params.Tag != "" {
		url = base + "/tags/v" + rules.NormalizeVersion(params.Tag)
	}

	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return domain.ReleaseInfo{}, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	if params.UserAgent != "" {
		req.Header.Set("User-Agent", params.UserAgent)
	}

	client := &http.Client{Timeout: params.Timeout}
	resp, err := client.Do(req)
	if err != nil {
		return domain.ReleaseInfo{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return domain.ReleaseInfo{}, domain.ErrReleaseNotFound
	}
	if resp.StatusCode != http.StatusOK {
		return domain.ReleaseInfo{}, fmt.Errorf("github releases API: %s", resp.Status)
	}

	var payload releasePayload
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&payload); err != nil {
		return domain.ReleaseInfo{}, fmt.Errorf("decode release: %w", err)
	}

	info := domain.ReleaseInfo{
		Version: rules.NormalizeVersion(payload.TagName),
		Tag:     payload.TagName,
		URL:     payload.HTMLURL,
	}
	for _, asset := range payload.Assets {
		info.Assets = append(info.Assets, domain.ReleaseAsset{Name: asset.Name, URL: asset.URL})
	}

	return info, nil
}
```

- [ ] **Step 4: Run the tests**

Run: `go test ./internal/service/selfupdate/ -race -count=1 -v`
Expected: PASS (5 tests)

- [ ] **Step 5: Commit**

```bash
git add internal/service/selfupdate/release.go internal/service/selfupdate/release_test.go
git commit -m "feat(upgrade): GitHub releases client"
```

---

### Task 7: `selfupdate.DetectInstall`

**Files:**
- Create: `internal/service/selfupdate/detect.go`
- Test: `internal/service/selfupdate/detect_test.go`

**Interfaces:**
- Consumes: `rules.ClassifyInstall`, `rules.ClassifyInstallParams`
- Produces: `selfupdate.Install{Method domain.InstallMethod; BinaryPath string}`, `selfupdate.DetectInstall(version string) Install`

- [ ] **Step 1: Write the failing test**

Create `internal/service/selfupdate/detect_test.go`:

```go
package selfupdate_test

import (
	"path/filepath"
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/service/selfupdate"
)

func TestDetectInstallReportsSourceForDevBuild(t *testing.T) {
	got := selfupdate.DetectInstall(domain.Version)
	if got.Method != domain.InstallSource {
		t.Fatalf("Method = %q, want %q", got.Method, domain.InstallSource)
	}
}

func TestDetectInstallResolvesARealBinaryPath(t *testing.T) {
	got := selfupdate.DetectInstall("0.26.1")
	if got.BinaryPath == "" {
		t.Fatal("BinaryPath is empty; DetectInstall must resolve the running executable")
	}
	if !filepath.IsAbs(got.BinaryPath) {
		t.Fatalf("BinaryPath = %q, want an absolute path", got.BinaryPath)
	}
}
```

The test binary is not `wtm`, so only these two invariants are assertable here — the classification table itself is covered in Task 3.

- [ ] **Step 2: Run the test to verify it fails**

Run: `go test ./internal/service/selfupdate/ -run TestDetectInstall -v`
Expected: FAIL — `undefined: selfupdate.DetectInstall`.

- [ ] **Step 3: Implement**

Create `internal/service/selfupdate/detect.go`:

```go
package selfupdate

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/rules"
)

type Install struct {
	Method     domain.InstallMethod
	BinaryPath string
}

func DetectInstall(version string) Install {
	execPath, err := os.Executable()
	if err != nil {
		return Install{Method: domain.InstallStandalone}
	}

	resolved, err := filepath.EvalSymlinks(execPath)
	if err != nil {
		resolved = execPath
	}

	return Install{
		Method: rules.ClassifyInstall(rules.ClassifyInstallParams{
			ExecPath:     execPath,
			ResolvedPath: resolved,
			GoBinDir:     goBinDir(),
			Version:      version,
		}),
		BinaryPath: resolved,
	}
}

func goBinDir() string {
	if bin := os.Getenv("GOBIN"); bin != "" {
		return bin
	}

	out, err := exec.Command("go", "env", "GOBIN", "GOPATH").Output()
	if err == nil {
		lines := strings.Split(strings.TrimSpace(string(out)), "\n")
		if len(lines) == 2 {
			if lines[0] != "" {
				return lines[0]
			}
			if lines[1] != "" {
				return filepath.Join(lines[1], "bin")
			}
		}
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}

	return filepath.Join(home, "go", "bin")
}
```

`BinaryPath` is the resolved path so the atomic rename in Task 8 targets the real file, not a symlink.

- [ ] **Step 4: Run the tests**

Run: `go test ./internal/service/selfupdate/ -race -count=1`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/service/selfupdate/detect.go internal/service/selfupdate/detect_test.go
git commit -m "feat(upgrade): install detection"
```

---

### Task 8: `selfupdate.ReplaceBinary`

The dangerous path. The checksum-mismatch test asserting the original binary is byte-identical afterwards is the point of this task.

**Files:**
- Create: `internal/service/selfupdate/replace.go`
- Test: `internal/service/selfupdate/replace_test.go`

**Interfaces:**
- Consumes: `domain.ReleaseInfo`, `domain.ErrChecksumMismatch`, `domain.ErrReleaseAssetMissing`, `domain.ErrUpgradeNotWritable`, `rules.ReleaseAssetName`
- Produces: `selfupdate.ReplaceBinaryParams{Release domain.ReleaseInfo; BinaryPath, GOOS, GOARCH, UserAgent string; Timeout time.Duration}`, `selfupdate.ReplaceBinary(params ReplaceBinaryParams) error`

- [ ] **Step 1: Write the failing test**

Create `internal/service/selfupdate/replace_test.go`:

```go
package selfupdate_test

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/service/selfupdate"
)

func buildArchive(t *testing.T, payload string) []byte {
	t.Helper()

	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)

	if err := tw.WriteHeader(&tar.Header{Name: "wtm", Mode: 0o755, Size: int64(len(payload))}); err != nil {
		t.Fatalf("write header: %v", err)
	}
	if _, err := tw.Write([]byte(payload)); err != nil {
		t.Fatalf("write body: %v", err)
	}
	if err := tw.Close(); err != nil {
		t.Fatalf("close tar: %v", err)
	}
	if err := gz.Close(); err != nil {
		t.Fatalf("close gzip: %v", err)
	}

	return buf.Bytes()
}

type releaseServer struct {
	*httptest.Server
	assetName string
}

func newReleaseServer(t *testing.T, archive []byte, checksum string) releaseServer {
	t.Helper()

	assetName := "wtm_0.27.0_darwin_arm64.tar.gz"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/" + assetName:
			_, _ = w.Write(archive)
		case "/checksums.txt":
			fmt.Fprintf(w, "%s  %s\n", checksum, assetName)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))

	return releaseServer{Server: srv, assetName: assetName}
}

func (s releaseServer) release() domain.ReleaseInfo {
	return domain.ReleaseInfo{
		Version: "0.27.0",
		Tag:     "v0.27.0",
		Assets: []domain.ReleaseAsset{
			{Name: s.assetName, URL: s.URL + "/" + s.assetName},
			{Name: domain.ChecksumsFileName, URL: s.URL + "/checksums.txt"},
		},
	}
}

func sha256Hex(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

func TestReplaceBinarySwapsTheFile(t *testing.T) {
	archive := buildArchive(t, "NEW BINARY")
	srv := newReleaseServer(t, archive, sha256Hex(archive))
	defer srv.Close()

	dir := t.TempDir()
	target := filepath.Join(dir, "wtm")
	if err := os.WriteFile(target, []byte("OLD BINARY"), 0o755); err != nil {
		t.Fatalf("seed binary: %v", err)
	}

	err := selfupdate.ReplaceBinary(selfupdate.ReplaceBinaryParams{
		Release:    srv.release(),
		BinaryPath: target,
		GOOS:       "darwin",
		GOARCH:     "arm64",
		Timeout:    5 * time.Second,
	})
	if err != nil {
		t.Fatalf("ReplaceBinary: %v", err)
	}

	got, err := os.ReadFile(target)
	if err != nil {
		t.Fatalf("read back: %v", err)
	}
	if string(got) != "NEW BINARY" {
		t.Fatalf("binary content = %q, want %q", got, "NEW BINARY")
	}

	info, err := os.Stat(target)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if info.Mode().Perm() != 0o755 {
		t.Fatalf("mode = %v, want 0755", info.Mode().Perm())
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("readdir: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("directory holds %d entries, want 1 — a temp file leaked", len(entries))
	}
}

func TestReplaceBinaryLeavesOriginalIntactOnChecksumMismatch(t *testing.T) {
	archive := buildArchive(t, "NEW BINARY")
	srv := newReleaseServer(t, archive, sha256Hex([]byte("something else")))
	defer srv.Close()

	dir := t.TempDir()
	target := filepath.Join(dir, "wtm")
	if err := os.WriteFile(target, []byte("OLD BINARY"), 0o755); err != nil {
		t.Fatalf("seed binary: %v", err)
	}

	err := selfupdate.ReplaceBinary(selfupdate.ReplaceBinaryParams{
		Release:    srv.release(),
		BinaryPath: target,
		GOOS:       "darwin",
		GOARCH:     "arm64",
		Timeout:    5 * time.Second,
	})
	if !errors.Is(err, domain.ErrChecksumMismatch) {
		t.Fatalf("err = %v, want ErrChecksumMismatch", err)
	}

	got, err := os.ReadFile(target)
	if err != nil {
		t.Fatalf("read back: %v", err)
	}
	if string(got) != "OLD BINARY" {
		t.Fatalf("binary content = %q, want the original %q untouched", got, "OLD BINARY")
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("readdir: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("directory holds %d entries, want 1 — a temp file leaked", len(entries))
	}
}

func TestReplaceBinaryMissingAssetForPlatform(t *testing.T) {
	archive := buildArchive(t, "NEW BINARY")
	srv := newReleaseServer(t, archive, sha256Hex(archive))
	defer srv.Close()

	dir := t.TempDir()
	target := filepath.Join(dir, "wtm")
	if err := os.WriteFile(target, []byte("OLD BINARY"), 0o755); err != nil {
		t.Fatalf("seed binary: %v", err)
	}

	err := selfupdate.ReplaceBinary(selfupdate.ReplaceBinaryParams{
		Release:    srv.release(),
		BinaryPath: target,
		GOOS:       "linux",
		GOARCH:     "amd64",
		Timeout:    5 * time.Second,
	})
	if !errors.Is(err, domain.ErrReleaseAssetMissing) {
		t.Fatalf("err = %v, want ErrReleaseAssetMissing", err)
	}
}

func TestReplaceBinaryReadOnlyDirectory(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root bypasses directory permissions")
	}

	archive := buildArchive(t, "NEW BINARY")
	srv := newReleaseServer(t, archive, sha256Hex(archive))
	defer srv.Close()

	dir := t.TempDir()
	target := filepath.Join(dir, "wtm")
	if err := os.WriteFile(target, []byte("OLD BINARY"), 0o755); err != nil {
		t.Fatalf("seed binary: %v", err)
	}
	if err := os.Chmod(dir, 0o555); err != nil {
		t.Fatalf("chmod dir: %v", err)
	}
	t.Cleanup(func() { _ = os.Chmod(dir, 0o755) })

	err := selfupdate.ReplaceBinary(selfupdate.ReplaceBinaryParams{
		Release:    srv.release(),
		BinaryPath: target,
		GOOS:       "darwin",
		GOARCH:     "arm64",
		Timeout:    5 * time.Second,
	})
	if !errors.Is(err, domain.ErrUpgradeNotWritable) {
		t.Fatalf("err = %v, want ErrUpgradeNotWritable", err)
	}
}
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test ./internal/service/selfupdate/ -run TestReplaceBinary -v`
Expected: FAIL — `undefined: selfupdate.ReplaceBinary`.

- [ ] **Step 3: Implement**

Create `internal/service/selfupdate/replace.go`:

```go
package selfupdate

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/rules"
)

const maxArchiveBytes = 100 << 20

type ReplaceBinaryParams struct {
	Release    domain.ReleaseInfo
	BinaryPath string
	GOOS       string
	GOARCH     string
	UserAgent  string
	Timeout    time.Duration
}

func ReplaceBinary(params ReplaceBinaryParams) error {
	assetName := rules.ReleaseAssetName(rules.ReleaseAssetNameParams{
		Version: params.Release.Version,
		GOOS:    params.GOOS,
		GOARCH:  params.GOARCH,
	})

	assetURL, ok := findAsset(params.Release, assetName)
	if !ok {
		return fmt.Errorf("%w: %s", domain.ErrReleaseAssetMissing, assetName)
	}

	checksumsURL, ok := findAsset(params.Release, domain.ChecksumsFileName)
	if !ok {
		return fmt.Errorf("%w: %s", domain.ErrReleaseAssetMissing, domain.ChecksumsFileName)
	}

	client := &http.Client{Timeout: params.Timeout}

	archive, err := download(client, assetURL, params.UserAgent)
	if err != nil {
		return err
	}

	manifest, err := download(client, checksumsURL, params.UserAgent)
	if err != nil {
		return err
	}

	want, ok := checksumFor(string(manifest), assetName)
	if !ok {
		return fmt.Errorf("%w: %s absent from %s", domain.ErrChecksumMismatch, assetName, domain.ChecksumsFileName)
	}

	sum := sha256.Sum256(archive)
	if hex.EncodeToString(sum[:]) != want {
		return domain.ErrChecksumMismatch
	}

	binary, err := extractBinary(archive)
	if err != nil {
		return err
	}

	return swap(params.BinaryPath, binary)
}

func findAsset(release domain.ReleaseInfo, name string) (string, bool) {
	for _, asset := range release.Assets {
		if asset.Name == name {
			return asset.URL, true
		}
	}
	return "", false
}

func download(client *http.Client, url string, userAgent string) ([]byte, error) {
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	if userAgent != "" {
		req.Header.Set("User-Agent", userAgent)
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("download %s: %s", url, resp.Status)
	}

	return io.ReadAll(io.LimitReader(resp.Body, maxArchiveBytes))
}

func checksumFor(manifest string, assetName string) (string, bool) {
	for _, line := range strings.Split(manifest, "\n") {
		fields := strings.Fields(line)
		if len(fields) == 2 && fields[1] == assetName {
			return fields[0], true
		}
	}
	return "", false
}

func extractBinary(archive []byte) ([]byte, error) {
	gz, err := gzip.NewReader(bytes.NewReader(archive))
	if err != nil {
		return nil, fmt.Errorf("open archive: %w", err)
	}
	defer gz.Close()

	tr := tar.NewReader(gz)
	for {
		header, err := tr.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("read archive: %w", err)
		}
		if filepath.Base(header.Name) != domain.AppName {
			continue
		}

		return io.ReadAll(io.LimitReader(tr, maxArchiveBytes))
	}

	return nil, fmt.Errorf("%w: no %s entry in the archive", domain.ErrReleaseAssetMissing, domain.AppName)
}

// swap writes the new binary next to the current one — same filesystem, so the
// rename is atomic — then renames it over the target.
func swap(binaryPath string, content []byte) error {
	dir := filepath.Dir(binaryPath)

	tmp, err := os.CreateTemp(dir, "."+domain.AppName+"-upgrade-*")
	if err != nil {
		if errors.Is(err, os.ErrPermission) {
			return domain.ErrUpgradeNotWritable
		}
		return err
	}
	tmpPath := tmp.Name()

	cleanup := func() { _ = os.Remove(tmpPath) }

	if _, err := tmp.Write(content); err != nil {
		tmp.Close()
		cleanup()
		return err
	}
	if err := tmp.Close(); err != nil {
		cleanup()
		return err
	}
	if err := os.Chmod(tmpPath, 0o755); err != nil {
		cleanup()
		return err
	}
	if err := os.Rename(tmpPath, binaryPath); err != nil {
		cleanup()
		if errors.Is(err, os.ErrPermission) {
			return domain.ErrUpgradeNotWritable
		}
		return err
	}

	return nil
}
```

- [ ] **Step 4: Run the tests**

Run: `go test ./internal/service/selfupdate/ -race -count=1 -v`
Expected: PASS. If `TestReplaceBinaryReadOnlyDirectory` fails with a non-permission error, check that `os.CreateTemp` in a `0555` directory returns `os.ErrPermission` on this platform before adjusting the mapping.

- [ ] **Step 5: Commit**

```bash
git add internal/service/selfupdate/replace.go internal/service/selfupdate/replace_test.go
git commit -m "feat(upgrade): verified atomic binary replacement"
```

---

### Task 9: `selfupdate.Delegate` and update state

**Files:**
- Create: `internal/service/selfupdate/delegate.go`
- Create: `internal/service/selfupdate/state.go`
- Test: `internal/service/selfupdate/state_test.go`

**Interfaces:**
- Consumes: `domain.InstallMethod`, `domain.UpdateState`, `rules.UpgradeCommandFor`
- Produces: `selfupdate.DelegateParams{Method domain.InstallMethod; Stdout, Stderr io.Writer}`, `selfupdate.Delegate(params DelegateParams) (ran bool, err error)`; `selfupdate.StatePath() (string, error)`, `selfupdate.LoadState() domain.UpdateState`, `selfupdate.SaveState(state domain.UpdateState) error`

- [ ] **Step 1: Write the failing state test**

Create `internal/service/selfupdate/state_test.go`:

```go
package selfupdate_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/service/selfupdate"
)

func TestSaveAndLoadState(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("HOME", t.TempDir())

	want := domain.UpdateState{
		CheckedAt:     time.Date(2026, 8, 20, 12, 0, 0, 0, time.UTC),
		LatestVersion: "0.27.0",
	}
	if err := selfupdate.SaveState(want); err != nil {
		t.Fatalf("SaveState: %v", err)
	}

	got := selfupdate.LoadState()
	if !got.CheckedAt.Equal(want.CheckedAt) {
		t.Fatalf("CheckedAt = %v, want %v", got.CheckedAt, want.CheckedAt)
	}
	if got.LatestVersion != want.LatestVersion {
		t.Fatalf("LatestVersion = %q, want %q", got.LatestVersion, want.LatestVersion)
	}
}

func TestLoadStateMissingFileIsZero(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("HOME", t.TempDir())

	got := selfupdate.LoadState()
	if !got.CheckedAt.IsZero() {
		t.Fatalf("CheckedAt = %v, want zero on a missing state file", got.CheckedAt)
	}
}

func TestLoadStateCorruptFileIsZero(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	t.Setenv("HOME", t.TempDir())

	path, err := selfupdate.StatePath()
	if err != nil {
		t.Fatalf("StatePath: %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(path, []byte("{not json"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	got := selfupdate.LoadState()
	if !got.CheckedAt.IsZero() {
		t.Fatalf("CheckedAt = %v, want zero on a corrupt state file", got.CheckedAt)
	}
}
```

`os.UserConfigDir` honors `XDG_CONFIG_HOME` on Linux and `HOME` on macOS; setting both keeps the test hermetic on either platform. Verify the resolved path lands in the temp dir if a case fails.

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test ./internal/service/selfupdate/ -run TestState -v; go test ./internal/service/selfupdate/ -run TestSaveAndLoadState -v`
Expected: FAIL — `undefined: selfupdate.SaveState`.

- [ ] **Step 3: Implement the state store**

Create `internal/service/selfupdate/state.go`:

```go
package selfupdate

import (
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/LucasPcq/wtm/internal/domain"
)

func StatePath() (string, error) {
	configDir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}

	return filepath.Join(configDir, domain.GlobalConfigDir, domain.GlobalStateFile), nil
}

// LoadState reads the update-check state. A missing or corrupt file means
// "never checked", never an error: the notifier must not fail a command.
func LoadState() domain.UpdateState {
	path, err := StatePath()
	if err != nil {
		return domain.UpdateState{}
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return domain.UpdateState{}
	}

	var state domain.UpdateState
	if err := json.Unmarshal(data, &state); err != nil {
		return domain.UpdateState{}
	}

	return state
}

func SaveState(state domain.UpdateState) error {
	path, err := StatePath()
	if err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}

	data, err := json.Marshal(state)
	if err != nil {
		return err
	}

	return os.WriteFile(path, data, 0o644)
}
```

- [ ] **Step 4: Implement delegation**

Create `internal/service/selfupdate/delegate.go`:

```go
package selfupdate

import (
	"io"
	"os/exec"

	"github.com/LucasPcq/wtm/internal/domain"
)

type DelegateParams struct {
	Method domain.InstallMethod
	Stdout io.Writer
	Stderr io.Writer
}

// Delegate hands the upgrade to the package manager that owns the binary.
// It reports ran=false when that tool is absent from PATH, so the caller can
// print the command instead of failing.
func Delegate(params DelegateParams) (ran bool, err error) {
	commands, ok := delegatedCommands(params.Method)
	if !ok {
		return false, nil
	}

	if _, err := exec.LookPath(commands[0][0]); err != nil {
		return false, nil
	}

	for _, argv := range commands {
		cmd := exec.Command(argv[0], argv[1:]...)
		cmd.Stdout = params.Stdout
		cmd.Stderr = params.Stderr
		if err := cmd.Run(); err != nil {
			return true, err
		}
	}

	return true, nil
}

func delegatedCommands(method domain.InstallMethod) ([][]string, bool) {
	switch method {
	case domain.InstallHomebrew:
		return [][]string{
			{"brew", "update"},
			{"brew", "upgrade", domain.BrewFormula},
		}, true
	case domain.InstallGoInstall:
		return [][]string{
			{"go", "install", domain.ModulePath + "@latest"},
		}, true
	default:
		return nil, false
	}
}
```

`brew update` runs first: without it the tap never sees the new formula.

- [ ] **Step 5: Run the tests**

Run: `go test ./internal/service/selfupdate/ -race -count=1`
Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add internal/service/selfupdate/delegate.go internal/service/selfupdate/state.go internal/service/selfupdate/state_test.go
git commit -m "feat(upgrade): package-manager delegation and update-check state"
```

---

### Task 10: `output/upgrade.go`

**Files:**
- Create: `internal/output/upgrade.go`
- Test: `internal/output/upgrade_test.go`

**Interfaces:**
- Consumes: `domain.UpgradeResult`, `domain.InstallMethod`, `rules.UpgradeCommandFor`, existing `output.Success/Message/InfoLine/Blank`, `output.encodeJSON`
- Produces: `output.UpgradeResultJSON(w io.Writer, result domain.UpgradeResult) error`, `output.UpgradeReport(w io.Writer, result domain.UpgradeResult)`, `output.UpdateNotice(w io.Writer, params UpdateNoticeParams)` with `UpdateNoticeParams{Current, Latest string; Method domain.InstallMethod}`

- [ ] **Step 1: Write the failing test**

Create `internal/output/upgrade_test.go`:

```go
package output_test

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/output"
)

func TestUpgradeResultJSON(t *testing.T) {
	var buf bytes.Buffer
	err := output.UpgradeResultJSON(&buf, domain.UpgradeResult{
		Installed: "0.26.1",
		Latest:    "0.27.0",
		UpToDate:  false,
		Method:    domain.InstallStandalone,
		Action:    domain.UpgradeActionReplaced,
	})
	if err != nil {
		t.Fatalf("UpgradeResultJSON: %v", err)
	}

	var got map[string]any
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	want := map[string]any{
		"installed":  "0.26.1",
		"latest":     "0.27.0",
		"up_to_date": false,
		"method":     "standalone",
		"action":     "replaced",
	}
	for key, value := range want {
		if got[key] != value {
			t.Fatalf("json[%q] = %v, want %v", key, got[key], value)
		}
	}
	if len(got) != len(want) {
		t.Fatalf("json has %d keys, want %d: %v", len(got), len(want), got)
	}
}

func TestUpdateNoticeNamesTheCommand(t *testing.T) {
	cases := []struct {
		name   string
		method domain.InstallMethod
		want   string
	}{
		{"standalone points at wtm upgrade", domain.InstallStandalone, "wtm upgrade"},
		{"homebrew points at brew", domain.InstallHomebrew, "brew upgrade LucasPcq/tap/wtm"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var buf bytes.Buffer
			output.UpdateNotice(&buf, output.UpdateNoticeParams{
				Current: "0.26.1",
				Latest:  "0.27.0",
				Method:  tc.method,
			})

			got := buf.String()
			for _, fragment := range []string{"0.26.1", "0.27.0", tc.want} {
				if !strings.Contains(got, fragment) {
					t.Fatalf("notice %q does not mention %q", got, fragment)
				}
			}
			if strings.HasPrefix(got, "\n") {
				t.Fatal("the notice must not frame itself with a leading blank line")
			}
		})
	}
}
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `go test ./internal/output/ -run "TestUpgrade|TestUpdateNotice" -v`
Expected: FAIL — `undefined: output.UpgradeResultJSON`.

- [ ] **Step 3: Implement**

Create `internal/output/upgrade.go`:

```go
package output

import (
	"fmt"
	"io"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/rules"
	"github.com/LucasPcq/wtm/internal/styles"
)

func UpgradeResultJSON(w io.Writer, result domain.UpgradeResult) error {
	return encodeJSON(w, result)
}

// UpgradeReport renders the raw body of a wtm upgrade run; the command frames it.
func UpgradeReport(w io.Writer, result domain.UpgradeResult) {
	if result.UpToDate {
		Unchanged(w, fmt.Sprintf("%s %s is already the latest release", domain.AppName, result.Installed))
		return
	}

	switch result.Action {
	case domain.UpgradeActionReplaced, domain.UpgradeActionDelegated:
		Success(w, fmt.Sprintf("%s %s → %s", domain.AppName, result.Installed, result.Latest))
	case domain.UpgradeActionChecked:
		Update(w, fmt.Sprintf("%s %s → %s available", domain.AppName, result.Installed, result.Latest))
		Message(w, styles.Muted.Render("run "+rules.UpgradeCommandFor(result.Method)))
	default:
		Unchanged(w, fmt.Sprintf("%s %s unchanged", domain.AppName, result.Installed))
	}
}

type UpdateNoticeParams struct {
	Current string
	Latest  string
	Method  domain.InstallMethod
}

// UpdateNotice is the passive notifier line. It is written to stderr outside any
// frame, so it carries no padding of its own.
func UpdateNotice(w io.Writer, params UpdateNoticeParams) {
	body := fmt.Sprintf("%s %s → %s · run `%s`", domain.AppName, params.Current, params.Latest, rules.UpgradeCommandFor(params.Method))
	fmt.Fprintf(w, "%s%s\n", Indent, styles.Muted.Render(body))
}
```

- [ ] **Step 4: Run the tests**

Run: `go test ./internal/output/ -race -count=1`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/output/upgrade.go internal/output/upgrade_test.go
git commit -m "feat(upgrade): upgrade and notice rendering"
```

---

### Task 11: The `wtm upgrade` command

**Files:**
- Create: `internal/commands/upgrade/upgrade.go`
- Modify: `cmd/root.go`

**Interfaces:**
- Consumes: everything from Tasks 1–10
- Produces: `upgrade.NewCmd(params NewCmdParams) *cobra.Command` with `NewCmdParams{Version string}`

- [ ] **Step 1: Implement the command**

Create `internal/commands/upgrade/upgrade.go`:

```go
package upgrade

import (
	"errors"
	"fmt"
	"os"
	"runtime"

	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/LucasPcq/wtm/internal/commands/shared"
	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/output"
	"github.com/LucasPcq/wtm/internal/rules"
	"github.com/LucasPcq/wtm/internal/service/selfupdate"
	"github.com/LucasPcq/wtm/internal/tui/components"
)

type NewCmdParams struct {
	Version string
}

func NewCmd(params NewCmdParams) *cobra.Command {
	cmd := &cobra.Command{
		Use:   domain.CmdUpgrade,
		Short: "Update wtm to the latest release",
		Long: "Bring wtm up to the latest published release, doing the right thing for how it was\n" +
			"installed. A standalone binary is replaced in place after its SHA256 is verified\n" +
			"against the release checksums. A Homebrew or `go install` binary is handed to that\n" +
			"tool instead — replacing a package-manager-owned binary would desynchronize it.\n" +
			"A binary built from source is refused, since no published release corresponds to it.\n" +
			"\n" +
			"--check reports what is available without changing anything. --yes skips the\n" +
			"confirmation (required with --output json). --version pins an explicit release and\n" +
			"applies to standalone installs only.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return run(cmd, params.Version)
		},
	}

	cmd.Flags().Bool(domain.FlagCheck, false, "Report whether a newer release exists without installing anything")
	cmd.Flags().BoolP(domain.FlagYes, "y", false, "Skip the confirmation prompt")
	cmd.Flags().String(domain.FlagVersionPin, "", "Install a specific release instead of the latest (standalone installs only)")
	shared.AddOutputFlag(cmd)

	return cmd
}

func run(cmd *cobra.Command, version string) error {
	check, _ := cmd.Flags().GetBool(domain.FlagCheck)
	yes, _ := cmd.Flags().GetBool(domain.FlagYes)
	pin, _ := cmd.Flags().GetString(domain.FlagVersionPin)
	format, _ := cmd.Flags().GetString(domain.FlagOutput)

	if format == domain.OutputJSON && !yes && !check {
		return errors.New(domain.UpgradeJSONNeedsYes)
	}

	install := selfupdate.DetectInstall(version)
	if install.Method == domain.InstallSource {
		return domain.ErrUpgradeFromSource
	}
	if pin != "" && install.Method != domain.InstallStandalone {
		return errors.New(domain.UpgradePinUnsupported)
	}

	release, err := selfupdate.FetchRelease(selfupdate.FetchReleaseParams{
		Tag:       pin,
		UserAgent: userAgent(version),
		Timeout:   domain.DownloadTimeout,
	})
	if err != nil {
		return err
	}

	result := domain.UpgradeResult{
		Installed: rules.NormalizeVersion(version),
		Latest:    release.Version,
		UpToDate:  !rules.IsNewerVersion(version, release.Version),
		Method:    install.Method,
		Action:    domain.UpgradeActionNone,
	}

	if check {
		result.Action = domain.UpgradeActionChecked
		if result.UpToDate {
			result.Action = domain.UpgradeActionNone
		}
		return report(cmd, format, result)
	}

	if result.UpToDate && pin == "" {
		return report(cmd, format, result)
	}

	interactive := rules.IsHumanFormat(format) && term.IsTerminal(int(os.Stdin.Fd())) && !yes
	if interactive {
		confirmed, err := components.RunStandaloneConfirm(components.NewConfirm(components.NewConfirmParams{
			Title:       fmt.Sprintf("Update %s %s → %s?", domain.AppName, result.Installed, result.Latest),
			Description: rules.UpgradeCommandFor(install.Method),
			DefaultYes:  true,
		}))
		if err != nil {
			return err
		}
		if !confirmed {
			return domain.ErrUserAborted
		}
	}

	if err := apply(cmd, install, release, version); err != nil {
		return err
	}

	result.Action = domain.UpgradeActionReplaced
	if install.Method != domain.InstallStandalone {
		result.Action = domain.UpgradeActionDelegated
	}

	return report(cmd, format, result)
}

func apply(cmd *cobra.Command, install selfupdate.Install, release domain.ReleaseInfo, version string) error {
	if install.Method != domain.InstallStandalone {
		ran, err := selfupdate.Delegate(selfupdate.DelegateParams{
			Method: install.Method,
			Stdout: cmd.OutOrStdout(),
			Stderr: cmd.ErrOrStderr(),
		})
		if err != nil {
			return err
		}
		if !ran {
			output.Frame(cmd.ErrOrStderr(), func() {
				output.Warning(cmd.ErrOrStderr(), fmt.Sprintf("run `%s` to update", rules.UpgradeCommandFor(install.Method)))
			})
		}
		return nil
	}

	return selfupdate.ReplaceBinary(selfupdate.ReplaceBinaryParams{
		Release:    release,
		BinaryPath: install.BinaryPath,
		GOOS:       runtime.GOOS,
		GOARCH:     runtime.GOARCH,
		UserAgent:  userAgent(version),
		Timeout:    domain.DownloadTimeout,
	})
}

func report(cmd *cobra.Command, format string, result domain.UpgradeResult) error {
	w := cmd.OutOrStdout()
	if !rules.IsHumanFormat(format) {
		return output.UpgradeResultJSON(w, result)
	}

	output.Frame(w, func() { output.UpgradeReport(w, result) })

	return nil
}

func userAgent(version string) string {
	return domain.AppName + "/" + rules.NormalizeVersion(version)
}
```

- [ ] **Step 2: Register the command**

In `cmd/root.go`, add the import `"github.com/LucasPcq/wtm/internal/commands/upgrade"` and, inside `init()` next to the other setup commands:

```go
	upgradeCmd := upgrade.NewCmd(upgrade.NewCmdParams{Version: version})
	upgradeCmd.GroupID = domain.CmdGroupSetup
	rootCmd.AddCommand(upgradeCmd)
```

`version` is the package-level variable the ldflag writes — this is the only place the real version enters `internal/`.

- [ ] **Step 3: Build and exercise the command**

```bash
make build
./bin/wtm upgrade --check
./bin/wtm upgrade --check --output json
```

Expected: the dev build refuses with the from-source message and exit code 17 (`echo $?`). To exercise the release path, link a version in:

```bash
go build -ldflags "-X github.com/LucasPcq/wtm/cmd.version=0.1.0" -o /tmp/wtm-old . && /tmp/wtm-old upgrade --check
```

Expected: reports `0.1.0 → <latest>` and names an upgrade command. `--output json` prints the five-key object.

- [ ] **Step 4: Run the full suite**

Run: `make test`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/commands/upgrade cmd/root.go
git commit -m "feat(upgrade): add wtm upgrade command"
```

---

### Task 12: The passive notifier

**Files:**
- Create: `internal/service/selfupdate/notify.go`
- Modify: `internal/domain/config.go`, `internal/config/wtm.toml.tmpl`, `internal/schemas/global.schema.json`
- Modify: `cmd/root.go`

**Interfaces:**
- Consumes: `rules.ShouldCheckUpdate`, `selfupdate.LoadState/SaveState`, `selfupdate.FetchRelease`, `selfupdate.DetectInstall`, `output.UpdateNotice`
- Produces: `selfupdate.StartCheck(params StartCheckParams) *Check` with `StartCheckParams{Version, Format, Command string; ConfigCheck *bool}`, and `(*Check).Notice(timeout time.Duration) (current string, latest string, method domain.InstallMethod, ok bool)` — nil-safe

- [ ] **Step 1: Add the config key**

In `internal/domain/config.go`, add the type and the field:

```go
// UpdateConfig groups update-check preferences. Check is a pointer so an absent
// key is distinguishable from an explicit false, like UIConfig.Animations.
type UpdateConfig struct {
	Check *bool `toml:"check" json:"check"`
}
```

and on `GlobalConfig`:

```go
	Update UpdateConfig `toml:"update"`
```

Add the matching object to `internal/schemas/global.schema.json`, as a sibling of `ui` (the root and every nested object there use `"additionalProperties": false`):

```json
    "update": {
      "type": "object",
      "additionalProperties": false,
      "properties": {
        "check": {
          "type": "boolean",
          "description": "Whether wtm checks GitHub for a newer release at most once a day and prints a notice on stderr. Absent means on; set to false to disable it. WTM_NO_UPDATE_CHECK=1 disables it for a single run."
        }
      }
    }
```

Add a commented `[update]` block to `internal/config/wtm.toml.tmpl` in the style of the existing commented sections. `internal/config/strict.go` rejects unknown keys, so the struct, the schema and the template must agree.

- [ ] **Step 2: Verify the strict decoder accepts the new key**

Run: `go test ./internal/config/ -race -count=1`
Expected: PASS. Then:

```bash
mkdir -p /tmp/wtm-cfg/wtm && printf 'shell = "zsh"\n\n[update]\ncheck = false\n' > /tmp/wtm-cfg/wtm/config.toml
XDG_CONFIG_HOME=/tmp/wtm-cfg HOME=/tmp/wtm-cfg ./bin/wtm list --output json
```

Expected: no "unknown key" error.

- [ ] **Step 3: Implement the notifier**

Create `internal/service/selfupdate/notify.go`:

```go
package selfupdate

import (
	"os"
	"time"

	"golang.org/x/term"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/rules"
)

type StartCheckParams struct {
	Version     string
	Format      string
	Command     string
	ConfigCheck *bool
}

type Check struct {
	done    chan struct{}
	version string
	latest  string
	method  domain.InstallMethod
}

// StartCheck kicks off the passive update check. It returns nil when policy says
// no check may run, so callers can ignore the result entirely.
func StartCheck(params StartCheckParams) *Check {
	state := LoadState()

	allowed := rules.ShouldCheckUpdate(rules.ShouldCheckUpdateParams{
		Version:     params.Version,
		Format:      params.Format,
		Command:     params.Command,
		StderrIsTTY: term.IsTerminal(int(os.Stderr.Fd())),
		CIEnv:       os.Getenv(domain.EnvCI) != "" || os.Getenv(domain.EnvGitHubActions) != "",
		OptOutEnv:   os.Getenv(domain.EnvNoUpdateCheck) != "",
		ConfigCheck: params.ConfigCheck,
		CheckedAt:   state.CheckedAt,
		Now:         time.Now(),
	})
	if !allowed {
		return nil
	}

	check := &Check{done: make(chan struct{}), version: params.Version}

	go func() {
		defer close(check.done)

		release, err := FetchRelease(FetchReleaseParams{
			UserAgent: domain.AppName + "/" + rules.NormalizeVersion(params.Version),
			Timeout:   domain.UpdateCheckTimeout,
		})
		if err != nil {
			return
		}

		_ = SaveState(domain.UpdateState{CheckedAt: time.Now(), LatestVersion: release.Version})

		if !rules.IsNewerVersion(params.Version, release.Version) {
			return
		}

		check.latest = release.Version
		check.method = DetectInstall(params.Version).Method
	}()

	return check
}

// Notice waits at most timeout for the check to land. A check that is still in
// flight is dropped: the state file it writes makes the next run cheap.
func (c *Check) Notice(timeout time.Duration) (current string, latest string, method domain.InstallMethod, ok bool) {
	if c == nil {
		return "", "", "", false
	}

	select {
	case <-c.done:
	case <-time.After(timeout):
		return "", "", "", false
	}

	if c.latest == "" {
		return "", "", "", false
	}

	return rules.NormalizeVersion(c.version), c.latest, c.method, true
}
```

- [ ] **Step 4: Wire it into the root command**

In `cmd/root.go`, add a package-level variable and a `PersistentPreRun`. No subcommand defines one today (`grep -rn "PersistentPreRun" --include="*.go" .` returns nothing), so cobra will always reach this one.

```go
var updateCheck *selfupdate.Check
```

On `rootCmd`:

```go
	PersistentPreRun: func(cmd *cobra.Command, _ []string) {
		format, _ := cmd.Flags().GetString(domain.FlagOutput)
		updateCheck = selfupdate.StartCheck(selfupdate.StartCheckParams{
			Version:     version,
			Format:      format,
			Command:     cmd.Name(),
			ConfigCheck: globalUpdateCheck(),
		})
	},
```

`cmd.Flags().GetString` returns an error for a command with no `--output` flag; the ignored value is the empty string, which `rules.IsHumanFormat` treats as human — the correct default for commands that only print prose.

The global config loader is unexported today (`loadGlobalConfig` in `internal/config/load.go`); the only exported entry point, `config.Load`, needs a project. Export a thin wrapper in `internal/config/load.go`:

```go
// LoadGlobal reads the user-level config on its own, for callers that run
// outside a wtm project (the update notifier).
func LoadGlobal() (domain.GlobalConfig, error) {
	return loadGlobalConfig()
}
```

Then read it in `cmd/root.go`, tolerant of a machine with no config at all — `loadGlobalConfig` already returns a zero value rather than an error when the file is absent:

```go
func globalUpdateCheck() *bool {
	cfg, err := config.LoadGlobal()
	if err != nil {
		return nil
	}
	return cfg.Update.Check
}
```

Add `"github.com/LucasPcq/wtm/internal/config"` to the `cmd/root.go` imports.

At the end of `Execute`, after the error handling and before the successful return, drain the check:

```go
func printUpdateNotice() {
	current, latest, method, ok := updateCheck.Notice(domain.UpdateNoticeWait)
	if !ok {
		return
	}

	output.Blank(os.Stderr)
	output.UpdateNotice(os.Stderr, output.UpdateNoticeParams{Current: current, Latest: latest, Method: method})
}
```

Call `printUpdateNotice()` on both paths of `Execute` — before `os.Exit(rules.ExitCode(err))` and at the end of the success path. `Notice` is nil-safe, so no guard is needed.

- [ ] **Step 5: Verify the notifier end to end**

```bash
go build -ldflags "-X github.com/LucasPcq/wtm/cmd.version=0.1.0" -o /tmp/wtm-old .
rm -f ~/.config/wtm/state.json
/tmp/wtm-old list
```

Expected: normal output, then a muted `wtm 0.1.0 → <latest> · run ...` line on stderr. Verify each suppression axis:

```bash
rm -f ~/.config/wtm/state.json && CI=1 /tmp/wtm-old list                    # no notice
rm -f ~/.config/wtm/state.json && WTM_NO_UPDATE_CHECK=1 /tmp/wtm-old list   # no notice
rm -f ~/.config/wtm/state.json && /tmp/wtm-old list 2>/dev/null             # no notice (stderr not a tty is not testable this way; check the file instead)
rm -f ~/.config/wtm/state.json && /tmp/wtm-old list --output json           # no notice
/tmp/wtm-old list                                                            # second run inside the TTL: no notice
cat ~/.config/wtm/state.json                                                 # checked_at + latest_version present
rm -f ~/.config/wtm/state.json && /tmp/wtm-old shell-init                    # no notice, output stays eval-safe
```

Confirm `eval "$(/tmp/wtm-old shell-init)"` still succeeds — that is the regression the exclusion list exists to prevent.

- [ ] **Step 6: Run the full suite**

Run: `make test`
Expected: PASS

- [ ] **Step 7: Commit**

```bash
git add internal/service/selfupdate/notify.go internal/domain/config.go internal/config internal/schemas cmd/root.go
git commit -m "feat(upgrade): passive update notifier with 24h throttle"
```

---

### Task 13: Documentation and validation

**Files:**
- Modify: `README.md`
- Modify: `internal/commands/agents/assets/using-wtm.skill.md`
- Regenerate: `docs/`

- [ ] **Step 1: Regenerate the command reference**

Run: `make docs`
Expected: a new `docs/wtm_upgrade.md` plus an updated `docs/wtm.md`. Never hand-edit these.

- [ ] **Step 2: Update the README**

Add `upgrade` to the command overview table in the Setup group (same grouping as the root `--help`), and add an "Updating" subsection right after Installation:

```markdown
**Updating**

```bash
wtm upgrade          # update to the latest release
wtm upgrade --check  # see what's available without installing
```

`wtm upgrade` detects how wtm was installed: a standalone binary is replaced in
place after checksum verification, a Homebrew or `go install` binary is handed to
that tool. wtm also checks for new releases at most once a day and prints a notice
on stderr; disable it with `WTM_NO_UPDATE_CHECK=1` or in `~/.config/wtm/config.toml`:

```toml
[update]
check = false
```
```

Do not add a flag table — `wtm upgrade --help` and `docs/` are the source of truth.

- [ ] **Step 3: Update the agent skill**

Open `internal/commands/agents/assets/using-wtm.skill.md`, find how a comparable command is documented (`wtm prune` is the closest: flags, JSON shape, failure semantics), and add `wtm upgrade` in that same format. The content it must carry:

- `wtm upgrade` updates the CLI itself. It is **not** how you update worktrees — that is `wtm sync`.
- `--check` reports availability and changes nothing. `--yes` skips the confirmation and is **required** with `--output json` unless `--check` is passed; without it the command errors naming `--yes`.
- `--version <v>` pins a release and only applies to a standalone install; on a Homebrew or `go install` binary it errors.
- `--output json` emits exactly: `{"installed", "latest", "up_to_date", "method", "action"}` where `method` is one of `homebrew|go-install|standalone|source` and `action` is one of `replaced|delegated|none|checked`.
- Exit code **17** means the upgrade cannot proceed on this install: built from source, or the binary is not writable (re-run with sudo). Exit code 1 covers network and checksum failures.
- An agent should never run `wtm upgrade` without `--check` unless the user asked for it: it replaces the binary the agent is driving.

Also add one line where environment behavior is described: wtm prints an update notice on **stderr** at most once a day, suppressed under `CI`, non-TTY, `--output json`, and `WTM_NO_UPDATE_CHECK=1`. Agents parsing stdout are unaffected.

- [ ] **Step 4: Validate**

Invoke the `build-validator` subagent. It runs `go build`, `go vet`, `staticcheck`, and the test suite.
Expected: no findings.

- [ ] **Step 5: Confirm no new dependencies**

```bash
go mod tidy && git diff --exit-code -- go.mod go.sum
```

Expected: no diff. CI fails the build otherwise.

- [ ] **Step 6: Commit**

```bash
git add README.md docs internal/commands/agents/assets/using-wtm.skill.md
git commit -m "docs: document wtm upgrade and the update notifier"
```

---

## Verification checklist

Before opening the PR:

- [ ] `make test` passes
- [ ] `build-validator` reports no findings
- [ ] `go mod tidy` produces no diff
- [ ] `eval "$(wtm shell-init)"` still works with a stale state file
- [ ] `wtm upgrade --check --output json` prints exactly five keys
- [ ] `wtm upgrade --output json` without `--yes` errors, naming `--yes`
- [ ] A dev build refuses to upgrade and exits 17
- [ ] `grep -rn "worktree-manager-cli" --include="*.go" --include="*.md" --include="*.yaml" --include="*.json" . | grep -v _test.go` is clean apart from `.claude/settings.json`
