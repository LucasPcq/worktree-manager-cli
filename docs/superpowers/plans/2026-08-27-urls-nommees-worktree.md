# URLs nommées par worktree — plan d'implémentation

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** donner à chaque job HTTP d'un worktree une URL stable et découvrable, d'abord son adresse directe (F1), puis un nom d'hôte distinct servi par un reverse proxy dans le daemon (F2), pour que deux worktrees ne partagent plus leur jar de cookies.

**Architecture:** la déclaration vit dans `run.toml` (`[[job]].url`), le nommage est une fonction pure de `rules/`, le proxy est un `service/proxy/` alimenté par le daemon existant — qui tient déjà la table `(job, worktree) → port résolu`. L'URL voyage jusqu'aux surfaces dans `runlogs.Event`, à côté des ports qu'elle accompagne. Aucun nouvel état persistant, aucun nouveau cycle de vie.

**Tech Stack:** Go, Cobra, Bubbletea/Lipgloss, BurntSushi/toml, `net/http/httputil.ReverseProxy` (stdlib, gère nativement l'upgrade WebSocket).

**Spec:** `docs/superpowers/specs/2026-08-27-luc-104-urls-nommees-design.md`

## Global Constraints

- **Architecture en couches (CLAUDE.md §9), non négociable.** `rules/` n'importe que la stdlib et `internal/domain`. `service/` n'importe jamais `cobra`, `bubbletea`, `lipgloss`. `output/` et `tui/` ne décident rien. `flow/` n'importe que `service/`, `rules/`, `domain/` et la stdlib.
- **Structs pour 2+ paramètres**, toujours initialisées en champs nommés (CLAUDE.md §2).
- **Zéro magic string** : toute chaîne de format, tout nom de flag, tout nom de commande est une constante de `internal/domain/constants.go` (CLAUDE.md §5).
- **Retours anticipés**, jamais d'imbrication de `if` (CLAUDE.md §6).
- **Commentaires quasi nuls** (CLAUDE.md §8) : le pourquoi, jamais le quoi. Cible ~3,7 % comme `internal/flow`. En touchant un fichier, supprimer les commentaires qui paraphrasent le code.
- **Port du proxy par défaut : `4000`.** TLD : `localhost`. Longueur max d'un label DNS : `63`.
- **Le proxy ne doit jamais empêcher un job de tourner.** Un bind impossible se signale et se poursuit.
- **Le serveur se binde sur `127.0.0.1`, jamais `0.0.0.0`.**
- **L'en-tête `Host` est passé tel quel au job** — le réécrire remettrait tous les worktrees dans le même jar de cookies et annulerait la feature.
- **Docs au fil** (CLAUDE.md) : tout changement de surface CLI met à jour `internal/commands/agents/assets/using-wtm.skill.md`, `make docs` et la table du README **dans le même lot**.
- **Validation** : `build-validator` avant chaque commit (`go build`, `go vet`, `staticcheck`, `go test ./... -race -count=1`).
- Tests en tables, stdlib `testing`, pas de framework d'assertion — voir `internal/rules/jobports_test.go` pour le style de la maison.

---

## Terrain d'essai

Un dépôt réel est disponible pour tout ce qui ne se teste pas en `go test` : **`~/Documents/Dev/monorepo-exemple-wtm`**, déjà initialisé pour wtm, avec ses worktrees dans `~/Documents/Dev/monorepo-exemple-wtm.worktrees` (`main` et `test` existent déjà — la paire de worktrees du test d'acceptation est donc là sans rien créer).

C'est exactement le cas qui a dicté le nommage : un monorepo pnpm + turbo, deux apps, des profils par app.

```
apps/web            Vite 8 + React 19   → job web-dev, WEB_DEV_PORT = 5173
apps/api            Hono + tsx watch    → job api-dev, PORT = 3001
docker-compose.yml  postgres + redis    → job docker-compose, POSTGRES_PORT = 5432, REDIS_PORT = 6379
profils             api · web · all (défaut)
```

Ce qu'il permet de vérifier à la main, lot par lot :

| Lot | Vérification |
| --- | --- |
| 1 | Ajouter `url = { port = "WEB_DEV_PORT" }` à `web-dev` et `url = { port = "PORT", host = "api.app-1" }` à `api-dev`, puis `wtm run list` : le fichier est relu sans erreur. Mettre le même `host` sur les deux : le chargement doit refuser en nommant les deux jobs. |
| 2-3 | `wtm run up web` puis `wtm run url` et `wtm run open` — d'abord depuis `main`, puis depuis le worktree `test`, dont l'ordinal décale les ports. |
| 5-6 | `wtm run up all` dans les deux worktrees, et les quatre URLs ouvertes. **Vite 8 autorise `.localhost` par défaut** : si `web-dev` répond sous son nom sans toucher à `vite.config.ts`, l'hypothèse centrale de la spec est vérifiée sur le terrain. L'API Hono sert de seconde route, en HTTP nu. |
| 6 | `postgres` et `redis` restent joignables **par leur port seulement** — c'est la limite « HTTP seulement » rendue concrète, pas une régression. |
| 7 | **Non couvert : le dépôt n'a pas de Next.js.** L'avertissement `allowedDevOrigins` se valide par ses tests unitaires ; pour un essai manuel, il faut un `next.config.ts` jetable dans un `apps/` de brouillon. À dire explicitement plutôt qu'à cocher sans avoir regardé. |

Deux choses à savoir avant de s'y appuyer :

Son `run.toml` porte `port_offset_block = 0`. Ce n'est pas une panne : `rules.EffectivePortOffsetBlock` traite zéro comme « non renseigné » et applique le bloc par défaut. C'est le défaut d'écriture de [LUC-205](https://linear.app/lucaspcq/issue/LUC-205), hors périmètre ici — ne pas le corriger en passant.

Son `docker-compose.yml` nomme déjà ses conteneurs via `COMPOSE_PROJECT_NAME`, donc les deux worktrees peuvent monter leur stack en même temps. C'est ce qui rend le test d'acceptation à deux worktrees réellement exécutable.

---

## Structure des fichiers

**Créés**

| Fichier | Responsabilité |
| --- | --- |
| `internal/rules/hostlabel.go` | `HostLabel` (slugification d'un segment dérivé) et `IsHostLabels` (validation d'un segment saisi). Pur. |
| `internal/rules/joburl.go` | `JobURL` : de `(JobConfig, ports résolus)` vers la chaîne à afficher. Pur. |
| `internal/rules/proxyroute.go` | `RouteHost` et `ProxyPort`. Pur. La détection de collision d'hôtes que la spec range ici atterrit plus tôt, dans `validateJobURLs` (Lot 1) : elle se décide sur `run.toml` seul, donc au chargement, et pas au démarrage. |
| `internal/service/proxy/registry.go` | `Registry` : la table `host → cible`, sous verrou. |
| `internal/service/proxy/server.go` | `Server` : écoute, aiguillage sur `Host`, `ReverseProxy`, pages d'échec. |
| `internal/commands/run/url.go` | `wtm run url [job]`. |
| `internal/commands/run/open.go` | `wtm run open [job]`. |

**Modifiés**

| Fichier | Changement |
| --- | --- |
| `internal/domain/jobs.go` | `JobURLConfig`, champ `JobConfig.URL`. |
| `internal/domain/proxy.go` *(créé)* | `ProxyRoute`, `ProxyConfig`. |
| `internal/domain/config.go` | `GlobalConfig.Proxy`. |
| `internal/domain/constants.go` | constantes de nommage, de format d'URL, de commandes, de flags. |
| `internal/rules/validate.go` | validation des `url` dans `ValidateRunPorts`. |
| `internal/rules/jobenv.go` | injection de `WTM_PROJECT`. |
| `internal/schemas/run.schema.json` | bloc `url` sur `job`. |
| `internal/flow/runlogs/runlogs.go` | `Event.URL`. |
| `internal/flow/runlogs/run.go` | calcul de l'URL au moment d'émettre. |
| `internal/flow/runlogs/daemon.go` | `RouteHost` passé au démarrage. |
| `internal/service/process/protocol.go` | `Request.RouteHost`. |
| `internal/service/process/manager.go` | `RouteSink`, enregistrement/retrait des routes. |
| `internal/service/process/daemon.go` | possession du proxy. |
| `internal/output/runlogs.go` | colonne URL sur le flux. |
| `internal/output/hyperlink.go` *(créé)* | `Hyperlink` (OSC-8) avec dégradation. |
| `internal/tui/runview/render.go` | colonne URL + touche `o`. |

---

# Phase F1 — Découvrabilité

Livre une valeur seule : sans proxy, l'URL d'un job vaut `http://localhost:<port résolu>`.

## Lot 1 : la déclaration `url` dans run.toml

**Files:**
- Modify: `internal/domain/jobs.go`
- Modify: `internal/domain/constants.go`
- Create: `internal/rules/hostlabel.go`
- Modify: `internal/rules/validate.go:208-239` (`ValidateRunPorts`)
- Modify: `internal/schemas/run.schema.json`
- Test: `internal/rules/hostlabel_test.go`, `internal/rules/urlvalidate_test.go`, `internal/config/jobs_test.go`

**Interfaces:**
- Consumes: `domain.JobConfig`, `domain.RunConfig`, `rules.ValidateRunPorts` (existants).
- Produces:
  - `domain.JobURLConfig{Port string; Host string}`
  - `domain.JobConfig.URL *domain.JobURLConfig`
  - `rules.HostLabel(s string) string`
  - `rules.IsHostLabels(s string) bool`

- [ ] **Step 1: Écrire les tests de `HostLabel` et `IsHostLabels` (échec attendu)**

Créer `internal/rules/hostlabel_test.go` :

```go
package rules

import (
	"strings"
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
)

func TestHostLabel(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"déjà un label", "web", "web"},
		{"majuscules abaissées", "Web-API", "web-api"},
		{"scope npm et slash", "@acme/web", "acme-web"},
		{"underscore interdit en hôte", "app_web", "app-web"},
		{"séquences réduites", "app///web", "app-web"},
		{"tirets de bord coupés", "-web-", "web"},
		{"tronqué à 63", "a" + strings.Repeat("b", 79), "a" + strings.Repeat("b", 62)},
		{"vide se replie", "///", domain.HostLabelFallback},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := HostLabel(tt.in); got != tt.want {
				t.Errorf("HostLabel(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestIsHostLabels(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want bool
	}{
		{"un label", "web", true},
		{"deux labels", "web.app-1", true},
		{"vide", "", false},
		{"majuscule refusée", "Web", false},
		{"underscore refusé", "app_web", false},
		{"label vide", "web..app", false},
		{"tiret de tête", "-web", false},
		{"tiret de queue", "web-", false},
		{"label trop long", strings.Repeat("a", 64), false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsHostLabels(tt.in); got != tt.want {
				t.Errorf("IsHostLabels(%q) = %v, want %v", tt.in, got, tt.want)
			}
		})
	}
}
```

- [ ] **Step 2: Lancer le test, vérifier qu'il échoue**

Run: `go test ./internal/rules/ -run 'TestHostLabel|TestIsHostLabels' -v`
Expected: FAIL — `undefined: HostLabel`, `undefined: IsHostLabels`.

- [ ] **Step 3: Ajouter les constantes de nommage**

Dans `internal/domain/constants.go`, dans le bloc des constantes `run` :

```go
	// HostLabelMaxLen is the DNS limit on a single label of a hostname.
	HostLabelMaxLen = 63
	// HostLabelFallback names a segment whose source slugified to nothing.
	HostLabelFallback = "wtm"
```

- [ ] **Step 4: Écrire `internal/rules/hostlabel.go`**

```go
package rules

import (
	"strings"

	"github.com/LucasPcq/wtm/internal/domain"
)

// HostLabel turns a derived name into one DNS label. WorktreeSlug is not enough
// here: it keeps underscores, leaves a trailing dash and bounds nothing, none of
// which a hostname accepts.
func HostLabel(s string) string {
	mapped := strings.Map(func(r rune) rune {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9', r == '-':
			return r
		case r >= 'A' && r <= 'Z':
			return r + ('a' - 'A')
		default:
			return '-'
		}
	}, s)

	var b strings.Builder
	for _, r := range mapped {
		if r == '-' && strings.HasSuffix(b.String(), "-") {
			continue
		}
		b.WriteRune(r)
	}

	label := strings.Trim(b.String(), "-")
	if len(label) > domain.HostLabelMaxLen {
		label = strings.Trim(label[:domain.HostLabelMaxLen], "-")
	}
	if label == "" {
		return domain.HostLabelFallback
	}
	return label
}

// IsHostLabels reports whether s is a dotted sequence of valid DNS labels. It is
// the check a hand-written url.host goes through: a value the user typed is
// refused, never silently corrected, so the URL on screen is the one they wrote.
func IsHostLabels(s string) bool {
	if s == "" {
		return false
	}
	for _, label := range strings.Split(s, ".") {
		if !isHostLabel(label) {
			return false
		}
	}
	return true
}

func isHostLabel(label string) bool {
	if label == "" || len(label) > domain.HostLabelMaxLen {
		return false
	}
	if strings.HasPrefix(label, "-") || strings.HasSuffix(label, "-") {
		return false
	}
	for _, r := range label {
		if r >= 'a' && r <= 'z' || r >= '0' && r <= '9' || r == '-' {
			continue
		}
		return false
	}
	return true
}
```

- [ ] **Step 5: Lancer le test, vérifier qu'il passe**

Run: `go test ./internal/rules/ -run 'TestHostLabel|TestIsHostLabels' -v`
Expected: PASS.

- [ ] **Step 6: Ajouter le type et le champ dans `domain`**

Dans `internal/domain/jobs.go`, au-dessus de `JobConfig` :

```go
// JobURLConfig is a job's [[job]].url table: which of its declared ports speaks
// HTTP, and the host label it is published under. Port names a key of Ports, not
// a number — the number depends on the worktree, only the declaration is stable.
// Host is optional and defaults to the job's name.
type JobURLConfig struct {
	Port string `toml:"port"           json:"port"`
	Host string `toml:"host,omitempty" json:"host,omitempty"`
}
```

et dans `JobConfig`, après `Ports` :

```go
	// URL publishes one of the ports above under a name. Absent means the job
	// keeps no name and stays reachable by its port, as before.
	URL *JobURLConfig `toml:"url,omitempty" json:"url,omitempty"`
```

Le pointeur est ce qui rend l'absence propre à l'encodage : vérifié, `toml.Encoder` omet le champ nil et réécrit `[job.url]` sinon, et le décodeur relit aussi bien la table inline `url = { port = "PORT" }` écrite à la main.

- [ ] **Step 7: Écrire le test de validation (échec attendu)**

Créer `internal/rules/urlvalidate_test.go` :

```go
package rules

import (
	"strings"
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
)

func TestValidateRunPortsURL(t *testing.T) {
	tests := []struct {
		name string
		cfg  domain.RunConfig
		want string
	}{
		{
			name: "url.port doit nommer un port déclaré",
			cfg: domain.RunConfig{Jobs: []domain.JobConfig{
				{Name: "web", Ports: map[string]int{"PORT": 3000}, URL: &domain.JobURLConfig{Port: "HTTP_PORT"}},
			}},
			want: `job "web": url.port names HTTP_PORT, which the job does not declare`,
		},
		{
			name: "url.host doit être une suite de labels",
			cfg: domain.RunConfig{Jobs: []domain.JobConfig{
				{Name: "web", Ports: map[string]int{"PORT": 3000}, URL: &domain.JobURLConfig{Port: "PORT", Host: "Web_1"}},
			}},
			want: `job "web": url.host "Web_1" is not a valid hostname`,
		},
		{
			name: "deux jobs ne peuvent pas revendiquer le même hôte",
			cfg: domain.RunConfig{Jobs: []domain.JobConfig{
				{Name: "web", Ports: map[string]int{"PORT": 3000}, URL: &domain.JobURLConfig{Port: "PORT", Host: "app"}},
				{Name: "bo", Ports: map[string]int{"PORT": 3001}, URL: &domain.JobURLConfig{Port: "PORT", Host: "app"}},
			}},
			want: `jobs "web" and "bo" both publish host "app"`,
		},
		{
			name: "le nom du job sert d'hôte par défaut et collisionne pareil",
			cfg: domain.RunConfig{Jobs: []domain.JobConfig{
				{Name: "app_web", Ports: map[string]int{"PORT": 3000}, URL: &domain.JobURLConfig{Port: "PORT"}},
				{Name: "app-web", Ports: map[string]int{"PORT": 3001}, URL: &domain.JobURLConfig{Port: "PORT"}},
			}},
			want: `both publish host "app-web"`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errs := ValidateRunPorts(tt.cfg)
			if !containsSubstring(errs, tt.want) {
				t.Errorf("errors %v, want one containing %q", errs, tt.want)
			}
		})
	}
}

func TestValidateRunPortsURLAccepts(t *testing.T) {
	cfg := domain.RunConfig{Jobs: []domain.JobConfig{
		{Name: "app1-web", Ports: map[string]int{"PORT": 3000}, URL: &domain.JobURLConfig{Port: "PORT"}},
		{Name: "app1-api", Ports: map[string]int{"PORT": 4000}, URL: &domain.JobURLConfig{Port: "PORT", Host: "api.app-1"}},
		{Name: "db", Ports: map[string]int{"PG_PORT": 5432}},
	}}
	if errs := ValidateRunPorts(cfg); len(errs) > 0 {
		t.Errorf("a valid config must not be refused, got %v", errs)
	}
}

func containsSubstring(errs []string, want string) bool {
	for _, e := range errs {
		if strings.Contains(e, want) {
			return true
		}
	}
	return false
}
```

- [ ] **Step 8: Lancer, vérifier l'échec**

Run: `go test ./internal/rules/ -run TestValidateRunPortsURL -v`
Expected: FAIL — aucune erreur produite pour les trois premiers cas.

- [ ] **Step 9: Implémenter la validation**

Dans `internal/rules/validate.go`, remplacer la dernière ligne de `ValidateRunPorts` :

```go
	return append(errs, validateEnvPortLinks(cfg)...)
```

par :

```go
	errs = append(errs, validateJobURLs(cfg)...)
	return append(errs, validateEnvPortLinks(cfg)...)
```

et ajouter, à côté de `validateEnvPortLinks` :

```go
// validateJobURLs checks what run.toml can answer for on its own. The collision
// case is the one that matters: without it the proxy arbitrates silently and
// half the traffic goes to the wrong worktree.
func validateJobURLs(cfg domain.RunConfig) []string {
	var errs []string
	claimed := map[string]string{}

	for _, job := range cfg.Jobs {
		if job.URL == nil {
			continue
		}
		if _, declared := job.Ports[job.URL.Port]; !declared {
			errs = append(errs, fmt.Sprintf("job %q: url.port names %s, which the job does not declare", job.Name, job.URL.Port))
		}
		if job.URL.Host != "" && !IsHostLabels(job.URL.Host) {
			errs = append(errs, fmt.Sprintf("job %q: url.host %q is not a valid hostname — lowercase letters, digits and dashes, dot-separated", job.Name, job.URL.Host))
			continue
		}

		host := JobHostLabel(job)
		if owner, taken := claimed[host]; taken {
			errs = append(errs, fmt.Sprintf("jobs %q and %q both publish host %q — give one an explicit url.host", owner, job.Name, host))
			continue
		}
		claimed[host] = job.Name
	}
	return errs
}

// JobHostLabel is the host segment a job publishes under: what it declared, else
// its own name made safe for DNS.
func JobHostLabel(job domain.JobConfig) string {
	if job.URL == nil {
		return ""
	}
	if job.URL.Host != "" {
		return job.URL.Host
	}
	return HostLabel(job.Name)
}
```

- [ ] **Step 10: Lancer, vérifier que ça passe**

Run: `go test ./internal/rules/ -run TestValidateRunPortsURL -v`
Expected: PASS (les deux fonctions de test).

- [ ] **Step 11: Test de round-trip TOML**

Ajouter à `internal/config/jobs_test.go` :

```go
func TestLoadRunReadsJobURL(t *testing.T) {
	dir := t.TempDir()
	content := `
[[job]]
name  = "web"
kind  = "service"
cmd   = "pnpm dev --port ${PORT}"
ports = { PORT = 3000 }
url   = { port = "PORT" }

[[job]]
name  = "api"
kind  = "service"
cmd   = "pnpm dev --port ${PORT}"
ports = { PORT = 4000 }
url   = { port = "PORT", host = "api.app-1" }

[[job]]
name  = "db"
kind  = "service"
cmd   = "docker compose up"
ports = { PG_PORT = 5432 }
`
	if err := os.WriteFile(filepath.Join(dir, domain.RunFileName), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := LoadRun(dir)
	if err != nil {
		t.Fatalf("LoadRun: %v", err)
	}
	if cfg.Jobs[0].URL == nil || cfg.Jobs[0].URL.Port != "PORT" || cfg.Jobs[0].URL.Host != "" {
		t.Errorf("web url = %+v, want port PORT and no host", cfg.Jobs[0].URL)
	}
	if cfg.Jobs[1].URL == nil || cfg.Jobs[1].URL.Host != "api.app-1" {
		t.Errorf("api url = %+v, want host api.app-1", cfg.Jobs[1].URL)
	}
	if cfg.Jobs[2].URL != nil {
		t.Errorf("db must carry no url, got %+v", cfg.Jobs[2].URL)
	}
}
```

Ajouter les imports `os`, `path/filepath` et `domain` s'ils manquent au fichier.

- [ ] **Step 12: Lancer**

Run: `go test ./internal/config/ -run TestLoadRunReadsJobURL -v`
Expected: PASS. Si le décodeur strict refuse `url`, c'est que `JobConfig.URL` n'a pas le bon tag `toml` — corriger le tag, pas le test.

- [ ] **Step 13: Mettre à jour le schéma JSON**

Dans `internal/schemas/run.schema.json`, dans `properties.job.items.properties`, après `ports` :

```json
"url": {
  "type": "object",
  "additionalProperties": false,
  "required": ["port"],
  "description": "Publishes one of this job's declared ports under a name, so the worktree gets its own hostname instead of a port to remember — and its own cookie jar, since cookies ignore the port. Omit for a job that does not speak HTTP.",
  "properties": {
    "port": {
      "type": "string",
      "description": "Name of the port to publish. Must be one of the keys declared in `ports` above — the name, not the number: the number depends on the worktree."
    },
    "host": {
      "type": "string",
      "pattern": "^[a-z0-9]([a-z0-9-]*[a-z0-9])?(\\.[a-z0-9]([a-z0-9-]*[a-z0-9])?)*$",
      "description": "Host segment this job is published under, defaulting to the job's name. May contain dots to create an intermediate level: `web.app-1` puts the job under `.app-1.<worktree>.<project>.localhost`, which the app's siblings then share a cookie parent with."
    }
  }
}
```

- [ ] **Step 14: Valider et committer**

Run: `go build ./... && go vet ./... && go test ./internal/rules/ ./internal/config/ -count=1`
Expected: tout passe.

```bash
git add internal/domain/jobs.go internal/domain/constants.go internal/rules/hostlabel.go internal/rules/hostlabel_test.go internal/rules/validate.go internal/rules/urlvalidate_test.go internal/config/jobs_test.go internal/schemas/run.schema.json
git commit -m "feat(run): un job peut publier un de ses ports sous un nom"
```

---

## Lot 2 : l'URL jusqu'aux trois surfaces

**Files:**
- Create: `internal/rules/joburl.go`, `internal/rules/joburl_test.go`
- Create: `internal/output/hyperlink.go`, `internal/output/hyperlink_test.go`
- Modify: `internal/domain/constants.go`
- Modify: `internal/rules/jobenv.go`
- Modify: `internal/flow/runlogs/runlogs.go:140-162` (`Event`)
- Modify: `internal/flow/runlogs/run.go:150,159`
- Modify: `internal/output/runlogs.go:40-56`
- Modify: `internal/tui/runview/render.go:213-220`
- Test: `internal/flow/runlogs/run_test.go`, `internal/output/runlogs_ports_test.go`

**Interfaces:**
- Consumes: `rules.JobHostLabel`, `domain.JobURLConfig` (Lot 1) ; `runlogs.Event`, `rules.LabelWithPorts` (existants).
- Produces:
  - `rules.JobURL(params rules.JobURLParams) string` avec `JobURLParams{Job domain.JobConfig; Ports map[string]int}`
  - `runlogs.Event.URL string`
  - `output.Hyperlink(params output.HyperlinkParams) string` avec `HyperlinkParams{Text, URL string; Enabled bool}`
  - `domain.EnvProject` (= `"WTM_PROJECT"`), injecté par `rules.WorktreeJobEnv`

- [ ] **Step 1: Écrire le test de `JobURL` (échec attendu)**

Créer `internal/rules/joburl_test.go` :

```go
package rules

import (
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
)

func TestJobURL(t *testing.T) {
	web := domain.JobConfig{Name: "web", Ports: map[string]int{"PORT": 3000}, URL: &domain.JobURLConfig{Port: "PORT"}}

	tests := []struct {
		name  string
		job   domain.JobConfig
		ports map[string]int
		want  string
	}{
		{"port résolu du worktree", web, map[string]int{"PORT": 3010}, "http://localhost:3010"},
		{"worktree principal", web, map[string]int{"PORT": 3000}, "http://localhost:3000"},
		{"job sans url", domain.JobConfig{Name: "db", Ports: map[string]int{"PG_PORT": 5432}}, map[string]int{"PG_PORT": 5442}, ""},
		{"port absent des résolus", web, map[string]int{"OTHER": 9000}, ""},
		{"aucun port résolu", web, nil, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := JobURL(JobURLParams{Job: tt.job, Ports: tt.ports}); got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}
```

- [ ] **Step 2: Lancer, vérifier l'échec**

Run: `go test ./internal/rules/ -run TestJobURL -v`
Expected: FAIL — `undefined: JobURL`.

- [ ] **Step 3: Ajouter les constantes de format**

Dans `internal/domain/constants.go` :

```go
	// DirectURLFmt is a job's URL without the proxy: its own port on the loopback.
	DirectURLFmt = "http://localhost:%d"
	// EnvProject is the repository's slug, as the hostname and the compose
	// project name both derive from it.
	EnvProject = "WTM_PROJECT"
```

- [ ] **Step 4: Écrire `internal/rules/joburl.go`**

```go
package rules

import (
	"fmt"

	"github.com/LucasPcq/wtm/internal/domain"
)

type JobURLParams struct {
	Job domain.JobConfig
	// Ports is what the job actually bound in this worktree, base plus offset.
	Ports map[string]int
}

// JobURL is where a job is reachable, empty for one that publishes nothing. This
// is the single place a surface asks; the proxy changes what it answers, not who
// asks.
func JobURL(params JobURLParams) string {
	if params.Job.URL == nil {
		return ""
	}
	port, bound := params.Ports[params.Job.URL.Port]
	if !bound {
		return ""
	}
	return fmt.Sprintf(domain.DirectURLFmt, port)
}
```

- [ ] **Step 5: Lancer, vérifier que ça passe**

Run: `go test ./internal/rules/ -run TestJobURL -v`
Expected: PASS.

- [ ] **Step 6: Injecter `WTM_PROJECT`**

Dans `internal/rules/jobenv.go`, `WorktreeJobEnv`, ajouter à la map retournée :

```go
		domain.EnvProject: HostLabel(params.Project),
```

Puis mettre à jour l'assertion correspondante dans `internal/rules/jobenv_test.go` si elle compare la map entière.

Run: `go test ./internal/rules/ -run TestWorktreeJobEnv -v`
Expected: PASS.

- [ ] **Step 7: Porter l'URL dans l'événement**

Dans `internal/flow/runlogs/runlogs.go`, `Event`, après `Ports` :

```go
	// URL is where a PhaseStarted or PhaseDone job is reachable, empty for one
	// that publishes no name.
	URL string
```

Dans `internal/flow/runlogs/run.go`, remplacer les deux émissions :

```go
			r.emit(Event{Phase: PhaseDone, Job: job.Name, Step: i + 1, Ports: result.Ports})
```
```go
			r.emit(Event{Phase: PhaseStarted, Job: job.Name, Step: i + 1, AlreadyRunning: alreadyRunning, Ports: result.Ports})
```

par les mêmes lignes avec `URL: rules.JobURL(rules.JobURLParams{Job: job, Ports: result.Ports})`. Ajouter l'import `"github.com/LucasPcq/wtm/internal/rules"` s'il manque.

- [ ] **Step 8: Test du flow**

Ajouter à `internal/flow/runlogs/run_test.go` :

```go
func TestRunEmitsJobURL(t *testing.T) {
	job := domain.JobConfig{
		Name:  "web",
		Kind:  domain.JobKindService,
		Cmd:   "pnpm dev",
		Ports: map[string]int{"PORT": 3000},
		URL:   &domain.JobURLConfig{Port: "PORT"},
	}
	service := &runlogstest.Service{Ports: map[string]map[string]int{"web": {"PORT": 3010}}}
	sink := &runlogstest.Sink{}

	if _, err := Run(context.Background(), RunParams{Service: service, Sink: sink, Jobs: []domain.JobConfig{job}, WorkDir: "/w"}); err != nil {
		t.Fatalf("Run: %v", err)
	}

	for _, e := range sink.Events {
		if e.Phase == PhaseStarted && e.URL != "http://localhost:3010" {
			t.Errorf("started URL = %q, want http://localhost:3010", e.URL)
		}
	}
}
```

Adapter les noms des doubles (`runlogstest.Service`, `runlogstest.Sink`) à ceux réellement présents dans `internal/testutil/runlogstest` — les lire avant d'écrire le test, et calquer la construction sur `internal/flow/runlogs/probe_test.go:26-35`.

Run: `go test ./internal/flow/runlogs/ -run TestRunEmitsJobURL -v`
Expected: PASS.

- [ ] **Step 9: Le lien cliquable**

Créer `internal/output/hyperlink.go` :

```go
package output

import "fmt"

type HyperlinkParams struct {
	Text string
	URL  string
	// Enabled is false for a pipe, a JSON run, or a terminal that would only
	// show the escape sequence as garbage.
	Enabled bool
}

// Hyperlink wraps text in an OSC-8 sequence so the terminal makes it clickable,
// and returns it untouched everywhere else.
func Hyperlink(params HyperlinkParams) string {
	if !params.Enabled || params.URL == "" {
		return params.Text
	}
	return fmt.Sprintf("\x1b]8;;%s\x1b\\%s\x1b]8;;\x1b\\", params.URL, params.Text)
}
```

Créer `internal/output/hyperlink_test.go` :

```go
package output

import (
	"strings"
	"testing"
)

func TestHyperlink(t *testing.T) {
	on := Hyperlink(HyperlinkParams{Text: "http://x.localhost:4000", URL: "http://x.localhost:4000", Enabled: true})
	if !strings.HasPrefix(on, "\x1b]8;;") || !strings.Contains(on, "http://x.localhost:4000") {
		t.Errorf("enabled must wrap in OSC-8, got %q", on)
	}

	off := Hyperlink(HyperlinkParams{Text: "http://x.localhost:4000", URL: "http://x.localhost:4000"})
	if off != "http://x.localhost:4000" {
		t.Errorf("disabled must stay raw, got %q", off)
	}

	none := Hyperlink(HyperlinkParams{Text: "web", Enabled: true})
	if none != "web" {
		t.Errorf("no URL must stay raw, got %q", none)
	}
}
```

Run: `go test ./internal/output/ -run TestHyperlink -v`
Expected: PASS.

- [ ] **Step 10: La colonne URL sur le flux**

Dans `internal/output/runlogs.go` : `RunPrinterParams` gagne `Hyperlinks bool`, `RunPrinter` le champ `hyperlinks bool` correspondant, et `NewRunPrinter` le recopie. Les runners qui construisent le printer (`internal/commands/run/surface.go`, `start.go`) le renseignent à `rules.IsHumanFormat(format) && isTTY()`. Sur `PhaseStarted` et `PhaseDone`, faire suivre la ligne existante de l'URL quand elle n'est pas vide :

```go
		line := rules.LabelWithPorts(rules.LabelWithPortsParams{
			Label: fmt.Sprintf(domain.RunStreamStartedFmt, event.Job),
			Ports: event.Ports,
		})
		if event.URL != "" {
			line += domain.RunURLSuffixSep + Hyperlink(HyperlinkParams{Text: event.URL, URL: event.URL, Enabled: p.hyperlinks})
		}
		Success(p.out, line)
```

Ajouter dans `constants.go` : `RunURLSuffixSep = "   "`. Appliquer la même chose au bras `PhaseDone`.

- [ ] **Step 11: Test de la sortie flux**

Ajouter à `internal/output/runlogs_ports_test.go` :

```go
func TestRunPrinterShowsURL(t *testing.T) {
	var out, errOut bytes.Buffer
	p := NewRunPrinter(RunPrinterParams{Out: &out, Err: &errOut})
	p.Emit(runlogs.Event{Phase: runlogs.PhaseStarted, Job: "web", Ports: map[string]int{"PORT": 3010}, URL: "http://localhost:3010"})

	if !strings.Contains(out.String(), "http://localhost:3010") {
		t.Errorf("started line must carry the URL, got %q", out.String())
	}
	if strings.Contains(out.String(), "\x1b]8;;") {
		t.Errorf("hyperlinks are off by default, got %q", out.String())
	}
}
```

Run: `go test ./internal/output/ -run TestRunPrinterShowsURL -v`
Expected: PASS.

- [ ] **Step 12: La colonne URL dans la vue**

Dans `internal/tui/runview/render.go:213-220`, `renderPaneTitle` ajoute l'URL au `status` quand `m.sequence` en tient une pour ce job. Suivre le chemin de `m.sequence.ports[params.View.Name]` : ajouter un `urls map[string]string` à la structure `sequence`, alimenté au même endroit que `ports` depuis `Event.URL`. Rendre l'URL avec `styles.Muted`, jamais en instanciant un `lipgloss.Style` ici — `styles/` est le seul paquet qui a le droit.

Chercher l'endroit exact : `grep -n "ports\[" internal/tui/runview/*.go`.

- [ ] **Step 12b: La touche `o` ouvre l'URL du job sélectionné**

Dans le `Update` de `internal/tui/runview`, ajouter le cas `"o"` à côté des touches déjà gérées : si le job sélectionné a une URL, l'ouvrir ; sinon ne rien faire (pas de message d'erreur pour une touche sans objet).

La vue n'a pas le droit d'ouvrir un navigateur elle-même — `tui/` ne décide rien et n'a pas d'I/O métier. Le modèle rend une `tea.Cmd` qui appelle l'ouvreur de `internal/service/integration/`, injecté dans le modèle comme les autres coutures de ce paquet. Vérifier ce qui existe déjà : `grep -rn "xdg-open\|exec.Command(\"open\"" internal/service`.

Ajouter au fichier de test de la vue un cas vérifiant qu'un `o` sur un job sans URL ne produit aucune commande.

- [ ] **Step 13: Valider et committer**

Run: `make test`
Expected: tout passe.

```bash
git add internal/rules/joburl.go internal/rules/joburl_test.go internal/rules/jobenv.go internal/domain/constants.go internal/flow/runlogs/ internal/output/hyperlink.go internal/output/hyperlink_test.go internal/output/runlogs.go internal/output/runlogs_ports_test.go internal/tui/runview/
git commit -m "feat(run): la ligne d'un job dit où le joindre, pas seulement sur quel port"
```

---

## Lot 3 : `wtm run url` et `wtm run open`

**Files:**
- Create: `internal/commands/run/url.go`, `internal/commands/run/url_test.go`
- Create: `internal/commands/run/open.go`
- Modify: `internal/commands/run/run.go:22-33`
- Modify: `internal/domain/constants.go`
- Modify: `internal/domain/errors.go`
- Modify: `internal/output/jobs.go` (JSON), `internal/domain/jobs.go` (`JobInfo.URL`)
- Modify: `internal/commands/agents/assets/using-wtm.skill.md`, `README.md`, `docs/`

**Interfaces:**
- Consumes: `rules.JobURL`, `rules.JobURLParams` (Lot 2) ; `shared.LoadConfig`, `config.LoadRun`, `shared.AddOutputFlag`, `worktree.JobEnv`, `rules.JobPorts` (existants).
- Produces:
  - `domain.CmdURL = "url"`, `domain.CmdOpen = "open"`, `domain.FlagRaw = "raw"`
  - `domain.ErrJobAmbiguous`
  - `domain.JobInfo.URL string` (champ JSON `url`, omis si vide)

- [ ] **Step 1: Écrire le test des gardes non-interactives (échec attendu)**

Créer `internal/commands/run/url_test.go`. Les helpers existent déjà dans le paquet : `setupTestProject(t) string` (dépôt git temporaire + config), `writeRunTOML(t, stateDir, cfg)`, `runCmd(t, args...) (stdout, stderr string, err error)` et `fakeTTY(t, bool)`.

```go
package run

import (
	"errors"
	"strings"
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
)

func published(name string, base int, host string) domain.JobConfig {
	return domain.JobConfig{
		Name:  name,
		Kind:  domain.JobKindService,
		Cmd:   "pnpm dev --port ${PORT}",
		Ports: map[string]int{"PORT": base},
		URL:   &domain.JobURLConfig{Port: "PORT", Host: host},
	}
}

func TestRunURLPrintsTheOnlyURL(t *testing.T) {
	stateDir := setupTestProject(t)
	writeRunTOML(t, stateDir, domain.RunConfig{Jobs: []domain.JobConfig{
		published("web", 3000, ""),
		{Name: "db", Kind: domain.JobKindService, Cmd: "docker compose up", Ports: map[string]int{"PG_PORT": 5432}},
	}})
	fakeTTY(t, false)

	stdout, _, err := runCmd(t, domain.CmdURL)
	if err != nil {
		t.Fatalf("run url: %v", err)
	}
	// Le dépôt de test est le worktree principal : ordinal 0, donc le port déclaré.
	if stdout != "http://localhost:3000\n" {
		t.Errorf("stdout = %q, want the bare URL and nothing else", stdout)
	}
}

func TestRunURLNamesTheJobWhenAmbiguous(t *testing.T) {
	stateDir := setupTestProject(t)
	writeRunTOML(t, stateDir, domain.RunConfig{Jobs: []domain.JobConfig{
		published("web", 3000, ""),
		published("api", 4000, ""),
	}})
	fakeTTY(t, false)

	_, _, err := runCmd(t, domain.CmdURL)
	if !errors.Is(err, domain.ErrJobAmbiguous) {
		t.Fatalf("err = %v, want ErrJobAmbiguous — a machine surface never falls back to a picker", err)
	}
}

func TestRunURLNamedJobWins(t *testing.T) {
	stateDir := setupTestProject(t)
	writeRunTOML(t, stateDir, domain.RunConfig{Jobs: []domain.JobConfig{
		published("web", 3000, ""),
		published("api", 4000, ""),
	}})
	fakeTTY(t, false)

	stdout, _, err := runCmd(t, domain.CmdURL, "api")
	if err != nil {
		t.Fatalf("run url api: %v", err)
	}
	if !strings.Contains(stdout, "4000") {
		t.Errorf("stdout = %q, want api's own port", stdout)
	}
}

func TestRunURLUnknownJobNamesTheOnesThatPublish(t *testing.T) {
	stateDir := setupTestProject(t)
	writeRunTOML(t, stateDir, domain.RunConfig{Jobs: []domain.JobConfig{published("web", 3000, "")}})
	fakeTTY(t, false)

	_, _, err := runCmd(t, domain.CmdURL, "nope")
	if err == nil || !strings.Contains(err.Error(), "web") {
		t.Fatalf("err = %v, want one naming the jobs that do publish", err)
	}
}
```

Au Lot 6, ajouter à ce fichier `TestRunURLRawStaysDirect` : la même mise en place avec un `[proxy]` actif, `--raw` devant toujours rendre `http://localhost:3000`.

- [ ] **Step 2: Lancer, vérifier l'échec**

Run: `go test ./internal/commands/run/ -run TestRunURL -v`
Expected: FAIL — la commande n'existe pas.

- [ ] **Step 3: Constantes et erreur sentinelle**

`internal/domain/constants.go` :

```go
	CmdURL  = "url"
	CmdOpen = "open"
	FlagRaw = "raw"
```

`internal/domain/errors.go` :

```go
// ErrJobAmbiguous is returned when several jobs publish a URL and the caller
// named none — a picker needs a fully interactive run, so the flag is the answer.
var ErrJobAmbiguous = errors.New("several jobs publish a URL: name one")
```

- [ ] **Step 4: Écrire `internal/commands/run/url.go`**

```go
package run

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/LucasPcq/wtm/internal/commands/shared"
	"github.com/LucasPcq/wtm/internal/config"
	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/output"
	"github.com/LucasPcq/wtm/internal/rules"
)

func newURLCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   domain.CmdURL + " [job]",
		Short: "Print where a job is reachable in this worktree",
		Long:  "Write a job's URL on stdout and nothing else, for $(…). --raw prints the job's own port instead of its name, which every OS resolves and no proxy has to serve.",
		RunE:  runURL,
	}
	cmd.Flags().Bool(domain.FlagRaw, false, "Print the direct http://localhost:<port> address")
	shared.AddOutputFlag(cmd)
	return cmd
}

func runURL(cmd *cobra.Command, args []string) error {
	entries, err := publishedJobs(cmd)
	if err != nil {
		return err
	}

	format, _ := cmd.Flags().GetString(domain.FlagOutput)
	if format == domain.OutputJSON {
		return output.WriteJobURLsJSON(cmd.OutOrStdout(), entries)
	}

	entry, err := pickPublished(entries, args)
	if err != nil {
		return err
	}
	fmt.Fprintln(cmd.OutOrStdout(), entry.URL)
	return nil
}
```

`publishedJobs` résout la configuration puis les ports du worktree courant, exactement comme `up.go:45-60` puis `openRunSeam` : `shared.LoadConfig` → `config.LoadRun` → `jobEnv(...)` pour lire `domain.EnvPortOffset` → `rules.JobPorts` par job → `rules.JobURL`. Elle ne garde que les jobs dont l'URL n'est pas vide.

`pickPublished` applique la règle du CLAUDE.md : zéro entrée → message clair ; une seule et aucun argument → celle-là ; un argument → la correspondance exacte, sinon une erreur nommant les jobs disponibles ; plusieurs et aucun argument → `domain.ErrJobAmbiguous`. Aucun picker : `run url` est une surface machine.

- [ ] **Step 5: Écrire `internal/commands/run/open.go`**

Même résolution, puis ouverture du navigateur. `run open` a le droit à un picker, mais seulement en run pleinement interactif :

```go
func runOpen(cmd *cobra.Command, args []string) error {
	entries, err := publishedJobs(cmd)
	if err != nil {
		return err
	}

	format, _ := cmd.Flags().GetString(domain.FlagOutput)
	interactive := isTTY() && rules.IsHumanFormat(format)

	entry, err := pickPublishedInteractive(entries, args, interactive)
	if err != nil {
		return err
	}
	return openInBrowser(entry.URL)
}
```

`openInBrowser` va dans `internal/service/integration/` (le paquet des adaptateurs tiers), pas dans `commands/` : `open` sur darwin, `xdg-open` sur linux, `rundll32` sur windows. Vérifier d'abord si `internal/service/integration/` en a déjà un — `grep -rn "xdg-open" internal` — et le réutiliser plutôt que d'en écrire un second.

- [ ] **Step 6: Brancher les deux commandes**

Dans `internal/commands/run/run.go`, après `cmd.AddCommand(newPsCmd())` :

```go
	cmd.AddCommand(newURLCmd())
	cmd.AddCommand(newOpenCmd())
```

- [ ] **Step 7: Lancer les tests**

Run: `go test ./internal/commands/run/ -run TestRunURL -v`
Expected: PASS.

- [ ] **Step 8: Le champ `url` en JSON**

Dans `internal/domain/jobs.go`, `JobInfo` :

```go
	// URL is where the job is reachable, absent for one that publishes no name.
	URL string `json:"url,omitempty"`
```

Le renseigner là où `JobInfo` est construit pour `run ps` (`internal/service/process/manager.go`, méthode `List`) — la config du job y est disponible via `ManagedJob.Config`, et les ports via `jobPorts(job.Config, job.Env)`.

Ajouter un cas à `internal/output/jobs_test.go` vérifiant qu'un job sans URL n'émet pas la clé.

- [ ] **Step 9: Docs, skill agent, README**

- `internal/commands/agents/assets/using-wtm.skill.md` : les deux commandes avec leur forme non-interactive (`wtm run url <job>`, `--raw`, `--output json`), le champ `url` de `run.toml`, le champ `url` du JSON de `run ps`, et la règle « plusieurs jobs publient → nommer le job, jamais de picker ».
- `README.md` : ajouter `run url` et `run open` à la table d'aperçu, dans la section des jobs.
- Run: `make docs`

- [ ] **Step 10: Valider et committer**

Run: `make test && make vet && make lint`

```bash
git add internal/commands/run/ internal/domain/ internal/output/ internal/service/ internal/commands/agents/assets/using-wtm.skill.md README.md docs/
git commit -m "feat(run): deux commandes pour aller voir un job sans chercher son port"
```

---

# Phase F2 — Le proxy

Change **ce que** `rules.JobURL` répond. Ne rouvre ni `run.toml`, ni les commandes du Lot 3.

## Lot 4 : le nommage

**Files:**
- Create: `internal/rules/proxyroute.go`, `internal/rules/proxyroute_test.go`
- Create: `internal/domain/proxy.go`
- Modify: `internal/domain/constants.go`
- Modify: `internal/rules/joburl.go` (+ son test)

**Interfaces:**
- Consumes: `rules.HostLabel`, `rules.JobHostLabel` (Lot 1) ; `rules.JobURLParams` (Lot 2).
- Produces:
  - `domain.ProxyRoute{Host, Target, Job, Worktree, Project string}`
  - `rules.RouteHost(params rules.RouteHostParams) string` avec `RouteHostParams{Job domain.JobConfig; Worktree, Project string}`
  - `rules.JobURLParams` gagne `Host string` et `ProxyPort int`
  - (`rules.ProxyPort` arrive au Lot 6, dans le même fichier)

- [ ] **Step 1: Écrire le test de `RouteHost` (échec attendu)**

Créer `internal/rules/proxyroute_test.go` :

```go
package rules

import (
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
)

func TestRouteHost(t *testing.T) {
	published := func(name, host string) domain.JobConfig {
		return domain.JobConfig{Name: name, Ports: map[string]int{"PORT": 3000}, URL: &domain.JobURLConfig{Port: "PORT", Host: host}}
	}

	tests := []struct {
		name     string
		job      domain.JobConfig
		worktree string
		project  string
		want     string
	}{
		{"nom du job par défaut", published("app1-web", ""), "feat-auth", "myapp", "app1-web.feat-auth.myapp.localhost"},
		{"host déclaré", published("app1-api", "api.app-1"), "feat-auth", "myapp", "api.app-1.feat-auth.myapp.localhost"},
		{"segments assainis", published("App_Web", ""), "feat/auth", "My.App", "app-web.feat-auth.my-app.localhost"},
		{"job sans url", domain.JobConfig{Name: "db"}, "feat-auth", "myapp", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := RouteHost(RouteHostParams{Job: tt.job, Worktree: tt.worktree, Project: tt.project})
			if got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestRouteHostOrderIsolatesCookies(t *testing.T) {
	job := domain.JobConfig{Name: "web", Ports: map[string]int{"PORT": 3000}, URL: &domain.JobURLConfig{Port: "PORT"}}

	a := RouteHost(RouteHostParams{Job: job, Worktree: "feat-auth", Project: "myapp"})
	b := RouteHost(RouteHostParams{Job: job, Worktree: "main", Project: "myapp"})

	// Le parent commun doit être le projet, jamais le job : sinon un cookie posé
	// sur le parent fuiterait d'un worktree à l'autre, ce que la feature répare.
	if a == b {
		t.Fatalf("two worktrees must not share a hostname, both are %q", a)
	}
	if parent(a) == parent(b) {
		t.Errorf("worktrees must not share their cookie parent: %q and %q", a, b)
	}
}

func parent(host string) string {
	for i := 0; i < len(host); i++ {
		if host[i] == '.' {
			return host[i+1:]
		}
	}
	return host
}
```

- [ ] **Step 2: Lancer, vérifier l'échec**

Run: `go test ./internal/rules/ -run TestRouteHost -v`
Expected: FAIL — `undefined: RouteHost`.

- [ ] **Step 3: Constantes et type de domaine**

`internal/domain/constants.go` :

```go
	// ProxyTLD is the special-use TLD every wtm route lives under (RFC 6761).
	ProxyTLD = "localhost"
	// ProxyDefaultPort is what the run proxy listens on when the config says nothing.
	ProxyDefaultPort = 4000
	// ProxyURLFmt is a job's named URL.
	ProxyURLFmt = "http://%s:%d"
	// ProxyLoopbackFmt is an address on the loopback: what the server binds, and
	// what a route targets. Never every interface.
	ProxyLoopbackFmt = "127.0.0.1:%d"
```

Créer `internal/domain/proxy.go` :

```go
package domain

// ProxyRoute is one entry of the run proxy's table: a hostname, and the loopback
// address of the job answering under it. It is a projection of a running job,
// never a second source of truth.
type ProxyRoute struct {
	Host     string `json:"host"`
	Target   string `json:"target"`
	Job      string `json:"job"`
	Worktree string `json:"worktree"`
	Project  string `json:"project"`
}

// ProxyConfig is the [proxy] table of ~/.config/wtm/config.toml. The port is a
// property of the machine, not of a repository: the daemon is global and serves
// every repo at once.
type ProxyConfig struct {
	Port    int   `toml:"port,omitempty"    json:"port,omitempty"`
	Enabled *bool `toml:"enabled,omitempty" json:"enabled,omitempty"`
}
```

`Enabled` est un pointeur pour la même raison que `UIConfig.Animations` : distinguer « absent » de « explicitement faux ».

- [ ] **Step 4: Écrire `internal/rules/proxyroute.go`**

```go
package rules

import (
	"strings"

	"github.com/LucasPcq/wtm/internal/domain"
)

type RouteHostParams struct {
	Job domain.JobConfig
	// Worktree and Project are the raw names; they are made DNS-safe here so a
	// caller never has to remember to.
	Worktree string
	Project  string
}

// RouteHost is the hostname a job is published under, empty for one that
// publishes nothing.
//
// The order is <job>.<worktree>.<project> and not the reverse: a cookie set on
// .<worktree>.<project>.localhost is then shared by that worktree's jobs and
// invisible to the others, which is the whole point. The reverse order would
// leak a cookie from one worktree to the next.
func RouteHost(params RouteHostParams) string {
	label := JobHostLabel(params.Job)
	if label == "" {
		return ""
	}
	return strings.Join([]string{
		label,
		HostLabel(params.Worktree),
		HostLabel(params.Project),
		domain.ProxyTLD,
	}, ".")
}
```

`JobHostLabel` slugifie déjà le nom de job dérivé (Lot 1) et laisse intact un `url.host` déclaré, qui a été validé au chargement.

- [ ] **Step 5: Lancer, vérifier que ça passe**

Run: `go test ./internal/rules/ -run TestRouteHost -v`
Expected: PASS.

- [ ] **Step 6: Étendre `JobURL` à la forme nommée**

Ajouter les cas au test existant `TestJobURL` :

```go
		{"forme nommée quand le proxy sert", web, map[string]int{"PORT": 3010}, "http://web.feat-auth.myapp.localhost:4000"},
```

en passant `Host: "web.feat-auth.myapp.localhost", ProxyPort: 4000` dans les params — restructurer le tableau pour porter ces deux champs, et garder un cas sans eux qui doit toujours rendre la forme directe.

Puis dans `internal/rules/joburl.go` :

```go
type JobURLParams struct {
	Job   domain.JobConfig
	Ports map[string]int
	// Host is the route the proxy serves this job under. Empty means no proxy —
	// the direct address is then the honest answer, not a degraded one.
	Host      string
	ProxyPort int
}
```

et dans le corps, avant le `return` direct :

```go
	if params.Host != "" && params.ProxyPort > 0 {
		return fmt.Sprintf(domain.ProxyURLFmt, params.Host, params.ProxyPort)
	}
```

Run: `go test ./internal/rules/ -run TestJobURL -v`
Expected: PASS.

- [ ] **Step 7: Valider et committer**

Run: `go build ./... && go test ./internal/rules/ ./internal/domain/ -count=1`

```bash
git add internal/rules/proxyroute.go internal/rules/proxyroute_test.go internal/rules/joburl.go internal/rules/joburl_test.go internal/domain/proxy.go internal/domain/constants.go
git commit -m "feat(run): nommer un job par son worktree et son dépôt, dans cet ordre"
```

---

## Lot 5 : le serveur proxy

**Files:**
- Create: `internal/service/proxy/registry.go`, `internal/service/proxy/registry_test.go`
- Create: `internal/service/proxy/server.go`, `internal/service/proxy/server_test.go`
- Modify: `internal/domain/constants.go`

**Interfaces:**
- Consumes: `domain.ProxyRoute` (Lot 4).
- Produces:
  - `proxy.NewRegistry() *proxy.Registry`
  - `(*Registry).Add(route domain.ProxyRoute)`, `.Remove(host string)`, `.Lookup(host string) (domain.ProxyRoute, bool)`, `.List() []domain.ProxyRoute`
  - `proxy.NewServer(params proxy.ServerParams) *proxy.Server` avec `ServerParams{Port int; Registry *Registry}`
  - `(*Server).Start() error`, `.Close() error`, `.Addr() string`

- [ ] **Step 1: Test du Registry (échec attendu)**

Créer `internal/service/proxy/registry_test.go` : `Add` puis `Lookup` rend la route ; `Lookup` d'un hôte inconnu rend `false` ; `Remove` la retire ; `List` rend les routes triées par hôte ; l'accès concurrent ne déclenche pas le détecteur de course (une centaine de goroutines qui `Add`/`Lookup`, exécuté sous `-race`).

- [ ] **Step 2: Lancer, vérifier l'échec**

Run: `go test ./internal/service/proxy/ -race -v`
Expected: FAIL — le paquet n'existe pas.

- [ ] **Step 3: Écrire le Registry**

```go
// Package proxy serves the named URLs of running jobs on one loopback port.
package proxy

import (
	"sort"
	"strings"
	"sync"

	"github.com/LucasPcq/wtm/internal/domain"
)

// Registry is the routing table. It is a projection of the jobs the daemon is
// running, so it is only ever written from where those start and stop.
type Registry struct {
	mu     sync.RWMutex
	routes map[string]domain.ProxyRoute
}

func NewRegistry() *Registry {
	return &Registry{routes: map[string]domain.ProxyRoute{}}
}

func (r *Registry) Add(route domain.ProxyRoute) {
	if route.Host == "" {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.routes[strings.ToLower(route.Host)] = route
}

func (r *Registry) Remove(host string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.routes, strings.ToLower(host))
}

func (r *Registry) Lookup(host string) (domain.ProxyRoute, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	route, found := r.routes[strings.ToLower(host)]
	return route, found
}

func (r *Registry) List() []domain.ProxyRoute {
	r.mu.RLock()
	defer r.mu.RUnlock()

	out := make([]domain.ProxyRoute, 0, len(r.routes))
	for _, route := range r.routes {
		out = append(out, route)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Host < out[j].Host })
	return out
}
```

- [ ] **Step 4: Lancer**

Run: `go test ./internal/service/proxy/ -race -v`
Expected: PASS.

- [ ] **Step 5: Test du serveur (échec attendu)**

Créer `internal/service/proxy/server_test.go` :

```go
package proxy

import (
	"bufio"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
)

// serve starts a proxy on a free port with routes already registered, and
// returns the address to dial.
func serve(t *testing.T, routes ...domain.ProxyRoute) string {
	t.Helper()

	registry := NewRegistry()
	for _, route := range routes {
		registry.Add(route)
	}
	server := NewServer(ServerParams{Port: 0, Registry: registry})
	if err := server.Start(); err != nil {
		t.Fatalf("start: %v", err)
	}
	t.Cleanup(func() { _ = server.Close() })
	return server.Addr()
}

// get dials addr but asks for host, which is what a browser resolving
// *.localhost to the loopback does.
func get(t *testing.T, addr, host string) *http.Response {
	t.Helper()

	req, err := http.NewRequest(http.MethodGet, "http://"+addr+"/", nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Host = host

	resp, err := http.DefaultTransport.RoundTrip(req)
	if err != nil {
		t.Fatalf("roundtrip: %v", err)
	}
	t.Cleanup(func() { _ = resp.Body.Close() })
	return resp
}

func TestServerPassesTheHostThrough(t *testing.T) {
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, r.Host)
	}))
	t.Cleanup(backend.Close)

	target := mustHost(t, backend.URL)
	addr := serve(t, domain.ProxyRoute{Host: "web.feat.myapp.localhost", Target: target, Job: "web"})

	resp := get(t, addr, "web.feat.myapp.localhost")
	body, _ := io.ReadAll(resp.Body)

	// Rewriting the Host would put every worktree back in one cookie jar, which
	// is the bug this whole feature exists to fix.
	if string(body) != "web.feat.myapp.localhost" {
		t.Errorf("backend saw Host %q, want it untouched", body)
	}
	if resp.Header.Get("X-Forwarded-Host") == "" && resp.StatusCode == http.StatusOK {
		t.Log("X-Forwarded-* are set on the outbound request; assert them in the backend if this ever regresses")
	}
}

func TestServerUpgradesWebSockets(t *testing.T) {
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Upgrade") != "websocket" {
			t.Errorf("upgrade header lost, headers = %v", r.Header)
		}
		conn, _, err := w.(http.Hijacker).Hijack()
		if err != nil {
			t.Errorf("hijack: %v", err)
			return
		}
		defer conn.Close()
		_, _ = conn.Write([]byte("HTTP/1.1 101 Switching Protocols\r\nUpgrade: websocket\r\nConnection: Upgrade\r\n\r\nping"))
	}))
	t.Cleanup(backend.Close)

	addr := serve(t, domain.ProxyRoute{Host: "web.feat.myapp.localhost", Target: mustHost(t, backend.URL), Job: "web"})

	conn, err := net.Dial("tcp", addr)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close()

	_, _ = conn.Write([]byte("GET / HTTP/1.1\r\nHost: web.feat.myapp.localhost\r\nUpgrade: websocket\r\nConnection: Upgrade\r\n\r\n"))

	reader := bufio.NewReader(conn)
	status, err := reader.ReadString('\n')
	if err != nil {
		t.Fatalf("read status: %v", err)
	}
	if !strings.Contains(status, "101") {
		t.Fatalf("status = %q, want 101 Switching Protocols", status)
	}
}

func TestServerListsRoutesForAnUnknownHost(t *testing.T) {
	addr := serve(t, domain.ProxyRoute{Host: "web.feat.myapp.localhost", Target: "127.0.0.1:1", Job: "web", Worktree: "feat"})

	resp := get(t, addr, "web.typo.myapp.localhost")
	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("status = %d, want 404", resp.StatusCode)
	}
	// Landing on the list of routes that exist beats ERR_CONNECTION_REFUSED.
	if !strings.Contains(string(body), "web.feat.myapp.localhost") {
		t.Errorf("body = %q, want the known routes listed", body)
	}
}

func TestServerReportsASilentTarget(t *testing.T) {
	// Port 1 on the loopback: registered, nothing listening.
	addr := serve(t, domain.ProxyRoute{Host: "web.feat.myapp.localhost", Target: "127.0.0.1:1", Job: "web"})

	resp := get(t, addr, "web.feat.myapp.localhost")
	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusBadGateway {
		t.Errorf("status = %d, want 502", resp.StatusCode)
	}
	if !strings.Contains(string(body), "web") {
		t.Errorf("body = %q, want the job named", body)
	}
}

func mustHost(t *testing.T, raw string) string {
	t.Helper()
	u, err := url.Parse(raw)
	if err != nil {
		t.Fatal(err)
	}
	return u.Host
}
```

`ServerParams.Port` valant `0`, le noyau attribue un port libre — c'est ce qui rend ces tests parallélisables et insensibles à ce qui tourne sur la machine. `Addr()` doit donc rendre l'adresse réelle du listener, pas `s.port`.

- [ ] **Step 6: Lancer, vérifier l'échec**

Run: `go test ./internal/service/proxy/ -run TestServer -race -v`
Expected: FAIL — `undefined: NewServer`.

- [ ] **Step 7: Écrire le serveur**

```go
package proxy

import (
	"fmt"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"

	"github.com/LucasPcq/wtm/internal/domain"
)

type ServerParams struct {
	Port     int
	Registry *Registry
}

type Server struct {
	registry *Registry
	listener net.Listener
	http     *http.Server
	port     int
}

func NewServer(params ServerParams) *Server {
	return &Server{registry: params.Registry, port: params.Port}
}

// Start binds the loopback and serves until Close. The bind error is returned
// rather than fatal: a busy port costs the names, never the jobs.
func (s *Server) Start() error {
	listener, err := net.Listen("tcp", fmt.Sprintf(domain.ProxyLoopbackFmt, s.port))
	if err != nil {
		return err
	}
	s.listener = listener
	s.http = &http.Server{Handler: http.HandlerFunc(s.route)}

	go func() { _ = s.http.Serve(listener) }()
	return nil
}

func (s *Server) Addr() string {
	if s.listener == nil {
		return ""
	}
	return s.listener.Addr().String()
}

func (s *Server) Close() error {
	if s.http == nil {
		return nil
	}
	return s.http.Close()
}

func (s *Server) route(w http.ResponseWriter, r *http.Request) {
	host := r.Host
	if h, _, err := net.SplitHostPort(host); err == nil {
		host = h
	}

	route, found := s.registry.Lookup(host)
	if !found {
		s.writeUnknownHost(w, host)
		return
	}

	rp := &httputil.ReverseProxy{
		Rewrite: func(pr *httputil.ProxyRequest) {
			pr.SetURL(&url.URL{Scheme: "http", Host: route.Target})
			// The inbound Host is what the browser scopes cookies to; forwarding
			// it unchanged is the isolation this whole feature exists for.
			pr.Out.Host = pr.In.Host
			pr.SetXForwarded()
		},
		ErrorHandler: func(ew http.ResponseWriter, _ *http.Request, _ error) {
			s.writeSilentTarget(ew, route)
		},
	}
	rp.ServeHTTP(w, r)
}
```

`writeUnknownHost` répond `404` avec la liste de `s.registry.List()` — hôte, job, worktree — et `writeSilentTarget` répond `502` en nommant le job et sa cible. Les deux écrivent du texte brut : le paquet `service/` n'a pas le droit d'importer `lipgloss`.

`pr.SetXForwarded()` pose `X-Forwarded-For`, `-Proto` et `-Host` ; c'est `pr.Out.Host = pr.In.Host` juste avant qui garde le `Host` d'origine, `SetURL` l'ayant réécrit.

- [ ] **Step 8: Lancer**

Run: `go test ./internal/service/proxy/ -race -v`
Expected: PASS, les cinq fonctions.

- [ ] **Step 9: Valider et committer**

Run: `go build ./... && go vet ./... && staticcheck ./internal/service/proxy/`

```bash
git add internal/service/proxy/ internal/domain/constants.go
git commit -m "feat(run): un proxy qui aiguille sur le Host sans jamais le réécrire"
```

---

## Lot 6 : le daemon sert les noms

**Files:**
- Modify: `internal/service/process/protocol.go:20-40`
- Modify: `internal/service/process/manager.go:59-90,116-180,318`
- Modify: `internal/service/process/daemon.go:34-60`
- Modify: `internal/domain/config.go`
- Modify: `internal/config/load.go`
- Modify: `internal/flow/runlogs/daemon.go`, `internal/flow/runlogs/run.go`, `internal/flow/runlogs/runlogs.go`
- Modify: `internal/commands/run/surface.go:51-85`
- Modify: `internal/schemas/global.schema.json`, `internal/commands/agents/assets/using-wtm.skill.md`, `README.md`
- Test: `internal/service/process/manager_test.go`, `internal/config/config_test.go`

**Interfaces:**
- Consumes: `proxy.Registry`, `proxy.NewServer` (Lot 5) ; `rules.RouteHost` (Lot 4).
- Produces:
  - `process.Request.RouteHost string`
  - `process.RouteSink` interface `{ Add(domain.ProxyRoute); Remove(host string) }`
  - `process.NewManagerWithRoutes(routes RouteSink) *Manager` — `NewManager()` reste et devient `NewManagerWithRoutes(nil)`, pour que les tests existants ne bougent pas
  - `process.StartParams.RouteHost string`, `process.DaemonParams.ProxyPort int`
  - `domain.GlobalConfig.Proxy domain.ProxyConfig`
  - `rules.ProxyPort(cfg domain.GlobalConfig) int` — `0` quand le proxy est désactivé
  - `runlogs.RunParams.Project string`, `runlogs.RunParams.ProxyPort int`

- [ ] **Step 1: Test de `rules.ProxyPort` (échec attendu)**

Dans `internal/rules/proxyroute_test.go` : absent → `domain.ProxyDefaultPort` ; `port = 5000` → `5000` ; `enabled = false` → `0` ; `enabled = true` sans port → le défaut.

Run: `go test ./internal/rules/ -run TestProxyPort -v` → FAIL.

- [ ] **Step 2: Implémenter `rules.ProxyPort`**

```go
// ProxyPort is the port the run proxy listens on, zero when it is switched off.
// Zero is what every caller reads to fall back to a job's own port, so the
// feature degrades to what it replaced instead of failing.
func ProxyPort(cfg domain.GlobalConfig) int {
	if cfg.Proxy.Enabled != nil && !*cfg.Proxy.Enabled {
		return 0
	}
	if cfg.Proxy.Port > 0 {
		return cfg.Proxy.Port
	}
	return domain.ProxyDefaultPort
}
```

Ajouter `Proxy ProxyConfig \`toml:"proxy"\`` à `domain.GlobalConfig`, et un cas à `internal/config/config_test.go` vérifiant qu'un `[proxy] port = 5000` du fichier global est relu.

Run: `go test ./internal/rules/ ./internal/config/ -run 'TestProxyPort|TestLoad' -v` → PASS.

- [ ] **Step 3: Le client résout la route, le daemon l'enregistre**

Dans `internal/service/process/protocol.go`, `Request` :

```go
	// RouteHost is the hostname the proxy serves this job under, resolved by the
	// client for the same reason LogDir and Env are: the daemon is global, and
	// only the client can ask git which worktree and which repository this is.
	RouteHost string `json:"route_host,omitempty"`
```

Dans `internal/service/process/manager.go` :

```go
// RouteSink is where a started job's route is published. The proxy implements
// it; nil means no proxy, which changes nothing about the job.
type RouteSink interface {
	Add(route domain.ProxyRoute)
	Remove(host string)
}
```

`Manager` gagne un champ `routes RouteSink` et `ManagedJob` un champ `RouteHost string`. `StartParams` gagne `RouteHost string`. Dans `startJob`, après avoir résolu les ports et enregistré le job, si `m.routes != nil && params.RouteHost != ""` et que le job déclare une `url` dont le port est résolu :

```go
	m.routes.Add(domain.ProxyRoute{
		Host:     params.RouteHost,
		Target:   fmt.Sprintf(domain.ProxyLoopbackFmt, ports[job.URL.Port]),
		Job:      job.Name,
		Worktree: params.Env[domain.EnvWorktree],
		Project:  params.Env[domain.EnvProject],
	})
```

`ProxyLoopbackFmt` est la constante déjà posée au Lot 5 : une adresse de loopback est la même chose des deux côtés, ne pas en créer une seconde. Retirer la route au même endroit que le job quitte `m.jobs` : dans `stopByKey` et dans la goroutine de reaping (`manager.go:318`).

- [ ] **Step 4: Test du câblage**

Ajouter à `internal/service/process/manager_test.go` : un `RouteSink` double qui enregistre les appels ; démarrer un job publiant une URL avec un `RouteHost` non vide → un `Add` avec le bon `Target` ; l'arrêter → un `Remove` du même hôte ; démarrer un job sans `url` → aucun appel.

Run: `go test ./internal/service/process/ -run TestManagerRoutes -race -v`
Expected: PASS.

- [ ] **Step 5: Le daemon possède le proxy**

Dans `internal/service/process/daemon.go`, `RunDaemon` : construire le Registry, le passer au Manager, et démarrer le serveur. **Un échec de bind ne remonte pas** :

```go
	registry := proxy.NewRegistry()
	d := &daemonServer{manager: NewManagerWithRoutes(registry), ...}

	if port := params.ProxyPort; port > 0 {
		server := proxy.NewServer(proxy.ServerParams{Port: port, Registry: registry})
		if err := server.Start(); err != nil {
			// A busy port costs the names, never the jobs.
			log.Printf("run proxy: %v — jobs keep their own ports", err)
		} else {
			defer server.Close()
		}
	}
```

`DaemonParams` gagne `ProxyPort int`, résolu par le client qui lance le daemon (`EnsureDaemon`) depuis la config globale. Vérifier comment `EnsureDaemon` fabrique sa ligne de commande et y ajouter le drapeau correspondant dans `internal/commands/daemon/`.

Le proxy meurt avec le daemon : c'est voulu, et c'est ce qui évite tout cycle de vie supplémentaire.

- [ ] **Step 6: Le flow calcule la forme nommée**

`runlogs.RunParams` gagne `Project string` et `ProxyPort int`. Dans `run.go`, avant de démarrer chaque job :

```go
	host := rules.RouteHost(rules.RouteHostParams{
		Job:      job,
		Worktree: params.Env[domain.EnvWorktree],
		Project:  params.Project,
	})
```

Passer `host` au démarrage (`daemon.go` le met dans `Request.RouteHost`) et l'utiliser pour l'événement :

```go
	URL: rules.JobURL(rules.JobURLParams{Job: job, Ports: result.Ports, Host: host, ProxyPort: params.ProxyPort}),
```

Dans `internal/commands/run/surface.go`, `openRunSeam` renseigne `Project: filepath.Base(params.ProjectDir)` et `ProxyPort: rules.ProxyPort(cfg.Global)` — `runSeamParams` gagne les deux champs, alimentés depuis `shared.LoadConfig`.

Faire la même chose dans `publishedJobs` (Lot 3), pour que `run url` rende la forme nommée et `--raw` la forme directe.

- [ ] **Step 7: Test bout en bout du flow**

Étendre `TestRunEmitsJobURL` (Lot 2) avec un second cas passant `Project: "myapp"` et `ProxyPort: 4000`, attendant `http://web.feat-auth.myapp.localhost:4000`, l'Env du test portant `WTM_WORKTREE=feat-auth`.

Run: `go test ./internal/flow/runlogs/ -run TestRunEmitsJobURL -race -v`
Expected: PASS.

- [ ] **Step 8: Docs**

- `internal/schemas/global.schema.json` : le bloc `proxy` (`port`, `enabled`).
- `using-wtm.skill.md` : la forme des URLs nommées, `[proxy]`, la limite Linux hors navigateur et le `--raw` qui est la réponse pour un agent.
- `README.md` : section Configuration, la table `[proxy]`.
- Run: `make docs`

- [ ] **Step 9: Valider et committer**

Run: `make test && make vet && make lint`

```bash
git add internal/service/process/ internal/service/proxy/ internal/domain/ internal/config/ internal/rules/ internal/flow/runlogs/ internal/commands/ internal/schemas/ README.md docs/
git commit -m "feat(run): le daemon sert les noms des jobs qu'il fait tourner"
```

---

## Lot 7 : l'avertissement `allowedDevOrigins`

**Files:**
- Create: `internal/rules/devorigins.go`, `internal/rules/devorigins_test.go`
- Create: `internal/service/detect/devorigins.go`
- Modify: `internal/flow/runlogs/runlogs.go`, `internal/flow/runlogs/run.go`
- Modify: `internal/output/runlogs.go`, `internal/domain/constants.go`

**Interfaces:**
- Consumes: `domain.JobConfig`, `runlogs.Event` (Lots 1-2).
- Produces:
  - `domain.DevOriginFix{Job, Config, Line string}`
  - `rules.NeedsDevOrigins(params rules.NeedsDevOriginsParams) bool`
  - `runlogs.Event.DevOrigins []domain.DevOriginFix`

- [ ] **Step 1: Test (échec attendu)**

`NeedsDevOriginsParams{Job domain.JobConfig; ConfigSource string}` — le contenu du fichier de config est lu par `service/detect`, jamais par `rules/`. `ConfigSource` vide veut dire « aucun `next.config.*` trouvé », donc rien à dire.

```go
package rules

import (
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
)

func TestNeedsDevOrigins(t *testing.T) {
	nextJob := domain.JobConfig{
		Name:  "web",
		Cmd:   "pnpm run dev --port ${PORT}",
		Ports: map[string]int{"PORT": 3000},
		URL:   &domain.JobURLConfig{Port: "PORT"},
	}

	tests := []struct {
		name   string
		job    domain.JobConfig
		source string
		want   bool
	}{
		{"next sans allowedDevOrigins", nextJob, "export default { reactStrictMode: true }\n", true},
		{"next avec allowedDevOrigins", nextJob, "export default { allowedDevOrigins: [\"*.localhost:4000\"] }\n", false},
		{"pas de next.config", nextJob, "", false},
		{"job qui ne publie rien", domain.JobConfig{Name: "db", Cmd: "docker compose up"}, "export default {}\n", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := NeedsDevOrigins(NeedsDevOriginsParams{Job: tt.job, ConfigSource: tt.source})
			if got != tt.want {
				t.Errorf("got %v, want %v", got, tt.want)
			}
		})
	}
}
```

Noter ce que ce tableau dit : la règle ne cherche **pas** à reconnaître Next dans le `cmd`. La présence d'un `next.config.*` dans le `cwd` du job est le signal, et c'est `service/detect` qui la constate — `rules/` n'a pas le droit de toucher au disque, et deviner un framework depuis une ligne de commande est précisément ce que l'epic a écarté.

Un job Vite n'a donc rien à déclencher : il n'a pas de `next.config.*`. C'est aussi la bonne réponse de fond, Vite autorisant `.localhost` par défaut.

- [ ] **Step 2: Lancer** → FAIL.

- [ ] **Step 3: Implémenter la règle et la détection**

`rules.NeedsDevOrigins` est pure : elle décide à partir du `cmd` et du contenu qu'on lui tend. `service/detect/devorigins.go` va chercher `next.config.{js,mjs,ts}` dans le `cwd` du job et rend son contenu, chaîne vide si absent.

Le message est une constante :

```go
	// DevOriginsFixFmt names the one line a Next project needs before a
	// subdomain of .localhost may reach its dev assets.
	DevOriginsFixFmt = "%s: add allowedDevOrigins: [\"*.%s:%d\"] to %s — Next blocks dev requests from other hosts"
```

- [ ] **Step 4: Lancer** → PASS.

- [ ] **Step 5: Émettre et rendre**

`Event` gagne `DevOrigins []domain.DevOriginFix`, émis sur `PhaseStarted`. `RunPrinter` et la vue les rendent comme un avertissement, exactement comme `probed` rend `rules.PortProbeLines` — suivre `internal/output/runlogs.go:66-80`.

- [ ] **Step 6: Valider et committer**

Run: `make test && make vet && make lint`

```bash
git add internal/rules/devorigins.go internal/rules/devorigins_test.go internal/service/detect/devorigins.go internal/flow/runlogs/ internal/output/runlogs.go internal/domain/constants.go
git commit -m "feat(run): dire à un projet Next la ligne qui lui manque pour répondre sous son nom"
```

---

## Clôture

- [ ] `make test && make vet && make lint && make docs` — l'arbre `docs/` régénéré ne doit produire aucune différence non commitée.
- [ ] Invoquer le subagent **build-validator**.
- [ ] Relire `using-wtm.skill.md` d'un bloc : les deux commandes, le champ `url`, `[proxy]`, la limite Linux, `--raw`.
- [ ] Vérifier à la main dans `~/Documents/Dev/monorepo-exemple-wtm` (voir « Terrain d'essai ») : `wtm run up all` depuis `main` **et** depuis le worktree `test`, les URLs des deux `web-dev` ouvertes côte à côte, une session ouverte dans l'un ne doit pas déconnecter l'autre. **C'est le test d'acceptation de la feature** — il ne s'automatise pas ici.
- [ ] Dans le même dépôt, confirmer que `web-dev` (Vite 8) répond sous son nom **sans** avoir touché à `vite.config.ts`. Si ce n'est pas le cas, l'hypothèse « Vite autorise `.localhost` par défaut » de la spec est fausse et il faut la corriger avant de documenter quoi que ce soit.

## Hors périmètre

**F3 — le port 80** (`wtm run proxy install` : règle pf `rdr` sur `lo0` sous macOS, `setcap cap_net_bind_service` sous Linux). Opt-in, un `sudo` une fois, les URLs perdent leur suffixe. La forme des noms ne change pas, donc rien de ce plan n'est à réécrire.

**Le dashboard** (`wtm ui`) : dépend de R2 ([LUC-193](https://linear.app/lucaspcq/issue/LUC-193)), qui n'est pas fait. L'URL passant par `runlogs.Event`, elle arrivera sans changement ici.

**La route d'un service détaché survivant au daemon** : c'est R1 ([LUC-194](https://linear.app/lucaspcq/issue/LUC-194)), qui le corrige pour les jobs et pour les routes d'un coup.
