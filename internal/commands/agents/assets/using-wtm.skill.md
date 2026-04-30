---
name: using-wtm
description: Use this skill whenever the user wants to create, list, switch, focus, or clean git worktrees; start, stop, or inspect per-worktree dev jobs (services + tasks); or list, create, or check out GitHub pull requests — even when they don't explicitly say "wtm". Always pass --output json on wtm data commands so you can parse results; never invoke wtm through an interactive picker.
---

# Using wtm

wtm is a CLI that manages **git worktrees**, **per-worktree dev jobs** (long-running services + one-shot tasks, via a background daemon), and **GitHub pull requests**. It's designed to be driven by LLMs: every data command accepts `--output json` and prints machine-parseable results to stdout, while human messages stay on stderr.

## How to drive wtm from an LLM

1. **Always pass arguments.** wtm commands without an argument often drop into an interactive picker, which you can't navigate. Pick the branch, PR number, profile, or service name from a prior `list` call first.
2. **Always add `--output json`** on data commands. The JSON is pretty-printed on stdout with `snake_case` fields. Human text and warnings stay on stderr; ignore stderr unless exit code is non-zero.
3. **Trust exit codes.** `0` on success, non-zero on failure. Error detail is plain text on stderr — surface it to the user on failure.
4. **Destructive commands need `--force`** (`wt clean`) because a confirmation prompt can't run from a non-interactive context.
5. **Every command supports `--help`.** Run `wtm <cmd> --help` if you need flags beyond the reference below.

## Discovery first — know before you act

Run these before taking action so you have names to pass as arguments:

| Goal | Command |
|---|---|
| All worktrees (branch, path, PR, services, dirty?) | `wtm wt list --output json` |
| All open PRs (number, title, branch, state, draft, url) | `wtm pr list --output json` |
| Declared jobs + profiles | `wtm run list --output json` |
| Jobs running right now (name, kind, status, pid, workdir) | `wtm run ps --output json` |
| Resolved project config (TOML on stdout) | `wtm config show` |

## Worktree commands (`wtm wt`)

- **`wtm wt list --output json`** — inventory of worktrees.
- **`wtm wt create <branch> --from <base> --output json`** — create a new worktree, run env provisioning + `on_create` hooks. Optional: `--env-from example|main|parent` to override the env strategy.
- **`wtm wt clean <branch> --force --output json`** — remove worktree and local branch (remote untouched). `--force` is mandatory in JSON mode.
- **`wtm wt go <branch>`** and **`wtm wt switch <branch>`** — navigate to a worktree (and start services, for `switch`). These **require the user's shell integration** to `cd`, so an LLM can't drive them directly. Prefer `wt list` + tell the user which branch to run `switch` on.
- **`wtm wt focus <branch>`** — mark a worktree as the active one in the state file. Pass the branch explicitly.

## Run commands (`wtm run`)

Jobs are declared in a per-clone `run.toml` (managed by wtm — agents never touch it directly) and executed by a background daemon. Each job has a `kind`:

- **`kind = "service"`** — long-running. With a `stop` command, it's a detached launcher (e.g. `docker compose up -d`); otherwise it's tracked by PID and killed via SIGTERM.
- **`kind = "task"`** — one-shot script. Blocks the profile, streams output live, removed after exit. A non-zero exit aborts the profile.

Profiles are named groups of jobs (run in declared order). The same TOML can host multiple profiles (e.g. `dev`, `test`, `staging`).

- **`wtm run list --output json`** — config introspection (jobs + profiles declared).
- **`wtm run ps --output json`** — runtime state (jobs running right now, with kind).
- **`wtm run up [profile] --output json`** — execute a profile in order. No arg → default profile. Flags: `--exclusive` (stop jobs on other worktrees first), `--parallel` (don't stop anything).
- **`wtm run down [profile] --output json`** — stop jobs in the **current worktree** (or a specific profile). Other worktrees are never touched. Add `--all` to stop jobs across every worktree.
- **`wtm run start <job> --output json`** — start one job. Tasks block until they exit; services launch in the background.
- **`wtm run stop <job> --output json`** — stop one job.
- **`wtm run logs [job]`** — attach to a job's PTY. No `--output json`: it's a raw text stream (already machine-readable).
- **`wtm run export [--profile <name>]`** — emit the run config as JSON on stdout. Use `--profile` to export only one profile and its jobs. Pipe to a file: `wtm run export > layout.json`.
- **`wtm run import [file|-] [--replace --force] [--output json]`** — ingest a JSON run config. Omit the file or pass `-` to read from stdin. Default: append new jobs/profiles, skip duplicates (prints what was added/skipped). `--replace --force` overwrites the file entirely.

## Config commands (`wtm config`)

- **`wtm config show`** — print the resolved project `config.toml` and its on-disk path. Use it to inspect `worktrees.base_path`, `env.strategy`, `hooks.on_create`, etc., before suggesting changes. Output is plain TOML on stdout.
- **`wtm config edit`** — opens `$EDITOR` on the config file. Interactive — **never invoke from an agent**. If the user wants to change a setting, run `wtm config show` to read the current state, then ask the user to run `wtm config edit` (or do the edit through a `Write`/`Edit` on the printed path if you have those tools and the user authorized the change).

## Pull request commands (`wtm pr`)

Backed by the `gh` CLI — the user must have `gh auth login` set up.

- **`wtm pr list --output json`** — open PRs. Filters: `--mine`, `--review`.
- **`wtm pr create --title "..." --base <branch> --output json`** — creates a PR for the current branch. Add `--draft` for draft PRs. Pass `--title` AND `--base` AND `--draft` (or omit `--draft`) to skip the wizard entirely.
- **`wtm pr checkout <number> --output json`** — fetch the PR's branch and create a worktree for it. Optional: `--env-from`. Refuses fork PRs.

## Conventions and invariants

- **Stdout** is structured (JSON when requested); **stderr** is for humans.
- **JSON payloads mirror Go structs** — field names are snake_case, the shape is stable. Run a command with `--output json` once to learn the exact schema; don't over-rely on any field being present besides the ones shown in this skill.
- **Idempotence is partial.** `wt create` fails if the worktree path already exists; `wt clean` fails if the branch isn't known. Check `wt list` first when in doubt.
- **Never invent names.** Branch names, service names, profile names, and PR numbers must come from a preceding `list` call, from the user's message, or from repo state (`git branch`, etc.).

## Failure handling

On non-zero exit, read stderr. Common cases:

- `config not found` → the repo isn't initialized; tell the user to run `wtm init` (interactive — they have to do it themselves).
- `branch not found` / `worktree not found` → the name is wrong; re-list.
- `PR already exists for branch X` → use `pr list` to find it instead of creating a new one.
- `gh: …` → the `gh` CLI isn't authenticated; tell the user to run `gh auth login`.

## Escalate to the user when

- A command requires shell integration (`wt go`, `wt switch` without a shell wrapper installed).
- A destructive action (`wt clean`) wasn't explicitly authorized by the user — do **not** add `--force` on your own initiative.
- `wtm init` is needed — the wizard is interactive and the user must answer questions.
- `wtm config edit` would be the natural answer — it opens `$EDITOR`. Either ask the user to run it themselves, or read with `wtm config show` and write the change directly to the path if you have a file-edit tool.
