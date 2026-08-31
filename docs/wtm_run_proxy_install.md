## wtm run proxy install

Redirect port 80 to the run proxy so named URLs drop their port

### Synopsis

Write the OS files that redirect port 80 to the run proxy. The recap shows every file before sudo is asked for, and `wtm run proxy uninstall` reverses all of it.

```
wtm run proxy install [flags]
```

### Options

```
      --dry-run         Print every file in full and write nothing
  -h, --help            help for install
      --output string   Output format: text or json (default "text")
  -y, --yes             Skip the confirmation (sudo still asks for a password)
```

### SEE ALSO

* [wtm run proxy](wtm_run_proxy.md)	 - Inspect and install the redirection that serves named URLs on port 80

