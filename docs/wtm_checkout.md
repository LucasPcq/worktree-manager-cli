## wtm checkout

Create a worktree from an existing pull request

### Synopsis

Create a worktree from a pull request.
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
```

### SEE ALSO

* [wtm](wtm.md)	 - Orchestrate git worktrees and team dev workflows from the terminal

