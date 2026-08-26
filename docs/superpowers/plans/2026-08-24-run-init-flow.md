# `wtm run init` produit une configuration qui démarre — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Faire de `wtm run init` un assistant qui produit une configuration démarrable — jobs choisis, ports pré-remplis, profils composés — au lieu d'un inventaire de tous les scripts du repo.

**Architecture:** On inverse l'ordre des questions du wizard (`internal/tui/inittui/project.go`) : l'intention d'abord, et rien de non coché n'est écrit. Toute la logique de décision reste pure dans `internal/rules/` (pré-cochage, proposition de profils, infra partagée) ; le wizard n'orchestre que des `components.Step`. Un nouveau composant `ProfileListModel` porte l'édition groupée (renommer / fusionner / retirer / créer), modelé sur `HookListModel`.

**Tech Stack:** Go 1.22+, Cobra, Bubbletea, Lipgloss. Tests : `go test`, table-driven, pas de mock quand un vrai objet suffit.

**Spec:** `docs/superpowers/specs/2026-08-24-run-init-flow-design.md`

## Global Constraints

Copiées de `CLAUDE.md` et de la spec — elles s'appliquent à **toutes** les tâches :

- `internal/rules/` n'importe que la stdlib et `internal/domain` — aucune I/O, aucun effet de bord.
- `internal/domain/` ne contient que types, erreurs et constantes — aucune méthode, aucune fonction libre.
- `internal/tui/` et `internal/output/` ne décident rien — rendu seulement.
- `internal/styles/` est le seul paquet autorisé à instancier `lipgloss.Style`.
- Toute fonction à 2 paramètres liés ou plus prend une struct unique, initialisée par champs nommés.
- Aucune chaîne ni valeur littérale à portée utilisateur hors de `internal/domain/constants.go`.
- Retours anticipés : aucun `if` imbriqué ; le chemin nominal est en dernier.
- Commentaires quasi nuls — le « pourquoi », jamais le « quoi ».
- Aucune contrainte de compatibilité : le module `run` est expérimental et n'a qu'un utilisateur.
- Lancer le subagent **`build-validator`** avant chaque commit.
- `make docs` après tout changement de commande ou de flag, et mettre à jour `internal/commands/agents/assets/using-wtm.skill.md` si la surface CLI bouge.

---

### Task 1: Pré-cochage conservateur des scripts

**Files:**
- Modify: `internal/rules/scripts.go`
- Modify: `internal/domain/constants.go`
- Test: `internal/rules/scripts_test.go`

**Interfaces:**
- Consumes: `domain.PackageScript`, `rules.ClassifyScriptKind` (existant, inchangé).
- Produces: `rules.PreselectScript(scriptName string) bool` — vrai si le script doit être coché par défaut dans le wizard.

`ClassifyScriptKind` reste tel quel : il décide du `kind` (service/task), pas du cochage. Ce sont deux axes distincts et les confondre est le bug d'aujourd'hui.

- [ ] **Step 1: Écrire le test qui échoue**

Ajouter à `internal/rules/scripts_test.go` :

```go
func TestPreselectScript(t *testing.T) {
	tests := []struct {
		script string
		want   bool
	}{
		{"dev", true},
		{"dev:api", true},
		{"api:dev", true},
		{"web-dev", true},
		{"start", false},
		{"serve", false},
		{"watch", false},
		{"preview", false},
		{"build", false},
		{"lint", false},
		{"format", false},
		{"check-types", false},
	}

	for _, tt := range tests {
		t.Run(tt.script, func(t *testing.T) {
			if got := rules.PreselectScript(tt.script); got != tt.want {
				t.Errorf("PreselectScript(%q) = %v, want %v", tt.script, got, tt.want)
			}
		})
	}
}

func TestPreselectScriptIsIndependentOfKind(t *testing.T) {
	// `start` est classé service — il tourne — mais on ne le coche pas : ce
	// n'est pas ce qu'on lance en dev. Les deux axes ne doivent pas se suivre.
	if rules.ClassifyScriptKind("start") != domain.JobKindService {
		t.Fatal("le test suppose que start reste classé service")
	}
	if rules.PreselectScript("start") {
		t.Error("start ne doit pas être coché par défaut")
	}
}
```

- [ ] **Step 2: Lancer le test et vérifier qu'il échoue**

Run: `go test ./internal/rules/ -run TestPreselectScript -v`
Expected: FAIL — `undefined: rules.PreselectScript`

- [ ] **Step 3: Implémenter**

Ajouter dans `internal/domain/constants.go`, dans le bloc où vit déjà `ScriptKeyDev` :

```go
	// ScriptPreselectKey is the only script name fragment `run init` checks by
	// default. Deliberately blunt: reading the command to guess whether
	// `vite preview` serves requests would rebuild a per-tool heuristic, and
	// maintaining one is what the port probe refused to do for turbo.
	ScriptPreselectKey = ScriptKeyDev
```

Ajouter dans `internal/rules/scripts.go` :

```go
// PreselectScript says whether `run init` checks a script by default. It is not
// ClassifyScriptKind: that one decides whether a job blocks its profile, this
// one decides whether the job is created at all.
func PreselectScript(scriptName string) bool {
	return strings.Contains(strings.ToLower(scriptName), domain.ScriptPreselectKey)
}
```

- [ ] **Step 4: Lancer le test et vérifier qu'il passe**

Run: `go test ./internal/rules/ -run TestPreselectScript -v`
Expected: PASS

- [ ] **Step 5: Valider et commiter**

Lancer le subagent `build-validator`, puis :

```bash
git add internal/rules/scripts.go internal/rules/scripts_test.go internal/domain/constants.go
git commit -m "feat(run): ne pré-cocher que les scripts dev dans run init"
```

---

### Task 2: Proposition de découpage en profils

**Files:**
- Create: `internal/rules/profileplan.go`
- Modify: `internal/domain/constants.go`
- Test: `internal/rules/profileplan_test.go`

**Interfaces:**
- Consumes: `domain.RunConfig`, `domain.JobConfig`, `domain.ProfileConfig`.
- Produces:
  - `rules.ProposeProfiles(params rules.ProposeProfilesParams) []domain.ProfileConfig`
  - `rules.ProposeProfilesParams{ Config domain.RunConfig; Existing []domain.ProfileConfig }`
  - `rules.IsSharedJob(job domain.JobConfig) bool`

- [ ] **Step 1: Écrire le test qui échoue**

