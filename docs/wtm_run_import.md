## wtm run import

Import a JSON run config into run.toml

### Synopsis

Read a JSON run config payload from a file (or stdin) and merge it into run.toml.

Pass "-" or omit the argument to read from stdin.

By default, new jobs and profiles are appended; duplicates are skipped with a warning.
Use --replace --force to overwrite the file entirely.

```
wtm run import [file] [flags]
```

### Options

```
      --force           Confirm destructive --replace
  -h, --help            help for import
      --output string   Output format: text or json (default "text")
      --replace         Overwrite run.toml entirely (requires --force)
```

### SEE ALSO

* [wtm run](wtm_run.md)	 - Manage dev jobs (services + tasks)

