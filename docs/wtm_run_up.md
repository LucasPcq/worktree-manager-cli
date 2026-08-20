## wtm run up

Start a profile's jobs

### Synopsis

Start every job in a profile, in declared order.
Without arguments, uses the default profile (or shows a picker if multiple exist).
Tasks block the profile and abort it on failure; services launch in the background.
Opens the run view on the profile's jobs; -d starts them and gives the terminal back.

```
wtm run up [profile] [flags]
```

### Options

```
  -d, --detach          Start the jobs and return instead of opening the run view
      --exclusive       Stop jobs on other worktrees before starting
  -h, --help            help for up
      --output string   Output format: text or json (default "text")
      --parallel        Start without stopping other worktrees
```

### SEE ALSO

* [wtm run](wtm_run.md)	 - Manage dev jobs (services + tasks)

