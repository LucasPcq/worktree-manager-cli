---
name: using-wtm
description: Use this skill whenever the user wants to create, list, switch, or clean git worktrees; extract/move uncommitted changes from one worktree to another (e.g. to split an oversized PR); start, stop, or inspect per-worktree dev jobs (services + tasks); or check out a GitHub pull request into a worktree — even when they don't explicitly say "wtm". Always pass --output json on wtm data commands so you can parse results; never invoke wtm through an interactive picker, and never launch the `wtm ui` dashboard.
---

# Using wtm

wtm manages **git worktrees**, **per-worktree dev jobs** (long-running services +
one-shot tasks, via a background daemon), and **GitHub pull requests**. It's built to
be driven by an LLM: every data command takes `--output json` and prints machine-parseable
results to stdout, while human messages stay on stderr.

This skill is the **driving guide** — the rules and vocabulary you can't get from a
single `--help`. It is deliberately not an exhaustive flag reference, because wtm is
self-documenting:

- **`wtm <cmd> --help`** — the full, always-current flags for any command.
- **`wtm <cmd> --output json`** — run it once to learn a command's exact JSON schema.
  The payload mirrors wtm's Go structs (stable `snake_case` fields); trust what you see
  over any field list you remember.

## Driving rules

1. **Always pass arguments.** Without one, most commands drop into an interactive picker
   you can't navigate. Get the branch / PR number / profile / job name from a prior
   discovery call (below) first.
2. **Never launch `wtm ui`.** It is a full-screen alt-screen dashboard meant for a human
   at a keyboard: it takes over the terminal until someone presses `q`, and there is no way
   for you to read it or get out of it. Treat it exactly like an interactive picker — do not
   run it to "look at" the worktrees; run `wtm list --output json` (or `wtm tree`) instead.
   Suggest `wtm ui` to the *user* when they want to browse worktrees themselves. wtm defends
   itself here (it errors on `--output json` and with no TTY), but don't rely on that.
3. **Always add `--output json`** on data commands. JSON goes to stdout; human text and
   warnings go to stderr — ignore stderr unless the exit code is non-zero.
4. **Trust exit codes.** `0` = success. Beyond generic `1`, wtm returns granular codes
   (table below) so you can branch precisely. On failure, surface the stderr text.
