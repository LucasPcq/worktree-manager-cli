## wtm run init

Configure the run module (services & tasks) for this repo

### Synopsis

Set up run.toml by detecting docker-compose files and package.json scripts and turning
the selected ones into jobs.

In a TTY, opens a wizard to pick which ones to include; non-interactively (or piped),
auto-generates from detection. Re-running merges new selections into the existing
run.toml without overwriting what's already there.

`wtm run` is experimental — the workflow is still stabilizing and commands may change.

```
wtm run init [flags]
```

### Options

```
  -h, --help              help for init
      --non-interactive   Auto-generate from detection; never prompt
```

### SEE ALSO

* [wtm run](wtm_run.md)	 - Manage dev jobs (services + tasks)

