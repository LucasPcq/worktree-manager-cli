# wtm — Worktree Manager

Orchestrate git worktrees and team dev workflows from the terminal.

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
wtm list                        # list all worktrees
wtm create feature/my-feature   # create a worktree
wtm switch feature/my-feature   # navigate + start services
wtm extract --to refactor       # move uncommitted changes to another worktree
wtm clean feature/my-feature    # clean up when done
```

## Teach your LLM to use wtm

If you work with Claude Code or Cursor, run:

```bash
wtm agents install
```

Detects `.claude/` and `.cursor/` (project and home) and installs a `using-wtm` skill so your agent knows every command, flag, and JSON payload — without having to explain them each session. See [Machine-readable output](#machine-readable-output---output-json) below for the underlying `--output json` contract.

## Commands

### `wtm init`

Interactive wizard that generates your configuration files. It's organized by concept, and
each optional section is introduced by a short explanation so you know what it does.

```bash
wtm init
```

On first run, creates two files:
- **Global config** (`~/.config/wtm/config.toml`) — shell type
- **Project config** (`<git-common-dir>/wtm/config.toml`) — worktree settings, env strategy, hooks

Both files live outside the working tree, so nothing is ever committed to your repo. The project config is scoped to your local clone (it lives inside `.git/`), invisible to teammates and to `git status`.

The wizard auto-detects:
- Default branch (via `git symbolic-ref`)
- `.env` and `.env.example` files
- Package manager (pnpm, npm, yarn, go, pip)
- Docker Compose files
- Monorepo packages (via `pnpm-workspace.yaml`)

**Optional sections.** Env provisioning, post-create hooks, and services/tasks are each
introduced by a **Configure / Skip** gate whose default follows what was detected — so you can
press Enter through and still land a sensible config, while discovering what wtm can do.
Skipped sections are written commented, ready to enable later. The hooks step is a small
editor where you **add, edit, remove, and reorder** `on_create` commands (cmd, cwd,
continue-on-error).

**Non-interactive bootstrap** (scripts / agents):

```bash
wtm init --non-interactive --base-branch main \
  [--env-strategy example|main|parent] [--install-command "pnpm install"] \
  [--skip-env] [--skip-hooks] [--skip-services]
```

**Reconfigure after init.** A config already exists? Instead of blocking, re-run init for a
single section to regenerate it cleanly — pre-filled with your current values, with a
confirmation before writing (`--yes` to skip):

```bash
wtm init --only env          # re-pick env strategy + files
wtm init --only hooks        # edit the on_create command list
wtm init --only services     # re-detect docker/scripts (run.toml jobs regenerated, profiles kept)
wtm init --only worktrees    # change the default base branch
```

`--only` accepts several sections (`--only env,services`) and is fully scriptable with the
flags above. `wtm config edit` remains available for hand edits.

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

### `wtm` — Worktree management

#### `wtm create [branch]`

Create a new git worktree with environment provisioning and hooks.

```bash
# Fully interactive — prompts for branch name, source branch, and env strategy
wtm create

# Specify branch name, pick source branch interactively
wtm create feature/auth

# Direct — specify everything, no interaction
wtm create feature/auth --from main --env-from parent
```

**What happens:**
1. Creates a git worktree at `<base_path>/<branch-name>` (slashes become dashes)
2. Copies `.env` files according to the configured strategy
3. Records `meta.json` (source branch, timestamp, strategy used) under `<git-common-dir>/wtm/worktrees/<branch>/`
4. Runs `on_create` hooks

**Flags:**
| Flag | Description |
|---|---|
| `--from <branch>` | Source branch (skips interactive picker) |
| `--env-from <strategy>` | Override env strategy: `example`, `main`, or `parent` |

#### `wtm list`

List all worktrees with their git status.

```bash
wtm list
```

Output:
```
  main              (parent)  ● active  clean
  feature-auth                           dirty   3 commits ahead
  feature-payment                        clean   1 commit ahead
