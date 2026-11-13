# Objectif en cours

Registre lu par les outils (`scripts/gate.py`, `scripts/block_done.py`) : noms de blocs, lots et reports de critères.
Plus les décisions encore en vigueur. L'état d'avancement, l'ordre et le reste à faire vivent dans `PROGRESS.md`.
Récit des lots terminés et journal de pilotage : [`docs/archive/2026-11-objectif-journal.md`](archive/2026-11-objectif-journal.md).

## Registre des blocs

Une ligne par bloc (porte `gates/<bloc>.toml` quand elle existe) ; l'état et l'ordre vivent dans `PROGRESS.md`,
le récit de chaque bloc dans l'archive.

| Bloc | Rôle |
|---|---|
| L0, L1, L2, L3 | lots MVP : références et portes de performance, bac à sable, éditeur multi-fichiers (G-0a), tinymist (L3) |
| P2, P3 | protocole, étapes 2 et 3 d'ADR 0034 (signature N-1 ; profil CBOR et schémas) |
| RL1 | règle L de la spec sur sync/1, perte silencieuse corrigée |
| NI0 | réseau perturbé (netsim) et mesures de lien v1 |
| WB, SP1, SP1b | budget d'écriture puis chemin d'envoi de sync/1 ; WB remplacé par SP1, SP1 par SP1b |
| CI1, CI1b | CI plus rapide (netsim dégradé seulement si touché) ; CI1 remplacé par CI1b |
| C5S | mesure de perte par la sonde de lien (ADR 0036) et clause C5 sûre |
| PB, RG | mesures de performance appariées et entrelacées ; garde de benchmark robuste à la mise en page du code |
| GR | reprise d'une porte critère par critère sur le même candidat |
| RC1 | reconnexion après redémarrage sans relais |
| SB1, SB1b | rapports de plantage et mémoire du bac à sable typst ; SB1 remplacé par SB1b |
| SI1, SI1b | indicateur de synchronisation honnête ; SI1 remplacé par SI1b |
| OI | une seule instance par profil et ouverture avec (OI1, OI2 : W41) |
| LF0, LF0a, LF0a2, LF0a3, LF0b | coloriage incrémental (W42) ; chaque variante remplace la précédente, LF0a3 livré, LF0b = analyseur Typst à état explicite |
| PR1 | présence correcte (W46 : départ, présence figée, couleurs ; R1) |
| SI1c | en-tête stable et lisible, amende DS-3 de SI1b (R1) |
| DU | mise à jour ciblée des dépendances pour R1 (`uuid` ≥ 11.1.1) |
| Sans porte encore | PA, AA (audits), ST1, DG1, DG2, CP0 à CP5, LF0c, LF1, IM1, G-0b, G-0c, S13, N35, Fondations-extensibles |

## Lots suivants (MVP, dans l'ordre)

1. ~~**L3 — Comprendre, corriger, produire**~~ ✅ (2026-11-08) : tinymist branché (N28, N29) ; preuves dans
   `docs/plans/2026-11-05-L3-implementation.md`, statut machine : `gates/L3.json`.
