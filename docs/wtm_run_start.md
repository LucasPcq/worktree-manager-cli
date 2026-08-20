## wtm run start

Start a single job

### Synopsis

Start an individual job by name (defined in run.toml).
A service attaches: its output opens in the run view, and leaving the view detaches without stopping it.
-d starts it and returns the prompt instead.
A task always runs inline and blocks until it exits, with or without -d.

```
wtm run start <job> [flags]
```

### Options

```
  -d, --detach          Start the service and return immediately instead of opening its output
  -h, --help            help for start
      --output string   Output format: text or json (default "text")
```

### SEE ALSO

* [wtm run](wtm_run.md)	 - Manage dev jobs (services + tasks)

