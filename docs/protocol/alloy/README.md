# Modèles Alloy

Modèles structurels (relations, droits) vérifiés par Alloy 6.2 (`mise run alloy`, jar épinglé par
SHA-256, outil de développement jamais livré). Chaque `check` doit être sans contre-exemple, chaque
`run` satisfiable, et chaque **mutant** listé dans `scripts/alloy_check.py` doit faire tomber son
assertion (preuve que l'assertion contraint la règle).

| Modèle | Source | Propriétés vérifiées (portée 6) |
|---|---|---|
| `Decisions.als` | ADR 0030 §2 (A4, D1) | `NoEscalation` : une clé jamais promue admin ne fait tenir aucune décision ; `AntedatedStaysDead` : un admin retiré qui antidate reste mort ; `SeenByRevokerStands` : ce que le révocateur avait vu tient ; `RevocationEffective` : une coupe connue empêche toute nouvelle décision de la clé coupée ; `MutualCut` (exploration) : deux admins qui se coupent en même temps |

Abstraction : le pli de la liste de membres (ADR 0017) est réduit à « promu et non coupé à la
frontière » ; il est vérifié ailleurs (vecteurs `fold.json`). Ce modèle vérifie l'interaction entre
décisions et révocations concurrentes. À venir : registre D2 (le refus gagne), arborescence de
l'index (cycles, homonymes) et suppression sur accord (ADR 0023).
