## wtm run profile edit

Edit an existing profile via wizard

### Synopsis

Edit a profile declared in <git-common-dir>/wtm/run.toml.

Without an argument, prompts to pick from the existing profiles. The wizard
is pre-filled with the current values; renaming is allowed and configuration
is re-validated on save.

```
wtm run profile edit [name] [flags]
```

### Options

```
  -h, --help            help for edit
      --output string   Output format: text or json (default "text")
```

### SEE ALSO

* [wtm run profile](wtm_run_profile.md)	 - Add, remove, or edit profiles in run.toml

