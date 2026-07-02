## wtm prune

Remove finished worktrees (merged, closed PR, gone, or old) in one pass

### Synopsis

Batch-remove worktrees whose work is done, reparenting any surviving children onto
their grandparent (like `clean --reparent-children`). Whether work is "done" is read
from GitHub via the `gh` CLI — never guessed from local commits — so squash- and
rebase-merges are detected correctly. By default prune considers every finished
worktree: merged PR, closed PR, or upstream branch gone. The reason flags restrict to
specific categories — --merged (PR merged), --closed (PR closed unmerged), --gone
(remote branch deleted).

--merged and --closed require the GitHub CLI (`gh`) to be installed and authenticated;
without it they match nothing and prune prints a notice — only --gone still applies.
gone-detection runs `git fetch --prune` first so deleted remote branches are seen
(pass --no-fetch to skip).

On a TTY, matches are shown for review (unsafe ones unchecked), then a prune
confirmation, then — like clean — a dedicated confirmation to reparent surviving
children onto their grandparent (or leave them orphaned). The main worktree and base
branch are always protected; the current worktree is removed and the shell
redirected to the base repo. Like clean, worktrees that are dirty, have unpushed
commits, or have an open PR are unsafe and need --force. Use --yes to skip the
prompts (required with --output json); non-interactively, children are left orphaned
unless --reparent-children is passed. --dry-run previews without changing anything.

```
wtm prune [flags]
```

### Options

```
      --closed              Restrict to worktrees whose PR was closed without merging (needs gh)
      --dry-run             Preview what would be pruned without removing anything
      --force               Also remove unsafe worktrees (dirty, unpushed commits, or open PR)
      --gone                Restrict to worktrees whose upstream branch was deleted on the remote
  -h, --help                help for prune
      --merged              Restrict to worktrees whose PR was merged on GitHub (needs gh)
      --no-fetch            Skip the git fetch --prune that gone-detection performs; use already-fetched state
      --output string       Output format: text or json (default "text")
      --reparent-children   Reparent orphaned child worktrees onto the grandparent (non-interactive; otherwise you're asked)
  -y, --yes                 Skip the confirmation/selection prompt (keeps every match)
```

### SEE ALSO

* [wtm](wtm.md)	 - Orchestrate git worktrees and team dev workflows from the terminal

