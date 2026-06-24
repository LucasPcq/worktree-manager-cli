---
name: go-cli
description: Expert guidance for developing CLI applications in Go following strict architectural principles, using Cobra, Bubbletea, and Lipgloss. Use this skill whenever the user is building, extending, or refactoring a Go CLI — including adding commands, flags, subcommands, TUI components, config parsing, output formatting, error handling, or structuring packages. Trigger on any mention of "go CLI", "cobra", "bubbletea", "bubble tea", "lipgloss", "TUI", "go command", "go flag", "go binary", "charm", or any request to scaffold or improve a Go terminal application. Also trigger when the user references the project's CLAUDE.md principles in the context of Go code, when adding a new command, or when discussing test patterns for commands.
---

# Go CLI Development Skill

## Stack

- **Cobra** — command routing and flag parsing
- **BurntSushi/toml** — configuration loading (strict decode with unknown-key rejection)
- **Bubbletea** — TUI programs (Model/Update/View)
- **Lipgloss** — terminal styling, centralized in `internal/styles/`

---

## Core Principles (from CLAUDE.md)

Always apply these rules. They are non-negotiable.

### 1. Immutability first — extract functions, not variables

Prefer short variable declarations (`:=`) for values that do not change.
Extract logic into named functions rather than reassigning variables.

```go
// ❌ Avoid
var result string
if condition {
  result = "a"
} else {
  result = "b"
}

// ✅ Prefer
result := resolveResult(condition)
```

### 2. Structs for 2+ parameters — always use named fields

Any function taking 2 or more related parameters must use a struct. Always
initialize with named fields (no positional struct literals outside tests).

```go
// ❌ Avoid
func RunCommand(name string, timeout int, verbose bool) error

// ✅ Prefer
type RunCommandParams struct {
  Name    string
  Timeout int
  Verbose bool
}
func RunCommand(params RunCommandParams) error
```

### 3. Shared types — single source of truth

All domain types, enums, and error sentinels live in `internal/domain/`
(`constants.go`, `errors.go`, `types.go`, `jobs.go`, `init.go`). Never duplicate a type across packages.

### 4. Validate at the boundary

Validate all external input (flags, env vars, config files) in `internal/config/`
before it reaches the service layer. Use explicit guard clauses.

### 5. Centralized constants — no magic strings or numbers

All flag names, command names, exit codes, and format identifiers
must be declared as constants in `internal/domain/constants.go`.

```go
const (
  // CLI command names — used in Use: and exec.Command(bin, ...) call sites
  CmdRun      = "run"
  CmdUp       = "up"
  CmdDown     = "down"
  CmdStart    = "start"
  CmdStop     = "stop"

  // Flag names
  FlagOutput  = "output"
  FlagForce   = "force"
  FlagProfile = "profile"

  // Output format values
  OutputText  = "text"
  OutputJSON  = "json"
)
```

### 6. Early returns — flatten all conditionals

Never nest `if` blocks. The happy path is always the last statement.

### 7. No unsafe type assertions

Always use the comma-ok idiom. Prefer typed interfaces and concrete structs
over `any`/`interface{}`.

```go
// ❌  val := data.(string)
// ✅
val, ok := data.(string)
if !ok {
  return fmt.Errorf("expected string, got %T", data)
}
```

### 8. Comments — only when necessary

Godoc on all exported symbols. No inline comments that restate the code.
Only comment non-obvious decisions or workarounds (with an issue reference).

### 9. Clean architecture layers

```
cmd/
  root.go                     ← cobra root, version, help

internal/
  commands/                   ← flag wiring only → delegates to service (zero logic)
  domain/                     ← types, errors, constants only (no methods, no functions)
  rules/                      ← pure business rules (stdlib + domain only, no I/O)
  config/                     ← load & validate config.toml + run.toml from <git-common-dir>/wtm/
  service/                    ← impure orchestration only (git exec, I/O, hooks):
    worktree/                 ←   create, list, clean, resolve
    env/                      ←   .env file provisioning strategies
    hooks/                    ←   on_create hook execution
    shell/                    ←   shell integration generation (zsh, bash, fish)
    process/                  ←   daemon, job manager, socket client
    github/                   ←   PR operations via gh CLI
    detect/                   ←   auto-detection (package manager, env files, scripts)
    integration/              ←   third-party adapters (VS Code, Cursor)
  output/                     ← format and print results (zero decision logic)
  styles/                     ← all Lipgloss styles + shared Indent constant
  tui/                        ← Bubbletea models (zero business logic, rendering only)
  infra/                      ← I/O, git exec, filesystem wrappers
  testutil/gittest/           ← shared test helpers (InitRepo, CreateBranch)
```

