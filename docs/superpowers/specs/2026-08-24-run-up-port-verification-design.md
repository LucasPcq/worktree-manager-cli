# `wtm run up` vérifie les ports déclarés

**Date** : 2026-08-24
**Statut** : design validé, prêt à implémenter
**Lot** : 1/2 (le lot 2 refond le flow de `run init`)

## Le problème

`wtm run up` annonce `✓ web started · WEB_PORT=5183` sur le seul fait que le
processus a démarré. Rien ne vérifie que le port déclaré a effectivement été
lié. Quatre situations produisent exactement le même mensonge :

| Situation | Ce que wtm annonce | Ce qui se passe |
| --- | --- | --- |
| Turborepo en `envMode: "strict"` (le défaut) | `✓ WEB_PORT=5183` | la variable n'atteint pas le package |
| une CLI qui ne lit que `--port` | `✓ WEB_PORT=5183` | le flag manque, le serveur bind sa valeur par défaut |
| un port en dur dans le code | `✓ WEB_PORT=5183` | la variable est ignorée |
| un `.env` qui réécrase la variable | `✓ WEB_PORT=5183` | le fichier gagne |

Vérifié sur une vraie stack turborepo : avec `envMode` par défaut, un job racine
`turbo run dev` portant `API_PORT`/`WEB_PORT` les livre à ses tâches en
`undefined`. Les deux correctifs (`globalPassThroughEnv` dans `turbo.json`, ou
`--env-mode=loose`) fonctionnent, mais **relèvent de l'utilisateur** : wtm ne
configure pas les outils d'à côté.

## Le principe

Du point de vue de wtm les quatre lignes du tableau sont un seul bug : la
variable a été injectée, le processus a lié autre chose. Un adaptateur par outil
n'en couvre qu'une, demande d'être maintenu, et vieillit. Une vérification les
couvre toutes — y compris celles qu'on n'a pas listées — et ne vieillit jamais,
parce qu'elle ne connaît aucun outil.

> **wtm déclare, injecte, et vérifie. Il ne configure jamais l'outil d'à côté.**

## Portée

Dans le lot :

- sonde post-démarrage des ports déclarés, sur les jobs `kind = "service"` ;
- verdict par port, rendu sur les trois surfaces (run view, flux texte, JSON) ;
- avertissement non bloquant.

Hors du lot, explicitement :

- **pré-sonde avant démarrage** (« ce port était déjà occupé »). Elle éviterait
  un faux vert si un process tiers tient le port, mais le cas fréquent — la
  variable ne passe pas — est déjà couvert sans elle. À rajouter si le besoin
  se montre.
- toute écriture dans `turbo.json`, `nx.json` ou quelque configuration tierce.
- l'appartenance du listener à un processus donné : voir ci-dessous.

## Méthode : dial TCP, et pourquoi pas les PID

L'appartenance par PID (`lsof`, `ss`) est impossible pour le cas le plus
courant : sur un job compose, le processus qui écoute est `dockerd`, jamais un
descendant du job — et `docker compose up -d` rend la main, donc il n'y a même
plus de PID à interroger. La méthode retenue est donc le **dial TCP** sur le
port attendu.

Ce que la sonde affirme, exactement : *quelque chose écoute sur le port que wtm
a déclaré*. Elle ne prouve pas que c'est votre service. C'est la limite honnête
de la méthode, et elle suffit : le cas à attraper est *rien n'écoute*.

Le dial vise `127.0.0.1`, avec repli sur `[::1]` — un bind sur `0.0.0.0` comme
un bind Docker répondent sur la première.

## Verdicts

Deux verdicts, plus un indice :

- **`PortListening`** — quelque chose a répondu sur le port résolu.
- **`PortSilent`** — rien n'a répondu dans le budget imparti.
  - indice **`BaseListening`** : le port *de base* répond, alors que le port
    résolu non. C'est la signature de « la variable n'a pas atteint le
    processus ». L'indice n'a de sens que hors du checkout principal : sur
    `main` l'offset vaut 0, donc base et résolu sont le même port et il n'y a
    rien à distinguer. On ne l'émet pas plutôt que de bluffer.

Rendu visé :

```
✓ api-dev      API_PORT=3011 écoute
⚠ web-dev      rien n'écoute sur WEB_PORT=5183
                 mais 5173 écoute — le port de base
                 la commande a tourné, la variable ne l'a pas atteinte
```

