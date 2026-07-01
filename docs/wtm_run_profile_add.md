## wtm run profile add

Add a profile to run.toml

### Synopsis

Append a profile to <git-common-dir>/wtm/run.toml.

Pass --jobs (comma-separated existing job names) for non-interactive use.
Without --jobs, prompts interactively (multiselect over existing jobs).

```
wtm run profile add [name] [flags]
```

### Options

```
      --default         Mark this profile as the default
  -h, --help            help for add
      --jobs strings    Comma-separated existing job names (skips wizard when set with name)
      --output string   Output format: text or json (default "text")
```

### SEE ALSO

* [wtm run profile](wtm_run_profile.md)	 - Add, remove, or edit profiles in run.toml

