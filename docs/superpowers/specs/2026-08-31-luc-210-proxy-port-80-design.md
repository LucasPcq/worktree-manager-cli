# LUC-210 — [F2] Des URLs nommées sans port : servir le proxy sur le 80

Statut : livré. Le §3 a été réécrit après implémentation — le mécanisme `pf` du design initial ne fonctionne pas sur macOS actuel, et a été remplacé par l'activation de socket launchd.

## Objectif

`http://web.feat-x.monorepo.localhost/` au lieu de `http://web.feat-x.monorepo.localhost:10080/`. Le port du proxy est un détail de transport ; tant qu'il figure dans l'URL, celle-ci n'est ni mémorisable ni collable dans un `.env` sans que le port fuite dans le projet.

Le ticket s'arrête à HTTP. HTTPS (et donc la CA locale) est explicitement hors périmètre.

## Périmètre

* **macOS** : implémentation complète (LaunchAgent + activation de socket launchd, modèle puma-dev v0.3+).
* **Linux et le reste** : l'interface de service existe, l'implémentation renvoie « non supporté sur cette plateforme » et les URLs gardent leur port. Ticket successeur.
* **Le cas « pas installé » est le chemin par défaut**, pas un cas dégradé : c'est l'état de toute nouvelle installation et de tout CI, et c'est exactement le comportement actuel, non modifié.

## Ce qui existe déjà (post-LUC-104 et LUC-209)

* `internal/service/proxy` porte la table de routage (`Registry`) et le reverse proxy (`Server`), qui route sur l'en-tête `Host`.
* `Server.Start()` scanne `[ProxyDefaultPort, +ProxyPortScanSpan[` et retient le premier port libre ; le daemon publie le port réellement bindé dans chaque réponse (`Response.ProxyPort`).
* `rules.JobURL` est le point unique de formatage d'une URL de job ; `run url` lit le port réellement servi via `servedProxyPort()` et retombe sur l'adresse directe quand le proxy est éteint.
* Le daemon est **non privilégié** : il est spawné à la demande par le client, possède une socket Unix et s'éteint sur idle-timeout. Rien de ce design ne change ce cycle de vie.

## 1. Le vocabulaire : port de bind vs port public

Aujourd'hui un seul nombre joue les deux rôles. Ce ticket les sépare.

* **Port de bind** — ce que `proxy.Server` écoute réellement : 10080, ou 10081…10095 après le scan de LUC-209. Non privilégié. C'est ce que `[proxy] port` désigne, sens inchangé.
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
* un forwarder qui viserait un port que le daemon a quitté → l'en-tête ne correspond pas au port réellement bindé → port public = port de bind. (Le forwarder livré résout le port auprès du daemon, donc ce cas ne survient plus dans le fonctionnement nominal.)

Quand rien n'écoute sur 80, la connexion est refusée immédiatement : coût nul dans le cas courant.

### L'ordre de résolution

La sonde est la vérité terrain mais elle exige qu'un daemon tourne. Or LUC-212 écrira des `.env` **à la création d'un worktree**, moment où il n'y a souvent rien à sonder : une sonde seule figerait `:10080` dans les `.env` alors que la redirection est installée. D'où trois étages, dans cet ordre :

1. **Un daemon tourne** → son `Response.ProxyPublicPort`, sondé **à chaque réponse** et jamais mis en cache : une install ou une désinstallation a lieu pendant la vie du daemon, et une valeur figée au démarrage annoncerait un nom que plus rien ne sert.
2. **Pas de daemon** → **état déclaré** par l'installation : la présence de `~/Library/LaunchAgents/dev.wtm.proxy.plist` → 80.
3. **Sinon** → port de bind.

L'étage 2 peut mentir (un agent présent mais déchargé à la main). La sonde le corrige dès qu'un daemon tourne, et `wtm run proxy status` affiche les deux côte à côte en signalant explicitement une divergence.

### Où ça se branche

* `internal/service/process/daemon.go` : `daemonServer.publicPort()` sonde à chaque réponse ; `proxyPort` reste le port réellement bindé.
* `internal/service/process/protocol.go` : `Response` gagne `ProxyPublicPort int` (`json:"proxy_public_port,omitempty"`). `ProxyPort` garde son sens de port de bind.
* `internal/commands/run/url.go` : `servedProxyPort` devient `publicProxyPort`, implémentant les trois étages. Les autres appelants de `rules.JobURL` (`run open`, `run ps`, la TUI, `flow/runlogs`) passent par cette même résolution.

