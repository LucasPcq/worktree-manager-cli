# wtm — Worktree Manager

Orchestrate git worktrees, AI agents, and team dev workflows from the terminal.

`wtm` manages the lifecycle of git worktrees: creation, environment provisioning, hook execution, navigation, and cleanup. It replaces manual `git worktree` commands with a streamlined workflow designed for teams working on multiple branches simultaneously.

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

# Open the interactive dashboard — manage everything from here
wtm

# Or use individual commands:
wtm new feature/my-feature      # create a worktree
wtm go feature/my-feature       # navigate to it
wtm focus feature/my-feature    # start the environment
wtm ls                          # list all worktrees
wtm clean feature/my-feature    # clean up when done
```

## Commands

### `wtm` — Interactive Dashboard

Run `wtm` without arguments to open the interactive dashboard. This is the main entry point for your daily workflow.

```bash
wtm
```

The dashboard displays all your worktrees in a two-panel layout:
- **Left panel** — worktree list with status (clean/dirty, commits ahead, focus indicator)
- **Right panel** — details of the selected worktree (path, source branch, modified files, context notes)

**Keyboard shortcuts:**

| Key | Action |
|---|---|
| `↑/↓` or `j/k` | Navigate the list or scroll the active panel |
| `Tab` / `Shift+Tab` | Cycle between panels (list → detail → hooks output) |
| `n` | Create a new worktree (opens the wizard) |
| `d` | Clean the selected worktree (opens confirmation) |
| `f` | Focus the selected worktree (runs hooks, shows output in split panel) |
| `Enter` | Navigate to the selected worktree directory |
| `r` | Refresh the worktree list |
| `Esc` | Close the hooks output panel |
| `q` | Quit the dashboard |

When you press `f` to focus a worktree, the right panel splits to show the hook output in real-time. The log panel stays visible after hooks complete and can be closed with `Esc`.

---

### `wtm init`

Interactive wizard that generates your configuration files.

```bash
wtm init
```

On first run, creates two files:
- **Global config** (`~/.config/wtm/config.toml`) — shell type, default AI agent
- **Project config** (`.wtm.toml`) — worktree settings, env strategy, hooks

The wizard auto-detects:
- Default branch (via `git symbolic-ref`)
- `.env` and `.env.example` files
- Package manager (pnpm, npm, yarn, go, pip)
- Docker Compose files
- Monorepo packages (via `pnpm-workspace.yaml`)

If `.wtm.toml` already exists, the command exits — delete it or edit manually to reconfigure.

---

### `wtm new [branch]`

Create a new git worktree with environment provisioning and hooks.

```bash
# Fully interactive — prompts for branch name, source branch, and env strategy
wtm new

# Specify branch name, pick source branch interactively
wtm new feature/auth

# Direct — specify everything, no interaction
wtm new feature/auth --from main --env-from parent
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

---

### `wtm ls`

List all worktrees with their git status.

```bash
wtm ls
```

Output:
```
  main              (parent)  ● active  clean
  feature-auth                           dirty   3 commits ahead
  feature-payment                        clean   1 commit ahead
```

Columns:
- **Branch name**
- **Tags** — `(parent)` for the main worktree, `● active` for the focused worktree
- **Git status** — `clean` or `dirty` (uncommitted changes)
- **Commits ahead** — number of commits ahead of the base branch

---

### `wtm go [branch]`

Navigate to a worktree directory. Requires shell integration.

```bash
# Navigate to a specific worktree
wtm go feature/auth

# Interactive picker if no argument
wtm go

# Substring match — if only one worktree matches, navigates directly
wtm go auth
```

**Setup required:** Add this to your shell config (`.zshrc`, `.bashrc`, or `config.fish`):

```bash
eval "$(wtm shell-init)"
```

Without shell integration, `wtm go` cannot change your working directory (a child process cannot `cd` its parent). The shell wrapper intercepts `wtm go` and performs the `cd` in your current shell.

---

### `wtm focus [branch]`

Switch the active worktree and run lifecycle hooks.

```bash
# Focus on a worktree — runs on_blur on previous, on_focus on target
wtm focus feature/auth

# Interactive picker if no argument
wtm focus

# Stop everything — runs on_blur and clears state
wtm focus --off
```

`focus` and `go` are independent and composable:
- `wtm go` changes your directory
- `wtm focus` manages your environment (starts/stops services via hooks)
- Use both: `wtm focus feature/auth && wtm go feature/auth`

**Flags:**
| Flag | Description |
|---|---|
| `--off` | Run blur hooks on active worktree and clear state |

---

### `wtm clean [branch]`

Remove a worktree and its local branch. The remote branch is never touched.

```bash
# Interactive — pick from list, confirm with safety checks
wtm clean

# Direct — specify branch
wtm clean feature/auth

# Skip all safety checks
wtm clean feature/auth --force
```

**Safety checks (before deletion):**
- Uncommitted changes in the worktree
- Unpushed commits to remote
- Open pull request (detected via `gh` CLI, skipped if not installed)

If any check triggers, the wizard offers three options: delete, force delete (bypass checks), or cancel.

The parent worktree cannot be cleaned.

**Flags:**
| Flag | Description |
|---|---|
| `--force` | Bypass all safety checks and delete immediately |

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

## Configuration

### Project config — `.wtm.toml`

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

### Global config — `~/.config/wtm/config.toml`

Created by `wtm init` on first run. Not committed — personal to each developer.

```toml
shell = "zsh"          # zsh | bash | fish
agent = "claude-code"  # claude-code | cursor | none
```

The project `.wtm.toml` can override the agent setting. Shell is always global.

---

## State

`wtm` tracks the active worktree in `~/.config/wtm/state.json`. This file is managed automatically by `wtm focus` — don't edit it manually.

---

## Worktree metadata

Each worktree created by `wtm new` contains a `.wtm/` directory with:

- **`meta.json`** — source branch, creation timestamp, env strategy used
- **`context.md`** — empty file for your notes (branch context, PR links, etc.)

---

## License

MIT
