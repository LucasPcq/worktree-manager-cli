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
Cobra command tree by `tools/gendocs` — never hand-edit it. The one exception is
`docs/dev/`, hand-written developer documentation (architecture, the `flow/` layer, how
to add a mutation command, the run log seam): gendocs only writes `wtm_*.md` at the root of `docs/`, so
that subdirectory survives a regeneration. Keep it in step with the code the same way
this file is. `README.md` is a lean guide
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

## 8. Comments — the exception, not the rule

Aim for **near-zero** comments. A well-named function with a typed signature explains
itself 99% of the time, and a comment that restates the code is noise that buries the
few that matter. Encode the meaning in names and signatures first: `Skip func(Answers)
(skip bool, reason string)` needs no prose, and a named result beats a line describing
the second return value.

Write a comment only when the code cannot carry the information:
- **Why, never what**: a non-obvious decision, an ordering constraint, an invariant a
  reader would otherwise break
- A workaround, with its reference (issue URL or ticket)
- A package comment (`// Package x …`), one line
- Godoc on an exported symbol **only** when its name and signature leave a caller
  guessing — not systematically

Documenting a pattern or an architecture belongs in `docs/` or in this file, not in a
header comment repeated across files. `internal/flow` is the reference for the density
to aim for: 84 comment lines out of 2261, ~3.7%.

**Migration:** the repo predates this rule, so it is applied as files are touched, not
in one sweep. When you modify a file, bring the comments **in that file** into line —
delete the ones that restate the code — in the same change.

## 9. Clean architecture layers

```
cmd/                          ← entry points, cobra setup only
internal/
  commands/                   ← flag wiring, delegates to flow/service (zero business logic)
    ui/                       ←   `wtm ui`: refuses JSON and a missing TTY, then hands off to tui/dashboard
    run/                      ←   `wtm run …`; surface.go is the line-by-line half of the
                                  runlogs seam (text sink + JSON) and picks between it and
                                  tui/runview through `rules.UseRunView`
  domain/                     ← types, errors, constants only (no methods, no functions)
  rules/                      ← pure functions (stdlib + domain only, no I/O)
  config/                     ← load & validate config.toml + run.toml from <git-common-dir>/wtm/, plus ~/.config/wtm/config.toml
  flow/                       ← the flow of each command, surface-independent (see below):
                                the vocabulary (Step, Session, Prompter, Presenter)
    decide/                   ←   branch/env decisions shared by the create-like flows
    create/                   ←   `wtm create`: the run (create.go) + its questions (steps.go)
    clean/                    ←   `wtm clean`: the run (clean.go) + its questions (steps.go)
    reparent/                 ←   `wtm reparent`: the run (reparent.go) + its questions (steps.go)
    prune/                    ←   `wtm prune`: the run (prune.go) + its questions (steps.go)
    sync/                     ←   `wtm sync`: the run (sync.go) + its questions (steps.go)
    runlogs/                  ←   `wtm run up`/`run logs`: the jobs a surface shows,
                                  their live streams, and the profile start sequence
                                  (asks nothing — it reports events instead of steps)
  service/                    ← impure orchestration only (git exec, I/O, hooks):
    worktree/                 ←   git worktree operations (create, list, remove)
    env/                      ←   .env provisioning (create) + drift reconciliation (`wtm env`, sync.go)
    hooks/                    ←   on_create hook execution
    shell/                    ←   shell integration generation (zsh, bash, fish)
    integration/              ←   third-party adapters (VS Code, Cursor)
    detect/                   ←   auto-detection (base branch, env files, package manager)
  output/                     ← format and print results (zero decision logic)
  styles/                     ← all Lipgloss styles (only package allowed to instantiate lipgloss.Style)
  tui/                        ← Bubbletea models (zero business logic, rendering only)
    flowui/                   ←   runs a flow.Session as a wizard (the only translator
                                  between flow.Step and components.Step)
    dashboard/                ←   `wtm ui`: the full-screen worktree dashboard, the second
                                  surface over flow/ (its own Prompter/Presenter, mouse
                                  zones via bubblezone)
    runview/                  ←   `wtm run up`/`start`/`logs`: the full-screen job view,
                                  the second surface over flow/runlogs — a job's raw PTY
                                  output replayed through a terminal emulator
                                  (`github.com/charmbracelet/x/vt`)
  infra/                      ← I/O, git exec, filesystem wrappers
```

**Hard rules:**
- `commands/` has zero business logic
- `domain/` has types, errors, and constants only — no methods, no free functions
- `rules/` imports only stdlib and `internal/domain` — no I/O, no side effects
- `service/` has zero imports of `cobra`, `bubbletea`, `lipgloss`
- `output/` and `tui/` have zero decision logic — only rendering
- `styles/` is the only package allowed to instantiate `lipgloss.Style`
- `flow/` imports **only** `internal/service/`, `internal/rules/`, `internal/domain/`
  and the stdlib — never cobra, bubbletea or lipgloss, and never `output/`, `tui/`,
  `config/` or `commands/`. It therefore cannot reach `infra/` either: add a thin
  `service/` wrapper instead (e.g. `worktree.FindByBranch`, `worktree.ListAll`)

**Vertical spacing (top/bottom padding):** centralized in one place. Each command
frames its human output **exactly once** with `output.Frame` (or the
`output.FrameStart`/`output.FrameEnd` pair for streaming/split-stream); helpers and
formatters return **raw** bodies (no outer blank lines). JSON (`--output json`) and
machine output (shell-eval: `resolve` success, `shell-init`) are never framed.
Route on `rules.IsHumanFormat(format)`. See the `go-cli` skill (Output section) for
the full convention.