## Cadence et budget

Polling toutes les 250 ms, **jusqu'à ce que tous les ports attendus répondent**
ou que le budget expire. Une stack saine conclut donc en une seconde ou deux :
seul un échec consomme le budget entier, ce qui rend un défaut généreux gratuit.

- budget par défaut : **15 s** ;
- réglable dans `run.toml` par `port_probe_timeout` (secondes) — la clé est à
  ajouter à `internal/schemas/run.schema.json`, versionné, en même temps qu'au
  type `domain.RunConfig` ;
- `0` désactive la sonde ;
- `--no-probe` la désactive pour une invocation.

`-d` (détaché) sonde comme les autres modes : c'est précisément là qu'on veut
savoir si ça a marché. `--no-probe` reste pour qui veut la main immédiatement.

## Ce qu'on sonde

Les ports d'un job qui remplit les trois conditions :

1. `kind = "service"` — une task n'écoute pas ;
2. démarré (`PhaseStarted`), y compris `AlreadyRunning` ;
3. `job.Ports` non vide, résolus en `base + WTM_PORT_OFFSET`.

Les ports résolus sont déjà là : `StartResult.Ports` les remonte du daemon, et
l'`Event` de `PhaseStarted` les porte. La sonde lit ce champ plutôt que de
recalculer `base + offset` de son côté — le nombre injecté et le nombre sondé ne
peuvent ainsi pas diverger. L'offset reste nécessaire, lui, pour savoir si
l'indice `BaseListening` a un sens ; il vient de `worktree.JobEnv`.

`AlreadyRunning` est vérifié : un re-démarrage de service émet bien
`PhaseStarted` avec le drapeau, donc le job est sondé comme les autres. Une
task refusée, elle, avorte la séquence et ne pose pas la question.

**Faux positif connu** : un job qui déclare un port servant à autre chose qu'un
bind sera signalé à tort. Le message dit ce qui est observé, jamais ce qui est
fautif, et le remède est de ne pas le déclarer comme port. Un opt-out par port
sera ajouté si le cas se présente réellement.

## Découpage en couches

Conforme à `CLAUDE.md` :

| Couche | Contenu |
| --- | --- |
| `internal/domain` | `PortProbe`, `PortProbeStatus`, constantes de message |
| `internal/rules/portprobe.go` | **pur** : attendu + observé + offset → verdicts ; lignes de rendu |
| `internal/service/portprobe/` | **impur** : dial TCP et polling, seul détenteur de l'I/O réseau |
| `internal/flow/runlogs` | phase `PhaseProbed`, verdicts portés par `Outcome` |
| `internal/output/runlogs.go` | rendu du flux texte et JSON |
| `internal/tui/runview/sequence.go` | rendu dans la run view |
| `internal/commands/run/up.go` | `--no-probe`, lecture du budget depuis la config |

Ajouter une phase oblige à l'enseigner aux deux surfaces qui rendent les
événements — c'est voulu, une phase muette serait une régression silencieuse.

## Tests

- **`rules`** : table de verdicts — port qui répond, port muet, port muet avec
  base qui répond, même cas à offset 0 (l'indice doit disparaître), aucun port
  déclaré.
- **`service/portprobe`** : contre un vrai `net.Listen` sur un port éphémère,
  pas de mock. Couvre le port qui répond, le port muet, le budget qui expire,
  et l'arrêt anticipé quand tout a répondu.
- **`flow/runlogs`** : la séquence émet `PhaseProbed` pour les seuls jobs
  éligibles, et `Outcome` porte les verdicts.
- **`output`** : le rendu ne décide rien — un verdict entrant, une ligne
  sortante.

## Critères d'acceptation

1. Sur la stack turborepo de test, `run up` depuis un worktree non principal
   signale `WEB_PORT` muet et nomme le port de base qui écoute.
2. Après ajout de `globalPassThroughEnv` dans `turbo.json`, le même `run up`
   passe au vert sans autre changement.
3. Une stack saine ne rallonge pas `run up` de plus d'une seconde ou deux.
4. Un port muet n'échoue pas `run up` : le code de sortie est inchangé.
5. `port_probe_timeout = 0` et `--no-probe` suppriment toute sonde.