5. **JSON mode requires `--yes` on every mutating command.** JSON is non-interactive, so any
   command that changes state (`create`, `clean`, `prune`, `sync`, `relocate`, `reparent`,
   `extract`, `checkout`) needs `--yes` — it errors otherwise. Two orthogonal axes: **`--yes`**
   resolves confirmations/decisions from flags and safe defaults; **`--force`** only lifts
   safety refusals and does *not* imply `--yes` (`--force` alone is rejected in JSON). Required
   selections must still be passed explicitly: `clean` needs a branch (add `--force` to also
   remove unsafe worktrees); `sync` needs branch args or `--all` (`--yes` won't push — pass
   `--push` — and won't fast-forward parents — pass `--ff-parents`); `extract` needs the source arg, `--files`, and `--to` (`--yes` defaults
   on-conflict to abort — pass `--on-conflict resolve`); `reparent` needs worktrees and `--to`;
   `checkout` needs the PR `<number>`; `relocate` uses `--to` to change base_path; `create`
   needs `--from` when the branch already exists locally (its parent can't be inferred).
   Read-only data commands (`list`, `tree`, `resolve`, `config show`, `run list`/`ps`) take
   `--output json` with no `--yes`. Check `--help` when unsure.
6. **Operations are idempotent — safe to retry.** `create --if-not-exists` no-ops on an
   existing worktree (including one holding the branch outside `base_path`, whose path it
   returns — even the **main** worktree's own path, if that's where the branch is checked
   out; it is still `already_exists: true`, just not a directory under `base_path`); `clean`
   no-ops on an absent one; `run up`/`down`/`stop` re-run cleanly. Note the flag is about
   the **worktree**, not the branch: an existing branch with no worktree is still created.
7. **An existing local branch is not an obstacle.** `create <branch>` and `checkout <number>`
   both check out a same-named local branch as-is, keeping commits that were never pushed —
   they never delete or reset it, so there is no `wtm clean` to run first. The response sets
   `existing_branch: true` and `origin_state`. Only a branch **another worktree already holds**
   is refused (exit `10`) — enter it with `wtm go <branch>` instead. What wtm will not do is
   **guess its parent**: `create` requires `--from` there, while `checkout` takes the PR's
   base branch (a fact, not a guess).

## Exit codes

| Code | Meaning |
|---|---|
| `0` | success |
| `1` | generic error |
| `2` | bad usage / invalid flags |
| `10` | worktree (or its path) already exists — or the branch is checked out in another worktree |
| `11` | branch not found |
| `12` | config not found — repo not initialized (`wtm init`) |
| `14` | service/job not declared in `run.toml` |
| `15` | `extract`: selected changes conflict with the target worktree |
| `16` | run module not initialized — run `wtm run init` first |

## Discovery first — get names before you act

| Goal | Command |
|---|---|
| All worktrees (branch, path, PR, services, dirty?) | `wtm list --output json` |
| Worktree **forest** (parent→child + which need sync) | `wtm tree --output json` |
| Open PRs | `gh pr list --json number,title,headRefName,state,isDraft,url` |
| Declared jobs + profiles | `wtm run list --output json` |
| Jobs running right now | `wtm run ps --output json` |
| Resolved project config | `wtm config show --output json` |
| A branch's worktree path | `wtm resolve <branch> --output json` |

## Command map

Reach for the right command, then read its `--help` for flags. Non-obvious behavior is
flagged; everything else is what the name implies.

**Worktrees**
- `wtm list` — flat inventory. `wtm tree` — the parent→child forest; use it (not `list`)
  when hierarchy/orchestration order matters. Its `needs_sync` flag is the key signal
  that a node's parent moved past it. Both expose an `origin` object
  (`{ahead, behind, state}`, or `null` when the branch has no origin counterpart)
  describing divergence from `origin/<branch>` — distinct from `commits_ahead`, which
  counts commits vs the **parent/base** branch. `state` is `up-to-date`/`behind`/`ahead`/`diverged`.
- `wtm create <branch> --from <base>` — new worktree + env provisioning + `on_create`
  hooks. `--from` accepts a remote ref (`origin/x`). Add `--if-not-exists` for idempotency.
  Add `--ff` to fast-forward a behind-only `--from` branch to origin first (so the worktree
  starts up to date); a diverged branch is left as-is (no prompt in JSON mode). `extract`
  accepts the same `--ff` for the parent branch of a newly-created target.
  **When `<branch>` already exists locally** it is checked out as-is, and `--from` stops
  being a start-point: it names the **parent to record for `wtm sync`**. That parent
  cannot be inferred (the branch was created outside wtm), so **`--from` is required
  there** — the command errors instead of guessing the base branch, because `sync` and
  `tree` would treat the guess as fact. Ask the user which branch it stacks on if you
  don't know. `--ff` switches subject too, updating `<branch>` itself rather than the
  source. The response adds `existing_branch: true` and `origin_state`
  (`up-to-date`/`behind`/`ahead`/`diverged`) so you can tell reuse from creation.
- `wtm clean <branch>` / `wtm prune [filters]` — remove one / batch-remove finished
  worktrees. **In JSON mode surviving children are left orphaned unless you pass
  `--reparent-children`** (they reparent onto the grandparent). `prune` decides "finished"
  from **GitHub PR state via the `gh` CLI** (not local commits): `--merged` = PR merged,
  `--closed` = PR closed without merging, `--gone` = remote branch deleted; no filter = all
  three. **`--merged`/`--closed` need `gh` installed + authenticated** — without it they match
  nothing and prune prints a notice on stderr; only `--gone` works offline. Prune `reason`
  values in JSON are `pr_merged` / `pr_closed` / `gone` (there is no plain `merged`). Both
  **refuse unsafe worktrees** — dirty, unpushed commits, or an open PR — unless you pass
  `--force`; in JSON/`--yes` mode those are reported under `skipped` (reason `dirty`/
  `unpushed`/`open_pr`) instead of being removed, so committed work is never silently lost.
  One exception for `prune`: when **every** match is unsafe, nothing survives the
  selection and the JSON is empty — `pruned: []` and `skipped: []` both. Read an empty
  result as "nothing was removed", not as "nothing matched".
  Both also run any configured **`on_clean`** hooks in the worktree just before removing it
  (e.g. `docker compose down`); a hook that exits non-zero **aborts the removal** unless its
  entry sets `continue_on_error`. For `prune`, every selected worktree is hooked before the
  first one is removed, so a hook failing partway aborts with nothing deleted — but the
  worktrees already hooked have had their teardown run, which is why `on_clean` hooks must
  be idempotent. If `git worktree remove` then fails on undeletable files
  (e.g. root-owned Docker files), interactive runs offer a `sudo rm -rf` fallback — this
  never triggers in JSON/`--yes` mode, where the failure is surfaced as an error.
- `wtm extract <source> --files <a,b> --to <branch>` — move part of the `<source>`
  worktree's uncommitted changes onto another branch (split an oversized PR). In JSON mode pass
  `--yes` plus the source arg, `--files`, and `--to` — all **required** (omitting any errors
  naming the missing flag; there is no picker). On conflict it changes nothing and exits `15`;
  retry with `--on-conflict resolve` to apply git conflict markers (`--yes` defaults on-conflict
  to abort). **When `--to` names a branch that already exists locally**, the same rule as
  `create` applies: its parent can't be inferred, so `--from` is **required** there too
  (the command errors naming the flag rather than guessing).
  `--files` takes paths exactly as reported (spaces and non-ASCII are never quoted or escaped),
  including each untracked file of a brand-new directory individually; pass a directory
  (`--files newmod/`) to take everything below it. Gitignored files are never listed — `.env`
  drift is `wtm env`'s job. A file entry may report `"status": "renamed"` with an extra
  `"orig_path"`: both paths move together, and `--files` names the new one.
- `wtm env [worktree]` — detect and fix a worktree's `.env` drift: reconcile it against its
  committed **template** (the expected keys, read from the worktree itself) plus a single
  **value source** chosen by the worktree's recorded strategy — never a silent mix:
  `example` → template placeholders only; `main` → the main worktree; `parent` → the parent
  worktree **only** (a key the parent lacks stays `missing_unresolved`, it is NOT pulled from
  main). The one exception (mirroring `wtm create`): when there is no readable parent file at
  all — the parent has no worktree, or that file isn't in it — it falls back to main for that
  file, flagged `parent_fallback:true` (with `parent_branch` naming the parent). The
  report/JSON `source` field names the source. `--from example|main|parent` is the only way to
  pull a different source for a run. If there is no `.env` to sync from at all yet (fresh
  project: templates detected by `init`, but no value files created), every expected key is
  `missing_unresolved` and `source` reads `template (no .env to sync from)` — filling them
  (interactively) scaffolds the `.env` from the template. `--mode add` (default)
  only fills missing keys and never touches an existing value; `--mode refresh` also settles
  values that diverge from the source. `--check` is a read-only drift report (writes nothing).
  In JSON/`--yes` mode it is **report-only except safe additions**: keys missing from the child
  but resolved from a real source are added; **conflicts** stay unless you pass
  `--on-conflict overwrite` (default `keep`), **orphans** stay unless you pass `--prune`, and
  keys with no real source value (`missing_unresolved`) are never auto-filled (they need the
  interactive prompt). JSON requires `--yes` **except with `--check`** (read-only, never
  prompts, so `wtm env <wt> --check --output json` works on its own); omit a worktree arg only
  interactively (else it errors — there is no picker under `--yes`/JSON). JSON shape:
  `{branch,mode,check,files:[{target,strategy,source,applied,parent_branch,parent_fallback,diff:{mode,
  entries:[{key,status,current_value,resolved_value,placeholder,source,export}]}}]}` where
  `status` is `resolved` / `missing_unresolved` / `conflict` / `orphan`. Round-trip is
  preserved: comments, ordering and formatting of the `.env` are kept; only decided keys change.
- `wtm ui` — the full-screen dashboard (worktree state, divergence, PRs, plus create and
  delete). **Never invoke it** (see driving rule 2): it holds the terminal until a human
  quits it. Everything it shows is available to you as JSON via `wtm list` / `wtm tree`, and
  everything it does via `wtm create` / `wtm clean`.
- `wtm relocate` — realign worktrees with `base_path` and adopt externally-created ones.
  `--to <path>` sets a new `base_path` non-interactively; the interactive wizard also lets
  the user change it. You can't drive the wizard — in JSON mode pass `--yes` (and `--to` to
  change base_path).