Créer `internal/rules/profileplan_test.go` :

```go
package rules_test

import (
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/rules"
)

func names(profiles []domain.ProfileConfig) []string {
	out := make([]string, 0, len(profiles))
	for _, p := range profiles {
		out = append(out, p.Name)
	}
	return out
}

func findProfile(t *testing.T, profiles []domain.ProfileConfig, name string) domain.ProfileConfig {
	t.Helper()
	for _, p := range profiles {
		if p.Name == name {
			return p
		}
	}
	t.Fatalf("no profile %q in %v", name, names(profiles))
	return domain.ProfileConfig{}
}

var (
	composeJob = domain.JobConfig{Name: "docker-compose", Kind: domain.JobKindService, Cwd: "."}
	apiJob     = domain.JobConfig{Name: "api-dev", Kind: domain.JobKindService, Cwd: "apps/api"}
	webJob     = domain.JobConfig{Name: "web-dev", Kind: domain.JobKindService, Cwd: "apps/web"}
	lintJob    = domain.JobConfig{Name: "lint", Kind: domain.JobKindTask, Cwd: "."}
)

func TestIsSharedJob(t *testing.T) {
	if !rules.IsSharedJob(composeJob) {
		t.Error("un job à la racine est de l'infra partagée")
	}
	if rules.IsSharedJob(apiJob) {
		t.Error("un job dans un package n'est pas partagé")
	}
}

func TestProposeProfilesGroupsByPackage(t *testing.T) {
	profiles := rules.ProposeProfiles(rules.ProposeProfilesParams{
		Config: domain.RunConfig{Jobs: []domain.JobConfig{composeJob, apiJob, webJob}},
	})

	if got := names(profiles); len(got) != 3 {
		t.Fatalf("expected 3 profiles (api, web, all), got %v", got)
	}

	api := findProfile(t, profiles, "api")
	if len(api.Jobs) != 2 || api.Jobs[0] != "docker-compose" || api.Jobs[1] != "api-dev" {
		t.Errorf("api = %v, want [docker-compose api-dev]", api.Jobs)
	}

	all := findProfile(t, profiles, domain.ProfileAllName)
	if len(all.Jobs) != 3 {
		t.Errorf("all = %v, want the three jobs", all.Jobs)
	}
	if !all.Default {
		t.Error("le profil global est le default pré-sélectionné dans le picker")
	}
}

func TestProposeProfilesCollapsesInASinglePackageRepo(t *testing.T) {
	// Un profil par package plus un global se confondent quand il n'y a qu'un
	// package : la règle doit dégrader d'elle-même, sans cas particulier.
	profiles := rules.ProposeProfiles(rules.ProposeProfilesParams{
		Config: domain.RunConfig{Jobs: []domain.JobConfig{composeJob, domain.JobConfig{
			Name: "dev", Kind: domain.JobKindService, Cwd: ".",
		}}},
	})

	if len(profiles) != 1 {
		t.Fatalf("expected a single profile, got %v", names(profiles))
	}
	if !profiles[0].Default {
		t.Error("le profil unique doit être le default")
	}
}

func TestProposeProfilesIgnoresTasks(t *testing.T) {
	profiles := rules.ProposeProfiles(rules.ProposeProfilesParams{
		Config: domain.RunConfig{Jobs: []domain.JobConfig{composeJob, apiJob, lintJob}},
	})

	all := findProfile(t, profiles, domain.ProfileAllName)
	for _, job := range all.Jobs {
		if job == "lint" {
			t.Error("une task n'entre pas dans un profil proposé")
		}
	}
}

func TestProposeProfilesKeepsTheExistingSplit(t *testing.T) {
	// Sur un init relancé, la proposition est la configuration en place : on
	// montre ce qu'il y a, on n'infère pas par-dessus une composition faite à
	// la main.
	existing := []domain.ProfileConfig{
		{Name: "app1", Jobs: []string{"docker-compose", "api-dev", "web-dev"}, Default: true},
	}

	profiles := rules.ProposeProfiles(rules.ProposeProfilesParams{
		Config:   domain.RunConfig{Jobs: []domain.JobConfig{composeJob, apiJob, webJob}},
		Existing: existing,
	})

	if len(profiles) != 1 || profiles[0].Name != "app1" {
		t.Fatalf("expected the existing split, got %v", names(profiles))
	}
}
```

- [ ] **Step 2: Lancer le test et vérifier qu'il échoue**

Run: `go test ./internal/rules/ -run 'TestProposeProfiles|TestIsSharedJob' -v`
Expected: FAIL — `undefined: rules.ProposeProfiles`

- [ ] **Step 3: Implémenter**

Ajouter dans `internal/domain/constants.go` :

```go
	// ProfileAllName is the profile gathering every service the init retained.
	// In a single-package repo it is the only one: one profile per package plus
	// a global one collapse into each other, which is what keeps the rule free
	// of a special case.
	ProfileAllName = "all"
	// ProfileRootCwd is the cwd of a job serving every profile — a compose
	// stack, a root script. Sitting at the root is what makes a job shared.
	ProfileRootCwd = "."
```

Créer `internal/rules/profileplan.go` :

```go
package rules

import (
	"path/filepath"
	"sort"

	"github.com/LucasPcq/wtm/internal/domain"
)

// IsSharedJob says whether a job serves every profile. A job whose cwd is the
// repository root — a compose stack, a root script — is infrastructure the
// packages sit on, so starting one package alone still needs it.
func IsSharedJob(job domain.JobConfig) bool {
	return job.Cwd == "" || job.Cwd == domain.ProfileRootCwd
}

type ProposeProfilesParams struct {
	Config domain.RunConfig
	// Existing is the split already in run.toml. Non-empty wins whole: a
	// composition the user made by hand is an answer, not a starting point to
	// infer over.
	Existing []domain.ProfileConfig
}

// ProposeProfiles suggests a split for the wizard to edit. It decides nothing
// final — the grouping is an intention, and two repos with the same directory
// shape can want opposite groupings.
func ProposeProfiles(params ProposeProfilesParams) []domain.ProfileConfig {
	if len(params.Existing) > 0 {
		return params.Existing
	}

	var shared, all []string
	byPackage := map[string][]string{}
	for _, job := range params.Config.Jobs {
		if job.Kind != domain.JobKindService {
			continue
		}
		all = append(all, job.Name)
		if IsSharedJob(job) {
			shared = append(shared, job.Name)
			continue
		}
		pkg := filepath.Base(job.Cwd)
		byPackage[pkg] = append(byPackage[pkg], job.Name)
	}

	if len(all) == 0 {
		return nil
	}

	global := domain.ProfileConfig{Name: domain.ProfileAllName, Jobs: all, Default: true}
	if len(byPackage) <= 1 {
		return []domain.ProfileConfig{global}
	}

	packages := make([]string, 0, len(byPackage))
	for pkg := range byPackage {
		packages = append(packages, pkg)
	}
	sort.Strings(packages)

	profiles := make([]domain.ProfileConfig, 0, len(packages)+1)
	for _, pkg := range packages {
		profiles = append(profiles, domain.ProfileConfig{
			Name: pkg,
			Jobs: append(append([]string{}, shared...), byPackage[pkg]...),
		})
	}
	return append(profiles, global)
}
```

