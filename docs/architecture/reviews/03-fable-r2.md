# Revue Fable n°2 de l'architecture (spec 1.0.0-brouillon.3, révision 2, 2026-11-01) — verdict : **NOT ACCEPTED** (4 bloquants, tous mécaniques)

Revue indépendante de `docs/architecture/{spec.md, decisions.md, traceability.md, CHANGES.md,
README.md, research/*.md}` contre `docs/protocol/spec.md` (acceptée), `CLAUDE.md`,
`docs/engineering-principles.md`, les ADR, `docs/exigences.md` (N18–N33 ; N34–N36 enregistrées
depuis, voir §5) et le code (`main` à `72a6101`).

**Méthode.**
- Chaque constat du tour 1 (Fable B-1…m-9, Codex B1…m1) relu contre le texte **exact** de la
  révision 2, pas contre la table de `CHANGES.md` ; un constat est « fermé » si la règle existe,
  est vérifiable et ne contredit rien d'autre dans le document.
- Les neuf désaccords D-1…D-9 tranchés sur preuve, en cherchant la meilleure option, pas la mienne.
- Portée nouvelle (§6.8, §7.3, §9.10, §15.10, §15.14, §15.17–§15.21) confrontée aux sources
  primaires revérifiées le 2026-11-01 : documentation Landlock du noyau (ABI 8, `TSYNC`), `unix(7)`
  (`SO_SNDBUF` borne les datagrammes), sources de Go 1.27.3 (`$GOROOT/src/runtime/os_wasip1.go`,
  `syscall/fs_wasip1.go`, `cmd/link/internal/wasm/asm.go:430`), proposition `go:wasmexport`
  (golang.org/issue/65199, réentrance), BIP-39, billet Typst 0.15 (HTML **expérimental**),
  `beautiful-mermaid` (rendu Mermaid sans DOM), Javy, LWN sur `LANDLOCK_RESTRICT_SELF_TSYNC`.
- Grille inchangée : norme ou règle du projet, preuve (fichier:ligne ou source), gravité.

**Bilan.** Le tour 1 est **réellement** fermé sur le fond : les quatre bloquants et les huit majeurs
de Fable, les cinq bloquants et sept majeurs de Codex ont chacun une règle nommée, vérifiable, au
bon endroit. La révision 2 est nettement meilleure que la 1 : §9.10, §7.3, §15.20, §6.7 réécrit,
§15.19 et la matrice des décisions sont ce que les deux revues demandaient. Les constats nouveaux
portent sur des **erreurs de contenu dans du texte devenu normatif** (ABI, IPC, enfermement,
delta du protocole, phrase de récupération) : aucune ne remet en cause une décision d'architecture,
toutes se corrigent en une courte révision 3 sans rouvrir de débat. C'est précisément parce que ces
sections sont maintenant « exécutables » qu'il faut qu'elles soient justes avant acceptation.

---

## 1. Vérification de fermeture du tour 1

### 1.1 Constats Fable

| # | Changement annoncé | Vérifié dans le texte | État |
|---|---|---|---|
| B-1 | `frame` retiré, Mermaid différé (P-FRAME), SC-11 | §15.14 point 2 (conditions P-FRAME nommées et prouvables), §15.5 `render.frame` et `ui.canvas` différées, SC-11 présent, AD-70 rév. 2 | **fermé** |
| B-2 | PD-1, PD-2, `renders` sans delta | §15.19 table PD avec vecteurs ; §15.10 « N23 n'est pas implémentable » tant que PD-1 n'est pas acceptée ; AD-83 | fermé sur la forme ; **résiduel R2-B3** (règle de pli à la réception absente de PD-1) |
| B-3 | §9.10 H-1…H-8, memfd, pas de cache disque, AppArmor, 512 Mio, échec fermé visible | §9.10 complet ; §4.2 `internal/plugin` dit encore « cache de compilation wazero (`<UserCacheDir>`) » (spec:497) — contradiction avec H-3 | fermé sur la forme ; **résiduels R2-B1** (Landlock, seccomp, `SOCK_SEQPACKET`) et **R2-m1** |
| B-4 | niveau « projet » = contribuer seulement | §15.10 table permis/interdit, `EXT-ND2` étendue au contenu (§15.11 ND-2), mandat, AD-69 rév. 2, question 3 de l'annexe reposée | **fermé** |
| M-1 | banc avec témoin | annexe P-PLUG-BENCH : témoin option 3, trajet complet, p50/p95/p99, arm64, 1 et 4 Mio, réentrance, RSS par hôte ; budget interaction ≤ 100 ms (§15.4 `panels`) | **fermé** |
| M-2 | canal hôte ↔ enfant | §7.3 IPC-1…6, AD-86, chapitre 18 du plan | fermé sur la forme ; **résiduel R2-B1** (SEQPACKET vs 4 Mio) et **R2-m2** (deux sens de « compartiment ») |
| M-3 | API étagée, DX-5 recalculé | §15.21 (13 interfaces en 1.0), DX-5 par étape ; mais la table « Mesures » de §15.15 dit encore « ≤ 60 fonctions, ≤ 40 types » (spec:2111) | fermé ; **R2-m1** (incohérence 40/45 types) |
| M-4 | ST-3 par personne, ST-7 honnête | §6.7 ST-3 (HKDF, domaines), ST-7 (appareil neuf : non vérifiable, résiduel écrit), tailles visibles ; mais §6.1 (spec:694) dit encore « par projet et par appareil » | fermé ; **R2-m1** ; **R2-B4** sur ST-8 (arithmétique) |
| M-5 | PV-6 sans promesse | §15.16 PV-6 réécrit, TLV retiré (§15.19) | **fermé** |
| M-6 | racines du ménage | §6.5 « Racines du ménage des blobs » ; §15.14 point 5 (orphelins, « Nettoyer les rendus ») ; §4.2 retrait d'un greffon global (spec:500) | **fermé** |
| M-7 | G-0a/b/c | §6.6 table, FM-OPEN-1…8, AD-77, ligne §16 ; `App.svelte:197` et `Sidebar.svelte:17` confirmés inchangés ; `project.ReadText` (`internal/project/project.go:345`) existe bien pour servir G-0a en lecture seule | **fermé** |
| M-8 | anti-rebond `tokens` | §15.4 ligne `tokens` (150 ms, annulation, dernières plages, coupure 2 Mi) | **fermé** |
| m-1…m-9 | voir CHANGES | m-1 §15.14 intro ✓ ; m-2 polices ✓ ; m-3 sans objet ✓ ; m-4 `strings.toml` ✓ ; m-5 §9.10 « Échec fermé, visible » ✓ (voir R2-m5) ; m-6 §2 ✓ ; m-7 TH-2 ✓ ; m-8 PP-6 ✓ ; m-9 ✓ | **fermés** |