**Hard rules:**
- `commands/` has zero business logic
- `domain/` imports only stdlib (unchanged)
- `rules/` imports only stdlib + internal/domain
- `service/` has zero imports of `cobra`, `bubbletea`, `lipgloss`
- `output/` and `tui/` have zero decision logic — only rendering
- `styles/` is the only package allowed to instantiate `lipgloss.Style`

### 10. Validate before commit — run `build-validator`

Before marking any task done, invoke the `build-validator` subagent.

---

## Cobra — Command Patterns

### Standard command

The real pattern used in this project — no dependency injection, uses helper functions:

```go
// internal/commands/start.go
func newRunStartCmd() *cobra.Command {
  cmd := &cobra.Command{
    Use:   domain.CmdStart + " <job>",
    Short: "Start a single job",
    Args:  cobra.ExactArgs(1),
    RunE:  runStart,
  }
  addOutputFlag(cmd)
  return cmd
}

func runStart(cmd *cobra.Command, args []string) error {
  dir, err := os.Getwd()
  if err != nil {
    return fmt.Errorf("get working directory: %w", err)
  }

  result, ok := loadConfig(cmd, dir)
  if !ok {
    return nil
  }

  // ... delegate to service, format output
}
```

Key conventions:
- `Use:` always uses `domain.CmdXxx` constants (+ literal arg placeholders)
- `addOutputFlag(cmd)` instead of duplicating the output flag registration
- `shared.LoadConfig(cmd, dir)` resolves the main worktree path **and** the state dir, then loads `<state-dir>/config.toml`. Returns `ConfigResult{Config, ProjectDir, StateDir}`.
- Unexported constructor (`newRunStartCmd`), registered by the parent group

### State-dir resolution

wtm stores all of its state under `<git-common-dir>/wtm/` (i.e. `.git/wtm/` for a normal clone), so nothing leaks into the user's working tree. Resolution helpers live in `internal/commands/shared/`:

```go
shared.StateDir(dir)                   // <git-common-dir>/wtm/
shared.WorktreeStateDir(WorktreeStateDirParams{Dir, Branch})  // <state-dir>/worktrees/<encoded-branch>/
```

The git common dir is fetched via `infra.GitCommonDir(GitCommonDirParams{Dir})` which wraps `git rev-parse --git-common-dir`. Branch names are encoded for filesystem safety with `rules.EncodeBranchSegment(branch)` (slashes → `%2F`).

Two env-var overrides exist for tests / CI:
- `WTM_PROJECT_DIR` — bypasses git for the main worktree path
- `WTM_STATE_DIR`   — bypasses git for the state dir

### Adding a new command

1. Create `internal/commands/<name>.go` with unexported constructor
2. Use `domain.CmdXxx` for the `Use:` field (add constant if new)
3. Register in the parent group's `NewXxxCmd()` function
4. Follow the `runStart` pattern: getwd → loadConfig → delegate → format output

### Shared flag helpers

```go
// internal/commands/helpers.go

// addOutputFlag registers the standard --output flag on cmd.
func addOutputFlag(cmd *cobra.Command) {
  cmd.Flags().String(domain.FlagOutput, domain.OutputText, "Output format: text or json")
}
```

### TUI command — Cobra launches a Bubbletea program

The command runner is the only place where `tea.NewProgram` is called.
All TUI state lives in `internal/tui/`, never in `commands/`.

```go
func runBrowseTUI(ctx context.Context, items []domain.Item) error {
  model := tui.NewBrowseModel(items)
  program := tea.NewProgram(model, tea.WithAltScreen())

  result, err := program.Run()
  if err != nil {
    return fmt.Errorf("tui: %w", err)
  }

  finalModel, ok := result.(tui.BrowseModel)
  if !ok {
    return nil
  }
  return handleBrowseResult(finalModel.Selected())
}
```

---

## Configuration — BurntSushi/toml

The project uses BurntSushi/toml with a custom strict decoder that rejects unknown keys.

```go
// internal/config/strict.go
func decodeStrict(path string, out any, ignorePrefixes ...string) error
func decodeStrictBytes(source string, data []byte, out any, ignorePrefixes ...string) error
```

Struct tags are `toml:"..."` (+ `json:"..."` for types that get JSON-exported).
Config files include a `#:schema ./schemas/xxx.json` directive for editor support.

---

## Bubbletea — TUI Architecture

### Shared components

All reusable TUI primitives live in `internal/tui/components/`:
- `WizardModel` — multi-step form driver
- `SelectListModel` — single-choice list with filter
- `MultiSelectModel` — toggle-able multi-choice list
- `TextInputModel` — single-line input with validation
- `ConfirmModel` — yes/no dialog
- `RunStandaloneSelect(model)` / `RunStandaloneConfirm(model)` — one-shot wrappers

