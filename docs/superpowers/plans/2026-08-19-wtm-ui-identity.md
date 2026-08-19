# `wtm ui` — identité & panneau détail : plan d'implémentation

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Donner au dashboard `wtm ui` une identité propre (palette duo signature, header contextuel, moments d'accueil) et transformer son panneau détail d'une fiche d'emplacement en une lecture du travail en cours.

**Architecture:** La logique nouvelle vit en `rules/` (pure, testable sans git) et en `service/worktree.Detail` (une fonction exportée, réutilisable hors dashboard). Le dashboard ne fait que déclencher, cacher et dessiner. Aucune donnée nouvelle n'entre dans le poll 3 s : le détail se charge paresseusement sur la sélection, débouncé, avec un contrat de fraîcheur explicite à quatre états.

**Tech Stack:** Go, Cobra, Bubbletea, Lipgloss, bubblezone, `bubbles/spinner`.

**Spec:** `docs/superpowers/specs/2026-08-19-wtm-ui-identity-design.md` — à lire avant la première tâche. Le plan argumente depuis lui.

## Global Constraints

Ces règles viennent de `CLAUDE.md` et s'appliquent à **chaque** tâche, sans être répétées :

- Toute fonction ou méthode à **2 entrées liées ou plus** prend une **struct unique**, initialisée en champs nommés.
- Types, erreurs sentinelles et **constantes** vivent dans `internal/domain/` uniquement. **Aucune chaîne ni nombre magique** : tout libellé, seuil ou format est une constante nommée dans `internal/domain/constants.go`.
- `internal/rules/` n'importe que la **stdlib** et `internal/domain` — aucune I/O, aucun effet de bord.
- `internal/styles/` est le **seul** paquet autorisé à instancier `lipgloss.Style`.
- `internal/tui/` et `internal/output/` n'ont **aucune logique de décision** — rendu seulement.
- **Retours anticipés** : chaque garde retourne immédiatement, le chemin nominal est en dernier. Pas de `if` imbriqués.
- Assertions de type **toujours en comma-ok**.
- **Commentaires quasi nuls** : le pourquoi, jamais le quoi. En modifiant un fichier, on aligne **ses** commentaires sur cette règle (on supprime ceux qui paraphrasent le code) dans le même changement.
- `internal/flow/` n'est **pas** touché par ce plan : il n'y a ici aucune commande mutante nouvelle.
- Le sous-agent **`build-validator`** tourne avant chaque commit.

Valeurs figées par le spec, à reprendre **verbatim** :

- Seuil de péremption du fetch : **24 h**. Délai d'apparition du spinner : **200 ms**. Débounce du chargement détail : **150 ms**. Plafond d'animation : **400 ms**.
- Ordre de repli des sections, du bas vers le haut : **`LINKS` → `ACTIVITY` → `CHANGES` → `REVIEW`**. La bande vitale et la ligne de blockers ne tombent jamais.
- Le worktree actif s'affiche **`● you are here`** (jamais « active », qui entrerait en collision avec `active 3 h ago`).
- Trois rôles de couleur, un propriétaire chacun : **accent navigation** (navigation/sélection), **accent signature** (wordmark + `+ New worktree` uniquement), **couleurs d'état** (pastilles, blockers, checks), **muted** (structure).

---

## Phase 1 — Fondations pures et lectures git

### Task 1: La palette duo signature

**Files:**
- Modify: `internal/styles/colors.go`
- Test: `internal/styles/colors_test.go` (créer)

**Interfaces:**
- Produces: `styles.ColorPrimary`, `styles.ColorSignature`, `styles.ColorSuccess`, `styles.ColorWarning`, `styles.ColorDanger`, `styles.ColorMuted` — toutes des `lipgloss.AdaptiveColor`.

Le spec (§3) pose une contrainte que ce test transforme en garde-fou permanent : **l'accent signature (corail) et `warning` (or) sont tous deux chauds et ne doivent jamais être confondables** — une pastille `dirty` et le bouton `+ New worktree` sont à quelques centimètres l'un de l'autre.

- [ ] **Step 1: Écrire le test d'invariant de teinte (il échoue)**

```go
package styles

import (
	"strconv"
	"testing"

	"github.com/charmbracelet/lipgloss"
)

// hueOf extrait la teinte HSL (0-360) d'un hex "#RRGGBB". Helper de test
// uniquement : la production n'a jamais besoin de raisonner sur les teintes.
func hueOf(t *testing.T, hex string) float64 {
	t.Helper()
	if len(hex) != 7 || hex[0] != '#' {
		t.Fatalf("hex mal formé: %q", hex)
	}
	channel := func(offset int) float64 {
		v, err := strconv.ParseUint(hex[offset:offset+2], 16, 8)
		if err != nil {
			t.Fatalf("hex mal formé: %q", hex)
		}
		return float64(v) / 255
	}
	r, g, b := channel(1), channel(3), channel(5)
	maxC, minC := max(r, max(g, b)), min(r, min(g, b))
	delta := maxC - minC
	if delta == 0 {
		return 0
	}
	var hue float64
	switch maxC {
	case r:
		hue = 60 * (((g - b) / delta) + 6)
	case g:
		hue = 60 * (((b-r)/delta) + 2)
	default:
		hue = 60 * (((r-g)/delta) + 4)
	}
	for hue >= 360 {
		hue -= 360
	}
	return hue
}

func hueDistance(a, b float64) float64 {
	d := a - b
	if d < 0 {
		d = -d
	}
	if d > 180 {
		return 360 - d
	}
	return d
}

// minWarmSeparation est l'écart de teinte sous lequel deux couleurs chaudes
// deviennent confondables sur un terminal.
const minWarmSeparation = 25

func TestSignatureAndWarningAreNotConfusable(t *testing.T) {
	pairs := []struct {
		theme            string
		signature, warn  string
	}{
		{"clair", ColorSignature.Light, ColorWarning.Light},
		{"sombre", ColorSignature.Dark, ColorWarning.Dark},
	}
	for _, pair := range pairs {
		t.Run(pair.theme, func(t *testing.T) {
			got := hueDistance(hueOf(t, pair.signature), hueOf(t, pair.warn))
			if got < minWarmSeparation {
				t.Errorf("signature %s et warning %s: écart de teinte %.0f°, minimum %d°",
					pair.signature, pair.warn, got, minWarmSeparation)
			}
		})
	}
}

func TestEveryColorDefinesBothThemes(t *testing.T) {
	colors := map[string]lipgloss.AdaptiveColor{
		"primary":   ColorPrimary,
		"signature": ColorSignature,
		"success":   ColorSuccess,
		"warning":   ColorWarning,
		"danger":    ColorDanger,
		"muted":     ColorMuted,
	}
	for name, color := range colors {
		if color.Light == "" || color.Dark == "" {
			t.Errorf("%s: les deux thèmes doivent être définis, got %+v", name, color)
		}
	}
}
```

- [ ] **Step 2: Lancer le test et vérifier qu'il échoue**

Run: `go test ./internal/styles/ -run 'TestSignature|TestEveryColor' -v`
Expected: FAIL — `undefined: ColorSignature`.

- [ ] **Step 3: Appliquer la palette**

Dans `internal/styles/colors.go`, remplacer les valeurs Carbon par le duo signature du spec §3. Le commentaire de chaque couleur dit **son rôle**, pas sa teinte :

```go
var (
	// ColorPrimary porte la navigation et la sélection : rien d'autre.
	ColorPrimary = lipgloss.AdaptiveColor{Light: "#5B4BE0", Dark: "#9B8CFF"}

	// ColorSignature n'habille que le wordmark et l'appel à l'action. Sa teinte
	// est tenue à l'écart de ColorWarning (voir colors_test.go) : les deux sont
	// chaudes et se côtoient dans le header.
	ColorSignature = lipgloss.AdaptiveColor{Light: "#C2521F", Dark: "#E8734A"}

	ColorMuted   = lipgloss.AdaptiveColor{Light: "#6F6F6F", Dark: "#8D8D8D"}
	ColorSuccess = lipgloss.AdaptiveColor{Light: "#158A4A", Dark: "#4ADE80"}
	ColorWarning = lipgloss.AdaptiveColor{Light: "#B07800", Dark: "#F0C000"}
	ColorDanger  = lipgloss.AdaptiveColor{Light: "#C21E28", Dark: "#FA6E76"}

	ColorBadgeFg    = lipgloss.AdaptiveColor{Light: "#FFFFFF", Dark: "#161616"}
	ColorSelectedBg = lipgloss.AdaptiveColor{Light: "#DCD7FB", Dark: "#3B2FA8"}
	ColorSelectedFg = lipgloss.AdaptiveColor{Light: "#1A1150", Dark: "#FFFFFF"}
	ColorRowTint    = lipgloss.AdaptiveColor{Light: "#EFEBFD", Dark: "#282341"}
)
```

- [ ] **Step 4: Vérifier que le test passe**

Run: `go test ./internal/styles/ -v`
Expected: PASS.

- [ ] **Step 5: Vérification visuelle des deux thèmes**

Compiler et lancer le dashboard dans un terminal clair **puis** sombre :

```bash
go build -o /tmp/wtm-preview . && /tmp/wtm-preview ui
```

Contrôler à l'œil : la pastille `dirty` et le bouton `+ New worktree` ne se confondent pas ; le texte muted reste lisible sur les deux fonds ; la ligne sélectionnée se distingue sans écraser son texte. **Le spec dit explicitement que les hex sont un point de départ, pas un acquis** — si un contraste passe mal, ajuster ici et relancer le test du Step 4.

- [ ] **Step 6: Valider et commiter**

```bash
go build ./... && go vet ./... && go test ./...
git add internal/styles/colors.go internal/styles/colors_test.go
git commit -m "feat(styles): palette duo signature et invariant de non-confusion"
```

---

### Task 2: Le worktree actif, par préfixe de cwd

**Files:**
- Modify: `internal/rules/dashboard.go`
- Test: `internal/rules/dashboard_test.go`

**Interfaces:**
- Produces: `rules.ActiveWorktree(rules.ActiveWorktreeParams) string` — la branche du worktree contenant le cwd, `""` si aucun.

Aucun appel git : les `Path` sont déjà en mémoire. Deux pièges que le test verrouille — `/a/b` ne doit pas matcher `/a/bc` (frontière de segment), et quand deux worktrees s'emboîtent c'est **le plus profond** qui gagne.

- [ ] **Step 1: Écrire le test (il échoue)**

```go
func TestActiveWorktree(t *testing.T) {
	statuses := []domain.WorktreeStatus{
		{Branch: "main", Path: "/repo"},
		{Branch: "feat/ui", Path: "/repo.worktrees/feat-ui"},
		{Branch: "feat/ui-extra", Path: "/repo.worktrees/feat-ui-extra"},
	}
	cases := []struct {
		name string
		cwd  string
		want string
	}{
		{"racine du worktree", "/repo.worktrees/feat-ui", "feat/ui"},
		{"sous-dossier", "/repo.worktrees/feat-ui/internal/tui", "feat/ui"},
		{"préfixe voisin non confondu", "/repo.worktrees/feat-ui-extra", "feat/ui-extra"},
		{"worktree principal", "/repo/internal", "main"},
		{"le plus profond gagne", "/repo.worktrees/feat-ui", "feat/ui"},
		{"hors de tout worktree", "/ailleurs", ""},
		{"cwd vide", "", ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := ActiveWorktree(ActiveWorktreeParams{Cwd: c.cwd, Statuses: statuses})
			if got != c.want {
				t.Errorf("ActiveWorktree(%q) = %q, want %q", c.cwd, got, c.want)
			}
		})
	}
}
```

- [ ] **Step 2: Lancer le test et vérifier qu'il échoue**

Run: `go test ./internal/rules/ -run TestActiveWorktree -v`
Expected: FAIL — `undefined: ActiveWorktree`.

- [ ] **Step 3: Implémenter**

Dans `internal/rules/dashboard.go` :

```go
type ActiveWorktreeParams struct {
	Cwd      string
	Statuses []domain.WorktreeStatus
}

// ActiveWorktree nomme la branche du worktree qui contient Cwd. Quand deux
// worktrees s'emboîtent, le plus profond l'emporte : c'est celui dans lequel on
// travaille réellement.
func ActiveWorktree(params ActiveWorktreeParams) string {
	if params.Cwd == "" {
		return ""
	}

	branch, deepest := "", 0
	for _, status := range params.Statuses {
		if status.Path == "" || !underPath(params.Cwd, status.Path) {
			continue
		}
		if length := len(status.Path); length > deepest {
			branch, deepest = status.Branch, length
		}
	}
	return branch
}

// underPath teste l'appartenance sur une frontière de segment, pour que /a/bc ne
// passe pas pour un enfant de /a/b.
func underPath(cwd, root string) bool {
	if cwd == root {
		return true
	}
	return strings.HasPrefix(cwd, root+string(filepath.Separator))
}
```

- [ ] **Step 4: Vérifier que le test passe**

Run: `go test ./internal/rules/ -run TestActiveWorktree -v`
Expected: PASS.

- [ ] **Step 5: Valider et commiter**

```bash
go build ./... && go vet ./... && go test ./internal/rules/
git add internal/rules/dashboard.go internal/rules/dashboard_test.go
git commit -m "feat(rules): détecter le worktree actif depuis le cwd"
```

---

### Task 3: Âge relatif et péremption du fetch

**Files:**
- Create: `internal/rules/age.go`
- Modify: `internal/domain/constants.go`
- Test: `internal/rules/age_test.go`

**Interfaces:**
- Produces: `rules.RelativeAge(rules.RelativeAgeParams) string` (`"3 h ago"`, `"2 d ago"`, `"just now"`, `""` si zéro) et `rules.FetchIsStale(rules.FetchStalenessParams) bool`.

Un seul formateur pour les trois usages du spec — `active 3 h ago`, `Created 2 d ago`, `fetched 3 d ago`.

- [ ] **Step 1: Écrire le test (il échoue)**

```go
package rules

import (
	"testing"
	"time"
)

func TestRelativeAge(t *testing.T) {
	now := time.Date(2026, 8, 19, 12, 0, 0, 0, time.UTC)
	cases := []struct {
		name string
		at   time.Time
		want string
	}{
		{"zéro", time.Time{}, ""},
		{"il y a 10 s", now.Add(-10 * time.Second), "just now"},
		{"il y a 5 min", now.Add(-5 * time.Minute), "5 min ago"},
		{"il y a 3 h", now.Add(-3 * time.Hour), "3 h ago"},
		{"il y a 2 j", now.Add(-48 * time.Hour), "2 d ago"},
		{"il y a 3 sem", now.Add(-21 * 24 * time.Hour), "3 w ago"},
		{"futur (horloge décalée)", now.Add(time.Hour), "just now"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := RelativeAge(RelativeAgeParams{At: c.at, Now: now})
			if got != c.want {
				t.Errorf("RelativeAge(%v) = %q, want %q", c.at, got, c.want)
			}
		})
	}
}

func TestFetchIsStale(t *testing.T) {
	now := time.Date(2026, 8, 19, 12, 0, 0, 0, time.UTC)
	cases := []struct {
		name      string
		fetchedAt time.Time
		want      bool
	}{
		{"jamais fetché", time.Time{}, true},
		{"il y a 1 h", now.Add(-time.Hour), false},
		{"il y a 23 h", now.Add(-23 * time.Hour), false},
		{"il y a 25 h", now.Add(-25 * time.Hour), true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := FetchIsStale(FetchStalenessParams{FetchedAt: c.fetchedAt, Now: now})
			if got != c.want {
				t.Errorf("FetchIsStale(%v) = %v, want %v", c.fetchedAt, got, c.want)
			}
		})
	}
}
```

- [ ] **Step 2: Lancer le test et vérifier qu'il échoue**

Run: `go test ./internal/rules/ -run 'TestRelativeAge|TestFetchIsStale' -v`
Expected: FAIL — `undefined: RelativeAge`.

- [ ] **Step 3: Ajouter les constantes**

Dans `internal/domain/constants.go`, à côté du bloc `Dashboard*` :

```go
	// FetchStaleAfter est l'âge au-delà duquel les refs origin sont annoncées
	// périmées dans le header. En dessous, rien ne s'affiche : un marqueur
	// permanent ne signale plus rien.
	FetchStaleAfter = 24 * time.Hour

	AgeJustNow   = "just now"
	AgeMinFmt    = "%d min ago"
	AgeHourFmt   = "%d h ago"
	AgeDayFmt    = "%d d ago"
	AgeWeekFmt   = "%d w ago"
```

- [ ] **Step 4: Implémenter**

Dans `internal/rules/age.go` :

```go
package rules

import (
	"fmt"
	"time"

	"github.com/LucasPcq/wtm/internal/domain"
)

type RelativeAgeParams struct {
	At  time.Time
	Now time.Time
}

// RelativeAge rend un âge lisible d'un coup d'œil. Une date future (horloge
// décalée, commit rejoué) se lit "just now" plutôt que de compter à l'envers.
func RelativeAge(params RelativeAgeParams) string {
	if params.At.IsZero() {
		return ""
	}

	elapsed := params.Now.Sub(params.At)
	switch {
	case elapsed < time.Minute:
		return domain.AgeJustNow
	case elapsed < time.Hour:
		return fmt.Sprintf(domain.AgeMinFmt, int(elapsed.Minutes()))
	case elapsed < 24*time.Hour:
		return fmt.Sprintf(domain.AgeHourFmt, int(elapsed.Hours()))
	case elapsed < 7*24*time.Hour:
		return fmt.Sprintf(domain.AgeDayFmt, int(elapsed.Hours()/24))
	default:
		return fmt.Sprintf(domain.AgeWeekFmt, int(elapsed.Hours()/(24*7)))
	}
}

type FetchStalenessParams struct {
	FetchedAt time.Time
	Now       time.Time
}

// FetchIsStale dit si les refs origin sont assez vieilles pour que la vue le
// déclare. Jamais fetché compte comme périmé : c'est le cas le plus trompeur.
func FetchIsStale(params FetchStalenessParams) bool {
	if params.FetchedAt.IsZero() {
		return true
	}
	return params.Now.Sub(params.FetchedAt) > domain.FetchStaleAfter
}
```

- [ ] **Step 5: Vérifier que les tests passent**

Run: `go test ./internal/rules/ -run 'TestRelativeAge|TestFetchIsStale' -v`
Expected: PASS.

- [ ] **Step 6: Valider et commiter**

```bash
go build ./... && go vet ./... && go test ./internal/rules/ ./internal/domain/
git add internal/rules/age.go internal/rules/age_test.go internal/domain/constants.go
git commit -m "feat(rules): âge relatif et seuil de péremption du fetch"
```

---

### Task 4: Les lectures git nouvelles

**Files:**
- Create: `internal/infra/history.go`
- Test: `internal/infra/history_test.go`

**Interfaces:**
- Consumes: `domain.CommitSummary` (défini en Task 5 — **le déclarer dans cette tâche si elle passe en premier**, voir Step 3).
- Produces: `infra.RecentCommits(infra.RecentCommitsParams) ([]domain.CommitSummary, error)`, `infra.DiffShortstat(infra.DiffShortstatParams) (domain.DiffStat, error)`, `infra.LastFetchAt(infra.LastFetchAtParams) time.Time`.

- [ ] **Step 1: Écrire le test (il échoue)**

```go
package infra

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/LucasPcq/wtm/internal/testutil/gittest"
)

func TestRecentCommits(t *testing.T) {
	dir := gittest.InitRepo(t)
	if err := os.WriteFile(filepath.Join(dir, "a.txt"), []byte("un"), 0o644); err != nil {
		t.Fatal(err)
	}
	gittest.Git(t, dir, "add", ".")
	gittest.Git(t, dir, "commit", "-m", "feat: premier ajout")

	commits, err := RecentCommits(RecentCommitsParams{WorktreePath: dir, Limit: 5})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(commits) == 0 {
		t.Fatal("aucun commit retourné")
	}
	if commits[0].Subject != "feat: premier ajout" {
		t.Errorf("Subject = %q, want %q", commits[0].Subject, "feat: premier ajout")
	}
	if len(commits[0].SHA) != 7 {
		t.Errorf("SHA = %q, want 7 caractères", commits[0].SHA)
	}
	if commits[0].At.IsZero() {
		t.Error("At ne doit pas être zéro")
	}
	if commits[0].Author == "" {
		t.Error("Author ne doit pas être vide")
	}
}

func TestRecentCommitsSubjectWithSeparator(t *testing.T) {
	dir := gittest.InitRepo(t)
	if err := os.WriteFile(filepath.Join(dir, "b.txt"), []byte("deux"), 0o644); err != nil {
		t.Fatal(err)
	}
	gittest.Git(t, dir, "add", ".")
	gittest.Git(t, dir, "commit", "-m", "fix: garder a|b intact")

	commits, err := RecentCommits(RecentCommitsParams{WorktreePath: dir, Limit: 1})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if commits[0].Subject != "fix: garder a|b intact" {
		t.Errorf("Subject = %q — le séparateur de champ ne doit pas couper le sujet", commits[0].Subject)
	}
}

func TestDiffShortstat(t *testing.T) {
	dir := gittest.InitRepo(t)
	if err := os.WriteFile(filepath.Join(dir, "c.txt"), []byte("l1\nl2\nl3\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gittest.Git(t, dir, "add", ".")
	gittest.Git(t, dir, "commit", "-m", "seed")
	if err := os.WriteFile(filepath.Join(dir, "c.txt"), []byte("l1\nl2\nl3\nl4\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	stat, err := DiffShortstat(DiffShortstatParams{WorktreePath: dir})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if stat.Insertions != 1 {
		t.Errorf("Insertions = %d, want 1", stat.Insertions)
	}
}

func TestLastFetchAtWithoutFetchHead(t *testing.T) {
	dir := gittest.InitRepo(t)
	if got := LastFetchAt(LastFetchAtParams{ProjectDir: dir}); !got.IsZero() {
		t.Errorf("LastFetchAt = %v, want zéro quand FETCH_HEAD est absent", got)
	}
}
```

- [ ] **Step 2: Lancer le test et vérifier qu'il échoue**

Run: `go test ./internal/infra/ -run 'TestRecentCommits|TestDiffShortstat|TestLastFetchAt' -v`
Expected: FAIL — `undefined: RecentCommits`.

- [ ] **Step 3: Déclarer les types et constantes**

Dans `internal/domain/worktree.go` (ou `detail.go` si la Task 5 l'a déjà créé — un seul endroit, jamais les deux) :

```go
// CommitSummary est une ligne d'historique telle que le détail l'affiche.
type CommitSummary struct {
	SHA     string
	Subject string
	Author  string
	At      time.Time
}

// DiffStat est le volume d'un diff, sans le détail par fichier.
type DiffStat struct {
	Insertions int
	Deletions  int
}
```

Dans `internal/domain/constants.go` :

```go
	// GitLogFieldSep sépare les champs de `git log --format`. Le pipe ne peut pas
	// servir : un sujet de commit en contient. Le séparateur d'unité ASCII, si.
	GitLogFieldSep    = "\x1f"
	GitLogFormat      = "%h\x1f%s\x1f%an\x1f%cI"
	GitLogFieldCount  = 4
	FetchHeadFileName = "FETCH_HEAD"
```

- [ ] **Step 4: Implémenter**

Dans `internal/infra/history.go` :

```go
package infra

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/LucasPcq/wtm/internal/domain"
)

type RecentCommitsParams struct {
	WorktreePath string
	Limit        int
}

func RecentCommits(params RecentCommitsParams) ([]domain.CommitSummary, error) {
	cmd := exec.Command("git", "-C", params.WorktreePath, "log",
		"--max-count="+strconv.Itoa(params.Limit), "--format="+domain.GitLogFormat)
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("git log: %w", err)
	}

	lines := strings.Split(strings.TrimRight(string(out), "\n"), "\n")
	commits := make([]domain.CommitSummary, 0, len(lines))
	for _, line := range lines {
		if line == "" {
			continue
		}
		fields := strings.SplitN(line, domain.GitLogFieldSep, domain.GitLogFieldCount)
		if len(fields) != domain.GitLogFieldCount {
			continue
		}
		at, _ := time.Parse(time.RFC3339, fields[3])
		commits = append(commits, domain.CommitSummary{
			SHA:     fields[0],
			Subject: fields[1],
			Author:  fields[2],
			At:      at,
		})
	}
	return commits, nil
}

type DiffShortstatParams struct {
	WorktreePath string
}

// DiffShortstat mesure le travail non commité, index compris.
func DiffShortstat(params DiffShortstatParams) (domain.DiffStat, error) {
	cmd := exec.Command("git", "-C", params.WorktreePath, "diff", "HEAD", "--shortstat")
	out, err := cmd.Output()
	if err != nil {
		return domain.DiffStat{}, fmt.Errorf("git diff: %w", err)
	}
	return parseShortstat(string(out)), nil
}

// parseShortstat lit "N files changed, N insertions(+), N deletions(-)", dont
// chacun des trois segments peut manquer.
func parseShortstat(out string) domain.DiffStat {
	stat := domain.DiffStat{}
	for _, segment := range strings.Split(out, ",") {
		fields := strings.Fields(strings.TrimSpace(segment))
		if len(fields) < 2 {
			continue
		}
		count, err := strconv.Atoi(fields[0])
		if err != nil {
			continue
		}
		switch {
		case strings.HasPrefix(fields[1], "insertion"):
			stat.Insertions = count
		case strings.HasPrefix(fields[1], "deletion"):
			stat.Deletions = count
		}
	}
	return stat
}

type LastFetchAtParams struct {
	ProjectDir string
}

// LastFetchAt date le dernier fetch depuis le mtime de FETCH_HEAD. Zéro quand le
// dépôt n'a jamais fetché — le cas le plus trompeur, que l'appelant traite comme
// périmé.
func LastFetchAt(params LastFetchAtParams) time.Time {
	gitDir, err := GitCommonDir(GitCommonDirParams{Dir: params.ProjectDir})
	if err != nil {
		return time.Time{}
	}
	info, err := os.Stat(filepath.Join(gitDir, domain.FetchHeadFileName))
	if err != nil {
		return time.Time{}
	}
	return info.ModTime()
}
```

`GitCommonDir` existe déjà dans `internal/infra/git_paths.go` ; vérifier le nom exact de son champ de paramètre avant d'écrire l'appel.

- [ ] **Step 5: Vérifier que les tests passent**

Run: `go test ./internal/infra/ -v`
Expected: PASS.

- [ ] **Step 6: Valider et commiter**

```bash
go build ./... && go vet ./... && go test ./internal/infra/
git add internal/infra/history.go internal/infra/history_test.go internal/infra/git_paths.go internal/domain/
git commit -m "feat(infra): commits récents, shortstat et date du dernier fetch"
```

---

### Task 5: Les types du détail et le comptage des changements

**Files:**
- Create: `internal/domain/detail.go`
- Create: `internal/rules/changes.go`
- Test: `internal/rules/changes_test.go`

**Interfaces:**
- Produces: `domain.WorktreeDetail`, `domain.WorkingChanges`, `domain.EnvDriftSummary`, `domain.DetailFamily` + ses constantes, et `rules.CountChanges([]domain.PorcelainEntry) domain.WorkingChanges`.

Le spec (§7) impose `Failures map[DetailFamily]error` : **une famille absente de la map a été lue correctement, vide y compris.** C'est ce qui sépare l'absence légitime de la panne au rendu (§8, état 4).

- [ ] **Step 1: Écrire le test de comptage (il échoue)**

Le champ `Status` est le XY porcelain brut, espaces compris : la colonne compte. Un fichier peut être **à la fois** indexé et modifié (`MM`), et doit alors compter dans les deux.

```go
package rules

import (
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
)

func TestCountChanges(t *testing.T) {
	entries := []domain.PorcelainEntry{
		{Status: " M", Path: "a.go"},
		{Status: "M ", Path: "b.go"},
		{Status: "MM", Path: "c.go"},
		{Status: "??", Path: "d.go"},
		{Status: "A ", Path: "e.go"},
		{Status: " D", Path: "f.go"},
	}

	got := CountChanges(entries)

	if got.Modified != 3 {
		t.Errorf("Modified = %d, want 3 (' M', 'MM', ' D')", got.Modified)
	}
	if got.Staged != 3 {
		t.Errorf("Staged = %d, want 3 ('M ', 'MM', 'A ')", got.Staged)
	}
	if got.Untracked != 1 {
		t.Errorf("Untracked = %d, want 1", got.Untracked)
	}
	if len(got.Files) != len(entries) {
		t.Errorf("Files = %d, want %d", len(got.Files), len(entries))
	}
}

func TestCountChangesEmpty(t *testing.T) {
	got := CountChanges(nil)
	if got.Modified != 0 || got.Staged != 0 || got.Untracked != 0 || len(got.Files) != 0 {
		t.Errorf("CountChanges(nil) = %+v, want zéro partout", got)
	}
}
```

- [ ] **Step 2: Lancer le test et vérifier qu'il échoue**

Run: `go test ./internal/rules/ -run TestCountChanges -v`
Expected: FAIL — `undefined: CountChanges`.

- [ ] **Step 3: Déclarer les types**

Dans `internal/domain/detail.go` :

```go
package domain

import "time"

// DetailFamily nomme un groupe de données du détail qui peut échouer seul.
type DetailFamily string

const (
	DetailFamilyCommits  DetailFamily = "commits"
	DetailFamilyChanges  DetailFamily = "changes"
	DetailFamilyEnv      DetailFamily = "env"
	DetailFamilyBlockers DetailFamily = "blockers"
)

// WorkingChanges est l'état du working tree, compté par colonne porcelain. Un
// fichier indexé puis remodifié ("MM") compte dans Staged et dans Modified.
type WorkingChanges struct {
	Modified   int
	Untracked  int
	Staged     int
	Insertions int
	Deletions  int
	Files      []PorcelainEntry
}

// EnvDriftSummary résume l'écart .env sans le détail par clé. Configured
// distingue "aucun drift" de "aucun fichier env configuré" — le rendu ne doit
// pas présenter le second comme un succès.
type EnvDriftSummary struct {
	Configured  bool
	Missing     int
	Conflicting int
	Orphan      int
}

// WorktreeDetail est ce que le panneau détail affiche au-delà de WorktreeStatus.
// Il se charge paresseusement sur la sélection : jamais dans le poll.
type WorktreeDetail struct {
	Branch   string
	Commits  []CommitSummary
	Changes  WorkingChanges
	Children []string
	Blockers []CleanBlocker
	EnvDrift EnvDriftSummary
	LoadedAt time.Time

	// Failures nomme les familles qui n'ont pas pu être lues. Une famille absente
	// a été lue correctement, vide y compris : c'est ce qui sépare l'absence
	// légitime de la panne au rendu.
	Failures map[DetailFamily]error
}
```

Dans `internal/domain/constants.go` :

```go
	// DashboardDetailCommits est le nombre de commits demandés pour ACTIVITY.
	DashboardDetailCommits = 5

	PorcelainUntracked = "??"
	PorcelainUnmodified = ' '
```

- [ ] **Step 4: Implémenter le comptage**

Dans `internal/rules/changes.go` :

```go
package rules

import "github.com/LucasPcq/wtm/internal/domain"

// CountChanges répartit des entrées porcelain par colonne : X est l'index, Y est
// le working tree. Un fichier peut peser dans les deux.
func CountChanges(entries []domain.PorcelainEntry) domain.WorkingChanges {
	changes := domain.WorkingChanges{Files: entries}
	for _, entry := range entries {
		if entry.Status == domain.PorcelainUntracked {
			changes.Untracked++
			continue
		}
		if len(entry.Status) < 2 {
			continue
		}
		if entry.Status[0] != domain.PorcelainUnmodified {
			changes.Staged++
		}
		if entry.Status[1] != domain.PorcelainUnmodified {
			changes.Modified++
		}
	}
	return changes
}
```

- [ ] **Step 5: Vérifier que le test passe**

Run: `go test ./internal/rules/ -run TestCountChanges -v`
Expected: PASS.

- [ ] **Step 6: Valider et commiter**

```bash
go build ./... && go vet ./... && go test ./internal/rules/ ./internal/domain/
git add internal/domain/detail.go internal/domain/constants.go internal/rules/changes.go internal/rules/changes_test.go
git commit -m "feat(domain,rules): types du détail worktree et comptage porcelain"
```

---

### Task 6: `service/worktree.Detail`

**Files:**
- Create: `internal/service/worktree/worktreedetail.go`
- Test: `internal/service/worktree/worktreedetail_test.go`

**Interfaces:**
- Consumes: `infra.RecentCommits`, `infra.DiffShortstat`, `infra.ListModifiedFiles`, `infra.UnpushedCommits`, `rules.CountChanges`, `rules.ParsePorcelainZ`, `rules.CleanBlockers`, `env.ComputeEnvDiff`.
- Produces: `worktree.Detail(worktree.DetailParams) domain.WorktreeDetail` — **ne retourne pas d'erreur** : les échecs partent dans `Failures`.

Le fichier s'appelle `worktreedetail.go` et non `detail.go` : `internal/service/worktree/detail.go` existe déjà (lecteur de métadonnées) et fait autre chose.

**Zéro appel `gh` ajouté.** `worktree.Check` en fait un (`ghservice.HasOpenPR`) ; ici le PR ouvert est **passé en entrée** par l'appelant, qui l'a déjà en mémoire. `Children` de même.

- [ ] **Step 1: Écrire le test (il échoue)**

```go
package worktree

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/testutil/gittest"
)

func TestDetailReadsCommitsAndChanges(t *testing.T) {
	dir := gittest.InitRepo(t)
	if err := os.WriteFile(filepath.Join(dir, "a.txt"), []byte("un"), 0o644); err != nil {
		t.Fatal(err)
	}
	gittest.Git(t, dir, "add", ".")
	gittest.Git(t, dir, "commit", "-m", "feat: seed")
	if err := os.WriteFile(filepath.Join(dir, "b.txt"), []byte("deux"), 0o644); err != nil {
		t.Fatal(err)
	}

	got := Detail(DetailParams{
		ProjectDir: dir,
		Status:     domain.WorktreeStatus{Branch: "main", Path: dir, IsDirty: true},
		Children:   []string{"feat/enfant"},
		Commits:    domain.DashboardDetailCommits,
	})

	if len(got.Commits) == 0 {
		t.Fatal("Commits vide")
	}
	if got.Commits[0].Subject != "feat: seed" {
		t.Errorf("Commits[0].Subject = %q, want %q", got.Commits[0].Subject, "feat: seed")
	}
	if got.Changes.Untracked != 1 {
		t.Errorf("Changes.Untracked = %d, want 1", got.Changes.Untracked)
	}
	if len(got.Children) != 1 || got.Children[0] != "feat/enfant" {
		t.Errorf("Children = %v, want [feat/enfant]", got.Children)
	}
	if len(got.Failures) != 0 {
		t.Errorf("Failures = %v, want vide", got.Failures)
	}
}

func TestDetailBlockersFromMemoryNotFromGH(t *testing.T) {
	dir := gittest.InitRepo(t)
	if err := os.WriteFile(filepath.Join(dir, "a.txt"), []byte("un"), 0o644); err != nil {
		t.Fatal(err)
	}
	gittest.Git(t, dir, "add", ".")
	gittest.Git(t, dir, "commit", "-m", "seed")

	got := Detail(DetailParams{
		ProjectDir: dir,
		Status:     domain.WorktreeStatus{Branch: "feat/x", Path: dir, IsDirty: true},
		PRs:        []domain.PRInfo{{Branch: "feat/x", Number: 7, URL: "https://example/7"}},
		Commits:    domain.DashboardDetailCommits,
	})

	keys := map[string]bool{}
	for _, blocker := range got.Blockers {
		keys[blocker.Key] = true
	}
	if !keys[domain.CleanBlockerDirty] {
		t.Error("le worktree sale doit produire un blocker dirty")
	}
	if !keys[domain.CleanBlockerOpenPR] {
		t.Error("la PR passée en entrée doit produire un blocker open-PR, sans appeler gh")
	}
}

func TestDetailRecordsFailurePerFamily(t *testing.T) {
	got := Detail(DetailParams{
		ProjectDir: t.TempDir(),
		Status:     domain.WorktreeStatus{Branch: "orpheline", Path: filepath.Join(t.TempDir(), "absent")},
		Commits:    domain.DashboardDetailCommits,
	})

	if _, failed := got.Failures[domain.DetailFamilyCommits]; !failed {
		t.Error("un chemin inexistant doit inscrire une panne pour la famille commits")
	}
	if got.Branch != "orpheline" {
		t.Errorf("Branch = %q — le détail reste exploitable malgré la panne", got.Branch)
	}
}
```

- [ ] **Step 2: Lancer le test et vérifier qu'il échoue**

Run: `go test ./internal/service/worktree/ -run TestDetail -v`
Expected: FAIL — `undefined: Detail`.

- [ ] **Step 3: Implémenter**

```go
package worktree

import (
	"time"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/infra"
	"github.com/LucasPcq/wtm/internal/rules"
	envsvc "github.com/LucasPcq/wtm/internal/service/env"
)

type DetailParams struct {
	ProjectDir string
	StateDir   string
	Config     domain.Config
	Status     domain.WorktreeStatus
	Parent     string
	Children   []string
	PRs        []domain.PRInfo
	Commits    int
}

// Detail rassemble ce que le panneau détail affiche au-delà du statut. Il ne
// retourne pas d'erreur : chaque famille échoue seule, dans Failures, pour que le
// reste du panneau reste lisible.
func Detail(params DetailParams) domain.WorktreeDetail {
	detail := domain.WorktreeDetail{
		Branch:   params.Status.Branch,
		Children: params.Children,
		LoadedAt: time.Now(),
		Failures: map[domain.DetailFamily]error{},
	}

	commits, err := infra.RecentCommits(infra.RecentCommitsParams{
		WorktreePath: params.Status.Path,
		Limit:        params.Commits,
	})
	if err != nil {
		detail.Failures[domain.DetailFamilyCommits] = err
	}
	detail.Commits = commits

	detail.Changes, err = readChanges(params.Status.Path)
	if err != nil {
		detail.Failures[domain.DetailFamilyChanges] = err
	}

	detail.Blockers = rules.CleanBlockers(domain.CleanCheckResult{
		WorktreePath:    params.Status.Path,
		Branch:          params.Status.Branch,
		IsDirty:         params.Status.IsDirty,
		IsParent:        params.Status.IsParent,
		UnpushedCommits: unpushed(params),
		HasOpenPR:       openPR(params) != nil,
		PRUrl:           openPRURL(params),
	})

	detail.EnvDrift, err = readEnvDrift(params)
	if err != nil {
		detail.Failures[domain.DetailFamilyEnv] = err
	}

	return detail
}

func readChanges(worktreePath string) (domain.WorkingChanges, error) {
	entries, err := infra.ListModifiedFiles(infra.ListModifiedFilesParams{WorktreePath: worktreePath})
	if err != nil {
		return domain.WorkingChanges{}, err
	}

	changes := rules.CountChanges(entries)
	stat, err := infra.DiffShortstat(infra.DiffShortstatParams{WorktreePath: worktreePath})
	if err != nil {
		return changes, err
	}
	changes.Insertions, changes.Deletions = stat.Insertions, stat.Deletions
	return changes, nil
}
```

Puis les quatre helpers, dans le même fichier :

```go
// unpushed : une branche sans remote n'est pas une panne, elle n'a rien à pousser.
func unpushed(params DetailParams) int {
	count, err := infra.UnpushedCommits(infra.UnpushedCommitsParams{
		ProjectDir: params.ProjectDir,
		Branch:     params.Status.Branch,
	})
	if err != nil {
		return 0
	}
	return count
}

func openPR(params DetailParams) *domain.PRInfo {
	for index, pr := range params.PRs {
		if pr.Branch == params.Status.Branch {
			return &params.PRs[index]
		}
	}
	return nil
}

func openPRURL(params DetailParams) string {
	pr := openPR(params)
	if pr == nil {
		return ""
	}
	return pr.URL
}

// readEnvDrift distingue "aucun fichier env configuré" (absence légitime, pas de
// panne) de "drift calculé" : le rendu ne doit pas présenter le premier comme un
// succès.
func readEnvDrift(params DetailParams) (domain.EnvDriftSummary, error) {
	files := params.Config.Project.Env.Files
	if len(files) == 0 {
		return domain.EnvDriftSummary{Configured: false}, nil
	}

	meta, _ := Metadata(ParentBranchParams{
		StateDir: params.StateDir,
		Branch:   params.Status.Branch,
	})

	results, err := envsvc.ComputeEnvDiff(envsvc.ComputeEnvParams{
		Branch:       params.Status.Branch,
		MainPath:     params.ProjectDir,
		WorktreePath: params.Status.Path,
		ParentBranch: params.Parent,
		Files:        files,
		Strategy:     meta.EnvStrategy,
		Mode:         domain.EnvModeAdd,
	})
	if err != nil {
		return domain.EnvDriftSummary{Configured: true}, err
	}

	drift := domain.EnvDriftSummary{Configured: true}
	for _, result := range results {
		drift.Missing += len(result.Diff.Missing)
		drift.Conflicting += len(result.Diff.Conflicting)
		drift.Orphan += len(result.Diff.Orphan)
	}
	return drift, nil
}
```

Les noms de champs de `domain.EnvDiff` (`Missing` / `Conflicting` / `Orphan`) et de `domain.EnvFileResult` sont à confirmer dans `internal/domain/` avant d'écrire la boucle : si le service expose déjà des compteurs, les lire plutôt que de compter les slices.

- [ ] **Step 4: Vérifier que les tests passent**

Run: `go test ./internal/service/worktree/ -run TestDetail -v`
Expected: PASS.

- [ ] **Step 5: Valider et commiter**

```bash
go build ./... && go vet ./... && go test ./internal/service/...
git add internal/service/worktree/worktreedetail.go internal/service/worktree/worktreedetail_test.go
git commit -m "feat(service): worktree.Detail, une lecture par famille sans appel gh"
```

---

## Phase 2 — Le panneau détail

### Task 7: La composition du panneau (logique pure)

**Files:**
- Create: `internal/rules/detailpanel.go`
- Modify: `internal/domain/detail.go`, `internal/domain/constants.go`
- Test: `internal/rules/detailpanel_test.go`

**Interfaces:**
- Produces: `rules.VitalChips(rules.VitalChipsParams) []domain.Chip`, `rules.DetailSections(rules.DetailSectionsParams) []domain.DetailSection`, `rules.FitSections(rules.FitSectionsParams) []domain.DetailSection`.

C'est le cœur du spec §6 : **une section n'apparaît que si elle a quelque chose à dire**, l'ordre est fixe, le repli se fait par le bas, et la bande vitale ne tombe jamais. Tout est pur — testable sans terminal ni git.

- [ ] **Step 1: Écrire les tests (ils échouent)**

```go
package rules

import (
	"testing"
	"time"

	"github.com/LucasPcq/wtm/internal/domain"
)

func sectionKeys(sections []domain.DetailSection) []string {
	keys := make([]string, 0, len(sections))
	for _, section := range sections {
		keys = append(keys, section.Key)
	}
	return keys
}

func TestDetailSectionsOmitsWhatHasNothingToSay(t *testing.T) {
	sections := DetailSections(DetailSectionsParams{
		Status: domain.WorktreeStatus{Branch: "feat/x", Path: "/wt/x"},
		Detail: domain.WorktreeDetail{Commits: []domain.CommitSummary{{SHA: "abc1234", Subject: "feat: x"}}},
	})

	for _, key := range sectionKeys(sections) {
		if key == domain.DetailSectionReview {
			t.Error("REVIEW ne doit pas apparaître sans PR")
		}
		if key == domain.DetailSectionChanges {
			t.Error("CHANGES ne doit pas apparaître sur un worktree propre")
		}
	}
}

func TestDetailSectionsKeepsFixedOrder(t *testing.T) {
	sections := DetailSections(DetailSectionsParams{
		Status: domain.WorktreeStatus{Branch: "feat/x", Path: "/wt/x", IsDirty: true},
		PR:     &domain.PRInfo{Number: 67, Title: "feat: x", State: "OPEN"},
		Detail: domain.WorktreeDetail{
			Commits: []domain.CommitSummary{{SHA: "abc1234", Subject: "feat: x"}},
			Changes: domain.WorkingChanges{Modified: 2, Files: []domain.PorcelainEntry{{Status: " M", Path: "a.go"}}},
		},
	})

	want := []string{
		domain.DetailSectionReview,
		domain.DetailSectionChanges,
		domain.DetailSectionActivity,
		domain.DetailSectionLinks,
	}
	got := sectionKeys(sections)
	if len(got) != len(want) {
		t.Fatalf("sections = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("section[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestFitSectionsDropsFromTheBottom(t *testing.T) {
	sections := []domain.DetailSection{
		{Key: domain.DetailSectionReview, Lines: []string{"a", "b"}},
		{Key: domain.DetailSectionChanges, Lines: []string{"c", "d"}},
		{Key: domain.DetailSectionActivity, Lines: []string{"e", "f"}},
		{Key: domain.DetailSectionLinks, Lines: []string{"g", "h"}},
	}

	got := sectionKeys(FitSections(FitSectionsParams{Sections: sections, Height: 8}))
	want := []string{domain.DetailSectionReview, domain.DetailSectionChanges}
	if len(got) != len(want) {
		t.Fatalf("sections retenues = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("section[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestVitalChipsStateFirstAndOnlyStateColored(t *testing.T) {
	now := time.Date(2026, 8, 19, 12, 0, 0, 0, time.UTC)
	chips := VitalChips(VitalChipsParams{
		Status: domain.WorktreeStatus{
			Branch: "feat/x", IsDirty: true, CommitsAhead: 3,
			OriginAhead: 2, OriginBehind: 1, OriginState: domain.DivergenceDiverged,
		},
		LastCommitAt: now.Add(-3 * time.Hour),
		Now:          now,
	})

	if len(chips) == 0 {
		t.Fatal("aucun chip")
	}
	if !chips[0].State {
		t.Error("le premier chip doit être l'état : c'est la lecture la plus rapide")
	}
	for i, chip := range chips[1:] {
		if chip.State {
			t.Errorf("chip[%d] est marqué State — l'état est le seul chip coloré", i+1)
		}
	}
}

func TestVitalChipsNeverMentionsCreated(t *testing.T) {
	now := time.Now()
	chips := VitalChips(VitalChipsParams{
		Status:       domain.WorktreeStatus{Branch: "feat/x", CreatedAt: now.Add(-48 * time.Hour)},
		LastCommitAt: now.Add(-time.Hour),
		Now:          now,
	})
	for _, chip := range chips {
		if chip.Text == "" {
			t.Error("un chip vide ne doit pas être émis")
		}
		if len(chip.Text) >= 7 && chip.Text[:7] == "created" {
			t.Error("created appartient à LINKS, pas à la bande vitale")
		}
	}
}
```

- [ ] **Step 2: Lancer les tests et vérifier qu'ils échouent**

Run: `go test ./internal/rules/ -run 'TestDetailSections|TestFitSections|TestVitalChips' -v`
Expected: FAIL — `undefined: DetailSections`.

- [ ] **Step 3: Déclarer les types et constantes**

Dans `internal/domain/detail.go` :

```go
// Chip est un élément de la bande vitale. State marque le seul chip coloré :
// l'état du working tree.
type Chip struct {
	Text  string
	State bool
	Kind  ChipKind
}

type ChipKind string

const (
	ChipKindClean    ChipKind = "clean"
	ChipKindDirty    ChipKind = "dirty"
	ChipKindRebasing ChipKind = "rebasing"
	ChipKindNeutral  ChipKind = "neutral"
)

// DetailSection est un bloc du panneau, déjà réduit à ses lignes de texte. Le
// rendu ne décide de rien : il stylise et il empile.
type DetailSection struct {
	Key   string
	Title string
	Lines []string
}
```

Dans `internal/domain/constants.go` :

```go
	DetailSectionReview   = "REVIEW"
	DetailSectionChanges  = "CHANGES"
	DetailSectionActivity = "ACTIVITY"
	DetailSectionLinks    = "LINKS"

	// DetailSectionDropOrder est l'ordre dans lequel les sections cèdent leur
	// place quand le panneau manque de hauteur : la dernière listée tombe en
	// premier. La bande vitale et la ligne de blockers n'y figurent pas — elles ne
	// tombent jamais.
	DetailYouAreHere = "● you are here"
	DetailMoreFmt    = "…  %d more"
	DetailBlockedFmt = "⚠ cannot be deleted — %s"
```

Déclarer aussi `DetailSectionDropOrder = []string{DetailSectionReview, DetailSectionChanges, DetailSectionActivity, DetailSectionLinks}` — un `var`, pas un `const`, dans `internal/domain/detail.go` puisque c'est une slice.

- [ ] **Step 4: Implémenter**

Dans `internal/rules/detailpanel.go` :

```go
type VitalChipsParams struct {
	Status       domain.WorktreeStatus
	LastCommitAt time.Time
	Now          time.Time
}

// VitalChips construit la bande vitale. L'état vient en tête — c'est la lecture
// la plus rapide — et il est le seul chip coloré. `created` n'y figure jamais :
// il appartient à LINKS.
func VitalChips(params VitalChipsParams) []domain.Chip {
	chips := []domain.Chip{stateChip(params.Status)}

	if params.Status.CommitsAhead > 0 {
		chips = append(chips, domain.Chip{
			Text: fmt.Sprintf(domain.ChipBaseFmt, params.Status.CommitsAhead),
			Kind: domain.ChipKindNeutral,
		})
	}
	if origin := originChipText(params.Status); origin != "" {
		chips = append(chips, domain.Chip{Text: origin, Kind: domain.ChipKindNeutral})
	}
	if age := RelativeAge(RelativeAgeParams{At: params.LastCommitAt, Now: params.Now}); age != "" {
		chips = append(chips, domain.Chip{
			Text: fmt.Sprintf(domain.ChipActiveFmt, age),
			Kind: domain.ChipKindNeutral,
		})
	}
	return chips
}

// stateChip : un rebase en pause l'emporte sur "sale". Une branche à mi-rebase est
// toujours sale, mais c'est le rebase qui est actionnable.
func stateChip(status domain.WorktreeStatus) domain.Chip {
	if status.RebaseInProgress {
		return domain.Chip{Text: domain.ChipRebasing, State: true, Kind: domain.ChipKindRebasing}
	}
	if status.IsDirty {
		return domain.Chip{Text: domain.ChipDirty, State: true, Kind: domain.ChipKindDirty}
	}
	return domain.Chip{Text: domain.ChipClean, State: true, Kind: domain.ChipKindClean}
}

func originChipText(status domain.WorktreeStatus) string {
	switch status.OriginState {
	case domain.DivergenceUnknown:
		return ""
	case domain.DivergenceDiverged:
		return fmt.Sprintf(domain.ChipOriginDivergedFmt, status.OriginAhead, status.OriginBehind)
	case domain.DivergenceAhead:
		return fmt.Sprintf(domain.ChipOriginAheadFmt, status.OriginAhead)
	case domain.DivergenceBehind:
		return fmt.Sprintf(domain.ChipOriginBehindFmt, status.OriginBehind)
	default:
		return ""
	}
}

type DetailSectionsParams struct {
	Status domain.WorktreeStatus
	Detail domain.WorktreeDetail
	PR     *domain.PRInfo
	Parent string
	Height int
}

// DetailSections émet les sections dans un ordre fixe, et n'émet que celles qui
// ont quelque chose à dire : la position d'une section varie d'un worktree à
// l'autre, jamais son rang.
func DetailSections(params DetailSectionsParams) []domain.DetailSection {
	sections := make([]domain.DetailSection, 0, 4)

	if params.PR != nil {
		sections = append(sections, reviewSection(*params.PR))
	}
	if changed := changedCount(params.Detail.Changes); changed > 0 {
		sections = append(sections, changesSection(params.Detail.Changes, listBudget(params, true)))
	}
	if len(params.Detail.Commits) > 0 {
		sections = append(sections, activitySection(params.Detail.Commits, listBudget(params, false)))
	}
	return append(sections, linksSection(params))
}

func changedCount(changes domain.WorkingChanges) int {
	return changes.Modified + changes.Untracked + changes.Staged
}

// listBudget répartit la hauteur restante entre CHANGES et ACTIVITY : un worktree
// sale la donne aux fichiers (ce sur quoi on travaille), un worktree propre aux
// commits (ce que la branche est).
func listBudget(params DetailSectionsParams, forChanges bool) int {
	room := max(params.Height-domain.DetailFixedRows, domain.DetailMinListRows)
	dirty := changedCount(params.Detail.Changes) > 0 && params.Status.IsDirty
	if forChanges == dirty {
		return room - domain.DetailMinListRows
	}
	return domain.DetailMinListRows
}

type FitSectionsParams struct {
	Sections []domain.DetailSection
	Height   int
}

// FitSections retire les sections par la fin de DetailSectionDropOrder tant que
// l'empilement dépasse la hauteur. La bande vitale et la ligne de blockers ne
// passent jamais par ici : elles ne tombent pas.
func FitSections(params FitSectionsParams) []domain.DetailSection {
	kept := params.Sections
	for sectionsHeight(kept) > params.Height && len(kept) > 0 {
		kept = dropLowestPriority(kept)
	}
	return kept
}

// sectionsHeight compte, par section, son titre, la ligne vide sous lui et ses
// lignes de corps.
func sectionsHeight(sections []domain.DetailSection) int {
	total := 0
	for _, section := range sections {
		total += domain.DetailSectionChrome + len(section.Lines)
	}
	return total
}

func dropLowestPriority(sections []domain.DetailSection) []domain.DetailSection {
	for i := len(domain.DetailSectionDropOrder) - 1; i >= 0; i-- {
		key := domain.DetailSectionDropOrder[i]
		for index, section := range sections {
			if section.Key == key {
				return append(sections[:index:index], sections[index+1:]...)
			}
		}
	}
	return nil
}
```

Écrire ensuite `reviewSection`, `changesSection`, `activitySection` et `linksSection` : chacune retourne une `domain.DetailSection` dont les `Lines` sont déjà du **texte brut** (le rendu ne fait que styliser), coupe sa liste à son budget avec `domain.DetailMoreFmt`, et omet toute ligne dont la valeur est vide — `linksSection` n'émet `Children` que s'il y en a, `Env` que si `EnvDrift.Configured`, et bascule sur `domain.DashboardUnavailableFmt` quand la famille figure dans `Detail.Failures`.

Ajouter les constantes correspondantes (`ChipBaseFmt`, `ChipActiveFmt`, `ChipOriginAheadFmt`, `ChipOriginBehindFmt`, `ChipOriginDivergedFmt`, `ChipClean`, `ChipDirty`, `ChipRebasing`, `DetailFixedRows`, `DetailMinListRows`, `DetailSectionChrome`) dans `internal/domain/constants.go`.

- [ ] **Step 5: Vérifier que les tests passent**

Run: `go test ./internal/rules/ -v`
Expected: PASS.

- [ ] **Step 6: Valider et commiter**

```bash
go build ./... && go vet ./... && go test ./internal/rules/
git add internal/rules/detailpanel.go internal/rules/detailpanel_test.go internal/domain/
git commit -m "feat(rules): composition du panneau détail, sections conditionnelles et repli"
```

---

### Task 8: Le chargement du détail et le contrat de fraîcheur

**Files:**
- Create: `internal/tui/dashboard/detailload.go`
- Modify: `internal/tui/dashboard/dashboard.go`
- Modify: `internal/tui/components/loading.go`
- Test: `internal/tui/dashboard/detailload_test.go`

**Interfaces:**
- Consumes: `worktree.Detail`, `rules.ActiveWorktree`.
- Produces: sur `Model` — `details map[string]domain.WorktreeDetail`, `detailLoading string`, `detailSince time.Time` ; les messages `detailMsg{branch, detail}` et `detailTickMsg` ; `components.MutedSpinner() spinner.Model`.

Les trois règles du spec §7 : **jamais dans le poll**, **débounce 150 ms**, **cache par branche invalidé par le poll et par la fin d'une opération**. Plus le délai d'apparition de 200 ms du §8.

- [ ] **Step 1: Écrire les tests (ils échouent)**

```go
func TestSelectionSchedulesDetailLoad(t *testing.T) {
	model := newTestModel(t, testWidth, testHeight, "main", "feat/a", "feat/b")

	model, cmd := updateCmd(model, key("j"))
	if cmd == nil {
		t.Fatal("changer de sélection doit programmer un chargement de détail")
	}
	if model.detailLoading != "feat/a" {
		t.Errorf("detailLoading = %q, want feat/a", model.detailLoading)
	}
}

func TestCachedDetailSurvivesReload(t *testing.T) {
	model := newTestModel(t, testWidth, testHeight, "main", "feat/a")
	model = update(model, detailMsg{branch: "main", detail: domain.WorktreeDetail{
		Branch:  "main",
		Commits: []domain.CommitSummary{{SHA: "abc1234", Subject: "feat: seed"}},
	}})

	model = update(model, pollMsg{})

	got, ok := model.details["main"]
	if !ok {
		t.Fatal("le poll ne doit pas vider le cache : le panneau afficherait du vide")
	}
	if len(got.Commits) != 1 {
		t.Errorf("Commits = %d, want 1", len(got.Commits))
	}
}

func TestStaleDetailIsMarkedNotErrored(t *testing.T) {
	model := newTestModel(t, testWidth, testHeight, "main")
	model = update(model, detailMsg{branch: "main", detail: domain.WorktreeDetail{Branch: "main"}})
	model.detailLoading = "main"
	model.detailSince = time.Now().Add(-time.Second)

	view := model.View()
	if !strings.Contains(view, domain.DashboardRefreshing) {
		t.Error("un détail en cours de rechargement doit porter son marqueur")
	}
}

func TestFreshDetailShowsNoMarker(t *testing.T) {
	model := newTestModel(t, testWidth, testHeight, "main")
	model = update(model, detailMsg{branch: "main", detail: domain.WorktreeDetail{Branch: "main"}})

	if strings.Contains(model.View(), domain.DashboardRefreshing) {
		t.Error("l'état frais ne se signale pas")
	}
}

func TestMarkerWaitsForTheGraceDelay(t *testing.T) {
	model := newTestModel(t, testWidth, testHeight, "main")
	model.detailLoading = "main"
	model.detailSince = time.Now()

	if strings.Contains(model.View(), domain.DashboardRefreshing) {
		t.Error("sous le délai d'apparition, aucun marqueur : ce serait du flash déguisé en feedback")
	}
}
```

- [ ] **Step 2: Lancer les tests et vérifier qu'ils échouent**

Run: `go test ./internal/tui/dashboard/ -run 'Detail|Stale|Marker' -v`
Expected: FAIL — champs et messages indéfinis.

- [ ] **Step 3: Ajouter les constantes et exporter le spinner**

Dans `internal/domain/constants.go` :

```go
	// DashboardDetailDebounce retarde le chargement du détail pour qu'un
	// parcours rapide de la liste ne lance pas un git log par ligne traversée.
	DashboardDetailDebounce = 150 * time.Millisecond
	// DashboardSpinnerGrace est le délai avant qu'un marqueur de chargement
	// apparaisse : en dessous, la donnée arrive avant lui.
	DashboardSpinnerGrace = 200 * time.Millisecond

	DashboardRefreshing   = "refreshing"
	DashboardLoadingField = "loading…"
	DashboardNotConfigured = "not configured"
	DashboardUnavailableFmt = "unavailable — %s"
```

Dans `internal/tui/components/loading.go`, exposer le spinner standard sans dupliquer sa définition :

```go
// MutedSpinner est le spinner du projet, pour les surfaces qui animent leur
// propre attente plutôt que d'utiliser RunLoading.
func MutedSpinner() spinner.Model { return newMutedSpinner() }
```

- [ ] **Step 4: Implémenter le chargement**

Dans `internal/tui/dashboard/detailload.go` :

- les champs de `Model` : `details map[string]domain.WorktreeDetail`, `detailLoading string`, `detailSince time.Time`, `spinner spinner.Model` ;
- `scheduleDetail(branch string) tea.Cmd` — `tea.Tick(domain.DashboardDetailDebounce, …)` qui n'émet le chargement que si `branch` est **toujours** la sélection à l'échéance ;
- `loadDetailCmd(branch string) tea.Cmd` — appelle `worktree.Detail` dans une goroutine et retourne un `detailMsg` ;
- `case detailMsg:` dans `Update` — écrit dans `m.details[msg.branch]`, remet `detailLoading` à `""` ;
- `case pollMsg:` — **ne vide pas** `m.details` ; il relance un chargement pour la seule branche sélectionnée ;
- `detailIsStale() bool` — `m.detailLoading == m.selectedBranch() && time.Since(m.detailSince) > domain.DashboardSpinnerGrace`.

`Update` doit aussi propager `spinner.TickMsg` vers `m.spinner`, sinon le marqueur reste figé.

- [ ] **Step 5: Vérifier que les tests passent**

Run: `go test ./internal/tui/dashboard/ -v`
Expected: PASS.

- [ ] **Step 6: Valider et commiter**

```bash
go build ./... && go vet ./... && go test ./internal/tui/...
git add internal/tui/dashboard/detailload.go internal/tui/dashboard/detailload_test.go internal/tui/dashboard/dashboard.go internal/tui/components/loading.go internal/domain/constants.go
git commit -m "feat(ui): chargement paresseux du détail et contrat de fraîcheur"
```

---

### Task 9: Le rendu du panneau détail

**Files:**
- Modify: `internal/tui/dashboard/detail.go` (réécriture du corps), `internal/tui/dashboard/render.go`, `internal/styles/dashboard.go`
- Test: `internal/tui/dashboard/detail_test.go` (créer)

**Interfaces:**
- Consumes: `rules.VitalChips`, `rules.DetailSections`, `rules.FitSections`, `domain.DetailSection`, `domain.Chip`.

Le rendu **ne décide de rien** : `rules/` a déjà produit les chips et les sections en texte brut. Ce fichier stylise, empile, et gère le repli chip par chip.

- [ ] **Step 1: Écrire les tests (ils échouent)**

```go
func TestDetailHeaderCarriesIdentityNotState(t *testing.T) {
	model := detailModel(t, domain.WorktreeStatus{Branch: "feat/x", Path: "/wt/x", IsDirty: true})
	lines := strings.Split(model.View(), "\n")

	titleLine := lineContaining(t, lines, "feat/x")
	if strings.Contains(titleLine, "dirty") {
		t.Error("la pastille d'état appartient à la bande vitale, pas à la ligne de titre")
	}
}

func TestDetailOmitsEmptyFields(t *testing.T) {
	model := detailModel(t, domain.WorktreeStatus{Branch: "feat/x", Path: "/wt/x"})
	view := model.View()

	for _, dead := range []string{"PR       none", "Parent   —", "up to date"} {
		if strings.Contains(view, dead) {
			t.Errorf("le panneau ne doit plus réciter %q", dead)
		}
	}
}

func TestVitalStripWrapsChipByChip(t *testing.T) {
	model := detailModel(t, domain.WorktreeStatus{
		Branch: "feat/x", Path: "/wt/x", IsDirty: true, CommitsAhead: 3,
		OriginAhead: 2, OriginBehind: 1, OriginState: domain.DivergenceDiverged,
	})
	model = update(model, tea.WindowSizeMsg{Width: narrowWide, Height: testHeight})

	if strings.Contains(model.View(), "↓…") {
		t.Error("un chip ne se coupe jamais au milieu : ce serait un mensonge, pas une troncature")
	}
}

func TestFirstLoadReservesHeightInsteadOfJumping(t *testing.T) {
	model := detailModel(t, domain.WorktreeStatus{Branch: "feat/x", Path: "/wt/x"})
	model.detailLoading = "feat/x"
	model.detailSince = time.Now().Add(-time.Second)

	before := len(strings.Split(model.View(), "\n"))
	model = update(model, detailMsg{branch: "feat/x", detail: domain.WorktreeDetail{
		Branch:  "feat/x",
		Commits: []domain.CommitSummary{{SHA: "abc1234", Subject: "feat: x"}},
	}})
	after := len(strings.Split(model.View(), "\n"))

	if before != after {
		t.Errorf("le panneau saute de %d à %d lignes — le placeholder doit réserver la hauteur", before, after)
	}
	if !strings.Contains(model.View(), "abc1234") {
		t.Error("la donnée arrivée doit remplacer le placeholder")
	}
}

func TestLegitimateAbsenceIsNotAFailure(t *testing.T) {
	model := detailModel(t, domain.WorktreeStatus{Branch: "feat/x", Path: "/wt/x"})
	model = update(model, detailMsg{branch: "feat/x", detail: domain.WorktreeDetail{
		Branch:   "feat/x",
		EnvDrift: domain.EnvDriftSummary{Configured: false},
	}})

	view := model.View()
	if !strings.Contains(view, domain.DashboardNotConfigured) {
		t.Error("aucun fichier env configuré se dit, et ne se présente pas comme un succès")
	}
	if strings.Contains(view, "unavailable") {
		t.Error("une absence légitime ne doit pas se lire comme une panne")
	}
}

func TestFailureSaysWhy(t *testing.T) {
	model := detailModel(t, domain.WorktreeStatus{Branch: "feat/x", Path: "/wt/x"})
	model = update(model, detailMsg{branch: "feat/x", detail: domain.WorktreeDetail{
		Branch:   "feat/x",
		Failures: map[domain.DetailFamily]error{domain.DetailFamilyEnv: errors.New("git error")},
	}})

	view := model.View()
	if !strings.Contains(view, "unavailable") || !strings.Contains(view, "git error") {
		t.Error("une famille en panne dit pourquoi : elle ne devient jamais vide en silence")
	}
}

func TestBlockersRenderAboveSections(t *testing.T) {
	model := detailModel(t, domain.WorktreeStatus{Branch: "feat/x", Path: "/wt/x", IsDirty: true})
	model = update(model, detailMsg{branch: "feat/x", detail: domain.WorktreeDetail{
		Branch:   "feat/x",
		Blockers: []domain.CleanBlocker{{Label: "uncommitted changes"}},
	}})

	view := model.View()
	blockerAt := strings.Index(view, "uncommitted changes")
	linksAt := strings.Index(view, domain.DetailSectionLinks)
	if blockerAt < 0 || linksAt < 0 || blockerAt > linksAt {
		t.Error("les blockers se lisent avant les sections : ils répondent à « pourquoi l'action est refusée »")
	}
}
```

Écrire les helpers `detailModel(t, status)` (un modèle avec ce seul worktree sélectionné) et `lineContaining(t, lines, needle)` en tête du fichier, dans le style des helpers existants de `dashboard_test.go`.

- [ ] **Step 2: Lancer les tests et vérifier qu'ils échouent**

Run: `go test ./internal/tui/dashboard/ -run 'TestDetail|TestVital|TestBlockers' -v`
Expected: FAIL.

- [ ] **Step 3: Ajouter les styles**

Dans `internal/styles/dashboard.go` — et **appliquer la discipline à trois rôles** du spec §4 dans le même passage :

```go
	// DashboardSectionTitle sépare deux groupes de champs : c'est de la
	// structure, pas un accent. Il ne prend donc pas la couleur de navigation.
	DashboardSectionTitle = lipgloss.NewStyle().Foreground(ColorMuted).Bold(true)

	// DashboardHeaderButton est l'action secondaire du header : muted, pour que
	// le CTA rempli reste le seul appel de la barre.
	DashboardHeaderButton = lipgloss.NewStyle().Foreground(ColorMuted).Bold(true)

	// DashboardAddButton est l'appel à l'action, et le seul usage de l'accent
	// signature avec le wordmark.
	DashboardAddButton = lipgloss.NewStyle().
		Foreground(ColorBadgeFg).Background(ColorSignature).Bold(true).Padding(0, 3)

	DashboardWordmark = lipgloss.NewStyle().Foreground(ColorSignature).Bold(true).Padding(0, 1)

	// DashboardChip rend un élément non-état de la bande vitale, DashboardChipSep
	// ce qui les sépare. L'état a ses propres styles, colorés.
	DashboardChip    = lipgloss.NewStyle().Foreground(ColorMuted)
	DashboardChipSep = lipgloss.NewStyle().Foreground(ColorMuted)

	// DashboardBlockers porte la ligne des refus de suppression.
	DashboardBlockers = lipgloss.NewStyle().Foreground(ColorWarning)

	// DashboardStale atténue un corps de panneau dont la donnée est en cours de
	// rechargement : il ne bouge pas, il se déclare.
	DashboardStale = lipgloss.NewStyle().Foreground(ColorMuted)

	// DashboardTreeNode passe en muted : une puce est de la structure. C'est
	// l'état du nœud qui la colore quand il en a un.
	DashboardTreeNode = lipgloss.NewStyle().Foreground(ColorMuted)
)
```

- [ ] **Step 4: Réécrire `detailBody`**

`detailBody` devient : ligne de titre (branche + `DetailYouAreHere` à droite quand c'est le worktree actif) → règle → bande vitale (chips, repli chip par chip) → ligne de blockers si `len(Blockers) > 0` → `FitSections(DetailSections(…))` empilées. Le corps entier passe par `styles.DashboardStale` quand `detailIsStale()`.

Les quatre états du spec §8 se rendent ici, et ce sont eux que testent les Steps 1-2 :

1. **frais** — aucun ornement ;
2. **rechargement** — corps en `DashboardStale`, marqueur `DashboardRefreshing` dans le titre, **hauteur inchangée** ;
3. **premier affichage** — quand `m.details[branch]` n'existe pas encore, les sections dépendant du détail rendent `domain.DashboardLoadingField` **sur le même nombre de lignes** que le contenu qu'elles remplaceront, pour que le panneau ne saute pas à l'arrivée ;
4. **indisponible** — une famille présente dans `Detail.Failures` rend `fmt.Sprintf(domain.DashboardUnavailableFmt, err)` ; une absence légitime (`EnvDrift.Configured == false`) rend `domain.DashboardNotConfigured`, sans glyphe d'alerte. Les deux sont muted, mais **ils ne disent pas la même chose**.

Supprimer `detailSections`, `baseText`, `originText`, `createdText` de `detail.go` : leur logique vit désormais dans `rules/`. Aligner les commentaires du fichier sur la règle « pourquoi, jamais quoi » en le faisant.

- [ ] **Step 5: Vérifier que les tests passent**

Run: `go test ./internal/tui/dashboard/ -v && go test ./internal/styles/`
Expected: PASS.

- [ ] **Step 6: Vérification visuelle**

```bash
go build -o /tmp/wtm-preview . && /tmp/wtm-preview ui
```

Comparer avec la maquette du spec §6 : titre sans pastille, bande vitale à un seul élément coloré, aucune section vide, blockers hauts.

- [ ] **Step 7: Valider et commiter**

```bash
go build ./... && go vet ./... && go test ./...
git add internal/tui/dashboard/detail.go internal/tui/dashboard/detail_test.go internal/styles/dashboard.go
git commit -m "feat(ui): panneau détail en zones de priorité et discipline de couleur"
```

---

## Phase 3 — Chrome et identité

### Task 10: Le header contextuel, et la réparation de `wtm list`

**Files:**
- Modify: `internal/tui/dashboard/render.go`, `internal/tui/dashboard/dashboard.go`, `internal/domain/dashboard.go`, `internal/rules/dashboard.go`, `internal/commands/wt/list.go`
- Test: `internal/tui/dashboard/render_test.go` (créer), `internal/rules/dashboard_test.go`

**Interfaces:**
- Consumes: `rules.ActiveWorktree`, `rules.FetchIsStale`, `rules.RelativeAge`, `infra.LastFetchAt`.

Le header passe de 2 à 3 lignes (`DashboardHeaderHeight`). **Ligne 1 = où tu es, ligne 2 = ce que tu peux faire**, pas de règle entre les deux : la règle d'onglet fait déjà la séparation.

`internal/commands/wt/list.go:107` passe `ActiveBranch: ""` en dur — le `● active` de `wtm list` est du code mort depuis toujours. On le renseigne ici avec `rules.ActiveWorktree`.

- [ ] **Step 1: Écrire les tests (ils échouent)**

```go
func TestHeaderCarriesRepoAndActiveWorktree(t *testing.T) {
	model := newTestModel(t, testWidth, testHeight, "main", "feat/a")
	model.repoName = "worktree-manager-cli"
	model.baseBranch = "main"
	model.activeBranch = "feat/a"

	first := strings.Split(model.View(), "\n")[0]
	for _, want := range []string{"wtm", "worktree-manager-cli", "main", "feat/a"} {
		if !strings.Contains(first, want) {
			t.Errorf("ligne 1 du header = %q, doit contenir %q", first, want)
		}
	}
}

func TestHeaderAnnouncesStaleFetchOnlyPastTheThreshold(t *testing.T) {
	model := newTestModel(t, testWidth, testHeight, "main")

	model.fetchedAt = time.Now().Add(-time.Hour)
	if strings.Contains(model.View(), "fetched") {
		t.Error("un fetch récent ne s'annonce pas : un marqueur permanent ne signale plus rien")
	}

	model.fetchedAt = time.Now().Add(-72 * time.Hour)
	if !strings.Contains(model.View(), "fetched") {
		t.Error("au-delà du seuil, la vue doit déclarer que ses refs origin sont périmées")
	}
}

func TestHeaderDropsSegmentsRightToLeftWhenNarrow(t *testing.T) {
	model := newTestModel(t, narrowWide, testHeight, "main")
	model.repoName = "un-nom-de-depot-vraiment-tres-long"
	model.baseBranch = "main"
	model.activeBranch = "feat/une-branche-au-nom-interminable"
	model.fetchedAt = time.Now().Add(-72 * time.Hour)

	first := strings.Split(model.View(), "\n")[0]
	if lipgloss.Width(first) > narrowWide {
		t.Errorf("ligne 1 large de %d, max %d", lipgloss.Width(first), narrowWide)
	}
	if !strings.Contains(first, domain.DashboardWordmark) {
		t.Error("le wordmark est le dernier segment à tomber")
	}
}
```

- [ ] **Step 2: Lancer les tests et vérifier qu'ils échouent**

Run: `go test ./internal/tui/dashboard/ -run TestHeader -v`
Expected: FAIL.

- [ ] **Step 3: Passer le header à 3 lignes**

Dans `internal/domain/dashboard.go`, `DashboardHeaderHeight = 3`, en expliquant **pourquoi** dans le commentaire (ligne de contexte + ligne d'onglets + règle). Ajouter dans `constants.go` :

```go
	DashboardContextSep  = " · "
	DashboardFetchedFmt  = "fetched %s"
	DashboardActiveGlyph = "●"
	DashboardBaseFmt     = "base %s"
```

- [ ] **Step 4: Implémenter `renderContextLine` et brancher les champs**

Ajouter à `Model` : `repoName`, `baseBranch`, `activeBranch string`, `fetchedAt time.Time`, renseignés au démarrage (`filepath.Base(params.ProjectDir)`, `Config.Project.Worktrees.BaseBranch`, `rules.ActiveWorktree` sur le cwd, `infra.LastFetchAt`) et rafraîchis sur `r`.

`renderContextLine(width int) string` construit les segments et les **lâche de droite à gauche** — `fetched` → worktree actif → base → nom du dépôt → wordmark — en réutilisant la mécanique de variantes de `headerRight`. Comme pour les boutons, on **laisse tomber un segment entier** plutôt que de trimer la ligne : une coupe franche traverserait un marqueur de zone.

`renderHeader` retourne alors `JoinVertical(contextLine, tabsBar, tabRule)`.

- [ ] **Step 5: Réparer `wtm list`**

Dans `internal/commands/wt/list.go`, remplacer `ActiveBranch: ""` par la valeur calculée :

```go
			ActiveBranch: rules.ActiveWorktree(rules.ActiveWorktreeParams{
				Cwd:      dir,
				Statuses: statuses,
			}),
```

- [ ] **Step 6: Vérifier que les tests passent**

Run: `go test ./internal/tui/dashboard/ ./internal/commands/... ./internal/rules/ -v`
Expected: PASS.

- [ ] **Step 7: Vérification manuelle des deux surfaces**

```bash
go build -o /tmp/wtm-preview . && cd <un worktree> && /tmp/wtm-preview list && /tmp/wtm-preview ui
```

`list` doit marquer `● active` sur le bon worktree ; le header du dashboard doit nommer le dépôt et le même worktree.

- [ ] **Step 8: Valider et commiter**

```bash
go build ./... && go vet ./... && go test ./...
git add internal/tui/dashboard/ internal/domain/ internal/rules/ internal/commands/wt/list.go
git commit -m "feat(ui): header contextuel et marqueur de worktree actif"
```

---

### Task 11: La liste — marqueur actif et opération en cours

**Files:**
- Modify: `internal/tui/dashboard/list.go`, `internal/tui/dashboard/ops.go`
- Test: `internal/tui/dashboard/list_test.go` (créer)

**Interfaces:**
- Consumes: `Model.activeBranch`, `Model.ops` (la cible verrouillée), `components.MutedSpinner`.

`ops.go` connaît déjà la cible qu'une opération en background verrouille et la liste n'en montre rien.

- [ ] **Step 1: Écrire les tests (ils échouent)**

```go
func TestActiveRowIsMarked(t *testing.T) {
	model := newTestModel(t, testWidth, testHeight, "main", "feat/a")
	model.activeBranch = "feat/a"

	row := lineContaining(t, strings.Split(model.View(), "\n"), "feat/a")
	if !strings.Contains(row, domain.DashboardActiveGlyph) {
		t.Error("la ligne du worktree actif doit porter son marqueur")
	}
}

func TestLockedRowShowsProgressNotState(t *testing.T) {
	model := newTestModel(t, testWidth, testHeight, "main", "feat/a")
	model = lockOperation(t, model, "feat/a", "running on_create hook…")

	row := lineContaining(t, strings.Split(model.View(), "\n"), "feat/a")
	if !strings.Contains(row, "running on_create hook…") {
		t.Error("une ligne verrouillée montre l'étape en cours")
	}
	if strings.Contains(row, "clean") || strings.Contains(row, "dirty") {
		t.Error("la pastille d'état cède sa place au spinner pendant l'opération")
	}
}
```

Écrire `lockOperation(t, model, branch, stage)` en s'appuyant sur ce que `ops.go` expose réellement (lire `internal/tui/dashboard/ops.go` et `ops_test.go` d'abord).

- [ ] **Step 2: Lancer les tests et vérifier qu'ils échouent**

Run: `go test ./internal/tui/dashboard/ -run 'TestActiveRow|TestLockedRow' -v`
Expected: FAIL.

- [ ] **Step 3: Implémenter**

Dans `renderRow` : préfixer le nom de la branche active par `domain.DashboardActiveGlyph` ; quand `m.ops` verrouille cette branche, remplacer la pastille par `m.spinner.View()` et la ligne meta par l'étape en cours. Ajouter les constantes d'étape dans `domain/constants.go` plutôt que des littéraux.

- [ ] **Step 4: Vérifier que les tests passent**

Run: `go test ./internal/tui/dashboard/ -v`
Expected: PASS.

- [ ] **Step 5: Valider et commiter**

```bash
go build ./... && go vet ./... && go test ./...
git add internal/tui/dashboard/list.go internal/tui/dashboard/ops.go internal/tui/dashboard/list_test.go internal/domain/constants.go
git commit -m "feat(ui): marqueur de worktree actif et progression en ligne"
```

---

### Task 12: La PR dépliée — checks CI et décision de review

**Files:**
- Modify: `internal/domain/github.go`, `internal/domain/constants.go`, `internal/service/github/pr.go`, `internal/rules/detailpanel.go`
- Test: `internal/service/github/pr_test.go`, `internal/rules/detailpanel_test.go`

**Interfaces:**
- Produces: `domain.PRInfo.Checks domain.PRChecks` et `domain.PRInfo.ReviewDecision string`.

`gh` reste appelé en async et hors du poll : on **élargit le payload**, on n'ajoute pas d'appel.

- [ ] **Step 1: Écrire le test de la section REVIEW (il échoue)**

```go
func TestReviewSectionShowsChecksAndDecision(t *testing.T) {
	sections := DetailSections(DetailSectionsParams{
		Status: domain.WorktreeStatus{Branch: "feat/x", Path: "/wt/x"},
		PR: &domain.PRInfo{
			Number: 67, Title: "feat: x", State: "OPEN",
			Checks:         domain.PRChecks{Passed: 12, Failed: 1},
			ReviewDecision: "CHANGES_REQUESTED",
		},
	})

	var review domain.DetailSection
	for _, section := range sections {
		if section.Key == domain.DetailSectionReview {
			review = section
		}
	}
	if review.Key == "" {
		t.Fatal("REVIEW absente alors qu'une PR existe")
	}

	body := strings.Join(review.Lines, "\n")
	for _, want := range []string{"#67", "feat: x", "12", "changes requested"} {
		if !strings.Contains(body, want) {
			t.Errorf("REVIEW = %q, doit contenir %q", body, want)
		}
	}
}

func TestReviewSectionWithoutChecks(t *testing.T) {
	sections := DetailSections(DetailSectionsParams{
		Status: domain.WorktreeStatus{Branch: "feat/x", Path: "/wt/x"},
		PR:     &domain.PRInfo{Number: 68, Title: "feat: y", State: "OPEN"},
	})
	for _, section := range sections {
		if section.Key != domain.DetailSectionReview {
			continue
		}
		if strings.Contains(strings.Join(section.Lines, "\n"), "checks") {
			t.Error("pas de ligne checks quand aucun check n'a tourné")
		}
	}
}
```

- [ ] **Step 2: Lancer le test et vérifier qu'il échoue**

Run: `go test ./internal/rules/ -run TestReviewSection -v`
Expected: FAIL — `unknown field Checks`.

- [ ] **Step 3: Élargir le payload `gh`**

Dans `internal/domain/github.go` :

```go
// PRChecks agrège le rollup de statuts d'une PR. Total distingue "aucun check
// configuré" de "checks tous en attente".
type PRChecks struct {
	Passed  int
	Failed  int
	Pending int
}
```

Ajouter `Checks PRChecks` et `ReviewDecision string` à `PRInfo` (avec leurs tags JSON).

Dans `internal/domain/constants.go`, étendre le champ set :

```go
	GHPRFields = "number,title,author,headRefName,baseRefName,url,isCrossRepository,isDraft,reviewDecision,statusCheckRollup"
```

Dans `internal/service/github/pr.go`, ajouter les champs correspondants à `ghPR` et agréger le rollup dans `convertGHPR` : chaque entrée porte un `conclusion` (`SUCCESS`, `FAILURE`, …) ou un `state`, à répartir en `Passed` / `Failed` / `Pending`.

- [ ] **Step 4: Rendre la section REVIEW**

Dans `rules.DetailSections`, la section `REVIEW` produit une ligne `#N titre … ÉTAT`, puis une seconde ligne `checks ✓ P ✗ F · review <décision>` — **chaque moitié omise quand elle n'a rien à dire**. Les glyphes et la traduction des décisions (`CHANGES_REQUESTED` → `changes requested`) sont des constantes.

- [ ] **Step 5: Vérifier que les tests passent**

Run: `go test ./internal/rules/ ./internal/service/github/ -v`
Expected: PASS.

- [ ] **Step 6: Valider et commiter**

```bash
go build ./... && go vet ./... && go test ./...
git add internal/domain/ internal/service/github/ internal/rules/
git commit -m "feat(ui): PR dépliée avec checks CI et décision de review"
```

---

### Task 13: Les états vides et le wordmark d'accueil

**Files:**
- Create: `internal/tui/dashboard/welcome.go`
- Modify: `internal/domain/constants.go`, `internal/styles/dashboard.go`, `internal/tui/dashboard/list.go`, `internal/tui/dashboard/outputpanel.go`
- Test: `internal/tui/dashboard/welcome_test.go`

Le wordmark apparaît à **exactement deux endroits, tous deux déjà vides** : le premier lancement et le chargement initial. Aucun splash artificiel — on habite une attente qui existe déjà.

- [ ] **Step 1: Écrire les tests (ils échouent)**

```go
func TestEmptyStatesNameTheNextAction(t *testing.T) {
	model := newTestModel(t, testWidth, testHeight)
	view := model.View()

	if !strings.Contains(view, domain.DashboardEmptyList) {
		t.Fatal("état vide absent")
	}
	if !strings.Contains(view, "press n") {
		t.Error("un état vide nomme l'action suivante au lieu de constater")
	}
}

func TestWelcomeWordmarkOnEmptyRepo(t *testing.T) {
	model := newTestModel(t, testWidth, testHeight)
	if !strings.Contains(model.View(), domain.DashboardWordmarkArt[0]) {
		t.Error("le dépôt sans worktree est le moment où l'identité se transmet")
	}
}

func TestWordmarkAbsentOnceWorktreesExist(t *testing.T) {
	model := newTestModel(t, testWidth, testHeight, "main", "feat/a")
	if strings.Contains(model.View(), domain.DashboardWordmarkArt[0]) {
		t.Error("le wordmark dessiné ne vit pas sur la surface de travail")
	}
}

func TestWordmarkSkippedWhenTooShort(t *testing.T) {
	model := newTestModel(t, testWidth, 10)
	if strings.Contains(model.View(), domain.DashboardWordmarkArt[0]) {
		t.Error("sur un panneau trop court, le message passe avant le décor")
	}
}
```

- [ ] **Step 2: Lancer les tests et vérifier qu'ils échouent**

Run: `go test ./internal/tui/dashboard/ -run 'TestEmpty|TestWelcome|TestWordmark' -v`
Expected: FAIL.

- [ ] **Step 3: Ajouter les constantes**

```go
	DashboardEmptyList      = "No worktrees yet — press n to create your first one."
	DashboardEmptySelection = "Select a worktree to see what's in it."
	DashboardEmptyOutput    = "Output from create, clean and sync runs appears here."
	DashboardWelcomeBody    = "A worktree is a second checkout of this repository, " +
		"with its own branch and its own .env, so you can work on two things at once."
```

Et dans `internal/domain/dashboard.go` :

```go
// DashboardWordmarkArt n'apparaît qu'aux deux endroits déjà vides : l'accueil
// d'un dépôt sans worktree et le chargement initial. Jamais sur la surface de
// travail.
var DashboardWordmarkArt = []string{
	`╻ ╻╺┳╸┏┳┓`,
	`┃╻┃ ┃ ┃┃┃`,
	`┗┻┛ ╹ ╹ ╹`,
}
```

- [ ] **Step 4: Implémenter**

`welcomeBody(width, height int) []string` rend le wordmark (dégradé de `ColorSignature` vers `ColorPrimary` ligne par ligne, via un style par ligne dans `styles/`), la tagline, le corps `DashboardWelcomeBody` et l'invite. Il **omet le wordmark** quand la hauteur disponible ne suffit pas : le message passe avant le décor. Brancher `listBody` sur `welcomeBody` quand `m.loaded && len(m.statuses) == 0`, et sur le wordmark + spinner quand `!m.loaded`.

- [ ] **Step 5: Vérifier que les tests passent**

Run: `go test ./internal/tui/dashboard/ -v`
Expected: PASS.

- [ ] **Step 6: Valider et commiter**

```bash
go build ./... && go vet ./... && go test ./...
git add internal/tui/dashboard/welcome.go internal/tui/dashboard/welcome_test.go internal/tui/dashboard/list.go internal/tui/dashboard/outputpanel.go internal/domain/ internal/styles/dashboard.go
git commit -m "feat(ui): états vides actionnables et accueil identitaire"
```

---

### Task 14: Les animations, leur interrupteur, et la validation finale

**Files:**
- Modify: `internal/domain/config.go`, `internal/domain/constants.go`, `internal/config/`, `internal/rules/dashboard.go`, `internal/tui/dashboard/render.go`, `internal/tui/dashboard/list.go`, `README.md`
- Test: `internal/rules/dashboard_test.go`, `internal/tui/dashboard/render_test.go`

Trois animations, plafonnées à 400 ms, **toutes coupables d'un coup**. Une animation qu'on ne peut pas éteindre est un bug en ssh sur une liaison lente.

- [ ] **Step 1: Écrire les tests (ils échouent)**

```go
func TestAnimationsEnabledByDefault(t *testing.T) {
	if !AnimationsEnabled(domain.Config{}) {
		t.Error("les animations sont actives par défaut : une clé absente n'est pas un refus")
	}
}

func TestAnimationsCanBeDisabled(t *testing.T) {
	off := false
	cfg := domain.Config{Global: domain.GlobalConfig{UI: domain.UIConfig{Animations: &off}}}
	if AnimationsEnabled(cfg) {
		t.Error("ui.animations = false doit tout éteindre")
	}
}

func TestNoAnimationExceedsTheCap(t *testing.T) {
	durations := map[string]time.Duration{
		"glissement d'onglet": domain.DashboardTabSlide,
		"fondu de ligne":      domain.DashboardRowFlash,
		"respiration":         domain.DashboardBreathe,
	}
	for name, got := range durations {
		if got > domain.DashboardAnimationCap {
			t.Errorf("%s dure %v, plafond %v", name, got, domain.DashboardAnimationCap)
		}
	}
}
```

- [ ] **Step 2: Lancer les tests et vérifier qu'ils échouent**

Run: `go test ./internal/rules/ -run TestAnimation -v`
Expected: FAIL — `undefined: AnimationsEnabled`.

- [ ] **Step 3: Ajouter la clé de config et la règle**

Dans `internal/domain/config.go` :

```go
// UIConfig regroupe les préférences d'affichage. Animations est un pointeur pour
// distinguer "absent" (donc actif) de "explicitement false".
type UIConfig struct {
	Animations *bool `toml:"animations"`
}
```

L'ajouter à `GlobalConfig` (`UI UIConfig `toml:"ui"``), la charger dans `internal/config/`, et écrire la règle pure :

```go
// AnimationsEnabled : une clé absente vaut actif. Seul un false explicite éteint.
func AnimationsEnabled(cfg domain.Config) bool {
	if cfg.Global.UI.Animations == nil {
		return true
	}
	return *cfg.Global.UI.Animations
}
```

Ajouter les durées dans `constants.go` : `DashboardAnimationCap = 400 * time.Millisecond`, `DashboardTabSlide = 200 * time.Millisecond`, `DashboardRowFlash = 400 * time.Millisecond`, `DashboardBreathe = 400 * time.Millisecond`.

- [ ] **Step 4: Implémenter les trois animations**

Chacune est une séquence de `tea.Tick` **bornée**, qui ne consomme aucune touche et n'entre pas dans le chemin d'une action ; chacune est court-circuitée quand `rules.AnimationsEnabled` est faux :

1. la règle d'onglet glisse de l'ancien onglet vers le nouveau (interpolation de `ActiveStart` dans `tabRule`) ;
2. la ligne d'un worktree qui vient d'être créé s'allume puis s'éteint (teinte décroissante sur `DashboardRowFlash`, déclenchée par `selectBranch`) ;
3. le dégradé du wordmark respire **uniquement** dans l'écran d'accueil vide.

- [ ] **Step 5: Vérifier que les tests passent**

Run: `go test ./... -v`
Expected: PASS.

- [ ] **Step 6: Documenter la clé de config**

Ajouter `ui.animations` à la section Configuration du `README.md`. **Ne pas** lancer `make docs` : aucune commande ni aucun flag n'a changé, et `docs/` est généré depuis l'arbre Cobra.

Vérifier ensuite si `internal/commands/agents/assets/using-wtm.skill.md` doit bouger : la surface d'invocation (commandes, flags, formes `--output json`, sémantiques d'abandon) est **inchangée** — seuls l'affichage humain et une préférence globale évoluent. Conclusion attendue : **pas de mise à jour de la skill**. Si la revue montre le contraire, la faire dans ce commit.

- [ ] **Step 7: Validation complète**

Lancer le sous-agent **`build-validator`** (compilation, `go vet`, analyse statique, suite de tests), puis la vérification manuelle finale sur les deux thèmes :

```bash
go build -o /tmp/wtm-preview . && /tmp/wtm-preview ui
```

Contrôler point par point contre le spec : header à 3 lignes qui nomme le dépôt et le worktree actif ; panneau détail sans section vide ; les quatre états de fraîcheur ; `ui.animations = false` qui éteint bien tout.

- [ ] **Step 8: Commiter**

```bash
git add internal/ README.md
git commit -m "feat(ui): animations bornées et interrupteur ui.animations"
```

---

## Suivi (hors de ce plan)

- **Approche B** — liste à 30 %, détail sur deux colonnes au-delà de ~140 colonnes : un ajout à `rules.ComputeDashboardLayout`, décidé après cette passe.
- **`wtm show`** / `list --output json` enrichi : `service/worktree.Detail` a été posé exporté pour ça.
- Services `run` dans le détail : écartés tant que le module n'est pas stabilisé.
