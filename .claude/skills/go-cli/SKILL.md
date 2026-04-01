---
name: go-cli
description: Expert guidance for developing CLI applications in Go following strict architectural principles, using Cobra, Viper, Bubbletea, and Lipgloss. Use this skill whenever the user is building, extending, or refactoring a Go CLI — including adding commands, flags, subcommands, TUI components, config parsing, output formatting, error handling, or structuring packages. Trigger on any mention of "go CLI", "cobra", "viper", "bubbletea", "bubble tea", "lipgloss", "TUI", "go command", "go flag", "go binary", "charm", or any request to scaffold or improve a Go terminal application. Also trigger when the user references the project's CLAUDE.md principles in the context of Go code.
---

# Go CLI Development Skill

## Stack

- **Cobra** — command routing and flag parsing
- **Viper** — configuration loading (files, env vars, flags)
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

func resolveResult(condition bool) string {
  if condition {
    return "a"
  }
  return "b"
}
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
(`constants.go`, `errors.go`, `types.go`). Never duplicate a type across packages.

### 4. Validate at the boundary

Validate all external input (flags, env vars, config files) in `internal/config/`
before it reaches the service layer. Use `go-playground/validator` struct tags or
explicit guard clauses.

```go
type Config struct {
  Host   string `validate:"required,hostname"`
  Port   int    `validate:"required,min=1,max=65535"`
  Format string `validate:"required,oneof=json text table"`
}
```

### 5. Centralized constants — no magic strings or numbers

All flag names, env var keys, exit codes, format identifiers, and Viper keys
must be declared as typed constants in `internal/domain/constants.go`.

```go
const (
  ExitCodeError    = 1
  FlagOutputFormat = "output-format"
  ViperKeyHost     = "server.host"
  FormatJSON       = "json"
  FormatTable      = "table"
)
```

### 6. Early returns — flatten all conditionals

Never nest `if` blocks. The happy path is always the last statement.

### 7. No unsafe type assertions

Always use the comma-ok idiom. Prefer typed interfaces and concrete structs
over `any`/`interface{}`. Type at the source, not downstream.

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
  root.go                     ← cobra root, global flags, version
  <command>.go                ← one file per top-level command

internal/
  commands/                   ← flag wiring only → delegates to service
  domain/                     ← types, errors, constants (single source of truth)
  config/                     ← load & validate .wtm.toml + ~/.config/wtm/config.toml
  service/                    ← all business logic, organized by domain:
    worktree/                 ←   git worktree operations (create, list, remove)
    env/                      ←   .env file provisioning strategies
    hooks/                    ←   on_create / on_focus / on_blur hook execution
    shell/                    ←   shell integration generation (zsh, bash, fish)
    state/                    ←   global state (~/.config/wtm/state.json)
    integration/              ←   third-party adapters (VS Code, Cursor)
  styles/                     ← all Lipgloss styles (see section below)
  output/                     ← plain-text / JSON / table renderers
  tui/                        ← Bubbletea models (see section below)
  infra/                      ← I/O, git exec, filesystem wrappers
```

**Hard rules:**
- `commands/` has zero business logic
- `service/` has zero imports of `cobra`, `bubbletea`, `lipgloss`
- `output/` and `tui/` have zero decision logic — only rendering
- `styles/` is the only package allowed to instantiate `lipgloss.Style`

### 10. Validate before commit — run `build-validator`

Before marking any task done, invoke the `build-validator` subagent.

---

## Cobra — Command Patterns

### Standard (plain-text) command

```go
// internal/commands/deploy.go
func NewDeployCommand(svc service.DeployService) *cobra.Command {
  var params DeployFlagParams

  cmd := &cobra.Command{
    Use:   "deploy",
    Short: "Deploy a resource",
    RunE: func(cmd *cobra.Command, args []string) error {
      return runDeploy(cmd.Context(), svc, params)
    },
  }

  cmd.Flags().StringVar(&params.Environment, FlagEnvironment, "", "Target environment")
  cmd.Flags().BoolVar(&params.DryRun, FlagDryRun, false, "Simulate without applying")
  _ = cmd.MarkFlagRequired(FlagEnvironment)

  return cmd
}

