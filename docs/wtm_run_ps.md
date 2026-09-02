## wtm run ps

List currently running jobs

### Synopsis

Show the jobs managed by the background daemon (name, kind, status, PID, uptime, worktree).
In a TTY, offers an interactive picker with stop/logs/restart actions.

```
wtm run ps [flags]
```

### Options

```
  -h, --help            help for ps
      --output string   Output format: text or json (default "text")
  -y, --yes             Skip the interactive picker; print the table instead
```

### SEE ALSO

* [wtm run](wtm_run.md)	 - Manage dev jobs (services + tasks)

