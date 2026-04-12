# Changelog

## v0.6.0 — Ready for LLM agents

### New features

- **`--output json` sur les commandes data** — `wt list`, `wt create`, `wt clean` (avec `--force`), `pr list`, `pr create`, `pr checkout`, `svc list`, `svc ps`, `svc up`, `svc down`, `svc start`, `svc stop` retournent maintenant du JSON machine-readable sur stdout. Le texte humain reste sur stderr. Permet aux agents (Claude Code, Cursor, scripts) de piloter wtm sans TUI.
- **`wtm svc list`** — Liste les services et profils déclarés dans `.wtm/services.toml`. En TTY, picker interactif avec actions `up`/`down` sur un profil ou `start`/`stop`/`logs` sur un service, pour découvrir le cycle de vie svc sans mémoriser chaque commande.
- **`wtm svc ps`** — Liste les services gérés en ce moment par le daemon (name, status, pid, worktree). Picker avec actions `stop`/`logs`/`restart` + une entrée "Stop all running services" qui dispatche `svc down --all`.
- **`wtm agents install`** — Détecte les destinations skill existantes (`.claude/` / `.cursor/` projet ou global) et installe un skill compact `using-wtm` que Claude Code / Cursor consultent automatiquement quand l'utilisateur parle worktrees, services ou PRs.
- **Détection `docker-compose` dans `wtm init`** — Si des fichiers `docker-compose*.yml/yaml` sont trouvés, étape wizard MultiSelect pour scaffolder des services correspondants dans `.wtm/services.toml` avec `up -d` / `down --remove-orphans` et détection automatique de la commande (`docker compose` v2 ou `docker-compose` v1).
- **`wtm svc down --all`** — Stoppe tous les services de tous les worktrees (le comportement par défaut de `svc down` reste scoped au worktree courant).

### Bug fixes

- **Échecs silencieux de `docker compose up -d`** (LUC-56) — Les services launcher-style (ceux avec un `Stop`) affichaient `✓ started` même quand docker échouait (port conflit, image manquante, compose invalide). Le manager attend maintenant la sortie du launcher et remonte l'erreur avec la sortie capturée, nettoyée des ANSI et des redraw `\r` de compose.
- **`svc down` traversant les worktrees** — `svc down` (et indirectement `wt clean`, `svc up --exclusive`) pouvait stopper des services d'autres worktrees parce que `handleStopAll` ignorait `Request.WorkDir`. Ajout de `StopAllInWorkDir` et respect du workdir côté daemon.

### Improvements

- **Picker `wt switch` aligné sur `wt list`** — Même styling (breadcrumb, badges parent / PR / services / dirty) quand l'utilisateur appelle `wt switch` sans argument.
- **Spinners sur les opérations svc** — `svc up`, `svc down`, `svc start` affichent maintenant un spinner pendant l'aller-retour daemon (utile quand `docker pull` prend plusieurs secondes).
- **`output.Error` multi-ligne** — Les erreurs avec sortie capturée (typiquement docker compose) sont formatées en bloc indenté au lieu d'une ligne illisible.
- **`wtm svc down` scoping par défaut** — Sans `--all`, ne touche que le worktree courant. Help text mis à jour.

## v0.5.1 — Fix TUI et navigation shell

### Corrections

- **TUI invisible dans `wt go` / `wt switch`** — Le picker de worktree ne s'affichait pas quand appelé via le shell wrapper. Le TUI Bubbletea rendait sur stdout, qui était capturé par la substitution `$()`. Le rendu passe maintenant sur stderr.
- **"Go to worktree" depuis `pr list` et `wt list`** — L'action affichait "requires shell integration" au lieu de naviguer. Les commandes résolvaient le path via un sous-processus `wtm wt go` qui tombait sur le fallback. Remplacé par une résolution directe et écriture dans `WTM_GO_FILE`.
- **Shell wrapper étendu** — La clause `else` du wrapper (bash/zsh/fish) passe maintenant `WTM_GO_FILE` à toutes les commandes, permettant à n'importe quelle sous-commande de déclencher un `cd`.

