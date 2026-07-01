## wtm run job rm

Remove a job from run.toml

### Synopsis

Remove a job from <git-common-dir>/wtm/run.toml.

Without an argument, prompts to pick from the existing jobs.
Fails if the job is referenced by any profile, unless --force is given
(in which case the references are stripped from those profiles too).

```
wtm run job rm [name] [flags]
```

### Options

```
      --force           Also strip references from profiles that use this job
  -h, --help            help for rm
      --output string   Output format: text or json (default "text")
```

### SEE ALSO

* [wtm run job](wtm_run_job.md)	 - Add, remove, or edit jobs in run.toml

