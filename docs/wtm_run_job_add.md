## wtm run job add

Add a job to run.toml

### Synopsis

Append a job to <git-common-dir>/wtm/run.toml.

Pass --cmd (and optionally --kind, --stop, --cwd) for non-interactive use.
Without --cmd, prompts interactively for each field.

```
wtm run job add [name] [flags]
```

### Options

```
      --cmd string      Command to run (skips wizard when set with name)
      --cwd string      Working directory (relative to project root)
  -h, --help            help for add
      --kind string     Job kind: service or task (default "service")
      --output string   Output format: text or json (default "text")
      --stop string     Stop command (services only)
```

### SEE ALSO

* [wtm run job](wtm_run_job.md)	 - Add, remove, or edit jobs in run.toml

