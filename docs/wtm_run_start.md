## wtm run start

Start a single job

### Synopsis

Start an individual job by name (defined in run.toml).
A task runs inline and blocks until it exits; a service opens the run view on itself, and -d starts it in the background instead.

```
wtm run start <job> [flags]
```

### Options

```
  -d, --detach          Start the job and return instead of opening the run view
  -h, --help            help for start
      --output string   Output format: text or json (default "text")
```

### SEE ALSO

* [wtm run](wtm_run.md)	 - Manage dev jobs (services + tasks)

