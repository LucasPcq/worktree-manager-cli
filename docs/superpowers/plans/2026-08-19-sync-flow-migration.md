# LUC-186 — `sync` vers `flow/` + dashboard — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Faire passer `wtm sync` par `internal/flow/sync/`, supprimer `internal/tui/syncpicker` et sa closure `PlanPreview`, et exposer sync dans le dashboard (menu ligne, menu base, `⋯ Actions`).

**Architecture:** `commands/wt/sync.go` ne garde que le wiring des flags et le choix Prompter/Presenter. Le flow déclare quatre `flow.Step` (sélection, on-conflict, parents, recap) et un `Presenter` à trois moments (`Planned`, `Rebased`, `Synced`), parce que la question du push tombe entre deux sorties. Le dashboard réutilise la même session via son propre Prompter/Presenter.

**Tech Stack:** Go 1.2x, Cobra, Bubbletea, Lipgloss, `internal/flow`, `internal/testutil/{gittest,flowtest}`.

**Spec:** `docs/superpowers/specs/2026-08-19-sync-flow-migration-design.md`

## Global Constraints

- `internal/flow/` n'importe **que** `internal/service/`, `internal/rules/`, `internal/domain/` et la stdlib. Jamais cobra, bubbletea, lipgloss, `output/`, `tui/`, `config/`, `commands/`.
- `internal/rules/` n'importe que la stdlib et `internal/domain`. Aucune I/O.
- Toute fonction à 2+ entrées liées prend une struct de params à champs nommés.
- Aucun magic string : toute chaîne d'UI, tout nom de flag, toute clé va dans `internal/domain/constants.go`.
- Retours anticipés, aucune imbrication de `if`. Comma-ok obligatoire pour toute assertion de type.
- Commentaires quasi nuls : uniquement le *pourquoi*. Les fichiers touchés voient leurs commentaires descriptifs supprimés au passage (règle de migration du CLAUDE.md).
- Les commits ne portent **aucune** attribution co-auteur.
- Les tests de caractérisation des tâches 1 et 2 ne sont **jamais** modifiés après la tâche 2.
- Nom de la branche de travail : `feat/ui/sync` (déjà en place).

---

### Task 1: Fixture de caractérisation + sélection, flags, dry-run, push

Les tests de caractérisation décrivent le comportement **actuel**. Ils passent donc dès leur écriture, sur du code non modifié : c'est le contrat qui protège la migration. Ils ne s'exécutent jamais sur un TTY (binaire de test) : chaque cas passe donc `--yes`, `--dry-run` ou `--output json`. Le cas « sortie humaine sans TTY et sans `--yes` » est le seul comportement que la migration change (D7) ; il est volontairement **absent** d'ici et arrive en tâche 7.

**Files:**
- Create: `internal/commands/wt/sync_characterization_test.go`

**Interfaces:**
- Consumes: `runWtCmd(t, args...) (stdout, stderr string, err error)` de `internal/commands/wt/integration_test.go` ; `gittest.InitRepo`, `gittest.AddOrigin`, `gittest.PushBranch`, `gittest.Git`.
- Produces: `setupSync(t, syncSetup) syncRepo` et ses méthodes `create`, `commitOn`, `tipOf` — réutilisés par la tâche 2.

- [ ] **Step 1: Écrire la fixture et les premiers cas**

```go
package wt

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/testutil/gittest"
)

// Ces tests caractérisent `wtm sync` tel qu'il se comporte aujourd'hui, avant la
// migration vers flow/ : quelles worktrees chaque entrée sélectionne, ce que --all,
// --dry-run, --push/--no-push et --yes résolvent, et l'ordre de la cascade. Ils
// doivent passer inchangés après la migration.

type syncSetup struct {
	// Stack décrit les worktrees à créer, dans l'ordre : chaque entrée est
	// "branche:parent". Le parent doit déjà exister.
	Stack []string
}

type syncRepo struct {
	dir      string
	stateDir string
	remote   string
	paths    map[string]string
}

func setupSync(t *testing.T, setup syncSetup) syncRepo {
	t.Helper()
	repo := syncRepo{paths: map[string]string{}}
	repo.dir = gittest.InitRepo(t)
	repo.stateDir = filepath.Join(repo.dir, ".git", "wtm")
	t.Setenv("WTM_PROJECT_DIR", repo.dir)
	t.Setenv("WTM_STATE_DIR", repo.stateDir)
	t.Setenv(domain.EnvGoFile, "")

	if err := writeSyncConfig(repo.stateDir); err != nil {
		t.Fatalf("setup config: %v", err)
	}
	repo.remote = gittest.AddOrigin(t, repo.dir)

	for _, entry := range setup.Stack {
		branch, parent, ok := strings.Cut(entry, ":")
		if !ok {
			t.Fatalf("stack entry %q must be \"branch:parent\"", entry)
		}
		repo.create(t, branch, parent)
	}
	return repo
}

func writeSyncConfig(stateDir string) error {
	if err := os.MkdirAll(stateDir, 0o755); err != nil {
		return err
	}
	content := `[worktrees]
base_path = "../.trees"
base_branch = "main"

[env]
strategy = "example"

[hooks]
on_create = []
on_clean = []
`
	return os.WriteFile(filepath.Join(stateDir, "config.toml"), []byte(content), 0o644)
}

func (r *syncRepo) create(t *testing.T, branch, from string) {
	t.Helper()
	stdout, _, err := runWtCmd(t, domain.CmdCreate, branch,
		"--from", from, "--output", domain.OutputJSON, "--"+domain.FlagYes)
	if err != nil {
		t.Fatalf("create %s: %v", branch, err)
	}
	var created struct {
		Path string `json:"path"`
	}
	if jsonErr := json.Unmarshal([]byte(stdout), &created); jsonErr != nil || created.Path == "" {
		t.Fatalf("create %s: cannot read the worktree path from %q", branch, stdout)
	}
	r.paths[branch] = created.Path
	gittest.PushBranch(t, r.dir, branch)
}

// commitOn ajoute un commit sur une branche, dans sa worktree (ou le dépôt
// principal pour la base), et le publie quand push est demandé.
func (r syncRepo) commitOn(t *testing.T, branch, file string, push bool) {
	t.Helper()
	dir := r.paths[branch]
	if dir == "" {
		dir = r.dir
	}
	if err := os.WriteFile(filepath.Join(dir, file), []byte(file+"\n"), 0o644); err != nil {
		t.Fatalf("write %s: %v", file, err)
	}
	gittest.Git(t, dir, "add", file)
	gittest.Git(t, dir, "commit", "-m", "add "+file)
	if push {
		gittest.PushBranch(t, dir, branch)
	}
}

func (r syncRepo) tipOf(t *testing.T, branch string) string {
	t.Helper()
	dir := r.paths[branch]
	if dir == "" {
		dir = r.dir
	}
	out, err := exec.Command("git", "-C", dir, "rev-parse", branch).Output()
	if err != nil {
		t.Fatalf("rev-parse %s: %v", branch, err)
	}
	return strings.TrimSpace(string(out))
}

// syncJSON lance sync en sortie machine et rend le résultat décodé.
func syncJSON(t *testing.T, args ...string) domain.SyncResult {
	t.Helper()
	full := append([]string{domain.CmdSync}, args...)
	full = append(full, "--output", domain.OutputJSON, "--"+domain.FlagYes)
	stdout, _, err := runWtCmd(t, full...)
	if err != nil {
		t.Fatalf("sync %v: %v", args, err)
	}
	var result domain.SyncResult
	if jsonErr := json.Unmarshal([]byte(stdout), &result); jsonErr != nil {
		t.Fatalf("sync %v: cannot decode %q: %v", args, stdout, jsonErr)
	}
	return result
}

func TestSyncArgSelectsOnlyThatWorktree(t *testing.T) {
	repo := setupSync(t, syncSetup{Stack: []string{"feat-a:main", "feat-b:feat-a"}})
	repo.commitOn(t, "main", "base.txt", true)

	result := syncJSON(t, "feat-a")

	if len(result.Steps) != 1 || result.Steps[0].Branch != "feat-a" {
		t.Fatalf("sync feat-a must rebase feat-a alone, got %+v", result.Steps)
	}
}

func TestSyncAllSyncsEveryWorktree(t *testing.T) {
	repo := setupSync(t, syncSetup{Stack: []string{"feat-a:main", "feat-b:feat-a"}})
	repo.commitOn(t, "main", "base.txt", true)

	result := syncJSON(t, "--"+domain.FlagAll)

	if len(result.Steps) != 2 {
		t.Fatalf("--all must rebase every worktree, got %+v", result.Steps)
	}
}

func TestSyncAllRefusesBranchArguments(t *testing.T) {
	setupSync(t, syncSetup{Stack: []string{"feat-a:main"}})

	_, _, err := runWtCmd(t, domain.CmdSync, "feat-a", "--"+domain.FlagAll, "--"+domain.FlagYes)
	if err == nil {
		t.Fatal("--all combined with branch arguments must be refused")
	}
	if !strings.Contains(err.Error(), "--"+domain.FlagAll) {
		t.Fatalf("the refusal must name --all, got: %v", err)
	}
}

func TestSyncPushAndNoPushAreMutuallyExclusive(t *testing.T) {
	setupSync(t, syncSetup{Stack: []string{"feat-a:main"}})

	_, _, err := runWtCmd(t, domain.CmdSync, "--"+domain.FlagAll,
		"--"+domain.FlagPush, "--"+domain.FlagNoPush, "--"+domain.FlagYes)
	if err == nil {
		t.Fatal("--push and --no-push must be refused together")
	}
}

func TestSyncFFParentsAndNoFFParentsAreMutuallyExclusive(t *testing.T) {
	setupSync(t, syncSetup{Stack: []string{"feat-a:main"}})

	_, _, err := runWtCmd(t, domain.CmdSync, "--"+domain.FlagAll,
		"--"+domain.FlagFFParents, "--"+domain.FlagNoFFParents, "--"+domain.FlagYes)
	if err == nil {
		t.Fatal("--ff-parents and --no-ff-parents must be refused together")
	}
}

func TestSyncYesWithoutTargetNamesAll(t *testing.T) {
	setupSync(t, syncSetup{Stack: []string{"feat-a:main"}})

	_, _, err := runWtCmd(t, domain.CmdSync, "--"+domain.FlagYes)
	if err == nil {
		t.Fatal("--yes with no branch and no --all must be refused")
	}
	if !strings.Contains(err.Error(), "--"+domain.FlagAll) {
		t.Fatalf("the refusal must name --all, got: %v", err)
	}
}

func TestSyncJSONRequiresYesOrDryRun(t *testing.T) {
	setupSync(t, syncSetup{Stack: []string{"feat-a:main"}})

	_, _, err := runWtCmd(t, domain.CmdSync, "--"+domain.FlagAll, "--output", domain.OutputJSON)
	if err == nil {
		t.Fatal("--output json without --yes or --dry-run must be refused")
	}
	if !strings.Contains(err.Error(), "--"+domain.FlagYes) {
		t.Fatalf("the refusal must name --yes, got: %v", err)
	}
}

func TestSyncDryRunRebasesNothing(t *testing.T) {
	repo := setupSync(t, syncSetup{Stack: []string{"feat-a:main"}})
	repo.commitOn(t, "main", "base.txt", true)
	before := repo.tipOf(t, "feat-a")

	stdout, _, err := runWtCmd(t, domain.CmdSync, "--"+domain.FlagAll,
		"--"+domain.FlagDryRun, "--output", domain.OutputJSON)
	if err != nil {
		t.Fatalf("sync --dry-run: %v", err)
	}
	if after := repo.tipOf(t, "feat-a"); after != before {
		t.Fatalf("--dry-run moved feat-a: %s → %s", before, after)
	}
}

func TestSyncYesDoesNotPushByDefault(t *testing.T) {
	repo := setupSync(t, syncSetup{Stack: []string{"feat-a:main"}})
	repo.commitOn(t, "main", "base.txt", true)

	result := syncJSON(t, "--"+domain.FlagAll)

	for _, step := range result.Steps {
		if step.Pushed {
			t.Fatalf("--yes must not push; %s was pushed", step.Branch)
		}
	}
}

func TestSyncPushForcePushesRebasedBranches(t *testing.T) {
	repo := setupSync(t, syncSetup{Stack: []string{"feat-a:main"}})
	repo.commitOn(t, "main", "base.txt", true)

	result := syncJSON(t, "--"+domain.FlagAll, "--"+domain.FlagPush)

	pushed := false
	for _, step := range result.Steps {
		if step.Pushed {
			pushed = true
		}
	}
	if !pushed {
		t.Fatalf("--push must push the rebased branches, got %+v", result.Steps)
	}
}

func TestSyncNoPushNeverPushes(t *testing.T) {
	repo := setupSync(t, syncSetup{Stack: []string{"feat-a:main"}})
	repo.commitOn(t, "main", "base.txt", true)

	result := syncJSON(t, "--"+domain.FlagAll, "--"+domain.FlagNoPush)

	for _, step := range result.Steps {
		if step.Pushed {
			t.Fatalf("--no-push must never push; %s was pushed", step.Branch)
		}
	}
}
```

