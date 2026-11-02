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

1. **L1 — Autoporteur (N42)** : `internal/sidecar` n'exécute plus `setpriv`, `prlimit`, `unshare`,
   `mount` ; appels système directs ou lanceur embarqué ; porte : contrôle d'architecture « aucun
   `os/exec` hors sidecars embarqués » + tests du confinement.
2. **L2 — Écrire et retrouver** : G-0a puis G-0b (ouvrir vraiment un fichier), puis onglets
   (G-0c, N19) ; porte : FM-OPEN-1…8, PERF-MEM sur ouverture réelle du corpus et PERF-TYPE natif WebKitGTK (frappe → peinture, froid/chaud).
3. **L3 — Comprendre, corriger, produire** : tinymist branché (N28, N29) ; porte : `problems.spec.ts`,
   `PERF-LSP`.
4. Puis : Collaborer sans perdre, Première minute et secours, Revenir en arrière, Fondations
   extensibles (`coherence.md` §5).

## Journal de pilotage

| Date | Décision |
|---|---|
| 2026-11-02 | Méthode HDAE adoptée ; on passe à la construction ; commentaires avec chaque fichier (AD-106) ; validation par la personne nommée (AD-107) |

### Preuves L0 — 2026-11-02

État vérifiable : `gates/L0.json` (critères pass/fail/deferred, reports séparés, raisons et lots cibles).
Références, limites de validité et seuils : `docs/perf/baseline/2026-11-02/README.md`
et `docs/perf/baseline/2026-11-02/thresholds.json`. Le registre N38 reste géré séparément.
Le démarrage mesure l'application actuelle sans plugins, processus froid du build serveur
jusqu'à l'éditeur Chromium utilisable ; navigateur pré-lancé, caches OS chauds : cette
référence locale ne certifie pas une fenêtre native. PERF-TYPE est un proxy Chromium censuré,
avec borne supérieure du p95, jamais une certification du p95 WebKitGTK natif.

| Date | Décision |
|---|---|
| 2026-11-02 — décision technique (boucle extérieure) | PERF-START : application actuelle sans plugins ; chargement des plugins différé au lot Fondations-extensibles (Fondations extensibles), jamais compté réussi. |
| 2026-11-02 — décision technique (boucle extérieure) | PERF-MEM : deferred: L2 ; ouverture réelle de plusieurs fichiers G-0 livrée en L2, ajoutée à sa porte. |
| 2026-11-02 — décision technique (boucle extérieure) | PERF-TYPE : référence proxy Chromium, avec réserve de validité ; frappe → peinture WebKitGTK natif ajoutée à la porte L2. |
| 2026-11-02 — décision technique (boucle extérieure) | PM-5 : Go, delta benchstat strictement > +5 % en temps ou allocs/op ET p < 0,05 ; latences e2e et mémoire : p95 de référence × 1,10. Seuils figés dans thresholds.json ; mémoire sans valeur tant que différée. regression-guard compare les nouvelles mesures ; L0 exige chargement et autotest. |
| 2026-11-02 — décision technique (boucle extérieure) | PERF-RELAY-ALLOC: 1 alloc/op vs 0 target — to fix in a later perf lot ; 48 B/op ; cible zéro inchangée, échec connu conservé. |
| 2026-11-02 — décision technique (boucle extérieure) | Critères pass, fail ou deferred ; deferred exige lot cible nommé et raison, liste séparée au manifeste. Tous les non-différés doivent passer ; autotests : report sans lot, sans raison et échec coexistant avec report. Un report ne masque jamais un échec. |
| 2026-11-02 — décision technique (boucle extérieure) | Annulation de la modification N38 dans exigences.md ; état du lot consultable via gates/L0.json ici, sans modifier l'état du registre. |
