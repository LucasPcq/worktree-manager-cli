# LUC-208 — Analyse des sept problèmes remontés sur `run`

Analyse de code uniquement : aucun correctif n'est appliqué ici. Chaque section
donne le symptôme, la cause identifiée dans le code, et la piste de correction.
Le niveau de certitude est indiqué : **confirmé** (démontré sur le code ou par un
test), **probable** (mécanisme identifié, non reproduit), **hypothèse**.

---

## 1. Les scripts d'un `package.json` profond ne sont pas détectés — **confirmé**

Symptôme : `apps/app-1/back/package.json` n'apparaît pas dans `wtm run init`.

Deux causes cumulées, toutes deux dans `internal/service/detect`.

**a) `**` n'est pas récursif en Go.** `PnpmWorkspacePackages`
(`internal/service/detect/monorepo.go:23`) résout les patterns de
`pnpm-workspace.yaml` avec `filepath.Glob`, qui ne connaît pas le `**` de
globstar : il le traite comme un `*` simple, borné à un seul segment de chemin.
Vérifié :

| pattern      | `filepath.Glob`                    | attendu (sémantique pnpm) |
|--------------|------------------------------------|---------------------------|
| `apps/*`     | `apps/app-1`                       | `apps/app-1`              |
| `apps/**`    | `apps/app-1`                       | `apps/app-1`, `apps/app-1/back`, … |
| `apps/*/*`   | `apps/app-1/back`, `apps/app-1/front` | idem                   |

Un workspace déclaré `packages: - "apps/**"` — la forme la plus courante — ne
remonte donc **que le premier niveau**. `apps/app-1/back` est invisible, ce qui
est exactement « trop loin dans l'arborescence ».

**b) Seul pnpm est supporté.** `ProjectEnvironment`
(`internal/service/detect/project.go:25`) et `PackageJSONScripts`
(`internal/service/detect/package_scripts.go:26`) n'appellent que
`PnpmWorkspacePackages`. Le champ `workspaces` d'un `package.json`
(npm / yarn / bun) n'est lu nulle part dans le dépôt. Un monorepo npm ou yarn
ne remonte donc que les scripts de la racine, quelle que soit la profondeur.

**c) Effet de bord** : `PackageJSONScripts` retourne `nil` d'emblée si la racine
n'a pas de `package.json` (`package_scripts.go:19-22`). Un dépôt dont tous les
`package.json` sont dans des sous-dossiers ne détecte **rien du tout**.

Piste : remplacer `filepath.Glob` par une marche d'arborescence avec un vrai
matcher globstar (bornée en profondeur et ignorant `node_modules`, `dist`,
`.git`), lire aussi le champ `workspaces` du `package.json` racine, et ne plus
faire dépendre la détection des workspaces de la présence d'un `package.json`
racine.

---

## 2. La ligne d'un profil déborde au lieu de passer à la ligne — **confirmé**

Symptôme : dans l'étape « profils » de `run init`, un profil qui contient
beaucoup de jobs occupe une ligne illisible et tronquée.

`profileLabel` (`internal/tui/components/profilelist.go:307`) construit
`"<nom> (<job1>, <job2>, …)"` en une seule chaîne, sur une seule ligne.
`renderRow` (`:315`) ne fait que **remplir** jusqu'à `m.width` :

```go
if pad := m.width - PrintableWidth(line); pad > 0 {
    line += strings.Repeat(" ", pad)
}
```

Il n'existe aucun appel à `ansi.Truncate` ni à un wrapping dans tout
`internal/tui/components/` — aucun composant de liste ne borne la largeur d'une
ligne. Au-delà de la largeur du terminal, c'est le terminal qui coupe (ou
enroule), et le fond de `styles.ListItemSelected` déborde avec.

`m.height` est stocké par `SetSize` mais n'est jamais lu : la liste n'a pas non
plus de viewport vertical, ce qui aggrave le point 4 ci-dessous.