- [ ] **Step 4: Lancer le test et vérifier qu'il passe**

Run: `go test ./internal/rules/ -run 'TestProposeProfiles|TestIsSharedJob' -v`
Expected: PASS

- [ ] **Step 5: Valider et commiter**

Lancer `build-validator`, puis :

```bash
git add internal/rules/profileplan.go internal/rules/profileplan_test.go internal/domain/constants.go
git commit -m "feat(run): proposer un découpage en profils depuis les jobs retenus"
```

---

### Task 3: Services sans port déclaré, nommés dans le recap

**Files:**
- Modify: `internal/rules/portprobe.go`
- Modify: `internal/domain/constants.go`
- Create: `internal/output/unportedjobs.go`
- Test: `internal/rules/portprobe_test.go`

**Interfaces:**
- Consumes: `domain.RunConfig`, `rules.ShouldProbeJob` (livré au lot 1).
- Produces:
  - `rules.ServicesWithoutPorts(cfg domain.RunConfig) []string`
  - `output.UnportedJobsReport(w io.Writer, jobs []string)`

L'étape « ports » ne demande rien quand la détection n'a rien trouvé. Ce constat rend l'angle mort visible : sans port déclaré il n'y a rien à sonder, donc le lot 1 resterait muet.

- [ ] **Step 1: Écrire le test qui échoue**

Ajouter à `internal/rules/portprobe_test.go` :

```go
func TestServicesWithoutPorts(t *testing.T) {
	cfg := domain.RunConfig{Jobs: []domain.JobConfig{
		{Name: "api-dev", Kind: domain.JobKindService, Ports: map[string]int{"API_PORT": 3001}},
		{Name: "web-dev", Kind: domain.JobKindService},
		{Name: "lint", Kind: domain.JobKindTask},
	}}

	got := rules.ServicesWithoutPorts(cfg)
	if len(got) != 1 || got[0] != "web-dev" {
		t.Errorf("ServicesWithoutPorts = %v, want [web-dev]", got)
	}
}

func TestServicesWithoutPortsIgnoresTasks(t *testing.T) {
	// Une task n'écoute pas : l'absence de port n'y est pas une lacune.
	cfg := domain.RunConfig{Jobs: []domain.JobConfig{{Name: "seed", Kind: domain.JobKindTask}}}
	if got := rules.ServicesWithoutPorts(cfg); len(got) != 0 {
		t.Errorf("ServicesWithoutPorts = %v, want empty", got)
	}
}
```

- [ ] **Step 2: Lancer le test et vérifier qu'il échoue**

Run: `go test ./internal/rules/ -run TestServicesWithoutPorts -v`
Expected: FAIL — `undefined: rules.ServicesWithoutPorts`

- [ ] **Step 3: Implémenter**

Ajouter dans `internal/domain/constants.go` :

```go
	// UnportedJobsTitle heads the services `run init` retained without a port to
	// isolate. Nothing is asked for them — inventing a port would move the guess
	// onto the user — but staying silent would hide that two worktrees will bind
	// the same one, and the probe has nothing to check where nothing is declared.
	UnportedJobsTitle  = "Services with no declared port — two worktrees will bind the same one"
	UnportedJobLineFmt = "%s"
```

Ajouter dans `internal/rules/portprobe.go` :

```go
// ServicesWithoutPorts names the services no port was declared for, in a stable
// order. They run, but nothing shifts them per worktree, and the probe has
// nothing to check where nothing is declared.
func ServicesWithoutPorts(cfg domain.RunConfig) []string {
	var jobs []string
	for _, job := range cfg.Jobs {
		if job.Kind == domain.JobKindService && len(job.Ports) == 0 {
			jobs = append(jobs, job.Name)
		}
	}
	sort.Strings(jobs)
	return jobs
}
```

Créer `internal/output/unportedjobs.go` :

```go
package output

import (
	"fmt"
	"io"

	"github.com/LucasPcq/wtm/internal/domain"
)

// UnportedJobsReport names the services the init kept without a port.
func UnportedJobsReport(w io.Writer, jobs []string) {
	if len(jobs) == 0 {
		return
	}
	lines := make([]string, 0, len(jobs))
	for _, job := range jobs {
		lines = append(lines, fmt.Sprintf(domain.UnportedJobLineFmt, job))
	}
	Blank(w)
	Callout(w, domain.UnportedJobsTitle, lines)
}
```

- [ ] **Step 4: Lancer le test et vérifier qu'il passe**

Run: `go test ./internal/rules/ ./internal/output/ -run TestServicesWithoutPorts -v`
Expected: PASS

- [ ] **Step 5: Valider et commiter**

Lancer `build-validator`, puis :

```bash
git add internal/rules/portprobe.go internal/rules/portprobe_test.go internal/output/unportedjobs.go internal/domain/constants.go
git commit -m "feat(run): nommer les services retenus sans port déclaré"
```

---

### Task 4: Étape de revue des ports détectés

**Files:**
- Create: `internal/tui/components/portlist.go`
- Modify: `internal/domain/constants.go`
- Test: `internal/tui/components/portlist_test.go`

**Interfaces:**
- Consumes: `domain.RunConfig`, `components.SetSizeParams`, `components.NewTextInput` (existant dans `textinput.go`).
- Produces:
  - `components.NewPortList(params components.NewPortListParams) components.PortListModel`
  - `components.NewPortListParams{ Title, Description string; Entries []components.PortEntry }`
  - `components.PortEntry{ Job, Name string; Base int }`
  - Méthodes : `Entries() []components.PortEntry`, `Done() bool`, `Aborted() bool`, `Init() tea.Cmd`, `Update(tea.Msg) (PortListModel, tea.Cmd)`, `View() string`, `SetSize(SetSizeParams)`

