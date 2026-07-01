## wtm reparent

Change the parent a worktree is rebased onto

### Synopsis

Change the recorded parent (source branch) of a worktree. Only the metadata is
updated — the rebase happens on the next `wtm sync`. Pass the worktree and --to <parent>,
or run with no arguments to pick interactively. The new parent must exist as a local or
origin remote-tracking branch (origin/x), and the resulting parent chain must stay acyclic.

```
wtm reparent [branch] [flags]
```

### Options

```
  -h, --help            help for reparent
      --output string   Output format: text or json (default "text")
      --to string       New parent branch to rebase onto
```

### SEE ALSO

* [wtm](wtm.md)	 - Orchestrate git worktrees and team dev workflows from the terminal

