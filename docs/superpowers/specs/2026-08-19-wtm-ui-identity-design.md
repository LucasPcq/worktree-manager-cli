# `wtm ui` — identité et panneau détail

Date : 2026-08-19
Statut : validé, prêt pour le plan d'implémentation
Périmètre : `internal/tui/dashboard`, `internal/styles`, `internal/service/worktree`,
`internal/rules`, `internal/domain`, plus une correction dans `internal/commands/wt/list.go`.

## 1. Le problème

Le dashboard fonctionne mais ne se distingue pas, et il en dit moins qu'il ne sait.

**Il ne dit pas où tu es.** Le header porte `wtm`, deux onglets et un compteur : ni le nom
du dépôt, ni la branche de base, ni le worktree dans lequel ton shell se trouve. Le tag
`● active` existe dans `internal/output/worktree.go` mais `internal/commands/wt/list.go:107`
passe `ActiveBranch: ""` en dur — **c'est du code mort, la notion n'existe nulle part dans
le produit.**

**Une seule couleur fait huit métiers.** `ColorPrimary` porte la règle d'onglet actif, les
titres de section, le nom de branche, les puces de l'arbre, les deux boutons du header, les
bordures de modale et les barres d'intro. Une couleur qui signale tout ne signale rien.

**Le panneau détail décrit un emplacement, pas un travail.** Six champs — Path, Parent,
Created, Base, Origin, PR — qui remplissent 15 lignes sur 37 : **plus de la moitié du
panneau est vide**. Rien sur le dernier commit, rien sur les fichiers modifiés, rien sur les
enfants, rien sur le drift `.env` que `service/env` sait pourtant calculer.

**Une donnée ment en silence.** Les badges de divergence origin ne fetchent jamais (décision
délibérée : seul `r` rafraîchit). `origin ↑2 ↓1` peut donc dater de trois jours sans que rien
ne l'indique — dans le détail comme dans chaque ligne de la liste.

## 2. Le registre et le principe directeur

Registre **sobre professionnel** : la hiérarchie vient de la couleur et du poids, pas de
l'ornement. Unicode seul, pas de nerd fonts, pas d'emoji.

Mais sobre mal exécuté donne terne, et un outil ouvert trente fois par jour doit donner
envie. D'où le principe qui arbitre tout le reste :

> **L'âme se dépense dans les moments rares. La surface de travail reste calme.**

Un ASCII art vu trente fois par jour devient un meuble, puis un obstacle. Vu une fois par
dépôt, c'est un accueil. Le caractère va donc là où l'utilisateur n'est pas en train de
travailler — et là, on est généreux.

## 3. La palette — duo signature

Deux teintes reconnaissables, dont une qu'on voit à peine mais qui réchauffe tout.

| Rôle | Usage | Clair | Sombre |
|---|---|---|---|
| **Accent navigation** | règle d'onglet actif, barre de ligne sélectionnée, nom de branche du détail, bordure de modale | `#5B4BE0` | `#9B8CFF` |
| **Accent signature** | **uniquement** le wordmark et le CTA `+ New worktree` | `#C2521F` | `#E8734A` |
| Succès | pastille `clean`, checks CI verts | `#158A4A` | `#4ADE80` |
| Warning | pastille `dirty` / `rebasing`, blockers | `#B07800` | `#F0C000` |
| Danger | actions destructrices, checks CI rouges | `#C21E28` | `#FA6E76` |
| Muted | structure : titres de section, étiquettes, règles, chips non-état | `#6F6F6F` | `#8D8D8D` |

**Contrainte à vérifier, pas à supposer.** L'accent signature (corail) et `warning` (or) sont
tous deux chauds. Ils ne doivent jamais être confondables : une pastille `dirty` et le bouton
`+ New worktree` sont à quelques centimètres l'un de l'autre. La validation des contrastes
**et de la non-confusion**, dans les deux thèmes, est une étape explicite du plan
d'implémentation. Les hex ci-dessus sont un point de départ, pas un acquis.

