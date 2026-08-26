# LUC-104 — Couche URLs nommées par worktree

Design validé le 2026-08-27. Ticket : [LUC-104](https://linear.app/lucaspcq/issue/LUC-104), brique **F** de l'epic [LUC-55](https://linear.app/lucaspcq/issue/LUC-55) (isolation des ports et des services entre worktrees).

## Le problème, recadré

Le ticket d'origine range F en **P3 (découvrabilité)**, « optionnel, hors chemin critique », et lui donne pour objectif « ne plus mémoriser des ports ». Le cadrage a montré que c'est le bénéfice le plus faible de la feature et pas celui qui la justifie.

Le vrai manque est ailleurs : **les cookies ignorent le port**. `localhost:3000` et `localhost:3010` partagent le même jar. Deux worktrees ouverts côte à côte dans le même navigateur se volent mutuellement leur session — on se connecte sur la branche A, on recharge la branche B, on est déconnecté ou pire, connecté avec l'état de l'autre. A isole les ressources Docker, B les bindings, G les `.env` ; **le navigateur reste le seul endroit où deux worktrees se marchent encore dessus**, et seul un nom d'hôte distinct le corrige.

F n'est donc pas du confort de saisie : c'est la dernière brique fonctionnelle d'isolation. La découvrabilité (« quelle URL pour ce worktree ? ») reste un objectif, mais elle est livrable séparément et bien moins chère.

## Ce que le socle offre déjà

Trois faits du code, postérieurs à la rédaction du ticket, changent l'arbitrage.

**La table de routage existe déjà.** `ManagedJob` (`internal/service/process/manager.go:59`) porte `Config` (donc `Ports`), `Env` (donc `WTM_WORKTREE`, `WTM_PORT_OFFSET`) et `WorkDir` ; la map `Manager.jobs` est clé `jobKey(name, workDir)`, donc worktree-qualifiée ; le socket est **global** (`~/.config/wtm/wtm.sock`), pas par dépôt. Le daemon connaît déjà, à l'échelle de la machine, quel job de quel worktree écoute sur quel port résolu. Un proxy n'a rien à découvrir : il projette `m.jobs`.

**Les ports sont déterministes et déclarés.** `rules.JobPorts` calcule `base + ordinal × block`, sans sonde ni réallocation (décisions actées de l'epic). Une route est calculable avant même que le job tourne.

**Le diagnostic de port existe.** `rules.DiagnosePortProbes` et `PortsToDial` savent déjà dire qu'un port est silencieux, et qu'un job bind son port de base au lieu du port décalé.

Ce qui manque, c'est uniquement la surface : `grep localhost --include=*.go internal` ne rend aucun résultat hors tests. Le seul rendu de port est `rules.LabelWithPorts` → `web (PORT=3010)`. Il n'existe ni `run open`, ni `run url`.

## Corrections apportées au ticket d'origine

Le ticket propose deux voies (lib tierce `portless`, ou proxy maison) et chiffre mal les deux.

| Affirmation du ticket | Constat |
| --- | --- |
| « Proxy maison : gros chantier (WS passthrough, gestion des routes/PID) » | Le WS passthrough est gratuit : `httputil.ReverseProxy` gère l'upgrade HTTP/1.1 → 101 nativement. La gestion des routes et du PID disparaît en se greffant sur le daemon, qui tient déjà l'état et le cycle de vie. |
| « portless ne couvre pas Docker » | Faux : `portless alias <name> <port>` enregistre des routes statiques, explicitement pour les conteneurs. |
| « intégration légère : sucre `[run] proxy` qui préfixe les `cmd` » | Incompatible avec B : `portless <name> <cmd>` assigne lui-même un port éphémère 4000-4999 via `PORT`, ce qui détruit le déterminisme et rejoue la « réallocation automatique » déjà écartée par l'epic. Seule l'intégration par `alias` serait compatible — et réduit alors portless à un serveur de routes, rôle que le daemon tient déjà. |

**Le vrai coût, que le ticket ne nomme pas, est le port privilégié.** Un nom d'hôte ne transporte pas de port : `http://x.localhost` vise le port 80, dont le binding demande root sur macOS comme sous Linux. C'est là que portless « auto-elevates with sudo » et installe une CA locale dans le trust store. Le proxy en lui-même est petit ; c'est l'accès au port 80 qui est cher.

## Décisions

### Le moyen : proxy maison dans le daemon, HTTP-only, sur un port haut

Écarté : **portless** (dépendance Node, binaire pré-1.0, sudo et CA non maîtrisés, conflit avec le déterminisme de B, pour une fonction que le daemon assure déjà à moitié) ; **générer un Caddyfile** (zéro code mais on ne livre pas la feature, l'utilisateur gère le process).

Le proxy écoute sur un **port haut unique et fixe pour toute la machine**, par défaut `4000`. Ce port apparaît dans l'URL. C'est ce qui le distingue des ports qu'on cherche à ne plus retenir : `3010`, `4010`, `5010` changent à chaque worktree et à chaque service, `:4000` s'apprend une fois. Avec `wtm run open`, on ne le tape de toute façon jamais.

Le port 80 reste atteignable plus tard par une commande opt-in (F3), qui ne change pas la forme des noms : les URLs sont les mêmes, moins le suffixe.

### Pas de HTTPS, pas de CA

`http://*.localhost` est un *secure context* au sens de la spec W3C « potentially trustworthy » : `crypto.subtle`, les service workers et WebAuthn fonctionnent en clair. La seule justification du HTTPS local tombe, et avec elle la CA à installer dans le trust store.

### Le schéma de nommage

```
<host-du-job>.<worktree>.<projet>.localhost:<port-proxy>
       |            |         |
       |            |         +-- slug du dépôt, même source que COMPOSE_PROJECT_NAME
       |            +------------ slug de branche, = WTM_WORKTREE
       +------------------------- nom du job, surchargeable via url.host
```

**L'ordre `<job>.<worktree>` est imposé par les cookies.** Un cookie posé sur `.feat-auth.myapp.localhost` est partagé entre les jobs du *même* worktree — ce qu'on veut — et invisible aux autres. L'ordre inverse (`feat-auth.web.localhost`) ferait fuiter un cookie de `.web.localhost` d'un worktree à l'autre, c'est-à-dire exactement le bug qu'on répare.

**Le segment projet est toujours présent.** Deux dépôts ouverts sur `main` avec chacun un job `web` se disputeraient `web.main.localhost` — ce n'est pas un cas tordu mais le cas courant, et le daemon étant global il devrait arbitrer. Avec le segment, la collision est structurellement impossible. Il coûte un mot qu'on ne tape jamais.

**Le profil n'apparaît pas dans l'URL.** Un job peut appartenir à plusieurs profils : si le profil était un segment DNS, l'URL d'un job dépendrait de la façon dont on l'a démarré, donc changerait entre `run up app-1` et `run up all`. Une URL instable ne se bookmarke pas et casse les redirect URIs, soit précisément ce qu'on cherchait à gagner. Le profil décide *quoi démarrer*, pas *comment ça s'appelle*.

**Il n'y a pas de « job principal ».** Un `main = true` désignant l'URL nue du worktree ne survit pas au monorepo à deux apps : il n'y a pas un front, il y en a deux, et arbitrer lequel prend `feat-auth.localhost` est une question sans bonne réponse. Ce qui le remplace : `run up <profil>` liste les URLs des jobs qu'il vient de démarrer. La découvrabilité vient de la sortie de la commande, pas d'une convention de nommage.

**La hiérarchie intermédiaire est déclarée, pas imposée.** `rules.jobs_builder.go:95` nomme déjà les jobs d'un monorepo `<pkg>-<script>`, donc le nom du job porte déjà l'app. Qui veut regrouper explicitement écrit `url.host = "web.app-1"` : les jobs de app-1 partagent alors le parent `.app-1.feat-auth.myapp.localhost`, donc un jar commun, isolé de app-2. Deux apps qui ne partagent pas de session en prod n'en partagent pas en dev.

## Surface de configuration

### `run.toml` — une clé sur `[[job]]`

```toml
[[job]]
name  = "app1-web"
kind  = "service"
cmd   = "pnpm run dev --port ${PORT}"
cwd   = "apps/web"
ports = { PORT = 3000 }
url   = { port = "PORT" }

[[job]]
name  = "app1-api"
kind  = "service"
cmd   = "pnpm run dev --port ${PORT}"
cwd   = "apps/api"
ports = { PORT = 4000 }
url   = { port = "PORT", host = "api.app-1" }

[[job]]
name  = "db"
kind  = "service"
cmd   = "docker compose up"
ports = { PG_PORT = 5432 }
# pas d'url : pas de nom, joignable par son port comme aujourd'hui
```

Une table inline `url`, nommée d'après ce qu'elle produit, groupant les deux clés liées — et se lisant comme le `ports = { … }` juste au-dessus. `url.port` désigne **une clé de `ports`**, jamais un numéro : le port réel dépend du worktree, seule la déclaration est stable. `url.host` est facultatif et vaut le nom du job par défaut.

Les profils ne changent pas. Un job sans `url` se comporte exactement comme aujourd'hui.

### Du nom au label DNS

Un nom de job n'est pas forcément un label DNS valide. `rules.jobs_builder.go` compose `<PkgName>-<script>` (le scope npm est déjà retiré par `detect`), mais `run job add --name` accepte n'importe quoi, et `rules.Validate` ne contrôle aujourd'hui que l'unicité des noms, pas leur alphabet. `rules.WorktreeSlug` ne suffit pas non plus : il conserve les `_`, ne coupe pas les `-` en fin de chaîne et ne borne pas la longueur.

D'où une règle pure dédiée, `rules.HostLabel(s)` : minuscules, tout ce qui n'est pas `[a-z0-9-]` devient `-`, séquences de `-` réduites, `-` de tête et de queue coupés, tronqué à 63 caractères, repli documenté si la chaîne se vide. Elle s'applique aux trois segments — job, worktree, projet.

L'asymétrie est volontaire : **un segment dérivé est slugifié, un segment saisi est validé**. `url.host` étant écrit à la main, une valeur qui n'est pas déjà une suite de labels valides est une **erreur** au chargement, pas une chaîne à corriger en silence — sinon l'URL affichée ne serait pas celle que l'utilisateur a écrite.

Conséquence à connaître : pour une branche contenant un `_`, le segment d'hôte diffère de `WTM_WORKTREE` et du suffixe de `COMPOSE_PROJECT_NAME`. Les deux notations restent cohérentes pour l'immense majorité des noms, mais ce ne sont pas strictement les mêmes fonctions.

Validation au chargement, aux côtés de `rules.ValidateRunPorts` dans `config.LoadRun` :

- `url.port` doit nommer un port déclaré par ce job ;
- `url.host` doit être une suite de labels DNS valides ;
- deux jobs ne peuvent pas revendiquer le même hôte — c'est le cas qui compte vraiment, sans lui le proxy arbitre silencieusement et route la moitié du trafic ailleurs.

### `~/.config/wtm/config.toml` — le port du proxy

```toml
[proxy]
port = 4000
enabled = true
```

Le port va dans la config **utilisateur**, pas dans `run.toml` : c'est une propriété de la machine, pas du dépôt. Le daemon est global et sert tous les dépôts à la fois ; un port par projet voudrait dire N proxies et rétablirait la mémorisation qu'on supprime.

## Architecture

```
domain/          JobConfig.URL (JobURL{Port, Host})  •  ProxyRoute  •  ProxyConfig
                 constantes : port par défaut, TLD, format d'hôte, erreurs

rules/           proxyroute.go — pur, testable sans ouvrir une socket :
                   RouteHost{JobHost, Job, Worktree, Project} → "web.app-1.feat-auth.myapp.localhost"
                   HostLabel • ValidateHostLabels • RouteCollisions • ProxyURL{Host, Port}

service/proxy/   Registry (add/remove/lookup sous verrou) + Server
                 (httputil.ReverseProxy, aiguillage sur l'en-tête Host)
                 zéro import cobra / bubbletea / lipgloss

service/process/ le daemon possède le cycle de vie du proxy et alimente le Registry
                 depuis start / stop / reap ; Response gagne un champ URL à côté de Ports

flow/runlogs/    l'URL voyage dans les événements qui portent déjà les ports
output/          rendu, liens cliquables OSC-8
commands/run/    open.go, url.go
```

Le point qui rend l'ensemble petit : **il n'y a pas de nouvelle source de vérité**. Le Registry est une projection de `m.jobs`, pas un second état à synchroniser — un seul endroit écrit. Et `RouteHost` est une fonction pure de trois chaînes : c'est la totalité de la logique de nommage.

Le segment projet dérive du même nom de dépôt que `ComposeProjectName`, à la normalisation d'hôte près (voir « Du nom au label DNS ») : `myapp-feat-auth` côté Docker et `feat-auth.myapp.localhost` côté HTTP nomment la même chose de deux façons cohérentes.

## Runtime

### La table de routage

`Manager.startJob` enregistre `RouteHost(…)` → `127.0.0.1:<port résolu>` ; la goroutine qui reape et `stopByKey` la retirent. Les valeurs viennent d'où elles viennent déjà : `ManagedJob.Config.Ports` pour le port, `Env[WTM_WORKTREE]` pour le worktree, `WorkDir` pour le dépôt.

Le serveur se binde explicitement sur `127.0.0.1:<port>`, **jamais `0.0.0.0`** : un proxy qui exposerait tous les dev servers de la machine au réseau local serait un trou de sécurité.

### Le cycle de vie est celui du daemon

Le proxy démarre avec le daemon et meurt avec lui. Aucune commande `proxy start` / `proxy stop`, aucun PID à suivre : sans job qui tourne il n'y a rien à router, donc l'idle timeout de 30 s qui éteint le daemon libère aussi le port. Ce que le ticket redoutait (« gestion des routes/PID ») disparaît en se greffant sur un cycle existant.

**Conséquence assumée :** un service détaché (`docker compose up -d`) survit à la mort du daemon, mais sa route non — le conteneur tourne, son port répond, son URL ne répond plus. C'est le bug d'orphelinage de R1 ([LUC-194](https://linear.app/lucaspcq/issue/LUC-194)) vu sous un autre angle : F ne le crée pas, elle le rend visible, et R1 le corrige des deux côtés d'un coup. En attendant, l'URL d'un détaché est valable tant qu'un job tourne.

### Ce qui traverse

`httputil.ReverseProxy`, en **passant l'en-tête `Host` tel quel** et en ajoutant les `X-Forwarded-For` / `-Proto` / `-Host`. Passer le `Host` d'origine est ce qui fait que les cookies se posent sur le bon hôte : le réécrire en `localhost:3010` remettrait tous les worktrees dans le même jar et annulerait la feature. C'est aussi ce qui fait que Vite fonctionne sans configuration, puisqu'il autorise `.localhost` par défaut.

Les WebSockets passent sans code : `ReverseProxy` gère l'upgrade HTTP/1.1 → 101 nativement.

### Les trois échecs

**Le port du proxy est déjà pris.** Le daemon démarre quand même, les jobs tournent, seul le proxy est absent, avec une ligne nommant le port et la clé à changer. Règle dure : *le proxy ne doit jamais pouvoir empêcher un job de tourner*. C'est un confort greffé sur le socle, pas une dépendance du socle.

**Un `Host` inconnu.** Une page listant les routes actives avec leur worktree et leur dépôt. C'est le filet de découvrabilité : en se trompant de branche dans l'URL on tombe sur la liste de celles qui existent, au lieu d'un `ERR_CONNECTION_REFUSED`.

**La route existe mais rien n'écoute.** 502, avec le vocabulaire que `rules.DiagnosePortProbes` emploie déjà (`silent`, `base_listening`) — y compris le cas utile où c'est le port de base qui répond, signe que le job ignore la variable de port et bind `3000` en dur.

### L'avertissement `allowedDevOrigins`

Next.js bloque les requêtes dev cross-origin depuis un sous-domaine `.localhost` tant que `allowedDevOrigins` n'est pas déclaré. Plutôt qu'une note de doc, le motif de `domain.JobCmdFix` : au `run up`, un job qui a une `url` et dont la config Next ne mentionne pas `allowedDevOrigins` déclenche une ligne donnant la ligne exacte à coller. Détecté, pas documenté.

## Les surfaces

Le module `run` a trois surfaces (`domain.RunSurface`) ; l'URL apparaît dans les trois.

### `RunSurfaceView` — la vue plein écran

C'est là qu'on atterrit après `wtm run up`, donc la surface qui compte le plus. `internal/tui/runview/render.go:215` appelle déjà `rules.LabelWithPorts` ; la ligne gagne une colonne.

```
  ● db          PG_PORT=5442
  ● app1-api    PORT=4010    http://api.app-1.feat-auth.myapp.localhost:4000
  ● app1-bo     PORT=5010    http://app1-bo.feat-auth.myapp.localhost:4000
  ● app1-web    PORT=3010    http://app1-web.feat-auth.myapp.localhost:4000
```

Le port reste affiché : il n'est pas devenu inutile, c'est lui qui va dans les `.env` (G), dans les bindings Docker (A) et dans `depends_on`. L'URL est une porte de plus, pas un remplacement.

Une touche `o` ouvre l'URL du job sélectionné — le geste le plus direct, on est déjà devant la liste.

### `RunSurfaceStream` — la sortie ligne à ligne

Même ligne, sans TUI, avec le lien rendu cliquable par OSC-8 via un helper `output.Hyperlink(texte, url)` qui dégrade en texte nu dès que `rules.IsHumanFormat` est faux ou que la sortie n'est pas un terminal — pas de séquence d'échappement dans un pipe.

### `RunSurfaceMachine` — deux commandes

`wtm run url [job]` imprime l'URL et rien d'autre, sur le modèle de `resolve` : sortie machine, jamais encadrée, faite pour `$(…)`. `--raw` rend l'adresse directe `http://localhost:3010`, `--output json` la table complète.

`wtm run open [job]` ouvre le navigateur. La désambiguïsation suit la règle du CLAUDE.md sur les sélections requises : un seul job avec une URL parmi ceux qui tournent, on l'ouvre ; plusieurs, picker **seulement** en run pleinement interactif, sinon erreur nommant l'argument manquant. Jamais de picker en JSON ou sans TTY.

Ces deux commandes ne dépendent pas du proxy : sans lui, `run url` rend `http://localhost:3010`. C'est ce qui les rend livrables avant.

### JSON

`run ps` et `run up --output json` gagnent un champ `url` par job, absent quand le job n'en a pas. On s'aligne sur la forme existante de chaque commande ; l'incohérence que R5 ([LUC-198](https://linear.app/lucaspcq/issue/LUC-198)) doit harmoniser n'est pas rouverte ici.

### Dashboard

`wtm ui` ne pilote pas encore `run` : R2 ([LUC-193](https://linear.app/lucaspcq/issue/LUC-193)) n'est pas fait, donc hors périmètre. L'URL passant par `flow/runlogs` comme les ports, elle arrivera dans le dashboard le jour où R2 branche la couture, sans rien à réécrire.

## Limites assumées

**Linux hors navigateur.** macOS résout `*.localhost` nativement (vérifié : `ping bar.localhost` → `127.0.0.1`) et Chrome comme Firefox le font en interne sur tous les OS. Mais glibc ne traite pas les sous-domaines de `localhost` comme spéciaux : sous Linux, `curl http://app1-web.feat-auth.myapp.localhost:4000` échoue sans dnsmasq. Le navigateur — le cas qui motive la feature — fonctionne partout ; les scripts et les agents sous Linux, non. L'échappatoire est `wtm run url --raw`, qui rend l'adresse directe et marche partout, sans proxy. Le nom est pour l'humain et son navigateur ; le port reste l'adresse des machines.

**HTTP seulement.** Postgres, Redis, un broker : pas de nom possible. Un protocole sans indicateur d'hôte dans le flux ne peut pas être aiguillé sur un port partagé — même impossibilité que la « redirection loopback transparente » déjà écartée par l'epic. Ces services restent sur `base + offset`, ce que B et G ont résolu.

**Pas de HTTPS.** Un projet qui impose `https://` en dur reste hors périmètre ; le jour où il faudra, la forme du nom ne changera pas.

**`:4000` dans l'URL** jusqu'à F3.

**Aucun déterminisme perdu.** Le proxy n'alloue rien, il lit `base + offset`. Les décisions actées de l'epic (pas de sonde, pas de réallocation) restent vraies telles quelles.

## Découpage

Le point de découpe est net : **la déclaration et les surfaces ne dépendent pas du proxy**.

| | Périmètre | Coût |
| --- | --- | --- |
| **F1 — Découvrabilité** | Le champ `url = { port, host }` et sa validation. `wtm run url [job]` (+ `--raw`, `--output json`), `wtm run open [job]`, la touche `o` dans la vue, la colonne URL sur les trois surfaces, le champ `url` en JSON. Sans proxy, l'URL vaut `http://localhost:3010`. Skill agent, `make docs`, README. | ~1 j |
| **F2 — Le proxy** | `rules/proxyroute.go`, `service/proxy/` (Registry + Server), le câblage dans `service/process`, les trois échecs, l'avertissement `allowedDevOrigins`, `[proxy]` en config utilisateur. F1 ne change pas : seule la valeur que `run url` rend change. | ~3-4 j |
| **F3 — Le port 80** | `wtm run proxy install` : règle pf `rdr` sur `lo0` (macOS), `setcap cap_net_bind_service` (Linux). Opt-in, un `sudo` une fois. Les URLs perdent leur suffixe. | ~1-2 j, plus tard |

Ce que chaque tranche pose dans `rules/`, pour lever l'ambiguïté : **F1** apporte `HostLabel` et le constructeur d'URL directe (`http://localhost:<port résolu>`) ; **F2** apporte `RouteHost`, `RouteCollisions` et `ProxyURL`, et bascule ce que rendent `run url`, `run open` et la colonne des trois surfaces. `url.host` est donc déclarable et validé dès F1 — y compris le refus de deux jobs revendiquant le même hôte — mais n'a d'effet visible qu'à partir de F2. C'est voulu : la surface de config atterrit une fois, et F2 ne rouvre pas `run.toml`.

F1 a une valeur seule et se livre sans attendre : si F2 ne se faisait jamais, F1 aurait quand même réglé « je ne sais pas où aller ».

## Tests

- `rules/proxyroute_test.go` — `RouteHost`, `ValidateHostLabels`, `RouteCollisions` en tables. C'est toute la logique de nommage, testée sans ouvrir une socket.
- `service/proxy/` — un backend `httptest` et un vrai listener : le `Host` arrive intact, l'upgrade WebSocket passe, un hôte inconnu rend la liste, une route sans rien derrière rend le 502 diagnostiqué.
- `service/process/` — le Registry suit `start` / `stop` / reap, sur le modèle de `daemon_fake_test.go` et `manager_test.go`.
- `config/` — `LoadRun` refuse un `url.port` absent de `ports` et deux jobs revendiquant le même hôte.
- `commands/run/` — les gardes non-interactives de `url` et `open`, sur le modèle de `up_test.go`.

`build-validator` avant tout commit.

## À mettre à jour dans le même changement

Règle du projet, pas optionnel :

- `internal/commands/agents/assets/using-wtm.skill.md` — deux nouvelles commandes, un nouveau champ `run.toml`, un nouveau champ JSON ;
- `make docs` pour régénérer `docs/` ;
- la table d'aperçu du `README.md` ;
- le wizard `run job add` / `run job edit`, qui doit savoir proposer `url` — et son pendant non-interactif, qui est le trou de I ([LUC-203](https://linear.app/lucaspcq/issue/LUC-203)).

## Faits vérifiés au cadrage

- **macOS résout `*.localhost` nativement.** Testé : `ping bar.localhost` → `PING localhost (127.0.0.1)` ; `curl http://baz.localhost:9` résout puis refuse la connexion. Aucun `/etc/hosts`, aucun dnsmasq.
- **Linux, non.** glibc ne traite pas les sous-domaines de `localhost` comme spéciaux et systemd-resolved n'est pas complet là-dessus ([dotnet/runtime#118569](https://github.com/dotnet/runtime/issues/118569)). Chrome et Firefox les résolvent en interne, quel que soit l'OS.
- **`http://*.localhost` est un secure context.** La spec W3C « potentially trustworthy » inclut tout hôte se terminant par `.localhost`.
- **Vite fonctionne sans configuration.** `server.allowedHosts` autorise `localhost` et tout ce qui est sous `.localhost` par défaut ([doc Vite](https://vite.dev/config/server-options)).
- **Next.js demande une ligne.** Depuis 15.2.2, les requêtes dev cross-origin sont bloquées et il faut `allowedDevOrigins: ["*.localhost:4000"]` ([doc Next](https://nextjs.org/docs/app/api-reference/config/next-config-js/allowedDevOrigins)).
- **`portless`** écoute sur 443 par défaut (80 avec `--no-tls`), auto-élève avec sudo, génère et installe une CA locale, assigne un port éphémère 4000-4999 via `PORT`, expose `portless alias <name> <port>` pour les routes statiques, préfixe le nom de branche en sous-domaine dans un worktree lié, et est pré-1.0.
