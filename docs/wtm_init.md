## wtm init

Initialize wtm configuration

### Synopsis

Interactive wizard to set up global config and project config in <git-common-dir>/wtm/config.toml.
Pass --non-interactive (or any config flag) to bootstrap from flags + auto-detection instead.
Use --only env|hooks|services to re-run init for specific sections and regenerate them cleanly
(run.toml jobs are regenerated while profiles are preserved).

```
wtm init [flags]
```

### Options

```
      --base-branch string       Default base branch for new worktrees
      --base-path string         Worktree directory, relative to repo root
      --env-strategy string      Env provisioning strategy: example, main, or parent
  -h, --help                     help for init
      --install-command string   Command to run after creating a worktree
      --non-interactive          Bootstrap from flags + auto-detection; never prompt
      --only strings             Re-init only these sections (env, hooks, services); regenerates them cleanly
      --shell string             Global shell: zsh, bash, or fish
      --skip-env                 Skip .env provisioning config
      --skip-hooks               Skip on_create hooks config
      --skip-services            Skip service/task detection (docker, scripts)
      --yes                      Skip the re-init confirmation prompt
```

### SEE ALSO

* [wtm](wtm.md)	 - Orchestrate git worktrees and team dev workflows from the terminal

