# Archive du journal de pilotage (2026-11)

History moved verbatim from docs/objectif-en-cours.md on 2026-11-13 (finished lot narratives, superseded decisions, old ordered list).
docs/objectif-en-cours.md keeps the block registry and the decisions in force; status lives in PROGRESS.md.

## Méthode (intro d'origine)

Une seule page, tenue à jour à chaque lot. Méthode : développement agentique dirigé par l'humain
(boucle extérieure humaine : vision, portes, jugement, arrêt ; boucle intérieure de l'agent :
coder, construire, lancer, mesurer, corriger, jusqu'à ce que la porte passe ou qu'une décision
humaine soit nécessaire). Référence : `docs/plans/2026-11-02-methode-hdae.md`.

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

**Ordre suivant (2026-11-09, soir).** 1. Bloc C5S (décision de l'utilisatrice : D2 tout de suite ; **porte complète en cours (2026-11-11)**) : mesure de perte par la sonde de lien du protocole (ADR 0036, partie publique du protocole aux dix points de `docs/protocol/README.md`), puis la clause C5 sûre ; ensuite bloc PA (audit du protocole existant aux dix points) puis bloc AA (audit du reste de l'application au regard de `docs/engineering-principles.md` § 0). Détail C5 : rendre sûre la clause C5 du netsim (estimation de perte sur trop peu de paquets, pertes en rafales, faux positifs de QUIC au réordonnancement), décision de l'utilisatrice avant SI1b. Trois causes corrigées à la racine pendant la porte : le plugin de synchronisation de l'éditeur pouvait se désactiver quand un rappel était envoyé pendant une mise à jour CodeMirror, si bien que les textes ne convergeaient plus (défaut antérieur à C5S, corrigé dans `amSync.ts`) ; le temps aller-retour affiché venait du RTT lissé de QUIC, sous-estimé de 8 à 15 % sur trafic dense avec réordonnancement, et il y a désormais une mesure par lien, celle de la sonde sur chemin direct (spec Annexe A.1, `rttMs`) ; le navigateur partageait l'espace de noms réseau du pont WAN (d'où `ERR_NETWORK_CHANGED` au chargement des pages), le commutateur WAN a désormais son propre espace de noms, et le préflight de `done` vérifie les ancres des mutants netsim. 1a. Bloc PB (W50, débloque C5S) : mesures de performance appariées et entrelacées contre la version enregistrée, budgets absolus jugés sur une borne statistique ; puis la non-régression de C5S est rejouée et C5S enregistré. Bloc ST1 ensuite : démarrage accéléré puis budget resserré. 1c. Bloc SB1b (W49, avant OI ; **remplace SB1** depuis le 2026-11-12 : la revue adverse de Codex a montré que la portée mémoire d'ADR 0042 rattachait un processus déjà lancé et que la porte SB1 ne voyait pas une charge restée hors de son cgroup ; `gates/SB1.toml` reste figée comme historique, aucun code n'a été écrit contre elle ; ADR 0042 amendé, porte `gates/SB1b.toml`) : plus aucune fausse alerte de plantage quand le bac à sable arrête typst — `RLIMIT_CORE` {1,1} (garanti pour un `core_pattern` tube ou fichier), SIGXFSZ ignoré, seccomp toujours tué et classé `blocked` sans aucun détail, limite de mémoire par un cgroup où le bac à sable naît (`clone3(CLONE_INTO_CGROUP)` dans une portée systemd déléguée, appartenance et limites relues avant le document), les deux budgets mémoire gardés, repli nommé pour ce qu'il est, message clair dans l'aperçu (`docs/plans/2026-11-11-sandbox-crash-reports.md`, section « Gate SB1b »). 1b. Bloc DU (W48, après C5S et LF0a3) : mises à jour des paquets, Wails 3.1.0 d'abord, gain de performance prouvé par mesure ; quic-go 0.65 à part, car la sonde de lien épingle son comportement. 2. SI1b (indicateur de synchronisation honnête, branche locale `si1b`, porte `gates/SI1b.toml`) reporté sur `main` et sa porte relancée. 3. OI2 et OI1 (W41) : instance unique et verrou (deux processus tournent aujourd'hui avec la même identité ; peut-être la cause du défaut de reconnexion), `%F` du `.desktop`. 3b. W46, blocs CP0 à CP3 (présence par fichier, curseurs et étiquettes, arrivée des modifications sans saut, suivre) ; CP4-CP5 (qui a écrit quoi) après LF0b. 4. W44, blocs DG1-DG2 (paquet `internal/diag`, fichier journal, plantage repris, santé visible ; premier cas : le repli silencieux quand le port 7419 est pris, `internal/app/node.go:88`). 5. Défaut de reconnexion après redémarrage, diagnostiqué avec DG. 6. W42 : LF0a (coloriage incrémental sans gel, Typst corrigé par correctif épinglé, Markdown à sa cause, sans plafond de taille) ; LF0b (analyseur Typst à état explicite pour les délimiteurs non fermés, choix puis spec et porte) ; LF0c (N20, redemandé le 2026-11-11 : coloration juste des autres langages, fichiers de code et blocs de code imbriqués dans Markdown et Typst, chargée à la demande, mêmes preuves d’équivalence et de fluidité que LF0) ; puis recherche dans le Worker et « tout remplacer » en une étape, porte LF1. Après le cœur (W47, décidé le 2026-11-11) : IM1, import simple et versionnable d'un classeur `.xlsx` en CSV (texte, différences visibles dans l'historique) par excelize isolé et borné, le tableau Typst lisant ce CSV ; le classeur gardé en binaire est écarté ; pas d'import Word (N40 maintenue). 7. L'interface dans l'ordre de `docs/plans/2026-11-09-ui-formal.md`. Ensuite ADR 0034 étapes 4-6.

### Suivis de sécurité de L1 (LOW, revue Opus n° 6, à traiter au début de L2)

- ~~Analyse des dépendances~~ : retirée, avec toute l'analyse des dépendances, par la décision de l'utilisatrice du 2026-11-03 (N42 précisée, journal ci-dessous). Seul notre propre code est contrôlé.
- ~~Environnement de tinymist~~ : fait. `runOnce` applique `supervisorEnv` (liste blanche : langue, fuseau, polices) à `cmd.Env` (`internal/sidecar/supervisor.go`, vérifié le 2026-11-08).
- ~~Dossier de projet~~ : fait. `checkWorkdir` refuse un dossier situé dans un dossier secret (`~/.ssh`, `~/.gnupg`, configuration, données et état XDG) et un dossier qui contient un dossier large (`internal/sidecar/workdir.go`, vérifié le 2026-11-08).

## Journal de pilotage (partie historique)

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
