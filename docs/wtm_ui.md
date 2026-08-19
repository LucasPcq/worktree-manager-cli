## wtm ui

Open the worktree dashboard

### Synopsis

Open a full-screen dashboard of the repository's worktrees.
The Worktrees tab lists them with their git state against both the base branch and
origin, and their pull requests; the Tree tab lays the same worktrees out as the
parent-child forest `wtm tree` prints. `n` creates a worktree; right-click a row
(or press `m`) to reparent, sync, or delete it; `a` opens the actions that run over
several worktrees at once, including syncing them all. Local git state refreshes on
a short poll; pull requests load once and refresh only on `r`. Press `?` for the key
reference.

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

