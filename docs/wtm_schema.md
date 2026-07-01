## wtm schema

Inspect or extract bundled JSON Schemas

### Synopsis

JSON Schemas describe the structure of wtm's TOML config files.
Use `wtm schema dump` to write them to <git-common-dir>/wtm/schemas/ so editors can pick them up via the `#:schema` directive.

### Options

```
  -h, --help   help for schema
```

### SEE ALSO

* [wtm](wtm.md)	 - Orchestrate git worktrees and team dev workflows from the terminal
* [wtm schema dump](wtm_schema_dump.md)	 - Write embedded schemas to <state-dir>/schemas/ (or ~/.config/wtm/schemas with --global)

