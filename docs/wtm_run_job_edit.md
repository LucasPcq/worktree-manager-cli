## wtm run job edit

Edit an existing job

### Synopsis

Edit a job declared in <git-common-dir>/wtm/run.toml.

Pass any of --name, --cmd, --kind, --stop, --cwd, --port, --port-clear,
--url-port or --url-host for non-interactive use: a flag left out keeps the
field as it is, and passing an empty string clears it (--stop '' drops the
stop command, --url-port '' withdraws the published name).

--port merges into the ports the job already declares, so one entry can be
changed without rewriting the others; --port-clear empties the table.
--name also rewrites the references to this job in every profile.

With no such flag, the wizard opens pre-filled with the current values, and
without an argument it prompts to pick from the existing jobs.

```
wtm run job edit [name] [flags]
```

### Options

```
      --cmd string         Command to run, as a /bin/sh line
      --cwd string         Working directory relative to project root (pass '' to drop it)
  -h, --help               help for edit
      --kind string        Job kind: service or task
      --name string        Rename the job, updating the profiles that reference it
      --output string      Output format: text or json (default "text")
      --port stringArray   Base port as NAME=PORT, repeatable — merged into the declared ports
      --port-clear         Drop every port this job declares
      --stop string        Stop command, as a /bin/sh line (pass '' to drop it)
      --url-host string    Host segment to publish under (pass '' to fall back to the job's name)
      --url-port string    Publish this declared port under a name (pass '' to withdraw the url)
```

### SEE ALSO

* [wtm run job](wtm_run_job.md)	 - Add, remove, or edit jobs in run.toml