2. **Protocole, étapes 2 à 8 d'ADR 0034** puis **G-0b** (index `G`, un document par fichier, FM-OPEN-4…8, PERF-MEM) et **G-0c** (onglets, Worker). Rappel de l'utilisatrice (2026-11-07, U13) : créer, supprimer, renommer, déplacer fichiers et dossiers, menu contextuel, onglets ; G-0b (un document par fichier) en est le prérequis, puis les opérations FM-* et les onglets de G-0c.
3. **S13 — Fenêtre et mise en page** (spec : `docs/design/S13-spec.md` ; N49 à N57 : barre système retirée et boutons gérés par l'app, aperçu façon visionneuse PDF synchronisé avec le texte, icônes d'une seule source, icône de l'app ; N51 menus atteignables dans une fenêtre basse ; N52 numéros de ligne) et **fenêtres étroites** (problème connu, présent sur `main`) : sous 1300 px, ouvrir le tiroir
   Historique pousse la mise en page (titre, boutons de l'en-tête et barre d'état se chevauchent) au
   lieu de couvrir l'aperçu (spec §3.4) ; un menu plus haut que la fenêtre passe sous la barre
   d'état. Porte : UX-LUP-1 (`frontend/e2e/ux/lup.spec.ts`, 1100 et 950 px) repasse de `test.fixme`
   à `test` et passe.
4. Étude de conception N53 (mode « enhance », éditions facilitées sur place, d'abord les tableaux) : passe design puis architecture, avant de fixer son lot.
5. Puis : Collaborer sans perdre, Première minute et secours, Revenir en arrière, Fondations
   extensibles (`coherence.md` §5).

### Lots connus et critères reçus en report

Chaque report (`status = "deferred"`) d'une porte vise un lot de cette liste ou un `gates/<lot>.toml`
existant, qui doit lister le critère à nouveau ; sinon `scripts/gate.py` le compte en échec. Un
critère reporté plusieurs fois figure dans chaque lot de la chaîne (PERF-MEM : L0 → L2 → G-0b).

```gate-lots
L2: PERF-TYPE-native PERF-MEM
L3: PERF-TYPE-native
G-0b: PERF-MEM
N35: native-cgo-darwin-windows seccomp-trace-arm64
Fondations-extensibles: PERF-START-plugins
```

## Décisions en vigueur

Les spécifications sont acceptées (design rév. 7 + 7.1, architecture rév. 8.1 + 8.2) ; on ne les
change plus que par une décision consignée dans `docs/architecture/decisions.md` ou
`docs/design/decisions-utilisatrice.md`.

2026-11-03 — décision technique (boucle extérieure), modèle de menace des portes : le garde anti-affaiblissement (`scripts/gate-guard.py`) protège contre un agent qui affaiblit une porte **par commodité ou par erreur** (test sauté, délai relevé, commande assouplie, exécuteur modifié), pas contre un agent délibérément malveillant ; il reste proportionné à la demande de l'utilisatrice (règles explicites et relisibles, dérogations revues, aucun bac à sable du dépôt ; `.git/info/exclude` hors périmètre). `mise run gate` exécute le `scripts/gate.py` de la base de fusion (copie temporaire, `python3 -I`), et celui-ci exécute le garde de la base de fusion hors du dépôt ; amorçage : si la base n'a pas d'exécuteur au protocole, celui de l'arbre tourne et le manifeste l'indique. Le garde protège aussi tout ce que les critères appellent : tâches mise, scripts et configurations d'outils, déterminés à partir des commandes de la base de fusion.

2026-11-03 — décision de sécurité (spec §9.11, refuser plutôt qu'affaiblir) : la sonde des espaces de noms (`probeOutputMount`) a un délai de 5 s. Seul un refus **définitif** choisit le lancement sans espaces de noms et est mis en cache : `clone`/`unshare` ou la pose des correspondances d'identifiants refusés par EPERM, EINVAL ou ENOSPC (sysctl, AppArmor, noyau sans espaces de noms utilisateur), ou l'enfant de sonde qui reçoit l'une de ces erreurs en montant (code de sortie 120). Ce repli garde no_new_privs, les capacités retirées, les limites de ressources, Landlock et seccomp (tous deux obligatoires), mais n'a ni espace de noms utilisateur, montage, PID ou IPC, ni surcouche IPC. Un dépassement du délai ou toute autre erreur passagère refuse **ce** lancement (`ErrProbeInconclusive`, à réessayer), n'est jamais mis en cache comme « non pris en charge », et la sonde est relancée après un recul exponentiel (1 s à 30 s). Même traitement pour la sonde d'isolement réseau (`probeIsolation`) : un refus définitif laisse `NoNetwork` au seul proxy, un résultat non concluant refuse le lancement (et le moteur Typst). Tests : `TestOutputMountProbeHungChildRefusesLaunch`, `TestOutputMountProbeDefinitiveRefusalSelectsFallback`, `TestIsolationProbeHungChildRefusesLaunch`, `TestIsolationProbeDefinitiveRefusalSelectsFallback`.

### 2026-11-03 — décision de l'utilisatrice (N42 précisée)

N42 (`docs/exigences.md`) : « aucun programme extérieur » signifie qu'inkwell ne doit
**exiger aucun programme installé à part**. Interagir avec la plate-forme est permis :
bibliothèques multiplateformes (Wails, x/sys, quic-go), navigateur par défaut pour un lien,
gestionnaire de fichiers, D-Bus, trousseau. « Il ne faut pas être extrémiste. » La sécurité
des documents non fiables est une **règle distincte** : le bac à sable reste strict, et le
contenu d'un pair ne déclenche jamais une action de la plate-forme sans geste de la personne.

S'applique à L0, L1 et L2. Retiré (correction de périmètre décidée par l'utilisatrice, pas
un affaiblissement ; dérogations du garde dans `gates/guard-overrides.json`) :

- l'analyse des dépendances de `tools/archexec` : parcours du graphe `go list -deps`
  (introduit en L1 : cgo et appels bruts dans une dépendance), étendu en L2 par l'audit par
  dépendance `tools/archexec/audited.json` (verdicts deny/exception/via, versions épinglées
  contre go.sum), les heuristiques syscall brut, assembleur, `go:linkname` et recherche
  dynamique de DLL (NewProc, FindProc, GetProcAddress), le mode `arch-exec -deps`, leurs
  tests et fixtures (`dependencies*.go`, `audits.go`, `l2_contract_test.go`, testdata
  `cgo`, `opener`, `rawsyscall`, `wrapped`, `execabs`, `wailsapi`, faux Wails) ;
- la règle de l'analyseur qui refusait un import atteignant os/exec par transitivité ;
- L2 : `app.RuntimeGuard` (refus de Browser.OpenURL et des appels runtime découpés),
  les règles de l'analyseur contre OpenURL/OpenFile/OpenFileManager et `pkg/updater`,
  `app.DisarmFrameworkLaunchers` (adresse D-Bus forcée, variables WAILS_UPDATER_HELPER*
  effacées). Aucune ne protégeait d'une action déclenchée par un pair : les deux
  variables d'environnement ne viennent que de la session locale ;
- L1 : la copie `third_party/memory` (pbnjay/memory sans `vm_stat`, qui est livré avec
  macOS), son `replace` et `internal/memorytest`, qui ne testait que cette copie.

Règle unique conservée : **notre propre code** (paquets du module, toutes configurations de
build) n'importe pas os/exec et ne démarre aucun processus, hors de l'allowlist du lanceur
`internal/sidecar`. Analyseur typé par configuration (`go vet -vettool`, Linux cgo
server/production, darwin et windows sans cgo) et passe syntaxique indépendante des tags,
inchangés par ailleurs ; `x/sys/windows` CreateProcess*/ShellExecute* s'ajoute aux
fonctions de lancement. Les critères `syscall-confinement` (L1) et
`L1-security-followups` (L2) gardent leurs identifiants et lancent ce contrôle réduit.

Conservé, car c'est de la sécurité et non N42 : le bac à sable L1 complet (seccomp,
Landlock, espaces de noms, init PID 1, règles du dossier de travail et de l'environnement,
refus hors Linux selon spec §9.11) et le remplacement de setpriv/prlimit/unshare/mount par
des appels système (ces outils ne sont pas garantis installés : conforme à N42) ; en L2 la
validation de la présence (`admission.ts`), les refus du dossier de travail, la liste
blanche d'environnement du Supervisor, le jeton du gestionnaire d'images, les contrôles de
liste de ReadProjectText/ReadImage. Ajouté : un lien ne s'ouvre que par
`frontend/src/ui/external.ts`, sur un clic de confiance en cours de traitement, en http,
https ou mailto ; un test de sources interdit à tout autre module Browser.OpenURL, le
gestionnaire de fichiers et l'injection de balisage (seules les icônes constantes en ont).
L0 : portes et mesures de performance conservées ; aucun excès N42 trouvé.

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

### 2026-11-07 — décision technique fournie par l’utilisatrice (lecture Claude, PERF-TYPE)

> Spec reading (Claude decision, record it in docs/objectif-en-cours.md as a technical decision): spec.md:2568 "≤ 16 ms par image" is a per-frame budget (frame work/interval; no dropped or long frame), not input-to-next-vsync latency.

> Use Event Timing to validate the oracle on the slow-layout positive control (which must exceed the threshold and must be detected), and use the handler/frame measurement for normal keystrokes.

> You cannot open a window: write, unit-test, then give me the exact command; I run it on the host.

Application : budget de travail par image **16 ms**, gestionnaire **4 ms**, aucune
pause **>50 ms**, inchangés. L’attente du prochain rafraîchissement et les durées
Event Timing sont des diagnostics séparés, pas des mesures du travail par image.
Les intervalles réels sont conservés et les images manquées sont recherchées par
rapport à la cadence observée ; aucune attente vsync n’est soustraite d’un budget
de travail. L’oracle de travail d’image doit détecter le témoin lent, corroboré
par Event Timing ; l’absence d’entrées rapides (seuil minimal 16 ms) n’est pas un
échec d’oracle. Aucun témoin absent/manqué ne donne un succès.
Le poste doit avoir l’écran allumé et déverrouillé ; le dialogue GNOME autorisant
XTest peut voler le focus. Cette passe n’ouvre aucune fenêtre. L3 reste bloqué
jusqu’à la nouvelle mesure native ; aucune optimisation ni réussite n’est déduite
des seuls tests unitaires.

### 2026-11-07 — décisions de l'utilisatrice (banc PERF-TYPE natif, après l'optimisation)

Questions posées après la passe d'optimisation (`docs/perf/2026-11-07-mutter-instrumentation.md`) ; réponses :

1. **Règle à vide.** La règle « NULL upper p95 ≤ 1 ms » (ajoutée par le banc, pas par la spec) est impossible : une frappe à vide coûte ~1,3 ms de vrai travail du navigateur. Remplacée par : **surcoût de l'instrument ≤ 1 ms**, mesuré par essais NULL entrelacés sondes actives / inactives (mêmes deux lectures externes), critère sur la borne haute unilatérale à 95 % de l'écart des moyennes.
2. **Départ d'une frappe.** `event.timeStamp` dérive de −4 à +5 ms contre `performance.now()`. Le départ devient **la première lecture `performance.now()` de l'écouteur `window` en capture** ; `event.timeStamp` reste un diagnostic.
3. **Première frappe.** Les budgets 4 ms (gestionnaire) et 16 ms (travail par image) valent pour la frappe à chaud. **La première frappe d'un processus neuf** a son propre seuil, mesuré et suivi : gestionnaire p95 ≤ 50 ms et travail d'image p95 ≤ 50 ms (pratique INP : le démarrage n'est pas la réactivité). La règle « aucune pause > 50 ms » est inchangée.

Aucun autre seuil ne change. L3 reste bloqué jusqu'à une mesure native qui passe avec ces règles.

### 2026-11-07 (soir) — décisions de l'utilisatrice (« je suis tes recommandations »)

1. **Gestionnaire à chaud :** la porte L3 passe à **p95 ≤ 8 ms** ; **4 ms reste l'objectif**, suivi par le banc à chaque lot (valeur affichée à côté du verdict). Raison : sur WebKitGTK la moyenne est ~5 ms (3,7 ms avec le prototype) mais le p95 reste 6–9 ms ; ce qui se perçoit est l'image dans les 16 ms.
2. **Prototype « CodeMirror insère le texte » :** reste **désactivé** pour la fusion de L3 (aucun risque de régression de saisie) ; il sera activé après une vérification à la main par l'utilisatrice (ibus, touche morte, Orca), procédure à fournir.
3. **Premier affichage de l'aperçu (~50 ms, présentation WebKit, une fois par lancement) :** exception de démarrage, comme la première frappe : seuil séparé **≤ 70 ms** pour la phase froide ; « aucune pause > 50 ms » reste la règle de la phase chaude.
4. Commit des documents (registre N54–N70, études, spec S13) autorisé.

Les autres budgets (travail d'image ≤ 16 ms à chaud, rafraîchissements manqués, phase froide ≤ 50 ms) sont inchangés.

### 2026-11-07 (soir, 2) — décision de l'utilisatrice (rafraîchissements manqués)

Question posée : la règle « aucun rafraîchissement manqué » échoue à cause du premier affichage de l'aperçu (froid, déjà sous l'exception de 70 ms) et parce que la frappe à vide (sans éditeur) manque elle-même 1 à 3 images sur 30 (WebKit et machine). Réponse : **relatif à la frappe à vide.**
- Froid : le premier affichage de l'aperçu relève de l'exception de démarrage (pause ≤ 70 ms) et n'est pas compté comme rafraîchissement manqué de la frappe.
- Chaud : l'éditeur ne doit pas manquer **significativement plus** d'images que la frappe à vide de la même session (test exact unilatéral sur les proportions, p < 0,05) ; les deux taux sont publiés.
- La frappe à vide ne bloque plus la certification par ses propres images manquées ; la certification de l'instrument reste le surcoût ≤ 1 ms (borne haute à 95 %).
Rien d'autre ne change.

### 2026-11-07 (nuit) — décision de l'utilisatrice (cohérence de la phase froide)

Constat : la première frappe acceptée jusqu'à 50 ms couvre forcément plus d'un rafraîchissement (16 ms) ; la règle « aucun rafraîchissement manqué » contredit donc l'exception de démarrage déjà décidée. Réponse : « je suis tes recommandations ».
- **Froid :** la règle des rafraîchissements manqués est remplacée par les seuils de démarrage déjà décidés (gestionnaire p95 ≤ 50 ms, travail d'image p95 ≤ 50 ms, pause ≤ 70 ms). Les rafraîchissements manqués à froid restent **publiés en diagnostic**.
- **Chaud :** inchangé (gestionnaire p95 ≤ 8 ms, objectif 4 ms ; travail d'image ≤ 16 ms ; manqués de l'éditeur pas significativement plus que la frappe à vide ; pause ≤ 50 ms).
- **Fusion de L3 : 3 mesures réussies sur 3**, d'affilée, sur le code final.