- [ ] **Step 2: Compléter les imports**

Ajouter `os/exec` (requis par `tipOf`). Les champs utilisés existent et sont vérifiés dans `internal/domain/sync.go` : `SyncStepResult.Branch`, `.Status`, `.Path`, `.Pushed`, `.PushPending`, `.KeptInProgress`, et `SyncResult.Steps`. Le champ dry-run du JSON est `SyncResult`/`PruneResult`-dépendant : lire le tag réel dans `internal/domain/sync.go` plutôt que de tester les deux orthographes.

Run: `go build ./... && go vet ./internal/commands/wt/`

- [ ] **Step 3: Vérifier que les tests passent sur le code NON modifié**

Run: `go test ./internal/commands/wt/ -run 'TestSync' -v`
Expected: PASS. Un test de caractérisation qui échoue ici décrit mal l'existant — c'est le test qu'il faut corriger, jamais la production.

- [ ] **Step 4: Commit**

```bash
git add internal/commands/wt/sync_characterization_test.go
git commit -m "test(sync): characterize selection, flags and push before the flow migration"
```

---

### Task 2: Caractérisation — ordre de la cascade, conflit, ordre de sortie

**Files:**
- Modify: `internal/commands/wt/sync_characterization_test.go`

**Interfaces:**
- Consumes: `setupSync`, `syncRepo.commitOn`, `syncRepo.tipOf`, `syncJSON` (tâche 1).
- Produces: rien de nouveau.

- [ ] **Step 1: Écrire les cas d'ordre et de conflit**

```go
func TestSyncCascadeKeepsTopologicalOrder(t *testing.T) {
	repo := setupSync(t, syncSetup{Stack: []string{"feat-a:main", "feat-b:feat-a", "feat-c:feat-b"}})
	repo.commitOn(t, "main", "base.txt", true)

	result := syncJSON(t, "--"+domain.FlagAll)

	order := make([]string, 0, len(result.Steps))
	for _, step := range result.Steps {
		order = append(order, step.Branch)
	}
	want := []string{"feat-a", "feat-b", "feat-c"}
	if strings.Join(order, ",") != strings.Join(want, ",") {
		t.Fatalf("cascade order = %v, want %v (parents before children)", order, want)
	}
}

// Un conflit saute les descendants SÉLECTIONNÉS du nœud fautif, pas tous : une
// branche indépendante continue de se synchroniser.
func TestSyncConflictSkipsSelectedDescendantsOnly(t *testing.T) {
	repo := setupSync(t, syncSetup{Stack: []string{"feat-a:main", "feat-b:feat-a", "solo:main"}})
	// Le même fichier avance des deux côtés : le rebase de feat-a sur main conflit.
	repo.commitOn(t, "main", "shared.txt", true)
	repo.commitOn(t, "feat-a", "shared.txt", false)
	soloBefore := repo.tipOf(t, "solo")

	result := syncJSON(t, "--"+domain.FlagAll)

	byBranch := map[string]domain.SyncStepResult{}
	for _, step := range result.Steps {
		byBranch[step.Branch] = step
	}
	if byBranch["feat-a"].Status != domain.SyncStatusConflict {
		t.Fatalf("feat-a must conflict, got %+v", byBranch["feat-a"])
	}
	if byBranch["feat-b"].Status != domain.SyncStatusSkippedAncestor {
		t.Fatalf("feat-b (descendant of the conflict) must be skipped, got %+v", byBranch["feat-b"])
	}
	if repo.tipOf(t, "solo") == soloBefore {
		t.Fatal("an independent branch must keep syncing through another branch's conflict")
	}
}

// Sans --keep-conflict le rebase est abandonné : la worktree est laissée propre.
func TestSyncConflictAbortsAndLeavesWorktreeClean(t *testing.T) {
	repo := setupSync(t, syncSetup{Stack: []string{"feat-a:main"}})
	repo.commitOn(t, "main", "shared.txt", true)
	repo.commitOn(t, "feat-a", "shared.txt", false)

	syncJSON(t, "--"+domain.FlagAll)

	if _, err := os.Stat(filepath.Join(repo.paths["feat-a"], ".git")); err != nil {
		t.Fatalf("feat-a worktree must survive the aborted rebase: %v", err)
	}
	if inRebase(t, repo.paths["feat-a"]) {
		t.Fatal("without --keep-conflict the rebase must be aborted, not left in progress")
	}
}

func TestSyncKeepConflictLeavesRebaseInProgress(t *testing.T) {
	repo := setupSync(t, syncSetup{Stack: []string{"feat-a:main"}})
	repo.commitOn(t, "main", "shared.txt", true)
	repo.commitOn(t, "feat-a", "shared.txt", false)

	syncJSON(t, "--"+domain.FlagAll, "--"+domain.FlagKeepConflict)

	if !inRebase(t, repo.paths["feat-a"]) {
		t.Fatal("--keep-conflict must leave the rebase in progress in its worktree")
	}
}

// inRebase lit l'état de rebase d'une worktree via les répertoires que git pose.
func inRebase(t *testing.T, path string) bool {
	t.Helper()
	out, err := exec.Command("git", "-C", path, "rev-parse", "--git-path", "rebase-merge").Output()
	if err != nil {
		t.Fatalf("rev-parse --git-path: %v", err)
	}
	dir := strings.TrimSpace(string(out))
	if !filepath.IsAbs(dir) {
		dir = filepath.Join(path, dir)
	}
	_, statErr := os.Stat(dir)
	return statErr == nil
}
```

- [ ] **Step 2: Écrire les cas d'ordre de sortie (les deux chemins)**

```go
// Le chemin sans picker (--yes) imprime le plan sur stderr AVANT le recap, qui
// part sur stdout. C'est l'ordre que la migration doit reproduire à l'octet près.
func TestSyncYesPrintsPlanOnStderrThenRecapOnStdout(t *testing.T) {
	repo := setupSync(t, syncSetup{Stack: []string{"feat-a:main"}})
	repo.commitOn(t, "main", "base.txt", true)

	stdout, stderr, err := runWtCmd(t, domain.CmdSync, "--"+domain.FlagAll, "--"+domain.FlagYes)
	if err != nil {
		t.Fatalf("sync --all --yes: %v", err)
	}
	if !strings.Contains(stderr, "Sync plan") {
		t.Fatalf("the plan must be printed on stderr, got: %q", stderr)
	}
	if strings.Contains(stdout, "Sync plan") {
		t.Fatalf("the plan must not reach stdout, got: %q", stdout)
	}
	if !strings.Contains(stdout, "feat-a") {
		t.Fatalf("the recap must name the synced branch on stdout, got: %q", stdout)
	}
	// Le recap est précédé d'exactement une ligne vide.
	if !strings.HasPrefix(stdout, "\n") {
		t.Fatalf("the recap must keep its single leading blank line, got: %q", stdout)
	}
	if strings.HasPrefix(stdout, "\n\n") {
		t.Fatalf("the recap must not stack two blank lines, got: %q", stdout)
	}
}

func TestSyncDryRunPrintsThePlanAndNoPushSummary(t *testing.T) {
	repo := setupSync(t, syncSetup{Stack: []string{"feat-a:main"}})
	repo.commitOn(t, "main", "base.txt", true)

	_, stderr, err := runWtCmd(t, domain.CmdSync, "--"+domain.FlagAll, "--"+domain.FlagDryRun)
	if err != nil {
		t.Fatalf("sync --dry-run: %v", err)
	}
	if !strings.Contains(stderr, "Sync plan") {
		t.Fatalf("--dry-run must print the plan, got: %q", stderr)
	}
}

func TestSyncNothingToSyncReportsIt(t *testing.T) {
	setupSync(t, syncSetup{Stack: nil})

	stdout, _, err := runWtCmd(t, domain.CmdSync, "--"+domain.FlagAll, "--"+domain.FlagYes)
	if err != nil {
		t.Fatalf("sync --all on an empty repo: %v", err)
	}
	if !strings.Contains(stdout, "No worktrees to sync") {
		t.Fatalf("an empty plan must say so, got: %q", stdout)
	}
}
```

- [ ] **Step 3: Ajuster aux vrais noms**

