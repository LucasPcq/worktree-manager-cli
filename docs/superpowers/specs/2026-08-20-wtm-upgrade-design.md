# `wtm upgrade` — self-update and update notifier

Date: 2026-08-20
Status: approved, ready for implementation planning

## Problem

`wtm` ships three ways (Homebrew tap, standalone tarball, `go install`) and has no
story for staying current. Users are told to run `brew upgrade` by hand, which
only covers one of the three, and nothing ever tells them a new version exists.
Release cadence is high (v0.26.1 today), so users silently run stale binaries.

## Goals

1. A user on any install method learns a new version exists, without asking.
2. A single command brings them to the latest version, doing the right thing for
   how they installed.
3. Neither behavior may corrupt a package-manager-owned install, break shell-eval
   output, or fire in CI.

## Non-goals

- Silent background auto-update. Rejected: standard for end-user apps, alien to
  dev CLIs, and a version changing mid-workflow is a support burden.
- Signature verification (cosign / SLSA attestation). SHA256 over HTTPS is the
  de-facto standard for Go CLIs; revisit if the threat model changes.
- Windows support. `.goreleaser.yaml` builds darwin and linux only.
- A `flow/` migration. `upgrade` mutates no worktree.

## Decisions

| Question | Decision | Rationale |
|---|---|---|
| Command name | `wtm upgrade` | `update` reads as "update my worktrees", which is `wtm sync`. Matches deno/bun/brew. |
| Scope | Hybrid: self-replace for standalone, delegate for brew/go | Replacing a Homebrew-owned binary desynchronizes the Cellar and breaks the next `brew upgrade`. |
| Delegation | Execute the package-manager command, don't just print it | A command that only echoes another command is the frustration we are fixing. Falls back to printing when the tool is absent from `PATH`. |
| Notifier | Yes, 24h throttle, stderr | The gh/npm/deno convention. stdout would break pipes. |
| Auto-update | No | See non-goals. |
| `flow/` | No | No worktree mutation, no multi-step session, one binary question. Uses `components.NewConfirm`. |
| `--force` | Absent | The safety axis has nothing to lift here. `--yes` alone is the confirmation axis. |
| Release source | GitHub Releases REST API over `net/http` | `gh` is an optional dependency of wtm and must not become required. |

## Architecture

```
internal/domain/            InstallMethod, ReleaseInfo, UpdateState, errors, constants
internal/rules/upgrade.go   ClassifyInstall, IsNewerVersion, ShouldCheckUpdate,
                            UpgradeCommandFor, ReleaseAssetName
internal/service/selfupdate/
    detect.go               DetectInstall  (os.Executable, EvalSymlinks, go env)
    release.go              FetchLatest    (net/http)
    replace.go              ReplaceBinary  (download, SHA256, atomic rename)
    delegate.go             RunPackageManager
    state.go                LoadState / SaveState
    notify.go               CheckAsync / Result
internal/output/upgrade.go  human + JSON rendering, zero decisions
internal/commands/upgrade/  flags -> service
cmd/root.go                 notifier wiring
```

Layer rules hold: `rules/` imports stdlib + `domain` only, `service/` imports no
cobra/bubbletea/lipgloss, `output/` decides nothing.

### Domain additions

```go
type InstallMethod string

const (
    InstallHomebrew   InstallMethod = "homebrew"
    InstallGoInstall  InstallMethod = "go-install"
    InstallStandalone InstallMethod = "standalone"
    InstallSource     InstallMethod = "source"
)

type ReleaseInfo struct {
    Version string        // normalized, no leading "v"
    Tag     string        // as published, e.g. "v0.26.1"
    URL     string        // release page
    Assets  []ReleaseAsset
}

type ReleaseAsset struct {
    Name string
    URL  string
}

type UpdateState struct {
    CheckedAt     time.Time `json:"checked_at"`
    LatestVersion string    `json:"latest_version"`
}
```

Constants in `internal/domain/constants.go`: `RepoOwner = "LucasPcq"`,
`RepoName = "wtm"`, `ReleaseAPIURL`, `ChecksumsFileName = "checksums.txt"`,
`BrewFormula = "LucasPcq/tap/wtm"`, `ModulePath = "github.com/LucasPcq/wtm"`,
`GlobalStateFile = "state.json"`, `UpdateCheckTTL = 24 * time.Hour`,
`EnvNoUpdateCheck = "WTM_NO_UPDATE_CHECK"`, `FlagCheck = "check"`,
`FlagVersion = "version"`, `ExitCodeUpgradeUnsupported = 17`.

