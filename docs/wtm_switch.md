## wtm switch

Navigate to a worktree and start its services

### Synopsis

Combines `go` and `svc up` in one command.
Requires shell integration to change your working directory.

```
wtm switch [branch] [flags]
```

### Options

```
      --exclusive        Stop services on other worktrees before starting
  -h, --help             help for switch
      --parallel         Start without stopping other worktrees
      --profile string   Service profile to start
```

### SEE ALSO

* [wtm](wtm.md)	 - Orchestrate git worktrees and team dev workflows from the terminal

