## wtm run open

Open a job's URL in the browser

### Synopsis

Hand a job's URL to the desktop's own opener. Naming the job is required outside a fully interactive run — a picker never runs under a pipe or --output json.

```
wtm run open [job] [flags]
```

### Options

```
  -h, --help            help for open
      --output string   Output format: text or json (default "text")
      --raw             Open the direct http://localhost:<port> address
```

### SEE ALSO

* [wtm run](wtm_run.md)	 - Manage dev jobs (services + tasks)

