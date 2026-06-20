# wtm — Worktree Manager

Orchestrate git worktrees, AI agents, and team dev workflows from the terminal.

`wtm` manages the lifecycle of git worktrees: creation, environment provisioning, hook execution, navigation, and cleanup. It replaces manual `git worktree` commands with a streamlined workflow designed for teams working on multiple branches simultaneously.

## Dependencies

| Tool | Required | Purpose |
|---|---|---|
| `git` | ✅ Required | Worktree management |
| `gh` | ⭐ Recommended | PR listing, creation, and checkout |

`gh` is not required to use `wtm` — worktree creation, navigation, hooks, and services work without it. Install and authenticate it to unlock all GitHub features: [cli.github.com](https://cli.github.com).

---

## Installation

### Homebrew (macOS / Linux)

```bash
brew install LucasPcq/tap/wtm
```

### Download binary

Download the latest release from [GitHub Releases](https://github.com/LucasPcq/worktree-manager-cli/releases), extract it, and move the binary to your PATH:

```bash
# macOS (Apple Silicon)
tar -xzf wtm_*_darwin_arm64.tar.gz
sudo mv wtm /usr/local/bin/

# macOS (Intel)
tar -xzf wtm_*_darwin_amd64.tar.gz
sudo mv wtm /usr/local/bin/

# Linux
tar -xzf wtm_*_linux_amd64.tar.gz
sudo mv wtm /usr/local/bin/
```

### Go install

```bash
go install github.com/LucasPcq/wtm@latest
```

## Quick Start

```bash
# Set up shell integration (required for navigation)
echo 'eval "$(wtm shell-init)"' >> ~/.zshrc
source ~/.zshrc

# Initialize wtm in your project
cd your-repo
wtm init

# Use individual commands:
wtm wt list                        # list all worktrees
wtm wt create feature/my-feature   # create a worktree
wtm wt switch feature/my-feature   # navigate + start services
wtm wt extract --to refactor       # move uncommitted changes to another worktree
wtm wt clean feature/my-feature    # clean up when done
```

## Teach your LLM to use wtm

If you work with Claude Code or Cursor, run:

```bash
wtm agents install
```

Detects `.claude/` and `.cursor/` (project and home) and installs a `using-wtm` skill so your agent knows every command, flag, and JSON payload — without having to explain them each session. See [Machine-readable output](#machine-readable-output---output-json) below for the underlying `--output json` contract.

## Commands

### `wtm init`

Interactive wizard that generates your configuration files.

```bash
wtm init
```

On first run, creates two files:
- **Global config** (`~/.config/wtm/config.toml`) — shell type, default AI agent
- **Project config** (`<git-common-dir>/wtm/config.toml`) — worktree settings, env strategy, hooks

Both files live outside the working tree, so nothing is ever committed to your repo. The project config is scoped to your local clone (it lives inside `.git/`), invisible to teammates and to `git status`.

The wizard auto-detects:
- Default branch (via `git symbolic-ref`)
- `.env` and `.env.example` files
- Package manager (pnpm, npm, yarn, go, pip)
- Docker Compose files
- Monorepo packages (via `pnpm-workspace.yaml`)

If a project config already exists, the command exits — use `wtm config edit` to modify it.

---

### `wtm config` — Inspect or edit the project config

#### `wtm config show`

Print the resolved `config.toml` and its on-disk path.

```bash
wtm config show
```

#### `wtm config edit`

Open `<git-common-dir>/wtm/config.toml` in `$EDITOR` (falls back to `vi`). After save, the file is re-validated and any error is surfaced.

```bash
wtm config edit
```

`#:schema` directives at the top of the file resolve to the bundled JSON schemas, so editors like VS Code (with [Even Better TOML](https://marketplace.visualstudio.com/items?itemName=tamasfe.even-better-toml)) auto-complete every field.

---

### `wtm shell-init`

Output a shell wrapper function. Eval it in your rc file.

```bash
# Detect shell automatically
wtm shell-init

# Add to your shell config
echo 'eval "$(wtm shell-init)"' >> ~/.zshrc   # zsh
echo 'eval "$(wtm shell-init)"' >> ~/.bashrc  # bash
echo 'wtm shell-init | source'  >> ~/.config/fish/config.fish  # fish
```

---

### `wtm wt` — Worktree management

#### `wtm wt create [branch]`

Create a new git worktree with environment provisioning and hooks.

```bash
# Fully interactive — prompts for branch name, source branch, and env strategy
wtm wt create

# Specify branch name, pick source branch interactively
wtm wt create feature/auth

# Direct — specify everything, no interaction
wtm wt create feature/auth --from main --env-from parent
```

**What happens:**
1. Creates a git worktree at `<base_path>/<branch-name>` (slashes become dashes)
2. Copies `.env` files according to the configured strategy
3. Records `meta.json` (source branch, timestamp, strategy used) under `<git-common-dir>/wtm/worktrees/<branch>/`
4. Creates an empty `context.md` next to it for your notes
5. Runs `on_create` hooks

**Flags:**
| Flag | Description |
|---|---|
| `--from <branch>` | Source branch (skips interactive picker) |
| `--env-from <strategy>` | Override env strategy: `example`, `main`, or `parent` |

#### `wtm wt list`

List all worktrees with their git status.

```bash
wtm wt list
```

Output:
```
  main              (parent)  ● active  clean
  feature-auth                           dirty   3 commits ahead
  feature-payment                        clean   1 commit ahead
```

In an interactive terminal, shows a picker with actions: go, start profile, stop profile, view logs, clean.

#### `wtm wt go [branch]`

Navigate to a worktree directory. Requires shell integration.

```bash
wtm wt go feature/auth     # navigate to a specific worktree
wtm wt go                  # interactive picker
wtm wt go auth             # substring match
```

Without shell integration, `wtm wt go` cannot change your working directory. The shell wrapper intercepts `wtm wt go` and performs the `cd` in your current shell.

#### `wtm wt switch [branch]`

Navigate to a worktree **and** start its services in one command. Combines `wt go` + `run up`.

```bash
wtm wt switch feature/auth               # go + start default profile
wtm wt switch feature/auth --exclusive    # go + stop others + start
wtm wt switch feature/auth --parallel     # go + start without stopping others
wtm wt switch feature/auth --profile api  # go + start specific profile
wtm wt switch                             # interactive picker + start
```

Requires shell integration (same as `wt go`).

**Flags:**
| Flag | Description |
|---|---|
| `--exclusive` | Stop services on other worktrees before starting |
| `--parallel` | Start without stopping other worktrees |
| `--profile <name>` | Service profile to start (default: default profile) |

#### `wtm wt clean [branch]`

Remove a worktree and its local branch. The remote branch is never touched.

```bash
wtm wt clean                        # interactive picker with safety checks
wtm wt clean feature/auth           # direct
wtm wt clean feature/auth --force   # skip all safety checks
```

**Safety checks:** uncommitted changes, unpushed commits, open pull request.

**Flags:**
| Flag | Description |
|---|---|
| `--force` | Bypass all safety checks and delete immediately |

#### `wtm wt extract`

Move a subset of the **current worktree's uncommitted changes** to another worktree — to split an oversized PR or peel off unrelated work for easier review and parallel development. Without flags it runs an interactive wizard: pick files → target → move/copy.

```bash
wtm wt extract                                              # interactive: pick files, target, mode
wtm wt extract --files src/api.go,src/db.go --to refactor  # move files to an existing worktree
wtm wt extract --files src/api.go --to spike --from main   # create 'spike' from main, move into it
wtm wt extract --files notes.md --to docs --keep           # copy instead of move (keep in source)
```

**Move vs copy:** files are **moved** by default (removed from the source once they land); `--keep` copies them instead.

**Conflicts** (a selected file was also changed in the target): by default the extraction **aborts** and nothing changes (exit code `15`). With `--on-conflict resolve`, the changes are applied to the target with git **conflict markers** — like a rebase — so you can resolve them in your editor; the source is always kept intact in that case.

**Safety:** the source is cleaned only when the whole extraction applies cleanly. If anything conflicts, the source is left fully intact and recoverable — there is never a half-moved state.

**Flags:**
| Flag | Description |
|---|---|
| `--files <a,b>` | Comma-separated files to extract (skips the file picker) |
| `--to <branch>` | Target worktree branch; created if it doesn't exist |
| `--from <base>` | Parent branch when creating the target worktree |
| `--keep` | Copy instead of move (keep the changes in the source) |
| `--on-conflict <abort\|resolve>` | On conflict: abort (default) or write conflict markers in the target |

---

### `wtm run` — Dev jobs (services + tasks)

#### `wtm run list`

List the jobs and profiles declared in `run.toml`. In a terminal, shows an interactive picker with inline actions (start/stop/logs).

```bash
wtm run list                # picker if TTY, table if piped
wtm run list --output json  # machine-readable
```

The picker lets you pick either a profile (actions: `up`, `down`) or a job (actions: `start`, `stop`, `logs`).

#### `wtm run ps`

List jobs currently managed by the background daemon (running or crashed), with their kind. In a terminal, shows an interactive picker with stop/logs/restart actions.

```bash
wtm run ps                  # picker if TTY, table if piped
wtm run ps --output json    # machine-readable
```

#### `wtm run up [profile]`

Execute a profile defined in `run.toml`. Jobs run in declared order — tasks block and **stream their output live** (a non-zero exit aborts the rest of the profile), services launch in the background.

Once the profile's services are up, `run up` **tails their logs automatically** so you don't need a separate `run logs` — start and watch in one command, like `docker compose up`:

- a single foreground service is attached directly (full PTY), so its own TUI — turbo, vite, next — renders natively and stays interactive;
- two or more are multiplexed as color-prefixed log lines;
- detached launchers (`docker compose up -d`) are skipped (nothing to tail).

Press `Ctrl+C` to **detach** — the services keep running in the background.

```bash
wtm run up                  # start default profile, then tail its services
wtm run up full             # start a specific profile
wtm run up -d               # start and return immediately (no tail)
wtm run up --exclusive      # stop jobs on other worktrees first
wtm run up --parallel       # start without stopping others
```

`run up` does not tail when output is piped, with `--output json`, or with `-d/--detach` — it starts the jobs and returns (the original behavior, ideal for scripts and LLM agents).

**On failure:** any failing job — a task exiting non-zero **or** a service that fails to start — aborts the rest of the profile the same way. Jobs already started are **left running** (docker/DB stay warm for the fix-and-retry loop — wtm never tears down what it didn't fail to start), and `run up` prints where it stopped, what's still running, and what never started:

```
✗ task migrate failed (exit 1)
ERROR: relation "users" does not exist

  ! Profile aborted at step 2/3 (migrate).
  Left running:  docker
  Not started:   dev

  › fix and re-run `wtm run up` · `wtm run down` to stop everything
```

The failing job's output is shown so you see *why* it failed — streamed live in the terminal, and embedded in the error entry under `--output json` so scripts and LLM agents get the reason too. Re-running `wtm run up` while services are still up is safe: already-running services are reported as such and skipped, not treated as errors.

If services are already running on another worktree, `run up` prompts you to stop them or run in parallel.

**Flags:**
| Flag | Description |
|---|---|
| `-d, --detach` | Start jobs and return immediately instead of tailing their logs |
| `--exclusive` | Stop jobs on other worktrees before starting |
| `--parallel` | Start without stopping other worktrees |

#### `wtm run down [profile]`

Stop jobs running in the **current worktree**. Jobs in other worktrees are never touched unless you pass `--all`.

```bash
wtm run down            # stop jobs in this worktree
wtm run down full       # stop a specific profile (this worktree)
wtm run down --all      # stop every running job across all worktrees
```

`run ps` also offers a "Stop all running jobs" entry at the bottom of its picker, which shells out to `run down --all`.

#### `wtm run start <job>`

Start a single job by name. Tasks run inline and block until they exit; services launch in the background.

```bash
wtm run start dev       # start the "dev" service
wtm run start migrate   # run the "migrate" task to completion
```

#### `wtm run stop <job>`

Stop a single running job by name.

```bash
wtm run stop dev
```

#### `wtm run logs [job]`

Attach to a job's output. Without arguments, multiplexes all running jobs with colored prefixes. With a job name, attaches to that single job's PTY.

```bash
wtm run logs            # stream all running jobs (multiplexed)
wtm run logs dev        # attach to a single job
```

Press `Ctrl+C` to detach — services keep running in the background.

#### `wtm run export`

Emit `run.toml` as JSON on stdout — useful for sharing a service layout between projects or teammates.

```bash
wtm run export                     # full config as JSON
wtm run export --profile dev       # only the "dev" profile and its jobs
wtm run export > layout.json       # save to file
```

#### `wtm run import [file|-]`

Ingest a JSON run config. Pass a file path, `-`, or omit the argument to read from stdin.

```bash
wtm run import layout.json                    # merge into run.toml
wtm run import layout.json --replace --force  # overwrite entirely
wtm run export | wtm run import -             # roundtrip (no-op when names match)
```

By default, new jobs and profiles are appended; duplicate names are skipped with a warning.

#### `wtm run job add|rm|edit` — CRUD on jobs

Manage individual jobs in `run.toml` without opening the file by hand.

```bash
# Add a service (flag-driven, LLM-friendly)
wtm run job add api --cmd "go run ./cmd/api" --kind service --stop "pkill api"

# Add a task
wtm run job add migrate --cmd "make migrate" --kind task

# Add interactively (wizard pops up when name or --cmd is missing)
wtm run job add

# Remove (no arg → interactive picker)
wtm run job rm                   # picker over all jobs
wtm run job rm migrate           # by name; errors if any profile references the job
wtm run job rm migrate --force   # also strips the reference from those profiles

# Edit (no arg → interactive picker → pre-filled wizard)
wtm run job edit                 # picker, then wizard
wtm run job edit api             # straight to the pre-filled wizard

# List (TTY = picker → Edit/Remove menu; non-TTY or --output json = listing)
wtm run job list
wtm run job list --output json
```

#### `wtm run profile add|rm|edit` — CRUD on profiles

```bash
# Add a profile (flag-driven)
wtm run profile add dev --jobs api,migrate --default

# Add interactively
wtm run profile add

# Remove (no arg → picker; jobs are left untouched)
wtm run profile rm
wtm run profile rm dev

# Edit (no arg → picker → pre-filled wizard)
wtm run profile edit
wtm run profile edit dev

# List (same pattern as run job list)
wtm run profile list
wtm run profile list --output json
```

When you mark a profile as `--default` (or via the wizard) while another profile is already the default, the previous default is automatically unset — no need to clear it first.

All `add` / `rm` commands support `--output text|json`. `edit` is intrinsically interactive.

---

### `wtm pr` — Pull requests

Interact with GitHub pull requests from the CLI. Requires the [`gh` CLI](https://cli.github.com) installed and authenticated (`gh auth login`).

#### `wtm pr list`

List open pull requests. Interactive picker with actions (checkout, open in browser, view details).

```bash
wtm pr list           # all open PRs
wtm pr list --mine    # PRs you authored
wtm pr list --review  # PRs where you are a requested reviewer
```

#### `wtm pr create`

Create a PR for the current branch via an interactive wizard. Pushes the branch first if needed.

```bash
wtm pr create                              # full interactive
wtm pr create --title "..." --base main    # skip wizard fields
wtm pr create --draft                      # draft PR
```

#### `wtm pr checkout [number]`

Create a worktree from an existing pull request and run `on_create` hooks. Perfect for reviewing a teammate's PR with the full environment set up.

```bash
wtm pr checkout 42                 # checkout PR #42
wtm pr checkout                    # interactive picker of open PRs
wtm pr checkout 42 --env-from main # override env strategy
```

Behavior:
- Runs `git fetch origin <pr-branch>`, then creates a worktree on that branch
- Applies the configured env strategy (interactive wizard if `--env-from` not provided)
- Runs `on_create` hooks exactly like `wtm wt create`
- Refuses if a local branch with the same name already exists — run `wtm wt clean <branch>` first

**Forks (by design)** — wtm doesn't check out fork PRs. A fork's branch lives on the contributor's repo (not `origin`) and a fork worktree couldn't push back, which breaks wtm's "develop here" model. To review a fork PR, use `gh pr checkout <number>`.

---

## Machine-readable output (`--output json`)

Every data-returning command supports `--output json` for scripting and LLM agents (Claude Code, Cursor, …). JSON is pretty-printed on stdout with `snake_case` fields; human messages stay on stderr; exit codes are unchanged.

**The payload *is* the schema** — the shape mirrors the command's Go domain type and stays stable. To discover the exact fields of any command, just run it with the flag:

```bash
wtm wt list --output json
wtm pr list --output json
wtm run list --output json
```

Supported commands:

- `wt list`, `wt create`, `wt clean` (requires `--force`), `wt extract`
- `pr list`, `pr create`, `pr checkout`
- `run list`, `run ps`, `run up`, `run down`, `run start`, `run stop`

Example — pipe into `jq`:

```bash
wtm wt list --output json | jq '.[] | select(.is_dirty).branch'
wtm pr list --output json | jq '.[].number'
wtm run ps --output json | jq '.[] | select(.status=="running").name'
```

See also [Teach your LLM to use wtm](#teach-your-llm-to-use-wtm) above — `wtm agents install` drops a `using-wtm` skill into `.claude/` or `.cursor/` so agents can drive every command without being told.

---

## Configuration

All wtm files live under `<git-common-dir>/wtm/` (i.e. `.git/wtm/` for a normal clone — `git rev-parse --git-common-dir` for the exact path). Git never commits anything inside `.git/`, so wtm is invisible to teammates and to `git status`. Worktree usage is personal: each developer can adopt or skip wtm without affecting the rest of the team.

```
<git-common-dir>/wtm/
├── config.toml                       # project-level settings (this section)
├── run.toml                          # jobs + profiles (next section)
├── schemas/                          # JSON schemas for editor auto-complete
└── worktrees/<encoded-branch>/
    ├── meta.json                     # per-worktree metadata
    └── context.md                    # per-worktree notes
```

You can edit any file by hand if you want — they're plain TOML, validated at load time. The discoverable path is `wtm config show` / `wtm config edit` and the `wtm run import` flow.

### Project config — `<git-common-dir>/wtm/config.toml`

Generated by `wtm init`. Per-clone — never committed.

```toml
[worktrees]
# Directory where worktrees are created (relative to repo root)
base_path = "../.trees"

# Branch used as the default base for new worktrees
base_branch = "main"

[env]
# Strategy for provisioning .env files in new worktrees:
#   example — copy .env.example and rename to .env
#   main    — copy .env from the main worktree
#   parent  — copy .env from the source worktree (--from)
strategy = "example"

# Files to copy into each new worktree
copy_files = [
  ".env",
  "apps/api/.env",
]

[hooks]
# Commands run after creating a new worktree
on_create = [
  "pnpm install",
  { cmd = "pnpm install", cwd = "apps/api" },
]

[github]
auto_draft = false
base_branch = "main"

[agents]
# Default AI agent: claude-code | cursor | none
default = "claude-code"

[integrations]
# VS Code / Cursor Project Manager integration (coming soon)
vscode_project_manager = false
cursor_project_manager = false
```

### Hook format

Hooks accept two formats in the same array:

```toml
on_create = [
  # Simple string — runs from the worktree root
  "pnpm install",

  # Object with cwd — runs from a subdirectory
  { cmd = "pnpm install", cwd = "apps/api" },

  # Object with continue_on_error — doesn't stop if this hook fails
  { cmd = "pnpm install", cwd = "apps/web", continue_on_error = true },
]
```

### Template variables

Hooks support variable interpolation:

| Variable | Description |
|---|---|
| `{{worktree}}` | Absolute path of the new/target worktree |
| `{{branch}}` | Branch name (e.g. `feature/auth`) |
| `{{root}}` | Absolute path of the main worktree (repo root) |
| `{{from_branch}}` | Source branch used for creation (on_create only) |

Example:
```toml
on_create = [
  "echo 'Created {{branch}} at {{worktree}} from {{from_branch}}'",
]
```

### Env strategies

| Strategy | Behavior |
|---|---|
| `example` | Copies `file.example` from the main worktree and renames to `file` in the new worktree. Warns if `.example` not found. |
| `main` | Copies the actual file from the main worktree. Useful when `.env.example` files are incomplete. |
| `parent` | Copies from the source worktree (`--from`). Falls back to `main` if the file isn't found in the parent. |

---

### Run config — `<git-common-dir>/wtm/run.toml`

Optional file for managing dev jobs — long-running services (dev servers, docker, workers) and one-shot tasks (migrations, seeds, formatters). Per-clone — never committed. Use `wtm run export | wtm run import -` to share layouts between machines or teammates.

```toml
[[job]]
name = "docker"
kind = "service"
cmd  = "docker compose up -d"
stop = "docker compose down"

[[job]]
name = "migrate"
kind = "task"
cmd  = "pnpm migrate"

[[job]]
name = "seed"
kind = "task"
cmd  = "pnpm seed"

[[job]]
name = "dev"
kind = "service"
cmd  = "pnpm dev"
cwd  = "apps/web"

[[profile]]
name = "full"
jobs = ["docker", "migrate", "seed", "dev"]
default = true

[[profile]]
name = "back"
jobs = ["docker", "migrate"]
```

**Jobs** declare commands to run. Two kinds:
- `kind = "service"` — long-running. With a `stop` command set, it's treated as detached (e.g. `docker compose up -d` whose containers persist after the launcher exits). Without a `stop`, it's tracked by PID and killed via SIGTERM.
- `kind = "task"` — one-shot script. Blocks the profile, streams output live, removed from `run ps` after exit. A non-zero exit aborts the rest of the profile.

**Profiles** group jobs into named, ordered sets you start together. One profile can be marked `default = true`.

- `wtm run up [profile]` / `wtm run down [profile]` — start or stop a whole profile
- `wtm run start <job>` / `wtm run stop <job>` — start or stop a single job
- `wtm run logs` — stream all running jobs (multiplexed), or `wtm run logs <job>` for one

Services run in a background daemon with PTY support; `run up` tails them automatically (press `Ctrl+C` to detach without killing them). Tasks run inline and stream their output live back over the daemon's connection.

`run.toml` is shared across every worktree of the same clone (loaded from the main repo's git common dir). Jobs are scoped per worktree at runtime: starting `dev` from worktree A runs it with `cwd = A`, starting it from worktree B runs a separate process. The daemon tracks them independently and `wtm run down` only stops jobs from the current worktree unless you pass `--all`.

---

### Global config — `~/.config/wtm/config.toml`

Created by `wtm init` on first run. Not committed — personal to each developer.

```toml
shell = "zsh"          # zsh | bash | fish
agent = "claude-code"  # claude-code | cursor | none
```

The project `config.toml` can override the agent setting. Shell is always global.

---

## IDE autocomplete + validation

Every TOML file `wtm init` writes starts with a `#:schema ./schemas/...json` directive. Pair it with the [Even Better TOML](https://marketplace.visualstudio.com/items?itemName=tamasfe.even-better-toml) extension (Taplo, also bundled in JetBrains' TOML plugin) to get:

- **Autocomplete** on every field name and enum value (`kind = "service" | "task"`, `env.strategy = "example" | "main" | "parent"`, etc.)
- **Hover docs** describing each option
- **Real-time validation** flagging unknown keys, missing required fields, and bad enum values before you ever run wtm

The schemas are bundled with the binary. `wtm init` writes them to `<git-common-dir>/wtm/schemas/` so the directive resolves locally — no internet required. Re-extract them after upgrading wtm with:

```bash
wtm schema dump            # writes <git-common-dir>/wtm/schemas/{run,project}.schema.json
wtm schema dump --global   # writes ~/.config/wtm/schemas/global.schema.json
```

Even without the editor extension, wtm itself rejects unknown keys at load time — typos like `[[profiles]]` instead of `[[profile]]` surface as `unknown keys in <path>/run.toml: profiles` rather than being silently ignored.

---

## Worktree metadata

Each worktree created by `wtm wt create` records two files under `<git-common-dir>/wtm/worktrees/<encoded-branch>/`:

- **`meta.json`** — source branch, creation timestamp, env strategy used
- **`context.md`** — empty file for your notes (branch context, PR links, etc.)

Branch names are URL-path-escaped on disk (e.g. `feat/x` → `feat%2Fx/`) so slashes don't create nested directories. Both files live alongside `git`'s own per-worktree state, completely outside the working tree.

---

## License

MIT