Tout reste en `lipgloss.AdaptiveColor` comme aujourd'hui.

## 4. La discipline de couleur — trois rôles, un propriétaire chacun

| Rôle | Couleur | Couvre |
|---|---|---|
| Navigation & sélection | accent navigation | règle d'onglet actif, barre d'accent de la ligne sélectionnée, nom de branche du détail, bordure de modale |
| Identité & appel à l'action | accent signature | wordmark, `+ New worktree` |
| État des choses | succès / warning / danger | pastilles, ligne de blockers, checks CI, divergence anormale |
| Structure | muted | titres de section, étiquettes, règles, chips non-état, puces de l'arbre |

Trois changements concrets dans `internal/styles/dashboard.go` :

- `DashboardSectionTitle` : accent+gras → **muted+gras**. `REVIEW` et `CHANGES` sont des
  séparateurs, pas des accents ; ils captaient un regard qu'ils ne méritaient pas. C'est le
  plus gros gain.
- `DashboardHeaderButton` (`⋯ Actions`) : accent → **muted+gras**, pour que
  `+ New worktree` soit le seul appel de la barre. Le commentaire déjà présent dans
  `styles/dashboard.go` dit vouloir exactement ça ; on va au bout.
- Puces de l'arbre : accent → muted, et c'est **l'état du nœud** qui les colore quand il en
  a un.

Risque assumé : sans accent, les sections peuvent paraître plates. Elles gardent le gras et
la ligne vide au-dessus, et le panneau **gagne** de la couleur d'état dans la bande vitale,
qu'il n'avait pas du tout. Le contraste net monte.

## 5. Le header

> **Révisé après essai sur l'UI construite.** Le header à 3 lignes décrit plus bas
> a été jugé « trop petit, trop collé aux onglets, on ne le lit pas comme un
> header ». Il devient un **bloc signature de 6 lignes** au-dessus d'un seuil de
> hauteur de terminal, et garde la forme à 3 lignes en dessous. Le wordmark
> dessiné, écarté au §9 faute d'un moment d'accueil qui existe, revient ici comme
> ancrage permanent — et il **porte** le contexte au lieu de s'y ajouter, ce qui
> règle du même coup la surcharge de la ligne unique.

**Forme haute (≥ `DashboardHeaderTallThreshold` lignes de terminal) :**

```
  ╻ ╻ ╺┳╸ ┏┳┓   worktree-manager-cli
  ┃╻┃  ┃  ┃┃┃   base main · ● feat/ui/improve-design
  ┗┻┛  ╹  ╹ ╹   4 worktrees · fetched 3 d ago

   Worktrees    Tree                    + New worktree    ⋯ Actions
  ━━━━━━━━━━━━━━────────────────────────────────────────────────────
```

La ligne vide avant les onglets n'est pas optionnelle : c'est elle qui fait lire
l'ensemble comme un header. Le bloc n'a pas de variante partielle — la mise en page
ne le sélectionne que si ses six lignes tiennent.

**Forme compacte (en dessous du seuil) :**

```
 wtm  worktree-manager-cli · base main · ● feat/ui/improve-design      fetched 3 d ago
  Worktrees    Tree                             + New worktree   ⋯ Actions   3 worktrees
 ━━━━━━━━━━━━━━━───────────────────────────────────────────────────────────────────────
```

**Ligne 1 = où tu es. Ligne 2 = ce que tu peux faire.** L'articulation rend le repli
trivial : la ligne 1 lâche ses segments de droite à gauche (`fetched` → worktree actif →
base → dépôt), la ligne 2 garde la mécanique de variantes existante de `headerRight`.

Pas de règle intermédiaire entre les deux lignes : la règle d'onglet en dessous fait déjà le
travail de séparation. Sous une contrainte de hauteur extrême, la forme compacte lâche ses
lignes par le bas (la règle, puis la barre) plutôt que d'émettre un nombre fixe de lignes.

