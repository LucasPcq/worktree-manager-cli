# `wtm run init` produit une configuration qui démarre

**Date** : 2026-08-24
**Statut** : design validé, prêt à planifier
**Lot** : 2/2 (le lot 1, la vérification des ports, est livré)

## Le problème, mesuré

Repo neuf, un `docker-compose.yml` et quatre scripts (`dev`, `build`, `lint`,
`start`). `wtm run init --non-interactive` puis `wtm run up` :

```
› [1/5] docker-compose
```

Cinq jobs démarrés. L'init n'a créé **aucun profil**, et sans profil `run up`
lance tout : le serveur de dev, mais aussi `build`, `lint`, et `start` — le
serveur de production, en concurrence avec le dev sur le même port.

Sur un monorepo réel (2 apps × front/back/backoffice) c'est pire : l'init a
produit **10 jobs, aucun dans un profil**, dont un job racine `dev` =
`turbo run dev` qui refait tourner les jobs par package et se bat avec eux sur
les mêmes ports.

Le diagnostic n'est pas « le flow est en plusieurs étapes ». C'est :

> **`run init` produit un inventaire, pas une configuration.**

Il répond « qu'y a-t-il dans ce repo ? », alors que la seule question utile est
« qu'est-ce que je veux voir tourner quand j'ouvre un worktree ? ». Tout le
travail fait ensuite à la main — trier les jobs morts, déclarer les ports,
composer un profil — c'est l'utilisateur qui convertit l'inventaire en
configuration.

## Le principe

L'init ne cherche **pas** à tomber juste du premier coup : c'est un assistant
qui propose et qu'on corrige, pas un devin. Ce qui change, c'est l'ordre des
questions et ce qui est écrit :

> **On demande l'intention d'abord, et rien de non coché n'est écrit.**

## Le flow cible

```
1. Quoi démarrer ?     compose + scripts service cochés, tasks décochées
                       → seul le coché devient un job

2. Ports               pour chaque service retenu : pré-rempli par la détection,
                       demandé quand elle n'a rien trouvé

3. Réécriture compose  ports littéraux + noms absolus (livré, une seule question)

4. Profils             découpage proposé, éditable : renommer · fusionner ·
                       retirer · nouveau

5. Liens .env          confirmation existante
```

## Étape 1 — l'intention

Tout est proposé, **le moins possible est coché** : seuls les scripts dont le nom
contient `dev`. Le reste — `build`, `lint`, `format`, `check-types`, mais aussi
`start`, `serve`, `watch`, `preview` — est visible et décoché. Un job non coché
n'existe pas : c'est ce qui supprime les dix entrées mortes.

Cette règle est volontairement bête. Lire la *commande* pour deviner si
`vite preview` sert des requêtes reviendrait à reconstruire une heuristique par
outil, à la maintenir et à la voir vieillir — exactement ce que le lot 1 a
refusé de faire pour turbo. Le nom suffit à pré-cocher ; l'utilisateur tranche
le reste.

**Conséquence à traiter : le `kind` de ce qu'on coche à la main.** `kind`
(`service` / `task`) ne sert pas qu'à cocher — il décide si le job **bloque le
profil**. `ClassifyScriptKind` le déduit du nom, donc `preview` est classé
`task` : coché à la main, il ferait attendre `run up` indéfiniment sur un
serveur. Un script coché hors des `dev` doit donc pouvoir voir son `kind` fixé
dans le wizard, avec la classification par le nom comme valeur de départ.

## Étape 2 — les ports

Pour chaque service retenu, et **seulement quand la détection a trouvé quelque
chose** (port compose, port `.env`) : le port est pré-rempli, à confirmer ou à
corriger. Si la commande a besoin d'un flag pour le lire, la commande à écrire
est montrée.

**Quand la détection n'a rien trouvé, on ne demande rien.** Poser la question
reviendrait à déplacer la devinette sur l'utilisateur, et une mauvaise réponse
est pire qu'une absence de réponse : elle déclare une isolation qui n'existe
pas.

**Conséquence à traiter** : un service sans port déclaré n'a aucune isolation,
et le lot 1 restera muet puisqu'il n'y a rien à sonder. Le recap de l'init doit
donc les nommer — pas une question, une ligne de constat :

```
Services sans port déclaré — deux worktrees lieront le même
  web-dev · aucun port détecté dans apps/web
```

Le lot 1 ferme la boucle pour tous les autres : `run up` dira si le port déclaré
a réellement été lié, donc l'étape n'a pas à promettre ce qu'elle ne peut pas
garantir.

