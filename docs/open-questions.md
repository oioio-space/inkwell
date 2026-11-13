# Questions ouvertes

Reprises du document de conception. Une question tranchée devient une ADR et sort d'ici
(licence → ADR 0012 ; pont Wails → ADR 0009 ; updates hors ligne d'un révoqué → ADR 0018).

| Question | Proposition actuelle | À trancher avant |
|---|---|---|
| Rôle admin distinct de l'hôte initial | Oui : l'UI pousse à nommer un second admin au premier partage | étape 2 |
| Taille maximale d'un blob et d'un projet | 25 Mo par fichier ; projet à fixer | étape 4 |
| Arrêt d'un sidecar quand Inkwell meurt sous macOS et Windows | Surveiller un tube / objet de tâche ; Linux a `Pdeathsig` | étape 6 |
| Thèmes Touying au-delà de `simple`, touying 0.8.0 | Avec C4 (présentations) | étape 3 |
| Q1 — « peut-être » compte pour 1 ou 2 mots ? (compteur de mots) | 1 mot (comme Word et LibreOffice), à confirmer | avant le premier tag |
| Q2 — Garder « Page X sur Y » dans la barre d'état ? | Sans réponse ; gardé (C2 : « Page X sur Y » dans la barre d'état, quand l'aperçu est affiché) | avant le premier tag |

## Questions tranchées

Fermées, avec leur décision ; une question tranchée par ADR sort de la liste ci-dessus.

| Question | Décision |
|---|---|
| Nom du format de dossier projet | `.inkwell/` : [AD-11 et AD-12](architecture/decisions.md), code `internal/project/names.go` |
| Résumés automatiques de l'historique ; Tiptap en vue riche Markdown | Reportés en v2 : non-objectif NG10 de [architecture/decisions.md](architecture/decisions.md) |
| Cloisonner les compilations (Landlock) | Fait : [ADR 0033](adr/0033-sidecar-directory-bound.md), AD-28 de [architecture/decisions.md](architecture/decisions.md), amendé par [ADR 0042](adr/0042-sandbox-memory-scope-and-no-core-helper.md) |