```

In an interactive terminal, shows a picker with actions: go, start profile, stop profile, view logs, clean.

#### `wtm go [branch]`

Navigate to a worktree directory. Requires shell integration.

```bash
wtm go feature/auth     # navigate to a specific worktree
wtm go                  # interactive picker
wtm go auth             # substring match
```

Without shell integration, `wtm go` cannot change your working directory. The shell wrapper intercepts `wtm go` and performs the `cd` in your current shell.

#### `wtm switch [branch]`

Navigate to a worktree **and** start its services in one command. Combines `go` + `run up`.

```bash
wtm switch feature/auth               # go + start default profile
wtm switch feature/auth --exclusive    # go + stop others + start
wtm switch feature/auth --parallel     # go + start without stopping others
wtm switch feature/auth --profile api  # go + start specific profile
wtm switch                             # interactive picker + start
```

Requires shell integration (same as `go`).

**Flags:**
| Flag | Description |
|---|---|
| `--exclusive` | Stop services on other worktrees before starting |
| `--parallel` | Start without stopping other worktrees |
| `--profile <name>` | Service profile to start (default: default profile) |

#### `wtm clean [branch]`

Remove a worktree and its local branch. The remote branch is never touched.

```bash
wtm clean                        # interactive picker with safety checks
wtm clean feature/auth           # direct
wtm clean feature/auth --force   # skip all safety checks
```

**Safety checks:** uncommitted changes, unpushed commits, open pull request.

**Orphaned children:** if the worktree you clean is the **parent** of others (their `source_branch` points at it), removing it would leave them dangling. In interactive mode wtm shows a recap of the proposed reparenting (`dev/b: dev/a → feat`) and asks before moving the children onto the **grandparent** (the cleaned worktree's own parent, or the base branch as fallback). In non-interactive mode nothing is reparented unless you pass `--reparent-children` — otherwise the orphaned children are reported and left untouched.

**Flags:**
| Flag | Description |
|---|---|
| `--force` | Bypass all safety checks and delete immediately |
| `--reparent-children` | Reparent orphaned child worktrees onto the grandparent without prompting |

#### `wtm extract`

Move a subset of the **current worktree's uncommitted changes** to another worktree — to split an oversized PR or peel off unrelated work for easier review and parallel development. Without flags it runs an interactive wizard: pick files → target → move/copy.

```bash
wtm extract                                              # interactive: pick files, target, mode
wtm extract --files src/api.go,src/db.go --to refactor  # move files to an existing worktree
wtm extract --files src/api.go --to spike --from main   # create 'spike' from main, move into it
wtm extract --files notes.md --to docs --keep           # copy instead of move (keep in source)
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

#### `wtm sync`

Rebase **selected** managed worktrees onto their recorded parent (its `source_branch`), in cascade. Target one branch, several, or the whole stack — `main → feature → spike` all get replayed onto a fresh base in one command.

```bash
wtm sync                 # pick worktrees interactively (multi-select)
wtm sync feature         # sync just one worktree onto its (refreshed) parent
wtm sync feature spike   # sync several specific worktrees
wtm sync --all           # rebase the whole cascade, then ask before pushing
wtm sync --all --dry-run # preview the plan, fully offline (no fetch/rebase/push)
wtm sync feature --push  # rebase + force-push (with lease) without prompting
wtm sync feature --no-push   # rebase locally only, never push
wtm sync --all --base develop  # sync from a base branch other than the configured one
```

**Choosing what to sync:**
- No arguments → an interactive **multi-select picker** of worktrees (nothing pre-checked).
- One or more branch names → exactly those worktrees (a name also matches by unambiguous substring; an unknown name exits `11`).
- `--all` → every managed worktree (cannot be combined with branch arguments).
- Selecting the **base/main worktree** just fetches + fast-forwards the base (no rebase).

> In non-interactive mode (`--output json`), there is no picker — you must pass branch names or `--all`, otherwise the command exits with a usage error.

