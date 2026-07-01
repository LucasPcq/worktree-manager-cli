## wtm prune

Remove finished worktrees (merged, closed PR, gone, or old) in one pass

### Synopsis

Batch-remove worktrees whose work is done, reparenting any surviving children onto
their grandparent (like `clean --reparent-children`). By default prune considers
every finished worktree: merged into the base, closed/merged PR, or upstream branch
gone. The reason flags restrict to specific categories — --merged (no commits ahead),
--closed (PR merged/closed, needs gh), --gone (remote branch deleted).

gone-detection runs `git fetch --prune` first so deleted remote branches are seen
(pass --no-fetch to skip). --merged does not catch squash-merges (the branch keeps
distinct commits); --gone or --closed do.

On a TTY, matches are shown for review (dirty ones unchecked), then a prune
confirmation, then — like clean — a dedicated confirmation to reparent surviving
children onto their grandparent (or leave them orphaned). The main worktree and base
branch are always protected; the current worktree is removed and the shell
redirected to the base repo. Dirty worktrees need --force. Use --yes to skip the
prompts (required with --output json); non-interactively, children are left orphaned
unless --reparent-children is passed. --dry-run previews without changing anything.

```
wtm prune [flags]
```

### Options

```
      --closed              Restrict to worktrees whose PR is merged or closed (needs gh)
      --dry-run             Preview what would be pruned without removing anything
      --force               Also remove dirty worktrees
      --gone                Restrict to worktrees whose upstream branch was deleted on the remote
  -h, --help                help for prune
      --merged              Restrict to worktrees whose branch is merged into the base (no commits ahead)
      --no-fetch            Skip the git fetch --prune that gone-detection performs; use already-fetched state
      --output string       Output format: text or json (default "text")
      --reparent-children   Reparent orphaned child worktrees onto the grandparent (non-interactive; otherwise you're asked)
  -y, --yes                 Skip the confirmation/selection prompt (keeps every match)
```

### SEE ALSO

* [wtm](wtm.md)	 - Orchestrate git worktrees and team dev workflows from the terminal

