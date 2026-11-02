# Objectif en cours

Une seule page, tenue à jour à chaque lot. Méthode : développement agentique dirigé par l'humain
(boucle extérieure humaine : vision, portes, jugement, arrêt ; boucle intérieure de l'agent :
coder, construire, lancer, mesurer, corriger, jusqu'à ce que la porte passe ou qu'une décision
humaine soit nécessaire). Référence : `docs/plans/2026-11-02-methode-hdae.md`.

Les spécifications sont acceptées (design rév. 7 + 7.1, architecture rév. 8.1 + 8.2) ; on ne les
change plus que par une décision consignée dans `docs/architecture/decisions.md` ou
`docs/design/decisions-utilisatrice.md`.

## Lot actif : L0 — Portes et mesure de référence

| | |
|---|---|
| But | chaque lot suivant se termine par une porte qu'une machine vérifie ; les budgets de performance deviennent des seuils |
| Exigences | N38 ; `docs/architecture/coherence.md` §5, lot « Mesure de référence » |
| Porte | `mise run gate -- L0` réussit et écrit `gates/L0.json` (liste réussi / échoué, versions des outils, empreintes des résultats bruts) |
| Contenu | (1) harnais `scripts/gate.sh` + tâche mise `gate` ; (2) mesures de référence `PERF-TYPE`, `PERF-START`, `PERF-MEM`, bancs Go des `HOT_PATHS` ; résultats bruts archivés sous `docs/perf/baseline/` avec empreintes SHA-256 |
| Condition d'arrêt | porte verte, ou décision humaine requise (budget irréaliste, mesure impossible sur cette machine) |
| Échecs connus | F10 (panique libp2p intermittente), F11 (quota `/tmp` du test sidecar), F12 (générateur de vecteurs ≠ corpus) — listés dans le manifeste, jamais masqués |

## Lots suivants (MVP, dans l'ordre)

1. **L1 — Autoporteur (N42)** — inclut aussi, côté portes : un contrôle qui échoue si un test, un
   seuil ou un délai est affaibli dans le diff (idée gstack, `docs/plans/2026-11-02-outils-externes.md`) ; `internal/sidecar` n'exécute plus `setpriv`, `prlimit`, `unshare`,
   `mount` ; appels système directs ou lanceur embarqué ; porte : contrôle d'architecture « aucun
   `os/exec` hors sidecars embarqués » + tests du confinement.
2. **L2 — Écrire et retrouver** : G-0a puis G-0b (ouvrir vraiment un fichier), puis onglets
   (G-0c, N19) ; porte : FM-OPEN-1…8.
3. **L3 — Comprendre, corriger, produire** : tinymist branché (N28, N29) ; porte : `problems.spec.ts`,
   `PERF-LSP`.
4. Puis : Collaborer sans perdre, Première minute et secours, Revenir en arrière, Fondations
   extensibles (`coherence.md` §5).

## Journal de pilotage

| Date | Décision |
|---|---|
| 2026-11-02 | Idées reprises de 10 dépôts externes, rien d'installé (`docs/plans/2026-11-02-outils-externes.md`) : discipline de modification dans CLAUDE.md, garde anti-affaiblissement des portes, check-list anti-tics en français, notation 0–10 et trois largeurs dans `ux-walkthrough` |
| 2026-11-02 | Méthode HDAE adoptée ; on passe à la construction ; commentaires avec chaque fichier (AD-106) ; validation par la personne nommée (AD-107) |