Piste : tronquer avec ellipse à `m.width` (ou faire tenir les jobs sur des lignes
de continuation indentées, la ligne sélectionnée seule étant développée), et
ajouter un viewport vertical à `ProfileListModel`.

---

## 3. Des tasks semblent coupées avant la fin — **cause trouvée : elles ne tournent pas**

Symptôme : une task longue (migrations + seeds, 10 à 15 s) doit passer pour que le
service suivant démarre, et le service suivant démarre mal **à chaque fois**.

Le caractère déterministe écarte toute explication par une perte de logs. Deux
mesures ont été faites.

### L'ordonnancement, lui, est correct

Un run réel — vrai daemon sur socket unix, vrai `runlogs.Run`, task de 8 s
écrivant 2000 lignes puis créant un marqueur, suivie d'un service qui teste ce
marqueur — donne :

```
elapsed=8.01s   completed=[seed]  started=[api]  failed=""
streamed: 2001 lignes sur 2001 attendues ; fichier de log : 2001 lignes
le service a vu le marqueur : "READY"
```

La task bloque bien la séquence, aucune ligne n'est perdue, et le service
démarre après. Le débit du panneau n'est pas non plus en cause (~40 µs par
ligne, 2000 lignes en 79 ms). **Quand la task est dans le profil, tout est
correct.**

### Le vrai défaut : les tasks sont filtrées hors du run

Deux endroits indépendants retirent les tasks de ce que `run up` lance.

**a) `rules.JobsWithoutProfile` (`internal/rules/jobs.go:214-222`)** — utilisé par
`resolveProfileJobs` quand `run.toml` ne déclare **aucun profil** :

```go
for _, job := range cfg.Jobs {
    if job.Kind == domain.JobKindService {   // toute task est écartée
        jobs = append(jobs, job)
    }
}
```

**b) `rules.ProposeProfiles` (`internal/rules/profileplan.go:36`)** — le découpage
que propose `run init` :

```go
if job.Kind != domain.JobKindService {
    continue                                  // aucune task n'entre dans un profil
}
```

Aucun profil produit par le wizard — `all` compris — ne contient donc de
`migrate` ni de `seed`.

### Reproduction de bout en bout

Sur `wtm run up -d`, avec les jobs `migrate` (task, crée un marqueur) puis `api`
(service, teste ce marqueur) :

| `run.toml` | sortie de `run up` | le service voit le marqueur |
|---|---|---|
| jobs seuls, **aucun profil** | `[1/1] api` | `MISSING` — `migrate` n'a jamais tourné |
| profil listant `migrate, api` | `[1/2] migrate` → `[2/2] api` | `READY` |

La task est retirée **silencieusement** : le compteur d'étapes affiche `[1/1]`,
donc rien à l'écran ne signale qu'un job déclaré a été ignoré.

### Le diagnostic à faire sur le dépôt concerné

Deux questions tranchent :

1. **Le compteur d'étapes de `run up` inclut-il la task ?** `[1/2] migrate` →
   elle est dans le run. Un compteur qui la saute → c'est (a) ou (b) ci-dessus.
2. **Quel `kind` a le job dans `run.toml` ?** `ClassifyScriptKind`
   (`internal/rules/scripts.go:73`) classe en **service** tout script dont le nom
   vaut `dev`/`start`/`serve`/`watch`, ou les porte comme segment délimité par
   `:` (`dev:api`, `api:watch`). Un script de seed nommé `dev:seed` ou
   `seed:watch` devient un service — et un service n'est **pas** attendu par la
   séquence : `run up` le lance en arrière-plan et passe immédiatement au job
   suivant. C'est l'unique scénario qui explique à la fois de voir les lignes
   défiler pendant 10 à 15 s **et** le service suivant démarrer trop tôt à chaque
   fois.

Pistes de correction :
- faire porter les tasks par les profils proposés (§4) plutôt que de les exclure ;
- que `JobsWithoutProfile` conserve les tasks dans l'ordre déclaré, ou qu'elle
  refuse le run en nommant les jobs qu'elle ne sait pas ordonner ;
