# Questions ouvertes

Reprises du document de conception. Une question tranchée devient une ADR et sort d'ici
(licence → ADR 0012 ; pont Wails → ADR 0009 ; updates hors ligne d'un révoqué → ADR 0018).

| Question | Proposition actuelle | À trancher avant |
|---|---|---|
| Nom du format de dossier projet | `.inkwell/` (le document disait `.collab/`) | étape 2 |
| Rôle admin distinct de l'hôte initial | Oui : l'UI pousse à nommer un second admin au premier partage | étape 2 |
| Résumés automatiques de l'historique | Modèle local optionnel ou abandon | v2 |
| Tiptap en vue riche Markdown | v2 ou jamais | v2 |
| Taille maximale d'un blob et d'un projet | 25 Mo par fichier ; projet à fixer | étape 4 |
| Cloisonner les compilations (Landlock : système de fichiers et réseau, Linux ≥ 6.7) | Ré-exécutable d'amorçage ; aujourd'hui : proxys refusés, `--root`, parcours des liens, limites | étape 5 |
| Arrêt d'un sidecar quand Inkwell meurt sous macOS et Windows | Surveiller un tube / objet de tâche ; Linux a `Pdeathsig` | étape 6 |
| Thèmes Touying au-delà de `simple`, touying 0.8.0 | Avec C4 (présentations) | étape 3 |
