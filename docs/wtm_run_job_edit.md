## wtm run job edit

Edit an existing job via wizard

### Synopsis

Edit a job declared in <git-common-dir>/wtm/run.toml.

Without an argument, prompts to pick from the existing jobs. The wizard
is pre-filled with the current values; renaming is allowed and references
in profiles will be checked against the new name on save.

```
wtm run job edit [name] [flags]
```

### Options

```
  -h, --help            help for edit
      --output string   Output format: text or json (default "text")
```

### SEE ALSO

* [wtm run job](wtm_run_job.md)	 - Add, remove, or edit jobs in run.toml