**The `flow/` layer (LUC-175).** A command's flow lives in `internal/flow/`, not in
`commands/`: `runCreate`/`runClean` read the flags, decide *who may be asked* and
*where output goes*, then call `create.Run` / `clean.Run`. One package per command,
each splitting the run from the questions it asks. Three seams let a second surface
(`tui/dashboard`) replay the same flow:
- **`flow.Prompter`** answers the questions: `Ask(Session)` for a whole
  question-and-recap sequence, `Confirm` for a standalone post-execution question,
  `Interactive()` to know whether a decision may be offered at all. Implementations:
  `tui/flowui` (the CLI wizard), `flow.Unattended` (`--yes` / no TTY / JSON), and the
  dashboard. `Interactive()` is only ever read to (a) not offer a decision nobody can
  answer and (b) feed a pure rule that takes it as input (`rules.DecidePush`).
- **`flow.Presenter`** shows the phases (`Stage`, `HookPhase`, `Notice`, `Status`) plus
  one typed per-command conclusion (`Created`, `Cleaned`). A flow never frames, never
  animates and never picks a stream; errors are **returned**, never presented.
- **`flow.Request`** (`CreateRequest`, `CleanRequest`) carries what the surface already
  knows. It holds no `--yes` and no output format: the confirmation axis is the
  installed Prompter, the format is the surface. `--force` *does* belong there — it is
  the safety axis, a business input.

Steps are declared as `flow.Step` values (`Kind`, `Key`, `Label`, `Options`, `Skip`,
`Build`, `Load`, `Resolve`, `Summarize`). `Resolve` is the entire bypass taxonomy in
one place: returning an `Answer` is a decision with a safe default, returning an error
refuses the run naming the flag, and a step with no `Resolve` can only be answered
interactively (`flow.Unattended` never falls back to a picker). A value the request
already carries goes in `Session.Presets`: the step is not asked but is still read
back, which is what keeps a flag from erasing a recap line. The `StepContent` a step
builds also carries `Blockers` — the safety refusals standing in the way of its
dangerous option, named one by one instead of folded into the prose, so a surface can
have each of them lifted separately (`rules.CleanBlockers` feeds
`internal/flow/clean/steps.go`).

**`flow.Operation`** (`Kind`, `Mode`, `TargetKey`) is what a flow declares about *how it
is scheduled*, for a surface that runs several at once. `Mode` says how long it holds
that surface — `ModeBlocking` (`clean`) keeps it until the run ends, `ModeBackground`
(`create`) gives it back and locks its target instead — and `TargetKey` names the answer
carrying the worktree it locks, known only once that step is answered. The CLI ignores it
(one run, one terminal); `internal/tui/dashboard/ops.go` is where it is enforced, once,
rather than at every action site.

Adding a kind means teaching every surface to render it: `flowui` refuses an unknown
kind rather than guessing. Test doubles for the two seams live in
`internal/testutil/flowtest`. `create`, `clean`, `reparent`, `prune` and `sync` are
migrated; `extract` and the other commands still drive their wizard packages
directly, and `tui/newwt` stays until `extract` (which embeds it) migrates.

A **non-mutating mode** (`prune --dry-run`) belongs in the `Request`, not in the runner:
it changes what the run does, not how it reads. The flow returns its `Outcome` before
asking anything and before touching anything, and any rule that reads `Interactive()`
must take the mode as an input too — a surface may install an interactive Prompter for a
preview. See `rules.PruneClassifyForce` and `docs/dev/flow-layer.md`.

**Every new worktree-mutating command goes through `flow/`** — no exception, and no new
command written on the pre-`flow` model even to match a neighbour that has not migrated
yet. Concretely: declare `Request`/`Outcome`/`Presenter`/`Params`/`Run` in
`internal/flow/<cmd>/`, its questions as `flow.Step` in `steps.go`, and keep the runner
in `commands/` to flags → `Request` → pick the two seams → `<cmd>.Run`. A runner that
inspects state, orders service calls or gates a picker on `interactive` beyond choosing
the Prompter has put the flow in the wrong layer, and a service closure injected into a
`tui/` package is the same mistake in its older form. The developer reference is
`docs/dev/` (`flow-layer.md`, `adding-a-mutation-command.md`, `run-log-seam.md`); the import rule above is
checked mechanically by the `build-validator` subagent.

**Mutation commands — bypass flags (two orthogonal axes):** every worktree-mutating
command (`create`, `clean`, `sync`, `prune`, `relocate`, `reparent`, `extract`,
`checkout`, `env`) exposes bypass on two independent axes. This is the standardized model
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
false. See `internal/commands/wt/extract.go`, and — for a migrated command —
`internal/flow/sync/steps.go` (`selectionStep`'s `Resolve`, which names `--all` instead
of falling back to a picker). Route decision defaults through a pure rule where one
exists (`rules.DecidePush` takes a `Yes` field).

**Recap completeness:** every recap builder reads the value from its wizard step,
**else falls back to the flag/arg** that resolved it. A flag must never make a line
disappear from the recap. A migrated command gets this from `Session.Presets` (a preset
step is not asked but is still read back — see `internal/flow/create/steps.go`
`createFlow.recap`);
the others do it in their recap builder (e.g. `internal/tui/extract`
`buildCombinedRecap`, `internal/tui/newwt` `buildCreateRecap`, `internal/tui/checkout`
`buildCheckoutRecap`).

## 10. Validate before commit

Run the **`build-validator`** subagent at the end of every development session
or before any commit. It checks compilation, vet, static analysis, and tests.

```
→ Invoke build-validator before marking any task done.
```