- ne jamais retirer un job en silence : si `run up` ignore un job déclaré, le dire
  dans la sortie plutôt que de réduire le compteur d'étapes ;
- exposer une dépendance explicite entre jobs (`needs = [...]`) pour que l'ordre ne
  dépende plus du seul `kind`.

### Défauts secondaires trouvés en chemin (réels, non responsables du symptôme)

- **Chunks jetés sous charge.** `outputHub.Write`
  (`internal/service/process/manager.go:529`) publie vers des canaux de 256
  chunks et jette le chunk quand la file est pleine (`select`/`default`). Le
  fichier de log reste complet — c'est assumé — mais l'affichage peut perdre des
  morceaux. Non reproduit à 2000 lignes.
- **Environnement non interactif.** `jobEnv` (`manager.go:288-302`) force pour les
  tasks `TERM=dumb` et `CI=true`, et `spawnJob` laisse `stdin` à `/dev/null`. Un
  script attendant une confirmation reçoit EOF.
- **Fin de sortie tronquée à 2 s.** `runTask` (`:366-372`) attend au plus
  `detachedDrainGracePeriod` que le drain atteigne EOF avant de fermer le
  descripteur.
- **Course du timer d'inactivité.** `idleWatcher`
  (`internal/service/process/daemon.go:88`) arrête le daemon si `IsRunning()` est
  faux au tick des 30 s ; entre le `delete` de fin de task (`manager.go:392`) et
  l'enregistrement du job suivant, la map est momentanément vide.
- **Attache impossible sur une task.** `handleAttach`
  (`internal/service/process/daemon.go:310`) fait `io.Copy(session.PTY, conn)`
  alors que `session.PTY` est, pour une task, l'**extrémité de lecture** d'un
  pipe : l'écriture échoue aussitôt, `<-done` se débloque et la session se
  referme immédiatement. `Manager.Resize` refuse déjà explicitement les tasks
  (`manager.go:759`), mais `attachableJob` les laisse passer.

---

## 4. Trop de profils créés par défaut — **confirmé**

`ProposeProfiles` (`internal/rules/profileplan.go:36-70`) crée **un profil par
répertoire de package distinct**, plus un profil global `all`. Sur un monorepo
de vingt services, ça fait vingt-et-un profils, et l'étape §2 les affiche sans
viewport.

Deux défauts secondaires dans la même fonction :

- **Collision de noms** : la clé est `filepath.Base(job.Cwd)` (`:44`).
  `apps/app-1/back` et `apps/app-2/back` produisent tous deux `back` et sont
  **fusionnés dans un seul profil** — silencieusement, avec les jobs des deux
  applications.
- **Les tasks sont exclues** : `if job.Kind != domain.JobKindService { continue }`
  (`:36`). Aucun profil proposé — `all` compris — ne contient de `migrate` ou de
  `seed`. `wtm run up` ne lancera donc jamais les tasks d'un projet initialisé
  par le wizard sans édition manuelle. **C'est la cause du point 3** : voir
  la reproduction là-bas.

Pistes : ne proposer le découpage par package qu'au-dessus d'un seuil, ou
proposer par défaut le seul profil `all` et laisser l'utilisateur scinder ;
désambiguïser les noms par le chemin relatif quand `filepath.Base` collisionne ;
décider explicitement du sort des tasks dans les profils.

---

## 5. `run up` ne dit pas quel profil il lance et liste des jobs hors run — **confirmé**

Deux causes distinctes.

**a) Le nom du profil est jeté.** `runUp` (`internal/commands/run/up.go:76`)
appelle `resolveProfileJobs`, qui résout le profil en `[]domain.JobConfig` et
**ne retourne pas son nom**. Aucune couche en aval (ni `runSeam`, ni
`runlogs.Run`, ni `runview`, ni `Outcome`) ne porte le profil : il est
structurellement impossible de l'afficher aujourd'hui.

