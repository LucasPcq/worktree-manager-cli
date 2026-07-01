## wtm clean

Remove a worktree and its local branch

### Synopsis

Remove a git worktree and delete the local branch. The remote branch is never touched.
Without arguments, shows an interactive picker.

```
wtm clean [branch] [flags]
```

### Options

```
      --force               Bypass all safety checks
  -h, --help                help for clean
      --output string       Output format: text or json (default "text")
      --reparent-children   Reparent orphaned child worktrees onto the grandparent (no prompt)
  -y, --yes                 Skip the confirmation prompt but keep safety checks (refuses dirty/unpushed/open-PR worktrees)
```

### SEE ALSO

* [wtm](wtm.md)	 - Orchestrate git worktrees and team dev workflows from the terminal

