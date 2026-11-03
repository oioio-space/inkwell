# Objectif en cours

Une seule page, tenue à jour à chaque lot. Méthode : développement agentique dirigé par l'humain
(boucle extérieure humaine : vision, portes, jugement, arrêt ; boucle intérieure de l'agent :
coder, construire, lancer, mesurer, corriger, jusqu'à ce que la porte passe ou qu'une décision
humaine soit nécessaire). Référence : `docs/plans/2026-11-02-methode-hdae.md`.

Les spécifications sont acceptées (design rév. 7 + 7.1, architecture rév. 8.1 + 8.2) ; on ne les
change plus que par une décision consignée dans `docs/architecture/decisions.md` ou
`docs/design/decisions-utilisatrice.md`.

## Lot actif : L2 — Ouvrir vraiment un fichier (G-0a)

| | |
|---|---|
| But | un clic dans la liste des fichiers ouvre vraiment le bon fichier ; suivis de sécurité de L1 ; mesure native de la frappe |
| Exigences | N19 (début), N20, G-0 ; architecture §6.6 lot **G-0a** (le reste, G-0b/G-0c, dépend d'ADR 0031, de l'étape 8 d'ADR 0034 et du Worker) ; suivis LOW de L1 ci-dessous |
| Porte | `mise run gate -- L2` : (1) e2e `files.spec.ts` FM-OPEN-1 (clic → contenu du bon fichier), FM-OPEN-2 (sous-titre), FM-OPEN-3 (txt, csv, md, typ distingués ; lecture seule avec bandeau pour les autres que `main.typ`) ; (2) `?file=` supprimé ; (3) les trois suivis de sécurité de L1 faits et testés ; (4) **PERF-TYPE natif** WebKitGTK (frappe → peinture, froid/chaud) mesuré, seuil = p95 × 1,10 ; (5) portes L0 et L1 vertes |
| Reports | PERF-MEM → lot G-0b (ouverture réelle de plusieurs fichiers) |
| Condition d'arrêt | porte verte, ou décision humaine (comportement de l'interface non prévu par le design) |
| Revue | Sonnet (code) + test en personne `ux-walkthrough` (règle du projet pour toute modification d'interface) |
| Lots terminés | L0 (2026-11-02) ; L1 (2026-11-03) : confinement sans programme externe, approuvé après six revues de sécurité Opus |

### Suivis de sécurité de L1 (LOW, revue Opus n° 6, à traiter au début de L2)

- Analyse des dépendances : traiter comme dangereux tout appel `Syscall*`/`RawSyscall*`, tout fichier `.s` (`SFiles`) et tout `go:linkname` dans une dépendance, avec une liste blanche des paquets qui livrent légitimement de l'assembleur (`tools/archexec/dependencies.go:107-128`).
- Environnement de tinymist : filtrer `cmd.Env` contre une liste blanche dans `runOnce`, même si l'appelant passe `os.Environ()` (`internal/sidecar/supervisor.go:215`).
- Dossier de projet : refuser aussi les dossiers situés **dans** un dossier protégé (`~/.ssh`, `~/.gnupg`), pas seulement ceux qui en contiennent un (`internal/sidecar/workdir.go:44-76`).

## Lots suivants (MVP, dans l'ordre)

1. **L3 — Comprendre, corriger, produire** : tinymist branché (N28, N29) ; porte : `problems.spec.ts`,
   `PERF-LSP`.
2. **Protocole, étapes 2 à 8 d'ADR 0034** puis **G-0b** (index `G`, un document par fichier, FM-OPEN-4…8, PERF-MEM) et **G-0c** (onglets, Worker).
3. Puis : Collaborer sans perdre, Première minute et secours, Revenir en arrière, Fondations
   extensibles (`coherence.md` §5).

### Lots connus et critères reçus en report

Chaque report (`status = "deferred"`) d'une porte vise un lot de cette liste ou un `gates/<lot>.toml`
existant, qui doit lister le critère à nouveau ; sinon `scripts/gate.py` le compte en échec. Un
critère reporté plusieurs fois figure dans chaque lot de la chaîne (PERF-MEM : L0 → L2 → G-0b).

```gate-lots
L2: PERF-TYPE-native PERF-MEM
G-0b: PERF-MEM
N35: native-cgo-darwin-windows seccomp-trace-arm64
Fondations-extensibles: PERF-START-plugins
```

## Journal de pilotage

2026-11-03 — décision technique (boucle extérieure), modèle de menace des portes : le garde anti-affaiblissement (`scripts/gate-guard.py`) protège contre un agent qui affaiblit une porte **par commodité ou par erreur** (test sauté, délai relevé, commande assouplie, exécuteur modifié), pas contre un agent délibérément malveillant ; il reste proportionné à la demande de l'utilisatrice (règles explicites et relisibles, dérogations revues, aucun bac à sable du dépôt ; `.git/info/exclude` hors périmètre). `mise run gate` exécute le `scripts/gate.py` de la base de fusion (copie temporaire, `python3 -I`), et celui-ci exécute le garde de la base de fusion hors du dépôt ; amorçage : si la base n'a pas d'exécuteur au protocole, celui de l'arbre tourne et le manifeste l'indique. Le garde protège aussi tout ce que les critères appellent : tâches mise, scripts et configurations d'outils, déterminés à partir des commandes de la base de fusion.

2026-11-03 — décision de sécurité (spec §9.11, refuser plutôt qu'affaiblir) : la sonde des espaces de noms (`probeOutputMount`) a un délai de 5 s. Seul un refus **définitif** choisit le lancement sans espaces de noms et est mis en cache : `clone`/`unshare` ou la pose des correspondances d'identifiants refusés par EPERM, EINVAL ou ENOSPC (sysctl, AppArmor, noyau sans espaces de noms utilisateur), ou l'enfant de sonde qui reçoit l'une de ces erreurs en montant (code de sortie 120). Ce repli garde no_new_privs, les capacités retirées, les limites de ressources, Landlock et seccomp (tous deux obligatoires), mais n'a ni espace de noms utilisateur, montage, PID ou IPC, ni surcouche IPC. Un dépassement du délai ou toute autre erreur passagère refuse **ce** lancement (`ErrProbeInconclusive`, à réessayer), n'est jamais mis en cache comme « non pris en charge », et la sonde est relancée après un recul exponentiel (1 s à 30 s). Même traitement pour la sonde d'isolement réseau (`probeIsolation`) : un refus définitif laisse `NoNetwork` au seul proxy, un résultat non concluant refuse le lancement (et le moteur Typst). Tests : `TestOutputMountProbeHungChildRefusesLaunch`, `TestOutputMountProbeDefinitiveRefusalSelectsFallback`, `TestIsolationProbeHungChildRefusesLaunch`, `TestIsolationProbeDefinitiveRefusalSelectsFallback`.

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

### 2026-11-03 — décision technique

Pour L1, l'analyse typée tourne maintenant nativement sur Linux avec cgo activé,
avec les tags desktop (`production`) et `server`. Sur darwin et windows (amd64 et
arm64), elle tourne maintenant avec `CGO_ENABLED=0` et les tags desktop.
Une passe `go/parser` indépendante des tags parcourt les sources Go, y compris
celles qui importent `C`, et interdit `C` et `os/exec` hors de l'allowlist
`internal/sidecar` (les helpers de tests gardent leur exemption existante).
Les fixtures négatives `testdata` ne sont pas du code applicatif.

| Validation | Statut | Lot cible | Raison |
| --- | --- | --- | --- |
| Type-check complet avec cgo sur darwin/windows | deferred | Six plates-formes (N35) — CI natif par OS/arch | Nécessite les SDK et runners natifs par OS selon ADR 0003 ; la passe sans cgo ne valide pas ces fichiers. |
| Validation des traces seccomp arm64 | deferred | Six plates-formes (N35) — CI natif par OS/arch | Nécessite un runner Linux arm64 natif ; une analyse statique ne prouve pas les appels réellement exécutés. |

Ces reports ne sont jamais des passes et figurent séparément dans le manifeste L1.

Précision darwin : Wails ne compile pas son package `application` sans cgo
(même ses fichiers Go référencent les définitions natives exclues). Le root
`cmd/inkwell` est donc couvert maintenant par la passe syntaxique ; son analyse
typée complète relève explicitement du report cgo N35. Les autres packages
restent analysés sans cgo ; aucune erreur de ceux-ci n'est ignorée.
