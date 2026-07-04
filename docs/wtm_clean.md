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
      --dry-run             Preview what would be removed without deleting anything (no confirmation, no --yes needed)
      --force               Lift safety refusals (dirty/unpushed/open-PR); still asks to confirm unless --yes
  -h, --help                help for clean
      --output string       Output format: text or json (default "text")
      --reparent-children   Reparent orphaned child worktrees onto the grandparent (no prompt)
  -y, --yes                 Skip all prompts and run non-interactively; resolve every decision from flags and safe defaults (requires a branch; keeps safety checks unless --force)
```

### SEE ALSO

* [wtm](wtm.md)	 - Orchestrate git worktrees and team dev workflows from the terminal

