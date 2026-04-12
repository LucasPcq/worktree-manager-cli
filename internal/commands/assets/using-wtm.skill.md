---
name: using-wtm
description: Use this skill whenever the user wants to create, list, switch, focus, or clean git worktrees; start, stop, or inspect per-worktree dev services; or list, create, or check out GitHub pull requests — even when they don't explicitly say "wtm". Always pass --output json on wtm data commands so you can parse results; never invoke wtm through an interactive picker.
---

# Using wtm

wtm is a CLI that manages **git worktrees**, **per-worktree dev services** (via a background daemon), and **GitHub pull requests**. It's designed to be driven by LLMs: every data command accepts `--output json` and prints machine-parseable results to stdout, while human messages stay on stderr.

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
| Declared services + profiles from `.wtm/services.toml` | `wtm svc list --output json` |
| Services running right now (name, status, pid, workdir) | `wtm svc ps --output json` |

## Worktree commands (`wtm wt`)

- **`wtm wt list --output json`** — inventory of worktrees.
- **`wtm wt create <branch> --from <base> --output json`** — create a new worktree, run env provisioning + `on_create` hooks. Optional: `--env-from example|main|parent` to override the env strategy.
- **`wtm wt clean <branch> --force --output json`** — remove worktree and local branch (remote untouched). `--force` is mandatory in JSON mode.
- **`wtm wt go <branch>`** and **`wtm wt switch <branch>`** — navigate to a worktree (and start services, for `switch`). These **require the user's shell integration** to `cd`, so an LLM can't drive them directly. Prefer `wt list` + tell the user which branch to run `switch` on.
- **`wtm wt focus <branch>`** — mark a worktree as the active one in the dashboard / state. Pass the branch explicitly.

## Service commands (`wtm svc`)

Services are defined in `.wtm/services.toml` and executed by a background daemon. Profiles are named groups of services.

- **`wtm svc list --output json`** — config introspection (what's declared).
- **`wtm svc ps --output json`** — runtime state (what's running).
- **`wtm svc up [profile] --output json`** — start all services in a profile. No arg → default profile. Flags: `--exclusive` (stop services on other worktrees first), `--parallel` (don't stop anything).
- **`wtm svc down [profile] --output json`** — stop services in the **current worktree** (or a specific profile). Other worktrees are never touched. Add `--all` to stop services across every worktree.
- **`wtm svc start <service> --output json`** — start one service.
- **`wtm svc stop <service> --output json`** — stop one service.
- **`wtm svc logs [service]`** — stream logs. No `--output json` here: logs are a raw text stream (already machine-readable).

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
