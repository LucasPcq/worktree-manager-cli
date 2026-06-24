---
name: using-wtm
description: Use this skill whenever the user wants to create, list, switch, or clean git worktrees; extract/move uncommitted changes from one worktree to another (e.g. to split an oversized PR); start, stop, or inspect per-worktree dev jobs (services + tasks); or check out a GitHub pull request into a worktree — even when they don't explicitly say "wtm". Always pass --output json on wtm data commands so you can parse results; never invoke wtm through an interactive picker.
---

# Using wtm

wtm is a CLI that manages **git worktrees**, **per-worktree dev jobs** (long-running services + one-shot tasks, via a background daemon), and **GitHub pull requests**. It's designed to be driven by LLMs: every data command accepts `--output json` and prints machine-parseable results to stdout, while human messages stay on stderr.

## How to drive wtm from an LLM

1. **Always pass arguments.** wtm commands without an argument often drop into an interactive picker, which you can't navigate. Pick the branch, PR number, profile, or service name from a prior `list` call first.
2. **Always add `--output json`** on data commands. The JSON is pretty-printed on stdout with `snake_case` fields. Human text and warnings stay on stderr; ignore stderr unless exit code is non-zero.
3. **Trust exit codes.** `0` on success, non-zero on failure. Beyond the generic `1`, wtm returns **granular codes** (see the Exit codes table) so you can branch precisely. Error detail is plain text on stderr — surface it to the user on failure.
4. **Skip confirmations with the right flag.** `clean` needs `--force` (bypasses safety checks); `run up` uses `--exclusive`. JSON mode is always non-interactive.
5. **Operations are idempotent — safe to retry.** `create --if-not-exists` no-ops if the worktree exists; `clean` no-ops if it's already gone; `run up`/`run down`/`run stop` are safe to re-run.
6. **Every command supports `--help`.** Run `wtm <cmd> --help` if you need flags beyond the reference below.

## Exit codes

| Code | Meaning |
|---|---|
| `0` | success |
| `1` | generic error |
| `2` | bad usage / invalid flags |
| `10` | worktree (or its path) already exists |
| `11` | branch not found |
| `12` | config not found — repo not initialized (`wtm init`) |
| `14` | service/job not declared in `run.toml` |
| `15` | `extract`: selected changes conflict with the target worktree |

## Discovery first — know before you act

Run these before taking action so you have names to pass as arguments:

| Goal | Command |
|---|---|
| All worktrees (branch, path, PR, services, dirty?) | `wtm list --output json` |
| All open PRs (number, title, branch, state, draft, url) | `gh pr list --json number,title,headRefName,state,isDraft,url` |
| Declared jobs + profiles | `wtm run list --output json` |
| Jobs running right now (name, kind, status, pid, workdir) | `wtm run ps --output json` |
| Resolved project config (TOML on stdout) | `wtm config show` |

## Worktree commands (`wtm`)

- **`wtm list --output json`** — inventory of worktrees.
- **`wtm create <branch> --from <base> --output json`** — create a new worktree, run env provisioning + `on_create` hooks. Optional: `--env-from example|main|parent` to override the env strategy. Add **`--if-not-exists`** to make it idempotent (succeeds with `"already_exists": true` instead of exit `10` when the worktree is already there).
- **`wtm clean <branch> --force --output json`** — remove worktree and local branch (remote untouched). `--force` is mandatory in JSON mode. Idempotent: cleaning an absent worktree succeeds with `"already_absent": true`.
- **`wtm extract --files <a,b> --to <branch> [--from <base>] [--keep] [--on-conflict abort|resolve] --output json`** — move a subset of the **current worktree's uncommitted changes** to another worktree. Ideal when a change has grown too large or unrelated for one PR: split part of it onto a sibling branch for easier review and parallel work. `--files` is a comma-separated list of paths (from `git status` / `list`); `--to` is the target branch — an existing worktree, or one created on the fly from `--from` (defaults to the source's parent branch). Default is a **move** (files removed from the source once they land); `--keep` copies instead. JSON returns `files[]` (each `{path, status}`), `source_branch`, `target_branch`, `kept`, and `conflicts[]`.
  - **Unified safety rule:** the source is cleaned only when the **whole** extraction applies cleanly. If anything conflicts, the source is left **fully intact and recoverable** — there is never a half-moved state.
  - **On conflict** (a selected file was also changed in the target): default **`--on-conflict abort`** changes nothing and exits **`15`**, naming the conflicting files on stderr. **`--on-conflict resolve`** instead applies the changes into the target with git **conflict markers** (`<<<<<<<`) on the conflicting files — like a rebase — keeps the source intact, exits `0`, and lists the marked files in `conflicts[]`. The user then resolves the markers in the target and discards the same files in the source; or discards in the target to undo.
  - No uncommitted changes → exits `0` with empty `files[]`. Untracked-file name collisions in the target cannot be merged → always abort (exit `15`).
