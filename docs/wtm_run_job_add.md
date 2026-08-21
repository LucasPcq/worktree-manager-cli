## wtm run job add

Add a job to run.toml

### Synopsis

Append a job to <git-common-dir>/wtm/run.toml.

Pass --cmd (and optionally --kind, --stop, --cwd, --port) for non-interactive use.
Without --cmd, prompts interactively for each field.

--cmd and --stop are /bin/sh lines: quotes, && and ${VAR} behave as in a terminal,
so a declared port can be passed as a flag — --cmd 'pnpm dev --port ${PORT}'.

```
wtm run job add [name] [flags]
```

### Options

```
      --cmd string         Command to run, as a /bin/sh line (skips wizard when set with name)
      --cwd string         Working directory (relative to project root)
  -h, --help               help for add
      --kind string        Job kind: service or task (default "service")
      --output string      Output format: text or json (default "text")
      --port stringArray   Base port as NAME=PORT, repeatable (e.g. --port PORT=3000)
      --stop string        Stop command, as a /bin/sh line (services only)
```

### SEE ALSO

* [wtm run job](wtm_run_job.md)	 - Add, remove, or edit jobs in run.toml