Errors: `ErrUpgradeFromSource`, `ErrUpgradeNotWritable`, `ErrChecksumMismatch`,
`ErrReleaseAssetMissing`, `ErrUpgradeNeedsYes`. `rules.ExitCode` maps
`ErrUpgradeFromSource` and `ErrUpgradeNotWritable` to
`ExitCodeUpgradeUnsupported`.

### Install detection

`service/selfupdate.DetectInstall` collects facts, `rules.ClassifyInstall` decides.
Keeping the decision pure is what makes the table below testable without a
filesystem.

```go
type ClassifyInstallParams struct {
    ExecPath     string // os.Executable()
    ResolvedPath string // after filepath.EvalSymlinks
    GoBinDir     string // go env GOBIN, else $GOPATH/bin, else ~/go/bin
    Version      string // the linked version, see "Version source" below
}

func ClassifyInstall(params ClassifyInstallParams) domain.InstallMethod
```

Order matters — first match wins:

| Condition | Method |
|---|---|
| `Version == "dev"` | `InstallSource` |
| `ResolvedPath` contains a `/Cellar/` segment | `InstallHomebrew` |
| `ResolvedPath` is inside `GoBinDir` | `InstallGoInstall` |
| otherwise | `InstallStandalone` |

The Cellar check runs on the resolved path because Homebrew installs
`/opt/homebrew/bin/wtm` as a symlink into `/opt/homebrew/Cellar/wtm/<v>/bin/wtm`.
Matching on a path *segment* (not a substring) avoids a false positive on a user
directory literally named `Cellar`.

`Version == "dev"` first: a `make install` binary lands in `GoBinDir` and would
otherwise classify as `go-install`, sending the user to fetch a published release
over their own build.

### Version source

`.goreleaser.yaml` links `-X github.com/LucasPcq/wtm/cmd.version=...`, so the only
variable carrying a real version in a released binary is `cmd.version`.
`domain.Version` stays `"dev"` in every build — it is the fallback initializer,
not the value. Every consumer (`ClassifyInstall`, `ShouldCheckUpdate`,
`IsNewerVersion`, the upgrade command, the notifier) therefore receives the
version as an explicit input threaded down from `cmd`, and no package under
`internal/` may read `domain.Version` directly. `cmd/root.go` exposes it via a
small accessor so `commands/upgrade` can reach it without importing `cmd`.

Concretely: `internal/commands/upgrade` takes the version through its
`NewCmd` params, wired in `cmd/root.go` where `version` is in scope. A future
release that drops the ldflag would silently disable both features, so
`ShouldCheckUpdate` treating `"dev"` as "never check" is the intended fail-safe.

### Version comparison

`rules.IsNewerVersion(current, latest string) bool` parses `[v]MAJOR.MINOR.PATCH`
with an optional pre-release suffix. Rules: numeric field comparison; a
pre-release sorts below its release (`1.0.0-rc1 < 1.0.0`); an unparseable version
on either side returns false (never notify on garbage); `current == "dev"` returns
false.

### `wtm upgrade` behavior per method

**Standalone** — the only path that touches a binary:

1. `FetchLatest` → tag, assets.
2. If not newer than the running version and `--version` was not passed: report
   up to date, exit 0.
3. Download `wtm_<version>_<goos>_<goarch>.tar.gz` and `checksums.txt`.
4. Verify SHA256 against `checksums.txt`. Mismatch aborts before any write.
5. Extract the `wtm` entry to a temp file **in the target binary's own
   directory** — same filesystem, so the rename is atomic; `/tmp` may be a
   different mount. `chmod 0755`.
6. `os.Rename` over the current binary. On `EPERM`, return
   `ErrUpgradeNotWritable` naming `sudo wtm upgrade`. Never invoke `sudo`.
7. Remove the temp file on every failure path.

**Homebrew** — `brew upgrade LucasPcq/tap/wtm`, streamed to the user's terminal.
Preceded by `brew update` so the tap sees the new version.

**GoInstall** — `go install github.com/LucasPcq/wtm@latest`.

For both delegated methods, if the tool is missing from `PATH` the command prints
the exact line to run and exits 0.

**Source** — refuse with `ErrUpgradeFromSource`, pointing at `git pull && make install`.

### CLI surface

```
wtm upgrade                   # detect, confirm, upgrade
wtm upgrade --yes             # unattended
wtm upgrade --check           # report only, never mutates, exit 0
wtm upgrade --version 0.25.0  # pin/rollback, standalone only
wtm upgrade --output json     # requires --yes unless --check
```

