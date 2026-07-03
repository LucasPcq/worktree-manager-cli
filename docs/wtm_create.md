## wtm create

Create a new worktree

### Synopsis

Create a git worktree with env provisioning, metadata, and hooks.
Without arguments, prompts for the branch name interactively.

```
wtm create [branch] [flags]
```

### Options

```
      --env-from string   Override env strategy (example, main, parent)
      --ff                Fast-forward the source branch to origin before creating (non-interactive; skipped when it has diverged)
      --from string       Source branch (skips interactive picker)
  -h, --help              help for create
      --if-not-exists     Succeed silently if the worktree already exists (idempotent)
      --output string     Output format: text or json (default "text")
  -y, --yes               Skip all prompts; resolve every decision from flags and safe defaults (branch name required; source defaults to the base branch)
```

### SEE ALSO

* [wtm](wtm.md)	 - Orchestrate git worktrees and team dev workflows from the terminal