Ouvrir `internal/domain/sync.go` pour les constantes de statut réelles (`SyncStatusConflict`, `SyncStatusSkipped` ou leurs équivalents) et `internal/output/sync.go` pour le libellé exact du titre de plan (`planTitle` rend `"Sync plan"` ou `"Sync plan (base: %s)"`). Remplacer les littéraux de test par les constantes `domain.*` quand elles existent.

- [ ] **Step 4: Vérifier que tout passe sur le code non modifié**

Run: `go test ./internal/commands/wt/ -run 'TestSync' -v`
Expected: PASS (tous les cas des tâches 1 et 2).

- [ ] **Step 5: Commit**

```bash
git add internal/commands/wt/sync_characterization_test.go
git commit -m "test(sync): characterize cascade order, conflict handling and output order"
```

À partir d'ici, ce fichier est **gelé**.

---

### Task 3: `rules.SprintSyncPlan` et `rules.SyncSubtree`

**Files:**
- Create: `internal/rules/sync_subtree.go`
- Create: `internal/rules/sync_subtree_test.go`
- Modify: `internal/rules/sync_plan.go` (accueille `SprintSyncPlan`)
- Modify: `internal/rules/sync_plan_test.go`
- Modify: `internal/output/sync.go:36-60` (délègue)
- Modify: `internal/output/sync_test.go` (les tests de `SprintSyncPlan` restent verts via la délégation)

**Interfaces:**
- Produces:
  - `func SprintSyncPlan(plan domain.SyncPlan) string` dans `rules` — texte brut, sans style, sans ligne vide extérieure.
  - `func SyncSubtree(params SyncSubtreeParams) []string` avec `type SyncSubtreeParams struct { Nodes []domain.WorktreeNode; Root string }`.
- Consumes: `domain.SyncPlan`, `domain.WorktreeNode`.

- [ ] **Step 1: Écrire le test de `SyncSubtree`**

```go
package rules

import (
	"strings"
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
)

func TestSyncSubtreeReturnsRootAndDescendants(t *testing.T) {
	nodes := []domain.WorktreeNode{
		{Branch: "main", IsMain: true},
		{Branch: "feat-a", SourceBranch: "main"},
		{Branch: "feat-b", SourceBranch: "feat-a"},
		{Branch: "feat-c", SourceBranch: "feat-b"},
		{Branch: "solo", SourceBranch: "main"},
	}

	got := SyncSubtree(SyncSubtreeParams{Nodes: nodes, Root: "feat-a"})

	want := "feat-a,feat-b,feat-c"
	if strings.Join(got, ",") != want {
		t.Fatalf("SyncSubtree(feat-a) = %v, want %s", got, want)
	}
}

func TestSyncSubtreeLeafReturnsItself(t *testing.T) {
	nodes := []domain.WorktreeNode{
		{Branch: "main", IsMain: true},
		{Branch: "feat-a", SourceBranch: "main"},
	}

	got := SyncSubtree(SyncSubtreeParams{Nodes: nodes, Root: "feat-a"})

	if len(got) != 1 || got[0] != "feat-a" {
		t.Fatalf("SyncSubtree on a leaf = %v, want [feat-a]", got)
	}
}

func TestSyncSubtreeUnknownRootReturnsNothing(t *testing.T) {
	nodes := []domain.WorktreeNode{{Branch: "feat-a", SourceBranch: "main"}}

	if got := SyncSubtree(SyncSubtreeParams{Nodes: nodes, Root: "ghost"}); len(got) != 0 {
		t.Fatalf("SyncSubtree on an unknown root = %v, want nothing", got)
	}
}

// Un cycle dans la chaîne des parents ne doit pas boucler : la règle est appelée
// depuis un rendu de menu, elle ne peut pas se permettre de ne pas rendre la main.
func TestSyncSubtreeTerminatesOnACycle(t *testing.T) {
	nodes := []domain.WorktreeNode{
		{Branch: "a", SourceBranch: "b"},
		{Branch: "b", SourceBranch: "a"},
	}

	got := SyncSubtree(SyncSubtreeParams{Nodes: nodes, Root: "a"})

	if len(got) != 2 {
		t.Fatalf("SyncSubtree on a cycle = %v, want both branches once", got)
	}
}
```

- [ ] **Step 2: Lancer le test — il doit échouer**

Run: `go test ./internal/rules/ -run TestSyncSubtree -v`
Expected: FAIL — `undefined: SyncSubtree`.

- [ ] **Step 3: Implémenter `SyncSubtree`**

```go
package rules

import "github.com/LucasPcq/wtm/internal/domain"

type SyncSubtreeParams struct {
	Nodes []domain.WorktreeNode
	Root  string
}

// SyncSubtree lists Root and every managed worktree hanging under it, parents
// before children. It is what a surface pre-checks when the gesture is "sync this
// worktree": the selection stays exact, only what arrives checked changes.
func SyncSubtree(params SyncSubtreeParams) []string {
	children := make(map[string][]string, len(params.Nodes))
	known := make(map[string]bool, len(params.Nodes))
	for _, node := range params.Nodes {
		known[node.Branch] = true
		children[node.SourceBranch] = append(children[node.SourceBranch], node.Branch)
	}
	if !known[params.Root] {
		return nil
	}

	seen := map[string]bool{params.Root: true}
	subtree := []string{params.Root}
	for index := 0; index < len(subtree); index++ {
		for _, child := range children[subtree[index]] {
			if seen[child] {
				continue
			}
			seen[child] = true
			subtree = append(subtree, child)
		}
	}
	return subtree
}
```

- [ ] **Step 4: Vérifier**

Run: `go test ./internal/rules/ -run TestSyncSubtree -v`
Expected: PASS.

- [ ] **Step 5: Déplacer `SprintSyncPlan` dans `rules`**

Couper `SprintSyncPlan` et `planTitle` de `internal/output/sync.go` vers `internal/rules/sync_plan.go` (renommer `planTitle` en `syncPlanTitle` pour ne pas collisionner dans `rules`), puis laisser dans `output` :

```go
// SprintSyncPlan renders the plan as plain text. The rendering itself is a pure
// transform (rules), so a flow can build the same preview without reaching into
// this package.
func SprintSyncPlan(plan domain.SyncPlan) string { return rules.SprintSyncPlan(plan) }
```

Ajouter l'import `rules` dans `internal/output/sync.go` s'il manque.

- [ ] **Step 6: Vérifier que rien n'a bougé**

Run: `go build ./... && go test ./internal/output/ ./internal/rules/ -v`
Expected: PASS — les tests existants de `output.SprintSyncPlan` passent inchangés à travers la délégation.

- [ ] **Step 7: Commit**

```bash
git add internal/rules/sync_subtree.go internal/rules/sync_subtree_test.go internal/rules/sync_plan.go internal/output/sync.go
git commit -m "refactor(rules): move SprintSyncPlan into rules and add SyncSubtree"
```

---

### Task 4: `flow.ConfirmParams` gagne `YesLabel` / `NoLabel`

La question du push est aujourd'hui une SelectList à deux options nommées, « Keep local » en tête. `Confirm` rend un Yes/No : sans ce champ, la migration changerait le widget.

