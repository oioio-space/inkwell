# Questions ouvertes

Reprises du document de conception. Une question tranchée devient une ADR et sort d'ici.

| Question | Proposition actuelle | À trancher avant |
|---|---|---|
| Licence de l'application | MPL-2.0 (libre, SaaS fermé possible) ou AGPL (l'empêche) ; toutes les dépendances sont MIT/Apache/BSD | publication du dépôt |
| Nom du format de dossier projet | `.inkwell/` (le document disait `.collab/`) | étape 2 |
| Rôle admin distinct de l'hôte initial | Oui : l'UI pousse à nommer un second admin au premier partage | étape 2 |
| Updates hors ligne d'un auteur révoqué entre-temps | Accepter si causalement antérieures à la révocation, signaler sinon | étape 2 |
| Pont Wails : bindings ou WebSocket local dès le départ ? | Mesurer le débit à l'étape 1 | étape 1 |
| Résumés automatiques de l'historique | Modèle local optionnel ou abandon | v2 |
| Tiptap en vue riche Markdown | v2 ou jamais | v2 |
| Taille maximale d'un blob et d'un projet | 25 Mo par fichier ; projet à fixer | étape 4 |
