# Objectif en cours

Une seule page, tenue à jour à chaque lot. Méthode : développement agentique dirigé par l'humain
(boucle extérieure humaine : vision, portes, jugement, arrêt ; boucle intérieure de l'agent :
coder, construire, lancer, mesurer, corriger, jusqu'à ce que la porte passe ou qu'une décision
humaine soit nécessaire). Référence : `docs/plans/2026-11-02-methode-hdae.md`.

Les spécifications sont acceptées (design rév. 7 + 7.1, architecture rév. 8.1 + 8.2) ; on ne les
change plus que par une décision consignée dans `docs/architecture/decisions.md` ou
`docs/design/decisions-utilisatrice.md`.

## Lot actif : L1 — Autoporteur (N42)

| | |
|---|---|
| But | aucun programme externe lancé à l'exécution (seuls nos sidecars embarqués et vérifiés) ; aucune porte ne peut être affaiblie en douce |
| Exigences | N42 ; architecture §9.4–§9.11, INV-14, AC-19, §14.1 « Dépendances garanties » |
| Porte | `mise run gate -- L1` : (1) contrôle d'architecture « aucun `os/exec` hors sidecars embarqués » (exemption documentée des processus internes de WebKitGTK) ; (2) `internal/sidecar` sans `setpriv`, `prlimit`, `unshare`, `mount` — appels système Go ou lanceur embarqué `inkwell-sandbox` ; (3) tests du confinement existants verts + test qui prouve que le confinement tient sans ces outils (PATH vide) ; (4) garde anti-affaiblissement : échoue si le diff d'un lot affaiblit un test, un seuil ou un délai (`gates/*.toml`, `thresholds.json`, `t.Skip`, délais) ; (5) porte L0 toujours verte |
| Condition d'arrêt | porte verte, ou décision humaine (fonction de confinement impossible sans outil externe sur une cible) |
| Revue | sécurité (Opus) obligatoire avant fusion : c'est le bac à sable |
| Lot terminé | L0 (2026-11-02) : harnais de portes, référence de performance, `gates/L0.json` |

## Lots suivants (MVP, dans l'ordre)

1. **L2 — Écrire et retrouver** : G-0a puis G-0b (ouvrir vraiment un fichier), puis onglets
   (G-0c, N19) ; porte : FM-OPEN-1…8, PERF-MEM sur ouverture réelle du corpus et PERF-TYPE natif WebKitGTK (frappe → peinture, froid/chaud).
2. **L3 — Comprendre, corriger, produire** : tinymist branché (N28, N29) ; porte : `problems.spec.ts`,
   `PERF-LSP`.
3. Puis : Collaborer sans perdre, Première minute et secours, Revenir en arrière, Fondations
   extensibles (`coherence.md` §5).

## Journal de pilotage

2026-11-03 — décision technique : TYP-10 vérifie la propriété la plus forte du noyau, avec la même sonde ABI Landlock que le lanceur. ABI ≥ 4 : aucun TCP ne quitte le bac à sable, y compris vers le proxy espion local (aucune connexion pendant la compilation). ABI < 4 : la tentative doit atteindre uniquement le proxy refusant. Dans les deux cas, ErrMissingPackage doit guider vers `.typstdeps` en moins de 20 s. Cette adaptation renforce la preuve de confinement ; elle ne retire aucun test ni ne relève aucun délai. Toute dérogation du garde est documentée dans `gates/guard-overrides.json`, référencé par le manifeste L1.

2026-11-02 — décision technique : le gate Go compare la branche à son merge-base(main), mesurés dans la même session sur la même machine sous scripts/heavy.sh : 10 tours A/B entrelacés de count=2, ordre inversé à chaque tour. Régression si delta > +5 % en temps ou allocs/op et p < 0,05. Paquets et dépendances identiques : comparaison informative, exécutée et rapportée. Référence stockée conservée pour budgets absolus et tendances ; seuils e2e p95 × 1,10 inchangés. Le changement de méthode élimine le biais de charge entre sessions sans relever les seuils ; un autotest injecte +20 % de sommeil et exige sa détection.

| Date | Décision |
|---|---|
| 2026-11-02 | Spécifications closes : design rév. 7.2 et architecture rév. 8.3 confirmées (Codex + Sonnet, plus de Fable) ; on ne les change plus que par décision consignée |
| 2026-11-02 | Idées reprises de 10 dépôts externes, rien d'installé (`docs/plans/2026-11-02-outils-externes.md`) : discipline de modification dans CLAUDE.md, garde anti-affaiblissement des portes, check-list anti-tics en français, notation 0–10 et trois largeurs dans `ux-walkthrough` |
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

### Déviation connue L1 — audit de sécurité non approuvé

Le sandbox redémarre depuis le binaire GUI cgo au lieu du binaire statique
`inkwell-sandbox` prévu aux sections 9.5/9.10. Correction différée au lot
**Fondations extensibles (plugin host needs the same static launcher)**.

Décision L1 : les nouveaux tests IPC ignorent uniquement les protections absentes du
 noyau (signal : ABI < 6 ; socket Unix par chemin : ABI < 9), comme demandé par
 l'audit. Aucun test existant n'est ignoré. Sur ABI suffisante, un échec de
 confinement reste bloquant. L'exception anti-affaiblissement est liée au contenu
 exact du fichier de tests, et ne change aucun seuil ni aucune porte.