**Le worktree actif** se déduit d'un match de préfixe entre le cwd et les `Path` déjà en
mémoire — aucun appel git. Il est marqué `●` dans le header et dans la ligne de liste
correspondante.

**`fetched … ago`** est le seul élément non permanent du header : il n'apparaît qu'au-delà
d'un seuil (~24 h). C'est la propriété de **toute la vue**, pas du worktree sélectionné :
tous les badges origin de la liste ont le même âge. Il est placé à côté de ce qui le répare
(`r refresh`, dans la barre d'aide juste en dessous) et s'efface quand on appuie sur `r`.

**Correction associée :** `internal/commands/wt/list.go:107` passe `ActiveBranch: ""` en dur.
On le renseigne pour que le `● active` de `wtm list` cesse d'être du code mort.

## 6. Le panneau détail

```
╭────────────────────────────────────────────────────────────────────────────────────╮
│ Detail                                                              ⠋ refreshing   │
│                                                                                    │
│ feat/ui/improve-design                                    ● you are here           │
│ ────────────────────────────────────────────────────────────────────────────────── │
│                                                                                    │
│ dirty  ·  base ↑3  ·  origin ↑2 ↓1  ·  active 3 h ago                              │
│                                                                                    │
│ ⚠ cannot be deleted — uncommitted changes · 2 unpushed commits                      │
│                                                                                    │
│ REVIEW                                                                             │
│                                                                                    │
│ #67  feat(ui): improve dashboard design                                       OPEN │
│ checks ✓ 12  ✗ 1  ·  review  changes requested                                     │
│                                                                                    │
│ CHANGES                            4 modified · 2 untracked · 1 staged   +214 −38  │
│                                                                                    │
│   M  internal/tui/dashboard/detail.go                                              │
│   M  internal/styles/dashboard.go                                                  │
│   ?  internal/service/worktree/detail.go                                           │
│   …  4 more                                                                        │
│                                                                                    │
│ ACTIVITY                                                                           │
│                                                                                    │
│   a3f91c2  feat(ui): sections conditionnelles dans le detail    Lucas · 3 h ago    │
│   8b21ef0  refactor(styles): accent réservé à la navigation     Lucas · 5 h ago    │
│                                                                                    │
│ LINKS                                                                              │
│                                                                                    │
│ Parent    main                                                                     │
│ Children  chore/deps-bump                                                          │
│ Created   2 d ago                                                                  │
│ Env       2 keys missing                                                           │
│ Path      ~/Dev/wtm.worktrees/feat-ui-improve-design                               │
╰────────────────────────────────────────────────────────────────────────────────────╯
```

### Les cinq règles

**1. La ligne de titre porte l'identité, la bande porte l'état.** C'est pour ça que la
pastille `dirty` n'est plus sur la ligne du nom : c'est une constante vitale, exactement
comme `base ↑3`. La ligne de titre ne garde que le nom de branche et `● you are here`
(formulation choisie contre `active`, qui entrerait en collision avec `active 3 h ago` trois
lignes plus bas).

**2. La bande vitale n'a pas d'étiquettes** et **la pastille d'état est la seule chose
colorée** de la bande. Elle est construite depuis `WorktreeStatus`, déjà chargé, donc elle
est **toujours instantanée** — c'est elle qui tient l'écran pendant que le reste charge.
`DIVERGENCE` disparaît comme section ; `Created` descend dans `LINKS`.
La bande **se replie chip par chip** sur un panneau étroit, jamais en coupant un chip au
milieu : un `origin ↑2 ↓…` tronqué serait un mensonge, pas une troncature.

**3. Une section n'apparaît que si elle a quelque chose à dire.** Pas de PR → pas de
`REVIEW`. Worktree propre → pas de `CHANGES`. Pas d'enfants → pas de ligne. Fini `none`,
`—`, `up to date`. Corollaire assumé : **la position d'une section n'est pas stable d'un
worktree à l'autre.** C'est le prix, et il est bon.

**4. Les blockers sont hauts, et ne sont pas une section.** Une ligne juste sous la bande
vitale, seulement quand il y en a. C'est la réponse à « pourquoi le menu me refuse cette
action » : elle doit être lue avant d'ouvrir le menu, pas trouvée en scrollant.

**5. Les deux listes se partagent la hauteur restante, priorité selon l'état.** Un worktree
sale donne la place aux fichiers (ce sur quoi tu travailles), un worktree propre la donne aux
commits (ce que la branche *est*). Chacune se coupe sur `… N more`. Sur un terminal court,
les sections tombent **par le bas** dans l'ordre `LINKS → ACTIVITY → CHANGES → REVIEW` ; la
bande vitale et la ligne de blockers ne tombent jamais.

## 7. La couche de données

Un type dans `domain/`, une fonction dans `service/worktree/`, un `tea.Cmd` dans le
dashboard.

```go
// internal/domain/worktree.go (ou domain/detail.go)
type WorktreeDetail struct {
    Branch   string
    Commits  []CommitSummary // le plus récent en tête ; SHA court, sujet, auteur, date
    Changes  WorkingChanges  // Modified, Untracked, Staged, Insertions, Deletions, Files
    Children []string
    Blockers []CleanBlocker
    EnvDrift EnvDriftSummary // Missing, Conflicting, Orphan (compteurs)

    // Failures nomme les familles qui n'ont pas pu être lues, avec la raison.
    // Une famille absente de la map a été lue correctement — vide y compris.
    // C'est ce qui sépare l'absence légitime de la panne (§8, état 4).
    Failures map[DetailFamily]error
}
```

`DetailFamily` énumère `commits`, `changes`, `env`, `blockers`. Il n'y a **pas** de champ
`LastCommit` : le dernier commit est `Commits[0]`, et c'est lui qui alimente `active … ago`
dans la bande vitale. `DashboardDetailCommits` (constante) fixe combien on en demande.

`service/worktree.Detail(DetailParams)` l'assemble : un `git log`, un `git status --porcelain`
(via `ListModifiedFiles`, déjà présent), un `git diff --shortstat`, `infra.UnpushedCommits`,
`env.ComputeEnvDiff`.

**Les blockers ne coûtent aucun appel réseau.** `worktree.Check` appelle `ghservice.HasOpenPR`
(un `gh`, lent) ; le dashboard n'en a pas besoin, il a déjà `m.prs` en mémoire. Il compose donc
lui-même le `domain.CleanCheckResult` depuis `WorktreeStatus.IsDirty` + `UnpushedCommits` + les
PR déjà chargées, puis appelle la règle pure `rules.CleanBlockers`. Idem pour `Children`, qui
sort de `m.parents` : ces deux champs sont passés en entrée à `Detail`, la classification reste
dans `rules/`.

**Le chargement — trois règles qui protègent la réactivité :**

1. **Jamais dans le poll 3 s.** Le poll continue de ne rafraîchir que la liste. `Detail` ne se
   déclenche que sur changement de sélection et sur `r`.
2. **Débounce ~150 ms** — sinon un `G` qui traverse vingt worktrees lance vingt `git log`.
3. **Cache par branche, invalidé par le poll et par la fin d'une opération.** Le détail
   affiché reste à l'écran pendant le rechargement.

**Portée volontaire :** `service/worktree.Detail` est réutilisable par `wtm list --output json`
et par un futur `wtm show`. On ne le fait pas maintenant, mais la signature est posée pour que
ce soit possible sans réécriture — d'où une fonction de service exportée plutôt qu'un helper
privé du dashboard.

## 8. Le contrat de fraîcheur

Du périmé qui ne se déclare pas se lit comme de la vérité ; du vide qui ne s'explique pas se
lit comme une panne. **Quatre états, chacun avec son signe.**

**1. Frais** → rien. Aucun ornement. L'état normal ne se signale pas.

**2. En rafraîchissement, avec de l'ancien à l'écran** → le contenu **ne bouge pas d'un
pixel** (même hauteur, même texte, aucun reflow), le titre du panneau porte `⠋ refreshing` à
droite, et le corps passe en muted. Ni erreur, ni vérité : « vrai il y a une seconde ».

**3. Premier affichage, rien en cache** → la bande vitale s'affiche immédiatement (elle vient
de `WorktreeStatus`) ; seules les sections dépendant de `Detail` portent un placeholder **de
la bonne hauteur** (`⠋ loading…`). C'est le seul endroit où on réserve de la place pour du
vide, et c'est justifié : sinon le panneau saute quand la donnée arrive.