La spec impose que les ports détectés soient **confirmés ou corrigés** dans l'init, au lieu d'obliger à ressortir vers `run job edit`. Le composant ne liste que ce que la détection a trouvé : quand elle n'a rien, l'entrée n'existe pas et rien n'est demandé.

Beaucoup plus simple que `ProfileListModel` : pas de fusion, une seule opération d'édition sur un entier.

- [ ] **Step 1: Écrire le test qui échoue**

Créer `internal/tui/components/portlist_test.go` :

```go
package components

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func twoPorts() []PortEntry {
	return []PortEntry{
		{Job: "docker-compose", Name: "POSTGRES_PORT", Base: 5432},
		{Job: "web-dev", Name: "WEB_PORT", Base: 5173},
	}
}

func newPortList() PortListModel {
	return NewPortList(NewPortListParams{Title: "Ports", Entries: twoPorts()})
}

func typeRunes(m PortListModel, text string) PortListModel {
	for _, r := range text {
		m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
	return m
}

func TestPortListStartsFromTheDetection(t *testing.T) {
	m := newPortList()
	got := m.Entries()
	if len(got) != 2 {
		t.Fatalf("expected the 2 detected ports, got %d", len(got))
	}
	if got[0].Base != 5432 {
		t.Errorf("base = %d, want the detected 5432", got[0].Base)
	}
}

func TestPortListEditsABase(t *testing.T) {
	m, _ := newPortList().Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'e'}})
	m = typeRunes(m, "5555")
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})

	if got := m.Entries()[0].Base; got != 5555 {
		t.Errorf("base = %d, want 5555", got)
	}
}

func TestPortListRefusesANonPort(t *testing.T) {
	// Une saisie invalide ne doit pas écraser une valeur détectée qui, elle,
	// marche : mieux vaut refuser que déclarer un port impossible.
	m, _ := newPortList().Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'e'}})
	m = typeRunes(m, "99999")
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})

	if got := m.Entries()[0].Base; got != 5432 {
		t.Errorf("base = %d, want the detected 5432 kept", got)
	}
	if m.Done() {
		t.Error("une saisie refusée ne valide pas l'étape")
	}
}

func TestPortListConfirms(t *testing.T) {
	m := newPortList()
	if m.Done() {
		t.Fatal("a fresh list is not done")
	}
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if !m.Done() {
		t.Error("Enter confirms the list")
	}
}
```

- [ ] **Step 2: Lancer le test et vérifier qu'il échoue**

Run: `go test ./internal/tui/components/ -run TestPortList -v`
Expected: FAIL — `undefined: NewPortList`

- [ ] **Step 3: Implémenter**

Ajouter dans `internal/domain/constants.go` :

```go
	// The wizard step reviewing the ports detection pre-filled.
	PortListStepName  = "Ports"
	PortListStepTitle = "Ports detected for the jobs you kept"
	PortListStepDesc  = "Each is injected under its name, shifted by the worktree's offset. Only what\n" +
		"detection actually found is listed — a job with no detected port declares\n" +
		"none, and `wtm run up` will say so rather than pretend it is isolated."
	PortListKeyEdit  = "e"
	PortListHelp     = "e edit · enter confirm"
	PortListEntryFmt = "%s · %s = %d"
	// PortListRangeErrFmt refuses a value outside the usable range rather than
	// overwriting a detected port that works.
	PortListRangeErrFmt = "%q is not a port between %d and %d"
```

Créer `internal/tui/components/portlist.go`, sur le modèle de `HookListModel` : `entries []PortEntry`, `cursor int`, `editing bool` avec le `textinput` existant, `err error`, `done`/`aborted`. `e` ouvre l'édition de la ligne, `enter` valide la saisie (ou confirme l'étape hors édition), `esc` annule l'édition. La validation refuse tout ce qui n'est pas un entier dans `[1, 65535]` et conserve la valeur détectée.

- [ ] **Step 4: Lancer le test et vérifier qu'il passe**

Run: `go test ./internal/tui/components/ -run TestPortList -v`
Expected: PASS

- [ ] **Step 5: Valider et commiter**

Lancer `build-validator`, puis :

```bash
git add internal/tui/components/portlist.go internal/tui/components/portlist_test.go internal/domain/constants.go
git commit -m "feat(run): composant de revue des ports détectés"
```

---

### Task 5: Composant d'édition de profils

**Files:**
- Create: `internal/tui/components/profilelist.go`
- Modify: `internal/domain/constants.go`
- Test: `internal/tui/components/profilelist_test.go`

**Interfaces:**
- Consumes: `domain.ProfileConfig`, `components.SetSizeParams` (existant).
- Produces:
  - `components.NewProfileList(params components.NewProfileListParams) components.ProfileListModel`
  - `components.NewProfileListParams{ Title, Description string; Profiles []domain.ProfileConfig }`
  - Méthodes : `Profiles() []domain.ProfileConfig`, `Done() bool`, `Aborted() bool`, `Init() tea.Cmd`, `Update(tea.Msg) (ProfileListModel, tea.Cmd)`, `View() string`, `SetSize(SetSizeParams)`

Modelé sur `HookListModel` (`internal/tui/components/hooklist.go`) : mêmes conventions de lignes, de focus et d'aide. Les tests portent sur les **transitions**, jamais sur le rendu.

- [ ] **Step 1: Écrire le test qui échoue**

Créer `internal/tui/components/profilelist_test.go` :

