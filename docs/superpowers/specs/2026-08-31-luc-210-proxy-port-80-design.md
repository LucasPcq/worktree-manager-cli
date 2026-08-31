# LUC-210 — [F2] Des URLs nommées sans port : servir le proxy sur le 80

Statut : design validé, non commité (relecture locale avant plan d'implémentation).

## Objectif

`http://web.feat-x.monorepo.localhost/` au lieu de `http://web.feat-x.monorepo.localhost:4000/`. Le port du proxy est un détail de transport ; tant qu'il figure dans l'URL, celle-ci n'est ni mémorisable ni collable dans un `.env` sans que le port fuite dans le projet.

Le ticket s'arrête à HTTP. HTTPS (et donc la CA locale) est explicitement hors périmètre.

## Périmètre

* **macOS** : implémentation complète (ancre `pf` + `LaunchDaemon`, modèle Pow/Valet).
* **Linux et le reste** : l'interface de service existe, l'implémentation renvoie « non supporté sur cette plateforme » et les URLs gardent leur port. Ticket successeur.
* **Le cas « pas installé » est le chemin par défaut**, pas un cas dégradé : c'est l'état de toute nouvelle installation et de tout CI, et c'est exactement le comportement actuel, non modifié.

## Ce qui existe déjà (post-LUC-104 et LUC-209)

* `internal/service/proxy` porte la table de routage (`Registry`) et le reverse proxy (`Server`), qui route sur l'en-tête `Host`.
* `Server.Start()` scanne `[ProxyDefaultPort, +ProxyPortScanSpan[` et retient le premier port libre ; le daemon publie le port réellement bindé dans chaque réponse (`Response.ProxyPort`).
* `rules.JobURL` est le point unique de formatage d'une URL de job ; `run url` lit le port réellement servi via `servedProxyPort()` et retombe sur l'adresse directe quand le proxy est éteint.
* Le daemon est **non privilégié** : il est spawné à la demande par le client, possède une socket Unix et s'éteint sur idle-timeout. Rien de ce design ne change ce cycle de vie.

## 1. Le vocabulaire : port de bind vs port public

Aujourd'hui un seul nombre joue les deux rôles. Ce ticket les sépare.

* **Port de bind** — ce que `proxy.Server` écoute réellement : 4000, ou 4001…4015 après le scan de LUC-209. Non privilégié. C'est ce que `[proxy] port` désigne, sens inchangé.
* **Port public** — ce qui apparaît dans l'URL. Vaut `80` si et seulement si la redirection est réellement en place ; sinon, le port de bind.

**Aucune nouvelle clé de configuration.** Le port public n'est pas une préférence mais un fait machine : il découle de l'installation, jamais d'une valeur écrite dans `config.toml`. Deux clés qui peuvent se contredire seraient une incohérence à arbitrer ; il n'y en a qu'une.

Conséquences dans `internal/rules` :

* Nouvelle constante `domain.ProxyPrivilegedPort = 80` et `domain.ProxyOriginFmt = "http://%s"`.
* Nouvelle fonction `rules.JobOrigin(JobOriginParams) string` : rend `http://<host>` quand le port public vaut 80, `http://<host>:<port>` sinon, et l'origine directe `http://localhost:<port>` quand il n'y a pas de proxy.
* `rules.JobURL` s'écrit par-dessus `JobOrigin` et garde sa signature côté appelants (le champ `ProxyPort` devient le port **public**).

`JobOrigin` est le point d'entrée que LUC-212 consommera pour réécrire les `.env` en origines : une seule fonction décide de la forme d'une origine, pour `run url`, `run open`, la TUI et la réécriture d'env.

## 2. La sonde, et la résolution du port public

### La sonde

`internal/service/proxy/probe.go` :

```go
type ProbeParams struct {
    Port    int           // le port sondé ; 80 en production, arbitraire en test
    Timeout time.Duration // ProxyProbeTimeout, ~150ms
}

// Probe dit quel port de bind répond derrière Port, zéro quand ce n'est pas nous.
func Probe(params ProbeParams) int
```

Elle fait un `GET http://127.0.0.1:<Port>/` avec `Host: probe.wtm.localhost` (`domain.ProxyProbeHost`). Côté serveur, `Server.route` intercepte ce host **avant** de consulter le registre et répond `204` avec l'en-tête `X-Wtm-Proxy: <port de bind>` (`domain.ProxyProbeHeader`).

La sonde ne dit donc pas « quelque chose écoute sur 80 » mais « **c'est notre proxy, celui qui tourne actuellement**, qui répond sur 80 ». Deux cas que ça règle sans code spécifique :

* un autre serveur occupe le 80 → pas d'en-tête → port public = port de bind ;
* la règle `pf` pointe sur 4000 mais le daemon s'est replié sur 4001 (scan de LUC-209) → l'en-tête ne correspond pas au port réellement bindé → port public = port de bind.

Quand rien n'écoute sur 80, la connexion est refusée immédiatement : coût nul dans le cas courant.

### L'ordre de résolution

La sonde est la vérité terrain mais elle exige qu'un daemon tourne. Or LUC-212 écrira des `.env` **à la création d'un worktree**, moment où il n'y a souvent rien à sonder : une sonde seule figerait `:4000` dans les `.env` alors que la redirection est installée. D'où trois étages, dans cet ordre :

1. **Un daemon tourne** → son `Response.ProxyPublicPort`, issu de la sonde faite au démarrage du daemon. Vérité terrain.
2. **Pas de daemon** → **état déclaré** par l'installation, lu sans `sudo` (présence de l'ancre, du bloc balisé dans `/etc/pf.conf`, du plist — tous lisibles par tous) → 80.
3. **Sinon** → port de bind.

