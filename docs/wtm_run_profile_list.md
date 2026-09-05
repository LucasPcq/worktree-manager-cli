## wtm run profile list

List profiles from run.toml

### Synopsis

List profiles declared in <git-common-dir>/wtm/run.toml.

In a TTY, opens an interactive picker. Selecting a profile offers Edit or Remove.
Use --output json, --yes (or pipe stdout) for a non-interactive listing.

```
wtm run profile list [flags]
```

### Options

```
  -h, --help            help for list
      --output string   Output format: text or json (default "text")
  -y, --yes             Skip the picker; print the table instead
```

### SEE ALSO

* [wtm run profile](wtm_run_profile.md)	 - Add, remove, or edit profiles in run.toml

