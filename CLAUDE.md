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

## 10. Validate before commit

Run the **`build-validator`** subagent at the end of every development session
or before any commit. It checks compilation, vet, static analysis, and tests.

```
→ Invoke build-validator before marking any task done.
```