L'étage 2 peut mentir (un `pfctl -F` manuel, une mise à jour macOS qui réécrit `/etc/pf.conf`). La sonde le corrige dès qu'un daemon tourne, et `wtm run proxy status` affiche les deux côte à côte en signalant explicitement une divergence.

### Où ça se branche

* `internal/service/process/daemon.go` : après `server.Start()`, une seule sonde ; `daemonServer` porte désormais `proxyPort` (bind) **et** `proxyPublicPort`.
* `internal/service/process/protocol.go` : `Response` gagne `ProxyPublicPort int` (`json:"proxy_public_port,omitempty"`). `ProxyPort` garde son sens de port de bind.
* `internal/commands/run/url.go` : `servedProxyPort` devient `publicProxyPort`, implémentant les trois étages. Les autres appelants de `rules.JobURL` (`run open`, `run ps`, la TUI, `flow/runlogs`) passent par cette même résolution.

## 3. Le service privilégié

Une interface unique dans `internal/service/proxy`, une implémentation par plateforme derrière un build tag.

```go
type Redirector interface {
    Plan(PlanParams) Plan   // ce qui sera écrit — aucun effet de bord
    Apply(PlanParams) error // le sudo unique
    Remove() error          // l'inverse exact, idempotent
    Inspect() Status        // l'état déclaré, sans sudo et sans daemon
}
```

`Plan` porte la liste des fichiers avec leur contenu rendu, ce qui donne à la fois le récap affiché à l'utilisateur et la matière des tests. `Status` porte `Installed bool`, `Mechanism string`, `BindPort int`, `Supported bool`.

### `redirect_darwin.go`

Trois artefacts, le modèle Pow puis Valet :

1. `/etc/pf.anchors/wtm` — `rdr pass on lo0 inet proto tcp from any to 127.0.0.1 port 80 -> 127.0.0.1 port <bind>`
2. `/etc/pf.conf` — un `rdr-anchor "wtm"` inséré dans un **bloc balisé** (`# >>> wtm >>>` … `# <<< wtm <<<`), à sa place dans l'ordre imposé par `pf` : les `rdr-anchor` précèdent les règles de filtre.
3. `/Library/LaunchDaemons/dev.wtm.proxy.plist` — `RunAtLoad`, recharge `pfctl -E -f /etc/pf.conf` au démarrage de la machine.

`Apply` écrit les trois fichiers dans un répertoire temporaire non privilégié, puis lance **un seul** `sudo /bin/sh -c …` : `install` des fichiers, `launchctl bootstrap system`, `pfctl -f /etc/pf.conf`. `Remove` retire le bloc de `/etc/pf.conf`, supprime l'ancre et le plist, `launchctl bootout`, recharge `pf` — et ne fait rien sans erreur si l'installation est absente.

