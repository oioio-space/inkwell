# Objectif en cours

Une seule page, tenue à jour à chaque lot. Méthode : développement agentique dirigé par l'humain
(boucle extérieure humaine : vision, portes, jugement, arrêt ; boucle intérieure de l'agent :
coder, construire, lancer, mesurer, corriger, jusqu'à ce que la porte passe ou qu'une décision
humaine soit nécessaire). Référence : `docs/plans/2026-11-02-methode-hdae.md`.

Les spécifications sont acceptées (design rév. 7 + 7.1, architecture rév. 8.1 + 8.2) ; on ne les
change plus que par une décision consignée dans `docs/architecture/decisions.md` ou
`docs/design/decisions-utilisatrice.md`.

## Lot terminé : L3 — Comprendre, corriger, produire ✅

| | |
|---|---|
| But | erreurs cliquables et ancrées, F8 / Maj+F8, aide Typst depuis tinymist embarqué |
| Exigences | N28, N29 ; architecture §15.17 ER-1–4, LSP-1–5 ; AD-79, AD-80 |
| Porte | `INKWELL_NATIVE_PERF_RUN=1 mise run gate -- L3` verte : problèmes e2e, positions UTF-16 / Automerge, LSP-STALE, service hors ligne, PERF-LSP p95 ≤ 100 ms, PERF-TYPE natif (chaud p95 ≤ 8 ms, froid p95 ≤ 50 ms), régression L2 |
| Preuves | `gates/L3.json` (compact) ; mesures brutes dans `gates/L3-setup/native-raw-*`, citées par chemin et SHA-256 ; rapport perdu le 2026-11-08 (non versionné, worktree supprimé); reconstitution : `gates/L3/RECONSTRUCTED.md` ; résultat de la porte : `gates/L3.json` |
| Fusion | fusionné sur `main` (f66524f) |

## Lot actif : Protocole, étapes 2 à 8 d'ADR 0034

Ensuite G-0b, puis G-0c (liste ci-dessous). **Étape 2 (N-1, `protocol/sig`) faite** le 2026-11-08 : porte `gates/P2.toml` verte
(6/6, régression L3 comprise ; `gates/P2.json`) ; plan `docs/plans/2026-11-08-P2-implementation.md`, résumé : rapport perdu le 2026-11-08 (non versionné, worktree supprimé); reconstitution : `gates/P2/RECONSTRUCTED.md` ; résultat de la porte : `gates/P2.json`. **Étape 3 faite** le 2026-11-08 (porte `gates/P3.toml` verte, régressions comprises ; `mise run tla`,
profil CBOR et schémas §9). **Étape suivante : 4**, à spécifier d'abord en exécutable
(`docs/plans/2026-11-08-spec-executable.md`). Les étapes 3 à 8
auront chacune leur porte, écrite avant leur implémentation.

**Dérogations de la porte P3 (2026-11-08, `gates/guard-overrides.json`).** Quatre dépendances de porte
changent sans rien affaiblir : `.golangci.yml` ajoute le linter `modernize` (W26) ; `mise.toml`
(`mutate:go`) installe gremlins épinglé, pose `-count=1` et un plafond de 1,8 Go par binaire de test,
et passe le coefficient de délai de 3 à 10 — un mutant expiré sort du score, donc un délai plus long ne
peut pas le relever, il fait seulement juger plus de mutants (278 sur 278 expirés à 3 : mesure nulle),
et `done` refuse désormais plus de 10 % d'expirés ; `scripts/alloy_check.py` et `scripts/tla-check.sh`
délèguent le téléchargement épinglé à `scripts/fetch_pinned.py` (même empreinte, même délai de 120 s).
`protocol/trust` (92,4 %, 21 survivants antérieurs à P3) sort de P3 par décision de l'utilisatrice
et devient le bloc W31.
`scripts/gate.py` ne vide plus que les preuves de `gates/<lot>/` et garde les rapports `*.md` (son
`rmtree` avait effacé le rapport `/simplify` de P3 et les reconstitutions L3/P2, restaurés depuis les
transcriptions) ; testé par `scripts/gate_reset_test.py`.