**4. Indisponible** → visuellement distinct de « en attente », et **il dit pourquoi**. La
distinction qui compte n'est pas succès/échec mais **absence légitime vs panne** :

```
Env      not configured            ← absence légitime, muted, pas d'alarme
Env      unavailable — git error   ← panne, muted + ⚠, on ne fait pas semblant
```

Une section ne devient jamais vide en silence. C'est ce que le `Err` par famille permet.

**Le délai d'apparition (~200 ms).** Un `git log` local répond en 30 ms ; faire apparaître
puis disparaître un spinner en 30 ms, c'est du flash déguisé en feedback. Le marqueur
n'apparaît que si l'attente devient perceptible. Même principe que le seuil de 24 h sur
`fetched … ago` : **un marqueur affiché en permanence ne signale plus rien.**

Le spinner est celui du projet — `spinner.MiniDot` en muted, `newMutedSpinner` dans
`internal/tui/components/loading.go`, à exporter pour que le dashboard le réutilise plutôt
que d'en inventer un.

## 9. Les moments

**Les états vides nomment l'action suivante** au lieu de constater :

```
No worktrees.              →  No worktrees yet — press n to create your first one.
No worktree selected.      →  Select a worktree to see what's in it.
No operation output yet.   →  Output from create, clean and sync runs appears here.
```