La règle `pf` fige le port de bind. Si `[proxy] port` change après coup, la redirection pointe à côté : la sonde le détecte (l'en-tête ne correspond plus), `status` le dit en toutes lettres, et un `install` rejoué réécrit l'ancre. Aucune réparation automatique — une écriture privilégiée ne se déclenche jamais toute seule.

La manipulation du bloc balisé dans `/etc/pf.conf` (insertion au bon endroit, retrait exact, idempotence sur double install, retrait sur fichier vierge) est **une fonction pure** et vit donc dans `internal/rules/pfconf.go`, testable sans root ni fichier système.

### `redirect_other.go` (`//go:build !darwin`)

Renvoie `domain.ErrProxyRedirectUnsupported` sur `Apply`/`Remove`, et un `Status{Supported: false}` sur `Inspect`. `wtm run proxy status` y dit ce qui est vrai — les URLs portent leur port, et cette plateforme n'a pas encore d'implémentation — sans faire croire à une panne.

## 4. La surface CLI

Nouveau groupe `wtm run proxy`, dans `internal/commands/run/proxycmd/` (même forme que `jobcmd/` et `profilecmd/`). Sans sous-commande, il affiche `status`.

* **`wtm run proxy status`** — port de bind configuré et réellement bindé, redirection déclarée et son mécanisme, résultat de la sonde, une URL d'exemple, et le signalement d'une divergence entre déclaré et sondé. `--output json`. C'est le point de découverte principal.
* **`wtm run proxy install`** — affiche le récap exact (chaque chemin et son contenu), demande confirmation, puis lance `sudo`. `--yes` saute la confirmation mais **pas** le `sudo`. Hors TTY ou en `--output json`, refus explicite nommant la contrainte : un `sudo` ne peut pas être non-attendu.
* **`wtm run proxy uninstall`** — même récap, même confirmation, réversible à l'identique.

Ces commandes **ne passent pas par `internal/flow/`** : la règle vise les commandes qui mutent un worktree, or celle-ci mute la machine et n'en touche aucun. Elles respectent en revanche l'axe `--yes` de la convention des commandes mutantes. Pas de `--force` : il n'y a aucun refus de sécurité à lever, seulement une confirmation.

## 5. Le mode dégradé — et le seul endroit qui en parle

Sans installation, rien ne change : le proxy sert sur 4000, `run url` annonce `:4000`, les notices existantes (`ProxyUnavailable`, `ProxyMoved`, `ProxyPortCollision`) restent telles quelles. `run up`, `run open` et `run url` ne gagnent **aucun** message.

L'unique exception est l'épilogue de `wtm run init`, à côté du callout `ProxyPortCollisionTitle` — la section qui dit déjà « ce que le run n'a pas pu faire sans le lecteur ». Un callout y apparaît sous **deux** conditions cumulées : au moins un job déclare une `url`, et la redirection n'est pas installée (étage 2 de la résolution, donc sans `sudo` ni daemon).

```
Named URLs carry a port
  Jobs publishing a url answer on http://<job>.<worktree>.<repo>.localhost:4000
  `wtm run proxy install` redirects port 80 so the port disappears from the URL
```

Sur une plateforme non supportée, le callout ne mentionne pas `install`. `run init` **ne demande rien et n'exécute aucun `sudo`** : la découverte a lieu là, l'action reste dans sa propre commande — un `run init --non-interactive` ou en CI est inchangé. Les lignes du callout sont produites par une fonction pure `rules.ProxyInstallHintLines`.

## 6. Tests

* `internal/rules` — `JobOrigin`/`JobURL` avec port public 80 (aucun port dans l'URL) et 4000 (port présent) ; round-trip du bloc `pf.conf` : insertion, double insertion, retrait, retrait sur fichier vierge, fichier modifié à la main autour du bloc ; `ProxyInstallHintLines` sous ses deux conditions et sur plateforme non supportée.
* `internal/service/proxy` — la route sentinelle répond bien son en-tête et ne consulte pas le registre ; `Probe` contre un `httptest` sur un port arbitraire (le port sondé est un paramètre : **les tests ne touchent jamais au 80**) ; mismatch d'en-tête → zéro ; hôte sentinelle et hôte inconnu ne se confondent pas.
* `internal/service/proxy` (darwin) — le rendu de `Plan` (contenu de l'ancre, du plist, du bloc) est comparé à des fixtures. `Apply`/`Remove` ne sont pas exécutés en test.
* `internal/commands/run/proxycmd` — forme du JSON de `status` ; refus de `install` hors TTY et en JSON ; les trois étages de la résolution du port public.
* **Aucun test n'exécute `sudo`, ne lit un fichier système, ni ne bind un port privilégié.**

## 7. Documentation

`make docs` pour régénérer `docs/`, la ligne du tableau de commandes du README pour le nouveau groupe `run proxy`, et `internal/commands/agents/assets/using-wtm.skill.md` — les nouvelles commandes, et surtout le fait qu'une URL de job peut désormais ne porter aucun port, ce qu'un agent qui parse `run url` doit savoir.

## 8. Ce que ce ticket livre à LUC-212

* `rules.JobOrigin` — le point unique de production d'une origine.
* Un port public résoluble **sans daemon en marche** (étage 2), ce dont dépend l'écriture des `.env` à la création d'un worktree.
* Un port public **stable par construction** quand la redirection est installée : le 80 ne se déplace pas, ce qui fait disparaître le risque du `.env` figé sur un port dont le proxy s'est replié.

## Hors périmètre

HTTPS et la CA locale ; l'implémentation Linux ; le réglage `[run] addressing` et la réécriture des `.env` en origines (LUC-212) ; toute modification du scan de port de LUC-209.
