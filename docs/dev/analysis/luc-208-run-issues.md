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

## 3. Des tasks semblent coupées avant la fin — **partiellement confirmé**

Symptôme : des tasks longues (seeds, migrations Nest) paraissent ne pas aller au
bout.

**Aucun chemin de code ne tue une task en cours.** Vérifié : `StopAll` n'est
appelé que sur arrêt du daemon (signal ou timer d'inactivité), et
`stopOtherJobs` (`internal/commands/run/up.go`) ne cible que les *autres*
worktrees. Une task est bien enregistrée `Running` dans la map du manager
pendant toute son exécution, donc le timer d'inactivité ne la voit pas comme
inactive.

En revanche, quatre mécanismes réels peuvent produire ce symptôme :

**a) Perte de chunks (probable, le plus proche du symptôme).**
`outputHub.Write` (`internal/service/process/manager.go:529`) publie vers des
canaux de 256 chunks (`outputSubscriberQueue`, `:35`) et **jette** le chunk
quand la file est pleine :

```go
select {
case sub <- data:
default:            // abonné lent → chunk perdu
}
```

Une task bavarde (des seeds qui écrivent des centaines de lignes d'affilée)
sature la file pendant que la TUI rend une frame : des morceaux disparaissent au
milieu de la sortie. Le fichier de log, lui, est complet — le commentaire de
`Subscribe` l'assume explicitement (« le stream est une vue live, pas un
enregistrement »). Le rendu paraît donc coupé alors que la task est allée au
bout. **Test discriminant : comparer l'affichage avec `wtm run logs <job>`.**

**b) Environnement non interactif (probable).** `jobEnv`
(`manager.go:288-302`) force pour les tasks `TERM=dumb`, `CI=true`, et
`spawnJob` laisse `cmd.Stdin` à `nil` (donc `/dev/null`). Un script de seed qui
demande une confirmation reçoit EOF et abandonne ; beaucoup d'outils changent de
comportement sous `CI=true`. C'est un abandon réel du script, pas un affichage.

**c) Fin de sortie tronquée (confirmé, ≤ 2 s).** `runTask` (`:366-372`) attend
au plus `detachedDrainGracePeriod` (2 s, `:57`) que le drain du pipe atteigne
EOF, puis ferme le descripteur. Si un processus descendant garde le pipe ouvert,
les derniers octets sont perdus.

**d) Course entre deux jobs (confirmé, très improbable).** `idleWatcher`
(`internal/service/process/daemon.go:88`) arrête le daemon si
`IsRunning()` est faux au tick des 30 s. Entre le `delete(m.jobs, key)` de fin
de task (`manager.go:392`) et l'enregistrement du job suivant, il existe une
fenêtre de quelques millisecondes où plus rien n'est enregistré. Un tick tombant
exactement là arrête le daemon au milieu d'un profil. Fenêtre ≈ 0,01 % par tick,
mais le défaut est réel.

Pistes : (a) borner la file par octets et compter/signaler les chunks perdus
plutôt que les jeter silencieusement, ou faire lire l'abonné TUI par une
goroutine dédiée à file profonde ; (b) rendre `CI` et `TERM` configurables par
job dans `run.toml` ; (c) allonger ou rendre configurable le délai de drain ;
(d) faire porter au daemon un compteur de « runs en cours » plutôt que de
déduire l'activité de la map des jobs.

Voir aussi : `ProposeProfiles` **exclut les tasks des profils** (voir §4), donc
un profil proposé ne contient jamais `migrate`/`seed` — à vérifier dans le
`run.toml` en cause.

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
  par le wizard sans édition manuelle.

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
| 6/7 | Escalier des tasks | confirmé | faible | fort — illisible à chaque run |
| 5a | Profil non affiché | confirmé | faible | fort |
| 2 | Ligne de profil non bornée | confirmé | faible | moyen |
| 1 | Détection des packages profonds | confirmé | moyen | fort — bloque l'init |
| 4 | Trop de profils proposés | confirmé | moyen | moyen |
| 5b | Jobs hors run + anciens logs | confirmé | moyen | moyen |
| 3a/3b | Sortie de task perdue / env CI | probable | moyen | à reproduire d'abord |
| 3c/3d | Drain 2 s, course du timer d'inactivité | confirmé | faible | rare |

Pour le point 3, le test discriminant à faire en premier : lancer la task
incriminée, puis comparer l'affichage de la vue avec `wtm run logs <job>`. Si le
log est complet, c'est (a) ; s'il est tronqué au même endroit, c'est (b).