```go
package components

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/LucasPcq/wtm/internal/domain"
)

func threeProfiles() []domain.ProfileConfig {
	return []domain.ProfileConfig{
		{Name: "api", Jobs: []string{"docker-compose", "api-dev"}},
		{Name: "web", Jobs: []string{"docker-compose", "web-dev"}},
		{Name: "all", Jobs: []string{"docker-compose", "api-dev", "web-dev"}, Default: true},
	}
}

func key(m ProfileListModel, r rune) ProfileListModel {
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	return m
}

func newList() ProfileListModel {
	return NewProfileList(NewProfileListParams{Title: "Profils", Profiles: threeProfiles()})
}

func TestProfileListStartsFromTheProposal(t *testing.T) {
	m := newList()
	if got := m.Profiles(); len(got) != 3 {
		t.Fatalf("expected the 3 proposed profiles, got %d", len(got))
	}
}

func TestProfileListRemoves(t *testing.T) {
	m := key(newList(), 'd')

	got := m.Profiles()
	if len(got) != 2 {
		t.Fatalf("expected 2 profiles after removal, got %d", len(got))
	}
	if got[0].Name != "web" {
		t.Errorf("removed the wrong row: %s remains first", got[0].Name)
	}
}

func TestProfileListMergesTwoRows(t *testing.T) {
	// La fusion est l'opération qui porte le cas monorepo réel : six profils
	// proposés, deux fusions, et on obtient app1 et app2.
	m := key(newList(), 'f')                              // marque "api"
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})        // curseur sur "web"
	m = key(m, 'f')                                       // fusionne dans "api"

	got := m.Profiles()
	if len(got) != 2 {
		t.Fatalf("expected 2 profiles after the merge, got %d", len(got))
	}

	merged := got[0]
	if merged.Name != "api" {
		t.Errorf("la fusion garde le nom de la première ligne marquée, got %s", merged.Name)
	}
	if len(merged.Jobs) != 3 {
		t.Errorf("merged jobs = %v, want the union without duplicates", merged.Jobs)
	}
	for _, want := range []string{"docker-compose", "api-dev", "web-dev"} {
		if !hasJob(merged.Jobs, want) {
			t.Errorf("merged jobs are missing %s: %v", want, merged.Jobs)
		}
	}
}

func TestProfileListMergeKeepsOneCopyOfASharedJob(t *testing.T) {
	m := key(newList(), 'f')
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m = key(m, 'f')

	count := 0
	for _, job := range m.Profiles()[0].Jobs {
		if job == "docker-compose" {
			count++
		}
	}
	if count != 1 {
		t.Errorf("docker-compose apparaît %d fois, want 1", count)
	}
}

func TestProfileListConfirms(t *testing.T) {
	m := newList()
	if m.Done() {
		t.Fatal("a fresh list is not done")
	}
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if !m.Done() {
		t.Error("Enter confirms the list")
	}
}

func hasJob(jobs []string, name string) bool {
	for _, job := range jobs {
		if job == name {
			return true
		}
	}
	return false
}
```

- [ ] **Step 2: Lancer le test et vérifier qu'il échoue**

Run: `go test ./internal/tui/components/ -run TestProfileList -v`
Expected: FAIL — `undefined: NewProfileList`

- [ ] **Step 3: Implémenter**

Ajouter dans `internal/domain/constants.go` :

```go
	// The profile editor's keys and help line.
	ProfileListKeyRename = "r"
	ProfileListKeyMerge  = "f"
	ProfileListKeyRemove = "d"
	ProfileListKeyNew    = "n"
	ProfileListHelp      = "r rename · f merge · d remove · n new · enter confirm"
	// ProfileListMergeHint replaces the help line once a row is marked, so the
	// second half of a merge is not a guess.
	ProfileListMergeHint = "f again on another profile to merge it into %q · esc to cancel"
```