**L'opération en cours est visible là où elle agit.** `ops.go` connaît déjà la cible qu'une
op en background verrouille (`ModeBackground` + `TargetKey`) et la liste n'en montre rien. La
ligne du worktree verrouillé remplace sa pastille d'état par le spinner et sa ligne meta par
l'étape en cours (`creating worktree…`, `running on_create hook…`). Même vocabulaire que le
contrat de fraîcheur : le spinner dit « en cours », jamais « cassé ».

**Le wordmark dessiné**, à exactement deux endroits — tous deux déjà vides aujourd'hui :

```
   ╻ ╻╺┳╸┏┳┓
   ┃╻┃ ┃ ┃┃┃      worktree manager
   ┗┻┛ ╹ ╹ ╹
```

- **Le premier lancement** (dépôt sans worktree), au-dessus des trois lignes qui expliquent
  ce qu'est un worktree wtm et quelle touche en crée un. C'est le moment où l'identité se
  transmet.
- **Le chargement initial** — et c'est le point important : cette attente **existe déjà**
  (`loading: true`, liste et PR arrivent en async). On n'ajoute pas un splash qui retarde
  l'outil, on **habite une attente déjà présente**. Coût réel : zéro milliseconde.

**Trois animations, pas une de plus :**

- La règle d'onglet **glisse** au changement d'onglet au lieu de sauter (~200 ms).
- La ligne d'un worktree qui vient d'être créé **s'allume puis s'éteint** (~400 ms). C'est le
  moment de fin d'opération ; en fondu il est élégant là où un toast serait lourd.
- Le dégradé du wordmark **respire** lentement, **uniquement** dans l'écran d'accueil vide —
  jamais dans le header.

**Trois règles non négociables :** aucune animation ne dépasse ~400 ms ; aucune ne retarde
une action ni n'avale une touche ; toutes sont coupables d'un coup via `ui.animations = false`
en config (plus le respect de `NO_COLOR`, que lipgloss assure déjà). Une animation qu'on ne
peut pas éteindre est un bug en ssh sur une liaison lente.

