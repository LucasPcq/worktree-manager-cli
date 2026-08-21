## wtm run init

Configure the run module (services & tasks) for this repo

### Synopsis

Set up run.toml by detecting docker-compose files and package.json scripts and turning
the selected ones into jobs.

In a TTY, opens a wizard to pick which ones to include; non-interactively (or piped),
auto-generates from detection. Re-running merges new selections into the existing
run.toml without overwriting what's already there.

Ports declared in the selected compose files become per-worktree ports. A literal
host port ("5432:5432") binds the same port everywhere, so wtm offers to rewrite it
as "${DB_PORT:-5432}:5432" — the default keeps `docker compose up` working on its
own. Declining leaves the file untouched and declares no port for it.

Dev servers get theirs from the env files sitting next to their package.json —
a PORT (or *_PORT) entry in .env.local, .env, or a committed .env.example. wtm
only declares the port; check that the command actually reads the variable, and
pass it as a flag otherwise: --cmd 'pnpm dev --port ${PORT}'.

`wtm run` is experimental — the workflow is still stabilizing and commands may change.

```
wtm run init [flags]
```

### Options

```
  -h, --help              help for init
      --non-interactive   Auto-generate from detection; never prompt
      --patch-compose     Rewrite literal host ports in the selected compose files to read a variable
```

### SEE ALSO

* [wtm run](wtm_run.md)	 - Manage dev jobs (services + tasks)