**Files:**
- Modify: `internal/flow/flow.go` (`ConfirmParams`)
- Modify: `internal/tui/flowui/prompter.go:31-38`
- Modify: `internal/tui/dashboard/prompter.go:78-91`
- Create: `internal/tui/dashboard/prompter_test.go` (ou étendre `modal_test.go` s'il porte déjà les cas de Confirm)

**Interfaces:**
- Produces: `flow.ConfirmParams{Title, Description, Warning, DefaultYes, YesLabel, NoLabel string}`.
- Consumes: `components.NewSelectList`, `components.RunStandaloneSelect`, `components.NewConfirm`.

- [ ] **Step 1: Étendre `ConfirmParams`**

```go
type ConfirmParams struct {
	Title       string
	Description string
	Warning     string
	DefaultYes  bool
	// YesLabel and NoLabel name the two outcomes instead of answering yes or no.
	// A decision whose consequences differ (keeping local vs force-pushing) is
	// asked this way; empty labels keep the plain confirmation both surfaces
	// already render.
	YesLabel string
	NoLabel  string
}
```

- [ ] **Step 2: Rendre les libellés dans `flowui`**

```go
func (Prompter) Confirm(params flow.ConfirmParams) (bool, error) {
	if params.YesLabel == "" {
		return components.RunStandaloneConfirm(components.NewConfirm(components.NewConfirmParams{
			Title:       params.Title,
			Description: params.Description,
			Warning:     params.Warning,
			DefaultYes:  params.DefaultYes,
		}))
	}
	choice, err := components.RunStandaloneSelect(components.NewSelectList(components.NewSelectListParams{
		Title:       params.Title,
		Description: params.Description,
		Items:       confirmItems(params),
	}))
	return err == nil && choice == confirmYesValue, err
}

// confirmItems leads with the outcome that changes nothing unless the caller
// asked for the opposite: a decision with a destructive side is never the
// highlighted default.
func confirmItems(params flow.ConfirmParams) []components.SelectItem {
	yes := components.SelectItem{Label: params.YesLabel, Value: confirmYesValue}
	no := components.SelectItem{Label: params.NoLabel, Value: confirmNoValue}
	if params.DefaultYes {
		return []components.SelectItem{yes, {Separator: true}, no}
	}
	return []components.SelectItem{no, {Separator: true}, yes}
}

const (
	confirmYesValue = "yes"
	confirmNoValue  = "no"
)
```

Attention : `RunStandaloneSelect` rend `("", err)` sur Esc ; l'erreur est propagée telle quelle, comme le fait `confirmPush` aujourd'hui (`err == nil && choice == syncPushDo`). Vérifier dans `internal/tui/components/` la signature exacte et aligner.

- [ ] **Step 3: Rendre les libellés dans le dashboard**

```go
func (p prompter) Confirm(params flow.ConfirmParams) (bool, error) {
	session := flow.Session{Steps: []flow.Step{{
		Kind:        flow.StepRecap,
		Key:         keyConfirm,
		Title:       params.Title,
		Description: confirmDescription(params),
		Options:     confirmOptions(params),
	}}}

	answers, err := prompter{send: p.send, title: params.Title, shape: modalForm}.Ask(session)
	if err != nil {
		return false, err
	}
	return answers.Value(keyConfirm) == confirmYes, nil
}

// confirmOptions names both outcomes when the caller named them: closing the
// modal is a way out, not an answer, so a two-outcome decision has to offer both.
func confirmOptions(params flow.ConfirmParams) []flow.Option {
	if params.YesLabel == "" {
		return []flow.Option{{Label: domain.DashboardConfirmLabel, Value: confirmYes}}
	}
	return []flow.Option{
		{Label: params.NoLabel, Value: confirmNo},
		{Separator: true},
		{Label: params.YesLabel, Value: confirmYes},
	}
}

const confirmNo = "no"
```

- [ ] **Step 4: Écrire le test du dashboard**

```go
func TestConfirmOptionsNameBothOutcomesWhenLabelled(t *testing.T) {
	options := confirmOptions(flow.ConfirmParams{YesLabel: "Push to origin", NoLabel: "Keep local"})

	if len(options) != 3 {
		t.Fatalf("a labelled confirm must offer both outcomes, got %d options", len(options))
	}
	if options[0].Label != "Keep local" || options[2].Label != "Push to origin" {
		t.Fatalf("the harmless outcome must lead, got %+v", options)
	}
}

func TestConfirmOptionsStayPlainWithoutLabels(t *testing.T) {
	options := confirmOptions(flow.ConfirmParams{})

	if len(options) != 1 || options[0].Value != confirmYes {
		t.Fatalf("an unlabelled confirm must keep its single option, got %+v", options)
	}
}
```

- [ ] **Step 5: Vérifier**

Run: `go build ./... && go test ./internal/flow/... ./internal/tui/... -v`
Expected: PASS — `clean` et `create`, seuls appelants existants de `Confirm`, ne passent aucun libellé et ne changent pas.

- [ ] **Step 6: Commit**

```bash
git add internal/flow/flow.go internal/tui/flowui/prompter.go internal/tui/dashboard/prompter.go internal/tui/dashboard/prompter_test.go
git commit -m "feat(flow): let a confirmation name its two outcomes"
```

---

### Task 5: `internal/flow/sync/` — types et steps

**Files:**
- Create: `internal/flow/sync/sync.go` (types uniquement à cette tâche)
- Create: `internal/flow/sync/steps.go`
- Create: `internal/flow/sync/steps_test.go`
- Modify: `internal/domain/constants.go` (les libellés des steps)

**Interfaces:**
- Produces:
  - `type Request struct { Branches []string; All, KeepConflict, FFParents, NoFFParents, Push, NoPush, DryRun bool; Precheck []string; BaseBranch string }`
  - `type Outcome struct { Result domain.SyncResult; Plan domain.SyncPlan; Empty, Aborted bool }`
  - `type Presenter interface { flow.Presenter; Planned(domain.SyncPlan); Rebased(domain.SyncResult); Synced(Outcome) error }`
  - `func Operation() flow.Operation`
  - Clés exportées : `KeySelection = "sync.selection"`, `KeyConflict = "sync.conflict"`, `KeyParents = "sync.parents"`, `KeyConfirm = "sync.confirm"`.
- Consumes: `rules.SprintSyncPlan`, `rules.ParentFlagsDecision`, `worktree.PlanSync`, `worktree.List`, `worktree.StaleParents`.

- [ ] **Step 1: Écrire `sync.go` (types seuls, pas encore `Run`)**

```go
// Package sync runs the `wtm sync` flow.
package sync

import (
	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/flow"
)

type Request struct {
	// Branches and All fix the selection (branch args / --all): the step is not
	// asked, but stays read back in the recap.
	Branches []string
	All      bool
	// Precheck is what arrives checked when the selection step IS asked. A surface
	// that already knows a likely answer offers it; the selection stays exact.
	Precheck     []string
	KeepConflict bool
	FFParents    bool
	NoFFParents  bool
	Push         bool
	NoPush       bool
	// DryRun previews the cascade and rebases nothing.
	DryRun     bool
	BaseBranch string
}

type Outcome struct {
	Result domain.SyncResult
	Plan   domain.SyncPlan
	// Empty reports that the selection resolved to nothing to rebase.
	Empty   bool
	Aborted bool
}

// Presenter carries the three moments of a sync: the plan a run that cannot ask
// has to print, the recap the user reads before being asked to push, and the
// conclusion. They are separate because the push question falls between the last
// two.
type Presenter interface {
	flow.Presenter
	Planned(domain.SyncPlan)
	Rebased(domain.SyncResult)
	Synced(Outcome) error
}

type Params struct {
	Context   flow.Context
	Request   Request
	Prompter  flow.Prompter
	Presenter Presenter
}

// Operation declares how a surface must schedule a sync: it rebases several
// worktrees and asks first, so it holds the surface for its whole run and needs
// no per-worktree lock.
func Operation() flow.Operation {
	return flow.Operation{Kind: domain.OpKindSync, Mode: flow.ModeBlocking}
}

type syncFlow struct {
	ctx       flow.Context
	request   Request
	prompter  flow.Prompter
	presenter Presenter

	statuses   []domain.WorktreeStatus
	classified []domain.ParentUpdate
	// plan is the cascade the recap previews; it is rebuilt for the final
	// selection before the run, because the answers may have narrowed it.
	plan domain.SyncPlan
}
```

Ajouter dans `internal/domain/constants.go`, à côté des autres `OpKind*` et des constantes `Sync*` :

```go
	OpKindSync = "sync"

	// SyncSelectionTitle heads the worktree multi-select; SyncConflictTitle and
	// SyncParentsTitle head the two decisions that follow.
	SyncSelectionTitle   = "Select worktrees to sync"
	SyncConflictTitle    = "On conflict"
	SyncParentsTitle     = "Parent branches"
	SyncConflictNormal   = "Sync normally — abort & keep worktrees clean on conflict"
	SyncConflictKeep     = "Keep conflicts in progress — leave the rebase in its worktree for manual resolution"
	SyncConflictIntro    = "Choose what happens when a rebase hits a conflict."
	SyncCounterFmt       = "About to sync %d worktree(s) onto their parent."
	SyncParentFFOption   = "Fast-forward them — rebase onto the up-to-date parent"
	SyncParentKeepOption = "Leave them as they are — rebase onto the parent as it stands today"
	SyncConfirmOption    = "Yes, sync"
	SyncWizardErrLabel   = "sync"
	SyncNothingToSync    = "No worktrees to sync."
	SyncNeedsTerminal    = "sync needs a terminal to confirm the cascade; pass --yes to run unattended or --dry-run to preview"
	SyncSelectAtLeastOne = "select at least one worktree"
	SyncBaseSuffix       = " (base)"
	SyncTagDirty         = "dirty"
	SyncTagRebasing      = "rebasing"
	SyncKeepConflictHintFmt = "%s left mid-rebase in %s — run `git rebase --continue` or `git rebase --abort` there"
```

Reprendre **verbatim** les libellés déjà présents dans `internal/tui/syncpicker/picker.go` (« Sync normally — … », « Keep conflicts in progress — … », « Fast-forward them — … », « Leave them as they are — … », « Yes, sync », « About to sync %d worktree(s) onto their parent. », « Select worktrees to sync », « select at least one worktree ») : la sortie CLI ne doit pas bouger d'un caractère.

- [ ] **Step 2: Écrire `steps_test.go`**

```go
package sync

import (
	"strings"
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/flow"
)

// testFlow installe un Prompter : session() lit Interactive() pour résoudre la
// question des parents depuis les flags.
func testFlow(request Request, statuses []domain.WorktreeStatus) *syncFlow {
	return &syncFlow{request: request, statuses: statuses, prompter: flow.Unattended{}}
}

var stack = []domain.WorktreeStatus{
	{Branch: "main", IsParent: true},
	{Branch: "feat-a"},
	{Branch: "feat-b", IsDirty: true},
	{Branch: "feat-c", RebaseInProgress: true},
}

func TestSelectionOptionsTagWhatSyncWouldSkip(t *testing.T) {
	content, err := testFlow(Request{}, stack).selectionStep().Build(flow.Answers{})
	if err != nil {
		t.Fatalf("build selection: %v", err)
	}

	byValue := map[string]flow.Option{}
	for _, option := range content.Options {
		byValue[option.Value] = option
	}
	if byValue["main"].Label != "main"+domain.SyncBaseSuffix {
		t.Fatalf("the base must be labelled, got %q", byValue["main"].Label)
	}
	if byValue["feat-b"].Tag != domain.SyncTagDirty {
		t.Fatalf("a dirty worktree must be tagged, got %+v", byValue["feat-b"])
	}
	if byValue["feat-c"].Tag != domain.SyncTagRebasing {
		t.Fatalf("a worktree mid-rebase must be tagged, got %+v", byValue["feat-c"])
	}
}

// Le CLI n'envoie jamais de Precheck : son picker s'ouvre vide, comme aujourd'hui.
func TestSelectionStartsUncheckedWithoutPrecheck(t *testing.T) {
	content, _ := testFlow(Request{}, stack).selectionStep().Build(flow.Answers{})

	for _, option := range content.Options {
		if option.Selected {
			t.Fatalf("%s must not arrive checked without Precheck", option.Value)
		}
	}
}

func TestSelectionPrechecksWhatTheSurfaceAsked(t *testing.T) {
	content, _ := testFlow(Request{Precheck: []string{"feat-a", "feat-b"}}, stack).
		selectionStep().Build(flow.Answers{})

	checked := map[string]bool{}
	for _, option := range content.Options {
		checked[option.Value] = option.Selected
	}
	if !checked["feat-a"] || !checked["feat-b"] || checked["feat-c"] {
		t.Fatalf("Precheck must decide exactly what arrives checked, got %+v", checked)
	}
}

// Le modèle de référence du CLAUDE.md pour l'axe --yes : jamais de picker de repli,
// une erreur qui nomme le flag manquant.
func TestSelectionResolveNamesAll(t *testing.T) {
	_, err := testFlow(Request{}, stack).selectionStep().Resolve(flow.Answers{})

	if err == nil {
		t.Fatal("an unattended run with no selection must refuse")
	}
	if !strings.Contains(err.Error(), "--"+domain.FlagAll) {
		t.Fatalf("the refusal must name --all, got: %v", err)
	}
}

func TestConflictStepDefaultsToAborting(t *testing.T) {
	answer, err := testFlow(Request{}, stack).conflictStep().Resolve(flow.Answers{})
	if err != nil {
		t.Fatalf("resolve conflict: %v", err)
	}
	if answer.Value != conflictNormal {
		t.Fatalf("the safe default aborts the rebase, got %q", answer.Value)
	}
}

func TestConflictStepFollowsTheFlagWhenPreset(t *testing.T) {
	session := testFlow(Request{KeepConflict: true}, stack).session()

	answer, _ := session.Presets.Get(KeyConflict)
	if answer.Value != conflictKeep {
		t.Fatalf("--keep-conflict must preset the step, got %q", answer.Value)
	}
}

func TestParentsStepIsSkippedWhenNothingIsStale(t *testing.T) {
	skip, _ := testFlow(Request{}, stack).parentsStep().Skip(flow.Answers{})

	if !skip {
		t.Fatal("with no stale parent the question must not be asked")
	}
}

func TestRecapDescribesThePlan(t *testing.T) {
	f := testFlow(Request{BaseBranch: "main"}, stack)
	f.plan = domain.SyncPlan{
		BaseBranch:   "main",
		BaseTargeted: true,
		Steps:        []domain.SyncStep{{Branch: "feat-a", SourceBranch: "main"}},
	}

	content, err := f.confirmStep().Build(flow.Answers{})
	if err != nil {
		t.Fatalf("build recap: %v", err)
	}
	if !strings.Contains(content.Description, "feat-a") {
		t.Fatalf("the recap must carry the plan, got: %q", content.Description)
	}
}
```

- [ ] **Step 3: Lancer — doit échouer**

Run: `go test ./internal/flow/sync/ -v`
Expected: FAIL — `selectionStep`, `conflictStep`, `parentsStep`, `confirmStep`, `session` non définis.

- [ ] **Step 4: Écrire `steps.go`**

Squelette à compléter en suivant `internal/flow/prune/steps.go` :

```go
package sync

import (
	"errors"
	"fmt"
	"strings"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/flow"
	"github.com/LucasPcq/wtm/internal/rules"
	"github.com/LucasPcq/wtm/internal/service/worktree"
)

const (
	KeySelection = "sync.selection"
	KeyConflict  = "sync.conflict"
	KeyParents   = "sync.parents"
	KeyConfirm   = "sync.confirm"
)

const (
	conflictNormal = "normal"
	conflictKeep   = "keep"
	parentFF       = "fast-forward"
	parentKeep     = "keep"
	confirmSync    = "sync"
)

const (
	labelSelection = "Worktrees"
	labelConflict  = "On conflict"
	labelParents   = "Parent branches"
	labelConfirm   = "Confirm"
)

func (f *syncFlow) session() flow.Session {
	presets := flow.NewAnswers(map[string]string{KeyConflict: f.presetConflict(), KeyParents: f.presetParents()})
	return flow.Session{
		ErrLabel: domain.SyncWizardErrLabel,
		Presets:  presets.WithValues(KeySelection, f.fixedSelection()),
		Steps:    []flow.Step{f.selectionStep(), f.conflictStep(), f.parentsStep(), f.confirmStep()},
	}
}

// fixedSelection is what args or --all already settled. --all previews the
// resolved list; the service still receives nil, which is what "every worktree"
// means to it (see rules.SyncIncludesBase).
func (f *syncFlow) fixedSelection() []string {
	if f.request.All {
		return f.syncableBranches()
	}
	return f.request.Branches
}

func (f *syncFlow) selectionStep() flow.Step {
	return flow.Step{
		Kind:  flow.StepMultiSelect,
		Key:   KeySelection,
		Label: labelSelection,
		Build: func(flow.Answers) (flow.StepContent, error) {
			return flow.StepContent{
				Title:       domain.SyncSelectionTitle,
				Description: domain.MultiSelectHint,
				Options:     f.selectionOptions(),
			}, nil
		},
		ValidateSet: func(values []string) error {
			if len(values) == 0 {
				return errors.New(domain.SyncSelectAtLeastOne)
			}
			return nil
		},
		Resolve: func(flow.Answers) (flow.Answer, error) {
			return flow.Answer{}, fmt.Errorf(domain.SyncSelectionRequiredFmt,
				domain.FlagAll, domain.FlagYes, domain.FlagOutput, domain.OutputJSON)
		},
		Summarize: summarizeSelection,
	}
}
```

Reprendre le message d'erreur **verbatim** de `resolveSyncSelection` (`internal/commands/wt/sync.go`) dans une constante `domain.SyncSelectionRequiredFmt` : le test de caractérisation `TestSyncYesWithoutTargetNamesAll` le vérifie.

Les trois autres steps et les helpers d'options :

```go
func (f *syncFlow) selectionOptions() []flow.Option {
	prechecked := make(map[string]bool, len(f.request.Precheck))
	for _, branch := range f.request.Precheck {
		prechecked[branch] = true
	}

	options := make([]flow.Option, 0, len(f.statuses))
	for _, status := range f.statuses {
		tag, tone := statusTag(status)
		options = append(options, flow.Option{
			Label:    statusLabel(status),
			Value:    status.Branch,
			Selected: prechecked[status.Branch],
			Tag:      tag,
			Tone:     tone,
		})
	}
	return options
}

// statusLabel tells the base apart from the worktrees that hang off it.
func statusLabel(status domain.WorktreeStatus) string {
	if status.IsParent {
		return status.Branch + domain.SyncBaseSuffix
	}
	return status.Branch
}

// statusTag names what the cascade would skip, so a worktree left out is left
// out for a reason the user can read.
func statusTag(status domain.WorktreeStatus) (string, domain.Tone) {
	switch {
	case status.RebaseInProgress:
		return domain.SyncTagRebasing, domain.ToneWarning
	case status.IsDirty:
		return domain.SyncTagDirty, domain.ToneWarning
	}
	return "", domain.ToneNeutral
}

func summarizeSelection(answer flow.Answer) string {
	values := answer.Values
	if len(values) == 0 {
		return "none"
	}
	const maxNames = 5
	if len(values) <= maxNames {
		return strings.Join(values, ", ")
	}
	return strings.Join(values[:maxNames], ", ") + fmt.Sprintf(" +%d", len(values)-maxNames)
}

// syncableBranches is the explicit list --all previews. The service still
// receives nil, which is what "every worktree" means to it.
func (f *syncFlow) syncableBranches() []string {
	branches := make([]string, 0, len(f.statuses))
	for _, status := range f.statuses {
		if status.IsParent {
			continue
		}
		branches = append(branches, status.Branch)
	}
	return branches
}

// conflictStep is skipped when nothing is rebased: a base-only refresh has no
// conflict to have an opinion about.
func (f *syncFlow) conflictStep() flow.Step {
	return flow.Step{
		Kind:  flow.StepSelect,
		Key:   KeyConflict,
		Label: labelConflict,
		Skip: func(answers flow.Answers) (bool, string) {
			return len(f.planFor(answers).Steps) == 0, domain.SyncNoRebaseStep
		},
		Build: func(answers flow.Answers) (flow.StepContent, error) {
			return flow.StepContent{
				Title:       domain.SyncConflictTitle,
				Description: conflictDescription(len(answers.Values(KeySelection))),
				Options: []flow.Option{
					{Label: domain.SyncConflictNormal, Value: conflictNormal},
					{Label: domain.SyncConflictKeep, Value: conflictKeep, Danger: true},
				},
			}, nil
		},
		Resolve: func(flow.Answers) (flow.Answer, error) {
			return flow.Answer{Value: conflictNormal}, nil
		},
		Summarize: func(answer flow.Answer) string {
			if answer.Value == conflictKeep {
				return domain.SyncConflictKeepSummary
			}
			return domain.SyncConflictNormalSummary
		},
		Flag: domain.FlagKeepConflict,
	}
}

// conflictDescription names no branch: each worktree is rebased onto its own
// recorded parent, which the base only coincides with at the first level.
func conflictDescription(count int) string {
	if count == 0 {
		return domain.SyncConflictIntro
	}
	return fmt.Sprintf(domain.SyncCounterFmt, count) + "\n\n" + domain.SyncConflictIntro
}

func (f *syncFlow) presetConflict() string {
	if f.request.KeepConflict {
		return conflictKeep
	}
	return ""
}

// parentsStep asks about the parents no step covers. It is skipped when the
// selection leaves none of them behind their remote.
func (f *syncFlow) parentsStep() flow.Step {
	return flow.Step{
		Kind:  flow.StepSelect,
		Key:   KeyParents,
		Label: labelParents,
		Skip: func(answers flow.Answers) (bool, string) {
			return len(f.staleParents(answers)) == 0, domain.SyncNoStaleParent
		},
		Build: func(answers flow.Answers) (flow.StepContent, error) {
			return flow.StepContent{
				Title:       domain.SyncParentsTitle,
				Description: parentLines(f.staleParents(answers)) + "\n\n" + domain.SyncParentDescription,
				Options: []flow.Option{
					{Label: domain.SyncParentFFOption, Value: parentFF},
					{Label: domain.SyncParentKeepOption, Value: parentKeep},
				},
			}, nil
		},
		Resolve: func(flow.Answers) (flow.Answer, error) {
			return flow.Answer{Value: parentKeep}, nil
		},
		Summarize: func(answer flow.Answer) string {
			if answer.Value == parentFF {
				return domain.SyncParentFFSummary
			}
			return domain.SyncParentKeepSummary
		},
		Flag: domain.FlagFFParents,
	}
}

func parentLines(parents []domain.ParentUpdate) string {
	lines := make([]string, 0, len(parents))
	for _, parent := range parents {
		lines = append(lines, fmt.Sprintf(domain.SyncParentLineFmt,
			parent.Branch, rules.CommitCountLabel(parent.Behind),
			domain.RemoteBranchPrefix, parent.Branch,
			strings.Join(parent.Children, ", ")))
	}
	return strings.Join(lines, "\n")
}

// presetParents settles the question from the flags, through the same rule the
// command used to call. The step stays listed so a flag never makes a recap line
// disappear.
func (f *syncFlow) presetParents() string {
	switch rules.ParentFlagsDecision(rules.DecideParentFastForwardParams{
		FF:          f.request.FFParents,
		NoFF:        f.request.NoFFParents,
		Interactive: f.prompter.Interactive(),
	}) {
	case rules.ParentFastForward:
		return parentFF
	case rules.ParentAsk:
		return ""
	default:
		return parentKeep
	}
}

// confirmStep previews the cascade. Building the plan walks every selected
// worktree's history, so it goes through Load — off the UI goroutine, behind a
// spinner — exactly like clean's delete step.
func (f *syncFlow) confirmStep() flow.Step {
	content := func(answers flow.Answers) (flow.StepContent, error) {
		plan := f.planFor(answers)
		f.plan = plan
		return flow.StepContent{
			Title:       domain.SyncConfirmTitle,
			Description: confirmDescription(plan, answers.Value(KeyConflict) == conflictKeep),
			Options: []flow.Option{
				{Label: domain.SyncConfirmOption, Value: confirmSync},
				{Separator: true},
				{Label: domain.WizardCancelLabel, Value: domain.WizardCancelValue},
			},
		}, nil
	}

	step := flow.Step{
		Kind:           flow.StepRecap,
		Key:            KeyConfirm,
		Label:          labelConfirm,
		Title:          domain.SyncConfirmTitle,
		LoadingMessage: domain.SyncPlanComputing,
		Resolve: func(flow.Answers) (flow.Answer, error) {
			return flow.Answer{Value: confirmSync}, nil
		},
	}
	// A plan already computed for a fixed selection needs no I/O to be shown again.
	if len(f.plan.Steps) > 0 {
		step.Build = content
		return step
	}
	step.Load = content
	return step
}

func confirmDescription(plan domain.SyncPlan, keepConflict bool) string {
	description := fmt.Sprintf(domain.SyncConfirmPrompt, len(plan.Steps))
	if text := rules.SprintSyncPlan(plan); text != "" {
		description = text + "\n\n" + description
	}
	if keepConflict {
		description += "\n\n⚠ " + domain.SyncKeepConflictWarning
	}
	return description
}

type syncParamsInput struct {
	Selected     []string
	KeepConflict bool
	FastForward  bool
}

// syncParams is the one place the request becomes service inputs. --all keeps its
// meaning by passing nil: the service reads an empty selection as "every worktree".
func (f *syncFlow) syncParams(input syncParamsInput) worktree.SyncParams {
	selected := input.Selected
	if f.request.All {
		selected = nil
	}
	return worktree.SyncParams{
		ProjectDir:       f.ctx.ProjectDir,
		StateDir:         f.ctx.StateDir,
		Config:           f.ctx.Config,
		BaseBranch:       f.request.BaseBranch,
		DryRun:           f.request.DryRun,
		KeepConflict:     input.KeepConflict,
		SelectedBranches: selected,
		// Dry-run stays offline, so it never refreshes a parent whatever was asked.
		FastForwardParents: input.FastForward && !f.request.DryRun,
	}
}

// planFor previews the cascade for the selection as it stands. Conflict mode does
// not affect the plan, so it is not read here.
func (f *syncFlow) planFor(answers flow.Answers) domain.SyncPlan {
	plan, err := worktree.PlanSync(f.syncParams(syncParamsInput{Selected: answers.Values(KeySelection)}))
	if err != nil {
		return domain.SyncPlan{}
	}
	return plan
}

// staleParents narrows the inspection to the current selection. The scan itself
// ran once, before the session; nil means the run never inspected them.
func (f *syncFlow) staleParents(answers flow.Answers) []domain.ParentUpdate {
	if len(f.classified) == 0 {
		return nil
	}
	return worktree.StaleParents(worktree.StaleParentsParams{
		Sync:       f.syncParams(syncParamsInput{Selected: answers.Values(KeySelection)}),
		Branches:   answers.Values(KeySelection),
		Classified: f.classified,
	})
}
```

À vérifier en écrivant :
- Les libellés `Sync*Summary`, `SyncNoRebaseStep`, `SyncNoStaleParent`, `SyncConfirmTitle`, `SyncParentLineFmt` sont à ajouter dans `domain/constants.go`, repris **verbatim** de `syncpicker` (`conflictSummary`, `parentSummary`, `parentLines`).

- [ ] **Step 5: Vérifier**

Run: `go test ./internal/flow/sync/ -v`
Expected: PASS.

- [ ] **Step 6: Vérifier la règle d'import de `flow/`**

Run: `go list -deps ./internal/flow/sync/ | grep -E 'wtm/internal/(output|tui|commands|config)' && echo VIOLATION || echo OK`
Expected: `OK`.

- [ ] **Step 7: Commit**

```bash
git add internal/flow/sync internal/domain/constants.go
git commit -m "feat(flow): declare the sync session and its four steps"
```

---

### Task 6: `internal/flow/sync/` — le run

**Files:**
- Modify: `internal/flow/sync/sync.go` (ajoute `Run` et le pipeline)
- Modify: `internal/flow/sync/steps.go` (les helpers `planFor`, `staleParents`, `syncableBranches`)
- Create: `internal/flow/sync/sync_test.go`

**Interfaces:**
- Consumes: `flowtest.ScriptedPrompter`, `flowtest` Presenter (lire `internal/testutil/flowtest/flowtest.go` pour le nom exact et l'étendre avec `Planned`/`Rebased`/`Synced` via un double local au package de test si le double partagé ne porte que `flow.Presenter`).
- Produces: `func Run(params Params) (Outcome, error)`.

- [ ] **Step 1: Écrire `Run`**

```go
func Run(params Params) (Outcome, error) {
	f := &syncFlow{
		ctx:       params.Context,
		request:   params.Request,
		prompter:  params.Prompter,
		presenter: params.Presenter,
	}
	return f.run()
}

func (f *syncFlow) run() (Outcome, error) {
	if err := f.load(); err != nil {
		return Outcome{}, err
	}

	answers, err := f.prompter.Ask(f.session())
	if errors.Is(err, domain.ErrUserAborted) {
		f.presenter.Notice(flow.AbortedNotice)
		return Outcome{Aborted: true}, nil
	}
	if err != nil {
		return Outcome{}, err
	}
	if answers.Value(KeyConfirm) == domain.WizardCancelValue {
		f.presenter.Notice(flow.AbortedNotice)
		return Outcome{Aborted: true}, nil
	}

	selected := answers.Values(KeySelection)
	syncParams := f.syncParams(syncParamsInput{
		Selected:     selected,
		KeepConflict: answers.Value(KeyConflict) == conflictKeep,
		FastForward:  answers.Value(KeyParents) == parentFF,
	})

	plan, err := worktree.PlanSync(syncParams)
	if err != nil {
		return Outcome{}, err
	}
	if len(plan.Steps) == 0 && !rules.SyncIncludesBase(rules.SyncIncludesBaseParams{
		Selected: syncParams.SelectedBranches, BaseBranch: f.request.BaseBranch}) {
		return f.conclude(Outcome{Empty: true})
	}

	// A run that could not ask never saw the plan in a recap, so it is printed
	// here — which is the whole of the former two-confirmation-sites gymnastics.
	if !f.prompter.Interactive() {
		f.presenter.Planned(plan)
	}

	var result domain.SyncResult
	if err := f.presenter.Stage(flow.StageParams{
		Message: domain.SyncRebasing,
		Work:    func() error { var e error; result, e = worktree.Sync(syncParams); return e },
	}); err != nil {
		return Outcome{}, err
	}
	f.presenter.Rebased(result)

	if !f.request.DryRun && f.shouldPush(result) {
		if err := f.presenter.Stage(flow.StageParams{
			Message: domain.SyncPushing,
			Work: func() error {
				result = worktree.PushSynced(worktree.PushSyncedParams{
					ProjectDir: f.ctx.ProjectDir,
					Result:     result,
				})
				return nil
			},
		}); err != nil {
			return Outcome{}, err
		}
	}

	outcome, err := f.conclude(Outcome{Result: result, Plan: plan})
	if err != nil {
		return outcome, err
	}
	if !f.request.DryRun && rules.HasSyncFailure(result.Steps) {
		return outcome, domain.ErrAborted
	}
	return outcome, nil
}

// shouldPush resolves the pure decision and, when it asks, asks after the run —
// the user reads what happened before deciding to publish it.
func (f *syncFlow) shouldPush(result domain.SyncResult) bool {
	ready := rules.PushableCount(result.Steps)
	switch rules.DecidePush(rules.DecidePushParams{
		Push:          f.request.Push,
		NoPush:        f.request.NoPush,
		Interactive:   f.prompter.Interactive(),
		PushableCount: ready,
	}) {
	case rules.PushForce:
		return true
	case rules.PushConfirm:
		confirmed, err := f.prompter.Confirm(flow.ConfirmParams{
			Title:       fmt.Sprintf(domain.SyncPushPrompt, ready),
			Description: domain.SyncPushWarning,
			YesLabel:    domain.SyncPushOption,
			NoLabel:     domain.SyncKeepLocalOption,
		})
		return err == nil && confirmed
	default:
		return false
	}
}

func (f *syncFlow) conclude(outcome Outcome) (Outcome, error) {
	return outcome, f.presenter.Synced(outcome)
}
```

`load()` et `syncParams()` :

`syncParams` et `syncParamsInput` ont été écrits en tâche 5 (`steps.go` les appelle) : ne pas les réécrire ici.

```go
// load reads what the session needs before it can ask anything. The parent scan
// is I/O, so only the run that could actually ask pays for it; every other outcome
// is settled by the flags alone.
func (f *syncFlow) load() error {
	statuses, err := worktree.List(domain.ListParams{
		ProjectDir: f.ctx.ProjectDir,
		StateDir:   f.ctx.StateDir,
		Config:     f.ctx.Config,
	})
	if err != nil {
		return err
	}
	f.statuses = statuses

	if len(f.request.Branches) > 0 {
		resolved, resolveErr := worktree.ResolveSyncBranches(worktree.ResolveSyncBranchesParams{
			ProjectDir: f.ctx.ProjectDir,
			Queries:    f.request.Branches,
		})
		if resolveErr != nil {
			return resolveErr
		}
		f.request.Branches = resolved
	}

	if !f.prompter.Interactive() || f.request.DryRun {
		return nil
	}
	return f.presenter.Stage(flow.StageParams{
		Message: domain.SyncParentScanning,
		Work: func() error {
			classified, scanErr := worktree.ClassifyParents(worktree.ClassifyParentsParams{
				ProjectDir: f.ctx.ProjectDir,
				StateDir:   f.ctx.StateDir,
				BaseBranch: f.request.BaseBranch,
			})
			f.classified = classified
			return scanErr
		},
	})
}
```

Attention : `f.request.Branches` est réassigné dans `load()`. La règle d'immutabilité du CLAUDE.md interdit la réassignation dans un bloc — extraire la résolution dans une méthode `resolvedBranches() ([]string, error)` et stocker le résultat dans un champ `selection []string` du `syncFlow`, lu ensuite par `fixedSelection()`.

Ajouter les constantes manquantes dans `domain` : `SyncRebasing = "Rebasing worktrees…"`, `SyncPushing = "Pushing to origin…"`, `SyncPushOption = "Push to origin"`, `SyncKeepLocalOption = "Keep local"` (verbatim depuis `commands/wt/sync.go`).

- [ ] **Step 2: Écrire `sync_test.go`**

```go
package sync

import (
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/flow"
)

// recordingPresenter capte les trois moments dans l'ordre où ils tombent : c'est
// l'ordre de sortie que la migration doit préserver.
type recordingPresenter struct {
	order   []string
	planned domain.SyncPlan
}

func (p *recordingPresenter) Stage(params flow.StageParams) error {
	p.order = append(p.order, "stage:"+params.Message)
	return params.Work()
}
func (p *recordingPresenter) HookPhase(flow.HookPhaseParams) error { return nil }
func (p *recordingPresenter) Notice(flow.Notice)                   {}
func (p *recordingPresenter) Status(flow.Notice)                   {}
func (p *recordingPresenter) Planned(plan domain.SyncPlan) {
	p.order = append(p.order, "planned")
	p.planned = plan
}
func (p *recordingPresenter) Rebased(domain.SyncResult)  { p.order = append(p.order, "rebased") }
func (p *recordingPresenter) Synced(Outcome) error       { p.order = append(p.order, "synced"); return nil }

func TestOperationBlocksTheWholeSurface(t *testing.T) {
	op := Operation()

	if op.Mode != flow.ModeBlocking {
		t.Fatalf("sync must hold the surface, got mode %v", op.Mode)
	}
	if op.TargetKey != "" {
		t.Fatalf("holding everything, sync needs no per-worktree lock, got %q", op.TargetKey)
	}
	if op.Kind != domain.OpKindSync {
		t.Fatalf("Operation kind = %q, want %q", op.Kind, domain.OpKindSync)
	}
}
```

Les cas de bout en bout (ordre `planned` → `rebased` → `synced`, abandon, push) sont couverts par les tests de caractérisation CLI de la tâche 7 : ce package n'a besoin ici que de figer `Operation()` et de compiler contre le double.

- [ ] **Step 3: Vérifier**

Run: `go test ./internal/flow/sync/ -v && go vet ./internal/flow/...`
Expected: PASS.

- [ ] **Step 4: Commit**

```bash
git add internal/flow/sync internal/domain/constants.go
git commit -m "feat(flow): run the sync cascade through the flow layer"
```

---

### Task 7: Rebrancher `commands/wt/sync.go`

**Files:**
- Modify: `internal/commands/wt/sync.go` (passe de ~524 à ~110 lignes)
- Modify: `internal/commands/wt/presenter.go` (ajoute `syncPresenter`)
- Modify: `internal/commands/wt/sync_test.go` (les helpers testés y disparaissent avec leur code)
- Create: `internal/commands/wt/sync_terminal_test.go` (le seul comportement changé, D7)

**Interfaces:**
- Consumes: `syncflow.Run`, `syncflow.Request`, `syncflow.Outcome`, `flowContext`, `flowPrompter`, `newPresenter`.
- Produces: rien pour les tâches suivantes.

- [ ] **Step 1: Réécrire `runSync`**

Ne restent dans `commands/` que : la lecture des flags, les trois refus (`--push`/`--no-push`, `--ff-parents`/`--no-ff-parents`, `--all` + args), le refus JSON sans `--yes`/`--dry-run`, le nouveau refus D7, `resolveBase`, et l'appel au flow.

```go
	interactive := rules.IsHumanFormat(format) && term.IsTerminal(int(os.Stdin.Fd())) && !yes && !dryRun
	if !interactive && !yes && !dryRun && rules.IsHumanFormat(format) {
		return errors.New(domain.SyncNeedsTerminal)
	}

	_, err = syncflow.Run(syncflow.Params{
		Context: flowContext(cfg),
		Request: syncflow.Request{
			Branches:     args,
			All:          all,
			KeepConflict: keepConflict,
			FFParents:    ffParents,
			NoFFParents:  noFFParents,
			Push:         push,
			NoPush:       noPush,
			DryRun:       dryRun,
			BaseBranch:   resolveBase(baseOverride, cfg),
		},
		// Le picker peut être atteint à travers le wrapper shell, qui consomme stdout.
		Prompter:  flowPrompter(flowPrompterParams{Interactive: interactive, Stderr: true}),
		Presenter: syncPresenter{cliPresenter: newPresenter(cmd, format)},
	})
	return err
```

`Request.Branches` reçoit les arguments bruts : la résolution `worktree.ResolveSyncBranches` (aujourd'hui dans `resolveSyncSelection`) descend dans `f.load()` du flow.

Supprimer de `sync.go` : `resolveSyncSelection`, `syncSelection`, `resolveSyncSelectionParams`, `branchesForSync`, `pickerPreselection`, `syncableBranches`, `renderEmptyPlan`, `confirmSync`, `shouldPush`, `confirmPush`, `parentPreset`, `resolveFastForwardParents`, et les imports devenus inutiles (`syncpicker`, `components`, `output`, `worktree`, `detect` si `resolveBase` bouge — il reste, `prune.go` l'appelle).

- [ ] **Step 2: Écrire `syncPresenter`**

```go
type syncPresenter struct {
	cliPresenter
}

// Planned prints the cascade a run that could not ask never saw in a recap. It
// opens the frame on stderr, where the plan has always been written.
func (p syncPresenter) Planned(plan domain.SyncPlan) {
	if !p.human {
		return
	}
	output.FrameStart(p.cmd.ErrOrStderr())
	output.FormatSyncPlan(p.cmd.ErrOrStderr(), plan)
}

// Rebased is the recap the user reads BEFORE being asked to push. Its single
// leading blank separates the plan/spinner section (stderr) from the recap.
func (p syncPresenter) Rebased(result domain.SyncResult) {
	if !p.human {
		return
	}
	output.Blank(p.cmd.OutOrStdout())
	output.FormatSyncResult(p.cmd.OutOrStdout(), result)
}

func (p syncPresenter) Synced(outcome syncflow.Outcome) error {
	if p.format == domain.OutputJSON {
		return output.WriteSyncResultJSON(p.cmd.OutOrStdout(), outcome.Result)
	}
	if outcome.Empty {
		output.Frame(p.cmd.OutOrStdout(), func() {
			output.Message(p.cmd.OutOrStdout(), domain.SyncNothingToSync)
		})
		return nil
	}
	output.FormatSyncPushSummary(p.cmd.OutOrStdout(), outcome.Result.Steps)
	output.FrameEnd(p.cmd.OutOrStdout())
	return nil
}
```

Attention au cas `Empty` en JSON : aujourd'hui `renderEmptyPlan` écrit `domain.SyncResult{BaseBranch: base}`. Reprendre ce payload exact (l'`Outcome.Result` est alors le zéro : renseigner `BaseBranch` dans l'`Outcome` côté flow, ou le remplir ici depuis `outcome.Plan.BaseBranch`).

- [ ] **Step 3: Faire passer la caractérisation**

Run: `go test ./internal/commands/wt/ -run 'TestSync' -v`
Expected: PASS — **sans modifier une seule ligne** de `sync_characterization_test.go`. Toute différence est une régression de la migration.

- [ ] **Step 4: Nettoyer `sync_test.go`**

Supprimer `TestResolveSyncSelectionRequiresTargetWhenNoPrompt`, `TestBranchesForSync`, `TestSyncableBranchesExcludesBase`, `TestPickerPreselection` : leurs sujets ont déménagé (le premier est couvert par `TestSelectionResolveNamesAll` dans `flow/sync`, le troisième par les options du step). Si le fichier devient vide, le supprimer.

- [ ] **Step 5: Écrire le test du seul comportement changé (D7)**

```go
// Sans TTY, en sortie humaine et sans --yes ni --dry-run, sync refuse au lieu de
// tenter une confirmation qui ne peut pas s'afficher. C'est le modèle de prune
// (PruneNeedsTerminal), et le seul comportement que la migration change.
func TestSyncWithoutTerminalRefuses(t *testing.T) {
	setupSync(t, syncSetup{Stack: []string{"feat-a:main"}})

	_, _, err := runWtCmd(t, domain.CmdSync, "feat-a")
	if err == nil {
		t.Fatal("sync without a terminal and without --yes must be refused")
	}
	if !strings.Contains(err.Error(), "--"+domain.FlagYes) {
		t.Fatalf("the refusal must name --yes, got: %v", err)
	}
}
```

- [ ] **Step 6: Vérifier l'ensemble**

Run: `go build ./... && go vet ./... && go test ./internal/... `
Expected: PASS.

- [ ] **Step 7: Commit**

```bash
git add internal/commands/wt/sync.go internal/commands/wt/presenter.go internal/commands/wt/sync_test.go internal/commands/wt/sync_terminal_test.go
git commit -m "refactor(sync): drive the command through internal/flow/sync"
```

---

### Task 8: Supprimer `internal/tui/syncpicker`

**Files:**
- Delete: `internal/tui/syncpicker/picker.go`
- Delete: `internal/tui/syncpicker/picker_test.go`

- [ ] **Step 1: Vérifier qu'il n'a plus d'appelant**

Run: `grep -rn "syncpicker" --include '*.go' . || echo "aucun appelant"`
Expected: `aucun appelant`.

- [ ] **Step 2: Supprimer le package**

```bash
git rm -r internal/tui/syncpicker
```

- [ ] **Step 3: Vérifier**

Run: `go build ./... && go test ./internal/...`
Expected: PASS.

- [ ] **Step 4: Commit**

```bash
git commit -m "refactor(tui): drop syncpicker and its injected PlanPreview closure"
```

---

### Task 9: Les trois entrées du dashboard

**Files:**
- Modify: `internal/tui/dashboard/actions.go` (trois `start*`)
- Modify: `internal/tui/dashboard/menu.go` (`menuAction`, `worktreeMenuItems`, `globalMenuItems`, `activateMenu`)
- Modify: `internal/tui/dashboard/presenter.go` (`syncPresenter`)
- Modify: `internal/tui/dashboard/prompter.go` (`syncedMsg`)
- Modify: `internal/tui/dashboard/menu_test.go`, `internal/tui/dashboard/actions_test.go`
- Modify: `internal/domain/constants.go` (libellés de menu et titres de modale)

**Interfaces:**
- Consumes: `syncflow.Run/Request/Operation/Outcome`, `rules.SyncSubtree`, `worktree.Nodes`.
- Produces: `menuSync`, `menuSyncAll`, `menuRefreshBase` (valeurs de `menuAction`).

- [ ] **Step 1: Ajouter les constantes**

```go
	DashboardMenuSync        = "Sync this worktree"
	DashboardMenuSyncAll     = "Sync all worktrees"
	DashboardMenuRefreshBase = "Refresh base branch"
	DashboardSyncTitle       = "Sync worktrees"
	DashboardRefreshBaseTitle = "Refresh base branch"
```

- [ ] **Step 2: Écrire les tests de menu**

```go
func TestWorktreeMenuLeadsWithSync(t *testing.T) {
	m := modelWithWorktrees(t, []domain.WorktreeStatus{{Branch: "feat-a"}})

	items := m.worktreeMenuItems()

	if len(items) == 0 || items[0].action != menuSync {
		t.Fatalf("the row menu must lead with sync, got %+v", items)
	}
}

// La ligne de la base n'avait aucun menu : elle en gagne un, à une seule entrée.
func TestParentRowOffersRefreshBaseOnly(t *testing.T) {
	m := modelWithWorktrees(t, []domain.WorktreeStatus{{Branch: "main", IsParent: true}})

	items := m.worktreeMenuItems()

	if len(items) != 1 || items[0].action != menuRefreshBase {
		t.Fatalf("the base row must offer exactly the base refresh, got %+v", items)
	}
}

func TestGlobalMenuOffersSyncAll(t *testing.T) {
	m := modelWithWorktrees(t, []domain.WorktreeStatus{{Branch: "feat-a"}})

	found := false
	for _, item := range m.globalMenuItems() {
		if item.action == menuSyncAll {
			found = true
		}
	}
	if !found {
		t.Fatal("the Actions menu must offer syncing every worktree")
	}
}
```

`modelWithWorktrees` : reprendre le helper déjà présent dans `internal/tui/dashboard/menu_test.go` (lire le fichier ; s'il porte un autre nom, l'utiliser tel quel plutôt que d'en créer un second).

- [ ] **Step 3: Lancer — doit échouer**

Run: `go test ./internal/tui/dashboard/ -run 'Menu' -v`
Expected: FAIL — `menuSync`, `menuSyncAll`, `menuRefreshBase` non définis.

- [ ] **Step 4: Câbler le menu**

Ajouter les trois valeurs à `menuAction`, mettre `menuSync` en tête de `worktreeMenuItems` (avant `Change parent`), retirer le retour `nil` pour `selected.IsParent` et rendre à la place l'entrée unique `menuRefreshBase`, ajouter `menuSyncAll` à `globalMenuItems` (avant `Prune`, non `danger`), et router les trois dans `activateMenu`.

- [ ] **Step 5: Écrire les trois `start*`**

```go
// startSync pre-checks the row's own subtree: the cascade rebases a worktree and
// what hangs under it, so that is what the gesture offers — the selection itself
// stays exact and the user can uncheck.
func (m Model) startSync(branch string) (Model, tea.Cmd) {
	return m.runSync(runSyncParams{
		Title:    domain.DashboardSyncTitle,
		Precheck: rules.SyncSubtree(rules.SyncSubtreeParams{Nodes: m.nodes, Root: branch}),
	})
}

// startSyncAll offers every worktree but leaves unchecked the ones a rebase would
// skip anyway; they stay listed, with the tag that says why.
func (m Model) startSyncAll() (Model, tea.Cmd) {
	return m.runSync(runSyncParams{
		Title:    domain.DashboardSyncTitle,
		Precheck: m.syncablePrecheck(),
	})
}

// startRefreshBase fast-forwards the base alone: no rebase step, so the flow skips
// both the conflict and the parent questions and goes straight to the recap.
func (m Model) startRefreshBase() (Model, tea.Cmd) {
	return m.runSync(runSyncParams{
		Title:    domain.DashboardRefreshBaseTitle,
		Branches: []string{m.params.Config.Project.Worktrees.BaseBranch},
	})
}

type runSyncParams struct {
	Title    string
	Branches []string
	Precheck []string
}

func (m Model) runSync(params runSyncParams) (Model, tea.Cmd) {
	if reason, refused := m.busyReason(""); refused {
		return m.refuse(reason), nil
	}
	m, id := m.beginOp(beginParams{Operation: syncflow.Operation()})
	send := m.sender()

	flowParams := syncflow.Params{
		Context: m.flowContext(),
		Request: syncflow.Request{
			Branches:   params.Branches,
			Precheck:   params.Precheck,
			BaseBranch: m.params.Config.Project.Worktrees.BaseBranch,
		},
		Prompter: prompter{
			send:  send,
			title: params.Title,
			shape: modalStepper,
			opID:  id,
		},
		Presenter: syncPresenter{presenter{send: send}},
	}

	return m, func() tea.Msg {
		_, err := syncflow.Run(flowParams)
		return opDoneMsg{id: id, err: err}
	}
}

// syncablePrecheck leaves out what the cascade would skip: a dirty worktree and
// one already mid-rebase.
func (m Model) syncablePrecheck() []string {
	branches := make([]string, 0, len(m.worktrees))
	for _, status := range m.worktrees {
		if status.IsParent || status.IsDirty || status.RebaseInProgress {
			continue
		}
		branches = append(branches, status.Branch)
	}
	return branches
}
```

`m.nodes` : le modèle porte déjà la forêt pour l'onglet Tree — lire `internal/tui/dashboard/dashboard.go` et réutiliser ce qui existe (`m.treeRows` porte des `domain.TreeNode`). Si aucun `[]domain.WorktreeNode` n'est disponible, dériver le sous-arbre depuis les `TreeNode` plutôt que de recharger : la règle prend `[]domain.WorktreeNode`, donc convertir sur place (`Branch`, `SourceBranch`) dans un petit helper `nodesFromTree`.

- [ ] **Step 6: Écrire le `syncPresenter` du dashboard**

```go
type syncPresenter struct{ presenter }

func (p syncPresenter) Planned(domain.SyncPlan) {}

func (p syncPresenter) Rebased(result domain.SyncResult) {
	for _, step := range result.Steps {
		p.line(fmt.Sprintf(domain.DashboardSyncStepFmt, step.Branch, rules.SyncStatusLabel(step.Status)))
	}
}

func (p syncPresenter) Synced(outcome syncflow.Outcome) error {
	if outcome.Empty {
		p.line(domain.SyncNothingToSync)
		return nil
	}
	// Un rebase laissé en cours ne se résout pas depuis cette surface : le chemin
	// et les deux commandes qui en sortent sont nommés, comme pour la suppression
	// privilégiée.
	for _, step := range outcome.Result.Steps {
		if step.Status != domain.SyncStatusConflict || !step.KeptInProgress {
			continue
		}
		p.line(fmt.Sprintf(domain.SyncKeepConflictHintFmt, step.Branch, step.Path))
	}
	p.send(syncedMsg{})
	return nil
}
```

`rules.SyncStatusLabel` et `step.KeptInProgress` : vérifier les noms réels dans `internal/rules/` et `internal/domain/sync.go` ; si le label n'existe pas, l'ajouter dans `rules` (pur) plutôt que dans le presenter — `output/` et `tui/` n'ont aucune logique de décision.

Ajouter `type syncedMsg struct{}` dans `prompter.go` à côté de `prunedMsg`, et son cas dans `applyFlow` → `m.reload()`.

- [ ] **Step 7: Écrire le test du hint keep-conflict**

```go
func TestSyncPresenterNamesTheWayOutOfAKeptConflict(t *testing.T) {
	msgs := make(chan tea.Msg, 8)
	p := syncPresenter{presenter{send: func(msg tea.Msg) { msgs <- msg }}}

	_ = p.Synced(syncflow.Outcome{Result: domain.SyncResult{Steps: []domain.SyncStepResult{{
		Branch:         "feat-a",
		Path:           "/tmp/trees/feat-a",
		Status:         domain.SyncStatusConflict,
		KeptInProgress: true,
	}}}})
	close(msgs)

	var lines []string
	for msg := range msgs {
		if line, ok := msg.(OutputLineMsg); ok {
			lines = append(lines, line.Text)
		}
	}
	joined := strings.Join(lines, "\n")
	if !strings.Contains(joined, "/tmp/trees/feat-a") || !strings.Contains(joined, "rebase --continue") {
		t.Fatalf("a kept conflict must name its worktree and the way out, got: %q", joined)
	}
}
```

- [ ] **Step 8: Vérifier**

Run: `go build ./... && go test ./internal/tui/dashboard/ -v`
Expected: PASS.

- [ ] **Step 9: Commit**

```bash
git add internal/tui/dashboard internal/domain/constants.go
git commit -m "feat(ui): sync a worktree, the base or all of them from the dashboard"
```

---

### Task 10: Docs, skill agent et validation

**Files:**
- Modify: `docs/dev/flow-layer.md`
- Modify: `CLAUDE.md` (liste des commandes migrées)
- Modify: `internal/commands/agents/assets/using-wtm.skill.md`
- Modify: `docs/wtm_sync.md` (régénéré, jamais édité à la main)

- [ ] **Step 1: Écrire les décisions dans `docs/dev/flow-layer.md`**

Ajouter une section « sync » qui écrit, chacune avec sa raison :
- **`--keep-conflict` dans le dashboard (option B)** : l'option est offerte, la sortie est nommée. Ce qui a été écarté et pourquoi (cf. spec D9), et la limite connue : `ModeBlocking` protège pendant le run, pas après ; c'est le badge `⟳ rebasing` qui signale une worktree laissée mi-rebase.
- **`--dry-run` sans entrée dashboard**, aligné sur `prune` : le recap **est** le plan, fermer la modale ne rebase rien.
- **`Precheck` vs la sélection fixée** : deux notions distinctes dans la `Request`, et pourquoi le CLI n'en utilise qu'une.
- **La divergence assumée de « Sync all »** : `--all` coche tout, l'entrée dashboard laisse décochées les worktrees `dirty`/`rebasing`.
- **D7** : sans TTY et sans `--yes`, `sync` refuse désormais, comme `prune`.

- [ ] **Step 2: Mettre à jour `CLAUDE.md`**

Dans la section `flow/` : ajouter `sync/` à l'arborescence et faire passer `sync` de la liste « pas encore migrées » à « `create`, `clean`, `reparent`, `prune` et `sync` sont migrés ».

- [ ] **Step 3: Mettre à jour le skill agent**

`internal/commands/agents/assets/using-wtm.skill.md` : le seul changement de surface est D7 — sans TTY et sans `--yes`, `wtm sync` refuse en nommant `--yes`. Documenter la règle là où le skill décrit les modes non interactifs.

- [ ] **Step 4: Régénérer la doc**

Run: `make docs`
Expected: `docs/wtm_sync.md` régénéré. Vérifier avec `git diff docs/` que rien d'autre n'a bougé de façon inattendue. Le `README.md` ne change pas : aucune commande n'est ajoutée, renommée ni supprimée.

- [ ] **Step 5: Valider**

Lancer le subagent **`build-validator`**. Il doit être vert : compilation, `go vet`, staticcheck, tests.

Vérifier en plus, à la main :

Run: `go list -deps ./internal/flow/... | grep -E 'wtm/internal/(output|tui|commands|config)' && echo VIOLATION || echo OK`
Expected: `OK`.

Run: `go test ./internal/commands/wt/ -run 'TestSync' -v`
Expected: PASS, et `git diff --stat internal/commands/wt/sync_characterization_test.go` doit être **vide** depuis la tâche 2.

- [ ] **Step 6: Commit**

```bash
git add docs CLAUDE.md internal/commands/agents/assets/using-wtm.skill.md
git commit -m "docs(sync): record the flow migration and the keep-conflict decision"
```