**What happens:**
1. Fetches and fast-forwards the base branch (skipped if the main worktree is dirty)
2. Walks the selected worktrees in topological order (parents before children)
3. For each branch: fetches + fast-forwards it from its own `origin/<branch>`, then rebases it onto its refreshed parent (`git rebase --onto`, replaying only that branch's own commits)
4. Shows a per-branch recap, then (unless `--no-push`) asks once before force-pushing the rebased branches with `--force-with-lease`

The cascade is **fully local** — pushing is a separate, explicitly confirmed step. On a conflict the rebase is **auto-aborted** (the working tree is left clean) and that branch's selected descendants are skipped; the command exits non-zero so you can resolve it manually and re-run.

**Per-branch status:** `synced` (rebased), `up_to_date`, `skipped_dirty` (uncommitted changes), `skipped_ancestor` (a parent was skipped or failed), `diverged` (local **and** remote both moved — left untouched for manual reconcile), `conflict` (rebase aborted), `error`, `unknown_parent`.

**Flags:**
| Flag | Description |
|---|---|
| `--all` | Sync every managed worktree (cannot be combined with branch arguments) |
| `--dry-run` | Preview the cascade without fetching, rebasing, or pushing |
| `-y, --yes` | Skip the pre-sync confirmation |
| `--push` | Force-push (with lease) rebased branches without prompting |
| `--no-push` | Rebase locally only; never push |
| `--base <branch>` | Base branch to sync from (defaults to config or detected base) |

---

#### `wtm reparent [branch]`

Change the recorded **parent** of a worktree — the branch `wtm sync` rebases it onto (its `source_branch`). Only the metadata is updated; the rebase happens on the next `wtm sync`. Handy for **stacked branches**: when a middle branch is merged (e.g. `feat → dev/a → dev/b`, with `dev/a` merged into `feat`), reparent `dev/b` onto `feat` so the cascade keeps working.

```bash
wtm reparent                       # pick the worktree, then the new parent, interactively
wtm reparent dev/b --to feat       # reparent dev/b onto feat directly
```

**What happens:**
1. Resolves the worktree (argument or interactive picker) and the new parent (`--to` or interactive picker)
2. Validates: the new parent must exist locally, a worktree can't be its own parent, and the resulting parent chain must stay acyclic
3. Rewrites the worktree's `source_branch` — run `wtm sync <branch>` afterwards to actually rebase

> In non-interactive mode (`--output json`) there is no picker — you must pass the branch and `--to`, otherwise the command exits with a usage error.

**Flags:**
| Flag | Description |
|---|---|
| `--to <branch>` | New parent branch to rebase onto |

---

#### `wtm relocate`

Reconcile every worktree with the configured `base_path`. Worktrees scattered elsewhere are **moved** into it (`git worktree move`), and worktrees you created by hand (outside wtm) are **adopted** — wtm writes their metadata (recording a parent branch) so `sync` and `.env` sync work for them too. Use it to **change `base_path`** safely (with `--to`) or to **onboard a repo** that already has worktrees lying around.

```bash
wtm relocate                      # gather scattered worktrees into the current base_path + adopt external ones
wtm relocate --to ../.worktrees   # change base_path to ../.worktrees and move existing worktrees there
wtm relocate --dry-run            # preview the plan, change nothing
wtm relocate --force              # move dirty or locked worktrees too
```

**What happens:**
1. Shows a gate screen explaining the operation, then the per-worktree plan, and asks for confirmation (text mode)
2. For each worktree to adopt, asks which branch to record as its parent (defaults to the base branch)
3. Moves worktrees not already at their target path; writes `meta.json` for any worktree without it
4. Rewrites the config `base_path` when `--to` changed it

Dirty or locked worktrees are **skipped** unless `--force`. A target path that is already occupied is **blocked** and never overwritten. Re-running after committing/cleaning finishes the job — relocate always reconciles toward the configured `base_path`.

**Per-worktree status:** `moved`, `moved_adopted` (moved **and** adopted), `adopted` (already in place, metadata written), `noop` (managed + already in place), `skipped_dirty`, `skipped_locked`, `blocked_dest`, `error`.

**Flags:**
| Flag | Description |
|---|---|
| `--to <path>` | New `base_path` (relative to repo root); also moves existing worktrees there |
| `--dry-run` | Preview the plan without moving, adopting, or rewriting config |
| `--force` | Move dirty or locked worktrees too |
| `-y, --yes` | Skip the confirmation and parent prompts |

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

### `wtm checkout [number]` — Pull request → worktree

Create a worktree from an existing pull request and run `on_create` hooks. Perfect for reviewing a teammate's PR with the full environment set up. Requires the [`gh` CLI](https://cli.github.com) installed and authenticated (`gh auth login`). Creating a PR is out of scope — `gh pr create` already does it well (templates, branch push, base detection).

```bash
wtm checkout 42                 # checkout PR #42 into a worktree
wtm checkout                    # interactive picker of open PRs
wtm checkout --mine             # picker, only PRs you authored
wtm checkout --review           # picker, only PRs awaiting your review
wtm checkout 42 --env-from main # override env strategy
wtm checkout 42 --from develop  # override the parent (sync target)
```

Behavior:
- The worktree content is the **PR head branch**; `git fetch origin <pr-branch>` runs first
- The recorded **parent** (rebase target for `wtm sync`) defaults to the PR's **base branch** — override with `--from`, or pick it interactively (base pre-selected)
- Applies the configured env strategy (interactive wizard if `--env-from` not provided)
- Runs `on_create` hooks exactly like `wtm create`
- In the interactive picker, PRs already linked to a local worktree are shown as `linked` and can't be re-selected — use `wtm go <branch>` to enter them
- Refuses if a local branch with the same name already exists — run `wtm clean <branch>` first

**Forks (by design)** — wtm doesn't check out fork PRs. A fork's branch lives on the contributor's repo (not `origin`) and a fork worktree couldn't push back, which breaks wtm's "develop here" model. To review a fork PR, use `gh pr checkout <number>`.

---

## Machine-readable output (`--output json`)

Every data-returning command supports `--output json` for scripting and LLM agents (Claude Code, Cursor, …). JSON is pretty-printed on stdout with `snake_case` fields; human messages stay on stderr; exit codes are unchanged.

**The payload *is* the schema** — the shape mirrors the command's Go domain type and stays stable. To discover the exact fields of any command, just run it with the flag:

```bash
wtm list --output json
wtm checkout 42 --output json
wtm run list --output json
```

Supported commands:

- `list`, `create`, `clean` (requires `--force`), `extract`, `sync` (requires branch names or `--all`)
- `checkout`
- `run list`, `run ps`, `run up`, `run down`, `run start`, `run stop`

Example — pipe into `jq`:

```bash
wtm list --output json | jq '.[] | select(.is_dirty).branch'
wtm checkout 42 --output json | jq '.path'
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
    └── meta.json                     # per-worktree metadata
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

Each worktree created by `wtm create` records its metadata under `<git-common-dir>/wtm/worktrees/<encoded-branch>/`:

- **`meta.json`** — source branch, creation timestamp, env strategy used

Branch names are URL-path-escaped on disk (e.g. `feat/x` → `feat%2Fx/`) so slashes don't create nested directories. It lives alongside `git`'s own per-worktree state, completely outside the working tree.

---

## License

MIT