**Stacked branches**
- `wtm sync <branch…>` / `wtm sync --all` — rebase the selected worktrees onto their
  recorded parent, in cascade (parents before children), fetching first. A conflict aborts
  that branch's rebase (its descendants are skipped) unless `--keep-conflict` leaves it in
  progress. Local only — in JSON mode pass `--yes` (and `--push` to force-push with lease).
  A parent **no step covers** — a branch with no worktree, or one left out of the
  selection — is not refreshed by the cascade. Every run reports what it found about
  those parents in `parent_updates`: `{branch, status, old_tip, new_tip, behind,
  children, detail}`, where `behind` counts the commits the local ref lacks and
  `children` names the worktrees rebased onto it. `status` is one of `behind` (left
  as is — re-run with `--ff-parents` to refresh it), `fast_forwarded`, `diverged`
  (no fast-forward exists; reconcile it by hand, the flag will never move it), or
  `ff_failed` (the refresh was asked for and could not happen — `detail` says why,
  e.g. a dirty parent worktree; passing the flag again will not help). Pass
  `--ff-parents` to refresh them first, `--no-ff-parents` to never. `--dry-run`
  reports them without any network call and never refreshes, so `--ff-parents` is a
  no-op there.
  The **base is only involved when it is actually a target**: a step rebases onto it,
  the selection names it, or the run covers everything (`--all`, which includes the
  root). Otherwise it is neither fetched nor fast-forwarded and `base_targeted` is
  `false` — an explicit selection whose every worktree hangs off another parent
  leaves the base completely alone.
