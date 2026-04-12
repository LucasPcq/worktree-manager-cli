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
wtm wt go feature/my-feature       # navigate to it
wtm wt focus feature/my-feature    # start the environment
wtm wt clean feature/my-feature    # clean up when done
```

## Commands

### `wtm init`

Interactive wizard that generates your configuration files.

```bash
wtm init
```

On first run, creates two files:
- **Global config** (`~/.config/wtm/config.toml`) — shell type, default AI agent
- **Project config** (`.wtm/config.toml`) — worktree settings, env strategy, hooks

The wizard auto-detects:
- Default branch (via `git symbolic-ref`)
- `.env` and `.env.example` files
- Package manager (pnpm, npm, yarn, go, pip)
- Docker Compose files
- Monorepo packages (via `pnpm-workspace.yaml`)

If `.wtm/config.toml` already exists, the command exits — delete it or edit manually to reconfigure.

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
3. Creates `.wtm/meta.json` (source branch, timestamp, strategy used)
4. Creates `.wtm/context.md` (empty, for your notes)
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

In an interactive terminal, shows a picker with actions: go, focus, start profile, stop profile, view logs, clean.

#### `wtm wt go [branch]`

Navigate to a worktree directory. Requires shell integration.

```bash
wtm wt go feature/auth     # navigate to a specific worktree
wtm wt go                  # interactive picker
wtm wt go auth             # substring match
```

Without shell integration, `wtm wt go` cannot change your working directory. The shell wrapper intercepts `wtm wt go` and performs the `cd` in your current shell.

#### `wtm wt focus [branch]`

Switch the active worktree and run lifecycle hooks.

```bash
wtm wt focus feature/auth   # runs on_blur on previous, on_focus on target
wtm wt focus                # interactive picker
wtm wt focus --off          # stop everything — runs on_blur and clears state
```

`focus` and `go` are independent and composable:
- `wtm wt go` changes your directory
- `wtm wt focus` manages your environment (starts/stops services via hooks)
- Use both: `wtm wt focus feature/auth && wtm wt go feature/auth`

**Flags:**
| Flag | Description |
|---|---|
| `--off` | Run blur hooks on active worktree and clear state |

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

---

### `wtm svc` — Service management

#### `wtm svc up [profile]`

Start a service profile defined in `.wtm/services.toml`.

```bash
wtm svc up              # start default profile (or picker if multiple)
wtm svc up backend      # start a specific profile
```

#### `wtm svc down [profile]`

Stop a service profile.

```bash
wtm svc down            # stop all running services
wtm svc down backend    # stop a specific profile
```

#### `wtm svc start <service>`

Start a single service by name.

```bash
wtm svc start api
```

#### `wtm svc stop <service>`

Stop a single running service by name.

```bash
wtm svc stop api
```

#### `wtm svc logs [service]`

Stream service output. Without arguments, multiplexes all running services with colored prefixes. With a service name, attaches to that single service's PTY.

```bash
wtm svc logs            # stream all running services (multiplexed)
wtm svc logs api        # attach to a single service
```

Press `Ctrl+C` to detach — services keep running in the background.

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

**Limitation** — PRs from forks are not supported yet. Use `gh pr checkout` as a fallback for fork PRs.

---

## Configuration

### Project config — `.wtm/config.toml`

Generated by `wtm init`. Committed to the repo — shared by the team.

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

# Commands run when switching to this worktree (wtm focus)
on_focus = [
  "docker-compose up -d",
]

# Commands run when leaving this worktree (wtm focus --off or switching)
on_blur = [
  "docker-compose down --remove-orphans",
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

### Services config — `.wtm/services.toml`

Optional file for managing long-running services (dev servers, docker, workers). Committed to the repo — shared by the team.

```toml
[[services]]
name = "api"
cmd = "docker-compose up api db"
stop = "docker-compose stop api db"
cwd = "."

[[services]]
name = "web"
cmd = "pnpm dev"
cwd = "apps/web"

[[services]]
name = "worker"
cmd = "docker-compose up worker"
stop = "docker-compose stop worker"

[[profiles]]
name = "full"
services = ["api", "web", "worker"]
default = true

[[profiles]]
name = "back"
services = ["api", "worker"]

[[profiles]]
name = "front"
services = ["web", "api"]
```

**Services** define individual components (a dev server, a database, a worker). **Profiles** group services into named sets you start together. One profile can be marked `default = true`.

- `wtm svc up [profile]` / `wtm svc down [profile]` — start or stop a whole profile
- `wtm svc start <service>` / `wtm svc stop <service>` — start or stop a single service
- `wtm svc logs` — stream all running services (multiplexed), or `wtm svc logs <service>` for one

Services run in a background daemon with PTY support. Press `Ctrl+C` to detach without killing the service.

Each worktree has its own copy of this file (it's versioned in git), so services started in worktree A are independent from worktree B.

---

### Global config — `~/.config/wtm/config.toml`

Created by `wtm init` on first run. Not committed — personal to each developer.

```toml
shell = "zsh"          # zsh | bash | fish
agent = "claude-code"  # claude-code | cursor | none
```

The project `.wtm/config.toml` can override the agent setting. Shell is always global.

---

## State

`wtm` tracks the active worktree in `~/.config/wtm/state.json`. This file is managed automatically by `wtm wt focus` — don't edit it manually.

---

## Worktree metadata

Each worktree created by `wtm wt create` contains a `.wtm/` directory with:

- **`meta.json`** — source branch, creation timestamp, env strategy used
- **`context.md`** — empty file for your notes (branch context, PR links, etc.)

---

## License

MIT
