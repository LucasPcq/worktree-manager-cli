## wtm ui

Open the worktree dashboard

### Synopsis

Open a full-screen dashboard of the repository's worktrees.
It lists the worktrees, their git state against both the base branch and origin,
and their pull requests, and it runs the create and clean flows in place. Local
git state refreshes on a short poll; pull requests load once and refresh only on
`r`. Press `?` for the key reference.

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