- **`wtm sync --output json`** — rebase **every** worktree onto its recorded parent (`source_branch`) in cascade. It fetches + fast-forwards the base branch, then for each branch (parents before children): fetches + fast-forwards that branch from its own `origin/<branch>` (so commits merged into the remote elsewhere are pulled in), then rebases it onto its refreshed parent. A chain `main → feat → dev1/dev2` is updated in one shot. The cascade is **local** — pushing is separate. Per-branch `status` in the JSON: `synced` (rebased), `up_to_date`, `skipped_dirty` (uncommitted changes — its descendants become `skipped_ancestor`), `skipped_ancestor`, `diverged` (local **and** `origin/<branch>` carry commits the other lacks, by patch — a genuine conflict left untouched for manual reconcile; descendants skipped. A branch rebased locally in a prior run but not yet pushed is **not** diverged — its remote commits are already integrated, so it comes back as `up_to_date` and pushable), `conflict` (rebase auto-aborted, working tree left clean — needs manual resolution; descendants skipped), `error`, `unknown_parent` (no recorded parent). Each step also reports `old_tip`, `new_tip`, `onto_tip`, `commits_replayed`, `push_pending` (the branch is ahead of `origin/<branch>` and eligible for force-push — true for `synced` **and** for `up_to_date` branches still unpushed from an earlier run), and `pushed`. Exits non-zero if any `conflict`/`error`. Flags: `--dry-run` (preview only — stays fully offline, no fetch/rebase/push), `--base <branch>` (override base), `--push` (force-push every `push_pending` branch with `--force-with-lease` — **the only way to push in `--output json`/non-interactive mode**, since the grouped push confirmation can't prompt), `--no-push` (never push), `-y`/`--yes` (skip the pre-sync confirmation in text mode). Without `--push`/`--no-push` in JSON mode, nothing is pushed.
- **`wtm relocate --output json`** — reconcile every worktree with the configured `base_path`: worktrees not under it are **moved** (`git worktree move`), and worktrees created outside wtm are **adopted** (a `meta.json` recording their parent is written so `sync` and `.env` sync work for them). Pass **`--to <path>`** to change `base_path` to a new directory and move existing worktrees there — the config `base_path` is rewritten afterwards (`--to` must be a non-empty path **relative** to the repo root; an absolute or blank value is rejected before anything runs). Per-worktree `status` in the JSON: `moved`, `moved_adopted` (moved **and** adopted), `adopted` (already in place, metadata written), `noop` (managed + already in place), `skipped_dirty` / `skipped_locked` (not moved — re-run with `--force`), `blocked_dest` (target path already occupied — never overwritten), `error`. The result also reports `base_path` and `base_path_updated`. Exits non-zero if any `error`/`blocked_dest`. Adopted worktrees get `base_branch` as their recorded parent in non-interactive/JSON mode (the per-worktree parent picker only runs in interactive text mode). Flags: `--to <path>` (new base_path + move), `--dry-run` (preview only — no move/adopt/config write), `--force` (move dirty or locked worktrees too), `-y`/`--yes` (skip the confirmation + parent prompts in text mode).
- **`wtm go <branch>`** and **`wtm switch <branch>`** — navigate to a worktree (and start services, for `switch`). These **require the user's shell integration** to `cd`, so an LLM can't drive them directly. Prefer `list` + tell the user which branch to run `switch` on.

## Run commands (`wtm run`)

Jobs are declared in a per-clone `run.toml` (managed by wtm — agents never touch it directly) and executed by a background daemon. Each job has a `kind`:

- **`kind = "service"`** — long-running. With a `stop` command, it's a detached launcher (e.g. `docker compose up -d`); otherwise it's tracked by PID and killed via SIGTERM.
- **`kind = "task"`** — one-shot script. Blocks the profile, streams output live, removed after exit. A non-zero exit aborts the profile.

Profiles are named groups of jobs (run in declared order). The same TOML can host multiple profiles (e.g. `dev`, `test`, `staging`).

- **`wtm run list --output json`** — config introspection (jobs + profiles declared).
- **`wtm run ps --output json`** — runtime state (jobs running right now, with kind).
- **`wtm run up [profile] --output json`** — execute a profile in order. No arg → default profile. Flags: `--exclusive` (stop jobs on other worktrees first), `--parallel` (don't stop anything), `-d`/`--detach` (start and return immediately without tailing — implied in `--output json`/piped mode, where services always stay detached). A failing job (task **or** service) aborts the rest of the profile and exits non-zero, leaving already-started services running and emitting a partial-state report. Re-running `run up` while services are already up is a benign no-op, not an abort.
- **`wtm run down [profile] --output json`** — stop jobs in the **current worktree** (or a specific profile). Other worktrees are never touched. Add `--all` to stop jobs across every worktree.
- **`wtm run start <job> --output json`** — start one job. Tasks block until they exit (their output is captured in the JSON result); services launch in the background. In text mode `start`/`up` now tail foreground output, but `--output json`/piped mode stays detached — add `-d`/`--detach` if you want the same detach behavior in text mode.
- **`wtm run stop <job> --output json`** — stop one job.
- **`wtm run logs [job]`** — attach to a job's PTY. No `--output json`: it's a raw text stream (already machine-readable).
- **`wtm run export [--profile <name>]`** — emit the run config as JSON on stdout. Use `--profile` to export only one profile and its jobs. Pipe to a file: `wtm run export > layout.json`.
- **`wtm run import [file|-] [--replace --force] [--output json]`** — ingest a JSON run config. Omit the file or pass `-` to read from stdin. Default: append new jobs/profiles, skip duplicates (prints what was added/skipped). `--replace --force` overwrites the file entirely.
- **`wtm run job add <name> --cmd "..." [--kind service|task] [--stop "..."] [--cwd ...] [--output json]`** — append a job to `run.toml`. Pass all required flags for non-interactive use; otherwise drops into a wizard. `--kind` defaults to `service`.
- **`wtm run job rm <name> [--force] [--output json]`** — remove a job. Without `<name>` runs an interactive picker (do not invoke from an agent in that form). With a name, errors if any profile references the job; `--force` strips those references too.
- **`wtm run job edit [name]`** — pre-filled wizard over an existing job. Always interactive — **never invoke from an agent**. Use `wtm run export` to read the current state, then propose changes (or use `wtm run job rm <name> --force` followed by `wtm run job add` with the new flags).
- **`wtm run profile add <name> --jobs job1,job2 [--default] [--output json]`** — append a profile referencing existing jobs.
- **`wtm run profile rm <name> [--output json]`** — remove a profile (jobs are untouched). Without `<name>` runs an interactive picker.
- **`wtm run profile edit [name]`** — pre-filled wizard. Same caveat as `run job edit`.
- **`wtm run job list --output json`** — emits the jobs slice; `wtm run job list` without `--output json` is a TTY picker (do not invoke without `--output json` from an agent).
- **`wtm run profile list --output json`** — emits the profiles slice; same caveat as `wtm run job list` without the JSON flag.
- **Default profile auto-override** — `wtm run profile add <name> --jobs ... --default` (or the wizard "Default? yes") automatically unsets any previous default. No more "two defaults" rejection at save.

## Config commands (`wtm config`)

- **`wtm config show`** — print the resolved project `config.toml` and its on-disk path. Use it to inspect `worktrees.base_path`, `env.strategy`, `hooks.on_create`, etc., before suggesting changes. Output is plain TOML on stdout.
- **`wtm config edit`** — opens `$EDITOR` on the config file. Interactive — **never invoke from an agent**. If the user wants to change a setting, run `wtm config show` to read the current state, then ask the user to run `wtm config edit` (or do the edit through a `Write`/`Edit` on the printed path if you have those tools and the user authorized the change).
- **`wtm init --only <section> --non-interactive --yes [flags]`** — regenerate a single config section after init, without an editor (agent-drivable). Sections: `worktrees` (rewrites `base_branch`, via `--base-branch`), `env` (via `--env-strategy`; copy_files re-detected), `hooks` (via `--install-command`; rebuilt from detection — for arbitrary `on_create` entries edit the file directly), `services` (re-detects docker/scripts; **run.toml jobs regenerated, profiles preserved**). Combine sections with `--only env,services`. Untouched config sections keep their current values. Without `--non-interactive` it opens a pre-filled wizard (don't invoke that form from an agent).

## Pull request checkout (`wtm checkout`)

Backed by the `gh` CLI — the user must have `gh auth login` set up.

- **`wtm checkout <number> --output json`** — fetch the PR's branch and create a worktree for it. The worktree content is the PR head; the recorded parent (rebase target for `wtm sync`) defaults to the PR's base branch. Optional: `--from <branch>` (override the parent), `--env-from`, `--mine`/`--review` (filter the interactive picker). Fork PRs are out of scope by design — fall back to `gh pr checkout <number>`.
- **Always pass the PR number** — without it the command opens an interactive picker you can't drive. To list open PRs, use `gh pr list --json number,title,headRefName`.
- **Creating a PR is out of scope** — use `gh pr create` directly (it already handles templates, branch push, and base detection).

## Conventions and invariants

- **Stdout** is structured (JSON when requested); **stderr** is for humans.
- **JSON payloads mirror Go structs** — field names are snake_case, the shape is stable. Run a command with `--output json` once to learn the exact schema; don't over-rely on any field being present besides the ones shown in this skill.
- **Operations are idempotent when asked.** `create --if-not-exists` no-ops on an existing worktree (`already_exists: true`); `clean` no-ops on an absent one (`already_absent: true`); `run up`/`run down`/`run stop` are safe to re-run. Without `--if-not-exists`, a plain `create` on an existing path still fails with exit `10`.
- **Never invent names.** Branch names, service names, profile names, and PR numbers must come from a preceding `list` call, from the user's message, or from repo state (`git branch`, etc.).

## Failure handling

On non-zero exit, read stderr. Common cases:

- exit `12` (`config not found`) → the repo isn't initialized. Run `wtm init --non-interactive` with flags (see below), or tell the user to run the interactive `wtm init`.
- exit `11` (`branch not found`) / `worktree not found` → the name is wrong; re-list.
- exit `14` (`job not found`) → the job isn't declared in `run.toml`; check `run list`.
- exit `15` (`extract` conflict) → the selected changes clash with the target worktree; nothing was changed. Retry with `--on-conflict resolve` to apply conflict markers for the user to resolve, or pick another `--to` target.
- `gh: …` → the `gh` CLI isn't authenticated; tell the user to run `gh auth login`.
- A `run up`/`run start` job failed → in `--output json` mode the failing task's captured output is embedded in the error entry, so read it to surface *why* it failed. Already-started services stay up (fix-and-retry loop); re-run `run up` once fixed.
- A `sync` exited non-zero → at least one branch has `status: conflict` or `error` in the JSON. For `conflict`, the rebase was aborted (working tree is clean), so the branch and its descendants were skipped — tell the user to resolve that branch manually (`git rebase <parent>` in its worktree), then re-run `wtm sync`. A `diverged` status (exit stays 0) means local and `origin/<branch>` each have commits the other lacks (by patch) — the user must reconcile that branch (`git pull --rebase` or a manual merge) before re-running. Re-running `sync` after a local-only rebase is safe: branches already rebased come back `up_to_date` with `push_pending: true` (push them with `--push`), not `diverged`.

## Escalate to the user when

- A command requires shell integration (`go`, `switch` without a shell wrapper installed).
- A destructive action (`clean`) wasn't explicitly authorized by the user — do **not** add `--force` on your own initiative.
- `wtm init` is needed — you can bootstrap it non-interactively: `wtm init --non-interactive [--shell zsh] [--base-path ../.trees] [--base-branch main] [--env-strategy example|main|parent] [--install-command "..."] [--skip-env] [--skip-hooks] [--skip-services]`. Unset values fall back to auto-detection then sensible defaults; the `--skip-*` flags leave a section out (written commented). `--non-interactive` fails fast if the base branch can't be detected. To reconfigure a section *after* init, use `wtm init --only <section>` (see Config commands). Only escalate to the user if you can't supply a required value.
- `wtm config edit` would be the natural answer — it opens `$EDITOR`. Either ask the user to run it themselves, or read with `wtm config show` and write the change directly to the path if you have a file-edit tool.
