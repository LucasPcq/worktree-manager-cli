## wtm run down

Stop jobs running in the current worktree

### Synopsis

Stop jobs running in the current worktree.
With a profile argument, stops only that profile's jobs.
Jobs running in other worktrees are never touched.

```
wtm run down [profile] [flags]
```

### Options

```
      --all             Stop jobs across every worktree (bypasses per-worktree scoping)
  -h, --help            help for down
      --output string   Output format: text or json (default "text")
```

### SEE ALSO

* [wtm run](wtm_run.md)	 - Manage dev jobs (services + tasks)

