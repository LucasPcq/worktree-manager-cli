# CLAUDE.md — Go CLI Development Principles

This file defines the mandatory coding standards for this project.
All contributions must comply. When in doubt, consult the `go-cli` skill.

**Self-maintaining docs:** When a structural or architectural decision changes
(new package, renamed layer, new dependency, new convention), update this file
and/or the relevant skills (`go-cli`, `build-validator`) in the same session.
Standards must always reflect the actual codebase.

**User-facing agent skill:** `internal/commands/agents/assets/using-wtm.skill.md`
is the skill shipped to end users so their LLM can drive the `wtm` CLI. Whenever a
change alters the user-facing command surface or agent-relevant behavior (new/renamed
command or flag, changed `--output json` shape, new failure/abort semantics, changed
interactive-vs-non-interactive behavior), update this skill in the same session so it
stays aligned with the released CLI. Skip purely internal refactors and TUI-only
changes that don't affect how an agent invokes wtm.

**Docs & README:** the full command reference under `docs/` is **generated** from the
Cobra command tree by `tools/gendocs` — never hand-edit it. `README.md` is a lean guide
(concepts + a grouped command-overview table linking into `docs/`), not a flag reference.
Whenever a command or flag is **added, modified, or removed**:
1. run `make docs` to regenerate `docs/` (also runs automatically before `make release`);
2. if a command was added/renamed/removed, update the `README.md` overview table
   (grouped by the same sections as the root `--help`) and, if relevant, the Concepts or
   Configuration sections.
Do **not** re-add per-command flag tables to the README — `wtm <cmd> --help` and `docs/`
are the source of truth. This is mandatory alongside the agent skill above.

Use the fff MCP tools for all file search operations instead of default tools.

---

## 1. Immutability first

Prefer short variable declarations (`:=`) for values that do not change.
Use `var` only for zero-value initialization or package-level declarations.
If a block requires reassigning a variable, extract it into a function.

## 2. Structs for 2+ parameters

Any function or method that accepts 2 or more related inputs must take a single
struct argument. Always initialize structs with named fields.

```go
// ❌
func Connect(host string, port int) error

// ✅
type ConnectParams struct {
  Host string
  Port int
}
func Connect(params ConnectParams) error
```

## 3. Shared types — no duplication

All types, enums, sentinel errors, and constants are defined once in
`internal/domain/`. Import from there everywhere.
Never copy-paste a type across packages.

Pure functions with no I/O (lookups, transforms, classification) live in
`internal/rules/`, not in `internal/domain/` or `internal/service/`.

## 4. Validate all external input

Config files, CLI flags, and environment variables are validated at the
boundary (in `config/` or at command entry). Use `go-playground/validator`
struct tags or explicit guard clauses. Service layer receives only clean data.

## 5. Centralized constants — no magic strings or numbers

Every string key, exit code, flag name, env var name, and format identifier
must be a named constant in `internal/domain/constants.go`.

```go
// ❌
os.Exit(1)
cmd.Flags().String("output", ...)

// ✅
const (
  ExitCodeError   = 1
  FlagOutput      = "output"
)
```

## 6. Early returns — no nesting

Every error or guard condition returns immediately. The happy path is last.
Never nest `if` blocks; flatten with early returns.

## 7. No unsafe type assertions

Always use the comma-ok idiom for type assertions. Prefer typed interfaces and
concrete structs over `any`/`interface{}`. Type at the source, not downstream.

```go
// ❌
s := v.(string)

// ✅
s, ok := v.(string)
if !ok {
  return fmt.Errorf("expected string, got %T", v)
}
```

## 8. Comments — only when necessary

Do not comment what the code already says. Add comments only for:
- Non-obvious algorithmic reasoning
- Workarounds with a reference (issue URL or ticket)
- Godoc on all exported symbols

## 9. Clean architecture layers