## v0.5.0 — New TUI Components, Focus Removal & Unified Output

### Breaking changes

- **`wtm wt focus` removed** — The focus command, `on_focus`/`on_blur` hooks, and active worktree state tracking have been removed. Services are now managed exclusively through `svc up`/`svc down`.
- **`on_focus` / `on_blur` hooks removed from config** — Only `on_create` hooks remain. Docker lifecycle is handled by the service manager.
- **Dashboard hidden** — The interactive dashboard is disabled behind a feature flag while being reworked. `wtm` without arguments shows help instead.

### New features

- **`wtm wt switch [branch]`** — New command that combines `wt go` + `svc up` in one step. Supports `--exclusive`, `--parallel`, and `--profile` flags.
- **Smart `svc up`** — Detects services running on other worktrees and prompts to stop them before starting. Use `--exclusive` to auto-stop or `--parallel` to skip the prompt.
- **Auto-stop on clean** — `wt clean` automatically stops running services before deleting a worktree.
- **Reusable TUI components** — New Bubbletea component library (`internal/tui/components/`) with SelectList, TextInput, MultiSelect, Confirm, and Wizard. Full-row highlight, inline filtering (`/`), step breadcrumb, and Esc back navigation.
- **Contextual PR actions** — `pr list` picker shows "Go to worktree" if a worktree exists for the PR branch, "Checkout into worktree" otherwise.
- **Worktree badges** — `wt list` picker shows colored badge chips (parent, PR, services, dirty/clean) aligned to the right.

### Improvements

- **Unified output styling** — All CLI messages use standardized helpers (`output.Success`, `output.Error`, `output.Warning`, `output.Loading`, `output.Message`) with consistent `"  "` indent.
- **Uniform spacing** — Every command has blank line padding top and bottom. Help text is indented to match.
- **Centralized error display** — All errors go through a styled `✗` handler with proper padding.
- **PR detail view** — Rewritten with output helpers, no more lipgloss box.
- **Separator support** — Action lists use visual separators to group navigation, services, and danger actions.
- **Detached service fix** — Services using `docker compose up -d` (detached mode) are now correctly tracked as running and properly stopped.
- **Config resolution fix** — `svc up`, `svc start`, `svc down` now correctly read `services.toml` from the main worktree when run from a secondary worktree.

### Removed

- `charmbracelet/huh` dependency — Fully replaced by custom Bubbletea components.
- `state.json` active worktree tracking — No longer written to.
- Docker hooks from `wtm init` wizard — The docker-compose file selection and hook confirmation steps are removed.

### Tests

- Added 35+ new tests across commands, output, config, and infra layers.
- Commands: `branchInList`, `buildWorktreeLabel`, `joinTags`, `truncate`, `joinServiceNames`.
- Output: all block helpers (Success, Error, Warning, Loading, Message, etc.).
- Config: corrupted TOML, merge precedence, default application.
- Infra: IsDirty, CurrentBranch, CommitsAhead.

---

## v0.4.1 — Migrate GitHub integration to gh CLI

### Breaking changes

- **`wtm auth` supprimé** — Les commandes `wtm auth login/status/logout` n'existent plus.
  L'authentification GitHub est désormais gérée par le `gh` CLI.
  Installez-le et connectez-vous avec `gh auth login` : [cli.github.com](https://cli.github.com).
- **`WTM_GITHUB_TOKEN` non supporté** — Utilisez `GH_TOKEN` à la place (nativement supporté par `gh`).

### Improvements

- Suppression de toute la couche auth custom (OAuth Device Flow, token storage, auto-refresh) au profit du `gh` CLI.
- `wtm pr list`, `wtm pr create`, `wtm pr checkout` et le dashboard PR passent par `gh` en subprocess.
- Le README documente désormais les dépendances (`git` requis, `gh` recommandé).

---

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
