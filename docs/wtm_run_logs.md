## wtm run logs

Attach to a job's output

### Synopsis

Open the run view on [worktree]'s jobs — the current worktree when omitted, picked interactively when there is a terminal.
--job focuses one of them; without it, every job is shown.
Leaving the view detaches; the jobs keep running.
Without a terminal, every job's output is written as prefixed lines instead.

```
wtm run logs [worktree] [flags]
```

### Options

```
  -h, --help         help for logs
      --job string   Focus a single job instead of showing them all
```

### SEE ALSO

* [wtm run](wtm_run.md)	 - Manage dev jobs (services + tasks)

