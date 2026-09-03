## wtm ui

Open the worktree dashboard

### Synopsis

Open a full-screen dashboard of the repository's worktrees.
The Worktrees tab lists them with their git state against both the base branch and
origin, and their pull requests; the Tree tab lays the same worktrees out as the
parent-child forest `wtm tree` prints; the Running tab gathers every worktree the
run daemon holds something up in, with the addresses its jobs answer on. `n`
creates a worktree; right-click a row (or press `m`) to reparent, sync, or delete
it; `a` opens the actions that run over several worktrees at once, syncing or
reparenting a selection of them; `L` reads a job's logs in the detail panel.
The list's local git state refreshes on a short poll; the detail panel reloads
when the selection changes or an operation touches it, and pull requests load
once — both refresh on demand with `r`.
Press `?` for the key reference.

```
wtm ui [flags]
```

### Options

```
  -h, --help            help for ui
      --output string   Output format: text or json (default "text")
```

### SEE ALSO

* [wtm](wtm.md)	 - Orchestrate git worktrees and team dev workflows from the terminal

