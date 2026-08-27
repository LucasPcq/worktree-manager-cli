## wtm run url

Print where a job is reachable in this worktree

### Synopsis

Write a job's URL on stdout and nothing else, for $(…). --raw prints the job's own port instead of its name, which every OS resolves and no proxy has to serve.

```
wtm run url [job] [flags]
```

### Options

```
  -h, --help            help for url
      --output string   Output format: text or json (default "text")
      --raw             Print the direct http://localhost:<port> address
```

### SEE ALSO

* [wtm run](wtm_run.md)	 - Manage dev jobs (services + tasks)

