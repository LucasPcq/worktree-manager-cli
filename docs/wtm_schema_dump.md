## wtm schema dump

Write embedded schemas to <state-dir>/schemas/ (or ~/.config/wtm/schemas with --global)

### Synopsis

Extract every JSON Schema bundled with this wtm binary so editors can resolve the `#:schema` directives in your TOML files.
Project schemas land in <git-common-dir>/wtm/schemas/. Use --global to write the global schema next to ~/.config/wtm/config.toml.

```
wtm schema dump [flags]
```

### Options

```
      --global   Write the global config schema instead of the project ones
  -h, --help     help for dump
```

### SEE ALSO

* [wtm schema](wtm_schema.md)	 - Inspect or extract bundled JSON Schemas

