# Changelog

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
