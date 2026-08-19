# LUC-186 — Migrer `sync` vers `flow/` et l'exposer dans le dashboard

Design validé le 2026-08-19. Suite de LUC-175 (couche `flow/`) et LUC-173 (dashboard).

## Objectif

`sync` est la dernière commande de mutation à haute fréquence encore écrite sur le
modèle pré-`flow`. L'onglet Tree du dashboard affiche `⚠ needs sync` sans offrir le
remède ; cette migration ferme l'écart et supprime la dernière closure injectée dans
un package `tui/` (`PlanPreview`).

## État actuel

`internal/commands/wt/sync.go` (524 lignes) enchaîne : lecture des flags → scan des
parents (`worktree.ClassifyParents`, derrière un spinner) → `resolveSyncSelection`,
qui lance `internal/tui/syncpicker` (multi-select worktrees → on-conflict → parents →
recap avec preview asynchrone du plan) → `worktree.PlanSync` → un second point de
confirmation hors picker → `worktree.Sync` → recap → `rules.DecidePush` →
`worktree.PushSynced` → push summary.

Deux closures traversent la frontière `commands/` → `tui/` :
`PlanPreview` (appelle `worktree.PlanSync` puis `output.SprintSyncPlan`) et
`StaleParents`.

Le preview du plan est confirmé à deux endroits, avec cinq lignes de commentaire sur
la gestion des lignes vides selon le chemin emprunté (`selection.PlanConfirmed`).

## Décisions

### D1 — `output.SprintSyncPlan` descend dans `rules/`

Le flow ne peut pas importer `output/`. `SprintSyncPlan` est un pur constructeur de
chaîne (stdlib + `domain`) : il devient `rules.SprintSyncPlan`, et `output` l'appelle.
C'est ce qui supprime la closure `PlanPreview`.

### D2 — Trois axes dans la `Request`, dont un propre au dashboard

- `Branches []string` + `All bool` : la sélection **fixée** (args / `--all`). Elle
  devient un preset de la session : le step n'est pas posé mais reste lu en recap.
- `Precheck []string` : les cases **déjà cochées** quand le multi-select *est* posé.
  Le CLI ne le passe jamais, donc le picker CLI reste vide au départ, comme
  aujourd'hui. Seul le dashboard s'en sert.
- `KeepConflict`, `FFParents`, `NoFFParents`, `Push`, `NoPush`, `DryRun`,
  `BaseBranch`.

Les refus `--push`/`--no-push`, `--ff-parents`/`--no-ff-parents` et `--all` combiné à
des arguments restent dans `commands/` : c'est du wiring de flags, pas du flow.

### D3 — Quatre steps, dans l'ordre actuel

1. `StepMultiSelect` `sync.selection` — worktrees + la base (label `(base)`), tags
   `dirty` / `rebasing`, `Selected` piloté par `Precheck`. `Resolve` renvoie une
   erreur nommant `--all` : jamais de picker de repli.
2. `StepSelect` `sync.conflict` — « Sync normally » / « Keep conflicts in progress »
   (danger). `Skip` quand le plan n'a aucune étape de rebase (rafraîchissement de la
   base seule : aucun conflit possible). `Resolve` → `normal` ; preset par
   `--keep-conflict`.
3. `StepSelect` `sync.parents` — la question fast-forward. `Skip` quand rien n'est en
   retard ; preset par `--ff-parents`/`--no-ff-parents` via `rules.ParentFlagsDecision`,
   dont `Interactive` est branché sur `prompter.Interactive()`.
4. `StepRecap` `sync.confirm` — `Load` + `LoadingMessage` (le pattern de
   `clean.deleteStep`) : le plan se calcule hors du thread UI derrière un spinner,
   comme le `OnEnter`/`planRequestMsg` du picker aujourd'hui. Description =
   `rules.SprintSyncPlan` + la question + le ⚠ keep-conflict.

Le scan `ClassifyParents` reste **avant** la session, dans un `presenter.Stage`, et
seulement quand `prompter.Interactive() && !DryRun` — même condition, même spinner.

### D4 — Un `Presenter` à trois moments

La question du push tombe *entre* deux sorties, donc la conclusion ne peut pas être
unique :

- `Planned(domain.SyncPlan)` — appelé **uniquement quand `prompter.Interactive()` est
  faux**. C'est ce qui reproduit le double chemin actuel : sous `--yes` / `--dry-run` /
  JSON le plan est imprimé (`FrameStart` sur stderr) ; en interactif il n'apparaît que
  dans le recap. La gymnastique de padding disparaît sans changer un octet de sortie.
- `Rebased(domain.SyncResult)` — le recap avant la question du push (`Blank` +
  `FormatSyncResult` sur stdout).
- `Synced(Outcome)` — la conclusion : push summary + `FrameEnd`, ou le JSON en un bloc.

### D5 — `rules.DecidePush` réutilisée telle quelle

`rules.DecidePush{Push, NoPush, Interactive: prompter.Interactive(), PushableCount}`.
La table de vérité actuelle est reproduite sans passer `Yes` : `--yes` et JSON
installent tous deux un prompter non interactif, ce qui donne le même verdict.
`PushConfirm` passe par `prompter.Confirm`, après l'exécution — comme le on-conflict
d'`extract`, pas comme un step de session.

### D5b — `flow.ConfirmParams` gagne `YesLabel` / `NoLabel`

`flowui.Confirm` rend aujourd'hui un Yes/No, alors que le `confirmPush` actuel est une
SelectList à deux options nommées — « Keep local » en tête, « Push to origin » ensuite —
délibérément, pour que le force-push ne soit pas le défaut surligné. Utiliser `Confirm`
tel quel changerait le widget.

