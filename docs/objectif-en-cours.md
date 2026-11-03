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
| Porte | `mise run gate -- L2` : (1) e2e `files.spec.ts` FM-OPEN-1 (clic → contenu du bon fichier), FM-OPEN-2 (sous-titre), FM-OPEN-3 (txt, csv, md, typ distingués ; lecture seule avec bandeau pour les autres que `main.typ`) ; (2) `?file=` supprimé ; (3) les suivis de sécurité de L1 faits et testés (environnement de tinymist, dossiers protégés ; l'analyse des dépendances est retirée par décision de l'utilisatrice, N42 précisée) ; (4) **PERF-TYPE natif** WebKitGTK (frappe → peinture, froid/chaud) mesuré, seuil = p95 × 1,10 ; (5) portes L0 et L1 vertes |
| Reports | PERF-MEM → lot G-0b (ouverture réelle de plusieurs fichiers) |
| Condition d'arrêt | porte verte, ou décision humaine (comportement de l'interface non prévu par le design) |
| Revue | Sonnet (code) + test en personne `ux-walkthrough` (règle du projet pour toute modification d'interface) |
| Lots terminés | L0 (2026-11-02) ; L1 (2026-11-03) : confinement sans programme externe, approuvé après six revues de sécurité Opus |

### Suivis de sécurité de L1 (LOW, revue Opus n° 6, à traiter au début de L2)

- ~~Analyse des dépendances~~ : retirée, avec toute l'analyse des dépendances, par la décision de l'utilisatrice du 2026-11-03 (N42 précisée, journal ci-dessous). Seul notre propre code est contrôlé.
- Environnement de tinymist : filtrer `cmd.Env` contre une liste blanche dans `runOnce`, même si l'appelant passe `os.Environ()` (`internal/sidecar/supervisor.go:215`).
- Dossier de projet : refuser aussi les dossiers situés **dans** un dossier protégé (`~/.ssh`, `~/.gnupg`), pas seulement ceux qui en contiennent un (`internal/sidecar/workdir.go:44-76`).

## Lots suivants (MVP, dans l'ordre)

1. **L3 — Comprendre, corriger, produire** : tinymist branché (N28, N29) ; porte : `problems.spec.ts`,
   `PERF-LSP`.
2. **Protocole, étapes 2 à 8 d'ADR 0034** puis **G-0b** (index `G`, un document par fichier, FM-OPEN-4…8, PERF-MEM) et **G-0c** (onglets, Worker).
3. Puis : Collaborer sans perdre, Première minute et secours, Revenir en arrière, Fondations
   extensibles (`coherence.md` §5).

## Journal de pilotage

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
