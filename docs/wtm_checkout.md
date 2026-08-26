## wtm checkout

Create a worktree from an existing pull request

### Synopsis

Create a worktree from a pull request.
A local branch of the PR's name is checked out as-is, keeping commits you never
pushed; interactive runs offer to fast-forward it when it is behind origin.
Without arguments, shows an interactive picker of open PRs.

```
wtm checkout [number] [flags]
```

### Options

```
      --env-from string   Override env strategy (example, main, parent)
      --from string       Parent branch for sync (defaults to the PR base branch)
  -h, --help              help for checkout
      --mine              Show only your PRs
      --output string     Output format: text or json (default "text")
      --review            Show only PRs where you are requested as reviewer
  -y, --yes               Skip all prompts; resolve every decision from flags and safe defaults (PR number required)
```

### SEE ALSO

* [wtm](wtm.md)	 - Orchestrate git worktrees and team dev workflows from the terminal