`flow.ConfirmParams` gagne donc `YesLabel` et `NoLabel`. Quand ils sont vides, les deux
surfaces rendent ce qu'elles rendent déjà (aucun appelant existant ne bouge :
`clean` et `create` sont les seuls). Quand ils sont remplis, `flowui` rend la même
SelectList qu'aujourd'hui — l'option « non » en tête tant que `DefaultYes` est faux — et
le dashboard rend une modale à deux options au lieu d'une seule. La question du push est
alors identique sur les deux surfaces, sans que le flow sache lequel des deux widgets la
porte.

### D6 — `Operation()` : `ModeBlocking`, sans `TargetKey`

`sync` rebase plusieurs worktrees et pose des questions avant : elle tient la surface
pour tout son run, comme `reparent` et `prune`. Tenant tout, elle n'a besoin d'aucun
verrou par worktree.

### D7 — Sans TTY, sortie humaine, sans `--yes` : refuser

Aujourd'hui `wtm sync feat-a | cat` imprime le plan puis lance un confirm TUI sur un
non-TTY, qui échoue et affiche « Aborted. ». Le flow s'aligne sur `prune`
(`PruneNeedsTerminal`) : sortie humaine + pas de TTY + ni `--yes` ni `--dry-run` →
refus nommant `--yes`. Micro-évolution assumée, figée par un test.

### D8 — `--dry-run` : pas d'entrée dashboard, comme `prune`

Le recap de la session **est** le plan (`SprintSyncPlan`), et fermer la modale ne
rebase rien : le dry-run est structurellement offert sans flag. Les deux commandes
sont alignées ; la raison est écrite dans `docs/dev/`.

### D9 — `--keep-conflict` depuis le dashboard : option B, l'offrir et nommer la sortie

Le step est posé comme sur le CLI : la décision reste celle de l'utilisateur, le
dashboard n'en cache aucune. Quand des conflits sont gardés, le panneau de sortie
nomme, par branche, le chemin de la worktree et le `git rebase --continue` /
`git rebase --abort` à y lancer — même geste que `DashboardPrivilegedHintFmt` pour la
suppression privilégiée. Le rechargement post-run fait apparaître le badge
`⟳ rebasing` sur la ligne concernée.

Ce qui a été écarté : ne pas l'offrir (ampute une option que le CLI expose et force à
relancer toute la cascade depuis le terminal), et quitter le dashboard sur conflit
(dépend d'une intégration shell, et fait sortir d'une surface qu'on vient d'ouvrir).

Limite connue et acceptée : le lock `ModeBlocking` protège pendant le run, pas après.
Une worktree laissée en rebase reste actionnable ; c'est le badge `⟳ rebasing` qui la
signale.

**À écrire dans `docs/dev/flow-layer.md`** (critère d'acceptation de l'issue).

### D10 — Trois entrées dans le dashboard

- **Menu ligne (worktree)** : `Sync this worktree`, en tête, avant `Change parent`.
  `Precheck` = la worktree **et ses descendants** (nouvelle règle pure
  `rules.SyncSubtree`), multi-select ouvert et modifiable. La sémantique de sélection
  reste exacte — `rules.FilterSyncSteps` ne déduit aucun descendant ; ce qui change,
  c'est ce qui arrive coché.
- **Menu ligne (parent/base)** : la ligne `IsParent`, qui n'a aucun menu aujourd'hui,
  en gagne un à une seule entrée : `Refresh base branch` → `Branches: [base]`, steps
  conflit et parents auto-skippés, recap direct.
- **`⋯ Actions`** : `Sync all worktrees`, `Precheck` = tout **sauf** les worktrees
  `dirty` ou `rebasing`, listées avec leur tag et décochées (le modèle `prune`).
  Divergence assumée avec `--all` (qui coche tout), à écrire dans `docs/dev/`.

Le badge `⚠ needs sync` reste inerte : le menu contextuel de la ligne suffit.

## Méthode

1. **Compléter la couverture de caractérisation** sur `sync`, sur le modèle de
   `internal/commands/wt/prune_test.go` (repo git réel + origin) : ordre de la
   cascade, conflit avec et sans `--keep-conflict` et descendants sélectionnés
   sautés, `--all` vs arguments, `--push`/`--no-push`, `--dry-run`, erreurs de flag
   manquant sous `--yes`, exclusivité des flags, et l'ordre de sortie
   plan → recap → push summary sur **chacun des deux chemins** (avec et sans picker).
2. La faire passer **avant** toute modification de production.
3. Migrer sans jamais toucher à ces tests.

## Critères d'acceptation

- [ ] `sync` passe par `internal/flow/sync/` ; `commands/wt/sync.go` ne fait plus que
      le wiring des flags et le choix Prompter/Presenter.
- [ ] `internal/tui/syncpicker` supprimé, et avec lui la closure `PlanPreview`.
- [ ] `rules.DecidePush` réutilisée telle quelle, branchée sur `Prompter.Interactive()`.
- [ ] La question du push garde ses deux options nommées sur le CLI (`YesLabel`/`NoLabel`).
- [ ] Comportement CLI inchangé — ordre de la cascade, ordre de sortie
      preview → confirm → push, padding autour du recap — hors la micro-évolution D7,
      couverte par son propre test.
- [ ] Tests de caractérisation ajoutés avant le refactor, verts après, non modifiés.
- [ ] Entrées `Sync this worktree`, `Refresh base branch` et `Sync all worktrees` dans
      le dashboard.
- [ ] Le débat `--keep-conflict` tranché et sa raison écrite dans `docs/dev/`.
- [ ] `internal/flow/` toujours sans import interdit ; subagent `build-validator` vert.
- [ ] `make docs` rejoué et `using-wtm.skill.md` mis à jour si la surface CLI bouge.