- `wtm reparent <branch…> --to <parent>` — change the recorded parent of one or more
  worktrees to the same new parent (metadata only; the rebase happens on the next `sync`).
  Use after a middle branch merges. In JSON mode pass `--yes` with the worktrees and `--to`.
  JSON: `{"reparented":[{branch,old_parent,new_parent},…]}`.

**Navigate**
- `wtm go` / `wtm switch` need the user's **shell integration** to `cd`, so you can't drive
  them. To get a path yourself, use `wtm resolve <branch> --output json` → `{path, branch}`.

**Dev jobs (`wtm run`)** — jobs live in a per-clone `run.toml` (wtm-managed; never edit it
directly). Each is a `service` (long-running) or `task` (one-shot, blocks the profile,
non-zero exit aborts it); profiles are named, ordered job groups. The module is **opt-in**
and **experimental**: the global `wtm init` does not configure it.
- `run init` sets up `run.toml` from detection (docker-compose + package scripts). It is
  the only entry point that works before the module exists; every other run command exits
  `16` (run module not initialized) until at least one job/profile is declared. Non-TTY it
  auto-generates; re-running merges without overwriting. `run job add` / `run profile add`
  also work before init (they create the first job).
- `run up [profile]` / `run down` — start / stop a profile. `run start <job>` / `run stop
  <job>` — one job. A failing job aborts the rest and exits non-zero, leaving started
  services up (fix and re-run).