## Étape 4 — les profils

**Le groupement est une intention, pas une donnée.** `apps/api` et `apps/web`
sont deux apps ; `apps/app1/front` et `apps/app1/back` sont une seule. Même
structure de répertoires, groupement opposé. La détection propose, elle ne
décide pas.

**Proposition de départ**, sur un repo vierge : un profil par package ayant un
job service, plus un profil réunissant tout. Dans un repo mono-package les deux
se confondent — un seul profil, sans cas particulier à écrire.

**Sur un `run init` relancé, la proposition est la configuration déjà en
place** : les profils existants sont affichés tels quels et servent de point de
départ à l'édition. L'init n'infère pas un découpage par-dessus un découpage que
l'utilisateur a déjà composé ; il montre ce qu'il y a et laisse décider. C'est
aussi ce qui protège les modifications faites à la main dans `run.toml`.

**Infra partagée** : les jobs dont le `cwd` est la racine (donc les jobs
compose) entrent dans chaque profil. C'est ce qui donne « je lance juste l'api
de app1, avec son docker ».

**Édition** : renommer, fusionner, retirer, créer. La fusion est l'opération qui
porte le cas monorepo réel — six profils proposés, deux fusions, et on a `app1`
et `app2`. Aucun composant du wizard ne sait faire ça aujourd'hui : c'est le
gros du travail TUI de ce lot.

## `run up` sans argument

**Le picker sort systématiquement** dès qu'il y a plus d'un profil, y compris
quand l'un est marqué `default`. Décision prise : un défaut ne fonctionne que
pour les apps simples qui démarrent toujours la même chose, ce qui n'est pas le
cas courant sur un monorepo.

`default = true` **reste dans le modèle**, avec un rôle réduit et honnête : il
pré-sélectionne l'entrée du picker. C'est le comportement actuel de
`pickProfile`, qui devient sa seule raison d'être.

Le fallback actuel « zéro profil → lance tous les jobs » est traité dans le même
lot : c'est lui qui lance le linter. L'init écrivant désormais toujours au moins
un profil, ce chemin ne concerne plus que les `run.toml` écrits à la main.

## Contrainte de migration

**Aucune.** Le module `run` est expérimental et n'a qu'un utilisateur. On vise
le meilleur flow, pas la compatibilité. Un `run init` relancé doit néanmoins ne
pas détruire ce qui a été écrit à la main dans `run.toml`.

## Découpage en couches

| Couche | Contenu |
| --- | --- |
| `internal/rules/` | **pur** : proposition de découpage en profils, classification des scripts, ce qui compte comme infra partagée |
| `internal/service/detect/` | inchangé pour l'essentiel ; lit déjà les workspaces |
| `internal/tui/components/` | le composant d'édition de listes groupées (fusion incluse) |
| `internal/tui/inittui/` | l'enchaînement des étapes |
| `internal/commands/run/init.go` | câblage |

## Critères d'acceptation

1. Sur un repo neuf à un `docker-compose.yml` et quatre scripts
   (`dev`, `build`, `lint`, `start`), `run init` non interactif écrit **deux
   jobs** (compose + `dev`) et **un profil**, et `run up` démarre ces deux-là et
   rien d'autre — plus jamais `[1/5]`.
2. Sur le monorepo d'exemple, l'init ne produit plus de job racine `dev`
   concurrent des jobs par package sans qu'il ait été coché.
3. Un `run init` relancé sur une configuration existante affiche les profils en
   place et ne les remplace pas sans décision explicite.
4. Un service retenu sans port détecté apparaît dans le recap sous « services
   sans port déclaré », et l'init ne pose aucune question de port pour lui.
5. Un script coché hors des `dev` peut voir son `kind` fixé dans le wizard, de
   sorte que cocher `preview` ne fasse pas attendre `run up` indéfiniment.

## Tests

- **`rules`** : pré-cochage (seuls les noms contenant `dev`), proposition de
  découpage en profils (mono-package → un seul profil ; multi-package → un par
  package plus le global), ce qui compte comme infra partagée (`cwd` racine),
  et la liste des services sans port déclaré.
- **`tui/components`** : le composant d'édition groupée — renommer, fusionner,
  retirer, créer — testé sur ses transitions, sans rendu.
- **`tui/inittui`** : l'enchaînement des étapes, et le fait qu'un job non coché
  n'entre pas dans la configuration produite.
- **`commands/run`** : un init non interactif de bout en bout sur les deux repos
  de référence, comparé au `run.toml` attendu.