## 3. Le service de redirection

> **Révisé après implémentation.** Le design initial reposait sur une ancre `pf` et un `rdr` sur `lo0`, le modèle Pow. Vérifié sur une machine réelle : `pf` activé, ancre enregistrée, règle chargée — et zéro interception, sur deux formes de règle. macOS n'évalue pas un `rdr` sur `lo0` pour du trafic que la machine s'adresse à elle-même. puma-dev a fait la même découverte et a migré vers launchd en v0.3 ; on reprend son mécanisme.

Une interface unique dans `internal/service/proxy`, une implémentation par plateforme derrière un build tag.

```go
type Redirector interface {
    Plan() (Plan, error)  // ce qui sera écrit — aucun effet de bord
    Apply() error
    Remove() error        // l'inverse exact, idempotent
    Inspect() domain.ProxyStatus
}
```

### `redirect_darwin.go` — un LaunchAgent, aucun privilège

Un seul artefact : `~/Library/LaunchAgents/dev.wtm.proxy.plist`, déclarant une clé `Sockets` sur `127.0.0.1:80` et `[::1]:80`. **launchd bind la socket privilégiée pour le compte de l'agent**, et la passe à un process utilisateur ordinaire. `Apply` écrit le plist et lance `launchctl load` ; `Remove` fait `launchctl unload` et supprime le fichier. **Aucun `sudo`, aucun fichier système, aucune règle de pare-feu.**

La socket reste sur la loopback — jamais `0.0.0.0`, que puma-dev utilise et qui publierait chaque worktree sur le réseau.

Le process lancé est `wtm proxy-forward`, une sous-commande cachée qui récupère les descripteurs via `launch_activate_socket` et relaie les octets vers le proxy. Le relais est volontairement aveugle au HTTP : l'en-tête `Host` est l'affaire du proxy, et un relais d'octets fait fonctionner websockets et streaming sans effort.

**Le plist ne contient aucun port.** Le forwarder demande au daemon, par la socket Unix qu'il a déjà, sur quel port le proxy sert réellement, et mémorise la réponse une seconde. Un repli sur port libre (LUC-209) ou un redémarrage sur un autre port est donc suivi sans réinstallation.

### `launch_activate_socket` sans cgo

`.goreleaser.yaml` impose `CGO_ENABLED=0` et compile darwin en cross ; puma-dev appelle cette fonction en cgo, ce qui nous coûterait le pipeline. On la joint donc comme la stdlib joint libc sur darwin : `//go:cgo_import_dynamic` plus un trampoline assembleur, adapté de `bored-engineer/go-launchd` (MIT). Le symbole et `libxpc.dylib` restent visibles dans les load commands — ce n'est pas de la résolution `dlopen`/`dlsym`, ce qui compte pour ne pas ressembler à la dissimulation d'API que traquent les EDR.

### `redirect_other.go` (`//go:build !darwin`)

Renvoie `domain.ErrProxyRedirectUnsupported`, et un `Status{Supported: false}` sur `Inspect()`.

### Surface EDR

Écrire dans `~/Library/LaunchAgents/` est MITRE ATT&CK T1543.001, surveillé par SentinelOne, CrowdStrike et Jamf Protect. C'est inhérent à la fonctionnalité — servir le 80 après un reboot, c'est être persistant — et nettement plus discret que le chemin `pf` écarté, qui combinait un LaunchDaemon root, une modification de `/etc/pf.conf` et une règle de pare-feu. Contrepoids : loopback seulement, nommage transparent, opt-in strict, désinstallation complète, et la signature/notarisation des releases suivie par LUC-213.

## 4. La surface CLI

Nouveau groupe `wtm run proxy`, dans `internal/commands/run/proxycmd/` (même forme que `jobcmd/` et `profilecmd/`). Sans sous-commande, il affiche `status`.

* **`wtm run proxy status`** — port de bind configuré et réellement bindé, redirection déclarée et son mécanisme, résultat de la sonde, une URL d'exemple, et le signalement d'une divergence entre déclaré et sondé. `--output json`. C'est le point de découverte principal.
* **`wtm run proxy install`** — affiche le récap (le chemin du plist, ce qu'il change, la commande `launchctl`), demande confirmation, puis installe. `--dry-run` imprime le fichier en entier sans rien écrire, et n'exige aucun terminal puisqu'il n'écrit rien. Hors TTY ou en `--output json`, `--yes` est requis — la confirmation est le seul axe, il n'y a plus de privilège à protéger.
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