- `run export` / `run import` — share a layout as JSON.
- `run job add|rm` and `run profile add|rm` are agent-drivable with flags; the `edit`
  wizards are **interactive — never invoke them**. To change a job, `run export` to read
  state, then `run job rm <name> --force` + `run job add` with new flags.

**GitHub**
- `wtm checkout <number>` — fetch a PR's branch into a worktree; parent defaults to the
  PR's base. In JSON mode pass `--yes` and the PR `<number>` (both **required**; no picker);
  `--yes` resolves the parent → PR base and env → config default. A local branch of the
  PR's name is **reused as-is**, never reset — the response then sets `existing_branch: true`
  and `origin_state`; under `--yes` no ref is touched even when the branch is behind, so
  read `origin_state` and decide yourself. Fork PRs are out of scope — fall back to
  `gh pr checkout <number>`. Creating a PR is out of scope too — use `gh pr create`.

**Setup**
- `wtm config show` inspects config; `wtm config edit` and the `wtm init` wizard are
  interactive. Bootstrap non-interactively with
  `wtm init --non-interactive [--base-branch … --env-strategy … --install-command … --clean-command …]`, and
  reconfigure one section later with `wtm init --only env|hooks|worktrees --non-interactive --yes`.
  Services are **not** part of `wtm init` — configure them with `wtm run init`.
  See their `--help` for the full flag set.

## Failure handling

On non-zero exit, read stderr, then:

- `12` (config not found) → repo not initialized. Run `wtm init --non-interactive` with
  flags, or ask the user to run interactive `wtm init`.
- `11` (branch not found) → wrong name; re-run the relevant discovery call.
- `10` (worktree already exists) → either the path is taken, or the branch is checked out
  in another worktree. Enter it with `wtm go <branch>` (get its path from `wtm resolve
  <branch>`), or pick a different branch name. `--if-not-exists` turns this into a success
  returning the existing worktree's path.
- `14` (job not found) → not declared in `run.toml`; check `wtm run list`.
- `15` (extract conflict) → nothing changed; retry with `--on-conflict resolve` or a
  different `--to`. Covers both a file modified on both sides and one that merely already
  exists in the target. Exception: an untracked **binary** file already in the target cannot
  take conflict markers — `resolve` won't help, pick a different `--to`.
- `16` (run module not initialized) → run `wtm run init` (or `wtm run init --non-interactive`)
  to create `run.toml`, then re-run the command.
- `gh: …` → `gh` isn't authenticated; tell the user to run `gh auth login`.
- A `run up`/`run start` job failed → its captured output is in the JSON error entry; read
  it to see why, fix, and re-run.
- A `sync` exited non-zero → some branch is `status: conflict`/`error` in the JSON. For a
  `conflict` (default mode) the rebase was aborted and the branch + descendants skipped —
  the user resolves it manually in its worktree, then re-run `sync`. `kept_in_progress:
  true` means the rebase is paused in that step's `path` for the user to finish with `git
  rebase --continue`. A `diverged` status (exit stays 0) needs the user to reconcile that
  branch before re-running.

## Escalate to the user when

- A command needs shell integration (`go`/`switch` with no wrapper installed).
- The user wants to *browse* their worktrees rather than get an answer from you — tell them
  to run `wtm ui` themselves; never run it for them.
- A destructive action (`clean`/`prune --force`) wasn't explicitly authorized — don't add
  `--force` on your own initiative.
- `wtm config edit` is the natural answer — ask the user to run it, or read with `wtm
  config show` and write the change to the printed path if you have a file-edit tool and
  the user authorized it.
- You can't supply a value that non-interactive `wtm init` requires.