```
cmd/                          ← entry points, cobra setup only
internal/
  commands/                   ← flag wiring, delegates to service (zero business logic)
  domain/                     ← types, errors, constants only (no methods, no functions)
  rules/                      ← pure functions (stdlib + domain only, no I/O)
  config/                     ← load & validate config.toml + run.toml from <git-common-dir>/wtm/, plus ~/.config/wtm/config.toml
  service/                    ← impure orchestration only (git exec, I/O, hooks):
    worktree/                 ←   git worktree operations (create, list, remove)
    env/                      ←   .env file provisioning strategies
    hooks/                    ←   on_create hook execution
    shell/                    ←   shell integration generation (zsh, bash, fish)
    integration/              ←   third-party adapters (VS Code, Cursor)
    detect/                   ←   auto-detection (base branch, env files, package manager)
  output/                     ← format and print results (zero decision logic)
  styles/                     ← all Lipgloss styles (only package allowed to instantiate lipgloss.Style)
  tui/                        ← Bubbletea models (zero business logic, rendering only)
  infra/                      ← I/O, git exec, filesystem wrappers
```

**Hard rules:**
- `commands/` has zero business logic
- `domain/` has types, errors, and constants only — no methods, no free functions
- `rules/` imports only stdlib and `internal/domain` — no I/O, no side effects
- `service/` has zero imports of `cobra`, `bubbletea`, `lipgloss`
- `output/` and `tui/` have zero decision logic — only rendering
- `styles/` is the only package allowed to instantiate `lipgloss.Style`

**Vertical spacing (top/bottom padding):** centralized in one place. Each command
frames its human output **exactly once** with `output.Frame` (or the
`output.FrameStart`/`output.FrameEnd` pair for streaming/split-stream); helpers and
formatters return **raw** bodies (no outer blank lines). JSON (`--output json`) and
machine output (shell-eval: `resolve` success, `shell-init`) are never framed.
Route on `rules.IsHumanFormat(format)`. See the `go-cli` skill (Output section) for
the full convention.

**Mutation commands — bypass flags (two orthogonal axes):** every worktree-mutating
command (`create`, `clean`, `sync`, `prune`, `relocate`, `reparent`, `extract`,
`checkout`) exposes bypass on two independent axes. This is the standardized model
(aligned with `gcloud --quiet`, `terraform -input=false`, `apt -y` vs `--force-yes`,
and [clig.dev](https://clig.dev)); every new or refactored mutation command MUST follow it.
- **`--yes` / `-y` = the confirmation/decision axis — runs fully unattended, zero prompts.**
  Every input resolves in one of three ways, no interaction:
  1. **Decision / confirmation** (recap, reparent, push, on-conflict, fast-forward) →
     its flag value, else a documented **safe default** (never destructive: `sync --yes`
     does not push — use `--push`; `extract --yes` aborts on conflict; `clean`/`prune --yes`
     leave orphans unless `--reparent-children`).
  2. **Required selection with no safe default** (which files for `extract`, which
     worktrees for `sync`, source/branch args) → its flag/arg, else **error naming the
     missing flag**. Never fall back to an interactive picker under `--yes`.
  3. A picker only ever runs in a **fully interactive** run (no `--yes`, TTY, human output).
- **`--force` = the safety axis, strictly separate.** It only lifts safety refusals
  (dirty / unpushed / open-PR / locked). It does **not** imply `--yes`: `--force`
  alone still runs the wizard and asks to confirm (thread `--force` into the wizard as a
  preset so refusals are lifted without re-asking). JSON mode requires `--yes`.

Implementation rule: fold `--yes` into the command's `interactive` boolean
(`interactive := isTTY && IsHumanFormat(format) && !yes`); every picker/prompt gates on
`interactive`, and each required-selection guard returns a sentinel error when it is
false. See `internal/commands/wt/extract.go` and `internal/commands/wt/sync.go`
(`resolveSyncSelection`). Route decision defaults through a pure rule where one exists
(`rules.DecidePush` takes a `Yes` field).

**Recap completeness:** every recap builder reads the value from its wizard step,
**else falls back to the flag/arg** that resolved it. A flag must never make a line
disappear from the recap. See each `build*Recap` / `recapStep` (e.g.
`internal/tui/extract` `buildCombinedRecap`, `internal/tui/newwt` `buildCreateRecap`,
`internal/tui/checkout` `buildCheckoutRecap`, `internal/tui/reparent` `recapBody`).

## 10. Validate before commit

Run the **`build-validator`** subagent at the end of every development session
or before any commit. It checks compilation, vet, static analysis, and tests.

```
→ Invoke build-validator before marking any task done.
```