Créer `internal/tui/components/profilelist.go` en suivant la structure de `hooklist.go` : un `ProfileListModel` portant `profiles []domain.ProfileConfig`, `cursor int`, `mergeFrom int` (`-1` quand rien n'est marqué), `renaming bool` avec un `textinput` réutilisé de `textinput.go`, `done`/`aborted`.

Règles à respecter :
- `d` retire la ligne sous le curseur ; le curseur reste borné.
- `f` marque la ligne quand `mergeFrom == -1`, sinon fusionne la ligne courante **dans** la ligne marquée : la cible garde son nom, ses jobs sont l'union sans doublon (ordre : ceux de la cible puis les nouveaux), la ligne fusionnée disparaît, `mergeFrom` repasse à `-1`.
- `f` sur la ligne déjà marquée annule le marquage.
- `r` ouvre la saisie du nom ; `n` crée un profil vide nommé par saisie.
- `Default` : si la ligne portant `Default` disparaît, le drapeau passe à la première ligne restante — un picker sans pré-sélection n'a pas de sens.
- `enter` confirme, `esc` annule un marquage ou une saisie en cours, sinon abandonne.

- [ ] **Step 4: Lancer le test et vérifier qu'il passe**

Run: `go test ./internal/tui/components/ -run TestProfileList -v`
Expected: PASS

- [ ] **Step 5: Valider et commiter**

Lancer `build-validator`, puis :

```bash
git add internal/tui/components/profilelist.go internal/tui/components/profilelist_test.go internal/domain/constants.go
git commit -m "feat(run): composant d'édition de profils avec fusion"
```

---

### Task 6: Le wizard n'écrit que ce qui est coché

**Files:**
- Modify: `internal/tui/inittui/project.go`
- Modify: `internal/rules/init_answers.go`
- Test: `internal/tui/inittui/project_test.go`

**Interfaces:**
- Consumes: `rules.PreselectScript` (Task 1).
- Produces: aucune nouvelle signature exportée — le comportement de `addServicesSteps` change.

- [ ] **Step 1: Écrire le test qui échoue**

Ajouter à `internal/tui/inittui/project_test.go` :

```go
func TestServicesStepPreselectsOnlyDevScripts(t *testing.T) {
	scripts := []domain.PackageScript{
		{Name: "dev", Workspace: "apps/web", PkgName: "web"},
		{Name: "build", Workspace: "apps/web", PkgName: "web"},
		{Name: "preview", Workspace: "apps/web", PkgName: "web"},
		{Name: "start", Workspace: "apps/api", PkgName: "api"},
	}

	items := scriptItems(scriptItemsParams{
		Scripts:        scripts,
		PackageManager: domain.PkgManagerPnpm,
	})

	if len(items) != 4 {
		t.Fatalf("tous les scripts restent proposés, got %d", len(items))
	}
	selected := map[string]bool{}
	for i, item := range items {
		selected[scripts[i].Name] = item.Selected
	}
	if !selected["dev"] {
		t.Error("dev doit être coché")
	}
	for _, name := range []string{"build", "preview", "start"} {
		if selected[name] {
			t.Errorf("%s ne doit pas être coché", name)
		}
	}
}
```

- [ ] **Step 2: Lancer le test et vérifier qu'il échoue**

Run: `go test ./internal/tui/inittui/ -run TestServicesStepPreselects -v`
Expected: FAIL — `undefined: scriptItems`

- [ ] **Step 3: Implémenter**

Dans `internal/tui/inittui/project.go`, extraire la construction des items de scripts (aujourd'hui inline dans `addServicesSteps`) vers une fonction testable :

```go
type scriptItemsParams struct {
	Scripts        []domain.PackageScript
	PackageManager domain.PackageManager
	Prefill        *SectionPrefill
}

// scriptItems proposes every script and checks the fewest: a job nobody checked
// is a job that is never written, which is what stops the init from producing an
// inventory instead of a configuration.
func scriptItems(params scriptItemsParams) []components.MultiSelectItem {
	pm := string(params.PackageManager)
	items := make([]components.MultiSelectItem, 0, len(params.Scripts))
	for i, script := range params.Scripts {
		scope := "root"
		if script.Workspace != "" {
			scope = script.Workspace
		}
		label := fmt.Sprintf("%s / %s — %s run %s", scope, script.Name, pm, script.Name)
		selected := prefillSelected(params.Prefill,
			params.Prefill != nil && params.Prefill.ScriptIndices[i],
			rules.PreselectScript(script.Name))
		items = append(items, components.MultiSelectItem{
			Label:    label,
			Value:    strconv.Itoa(i),
			Selected: selected,
		})
	}
	return items
}
```

Remplacer le corps inline de `addServicesSteps` par un appel à `scriptItems`.

Dans `internal/rules/init_answers.go`, `AutoServicesAnswers` doit filtrer de la même façon pour le mode non interactif :

```go
	for _, script := range detection.PackageScripts {
		if PreselectScript(script.Name) {
			answers.SelectedPackageScripts = append(answers.SelectedPackageScripts, script)
		}
	}
```

en remplacement de `SelectedPackageScripts: detection.PackageScripts`.

- [ ] **Step 4: Lancer le test et vérifier qu'il passe**

Run: `go test ./internal/tui/inittui/ ./internal/rules/ -v -run 'TestServicesStepPreselects|TestAutoServices'`
Expected: PASS

- [ ] **Step 5: Valider et commiter**

Lancer `build-validator`, puis :

```bash
git add internal/tui/inittui/project.go internal/tui/inittui/project_test.go internal/rules/init_answers.go
git commit -m "feat(run): n'écrire en jobs que les scripts cochés"
```

---

### Task 7: Fixer le `kind` d'un script coché hors des `dev`

**Files:**
- Modify: `internal/domain/init.go`
- Modify: `internal/tui/inittui/project.go`
- Modify: `internal/rules/jobs_builder.go`
- Test: `internal/rules/jobs_builder_test.go`

**Interfaces:**
- Consumes: `domain.PackageScript` (champ `Kind` déjà présent).
- Produces: `BuildScriptJobs` honore `script.Kind` quand il est renseigné, au lieu de toujours rappeler `ClassifyScriptKind`.

`kind` décide si le job **bloque le profil**. `preview` est classé `task` par le nom : coché à la main, il ferait attendre `run up` indéfiniment sur un serveur.

- [ ] **Step 1: Écrire le test qui échoue**

Ajouter à `internal/rules/jobs_builder_test.go` :

```go
func TestBuildScriptJobsHonoursAnExplicitKind(t *testing.T) {
	cfg := rules.BuildScriptJobs(rules.BuildScriptJobsParams{
		PackageManager: domain.PkgManagerPnpm,
		Scripts: []domain.PackageScript{
			// `preview` sert des requêtes : classé task par son nom, il bloquerait
			// le profil pour toujours.
			{Name: "preview", Workspace: "apps/web", PkgName: "web", Kind: domain.JobKindService},
		},
	})

	if len(cfg.Jobs) != 1 {
		t.Fatalf("expected 1 job, got %d", len(cfg.Jobs))
	}
	if cfg.Jobs[0].Kind != domain.JobKindService {
		t.Errorf("kind = %s, want service", cfg.Jobs[0].Kind)
	}
}

func TestBuildScriptJobsFallsBackToTheNameWhenKindIsUnset(t *testing.T) {
	cfg := rules.BuildScriptJobs(rules.BuildScriptJobsParams{
		PackageManager: domain.PkgManagerPnpm,
		Scripts:        []domain.PackageScript{{Name: "dev", Workspace: "", PkgName: "root"}},
	})

	if cfg.Jobs[0].Kind != domain.JobKindService {
		t.Errorf("kind = %s, want service from the name", cfg.Jobs[0].Kind)
	}
}
```

- [ ] **Step 2: Lancer le test et vérifier qu'il échoue**

Run: `go test ./internal/rules/ -run TestBuildScriptJobsHonours -v`
Expected: FAIL — le kind vaut `task`, `ClassifyScriptKind` écrasant la valeur explicite

- [ ] **Step 3: Implémenter**

Dans `internal/rules/jobs_builder.go`, remplacer `Kind: ClassifyScriptKind(s.Name)` par un appel à une petite fonction :

```go
// scriptKind prefers the kind the caller settled. The name is only a guess, and
// it is wrong in both directions: `preview` serves requests while `start` is
// production. A kind chosen in the wizard outranks it.
func scriptKind(s domain.PackageScript) domain.JobKind {
	if s.Kind != "" {
		return s.Kind
	}
	return ClassifyScriptKind(s.Name)
}
```

Dans `internal/tui/inittui/project.go`, ajouter une étape `stepScriptKinds` juste après `stepScripts`, ne s'affichant (`Decide`) que si au moins un script coché n'est pas pré-sélectionné par `rules.PreselectScript`. L'étape est un `components.MultiSelect` dont les items sont ces scripts, cochés = `service`, décochés = `task`, avec pour titre `domain.ScriptKindStepTitle` :

```go
	// The wizard step settling the kind of a script checked outside the dev ones.
	ScriptKindStepName  = "Long-running?"
	ScriptKindStepTitle = "Which of these keep running?"
	ScriptKindStepDesc  = "A checked script runs as a service: started in the background, left up.\n" +
		"An unchecked one is a task: it blocks the profile until it exits, and a\n" +
		"non-zero exit aborts the run."
```

Reporter la réponse dans `answers.SelectedPackageScripts[i].Kind` lors de l'extraction.

- [ ] **Step 4: Lancer le test et vérifier qu'il passe**

Run: `go test ./internal/rules/ ./internal/tui/inittui/ -run 'TestBuildScriptJobs' -v`
Expected: PASS

- [ ] **Step 5: Valider et commiter**

Lancer `build-validator`, puis :

```bash
git add internal/rules/jobs_builder.go internal/rules/jobs_builder_test.go internal/tui/inittui/project.go internal/domain/constants.go internal/domain/init.go
git commit -m "feat(run): permettre de fixer le kind d'un script coché hors des dev"
```

---

### Task 8: L'étape profils dans le wizard

**Files:**
- Modify: `internal/tui/inittui/project.go`
- Modify: `internal/domain/init.go`
- Modify: `internal/commands/run/init.go`
- Test: `internal/tui/inittui/project_test.go`

**Interfaces:**
- Consumes: `rules.ProposeProfiles` (Task 2), `components.NewProfileList` (Task 4), `rules.BuildInitRunConfig`.
- Produces: `domain.InitProjectAnswers.Profiles []domain.ProfileConfig`.

- [ ] **Step 1: Écrire le test qui échoue**

Ajouter à `internal/tui/inittui/project_test.go` :

```go
func TestProfilesStepProposesFromTheRetainedJobs(t *testing.T) {
	cfg := domain.RunConfig{Jobs: []domain.JobConfig{
		{Name: "docker-compose", Kind: domain.JobKindService, Cwd: "."},
		{Name: "api-dev", Kind: domain.JobKindService, Cwd: "apps/api"},
		{Name: "web-dev", Kind: domain.JobKindService, Cwd: "apps/web"},
	}}

	proposed := rules.ProposeProfiles(rules.ProposeProfilesParams{Config: cfg})
	model := components.NewProfileList(components.NewProfileListParams{Profiles: proposed})

	if len(model.Profiles()) != 3 {
		t.Fatalf("expected api, web and all, got %d", len(model.Profiles()))
	}
}
```

- [ ] **Step 2: Lancer le test et vérifier qu'il échoue**

Run: `go test ./internal/tui/inittui/ -run TestProfilesStepProposes -v`
Expected: FAIL tant que Task 2 et Task 4 ne sont pas fusionnées dans la branche

- [ ] **Step 3: Implémenter**

Ajouter `Profiles []domain.ProfileConfig` à `domain.InitProjectAnswers`, avec le commentaire :

```go
	// Profiles is the split the wizard settled. Empty means the run was not
	// interactive: the proposal is then taken as answered, since a profile is
	// what makes `run up` start something rather than everything.
	Profiles []domain.ProfileConfig
```

Ajouter aussi une clé `stepPorts = "ports"` et son étape, déclarée **avant** `stepProfiles` : son `Build` construit les `components.PortEntry` depuis `rules.PlanComposePorts` et `rules.BackfillScriptPorts` pour les seuls jobs retenus, et n'émet aucune entrée pour un job sans port détecté. La réponse est reportée dans `answers` sous forme des bases à déclarer.

Ajouter dans `internal/tui/inittui/project.go` une clé `stepProfiles = "profiles"` et une étape déclarée **après** `stepComposePatch`, avec `Build` (et non `Decide`) pour lire les sélections vivantes :

```go
	s.add(stepProfiles, components.Step{
		Name: domain.ProfileStepName,
		Build: func(prev []components.Step) any {
			cfg := rules.BuildInitRunConfig(answersFromSteps(answersFromStepsParams{
				Prev:      prev,
				Docker:    docker,
				Scripts:   scripts,
				Detection: detection,
			}), detection.PackageManager)
			return components.NewProfileList(components.NewProfileListParams{
				Title:       domain.ProfileStepTitle,
				Description: domain.ProfileStepDesc,
				Profiles: rules.ProposeProfiles(rules.ProposeProfilesParams{
					Config:   cfg,
					Existing: params.Existing.Profiles,
				}),
			})
		},
		Summary: profilesSummary,
		Callout: true,
	})
```

Constantes à ajouter dans `internal/domain/constants.go` :

```go
	// The wizard step composing the profiles `run up` will offer.
	ProfileStepName  = "Profiles"
	ProfileStepTitle = "What should `wtm run up` start?"
	ProfileStepDesc  = "A profile is a set of jobs started together. Jobs at the repository root —\n" +
		"a compose stack — join every profile, so starting one package alone still\n" +
		"brings its infrastructure up."
```

Dans `internal/commands/run/init.go`, écrire les profils retenus dans la config produite, après `outcome.Config` et avant `runconfig.Save`, et rendre le compte-rendu via `output.UnportedJobsReport(cmd.OutOrStdout(), rules.ServicesWithoutPorts(outcome.Config))` inséré juste avant `output.EnvPortLinksReport`.

`answersFromSteps` n'existe pas encore : l'extraire de `composePatchesFor` (`internal/tui/inittui/project.go`), qui construit déjà ces mêmes réponses en dur, et faire appeler la nouvelle fonction par les deux sites — un seul endroit décide ce que valent les réponses vivantes du wizard :

```go
type answersFromStepsParams struct {
	Prev      []components.Step
	Docker    int
	Scripts   int
	Detection domain.InitDetectionResult
}

// answersFromSteps reads the wizard's live selections as an answers struct, so a
// step that must plan against them sees exactly what the recap will.
func answersFromSteps(params answersFromStepsParams) domain.InitProjectAnswers {
	answers := domain.InitProjectAnswers{
		DockerComposeCmd:       params.Detection.DockerComposeCmd,
		PatchCompose:           true,
		SelectedPackageScripts: selectedScripts(params.Prev, params.Scripts, params.Detection.PackageScripts),
	}
	if params.Docker >= 0 && params.Docker < len(params.Prev) {
		if selected, ok := params.Prev[params.Docker].Model.(components.MultiSelectModel); ok {
			answers.DockerComposeFiles = selected.Values()
		}
	}
	return answers
}
```

Pour le mode non interactif, `rules.AutoServicesAnswers` ne compose rien : `runRunInit` applique `rules.ProposeProfiles` sur la config produite quand `answers.Profiles` est vide.

- [ ] **Step 4: Lancer le test et vérifier qu'il passe**

Run: `go test ./internal/tui/inittui/ ./internal/commands/run/ -v`
Expected: PASS

- [ ] **Step 5: Valider et commiter**

Lancer `build-validator`, puis :

```bash
git add internal/tui/inittui/project.go internal/tui/inittui/project_test.go internal/domain/init.go internal/domain/constants.go internal/commands/run/init.go
git commit -m "feat(run): composer les profils dans le wizard d'init"
```

---

### Task 9: `run up` sans profil ne lance plus les tasks

**Files:**
- Modify: `internal/commands/run/up.go:106-129`
- Modify: `internal/domain/constants.go`
- Test: `internal/commands/run/up_test.go`

**Interfaces:**
- Consumes: `rules.DefaultProfile`, `rules.ProfileJobs` (existants).
- Produces: `rules.JobsWithoutProfile(cfg domain.RunConfig) []domain.JobConfig`.

L'init écrivant désormais toujours au moins un profil, ce chemin ne concerne plus que les `run.toml` écrits à la main. Il reste faux : c'est lui qui lance le linter.

- [ ] **Step 1: Écrire le test qui échoue**

Ajouter à `internal/rules/jobs_test.go` :

```go
func TestJobsWithoutProfileKeepsOnlyServices(t *testing.T) {
	cfg := domain.RunConfig{Jobs: []domain.JobConfig{
		{Name: "docker-compose", Kind: domain.JobKindService},
		{Name: "dev", Kind: domain.JobKindService},
		{Name: "lint", Kind: domain.JobKindTask},
		{Name: "build", Kind: domain.JobKindTask},
	}}

	got := rules.JobsWithoutProfile(cfg)
	if len(got) != 2 {
		t.Fatalf("expected the 2 services, got %d: %+v", len(got), got)
	}
	for _, job := range got {
		if job.Kind != domain.JobKindService {
			t.Errorf("%s is a task and must not run without a profile", job.Name)
		}
	}
}
```

- [ ] **Step 2: Lancer le test et vérifier qu'il échoue**

Run: `go test ./internal/rules/ -run TestJobsWithoutProfile -v`
Expected: FAIL — `undefined: rules.JobsWithoutProfile`

- [ ] **Step 3: Implémenter**

Ajouter dans `internal/rules/jobs.go` :

```go
// JobsWithoutProfile is what `run up` starts when the config declares no
// profile at all. Only the services: a run.toml written by hand has no reason
// to want its linter and its build started by `run up`, and a task would block
// the sequence on its way.
func JobsWithoutProfile(cfg domain.RunConfig) []domain.JobConfig {
	jobs := make([]domain.JobConfig, 0, len(cfg.Jobs))
	for _, job := range cfg.Jobs {
		if job.Kind == domain.JobKindService {
			jobs = append(jobs, job)
		}
	}
	return jobs
}
```

Dans `internal/commands/run/up.go`, remplacer `return cfg.Jobs, nil` par `return rules.JobsWithoutProfile(cfg), nil` dans `resolveProfileJobs`.

- [ ] **Step 4: Lancer le test et vérifier qu'il passe**

Run: `go test ./internal/rules/ ./internal/commands/run/ -v`
Expected: PASS

- [ ] **Step 5: Valider et commiter**

Lancer `build-validator`, puis :

```bash
git add internal/rules/jobs.go internal/rules/jobs_test.go internal/commands/run/up.go
git commit -m "fix(run): un run up sans profil ne lance plus les tasks"
```

---

### Task 10: Vérification bout en bout et documentation

**Files:**
- Modify: `README.md`
- Modify: `internal/commands/agents/assets/using-wtm.skill.md`
- Modify: `docs/` (généré)
- Test: `internal/commands/run/init_test.go`

**Interfaces:**
- Consumes: tout ce qui précède.
- Produces: aucune nouvelle signature.

- [ ] **Step 1: Écrire le test de bout en bout qui échoue**

Ajouter à `internal/commands/run/init_test.go`, en réutilisant le harnais déjà présent dans ce fichier — `setupTestProject`, `runCmd`, `config.LoadRun` :

```go
func TestRunInit_ProducesAStartableConfig(t *testing.T) {
	stateDir := setupTestProject(t)
	projectDir := os.Getenv("WTM_PROJECT_DIR")

	writeCompose(t, "docker-compose.yml", "services:\n  db:\n    image: alpine\n    ports:\n      - \"5432:5432\"\n")
	pkg := `{"name":"demo","scripts":{"dev":"vite","build":"tsc","lint":"eslint .","start":"node dist/i.js"}}`
	if err := os.WriteFile(filepath.Join(projectDir, "package.json"), []byte(pkg), 0o644); err != nil {
		t.Fatalf("write package.json: %v", err)
	}

	if _, _, err := runCmd(t, domain.CmdInit, "--"+domain.FlagNonInteractive); err != nil {
		t.Fatalf("run init: %v", err)
	}

	cfg, err := config.LoadRun(stateDir)
	if err != nil {
		t.Fatalf("load run: %v", err)
	}

	if len(cfg.Jobs) != 2 {
		t.Fatalf("expected compose + dev only, got %+v", cfg.Jobs)
	}
	for _, job := range cfg.Jobs {
		switch job.Name {
		case "build", "lint", "start":
			t.Errorf("%s ne doit pas devenir un job", job.Name)
		}
	}
	if len(cfg.Profiles) != 1 {
		t.Fatalf("expected exactly one profile, got %+v", cfg.Profiles)
	}
	if !cfg.Profiles[0].Default {
		t.Error("le profil unique doit porter Default")
	}
}
```

- [ ] **Step 2: Lancer le test et vérifier qu'il échoue**

Run: `go test ./internal/commands/run/ -run TestRunInitProducesAStartableConfig -v`
Expected: FAIL tant que les tâches 1 à 9 ne sont pas toutes fusionnées

- [ ] **Step 3: Vérifier à la main sur les deux repos de référence**

```bash
go build -o /tmp/wtm .
# repo neuf : deux jobs, un profil, run up ne lance que ces deux-là
# monorepo d'exemple : aucun job racine `dev` non coché
```

Contrôler les cinq critères d'acceptation de la spec, section « Critères d'acceptation ».

- [ ] **Step 4: Mettre la documentation à jour**

- `README.md` : la section run décrit l'init qui compose un profil, et non plus un inventaire.
- `internal/commands/agents/assets/using-wtm.skill.md` : `run init` crée des profils ; seuls les scripts `dev` sont pré-cochés ; un service sans port détecté est signalé et non isolé.
- `make docs` pour régénérer `docs/`.

- [ ] **Step 5: Valider et commiter**

Lancer `build-validator`, puis :

```bash
make docs
git add -A
git commit -m "feat(run): init compose une configuration démarrable de bout en bout"
```