### 1.2 Constats Codex

| # | Vérifié dans le texte | État |
|---|---|---|
| B1 ABI | §15.20 ABI-1…8 normatif ; réacteur `_initialize` (Go ≥ 1.24 `-buildmode=c-shared` : confirmé dans `cmd/link/internal/wasm/asm.go:430`) ; réentrance `host_call` → `inkwell_alloc` : supportée par Go (issue 65199 : l'appel réentrant tourne sur une goroutine neuve) ; « déterministe » corrigé (émission §4.2.1, décodage strict) ; PV-7 ; AD-74 honnête | fermé sur la forme ; **R2-B2** (liste WASI tolérée incompatible avec le runtime Go) |
| B2 Mermaid | comme Fable B-1 ; conditions P-FRAME (navigation refusée par l'hôte, boucle infinie, `MessagePort` par instance, source canari) toutes présentes | **fermé** |
| B3 fraîcheur | §6.7 ST-3…ST-8, AD-67 rév. 2 : clés, nonces, AAD, graphe, deux têtes, liste incomplète, résiduel écrit, autorité jamais rétablie | fermé ; **R2-B4** (ST-8) |
| B4 confinement, autorité IPC | H-4 compartiments, IPC-3 (le parent impose), `PLUG-COMPROMISED-*`, honnêteté sur l'évasion native | fermé sur la forme ; **R2-B1**, **R2-m2** |
| B5 deltas | §15.19 ; `renders` : validation à l'interprétation, justifiée | fermé ; **R2-B3** |
| M1 mandat | §15.10 « Mandat » (commande, documents, 30 s, compteur, révocation), IPC-5, SC-13 | **fermé** |
| M2 session | MF-19…MF-24, FM-OPEN-4…8 | **fermé** |
| M3 CSV | MF-25, AD-52 rév. 2 (oracle à identités de lignes remplaçant `collab.test.ts:217–220`, vérifié) | **fermé** |
| M4 écarts | decisions.md « Matrice des décisions finales », AD-84, AD-14 et AD-26 avec amendement | **fermé** |
| M5 coûts | banc avec témoin ; §15.21 ; « compilé » / « compilation comprise » distingués (§15.4 `viewers`) | **fermé** |
| M6 flux | §15.5 règle fermée I0–I3 × O0–O4, RF-1…RF-5 ; `import.read` (I3), `self.read`/`presence.read` (I1), entrée des `renderers` (I2) couverts ; secrets injectés par Go | **fermé** (voir R2-m7 sur O0 « visible ») |
| M7 a11y | §15.17 « Vue accessible de l'aperçu », AD-44 rév. 2 | fermé ; **R2-m4** (export HTML Typst encore expérimental : repli exigé) |
| m1 marques | §0.2 héritage, §15 [cible], §3.1 flux complétés, README « ne décode jamais Automerge » | **fermé** |

---

## 2. Les neuf désaccords (D-1…D-9) : position et décision proposée

| # | Position Fable tour 2 | Avec qui | Pourquoi |
|---|---|---|---|
| **D-1** portée de l'ABI | **Avec l'auteur** : noyau normatif ici (§15.20), interfaces et registres dans `docs/plugin-api/` | auteur (= Codex sur le fond) | Codex avait raison : sans ABI-1…8, deux implémentations ne peuvent pas converger ; le noyau tient en une page, le reste est du volume. Condition : corriger R2-B2 (sinon le vecteur Go d'ABI-8 est refusé par ABI-3). |
| **D-2** Mermaid | **Avec l'auteur** : différer tout rendu à DOM (P-FRAME) | auteur | L'option de Codex (« moteur **livré** et de confiance dans un cadre ») ne retire pas le risque : la **source** du diagramme vient d'un pair byzantin et Mermaid la transforme en DOM et CSS (CVE-2026-41148/41149) dans le processus web qui tient le pont ; un moteur de confiance qui lit une entrée hostile dans ce processus viole K-8 autant qu'un greffon hostile. Donc un Mermaid livré passe par la **même** porte P-FRAME : les deux positions convergent. **Au-delà** : la bonne voie pour la v1 est l'« alternative suivie » du §15.14, qui existe aujourd'hui : `beautiful-mermaid` (TypeScript, rendu SVG **sans DOM**, sous-ensemble flowchart/sequence/state/class/ER, MIT, ≈ 11 k étoiles) compilé par le kit JavaScript de l'API 1.1 (Javy, déjà cité en §15.15) devient un `renderer` `wasm` pur, dans l'hôte enfermé, sans P-FRAME. À inscrire comme exemple visé de l'API 1.1 (mesurer la taille du module). |
| **D-3** carte `renders` | **Je change d'avis : avec l'auteur et Codex** — fichiers ordinaires, validation à l'interprétation, aucun delta | auteur | « Ne pas toucher au protocole accepté pour une vue » est le bon critère. **Condition R2-B3 (b)** : `index.json` est un document **texte** Automerge écrit par plusieurs personnes ; une fusion de deux insertions concurrentes dans un objet JSON produit un texte invalide (`}{`, virgules), et « lecture stricte » casserait alors **tous** les rendus. Le format doit être tolérant à la fusion de texte : **JSON Lines**, une entrée par ligne, terminée par `\n`, auto-suffisante, lignes invalides ignorées et comptées (même motif que les trames ignorables). |
| **D-4** PV-6 | **Avec l'auteur et Codex** : retirer la promesse | auteur | Codex avait raison : une version annoncée par un pair byzantin ne prouve rien ; le repli à l'ouverture (ND-7) suffit. |
| **D-5** nombre d'hôtes | **Avec l'auteur** (H-4 par classe de sortie), sous deux bornes | auteur | H-4 satisfait Codex (compartiment par confiance) **et** ma crainte de sur-ingénierie, à condition (1) que les hôtes soient **paresseux** (créés à la première activation, aucun au démarrage — cohérent avec « 0 ms avant la première peinture ») et (2) que leur nombre soit borné et budgété au §2 (R2-m3). Avec AD-01 (un projet), la base est 2 hôtes (projet, globaux O0) + un par greffon à sortie. |
| **D-6** CSV | **Avec Codex** ; MF-25 maintenant | Codex = auteur | Je n'avais pas relevé AD-52 au tour 1 : le comptage de champs ne voit pas une valeur déplacée. MF-25 (détection au niveau des opérations, oracle à identités de lignes) est la bonne réponse sans changer de modèle. Une précision : « lignes touchées localement » se calcule depuis les opérations Automerge concurrentes (patches avec acteur), pas depuis un diff texte ; à dire dans MF-25. |
| **D-7** a11y de l'aperçu | **Avec Codex** ; Vue lecture | Codex = auteur | SC 1.1.1 exige un équivalent ; j'avais seulement regardé les vues déclaratives. Résiduel R2-m4 : l'export HTML de Typst 0.15 est **expérimental** (`--features html`, billet 0.15 et LWN) et ne couvre pas tout document ; la Vue lecture doit avoir un repli garanti (plan `typst query` des titres + texte source du nœud) pour que le parcours Orca ne soit jamais vide. |
| **D-8** plusieurs projets | **Avec Codex et l'auteur** : décision de l'utilisatrice, inscrite dans la matrice | auteur | La matrice existe (decisions.md:119). Sur le fond je recommande l'option 1 (un projet) **parce que** l'architecture garde l'option 2 ouverte (sessions par (PeerID, ProjectID), hôte par projet H-4a, `app.Host`) : rien à réécrire plus tard. |
| **D-9** GREASE | **Avec l'auteur** : seulement dans `host.features()` et injecté par le kit de conformité | auteur | Satisfait la demande de Codex (négociation définie, PV-3/PV-5/ABI-5) sans graisser un canal à hôte unique. |

Bilan : sur les neuf, huit convergent sur la recommandation de l'auteur ; je rejoins Codex sur
D-6 et D-7 ; **aucun désaccord ouvert** entre reviewers ne subsiste. Trois recommandations sont
conditionnées à une correction (D-1 → R2-B2, D-3 → R2-B3, D-7 → R2-m4).

---

## 3. Portée nouvelle : constats

### 3.1 Bloquants (tous mécaniques, aucun ne change une décision)

#### R2-B1 — §9.10 et §7.3 ne sont pas encore « exécutables » : trois défauts de contenu

- **Norme.** engineering-principles §4 (« échec fermé si l'enfermement manque », preuve par test) ;
  documentation Landlock du noyau ; `unix(7)`, `socket(7)` ; ADR 0033 ; §9.10 lui-même :
  « Exécutable et vérifiable, comme §9.5 ».
- **Preuves.**
  1. **Landlock n'a pas la portée annoncée.** H-1 : « l'enfant le pose lui-même au démarrage ».
     `landlock_restrict_self` ne restreint que « le fil appelant et ses enfants futurs » ; les fils
     que le runtime Go a déjà créés avant `main` (sysmon, GC, M de réserve) restent **sans
     restriction** et exécutent des goroutines. Le flag `LANDLOCK_RESTRICT_SELF_TSYNC` règle cela
     mais n'existe qu'à partir de l'**ABI 8** (séries LSM nov. 2025 ; absente d'Ubuntu 24.04
     [6.8, ABI 4] et de Debian 13 [6.12, ABI 6], deux cibles `pkg:test`). L'autre voie — poser Landlock
     **avant** `execve` comme le font déjà les sidecars (`dir_linux.go:39`, `mount_linux.go:121`,
     `setpriv --landlock-access`) — est incompatible avec « aucune règle » : les droits gérés sont
     refusés par défaut, donc `execve` du binaire hôte (`LANDLOCK_ACCESS_FS_EXECUTE`) et la lecture
     des bibliothèques partagées par l'éditeur de liens dynamique (`inkwell` est cgo/GTK) seraient
     refusés. Le texte doit choisir, et je recommande : **binaire hôte distinct, Go pur, statique**
     (`inkwell-plugin-host`, embarqué et extrait en lecture seule comme un sidecar), lancé par le
     parent à travers le chemin `setpriv` existant avec **une seule règle** (`read-file,execute` sur
     ce binaire) et `--no-new-privs` ; `TSYNC` en plus quand l'ABI ≥ 8. Bénéfices annexes : « hôte au
     repos ≤ 40 Mio RSS » (P-PLUG-BENCH) devient atteignable (un ré-exec du binaire GTK/WebKitGTK
     mappe des dizaines de Mio de bibliothèques), `RLIMIT_AS` 4 Gio devient crédible, et AC-16 (wazero
     seulement dans `internal/plugin/host`) se vérifie par le graphe de liens.
  2. **La liste blanche seccomp H-5 tue le runtime Go.** Elle omet `clone`/`clone3` (le runtime crée
     des fils à la demande : premier blocage dans un appel système → nouveau M → `SIGSYS`), `rt_sigaction`,
     `rt_sigprocmask`, `rt_sigreturn`, `sigaltstack`, `tgkill`/`gettid` (préemption asynchrone), `epoll_create1`,
     `epoll_ctl`, `epoll_pwait`, `eventfd2` (netpoller pour la socket), `fcntl`, `getpid`, `set_robust_list`,
     `rseq`, `prctl`/`seccomp` (pour se poser). « Signaux du runtime Go » n'est pas une règle
     exécutable. Correction : la liste **normative** vit dans `internal/plugin/host/seccomp_linux.go`,
     est **dérivée** par un test qui lance l'hôte sous `strace -f` en CI (comme le profil par défaut
     de Docker ou `SystemCallFilter=` de systemd), avec `clone`/`clone3` **filtrés par drapeaux**
     (refus de `CLONE_NEWUSER`, `CLONE_NEWNET`, …) ; §9.10 ne garde que les familles et les refus.
  3. **`SOCK_SEQPACKET` ne transporte pas 4 Mio.** IPC-1 choisit `SOCK_SEQPACKET`, IPC-2 permet des
     charges ≤ 4 Mio. `unix(7)` : « `SO_SNDBUF` impose une limite supérieure à la taille des datagrammes
     sortants (valeur doublée moins 32 octets) » ; `SO_SNDBUF` d'un processus sans privilège est plafonné
     par `net.core.wmem_max` (212 992 o par défaut) ; `af_unix.c` rend `EMSGSIZE` au-delà. Une
     proposition de 1 Mio échouerait sur toute machine par défaut. Correction : `SOCK_STREAM` + le
     cadrage par longueur **déjà défini** en IPC-2 (les frontières de message n'apportent rien de plus),
     ou fragments ≤ 64 Kio avec drapeau de suite. `SOCK_STREAM` est plus simple et c'est ce que le
     protocole fait déjà pour ses trames.
- **Gravité : bloquant** (le §9.10 est la seule justification de l'option 4 ; tel qu'écrit, l'hôte ne
  démarre pas et l'isolation annoncée n'est pas celle obtenue). Correction : texte seulement.

#### R2-B2 — ABI-3 refuse le module « bonjour » Go que ABI-8 exige

- **Norme.** §15.20 est « normatif dès maintenant » ; ABI-8 impose un vecteur Go ; ABI-3 : « toute
  autre importation → refus à l'installation ».
- **Preuve.** `$GOROOT/src/runtime/os_wasip1.go` (Go 1.27.3) importe `wasi_snapshot_preview1`
  `args_get`, `args_sizes_get`, `clock_time_get`, `environ_get`, `environ_sizes_get`, `fd_write`,
  **`poll_oneoff`**, `proc_exit`, `random_get`, **`sched_yield`** ; `syscall/fs_wasip1.go` importe
  en plus `fd_prestat_get`, `fd_prestat_dir_name` (parcourus dans un `init()` dès que `os` est
  lié, donc par tout greffon qui utilise `fmt`), `fd_read`, `fd_close`, `fd_fdstat_get`, `path_open`…
  (liés seulement si atteignables). `poll_oneoff` et `sched_yield` manquent à ABI-3 : le vecteur Go
  est refusé. Deuxième défaut : « `clock_time_get` … sinon 0 » : le runtime Go s'en sert pour
  `nanotime` ; une horloge figée à 0 bloque `time.Sleep`, `time.After`, le pas du GC et tout
  `poll_oneoff` à échéance.
- **Correction.** (1) Dériver la liste tolérée des **importations réelles** des deux modules « bonjour »
  d'ABI-8 (Rust `wasm32-wasip1`, Go `c-shared`) et la figer avec une sémantique de neutralisation
  par fonction : `poll_oneoff` (abonnement horloge → attente bornée par le délai de l'appel ; fd →
  `EBADF`), `sched_yield` (no-op), `fd_prestat_get` → `EBADF`, `fd_read` sur 0 → EOF, `path_open` →
  `ENOTCAPABLE`, le reste → refus. (2) Sans `time.read` : **horloge logique monotone** (compteur qui
  avance d'un pas fixe à chaque appel, ou horloge hôte quantifiée à 100 ms), jamais 0. (3) Ajouter au
  kit de conformité le test « module Go minimal accepté, `time.Sleep(0)` et `fmt.Println` ne bloquent
  pas ». Note : l'annexe (*Décision* point 2) cite encore « `now`, `random`, `yield` » parmi les
  six fonctions hôte alors qu'ABI-3 passe par WASI : aligner.
- **Gravité : bloquant** (contradiction interne d'un texte normatif) ; correction : texte.

#### R2-B3 — PD-1 manque la règle de pli côté réception ; `index.json` n'est pas sûr sous fusion

- **Norme.** Protocole §1.4.4 (« un nœud qui ne connaît pas cette mineure passe le projet en lecture
  seule… au lieu de calculer un autre pli ») ; ADR 0030 (pli identique partout) ; feedback du projet
  « protocole d'abord, chaque échange spécifié avant de patcher ».
- **Preuves.** (a) PD-1 (§15.19) gère l'**émission** (« seulement après `format ≥ 1` ») et la
  **tolérance** (entrée inconnue ignorée), mais ne dit pas ce qu'un appareil **2.1** fait d'une entrée
  `plugin/<id>` reçue alors que le pli porte `format = 0` (émise par un appareil 2.1 bogué ou
  byzantin) : s'il l'applique, les appareils 2.1 chargent un greffon que les 2.0 ignorent — deux plis.
  La règle manquante : **une entrée `plugin/` ou `ai/` n'a d'effet que si `format ≥ 1` dans le pli
  au moment de son application ; sinon elle est conservée et ignorée par tous** ; vecteur
  `policy.json#plugin-entry-before-format`. (b) `.renders/index.json` : voir D-3 ; un document texte
  Automerge à plusieurs écrivains n'est pas un JSON sûr ; passer en JSON Lines (une entrée par ligne,
  lignes invalides ignorées et comptées, bornes par ligne et par fichier). Au passage, la clé de rendu
  ne couvre pas les **polices** du projet (`--font-path`) alors que le point 1 en fait une entrée du
  rendu : ajouter leur empreinte à la clé, sinon un changement de police laisse un rendu périmé non
  détecté.
- **Gravité : bloquant** pour (a) (c'est le texte qui part vers la boucle du protocole ; une règle
  de pli fausse y coûterait un tour) ; majeur pour (b). Correction : texte, deux phrases et un
  vecteur.

#### R2-B4 — ST-8 est arithmétiquement impossible, et ST-7 (a) repose dessus

- **Norme.** BIP-39 : 24 mots = 256 bits d'entropie + 8 bits de somme de contrôle, rien d'autre.
- **Preuve.** ST-8 : « 24 mots (BIP-39…), encodant la clé de stockage [256 bits], le `ProjectID`
  [256 bits, protocole §5.3], le dernier compteur et sa date, avec somme de contrôle ». Il manque
  ≥ 300 bits. L'ancre de fraîcheur (a) de ST-7 (« la phrase porte le compteur et la date ») n'existe
  donc pas telle quelle.
- **Correction.** La phrase encode la **clé seule** (24 mots) ; le `ProjectID` se retrouve par le
  dorsal (les têtes `heads/<PeerID>/<compteur>` sont listées et l'AAD le porte) ou se choisit dans la
  liste ; l'ancre de fraîcheur devient un **second élément montré avec la phrase** : « Sauvegarde
  n° 42 du 12 mars », saisi (ou scanné) à la restauration et comparé à la tête servie ; ou, si l'on
  veut une seule chaîne, 27 mots (24 + 3 pour compteur 20 bits et jour 13 bits), hors BIP-39, à
  vectoriser. Réécrire ST-7 (a) en conséquence.
- **Gravité : bloquant** (une règle normative fausse sur la seule ancre de fraîcheur d'un appareil
  neuf) ; correction : texte.

### 3.2 Majeurs

- **R2-M1 — §9.10 « Ailleurs : greffons tiers refusés » contre N35.** N35 (enregistrée le
  2026-11-01, après la révision 2) exige que « toute décision (bac à sable des plugins…) tienne sur
  Linux, macOS et Windows ». La révision 3 ne peut plus écrire « ailleurs : — » dans la table H-1…H-8 :
  il faut une colonne macOS (`sandbox-exec`/profil SBPL, ou App Sandbox) et Windows (AppContainer +
  Job object : mémoire, CPU, pas de réseau), ou une décision explicite « hôte enfant sans enfermement
  noyau = niveau de confiance réduit : capacités O2/O3 refusées » (ce qui garde l'option 4 utile :
  la clé reste hors du processus hôte). AD-18 doit être rouverte par cette exigence, pas contournée.
  Norme : N35 ; engineering-principles §4. Gravité : majeur (nouvelle exigence, hors du tour 2 au
  sens strict, mais la table H-* est le lieu où elle se joue).
- **R2-M2 — Budget système des hôtes absent du §2.** Chaque hôte : 512 Mio linéaires + 640 Mio
  `SetMemoryLimit` ; H-4 en crée 2 + k. §2 « processus Go ≤ 300 Mo » ne les compte pas, et N32
  (gestionnaire de tâches) voudra la somme. Ajouter une ligne : « hôtes de greffons : ≤ 40 Mio RSS au
  repos chacun, ≤ 6 hôtes (au-delà : refus d'activation avec message), somme ≤ 1,5 Gio linéaires »,
  mesurée par P-PLUG-BENCH (RSS **et** VSZ, à cause de `RLIMIT_AS`). Norme : engineering-principles §3.
- **R2-M3 — Consentement à l'envoi IA : « premier envoi d'une session » seulement.** §15.18 et RF-3
  promettent « l'aperçu de ce qui part » ; le texte ne le montre qu'une fois par session, alors que
  des plages `autrui` nouvelles peuvent entrer plus tard. Règle demandée : aperçu au premier envoi
  **et** à chaque envoi qui introduit une provenance ou un fichier non encore montrés dans la session ;
  indicateur permanent « IA : envoi vers X » dans l'en-tête. Norme : RF-3, IA-1 ; WCAG 3.2.x
  (prévisibilité). Gravité : majeur (c'est la garantie affichée à l'utilisatrice).

### 3.3 Mineurs

- **R2-m1 — Restes de la révision 1** : §4.2 `internal/plugin` « cache de compilation wazero
  (`<UserCacheDir>`) » (contredit H-3) ; §6.1 « clé de stockage distant par projet et par appareil »
  (contredit ST-3) ; §15.15 « Mesures » « ≤ 40 types » (DX-5 dit 45) ; traceability.md N22
  « règle d'exclusion `CAP-EXCL` » (devenue règle de flux RF-*). N36 (passe de cohérence) les
  attrapera ; les lister ici évite qu'elles survivent.
- **R2-m2 — Deux sens de « compartiment ».** H-4 : un compartiment = un hôte ; IPC-2/IPC-3 : un
  `compartiment u16` par activation **dans** l'hôte. Nommer le second « créneau » et écrire la règle
  qui rend `PLUG-COMPROMISED-*` significatif : la table (créneau, génération) est **indexée par la
  socket d'arrivée** ; un créneau revendiqué dans une trame n'est jamais cherché hors de l'hôte dont
  vient la socket. Préciser aussi que les `memfd` ultérieurs passent par `SCM_RIGHTS` (`recvmsg` est
  bien dans H-5).
- **R2-m3 — Hôtes paresseux.** Dire explicitement qu'aucun hôte n'existe avant la première activation
  d'un greffon de sa classe (cohérent avec « 0 ms avant la première peinture » et ≤ 10 ms pour
  50 extensions).
- **R2-m4 — Vue lecture : repli.** L'export HTML de Typst 0.15 est expérimental et peut échouer sur un
  document (mise en page non convertible). Exiger un repli déterministe : plan (`typst query` des
  titres) + texte du nœud source, dans le même `iframe`, pour que le parcours Orca ne soit jamais
  vide ; et noter que `--features html` est un drapeau expérimental épinglé avec la version 0.15.1
  du manifeste.
- **R2-m5 — Tâches en WebAssembly.** Une fonction du produit (U29) qui dépend de l'enfermement Linux
  devient indisponible sur un noyau sans Landlock (< 5.13) et, par N35, sur macOS et Windows. Soit
  Tâches reste une extension intégrée TypeScript et Graphviz porte seul le dogfooding WebAssembly,
  soit la règle dit qu'un module **livré dans l'exécutable** (signé par la construction) peut tourner
  sans H-1/H-5 avec ses seules capacités (il est aussi digne de confiance que l'exécutable). Je
  recommande la première : plus simple.
- **R2-m6 — `compile.run` et `compile.read`** partagent une ligne au §15.5 ; §15.10 n'accorde que
  `compile.read` au niveau « projet ». Scinder la ligne (run = PDF, pages, `typst query` ; read =
  ancres et diagnostics du dernier aperçu), sinon un lecteur peut croire qu'un greffon de projet
  compile.
- **R2-m7 — O0 « visible » et deux UX de proposition.** Pour un greffon global, `document.propose`
  s'applique puis s'annule (§15.5) ; pour l'IA, la proposition se relit en diff avant d'entrer
  (§15.18). Deux comportements pour un même concept (U31, cohérence). Recommandé : une seule règle,
  celle de l'IA (diff, Accepter/Refuser), avec « appliquer sans relire » comme réglage par greffon
  choisi par la personne. Préciser aussi que `document.create` sous O0 crée des fichiers que personne
  n'ouvre : au moins une entrée dans Activité (« X a créé `.renders/…` »).
- **R2-m8 — §9.5 et H-6.** Le profil AppArmor `userns` livré pour l'hôte résout aussi le repli
  permissif de `netns_linux.go:22–34` pour `typst` et `tinymist` ; §9.5 doit y renvoyer (et LSP-1
  cesser de s'appuyer sur un espace de noms réseau qui peut manquer).
- **R2-m9 — ABI-2 / ABI-4 tailles.** « 4 Mio par défaut » (ABI-4) et « charge ≤ 4 Mio » (IPC-2)
  laissent la réponse d'un `host_call` (`document.read` d'une plage) sans borne propre : fixer
  « sortie d'un appel hôte ≤ 1 Mio sauf `document.read` ≤ 4 Mio », et dire que l'hôte refuse d'écrire
  dans la mémoire du greffon au-delà de `memory.size()` (vérification avant `inkwell_alloc`).

### 3.4 Sur-ingénierie (à alléger, non bloquant)

- **UTCP** (§15.18) : un manuel exporté « option de la dernière étape, désactivé par défaut, avec test
  de non-divergence » pour un protocole sans hôte majeur (AD-82 le dit) : retirer de la spec jusqu'à ce
  qu'un hôte l'adopte ; une ligne dans §15.12 « écarté, réexaminer si » suffit.
- **H-4 (c)** un hôte **par** greffon à sortie réseau : juste sur le principe (RF-1), mais la somme
  peut grimper ; la borne « ≤ 6 hôtes » de R2-M2 suffit à contenir le coût.
- **P-PLUG-BENCH** : trois chaînes d'outils (Rust, Go `wasip1`, TinyGo) **et** arm64 **et** témoin : le
  témoin et arm64 sont nécessaires (M-1) ; TinyGo peut attendre le kit 1.1 (le vecteur Go est Go
  standard).
- Ce qui n'est **pas** de la sur-ingénierie et qu'il faut garder : §9.10 par compartiments, le mandat
  (§15.10), la règle de flux fermée, JSON Lines pour les rendus, la Vue lecture.

---

## 4. Crochets pour N32 (gestionnaire de tâches) et N33 (RAG natif) — révision 3

### 4.1 N32 — « gestionnaire de tâches façon Firefox » et limites de ressources

Ce que la révision 2 fournit déjà et qu'il ne faut pas dupliquer : H-7 (mémoire par module et par
hôte), IPC-4 (files, appels en vol), IPC-5 (`cancel`), ND-5 (« trop lent, mis en pause »), O-4
(compteurs en mémoire, rien ne sort), §5.2 (table des acteurs). Crochets à poser :

1. **Une seule table des « travaux »** dans Go (`internal/tasks` ou dans `app`), alimentée par chaque
   propriétaire du §5.2 **plus** : sidecars (§9.5), hôtes et créneaux (§9.10, §7.3), Worker Automerge
   (ADR 0028 D2), flux IA (Stream `ai`), serveur MCP, agent ACP, indexeur (N33). Chaque entrée :
   identifiant stable, **nom sans jargon** (glossaire : « Dessin du diagramme », pas « plugin host »),
   classe, CPU cumulé, mémoire, dernier appel, état, et un **verbe** parmi `annuler | mettre en pause
   | arrêter | désactiver`. Firefox `about:processes` est le modèle ; comme lui, la mesure vient du
   **parent** (`/proc/<pid>/stat`, `getrusage`, cgroup) pour tout ce qui est un processus, et les
   chiffres par créneau rapportés par l'enfant (`memory.size()`, temps par appel) sont **indicatifs**
   (l'enfant est compromis : §7.3).
2. **Limites réglables** : H-7 donne des plafonds déclarés dans le manifeste ; N32 demande que la
   personne puisse les **baisser** (jamais les monter au-delà du manifeste) : un schéma `settings` du
   cœur par classe (greffons, IA, compilation, index) avec les valeurs H-7 par défaut ; l'hôte relit
   la limite à l'activation suivante.
3. **cgroup v2 par hôte** quand la session le permet (délégation `systemd --user`, comme
   `scripts/heavy.sh`) : seule borne CPU et mémoire **imposée par le noyau** par processus ; sinon
   `RLIMIT_*` (déjà prévu). À inscrire en H-9 avec « quand disponible, montré dans Détails
   techniques », jamais en échec fermé.
4. **Mesures sans contenu** : le gestionnaire ne lit que O-4 ; aucun nom de fichier ni de document
   dans une ligne (O-2) ; preuve par le test canari O-7 étendu.
5. **Emplacement UI** : un onglet du tiroir, contribué par le cœur sur le point `panels` (dogfooding),
   avec une colonne d'état dans la barre d'état (`status`) ; la zone « Extensions » (§15.13) y renvoie.

### 4.2 N33 — RAG natif (étude `research/ai.md` §12 déjà écrite, alignée avec ceci)

1. **Provenance par extrait** : la règle des deux (IA-2) ne tient que si chaque extrait rendu par
   `index.chercher` porte `soi | autrui | externe` ; le message `index` (webview → Go) doit transporter
   la provenance par intervalles et l'index la stocker ; un extrait mixte est `autrui`. C'est **le**
   crochet : sans lui, la récupération contourne IA-2.
2. **Classe de flux** : l'index est une **source I2** (contenu du projet) ; `index.*` exige
   `document.read` ; les embeddings et le reranking sont un **puits O4** (envoi par Go, politique,
   aperçu), le lexical est O0. À inscrire dans la table I/O de §15.5 pour que RF-3 reste fermée.
3. **Dérivé, local, borné** : `internal/index` possède un magasin **par appareil et par projet**, hors
   `.inkwell/` et hors protocole, reconstructible, supprimé à la fermeture ou au retrait ; quota
   (§6.5) et bornes par document (un pair byzantin ne gonfle pas l'index) ; `ai: never` exclu.
4. **Une source de vérité** (INV-3) : l'index lit l'**instantané** envoyé par la webview, jamais
   Automerge (INV-1) ni la projection seule ; `rev` et condensat par extrait ; `lire_plage` revérifie.
5. **Acteur et budget** : un acteur de plus au §5.2 (« indexeur », 1 goroutine + file bornée), hors du
   chemin de frappe, `PERF-IDX-*` ; et une ligne N32 (CPU ≤ 1 cœur, ≤ 150 Mo).
6. **Chiffrement au repos de l'index** : il contient une copie du texte ; même niveau que le projet
   (NG3 aujourd'hui : chiffrement du disque recommandé) ; ADR à prévoir plutôt qu'une promesse.
7. **Surfaces** : les cinq commandes `index.*` sont des commandes du registre (classe `lire`), donc
   disponibles à la palette, aux greffons, à MCP (portée `lecture`) et à ACP sans surface nouvelle.

### 4.3 N34, N35 (enregistrées dans le même lot, à traiter en révision 3)

- **N34** (fichiers ignorés façon `.gitignore`) : touche MF-12 (projection), `project.Watch`, MF-15
  (classement) et l'index N33 ; une règle « un fichier ignoré n'est ni annoncé (ADR 0031) ni
  projeté ni indexé, mais reste visible dans la liste avec un badge » et un vecteur de motifs partagé
  Go/TS (comme le reniflage MF-15).
- **N35** : voir R2-M1 ; concerne aussi §9.5 (sidecars), §6.7 (trousseau), §15.18 (socket Unix
  `0600` → sur Windows, tube nommé avec ACL).

---

## 5. Ce qui est bien dans la révision 2 et qu'il ne faut pas toucher

§6.7 réécrit (clés, AAD, graphe, deux têtes, fraîcheur honnête) ; §9.10 par compartiments avec
l'honnêteté sur l'évasion native ; le mandat de §15.10 ; la règle de flux fermée ; §15.19 comme seul
lieu des deltas ; §15.20 (sauf R2-B2) ; §15.21 et l'ordre d'implémentation ; MF-19…MF-25 ; G-0a
immédiat ; §6.8 (modèles = projets, jamais `typst init`) ; §15.17 (LSP-3 réponses périmées jetées,
ER-4 jamais de HTML) ; §15.18 (fournisseurs par Go, politique signée après PD-1, règle des deux, MCP
sans port, ACP recevant **notre** MCP) ; la matrice des décisions finales.

---

## Verdict : **NOT ACCEPTED**

Bloquants, tous corrigeables par du texte en une révision 3 courte, sans rouvrir de décision :
**R2-B1** (Landlock : portée des fils et `execve` ; seccomp : liste dérivée et `clone` filtré ;
`SOCK_STREAM` au lieu de `SOCK_SEQPACKET`), **R2-B2** (liste WASI tolérée dérivée des vecteurs Go
et Rust ; horloge logique au lieu de 0), **R2-B3** (règle de pli à la réception dans PD-1 ; JSON
Lines pour `.renders/index.json` ; polices dans la clé), **R2-B4** (ST-8 : phrase = clé seule, ancre
de fraîcheur séparée ; ST-7 (a) réécrit). Majeurs R2-M1…M3 attendus dans la même révision ;
mineurs au fil de l'eau ; crochets N32/N33 (§4) à intégrer en révision 3 avec N34–N36.
