## wtm extract

Move uncommitted changes to another worktree

### Synopsis

Move a subset of a worktree's uncommitted changes to another worktree
(new or existing) to split an oversized PR or isolate unrelated work.

The source worktree is the first thing chosen: pass its branch as [source],
or omit it to pick interactively from the worktrees that have changes. A source
is required when there is no terminal or with --output json.

On conflict it aborts by default, leaving the source intact; --on-conflict resolve
applies conflict markers in the target so you can resolve them like a rebase.

```
wtm extract [source] [flags]
```

### Options

```
      --ff                   Fast-forward the parent branch to origin before creating the target (non-interactive; skipped when it has diverged)
      --files strings        Files to extract (skips interactive selection)
      --from string          Parent branch when creating the target worktree
  -h, --help                 help for extract
      --keep                 Copy instead of move (keep the changes in the source)
      --on-conflict string   On conflict: abort (default) or resolve (write conflict markers in the target)
      --output string        Output format: text or json (default "text")
      --to string            Target worktree branch; created if it does not exist
  -y, --yes                  Skip all prompts; resolve every decision from flags and safe defaults (requires a source arg, --files and --to; errors if a selection is missing)
```

### SEE ALSO

* [wtm](wtm.md)	 - Orchestrate git worktrees and team dev workflows from the terminal

