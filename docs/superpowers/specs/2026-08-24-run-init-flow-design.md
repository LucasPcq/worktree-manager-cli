# `wtm run init` produit une configuration qui démarre

**Date** : 2026-08-24
**Statut** : brouillon — décisions ouvertes en fin de document
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

Les tasks (`build`, `lint`, `format`, `check-types`) restent **proposées mais
décochées**. Elles ne disparaissent pas du wizard — une migration ou un seed
sont de vrais jobs ponctuels — mais rien n'est écrit tant qu'on ne les coche
pas. Un job non coché n'existe pas : c'est ce qui supprime les dix entrées
mortes.

## Étape 2 — les ports

Pour chaque service retenu, et pour lui seul. Trois cas :

- la détection a trouvé (port compose, port `.env`) → pré-rempli, à confirmer ;
- elle n'a rien trouvé → demandé ;
- la commande a besoin d'un flag → la commande à écrire est montrée.

Le lot 1 ferme la boucle : `run up` dira si le port déclaré a réellement été
lié, donc l'étape n'a pas à promettre ce qu'elle ne peut pas garantir.

## Étape 4 — les profils

**Le groupement est une intention, pas une donnée.** `apps/api` et `apps/web`
sont deux apps ; `apps/app1/front` et `apps/app1/back` sont une seule. Même
structure de répertoires, groupement opposé. La détection propose, elle ne
décide pas.

**Proposition de départ** : un profil par package ayant un job service, plus un
profil réunissant tout. Dans un repo mono-package les deux se confondent — un
seul profil, sans cas particulier à écrire.

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

## Décisions restant à prendre

1. **`default = true` a-t-il encore un emploi ?** Le picker sortant toujours, il
   ne sert plus qu'à pré-sélectionner l'entrée du picker. Le garder à ce titre,
   ou le retirer du modèle ?
2. **Que fait l'étape « ports » quand la détection n'a rien trouvé ?** Demander
   une valeur de base, ou permettre de passer et laisser le job sans port
   déclaré (donc sans isolation, et signalé comme tel par le lot 1) ?
3. **La classification service/task doit-elle lire la commande** en plus du nom
   du script ? `web-preview` (`vite preview`) est classé task alors qu'il sert,
   `api-start` (`node dist/index.js`) est classé service alors qu'on ne le lance
   pas en dev. Le nom seul se trompe dans les deux sens.
4. **Un `run init` relancé** doit-il reproposer le découpage en profils, ou
   ne toucher qu'aux jobs et laisser les profils existants intacts ?