**b) La vue liste tous les jobs déclarés.** `openRunSeam`
(`up.go:98`) passe `Jobs: runCfg.Jobs` — la config **entière** — à
`runlogs.NewSession`, alors que le profil résolu est passé séparément à
`seam.starter(jobs)`. C'est assumé dans le code :

```go
// Jobs are the worktree's declared jobs, which the view lists whether or not
// this run touches them.
```

La vue affiche donc les jobs des autres profils. Et comme `OpenLogSink`
(`internal/service/process/logstore.go:58`) ouvre les logs en **mode append**
sans séparateur de run, sélectionner un de ces jobs affiche via `History()` le
log de son **run précédent** — les « anciens logs ».

Pistes : faire porter le nom du profil par `RunParams`/`Outcome` et l'afficher
dans le titre de la vue et dans le récap ; réduire la session au profil lancé
(`rules.FilterToProfile` existe déjà, `internal/rules/jobs.go:109`, et n'est
utilisé que par `run export`) ou marquer visuellement les jobs hors run ;
écrire un marqueur de début de run dans le log pour que `History()` puisse
n'afficher que le run courant.

---

## 6 & 7. Affichage « en escalier » pendant l'exécution d'une task — **confirmé**

Symptômes 6 et 7 sont le même bug. Cause : **les tasks n'ont pas de PTY**.

`spawnJob` (`internal/service/process/manager.go:229-245`) donne un `os.Pipe`
aux tasks et un vrai PTY aux services. Sur un PTY, le noyau applique `ONLCR` et
traduit chaque `\n` en `\r\n`. Sur un pipe, non : la task écrit des `\n` nus.

Ces octets bruts sont écrits tels quels dans l'émulateur de terminal du panneau —
`sequence.go:49` → `panes.write` → `Pane.Write` → `vt.Emulator.Write`. Pour un
émulateur, `\n` descend d'une ligne **sans revenir en colonne 0**. Démontré par
un test jetable sur `runview.Pane` (40×6) :

```
LF seul :                       CRLF :
Seeding·users...                Seeding·users...
················Seeding·orders...   Seeding·orders...
·································Done   Done
```

C'est exactement « tout en décalé pendant que ça écrit », et c'est spécifique
aux tasks — les services passent par un PTY et sont corrects.

Pourquoi ça redevient propre à la fin : le chemin de relecture d'historique
écrit des fins de ligne explicites — `pane.Write([]byte(line + "\r\n"))`
(`internal/tui/runview/panes.go:87`). Dès que le panneau est réalimenté depuis
le fichier de log au lieu du flux live, l'alignement est correct.

Piste : normaliser `\n` → `\r\n` sur la sortie des jobs *sans PTY* uniquement.
Le bon endroit est la frontière qui connaît le `Kind` — soit le daemon avant
d'émettre les chunks d'une task, soit `paneStore.write` via une `paneSource`.
Attention à ne pas doubler les `\r` sur la sortie des services (déjà en CRLF) et
à traiter le cas d'un `\r` en fin de chunk suivi d'un `\n` en tête du suivant.

---

## Ordre de traitement suggéré

| # | Problème | Certitude | Effort | Impact |
|---|----------|-----------|--------|--------|
| 3 | Tasks filtrées hors du run | reproduit | moyen | bloquant |
| 6/7 | Escalier des tasks | confirmé | faible | fort — illisible à chaque run |
| 5a | Profil non affiché | confirmé | faible | fort |
| 2 | Ligne de profil non bornée | confirmé | faible | moyen |
| 1 | Détection des packages profonds | confirmé | moyen | fort — bloque l'init |
| 4 | Trop de profils proposés | confirmé | moyen | moyen |
| 5b | Jobs hors run + anciens logs | confirmé | moyen | moyen |
| 3bis | Chunks jetés, drain 2 s, course du timer, attache sur task | confirmé | faible | rare |

Le point 3 est le plus grave : il est déterministe, silencieux, et casse le run.
Il partage sa cause avec le point 4 (`ProposeProfiles` exclut les tasks), donc les
deux se corrigent ensemble.