### Screen-specific TUI

Each screen lives in its own package under `internal/tui/`:
```
internal/tui/
  components/    ← shared primitives (wizard, selectlist, multiselect, confirm)
  new/           ← wt create wizard
  pr/            ← pr create wizard, env picker
  run/           ← run list picker, run ps picker
  init/          ← global + project init wizards
  clean/         ← worktree picker + deletion confirm
  worktreepicker/ ← shared worktree-selection picker
```

### Rules
- `Update` is a pure function — no side effects, only return `(tea.Model, tea.Cmd)`
- Never import `lipgloss` in TUI models — delegate all styling to `styles/`
- Never import `cobra` or `service` inside a `tui/` model
- TUI packages may import `domain/` (types) and `styles/` (rendering)

---

## Lipgloss — Centralized Styles

All styles live in `internal/styles/`, split by concern:
- `colors.go` — adaptive color palette
- `text.go` — typography styles + `Indent` constant
- `components.go` — composed styles (list items, badges, inputs)
- `indicators.go` — status indicators (DirtyIndicator, CleanIndicator)

The `Indent` constant is the canonical source for left-padding:
```go
// internal/styles/text.go
const Indent = "  "
```

`output/block.go` aliases it as `output.Indent`. TUI components use `styles.Indent`.
Never write a literal `"  "` for padding — always use the constant.

---

## Output — Formatted Printing

`internal/output/block.go` provides shared helpers for structured terminal output:

```go
output.Blank(w)                    // empty line
output.Success(w, "Done")          // ✓ Done
output.Warning(w, "Be careful")    // ! Be careful
output.Error(w, "Failed")          // ✗ Failed
output.Message(w, "Info")          // plain indented line
output.SectionTitle(w, "TITLE")    // bold title
output.InfoLine(w, "key", "value") // key  value
output.Announce(w, "Title", items) // padded block: blank + title + key-values + blank
```

JSON output uses `encodeJSON(w, v)` (pretty-printed, no HTML escaping).

---

## Testing Patterns

### Unit tests — config/service/domain

Use `t.TempDir()` as the state dir, write fixtures directly inside it (no nested `.wtm/` segment):

```go
func TestMyFeature(t *testing.T) {
  stateDir := t.TempDir()
  // write test fixtures
  os.WriteFile(filepath.Join(stateDir, domain.RunFileName), []byte(content), 0o644)
  // call function under test — config layer takes a state dir
  result, err := config.LoadRun(stateDir)
  // assert
}
```

### Integration tests — command-level via Cobra

Use `gittest.InitRepo` for the git repo, plus both `WTM_PROJECT_DIR` and `WTM_STATE_DIR` so the command short-circuits any git lookup:

```go
func TestRunExportEmpty(t *testing.T) {
  dir := gittest.InitRepo(t)
  stateDir := filepath.Join(dir, ".git", "wtm")
  t.Setenv("WTM_PROJECT_DIR", dir)
  t.Setenv("WTM_STATE_DIR", stateDir)

  // Write minimal config.toml into the state dir
  config.WriteProject(config.WriteProjectParams{StateDir: stateDir, Answers: ...})

  // Execute command via Cobra
  cmd := NewRunCmd()
  var out bytes.Buffer
  cmd.SetOut(&out)
  cmd.SetArgs([]string{domain.CmdExport})
  err := cmd.Execute()

  // Assert on stdout JSON
  var got domain.RunConfig
  json.Unmarshal(out.Bytes(), &got)
}
```

### Shared test helpers

`internal/testutil/gittest/gittest.go`:
```go
gittest.InitRepo(t)           // temp git repo with initial commit
gittest.CreateBranch(t, dir, name) // create a local branch
```

---

## Project Checklist

Before calling `build-validator`, verify manually:
- [ ] All new types are in `internal/domain/`
- [ ] All flag and command names use constants from `constants.go`
- [ ] No function has more than 1 unstructured parameter
- [ ] All external input is validated in `internal/config/` before reaching service
- [ ] No nested conditionals — early returns throughout
- [ ] No type assertions without comma-ok
- [ ] No business logic in `commands/` or `tui/`
- [ ] No `lipgloss` imports outside `internal/styles/`
- [ ] No `cobra` or `bubbletea` imports inside `internal/service/`
- [ ] Pure functions (no I/O) live in internal/rules/, not in service/
- [ ] All async service calls in TUI wrapped as `tea.Cmd`
- [ ] `addOutputFlag(cmd)` used instead of manual flag registration
- [ ] `styles.Indent` used instead of literal `"  "` for padding

Then run **`build-validator`**.
