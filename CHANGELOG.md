# Changelog

## v0.4.0 — GitHub Integration & PR Management

### New features

- **GitHub authentication** — `wtm auth login` lance un flux OAuth Device Flow.
  `wtm auth status` affiche l'état du token, `wtm auth logout` le révoque.
  Support de `WTM_GITHUB_TOKEN` comme PAT alternatif. Token auto-refreshé en arrière-plan.
- **`wtm pr list`** — Liste les pull requests du dépôt avec filtres `--mine` et `--review`.
  Intégré dans le dashboard (panneau droit, touche `p`).
- **`wtm pr create`** — Assistant interactif pour créer une PR depuis la branche courante :
  titre, body, draft, reviewers.
- **`wtm pr checkout`** — Crée un worktree directement depuis une branche de PR existante.

### Breaking changes

- **Commandes regroupées sous `wtm wt` et `wtm svc`** — Les commandes worktree passent
  sous `wtm wt` (ex. `wtm wt new`, `wtm wt ls`, `wtm wt go`). Les commandes service
  passent sous `wtm svc` (ex. `wtm svc up`, `wtm svc down`, `wtm svc start`, `wtm svc stop`).
- **`wtm svc start/stop`** — `start`/`stop` ciblent des services individuels,
  `up`/`down` gèrent les profils complets. Les deux sont sous `wtm svc`.

### Improvements

- Migration de `gh` CLI vers `go-github` pour la détection de PR ouverte lors du `clean`.
- Dashboard : logs multiplexés, focus corrigé, split panel worktrees/PRs 50/50.

---

## v0.3.0 — Services & PTY

### New features

- **Service management** — Define long-running services (dev servers, docker, workers) in `.wtm/services.toml` with named profiles.
- **`wtm up`** — Start services with interactive profile picker when multiple profiles exist. `--profile` flag for direct selection.
- **`wtm down`** — Stop running services gracefully.
- **`wtm logs <service>`** — Attach to a service's real terminal via PTY. Full ANSI support (colors, progress bars). Ctrl+C detaches without killing the service.
- **Daemon architecture** — Background daemon owns PTY file descriptors. Services survive CLI exits. Auto-starts on first `wtm up`, auto-exits when idle.
- **Services scoped per worktree** — Each worktree runs its own independent services. Dashboard shows service indicators per worktree.
- **Dashboard service controls** — `u` to start services, `x` to stop, `s` to attach to a running service's terminal.
- **Validation warnings** — Warns on duplicate service/profile names or multiple default profiles in `.wtm/services.toml`.

### Breaking changes

- **Config directory restructured** — `.wtm.toml` is now `.wtm/config.toml`, `.wtm.services.toml` is now `.wtm/services.toml`. All config lives in the `.wtm/` directory. Move your files: `mkdir -p .wtm && mv .wtm.toml .wtm/config.toml`.

### Improvements

- Profile picker with service list labels (e.g. "back (api, worker)").
- `handleKey` refactored — worktree actions extracted to `handleWorktreeAction`, eliminating duplicated selection checks.
- Code cleanup: extracted subprocess pattern, centralized constants (daemon timeouts, CtrlC byte), simplified focus writer logic.

---

## v0.2.0 — Interactive Dashboard

### New features

- **Interactive Dashboard** — Run `wtm` without arguments to open a full-screen terminal dashboard. See all your worktrees at a glance with their git status, and manage everything without leaving the interface.
- **Dashboard worktree list** — Left panel shows all worktrees with branch name, status (clean/dirty), commits ahead, and focus indicator.
- **Dashboard detail panel** — Right panel shows worktree details: path, source branch, unpushed commits count, context notes, and full list of modified files with scrollable viewport.
- **Dashboard actions** — Create worktrees (`n`), clean them (`d`), focus environment (`f`), and navigate (`Enter`) directly from the dashboard.
- **Hook output streaming** — When focusing a worktree, a split panel streams hook execution output in real-time. Stays visible for review, closes with `Esc`.
- **Panel navigation** — `Tab`/`Shift+Tab` cycles between list, detail, and hooks output panels. `j/k` or arrows to scroll the active panel.
- **Interactive branch prompt** — `wtm new` now works without arguments, prompting for the branch name interactively.

### Improvements

- Commands now work from any worktree, not just the project root.
- Hook blur fallback: if a worktree directory was deleted, blur hooks run from the project root instead of failing.
- Shell wrapper auto-returns to the main worktree after cleaning the current one.
- Hook errors in TUI mode are captured and displayed in the detail panel instead of corrupting the screen.

---

## v0.1.2 — Bugfixes

- Fix project root resolution from child worktrees (config not found error).
- Fix blur hooks failing when previous worktree directory no longer exists.
- Fix shell wrapper redirect after cleaning current worktree.

---

## v0.1.1 — First release

### Commands

- `wtm init` — Interactive wizard to set up global and project configuration.
- `wtm new [branch]` — Create a git worktree with env provisioning, metadata, and hooks.
- `wtm ls` — List all worktrees with git status (clean/dirty, commits ahead).
- `wtm go [branch]` — Navigate to a worktree directory via shell integration.
- `wtm focus [branch]` — Switch active worktree and run on_blur/on_focus hooks.
- `wtm clean [branch]` — Remove a worktree with safety checks (dirty, unpushed, open PR).
- `wtm shell-init` — Generate shell wrapper function for zsh, bash, and fish.

### Configuration

- TOML-based config: `.wtm.toml` (project) + `~/.config/wtm/config.toml` (global).
- Three env strategies: example, main, parent.
- Hook engine with template variables, continue_on_error, and timing display.
- Auto-detection: base branch, env files, package manager, Docker Compose, pnpm workspaces.

### Distribution

- Homebrew: `brew install LucasPcq/tap/wtm`
- GitHub Releases: binaries for macOS/Linux (amd64/arm64)
- `go install github.com/LucasPcq/wtm@latest`
