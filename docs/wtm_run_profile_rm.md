## wtm run profile rm

Remove a profile from run.toml

### Synopsis

Remove a profile from <git-common-dir>/wtm/run.toml.

Without an argument, prompts to pick from the existing profiles.
Jobs referenced by the profile are left untouched.

```
wtm run profile rm [name] [flags]
```

### Options

```
  -h, --help            help for rm
      --output string   Output format: text or json (default "text")
```

### SEE ALSO

* [wtm run profile](wtm_run_profile.md)	 - Add, remove, or edit profiles in run.toml