func runDeploy(ctx context.Context, svc service.DeployService, params DeployFlagParams) error {
  input, err := mapDeployInput(params)
  if err != nil {
    return err
  }
  result, err := svc.Deploy(ctx, input)
  if err != nil {
    return err
  }
  return output.PrintDeploy(result)
}
```

### TUI command — Cobra launches a Bubbletea program

The command runner is the only place where `tea.NewProgram` is called.
All TUI state lives in `internal/tui/`, never in `commands/`.

```go
// internal/commands/browse.go
func NewBrowseCommand(svc service.ItemService) *cobra.Command {
  return &cobra.Command{
    Use:   "browse",
    Short: "Browse items interactively",
    RunE: func(cmd *cobra.Command, args []string) error {
      return runBrowseTUI(cmd.Context(), svc)
    },
  }
}

func runBrowseTUI(ctx context.Context, svc service.ItemService) error {
  items, err := svc.List(ctx)
  if err != nil {
    return fmt.Errorf("load items: %w", err)
  }

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

## Viper — Configuration

Bind flags to Viper keys using constants. Always validate after loading.

```go
// internal/config/config.go
func Load(params LoadParams) (Config, error) {
  viper.SetConfigFile(params.ConfigPath)
  viper.AutomaticEnv()

  if err := viper.BindPFlag(ViperKeyHost, params.Flags.Lookup(FlagHost)); err != nil {
    return Config{}, fmt.Errorf("bind flag: %w", err)
  }

  if err := viper.ReadInConfig(); err != nil {
    return Config{}, fmt.Errorf("read config: %w", err)
  }

  cfg := Config{
    Host: viper.GetString(ViperKeyHost),
    Port: viper.GetInt(ViperKeyPort),
  }

  if err := validate.Struct(cfg); err != nil {
    return Config{}, fmt.Errorf("invalid config: %w", err)
  }

  return cfg, nil
}
```

---

## Bubbletea — TUI Architecture

### File layout — one directory per screen

```
internal/tui/
  browse/
    model.go     ← Model struct + Init/Update/View
    keys.go      ← keybindings (key.Binding)
    msgs.go      ← custom tea.Msg types for this screen
```

### Model structure

```go
// internal/tui/browse/model.go

// BrowseModel holds all state for the browse screen.
// It must not hold business logic — only UI state.
type BrowseModel struct {
  items    []domain.Item
  cursor   int
  selected *domain.Item
  err      error
}

func NewBrowseModel(items []domain.Item) BrowseModel {
  return BrowseModel{items: items}
}

func (m BrowseModel) Init() tea.Cmd {
  return nil
}

func (m BrowseModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
  switch msg := msg.(type) {
  case tea.KeyMsg:
    return m.handleKey(msg)
  case errMsg:
    return m.withError(msg.err), nil
  }
  return m, nil
}

func (m BrowseModel) View() string {
  if m.err != nil {
    return styles.Error.Render(m.err.Error())
  }
  return m.renderList()
}

// Selected returns the item chosen by the user, if any.
func (m BrowseModel) Selected() *domain.Item {
  return m.selected
}
```

### Custom messages — typed, never `interface{}`

```go
// internal/tui/browse/msgs.go
type errMsg struct{ err error }
type itemsLoadedMsg struct{ items []domain.Item }
```

### Keybindings — centralized per screen

```go
// internal/tui/browse/keys.go
type keyMap struct {
  Up     key.Binding
  Down   key.Binding
  Select key.Binding
  Quit   key.Binding
}

var defaultKeys = keyMap{
  Up:     key.NewBinding(key.WithKeys("up", "k"), key.WithHelp("↑/k", "up")),
  Down:   key.NewBinding(key.WithKeys("down", "j"), key.WithHelp("↓/j", "down")),
  Select: key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "select")),
  Quit:   key.NewBinding(key.WithKeys("q", "esc"), key.WithHelp("q", "quit")),
}
```

### Async calls — always via tea.Cmd

Never call service methods directly inside `Update` or `View`.
Wrap them as `tea.Cmd` so Bubbletea manages the goroutine.

```go
func loadItems(svc service.ItemService) tea.Cmd {
  return func() tea.Msg {
    items, err := svc.List(context.Background())
    if err != nil {
      return errMsg{err: err}
    }
    return itemsLoadedMsg{items: items}
  }
}
```

### Rules
- `Update` is a pure function — no side effects, only return `(tea.Model, tea.Cmd)`
- Never import `lipgloss` in `model.go` — delegate all styling to `styles/`
- Never import `cobra` or `service` inside a `tui/` model

---

## Lipgloss — Centralized Styles

All styles live in `internal/styles/`, split by concern (Charm community convention).
Never declare a `lipgloss.NewStyle()` outside this package.

```
internal/styles/
  colors.go      ← adaptive color palette
  text.go        ← typography (title, subtitle, muted, error, success)
  layout.go      ← box, padding, border, width helpers
  components.go  ← composed styles (list item active/inactive, badge, status)
```

### colors.go

```go
package styles

import "github.com/charmbracelet/lipgloss"

var (
  ColorPrimary = lipgloss.AdaptiveColor{Light: "#0F62FE", Dark: "#78A9FF"}
  ColorMuted   = lipgloss.AdaptiveColor{Light: "#6F6F6F", Dark: "#8D8D8D"}
  ColorError   = lipgloss.AdaptiveColor{Light: "#DA1E28", Dark: "#FF8389"}
  ColorSuccess = lipgloss.AdaptiveColor{Light: "#198038", Dark: "#42BE65"}
)
```

### text.go

```go
package styles

import "github.com/charmbracelet/lipgloss"

var (
  Title   = lipgloss.NewStyle().Bold(true).Foreground(ColorPrimary)
  Muted   = lipgloss.NewStyle().Foreground(ColorMuted)
  Error   = lipgloss.NewStyle().Foreground(ColorError)
  Success = lipgloss.NewStyle().Foreground(ColorSuccess)
)
```

### components.go

```go
package styles

import "github.com/charmbracelet/lipgloss"

var (
  ListItemActive = lipgloss.NewStyle().
      Bold(true).
      Foreground(ColorPrimary).
      PaddingLeft(1)

  ListItemInactive = lipgloss.NewStyle().
      Foreground(ColorMuted).
      PaddingLeft(1)
)
```

### Usage rule

```go
// ✅ In any View() function
return styles.Title.Render("My Title")

// ❌ Never inline lipgloss in model or output files
return lipgloss.NewStyle().Bold(true).Render("My Title")
```

---

## Error Handling

Always wrap errors with context. Use sentinel errors for known failure modes.

```go
var (
  ErrNotFound     = errors.New("resource not found")
  ErrUnauthorized = errors.New("unauthorized")
)

if err != nil {
  return fmt.Errorf("deploy %s: %w", params.Environment, err)
}
```

---

## Project Checklist

Before calling `build-validator`, verify manually:
- [ ] All new types are in `internal/domain/`
- [ ] All new flag / viper / env names use constants from `constants.go`
- [ ] No function has more than 1 unstructured parameter
- [ ] All external input is validated in `internal/config/` before reaching service
- [ ] No nested conditionals — early returns throughout
- [ ] No type assertions without comma-ok
- [ ] No business logic in `commands/` or `tui/`
- [ ] No `lipgloss` imports outside `internal/styles/`
- [ ] No `cobra` or `bubbletea` imports inside `internal/service/`
- [ ] All async service calls in TUI wrapped as `tea.Cmd`

Then run **`build-validator`**.