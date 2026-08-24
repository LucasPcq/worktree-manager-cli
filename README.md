# wtm — Worktree Manager

Orchestrate git worktrees and team dev workflows from the terminal.

`wtm` manages the full lifecycle of git worktrees — creation, environment
provisioning, hooks, stacked-branch syncing, per-worktree dev services, navigation,
and cleanup — replacing manual `git worktree` juggling with one streamlined workflow.

> **This README is a guide, not a reference.** Every command is self-documenting:
> run `wtm <command> --help` for its full flags, or browse the generated reference in
> [`docs/`](docs/wtm.md). The tables below are just a map.

## Dependencies

| Tool | Required | Purpose |
|---|---|---|
| `git` | ✅ Required | Worktree management |
| `gh` | ⭐ Recommended | PR listing, creation, checkout, and prune (merged/closed detection) |

`gh` is optional — worktree creation, navigation, hooks, and services all work without
it. Install and authenticate it to unlock GitHub features: [cli.github.com](https://cli.github.com).

## Installation

**Homebrew (macOS / Linux)**

```bash
brew install LucasPcq/tap/wtm
```

**Download binary** — grab the latest [release](https://github.com/LucasPcq/worktree-manager-cli/releases), extract, and move it onto your `PATH`:

```bash
tar -xzf worktree-manager-cli_*_darwin_arm64.tar.gz   # or _darwin_amd64 / _linux_amd64
sudo mv wtm /usr/local/bin/
```

**Go install**

```bash
go install github.com/LucasPcq/wtm@latest
```

## Quick Start

```bash
# 1. Shell integration — required so `go`/`switch` can cd for you
echo 'eval "$(wtm shell-init)"' >> ~/.zshrc && source ~/.zshrc

# 2. Initialize wtm in your repo
cd your-repo && wtm init

# 3. Everyday flow
wtm create feature/login     # create a worktree (env + hooks provisioned)
wtm go feature/login         # cd into it
wtm list                     # see all worktrees
wtm sync --all               # rebase the stack onto its parents
wtm clean feature/login      # remove it when the PR merges
```

## Teach your LLM to use wtm

Working with Claude Code or Cursor? Run:

```bash
wtm agents install
```

It detects `.claude/` and `.cursor/` (project and home) and installs a `using-wtm`
skill so your agent can drive every command via the [`--output json`](#machine-readable-output) contract — without being told how each session.

## Concepts

A few ideas explain how the commands fit together:

- **Worktree** — a checked-out branch in its own directory. wtm creates them under
  `base_path` (from `wtm init`) and records per-worktree metadata under
  `<git-common-dir>/wtm/`. A branch that already exists locally is checked out as-is
  (`create` and `checkout` both keep its commits — nothing to clean up first); only a
  branch another worktree already holds is refused. See [`create`](docs/wtm_create.md), [`list`](docs/wtm_list.md), [`clean`](docs/wtm_clean.md).
- **Stacking** — every worktree records the parent branch it was created from
  (`source_branch`) — the branch it was created from, or the parent you pick when reusing
  an existing branch. [`sync`](docs/wtm_sync.md) rebases a worktree (and its descendants,
  in cascade) onto its parent; [`tree`](docs/wtm_tree.md) shows the parent→child forest and
  which branches need syncing; [`reparent`](docs/wtm_reparent.md) rewires the parent after a
  middle branch merges.
- **Env provisioning** — on `create`, wtm copies `.env` files into the new worktree using
  a **strategy** (`example` / `main` / `parent`). See [Env strategies](#env-strategies).
- **Shell integration** — `go` changes your current directory, which a child process can't
  do for its parent shell. `eval "$(wtm shell-init)"` installs a shell function that makes
  it work. Without it, use [`resolve`](docs/wtm_resolve.md) to get a path.
- **Dev jobs** *(experimental)* — long-running **services** (dev servers, docker) and
  one-shot **tasks** (migrations, seeds) declared in `run.toml` and grouped into
  **profiles**, run per-worktree by a background daemon. Starting them **attaches**: the
  run view opens one pane per job, leaving it detaches without stopping anything, and
  `-d` skips it. The flow is still stabilizing — `wtm go` is the recommended way to enter
  a worktree today. See [`run`](docs/wtm_run.md) and [Run config](#run-config--runtoml).

## Commands

Full flags live in `wtm <command> --help` and [`docs/`](docs/wtm.md). Overview:

### Worktrees

| Command | Purpose |
|---|---|
| [`create`](docs/wtm_create.md) | Create a new worktree (runs env provisioning + `on_create` hooks) |
| [`list`](docs/wtm_list.md) | List all worktrees |
| [`tree`](docs/wtm_tree.md) | Show the worktree forest (parent → child) |
| [`clean`](docs/wtm_clean.md) | Remove a worktree and its local branch |
| [`prune`](docs/wtm_prune.md) | Remove finished worktrees (merged / closed PR / gone) in one pass — merged/closed need `gh` |
| [`extract`](docs/wtm_extract.md) | Move uncommitted changes to another worktree (split an oversized PR) |
| [`env`](docs/wtm_env.md) | Detect and fix a worktree's `.env` drift against its template + value source |
| [`relocate`](docs/wtm_relocate.md) | Move worktrees to align with `base_path` and adopt external ones |
| [`ui`](docs/wtm_ui.md) | Open the full-screen worktree dashboard: browse state and PRs, create and delete worktrees |

### Navigate

| Command | Purpose |
|---|---|
| [`go`](docs/wtm_go.md) | cd into a worktree |
| [`switch`](docs/wtm_switch.md) | cd into a worktree and start its dev services *(experimental)* |
| [`resolve`](docs/wtm_resolve.md) | Print a branch's worktree path (for scripts / agents) |

### Stacked branches

| Command | Purpose |
|---|---|
| [`sync`](docs/wtm_sync.md) | Rebase selected worktrees onto their parent, in cascade |
| [`reparent`](docs/wtm_reparent.md) | Change the parent a worktree is rebased onto |

### Dev jobs *(experimental)*

Per-worktree services + tasks. Functional, but the flow is still stabilizing. The
run module is **opt-in**: run `wtm run init` once to set it up (the global `wtm init`
no longer touches services). Until then, run commands stop with a hint pointing there.

| Command | Purpose |
|---|---|
| [`run init`](docs/wtm_run_init.md) | Set up run.toml (detect docker-compose + scripts, pre-fill ports, link .env keys) |
| [`run up`](docs/wtm_run_up.md) / [`down`](docs/wtm_run_down.md) | Start / stop a profile's jobs (`up` attaches, `-d` detaches) |
| [`run start`](docs/wtm_run_start.md) / [`stop`](docs/wtm_run_stop.md) | Start / stop a single job (`start` attaches, `-d` detaches) |
| [`run ps`](docs/wtm_run_ps.md) / [`list`](docs/wtm_run_list.md) | Running jobs / declared jobs + profiles |
| [`run logs`](docs/wtm_run_logs.md) | Reopen the run view on this worktree's jobs |
| [`run export`](docs/wtm_run_export.md) / [`import`](docs/wtm_run_import.md) | Share a job layout between machines |
| [`run job`](docs/wtm_run_job.md) / [`profile`](docs/wtm_run_profile.md) | Add / remove / edit jobs and profiles |

### GitHub

| Command | Purpose |
|---|---|
| [`checkout`](docs/wtm_checkout.md) | Create a worktree from an existing pull request (needs `gh`) |

### Setup

| Command | Purpose |
|---|---|
| [`init`](docs/wtm_init.md) | Initialize wtm configuration |
| [`shell-init`](docs/wtm_shell-init.md) | Generate the shell integration function |
| [`config`](docs/wtm_config.md) | Inspect or edit the project config |
| [`agents`](docs/wtm_agents.md) | Install the `using-wtm` skill for LLM agents |
| [`schema`](docs/wtm_schema.md) | Extract the bundled JSON Schemas |

## Machine-readable output

Every data command supports `--output json` for scripting and LLM agents. JSON is
pretty-printed on stdout with `snake_case` fields; human messages stay on stderr; exit
codes are unchanged.

**The payload *is* the schema** — its shape mirrors the command's Go type and stays
stable. Discover the exact fields by running the command once:

```bash
wtm list --output json | jq '.[] | select(.is_dirty).branch'
wtm checkout 42 --output json | jq '.path'
wtm run ps --output json | jq '.[] | select(.status=="running").name'
```

Non-interactive note: `--output json` never prompts, so destructive commands need an
explicit flag — `clean`/`prune` need `--yes` (or `--force`), and `sync` needs branch
args or `--all`. See each command's `--help`.

## Configuration

All wtm files live under `<git-common-dir>/wtm/` (`.git/wtm/` for a normal clone). Git
never commits anything inside `.git/`, so wtm is invisible to teammates and to
`git status` — worktree usage stays personal.

```
<git-common-dir>/wtm/
├── config.toml                    # project settings
├── run.toml                       # dev jobs + profiles
├── schemas/                       # JSON schemas for editor autocomplete
└── worktrees/<encoded-branch>/
    └── meta.json                  # per-worktree metadata (source branch, timestamp, env strategy)
```

Everything is plain TOML, validated at load time (unknown keys are rejected, not
silently ignored). Edit by hand, or use `wtm config show` / `wtm config edit` and the
`wtm run import` flow.

### Project config — `config.toml`

Generated by `wtm init`, per-clone, never committed.

```toml
[worktrees]
base_path   = "../.trees"   # where worktrees are created (relative to repo root)
base_branch = "main"        # default base for new worktrees

[env]
strategy = "example"        # example | main | parent (see below)

# Each detected value file and its committed template (schema). wtm distinguishes
# templates (.env.example / .dist / .sample / .template / .tmpl, committed) from
# value files (.env, gitignored). .env.local is detected and flagged local but
# stays syncable.
[[env.file]]
target   = ".env"
template = ".env.example"

[[env.file]]
target = ".env.local"
local  = true

[hooks]
on_create = [
  "pnpm install",                                             # string: runs from worktree root
  { cmd = "pnpm install", cwd = "apps/api" },                 # object: runs from a subdir
  { cmd = "pnpm install", cwd = "apps/web", continue_on_error = true },  # non-fatal
]
on_clean = [
  "docker compose down",                                      # runs right before a worktree is removed
]
```

`on_create` hooks run after a worktree is created; `on_clean` hooks run in the worktree
just before it is removed by `clean`/`prune` (e.g. to tear down external resources). A
non-zero hook aborts the operation unless the entry sets `continue_on_error`. Hooks
interpolate `{{worktree}}`, `{{branch}}`, `{{root}}`, and (for `on_create`) `{{from_branch}}`.

### Env strategies

| Strategy | Behavior |
|---|---|
| `example` | Copies `file.example` from the main worktree, renamed to `file`. Warns if `.example` is missing. |
| `main` | Copies the actual file from the main worktree. |
| `parent` | Copies from the source worktree (`--from`), falling back to `main`. |

The strategy is recorded per worktree. Later, [`wtm env`](docs/wtm_env.md) reconciles a
worktree's `.env` against its **template** (the committed schema) plus the **same value
source** — adding missing keys, and (with `--mode refresh`) settling values that drifted.
Override the source for a single run with `--from`; the report always shows which source
was used.

### Run config — `run.toml`

Optional and opt-in — created by `wtm run init` (which detects docker-compose files
and package scripts), not by the global `wtm init`. Declares dev **jobs** and groups
them into **profiles**. Per-clone, never committed — share layouts with
`wtm run export | wtm run import -`.

A job's `cmd` (and its `stop`) is a **`/bin/sh` line**, not a whitespace-split argv:
quotes, `&&`, pipes, redirections and globs behave as they do in a terminal, and `${VAR}`
expands from the job's environment. POSIX `sh` is used on every machine, never your own
interactive shell, so a shared `run.toml` behaves the same everywhere.

```toml
[[job]]
name = "docker"
kind = "service"            # long-running; with `stop` it's detached
cmd  = "docker compose up -d"
stop = "docker compose down"
  [job.ports]              # host binding per worktree; template it as "${DB_PORT}:5432"
  DB_PORT = 5432

[[job]]
name = "web"
kind = "service"
cmd  = "pnpm dev"
  [job.ports]              # PORT=3000 on the main checkout, 3010 on the next worktree
  PORT = 3000

[[job]]
name = "migrate"
kind = "task"               # one-shot; blocks the profile, streams output, non-zero aborts
cmd  = "pnpm migrate"

[[profile]]
name    = "full"
jobs    = ["docker", "web", "migrate"]
default = true
```

Jobs are scoped per worktree at runtime: starting `docker` from worktree A runs it with
`cwd = A`; a separate process runs from worktree B. `wtm run down` only stops the current
worktree's jobs unless you pass `--all`.

Every job — and every `on_create` / `on_clean` hook — also runs with the worktree's own
identity in its environment, so two worktrees running the same services never share a
resource:

| Variable | Value |
| --- | --- |
| `WTM_BRANCH` | the branch, verbatim |
| `WTM_WORKTREE` | the branch as a slug safe for a Docker project, network or volume name |
| `WTM_ORDINAL` | the worktree's stable number. The main checkout is always `0`; every other worktree gets the smallest number free, kept for its whole life and released when it is cleaned |
| `WTM_PORT_OFFSET` | `WTM_ORDINAL` × the block (`port_offset_block`, 10 by default) — the main checkout keeps the project's default ports |
| `COMPOSE_PROJECT_NAME` | `<repo>-<WTM_WORKTREE>`, unless your own environment already defines it. The Docker daemon is machine-wide, so the repository qualifies the name: two clones both sitting on `main` do not share a stack |

`COMPOSE_PROJECT_NAME` is what keeps two worktrees' containers, networks and volumes
apart — nothing to declare, it works as soon as your jobs use `docker compose`. It reaches
everything compose names for you, and nothing your file names itself: a `container_name`,
or a volume's or network's explicit `name`, is resolved by the Docker daemon directly, so
the second worktree to start meets the first one's. `wtm run init` finds those and offers
to front them with the project — see [absolute names](#absolute-names) below.

**Ports** are declared per job, and wtm injects `base + WTM_PORT_OFFSET` under the name
you chose — the command itself needs no arithmetic:

```console
$ wtm run job add web --cmd "pnpm dev" --port PORT=3000
$ wtm run up
✓ web started · PORT=3010
```

**wtm checks that the port was actually bound.** Declaring a port only injects a
variable — nothing guarantees the command reads it. Once the jobs are up, `run up` dials
each declared port and reports the ones nothing answers on:

```console
$ wtm run up
✓ web started · WEB_PORT=5183

  Ports declared but not bound
  web · nothing is listening on WEB_PORT=5183
    but 5173 is listening — the base port
    the command ran, but the variable did not reach it
```

The second line is the signature of a variable that never arrived: a CLI that only takes
`--port`, a hard-coded port, a `.env` that wins, or a task runner filtering the
environment — **Turborepo does this by default** (`envMode: "strict"`), so a root
`turbo run dev` job needs `globalPassThroughEnv` in `turbo.json` for the ports to reach
its packages. wtm never edits those files; it tells you what it observed.

It never fails the run, and a healthy stack costs nothing — the check stops as soon as
every port answers. `--no-probe` skips it, and `port_probe_timeout` in run.toml sets the
budget (default 15s, a negative value turns it off).

A declaration overrides whatever the environment already sets for that variable, and the
job's `stop` command runs with the same ports its `cmd` did. For Docker, template the host
side of the mapping (`"${DB_PORT}:5432"`) and declare `DB_PORT = 5432`: the container port
never moves, only the binding does.

`wtm run init` composes a configuration you can start, not an inventory of the repo. It
proposes everything it finds and **checks the fewest things**: only scripts whose name
contains `dev`, and not a root `dev` a workspace package also declares — that one is an
orchestrator (`turbo run dev`, `pnpm -r dev`) and running it beside the packages it fans
out to would start each of them twice on the same ports. Nothing unchecked is written.

It then walks you through the ports detection pre-filled, and the **profiles** `wtm run
up` will offer: one per package, plus one gathering everything, which you rename, merge
or drop. Jobs at the repository root — a compose stack — join every profile, so starting
one package alone still brings its infrastructure up. In a single-package repo the split
collapses to one profile.

A service detection found no port for is reported rather than asked about: inventing one
would move the guess onto you, and `wtm run up` will say the port was never bound anyway.

`wtm run init` writes those Docker declarations for you. It reads the `ports:` of the
compose files you pick: a mapping that already reads a variable is declared as-is, while a
literal `"5432:5432"` would bind the same port in every worktree and is therefore **not**
declared — wtm shows the line to write instead. Pass `--patch-compose` and it makes the
change itself:

```diff
   postgres:
     ports:
-      - "5432:5432"
+      - "${POSTGRES_PORT:-5432}:5432"
```

Only the port value is rewritten, at its exact position — comments, indentation and
quoting style are untouched — and the `:-5432` default keeps `docker compose up` working
on its own, with no dependency on wtm. Re-running `run init` backfills a compose job that
predates declarative ports without overwriting one you set by hand.

Dev servers are pre-filled the same way, from the env files next to their `package.json` —
a `PORT` or `*_PORT` entry in `.env.local`, `.env`, or a committed `.env.example`. Each
job takes the file in its own directory, so in a monorepo every package keeps its own port.
wtm declares the port and nothing else: it never edits your `.env` and never rewrites a
command. Checking that the command reads the variable is yours to do — `next dev` and most
node servers read `PORT` from the environment, while a CLI that only takes a flag needs
`--cmd 'pnpm dev --port ${PORT}'`. The port it declares is also the base the `[[env_port]]`
links below follow, so a `.env` holding both `PORT=5173` and a `VITE_API_URL` pointing at
it ends up with the two shifted together.

<a id="absolute-names"></a>
**Absolute names.** A compose file that pins its own names bypasses the project prefix,
which is what makes a second worktree fail outright — Docker refuses a duplicate
`container_name`, and a volume or network pinned by `name` is silently shared instead.
`run init` reports them, and `--patch-compose` fronts each with the project:

```diff
   postgres:
-    container_name: myapp-postgres
+    container_name: "${COMPOSE_PROJECT_NAME:-myapp}-postgres"
```

The `:-myapp` default reproduces the name the file used to pin, so `docker compose up`
on its own is unchanged. Ports and names are one question, not two — accepting half of
them still leaves two worktrees unable to run at once. A `name` under `external: true`
is left alone (sharing it is the declaration's whole point), as is one that already reads
a variable, and a volume declared as a bare key was never affected: compose already
prefixes it with the project. **A volume that pinned its `name` gains one per worktree,
each starting empty** — the data already written stays under the old name, and moving it
across is yours to do.

wtm declares only what it can actually isolate, and says why for the rest: a port range,
a mapping with no host port, a `ports:` list carrying a YAML **anchor or alias** (rewriting
it would move every service sharing it), a `${DB_PORT}` with no default (the file never
says which port it stands for), and a variable two services declare with two different
defaults. It also withdraws a detected port rather than write a `run.toml` its own loader
would refuse — two bases a multiple of the block apart — naming both sides.

A server that **ignores** `PORT` and only takes its port as a CLI flag — Vite is the usual
one — reads it back from the same variable, because `cmd` is a shell line:

```console
$ wtm run job add web --cmd 'pnpm dev --port ${PORT}' --port PORT=3000
```

#### Ports hard-coded in a `.env`

Shifting a service's host port only helps if whatever connects to it follows. That is easy
when the consumer reads `${DB_PORT}` — but in most projects the port is not in a variable
of its own, it is **buried in a URL**: `DATABASE_URL=postgres://u:pw@localhost:5432/app`,
`API_URL=http://localhost:3000/api`. An app running on the host, outside Docker, then talks
to the wrong worktree.

An `[[env_port]]` link says which key carries which port:

```toml
[[env_port]]
file = ".env"
key  = "DATABASE_URL"
port = "POSTGRES_PORT"      # a port declared by one of the jobs above
```

The link names the key, never a position. wtm looks for the **declared base** inside the
value and shifts only that number, leaving credentials, host, path and query exactly as
they were:

```diff
-DATABASE_URL=postgres://u:pw@localhost:5432/app
+DATABASE_URL=postgres://u:pw@localhost:5442/app
```

`wtm run init` scans your configured `.env` targets and offers the keys whose value holds a
declared base; `--link-env` writes them without asking. Nothing is ever inferred without one
or the other. The rewrite then happens when a worktree is created — proposed interactively,
applied under `--yes` — and whenever `wtm env` reconciles, whose recap offers "Apply, but
leave the port values alone" beside the plain apply, so neither command imposes the pass
(`--check` reports without writing, and counts a pending shift as drift). `wtm env --mode refresh` compares linked values **modulo the offset**, so a
worktree holding `5442` against a `main` holding `5432` is not a conflict; a real difference
in the same value still is.

wtm reports rather than guesses when it cannot be sure: the key is missing, the base appears
more than once in the value, or neither the base nor any offset of it is there. Rewriting on
a guess could corrupt a URL, so those lines are named and left alone.

Two base ports must not differ by a **multiple of the block**, or two worktrees end up on
the same one — `3000` and `3010` are refused when `run.toml` is read, naming both sides,
which is the last moment the problem is still explainable. Neighbouring ports are fine:
a uniform offset preserves the gaps, so `5434`/`5435`/`5436` become `5444`/`5445`/`5446`
on the next worktree. Set `port_offset_block` at the top of `run.toml` when a project's
ports genuinely need more room than 10.

> **Upgrading:** jobs used to run with no `COMPOSE_PROJECT_NAME`, so `docker compose`
> named the project after the working directory. Stacks started before this version are
> under the old name and a new `run up` will not find them — stop them once with
> `docker compose -p <old-name> down`.
>
> The run daemon is global and outlives the command that started it, so a daemon already
> running keeps serving with the binary that forked it: right after an upgrade your jobs
> may still start without their ports. `wtm run down` and let the next command fork a
> fresh one.

`run up` and `run start` **attach**: a full-screen view opens with one pane per job, and
`wtm run logs` reopens it later. Leaving the view (`q`, or Ctrl+C outside focus mode)
detaches — the daemon keeps the jobs running. `-d` starts them and hands the prompt back
instead. Without a terminal, or under `--output json`, no view opens: the run reports
itself as lines, which is what a script or an agent gets. Each job's output is also
journaled to `<git-common-dir>/wtm/logs/<url-escaped-branch>/<job>.log` (5 MB x 3), and
`run logs` reads that back for a job that is no longer running.

### Global config — `~/.config/wtm/config.toml`

Created by `wtm init`, personal to each developer.

```toml
shell = "zsh"          # zsh | bash | fish

[ui]
animations = true      # false disables every wtm ui animation (tab rule, new-row flash)
```

`ui.animations` defaults to on when absent — set it to `false` to turn off every
`wtm ui` animation at once, useful over a slow or laggy connection.

## IDE autocomplete + validation

Every TOML file `wtm init` writes starts with a `#:schema ./schemas/...json` directive.
Pair it with [Even Better TOML](https://marketplace.visualstudio.com/items?itemName=tamasfe.even-better-toml)
(or the bundled JetBrains TOML plugin) for autocomplete, hover docs, and real-time
validation. Schemas ship with the binary and are written locally by `wtm init` — no
internet required. Re-extract them after upgrading:

```bash
wtm schema dump            # <git-common-dir>/wtm/schemas/{run,project}.schema.json
wtm schema dump --global   # ~/.config/wtm/schemas/global.schema.json
```

## Contributing to the docs

The [`docs/`](docs/wtm.md) reference is generated from the Cobra command tree — never
edit it by hand. After changing any command or flag, regenerate:

```bash
make docs
```

## License

MIT