`--output json` on a real run without `--yes` returns `ErrUpgradeNeedsYes`, per
the two-axis rule in CLAUDE.md. `--version` on a delegated method errors naming
the package manager instead (brew and go pin their own way).

JSON shape:

```json
{
  "installed": "0.26.1",
  "latest": "0.27.0",
  "up_to_date": false,
  "method": "standalone",
  "action": "replaced"
}
```

`action` is one of `replaced`, `delegated`, `none`, `checked`.

### Notifier

`rules.ShouldCheckUpdate` is pure and takes every input as data:

```go
type ShouldCheckUpdateParams struct {
    Version      string
    Format       string
    Command      string
    StderrIsTTY  bool
    CIEnv        bool   // CI or GITHUB_ACTIONS set
    OptOutEnv    bool   // WTM_NO_UPDATE_CHECK set
    ConfigCheck  *bool  // [update] check in the global config; nil = unset
    CheckedAt    time.Time
    Now          time.Time
}
```

Returns false when: version is `dev`, format is JSON, stderr is not a TTY, CI is
set, opt-out is set, config says false, the command is excluded, or
`Now.Sub(CheckedAt) < UpdateCheckTTL`.

Excluded commands: `shell-init`, `resolve`, `daemon`, `completion`, `schema`,
`upgrade`. The first two are evaluated by the shell — a stray byte breaks them.

Wiring in `cmd/root.go`: `PersistentPreRun` starts a goroutine with a 2s HTTP
timeout; `Execute` reads the result at the end, waiting at most 300ms. A check
that does not finish in time still writes the state file, so the next run is
cheap. Network failure is silent — a notifier never turns into an error.

Rendered to **stderr**, outside `output.Frame`:

```
wtm 0.26.1 → 0.27.0 · run `wtm upgrade`
```

The trailing hint uses `rules.UpgradeCommandFor(method)`, so a Homebrew user sees
`brew upgrade LucasPcq/tap/wtm` if delegation is what they will get.

### Config and state

`domain.GlobalConfig` gains:

```go
type UpdateConfig struct {
    Check *bool `toml:"check" json:"check"`
}
```

Pointer so "unset" differs from an explicit `false`, matching `UIConfig.Animations`.
The JSON schema in `internal/schemas/global.schema.json` and the config template
gain the key.

State lives in `~/.config/wtm/state.json`, separate from `config.toml`: the CLI
must never rewrite a file the user hand-edits. A missing or corrupt state file is
treated as "never checked", not an error.

## Testing

- `rules/upgrade_test.go`: table-driven `ClassifyInstall` over real-world paths
  (`/opt/homebrew/Cellar/...`, `/usr/local/Cellar/...`, `/home/linuxbrew/...`,
  `~/go/bin/wtm`, `/usr/local/bin/wtm`, a dir named `Cellar` that is not brew);
  `IsNewerVersion` including pre-releases, equal versions, `dev`, garbage;
  `ShouldCheckUpdate` across every suppression axis and the TTL boundary.
- `service/selfupdate`: `httptest.Server` for `FetchLatest` (success, 404,
  rate-limited, malformed JSON); `t.TempDir()` for `ReplaceBinary` — the
  checksum-mismatch case must assert the original binary is byte-identical
  afterwards, and the read-only-dir case must return `ErrUpgradeNotWritable`.
- `output/upgrade_test.go`: JSON shape and the human notice line.

## Documentation obligations

Per CLAUDE.md, in the same change:

1. `make docs` to regenerate `docs/`.
2. README overview table gains `upgrade`; the Installation section gains an
   "Updating" subsection.
3. `internal/commands/agents/assets/using-wtm.skill.md` — new command, new JSON
   shape, new exit code.

## Housekeeping: stale repo links

The repo was renamed to `LucasPcq/wtm`; four references still point at
`worktree-manager-cli` and are fixed here since the release URL is now
load-bearing:

- `.goreleaser.yaml:43` — brew formula `homepage`
- `README.md:31` — releases link
- `README.md:34` — tarball name in the extract example; actual assets are
  `wtm_<version>_<os>_<arch>.tar.gz`
- `internal/schemas/{global,project,run}.schema.json` — `$id` URLs

Occurrences in `internal/tui/dashboard/*_test.go` are fixture strings, not links,
and stay. `.claude/settings.json` carries an OTEL attribute with the old name —
cosmetic, out of scope.
