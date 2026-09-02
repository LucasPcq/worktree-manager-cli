## wtm run up

Start a profile's jobs

### Synopsis

Start every job in a profile, in declared order, in [worktree] — the current one when omitted, picked interactively when there is a terminal.
Without --profile, uses the default profile (or shows a picker if multiple exist).
Once the jobs are up, each declared port is checked: a port nothing answers on is
reported rather than announced as bound. It never fails the run — see --no-probe
and run.toml's port_probe_timeout.
Tasks block the profile and abort it on failure; services launch in the background.
The run view opens on the jobs as they start; leaving it detaches without stopping them, and -d skips it.

```
wtm run up [worktree] [flags]
```

### Options

```
  -d, --detach           Start the jobs and return immediately instead of opening their output
      --exclusive        Stop jobs on other worktrees before starting
  -h, --help             help for up
      --no-probe         Skip the check that each declared port was actually bound
      --output string    Output format: text or json (default "text")
      --parallel         Start without stopping other worktrees
      --profile string   Profile to start (defaults to the default profile, or a picker when several exist)
```

### SEE ALSO

* [wtm run](wtm_run.md)	 - Manage dev jobs (services + tasks)