## 10. Hors périmètre

- **La refonte du layout (« B »)** — liste à 30 %, détail sur deux colonnes au-delà de ~140
  colonnes. Décidé après cette passe : c'est un ajout à `rules.ComputeDashboardLayout`, pas
  une refonte, donc rien ne se perd à le faire ensuite.
- **Les services `run` dans le détail** — écartés explicitement : le module est encore trop
  instable pour qu'on s'appuie dessus dans l'UI.
- **Un sous-onglet dans le détail** (Overview / Changes / Commits / PR) — écarté : un niveau
  de navigation de plus dans un panneau déjà sous des onglets, pour un gain que les sections
  conditionnelles donnent gratuitement.
- **Splash artificiel, nerd fonts, emoji, ornement sur la surface de travail.**

## 11. Fichiers touchés

| Fichier | Nature |
|---|---|
| `internal/styles/colors.go` | palette duo signature |
| `internal/styles/dashboard.go` | discipline à 3 rôles, styles de bande vitale / blockers / wordmark |
| `internal/domain/worktree.go` (ou `detail.go`) | `WorktreeDetail`, `CommitSummary`, `WorkingChanges`, `EnvDriftSummary` |
| `internal/domain/constants.go` | libellés, seuils (24 h, 200 ms, 150 ms), ordre de repli des sections |
| `internal/domain/dashboard.go` | `DashboardHeaderHeight` 2 → 3 |
| `internal/rules/` | visibilité et ordre de repli des sections, seuil de péremption, composition des chips, worktree actif par préfixe de cwd — **pur, testable sans git** |
| `internal/infra/status.go`, `branch.go` | compteurs porcelain, `--shortstat`, dernier commit, mtime de `FETCH_HEAD` |
| `internal/service/worktree/detail.go` | `Detail(DetailParams)` |
| `internal/tui/dashboard/` | `detail.go` (réécriture du corps), `render.go` (header), `list.go` (marqueur actif, spinner d'op), `dashboard.go` (cmd, débounce, cache) |
| `internal/tui/components/loading.go` | exporter le spinner standard |
| `internal/commands/wt/list.go` | `ActiveBranch` renseigné |
| `internal/config/` | clé `ui.animations` |

## 12. Tests

- **`rules/`** — l'essentiel de la logique y vit et se teste sans git : quelles sections sont
  visibles pour un état donné, l'ordre de repli à hauteur contrainte, le repli chip par chip
  de la bande, le seuil de péremption, la détection du worktree actif par préfixe de cwd, le
  partage de hauteur entre `CHANGES` et `ACTIVITY` selon l'état.
- **`service/worktree.Detail`** — sur dépôts git de test, dans le style des
  `worktree_test.go` existants, avec le cas « une famille échoue, les autres passent ».
- **`tui/dashboard`** — rendu, dans le style de `style_test.go` / `dashboard_test.go` : les
  quatre états de fraîcheur, un détail sans PR (section absente), un worktree propre
  (`CHANGES` absent), un terminal court (ordre de repli), le marqueur actif.
- **Validation manuelle des couleurs** — contraste **et non-confusion** signature/warning,
  en thème clair et sombre. Étape explicite, pas une hypothèse.

## 13. Risques

| Risque | Traitement |
|---|---|
| Signature corail et `warning` or confondues | validation explicite dans les deux thèmes, avant de figer les hex |
| Sections mobiles d'un worktree à l'autre — désorientation | ordre de priorité **fixe** ; seule la présence varie, jamais l'ordre |
| Header à 3 lignes sur terminal court | repli par segments ; la ligne 2 garde sa mécanique de variantes |
| `Detail` qui rame sur un gros dépôt | débounce, cache, jamais dans le poll, `Err` par famille non bloquant |
| Animations pénibles en ssh | `ui.animations = false`, plafond 400 ms, jamais bloquantes |