**Bloc RL1 (2026-11-08, porte `gates/RL1.toml`) — règle L de la spec sur sync/1, perte silencieuse
corrigée.** Trouvée par NI-2 (coupure de 25 s pendant un gros collage) : une écriture qui expire après
WriteIdle réinitialisait le flux sortant sans PeerDown, donc automerge-repo ne renvoyait jamais le
changement perdu. Correctif : un envoi échoué (hors annulation par l'appelant) annonce PeerDown puis
PeerUp, ce qui relance la synchronisation ; compté à part (`Resyncs`), ce n'est pas une coupure.
Revue : `gates/RL1/SIMPLIFY.md`. Le bloc embarque l'outillage réseau perturbé et les mesures de lien v1
de NI0 (encore ouvert : exposition à la webview, 3 graines, EM-1/EM-2, NI-5). Dérogations de porte
(`gates/guard-overrides.json`), sans rien affaiblir : `.golangci.yml` (exclusions `dupl` de p2p
indexées par paire de fichiers et non plus par numéro de ligne, qui bougeait à chaque édition),
`mise.toml` (tâches `e2e:netsim:impaired` et `netsim:mutants`, `ci` les lance — décision Q2),
`.github/workflows/ci.yml` (charge `sch_netem`, échec franc s'il manque, délai porté à 45 min). Après la porte : `e2e:build` attend (`wait_for`) les tâches qui compilent le paquet racine, car la construction vite vidait `frontend/dist` pendant qu'elles l'embarquaient (course vue par la CI avant push) ; elle construit aussi dans un dossier à part, échangé à la fin, pour qu'une construction interrompue ne laisse jamais `dist` vide. Dérogations suivantes (2026-11-08, sans rien affaiblir) : `scripts/tla-check.sh` ajoute le modèle `SyncIndicator` (indicateur de synchronisation honnête, W33) et ses sept mutants ; `scripts/netsim/run.sh` donne à chaque scénario son propre dossier de sortie Playwright, pour que les traces d'un échec survivent au scénario suivant (W36). `test:py` lance aussi `scripts/llm_test.py` (bascule entre fournisseurs gratuits Groq → Cloudflare → OpenRouter, sans réseau). Le nouveau test unitaire NI-4 C5 du compteur de synchronisation attend 300 ms puis 500 ms (fenêtre de calme) ; aucune attente existante n'est allongée.

**Bloc WB (ouvert le 2026-11-09, porte `gates/WB.toml`).** Budget d'écriture de sync/1 : 20 s sans progrès (inchangé), plus un total de 20 s + taille / 2 Kio/s au lieu de 16 Kio/s, pour qu'un gros collage sur un lien lent mais qui avance ne soit plus échoué puis renvoyé en boucle devant la frappe (NI-3 C2). Modèle `docs/protocol/tla/WriteBudget.tla`, quatre mutants ; décision « plancher seul plutôt qu'estimation du débit » justifiée dans le plan (un lecteur malveillant fait baisser toute estimation, seul le plancher protège). `scripts/tla-check.sh` ajoute ce modèle (dérogation consignée).

**Chemin d'envoi remis à plat (W38, 2026-11-09).** Le bloc WB est clos, remplacé par SP1 (décision de l'utilisatrice). Conception `docs/plans/2026-11-09-send-path-design.md` (causes RC1–RC10 mesurées par E1 et E1b), modèle `docs/protocol/tla/SendPath.tla` (règles SP-1 à SP-9, 12 propriétés, un mutant chacune) et configurations SP1 ; `scripts/tla-check.sh` les ajoute (dérogation consignée). Décisions D-SP1-1 à 4 prises après E1b (yamux `ConnectionWriteTimeout` 120 s, fenêtre épinglée à 256 Kio, réponse au keepalive bornée par 120 s, `partialTTL` 120 s) ; les configurations SP1 sont revérifiées avec ces valeurs, et la porte est écrite : `gates/SP1.toml`, à commiter seule avant le code (le brouillon est supprimé). `gates/WB.toml` reste figée, comme historique. Dérogation consignée : `scripts/tla-check.sh` renomme `SP1Hop90` en `SP1Hop`, ajoute `SP1Silence`, `SP1GrownKeepalive` (ancien témoin résiduel, fermé par 120 s) et le mutant `SP1PartialTTLAtIdle` ; aucune autre exécution n'est retirée. Après la mesure E3, la frappe sur relais lent (NI-3 C2 : 45 à 53 s) ne tient pas la borne de 44 s que SP1 exigeait, et aucune constante de SP1 ne peut l'abaisser (yamux refuse une fenêtre sous 256 Kio) : **bloc SP1b**, porte `gates/SP1b.toml`, qui remplace SP1 (décision de l'utilisatrice, 2026-11-09) avec les mêmes critères sauf cette borne ; NI-3 C1 devient strict. Bloc suivant : la voie prioritaire pour la frappe (ADR 0034 étapes 4 à 6), après avoir modélisé les pertes du relais.

**CI plus rapide (W35, 2026-11-09).** Bloc CI1b (porte `gates/CI1b.toml`, remplace CI1 dont une commande de test ne pouvait pas importer son module) : le netsim dégradé au pré-push seulement quand le diff touche ses entrées (décision de l'utilisatrice), toujours complet sur GitHub et dans `done`, plus de double exécution sur GitHub. Suite de l'étude : R9 (préconditions rapides de `done`), R8+R4 (reçus, couverture au commit), R1, R7.

### Suivis de sécurité de L1 (LOW, revue Opus n° 6, à traiter au début de L2)

- ~~Analyse des dépendances~~ : retirée, avec toute l'analyse des dépendances, par la décision de l'utilisatrice du 2026-11-03 (N42 précisée, journal ci-dessous). Seul notre propre code est contrôlé.
- ~~Environnement de tinymist~~ : fait. `runOnce` applique `supervisorEnv` (liste blanche : langue, fuseau, polices) à `cmd.Env` (`internal/sidecar/supervisor.go`, vérifié le 2026-11-08).
- ~~Dossier de projet~~ : fait. `checkWorkdir` refuse un dossier situé dans un dossier secret (`~/.ssh`, `~/.gnupg`, configuration, données et état XDG) et un dossier qui contient un dossier large (`internal/sidecar/workdir.go`, vérifié le 2026-11-08).

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

## Journal de pilotage

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

### 2026-11-03 — décision technique

*Remplacée par la décision de l'utilisatrice (N42 précisée) ci-dessus.*

L2, suivi (a), décision de la boucle extérieure : le garde cherche les **lancements
 de processus**, pas tous les appels noyau. Un appel Syscall*/RawSyscall* est
signalé si son numéro constant désigne fork, vfork, execve, execveat, clone ou
clone3, ou si son numéro n'est pas constant à la compilation. Les constantes
sont évaluées par go/types avec les exports de la cible (y compris les alias,
conversions et expressions numériques). Une référence indirecte au lanceur reste
opaque et bloquée. Les fichiers `.s`, directives `go:linkname` et numéros non
constants exigent un audit par **paquet, module et version exacte**, consigné dans
`tools/archexec/audited.json` (format JSON, embarqué dans le garde). Aucun joker,
aucune exception pour une version suivante ni pour un remplacement de module.
Les autotests vérifient les versions contre go.sum et font échouer le scan après
changement de version. Une exception n'autorise jamais un numéro de création de
processus constant, le cgo ou une dépendance os/exec.

Audit des cinq signalements initiaux (sources du cache des modules épinglés lues) :

| Paquet | Version exacte | Constat et verdict |
| --- | --- | --- |
| golang.org/x/net/internal/socket | v0.60.0 | Linknames getsockopt/setsockopt ; Syscall6 recvmmsg/sendmmsg constants ; empty.s Darwin vide. Réseau uniquement : exception asm/linkname auditée. |
| github.com/quic-go/quic-go/internal/handshake et internal/qtls | v0.64.1 | Constructeur AEAD AES-GCM TLS 1.3 FIPS et tables de suites TLS : exceptions linkname auditées, aucun lancement. |
| github.com/marten-seemann/tcp | v0.0.0-20210406111302-dfbc87cc63fd | SYS_IOCTL, SYS_SETSOCKOPT, SYS_GETSOCKOPT constants : aucune exception nécessaire sur les cibles vérifiées. Solaris contient des linknames réseau, hors matrice actuelle. |
| golang.org/x/sys/cpu | v0.49.0 | CPUID/XGETBV, registres ARM64, runtime.getAuxv et sysctl Darwin : détection des capacités CPU, exceptions asm/linkname auditées. |
| github.com/pion/dtls/v3 | v3.1.4 | os.SyscallError est un type d'erreur, pas un appel brut : faux positif corrigé, aucune exception. |

Les autres assembleurs cryptographiques et de masquage WebSocket sont désormais
épinglés aussi ; x/net/route v0.60.0 lie sysctl pour lire les tables de routage.
Les fournisseurs primitifs syscall et x/sys/unix/windows **peuvent** lancer des
processus : ils ne sont pas déclarés inoffensifs et ne sont pas allowlistés ; la
frontière existante contrôle leurs points d'entrée chez les appelants. Wails et
internal/sidecar conservent leurs frontières de confiance préexistantes.

Découverte Windows : github.com/go-ole/go-ole v1.3.0 contient des appels
Syscall à adresses de fonctions non constantes et CoCreateInstance avec
CLSCTX_SERVER (dont CLSCTX_LOCAL_SERVER). Il peut activer un serveur COM hors
processus : **exception refusée**, verdict `deny` épinglé dans audited.json.
Le graphe courant l'atteint uniquement via les dialogues Wails, à l'intérieur de
la frontière de confiance Wails préexistante ; aucun appelant applicatif direct
n'est exempté. Un nouvel import hors de cette frontière doit faire échouer le scan.

### 2026-11-03 — décision technique

*Partie « frontière du garde » remplacée par la décision de l'utilisatrice (N42 précisée)
ci-dessus ; le report PERF-TYPE reste valable.*

L2, revue avant fusion, décision de la boucle extérieure. go-ole v1.3.0 devient
une **exception** documentée (remplace le `deny` ci-dessus), limitée `via` le
dialogue de fichiers Wails (`internal/go-common-file-dialog/cfd`). Ces dialogues
sont ouverts par la personne depuis l'interface, jamais pilotés par le contenu
d'un pair, et Windows refuse tout travail des sidecars jusqu'à N35. Risque
résiduel : CoCreateInstance avec CLSCTX_SERVER pourrait démarrer un serveur COM
hors processus. Nouvel audit au lot « Six plates-formes (N35) ».

Frontière du garde, même revue : la confiance couvre le **code propre** d'un
paquet, plus jamais ce qu'il importe ; le parcours continue au-delà de Wails.
Seul `internal/sidecar` peut importer os/exec lui-même. Chaque paquet capable de
lancer un processus a son entrée épinglée (module, version exacte, `via`) :
`exception` (risque résiduel et nouvel audit consignés) ou `deny` (ce qui le rend
inatteignable est consigné et vérifié). Verdicts : godbus/dbus/v5 v5.2.2 `deny`
(autolaunch de dbus-launch neutralisé : l'adresse du bus est épinglée avant
application.New) ; Wails internal/browser `deny` (`app.RuntimeGuard` refuse
l'objet Browser du runtime, l'analyseur refuse BrowserManager.OpenURL/OpenFile) ;
internal/fileexplorer `deny` (l'analyseur refuse OpenFileManager) ; internal/git
`deny` (absent des builds de production ; go.mod ne remplace pas Wails) ;
pkg/updater `deny` (WAILS_UPDATER_HELPER effacé avant application.New,
l'analyseur refuse tout pkg/updater) ; pkg/w32 `deny` (ShellExecute sans appelant,
CoCreateInstance seulement en CLSCTX_INPROC_SERVER). Le garde signale aussi
SyscallN/RawSyscallN et toute recherche dynamique Windows (NewProc, FindProc,
GetProcAddress) d'une fonction de création de processus ou d'un nom non constant.

PERF-TYPE natif : build et essais réels archivés dans `gates/L2/` ; la fenêtre
WebKitGTK obtenue reste blanche (capture native-window.png). Le runtime par défaut
est non inscriptible ; les essais avec runtime privé et rendu logiciel n'établissent
pas d'éditeur mesurable ni d'oracle de peinture. Aucun échantillon froid/chaud
inventé, aucun proxy Chromium substitué. **Report explicite vers L3**, sur runner
natif avec compositeur, entrée OS et oracle de peinture fonctionnels ; seuil
conservé **p95 natif de référence × 1,10**, sans valeur numérique tant que la
référence native ne peut pas être établie. Voir PERF-TYPE.json.

### Preuves de la boucle intérieure L2 — 2026-11-03

Implémentation et limites : `docs/plans/2026-11-03-L2-implementation.md` ;
porte : `gates/L2.toml`, résultats bruts sous `gates/L2/` et manifeste `gates/L2.json`.
PERF-TYPE natif : application construite et lancée, WebView sans éditeur utilisable
après 15 s, capture et journaux archivés. Report explicite vers **L3**, sans valeur
p95 inventée ; seuil à établir sur la mesure native : p95 × 1,10.
Le signalement COM Windows (go-ole) est sans objet depuis la décision N42 précisée ;
les résultats des vérifications complètes sont conservés au manifeste.


### 2026-11-05 — L3 : comprendre, corriger, produire

Service tinymist épinglé relié au Stream `lsp`, complétion, signature et survol
avec tickets de révision et abandon à 100 ms. Diagnostics unifiés dans Relire,
compteur unique, F8 / Maj+F8, navigation au fichier et à la plage. Les notifications
sans version du tinymist épinglé sont rejetées ; les diagnostics actifs proviennent
de copies immuables des sources compilées, vérifiées dans la webview puis à
nouveau au saut. Des tests couvrent A→B→A, fichier secondaire modifié et changement
d'une dépendance ouverte. La nouvelle connexion remplace aussi l'ancien flux
Wails abandonné. `x/net` v0.62.0 respecte les sept jours de refroidissement.

Porte et résultats courants : `gates/L3.toml`, `gates/L3.json` ; détails, mesures et
limites : `docs/plans/2026-11-05-L3-implementation.md`. La chaîne L2/L1/L0 est
réexécutée sans modifier ses tests, seuils ou délais. Le navigateur mesure le vrai
aller-retour du service, en exclusif. **PERF-TYPE-native → G-0b : PENDING USER DECISION**, après build et
lancement réels donnant une WebView blanche ; aucune valeur native inventée.
La revue humaine avant fusion reste à faire ; aucun commit dans cette boucle.


### 2026-11-05 — correction L3 après revue Opus

Instructions de l’utilisatrice, citées pour délimiter cette passe :

> Keep slow but current answers, and send $/cancelRequest for superseded requests.

> Use a realistic document (thousands of lines, several files, imports), interleave edits with requests, and require a non-empty useful result.

> Shrink the allowlist to the methods actually used, and add a test that no response sent to the webview contains the absolute project path.

> Make it assert something real, or remove the criterion.

> For 5ce8 (PERF-TYPE-native deferred to G-0b), set reason "PENDING USER DECISION" and do not claim approval anywhere.

La passe retire donc `NATIVE-readiness`, qui ne prouvait pas de latence native.
Le report proposé PERF-TYPE-native reste **PENDING USER DECISION**. Les raisons des
trois scripts décrivent uniquement l’ajout de fichiers. La latence p95 ≤100 ms
reste une mesure exigée ; une réponse courante lente reste affichable, tandis
qu’une réponse périmée est annulée et rejetée. Les autres corrections portent
sur le bouton unique, le clic simple, la synchronisation incrémentale annoncée
par le serveur, les chemins sortants et le coût des copies des sources.

### 2026-11-05 — décision de l'utilisatrice (PERF-TYPE natif)

Question posée : reporter encore PERF-TYPE natif (de L3 à G-0b) ? Réponse : « Bloquer L3 ». L3 ne
fusionne pas tant que PERF-TYPE natif n'est pas mesuré dans l'application de bureau réelle. Constat
du 2026-11-05 : la vue web native reste blanche aussi hors du bac à sable de Codex, depuis la session
graphique réelle (GNOME Wayland, XWayland, rendu par défaut et cairo) ; c'est un défaut à corriger,
pas seulement un banc de mesure à construire.


### 2026-11-05 — diagnostic confirmé et harnais natif, décision de l’utilisatrice

> The native app is NOT blank — all assets load and the DOM holds the full UI; XGetImage under XWayland cannot capture any WebKitGTK content (a bare GTK4+WebKit red page gives the same blank PNG).

> Your sandbox cannot open a window: write and unit-test it, then stop and say exactly which command to run outside the sandbox; I will run it.

> Turn PERF-TYPE-native in gates/L3.toml into a real command criterion running that harness, with the spec thresholds unchanged.

L’ancienne interprétation des PNG blancs est invalidée : ils ne prouvent pas une
vue vide. La mesure abandonne cette capture. **L3 reste BLOQUÉ** jusqu’à mesure
dans la vraie webview native avec oracles validés. Le critère devient une commande
réelle et conserve mot pour mot sa raison de blocage. Sans opt-in explicite du
poste graphique, cette commande écrit un résultat échoué sans ouvrir de fenêtre.
Le build de mesure utilise le tag distinct `perf_native`, des ressources séparées,
des entrées natives dans CodeMirror et le Stream Wails existant ; aucune
instrumentation n’est activée dans le produit normal. Rappel PM-4 : rAF/double-rAF
sont enregistrés mais ne prouvent pas la peinture ; absence d’oracle validé,
contrôle lent manqué, vue masquée ou perte de focus font échouer le cas.

### 2026-11-07 — reprise du harnais après échec sur le poste graphique

> Try running it yourself first: your sandbox shares the network namespace, so DISPLAY=:0 (abstract X11 socket) may be reachable; if it is not, stop and give me the exact command, and I will run it and send you the result.

Le nouvel essai confirme que `DISPLAY=:0` est accessible. L'ancien champ `ready`
vide résultait du rejet sans conservation d'une vraie réponse du navigateur ;
GTK déclarait la fenêtre inactive. Le pilote active maintenant la fenêtre du
processus mesuré et vérifie le focus, sans injecter de touche pendant cette
activation. Les erreurs de fin anticipée restent des échecs avec leur vraie cause.
WebKitGTK 2.54.1 annonce Event Timing ; l'oracle demeure soumis aux témoins lents.
Dans le bac à sable, GTK et le navigateur deviennent visibles/focalisés mais aucun
premier rAF n'arrive, même au diagnostic logiciel séparé. Ce cas échoue avec une
raison conservée et ne fournit aucune latence native. **L3 reste BLOQUÉ** ; ni
seuil ni raison du critère natif ne changent. Le plan L3 donne la commande exacte
à réexécuter sur le poste graphique.

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
