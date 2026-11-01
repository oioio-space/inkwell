# Revue Fable n°3 de l'architecture (spec 1.0.0-brouillon.4, révision 3, 2026-11-01) — verdict : **ACCEPT** (0 bloquant, 5 majeurs de texte, 11 mineurs)

Revue indépendante de `docs/architecture/{spec.md, coherence.md, decisions.md, traceability.md,
CHANGES.md, research/*.md}` contre `docs/protocol/spec.md` (acceptée), `CLAUDE.md`,
`docs/engineering-principles.md`, les ADR (0023, 0024, 0028, 0031, 0033, 0034), `docs/exigences.md`
(N1–N41) et le code (`main` à `4013322`).

**Méthode.**
- Les 11 bloquants du tour 2 (Fable R2-B1…B4, Codex R2-1…R2-6, R2-10) et les 8 majeurs relus contre
  le **texte exact** de la révision 3 (numéros de ligne de `spec.md` cités), pas contre la table de
  `CHANGES.md`. Un constat est « fermé » si la règle existe, est vérifiable, et ne contredit rien.
- D-10…D-16 tranchés sur preuve. Sources revérifiées le 2026-11-01 : série Landlock
  « restrict flags example for ABI version 8 » (lkml, mars 2026 : `LANDLOCK_RESTRICT_SELF_TSYNC`
  = ABI 8 = Linux 7.0) ; `execve(2)` (« all threads other than the calling thread are destroyed ») ;
  documentation Landlock (domaine hérité à travers `execve`, portée = fil appelant et descendants) ;
  `unix(7)` ; dépôt `lukilabs/beautiful-mermaid` (MIT, 11,3 k étoiles, « zero DOM dependencies »,
  **ELK.js** en moteur de disposition « synchronous via FakeWorker bypass ») ; CVE-2026-45022
  (go-git, corrigée en 5.19.0 ; OSV, traqueur Debian) ; C2SP age ; RFC 9106 (Argon2) ;
  `$GOROOT/src/runtime/os_linux.go` (Go 1.27.3 : `clone`, jamais `clone3`).
- Code : `internal/sidecar/{dir_linux.go,mount_linux.go}` (chemin `setpriv`, tmpfs borné `size=`),
  `frontend/src/history/{versions.ts,graph.ts,testkit.ts}` (versions, variantes, `A.merge`),
  `frontend/src/editor/format/keymap.ts:16` (`Mod-k` = lien), `docs/ux-principles.md:14`,
  bancs présents (`internal/{blob,transport/frame,transport/mux,app}/…bench_test.go`,
  `protocol/trust/bench_test.go`, `BenchmarkRelay` dans `internal/app/wiring_bench_test.go:28`),
  `frontend/e2e/` (aucun cas `PERF-*` ; `INKWELL_PERF` seulement dans `sheet.spec.ts` d'un worktree).
- Grille inchangée : norme ou règle du projet, preuve, gravité ; sur-ingénierie signalée.

**Bilan.** Les 11 bloquants et les 8 majeurs du tour 2 sont **réellement fermés dans le texte**, au
bon endroit, sans contradiction résiduelle trouvée. Le §0.5 (expérimentations acceptées à leur
porte) est sain : il n'esquive aucune décision — chaque option est choisie — et il refuse d'appeler
« accepté » ce qui n'est pas mesuré, ce qui est exactement la règle du projet (« pas d'empirisme »,
`engineering-principles` §3). Les constats nouveaux sont des **précisions de texte** (une porte non
mesurable, un budget manquant, une sémantique à fixer) qui ne rouvrent aucune décision ; aucun ne
justifie un quatrième tour bloquant. J'accepte, et je demande que les cinq majeurs entrent dans la
révision 4 au fil de l'eau.

---

## 1. Vérification de fermeture du tour 2

### 1.1 Bloquants (11)

| # | Changement annoncé | Vérifié dans le texte | État |
|---|---|---|---|
| F R2-B1 (1) Landlock, fils, `execve` | H-0 démarrage en deux temps, binaire statique | spec:1495–1501 : `inkwell-plugin-host` Go pur `CGO_ENABLED=0` ; `--stage=lock` : `LockOSThread`, `PR_SET_NO_NEW_PRIVS`, Landlock sur ce fil, `execve` de soi ; `--stage=run` : seccomp `TSYNC` avant toute lecture, auto-vérification, puis `ready` ; H-1 : une règle `execute`+`read_file` sur le binaire (les deux sont nécessaires : `open_exec` ouvre en `FMODE_READ|FMODE_EXEC`), `TSYNC` si ABI ≥ 8 (= Linux 7.0, confirmé). Sémantique exacte : `execve` détruit les autres fils, le domaine Landlock est dans les `cred` du fil appelant, donc hérité | **fermé** |
| F R2-B1 (2) seccomp tue le runtime Go | H-5 familles, refus, `clone` filtré, liste dérivée | spec:1506 : architecture vérifiée, refus nommés, `clone` seulement avec `CLONE_THREAD` sans drapeaux d'espace de noms, `clone3` → `ENOSYS`, liste blanche **dérivée** par `strace -f` en CI dans `seccomp_linux.go`. Go 1.27 n'appelle jamais `clone3` (`os_linux.go`) : « se rabat » est inexact mais sans effet (m-1) | **fermé** |
| F R2-B1 (3) `SOCK_SEQPACKET` contre 4 Mio | IPC-1 flux d'octets | spec:1305–1311 : `SOCK_STREAM`, raison citée (`SO_SNDBUF`, `unix(7)`), tubes sous Windows, `memfd` par `SCM_RIGHTS` une fois, tout autre descripteur ancillaire fermé et compté | **fermé** |
| F R2-B2 ABI-3 refuse le bonjour Go ; horloge 0 | ABI-4 dérivé, neutralisé | spec:2808–2825 : table complète (`poll_oneoff`, `sched_yield`, `fd_prestat_*` → `EBADF`, horloge quantifiée 100 ms « jamais 0 ») ; ABI-9 test du kit « `fmt.Println`, `time.Sleep(0)` » ; les six fonctions hôte de l'annexe alignées (ABI-3 : `host_call`, `host_read`, `log`, `cancelled`) | **fermé** |
| F R2-B3 règle de pli ; `index.json` ; polices | PD-1 point 4 ; JSON Lines ; clé | spec:2757–2760 (conserve et ignore sans `format ≥ 1` dans le passé causal ; vecteur `#plugin-entry-before-format`) ; spec:2288–2294 (`.renders/index.jsonl`, ligne ≤ 4 Kio, ≤ 10 000, invalide ignorée et comptée ; empreinte des polices dans la clé) | **fermé** |
| F R2-B4 ST-8 impossible | phrase = secret seul, kit | spec:1082–1089 : 24 mots BIP-39 « rien d'autre », kit versionné HMAC (`inkwell-recovery/kit/1`) avec compteurs par appareil ; ST-7 (a) réécrit sur le kit (spec:1072–1074) | **fermé** |
| C R2-1 confinement non exécutable | H-0, H-1 masque par ABI, H-5, matrice | spec:1502 (masque ABI 1/2/3/≥4), spec:1522–1524 (Ubuntu 22.04, Debian 12, Ubuntu 24.04, Debian 13, Fedora ; ABI **sondée**), P-SANDBOX-LINUX, enfant natif hostile (spec:1516–1520) | **fermé** |
| C R2-2 autorité, transport | un hôte par greffon, codec | spec:1319–1324 (IPC-3 : autorité = greffon lancé **sur cette connexion**), spec:1312–1318 (gros-boutiste, 8 types, bornes par type), IPC-4 crédits en **octets**, IPC-5 annulation prioritaire, test 4 Mio sans privilège sur six cibles (spec:1335) | **fermé** |
| C R2-3 ABI Go/TinyGo, octets signés | ABI-4, ABI-7, ABI-9 | ABI-7 (spec:2833–2840) : `files` triée par octets du chemin, `signature.cbor` exclu, manifeste signé tel quel, chemins NFC sans `..`, bornes ; ABI-3 sans réentrance + `host_read` ; ABI-1 maximum imposé par l'hôte ; TinyGo en 1.1 « par mesure » ; vecteurs = livrable de P-PLUG-PROTO (§0.5) | **fermé** (expérimentation) |
| C R2-4 PD-1/PD-2 | base64url, schémas, émission, transition honnête ; PD-2 retiré | spec:2744–2769 ; spec:2771–2775 : préfixes externes `inkwell-ext:…\0` — ne peut pas entrer en collision avec `"inkwell/" ‖ domaine ‖ "/v1" ‖ 0x00` (protocole §3.3, ligne 245 : l'octet 8 diffère, `-` contre `/`) | **fermé** |
| C R2-5 récupération, formats, ménage | §6.7 réécrit | ST-4 nom = `hex(BLAKE3 à clé)` 64 car., ST-6 même calcul ; AAD des manifestes et têtes définies (spec:1059–1060) ; compteurs par `(ProjectID, PeerID)` ; **aucun ménage distant en v1** (spec:1096–1101) ; §9.2 ne promet plus la détection universelle du rejeu (grep : « rejeu » ne subsiste qu'au §15.15 pour le débogage) | **fermé** |
| C R2-6 baseline contre N35/N37/N39/N40 | §1.2, §1.3, INV-12, INV-13, AD-13/18/33/47 | G4, G9, G10, G13 (spec:84–91) ; NG1 et NG3 (spec:99–101) ; INV-12, INV-13 (spec:127–128) ; `AD-13` révision 3 (go-git ≥ 5.19, écrivain minimal = témoin), `AD-18` révision 3, `AD-33` révision 3, `AD-47` révision 3 | **fermé** |
| C R2-10 P-PLUG-BENCH non passée | §0.5, `AD-95` | spec:48–63 : hôte, ABI, compartiments = expérimentation ; « aucun code de production n'en dépend » ; résultats bruts dans `docs/plans/` | **fermé** (par rétrogradation honnête) |

### 1.2 Majeurs (8) et mineurs

| # | Vérifié | État |
|---|---|---|
| F R2-M1 « Ailleurs : — » | §9.11 table six cibles, INV-13, réductions écrites (spec:1550–1553) | fermé |
| F R2-M2 budget des hôtes | §2 (spec:150) : ≤ 40 Mio RSS au repos, ≤ 6 actifs, ≤ 1,5 Gio linéaire, global ≤ 2,5 Gio, RSS **et** VSZ | fermé |
| F R2-M3 consentement continu | spec:2611–2617 : premier envoi **et** chaque provenance/fichier/destination nouvelle, interruption sur changement de politique, indicateur permanent | fermé |
| C R2-7 session saturée, G-0a, diagnostics | MF-12 (spec:907), MF-21 (spec:975), G-0a honnête (spec:1013), LSP-3 (spec:2525) | fermé |
| C R2-8 MF-25 | spec:992 : graphe des opérations, toutes paires concurrentes | fermé |
| C R2-9 IA | limites, estimation marquée, coût inconnu, contamination monotone, jeton MCP, MCP désactivé en partagé avant PD-1, ACP sans garantie (spec:2619–2629, 2668–2675, 2711–2723) | fermé |
| C R2-11 crochets N32–N39 | §11.1, §15.18 RG, §6.9, §6.10, §6.11 ; traçabilité N32–N41 (traceability:66–75) | fermé |
| C R2-12 modèles | TM-2 (fichiers choisis, caches exclus), TM-3 (valeurs typées et échappées) | fermé |
| F R2-m1…m9 | m1 : §4.2:528 « aucun cache », §6.1 corrigé, §15.15 « ≤ 45 types » (spec:2434), traçabilité N22 « règle de flux » ✓ ; m2 : un hôte = un greffon, plus de « créneau » ✓ ; m3 : H-4 paresseux ✓ ; m4 : §15.17 repli ✓ ; m5 : Tâches TypeScript (spec:1513) ✓ ; m6, m7, m8, m9 ✓ | fermés |

**Aucun constat du tour 2 n'est ouvert.**

---

## 2. Désaccords D-10 à D-16 : position, preuve, décision proposée

| # | Position Fable tour 3 | Preuve | Décision proposée |
|---|---|---|---|
| **D-10** hôtes | **Je rejoins Codex et l'auteur : un hôte par greffon actif.** Mon « par classe de sortie » du tour 2 laissait un défaut réel : après évasion native, deux colocataires sont indiscernables (CWE-441) ; l'autorité attachée à la connexion (IPC-3) n'a de sens que si une connexion = un principal. C'est le modèle « un processus par principal » de Chromium (isolation de site) et de Firefox Fission. | Coût borné : hôte paresseux (H-4), ≤ 6 actifs, 40 Mio RSS au repos — un binaire Go statique au repos tient en 5–10 Mio, le reste est le module compilé. **Risque mesuré, pas décidé :** sans cache disque (H-3) et un hôte par greffon, **chaque activation recompile** (Graphviz Emscripten ≈ 2–3 Mo de wasm ; un bonjour Go ≈ 2 Mo) ; « ≤ 500 ms compilation comprise » est optimiste pour wazero sur arm64. | **Adopter** un hôte par greffon. Deux précisions de texte : (a) l'éviction H-4 ne touche jamais un hôte qui a un mandat ou un appel en vol ; (b) si P-PLUG-BENCH dépasse 500 ms, le repli nommé **à l'avance** est « module compilé à l'installation et conservé dans l'hôte tant qu'il vit ; pas d'éviction automatique sous 6 », jamais un cache disque. |
| **D-11** Landlock | **Avec l'auteur : démarrage en deux temps**, meilleur que ma voie `setpriv` (qui dépend d'util-linux ≥ 2.40 pour `--landlock-rule` ; Ubuntu 24.04 a 2.39). | `execve(2)` : les autres fils sont détruits ; Landlock : domaine dans les `cred`, hérité ; `PR_SET_NO_NEW_PRIVS` survit à `execve` ; seccomp `TSYNC` depuis 3.17 ; Landlock `TSYNC` = ABI 8 = Linux 7.0 (lkml mars 2026), donc hors de toutes les cibles de la matrice : le ré-`execve` est **la** solution portable, pas un pis-aller. | **Adopter.** Précision (m-2) : l'auto-vérification de H-0 doit distinguer Landlock (ouverture → `EACCES`, à tester **avant** seccomp) de seccomp (→ `EPERM`, après), sinon un Landlock absent passe inaperçu derrière seccomp. |
| **D-12** TinyGo | **Rust et Go en 1.0, TinyGo en 1.1** ; Codex a raison que TinyGo n'est pas « gratuit » (ordonnanceur `asyncify`, `poll_oneoff`) — raison de plus pour le mesurer après. | Le vecteur Go standard suffit à prouver l'ABI ; TinyGo vaut par la **taille** des modules (≈ 100 Kio contre ≈ 2 Mio), ce qui est précisément la mesure que P-PLUG-BENCH fournit (taille, temps de compilation). | **Adopter** ; P-PLUG-BENCH consigne taille et temps de compilation du module Go pour que la promotion de TinyGo en 1.1 soit une décision mesurée. |
| **D-13** UTCP | **Retirer** (fait, spec:2724). | Aucun hôte majeur, aucun client ; le laisser serait de la sur-ingénierie documentaire. | **Clos.** |
| **D-14** preuves | **Rétrograder en expérimentation** (§0.5) : juste et honnête. Voir §3 ci-dessous pour les portes. | ADR « proposée / acceptée » (Nygard) ; ISO/IEC/IEEE 42010 : une décision est liée à ses préoccupations et à ses preuves ; `engineering-principles` §3. | **Adopter** avec les trois compléments du §3 (porte P-IDX mesurable, porte ACP = report, repli nommé par expérimentation). |
| **D-15** Mermaid | **Position révisée, plus prudente que la mienne du tour 2.** `beautiful-mermaid` existe bien (MIT, 11,3 k étoiles, « zero DOM »), mais son moteur de disposition est **ELK.js** (Java transpilé, ≈ 1–2 Mo minifié), exécuté « synchronously via FakeWorker bypass » ; sa mesure de texte n'est pas documentée (tables de métriques, à vérifier). Compilé par Javy (QuickJS **interprété**), le module pèsera plusieurs Mio et la disposition de 200 nœuds peut dépasser le p95 ≤ 300 ms du §15.14 point 6. | Codex : « aucun résultat P-FRAME ne justifie un retour » — exact, mais sans objet : cette voie n'utilise **pas** de cadre, c'est un `renderer` `wasm` ordinaire dans l'hôte enfermé, donc la même sûreté que Graphviz. | **Décision commune proposée** : §15.14 dit « **candidat** pour l'API 1.1 » (pas « voie retenue »), avec sa porte : taille du module ≤ 8 Mio, p95 ≤ 300 ms pour 200 nœuds dans l'hôte, polices mesurées cohérentes avec le PDF ; Mermaid complet reste derrière P-FRAME ; rien en 1.0. Codex peut accepter cela sans rien prêter à P-FRAME. |
| **D-16** domaines | **Avec Codex et l'auteur : préfixes externes, PD-2 retiré.** | Protocole §3.3 : message signé = `"inkwell/" ‖ domaine ‖ "/v1" ‖ 0x00 ‖ …`, domaine sans `/` ni NUL (§20.4) ; un message `"inkwell-ext:…"` diffère dès l'octet 8 : séparation de domaines effective. Réutiliser la clé d'appareil pour des signatures hors protocole est sûr **parce que** les préfixes sont disjoints (cross-protocol attacks, Kelsey–Schneier–Wagner 1997). | **Clos.** Une ligne à ajouter au §9.4 : « la clé d'appareil signe aussi des objets `inkwell-ext:` ; liste fermée : `plugin-package`, `backup-head`, `export` » (m-3), pour que la revue de sécurité ait la liste. |

Bilan : D-10 et D-15 changent ma position du tour 2 sur preuve ; D-11, D-12, D-13, D-14, D-16
convergent. **Aucun désaccord ouvert** de mon côté.

---

## 3. §0.5 « expérimentations » : sain, à trois conditions

**Jugement.** Sain, et non une esquive : (1) chaque ligne a une **option choisie** (un hôte par
greffon, Seatbelt, AppContainer, coffre par objet, index lexical, JSON Lines…) — ce qui est différé
est l'**acceptation**, pas la décision ; (2) « aucun code de production n'en dépend » est tenu par
le phasage (`coherence.md` §5 : MVP sans hôte, sans coffre, sans index ; v1 « après P-PLUG-PROTO et
P-PLUG-BENCH ») ; (3) les portes sont, pour la plupart, **numériques et exécutables** :
P-PLUG-BENCH (annexe : p50/p95/p99, 64 Kio/1/4 Mio, témoin, arm64, seuils chiffrés),
P-SANDBOX-* (enfant natif hostile, matrice de noyaux, « tout refusé »), P-VAULT (§12.5 : ≤ +10 %
écriture, ≤ +20 % ouverture ; plantage à chaque étape ; canari), P-GIT-BENCH (≤ 1 s / 1 000
fichiers, 500 versions ≤ 60 s), P-FRAME (boucle infinie tuée, canari de navigation). C'est la
pratique des ADR « proposées » (Nygard) et de l'arc42 §11 (risques tenus avec leur preuve).

**Trois défauts, tous de texte.**

- **R3-M1 — P-IDX n'est pas mesurable.** §0.5 : « bancs, rappel en français » ; §15.18 RG-2 :
  « si un banc de rappel en français le justifie » ; §12.5 donne la latence (≤ 50 ms / 100 Ko,
  1 000 fichiers ≤ 5 s, ≤ 1 cœur, ≤ 150 Mio) mais **aucun corpus ni seuil de rappel**. Une porte
  sans seuil ne peut ni passer ni échouer. Norme : `engineering-principles` §3 (« chaque principe a
  sa preuve »), règle du projet « rubrique fixe ». Correction : corpus fixé (par exemple 200
  requêtes rédigées sur le projet d'exemple et deux projets Typst publics, jugements à trois niveaux),
  métrique **rappel@10 ≥ 0,8 et MRR ≥ 0,6** pour R0 ; R1/R2 seulement si **+0,1 de rappel@10**
  mesuré. Gravité : **majeur** (c'est la seule porte du §0.5 qui ne décide rien).
- **R3-M2 — Porte ACP indéfinie.** « bac à sable prouvé sur la cible » : un agent tiers « à pleins
  droits » a besoin de fichiers et de réseau **par nature** ; aucune des garanties de §9.11 ne lui
  est applicable, et le texte le reconnaît (ACP-1 : « aucune garantie »). Ce n'est donc pas une
  expérimentation à porte, c'est un **report** : l'écrire ainsi (« après v1, décision séparée »), ce
  que `coherence.md` §5 fait déjà (« Après v1 — ACP »). Gravité : mineur (cohérence entre §0.5 et
  le phasage).
- **R3-M3 — Repli nommé à l'avance.** Seule P-PLUG-BENCH dit ce qui se passe à l'échec (« la
  décision est revue, pas les budgets »). Pour les autres, nommer le repli **avant** de mesurer,
  sinon la mesure sera relue pour sauver l'option : P-VAULT échoue → NG3 reste et G13 passe en
  v1.x ; P-SANDBOX-MAC/WIN échoue → alpha Linux (déjà écrit §9.11) ; P-IDX échoue → recherche IA
  sans index (lecture du fichier courant seulement) ; P-GIT-BENCH échoue → écrivain minimal
  (`AD-13` option 2) ; P-FRAME échoue → Mermaid absent, et dit. Gravité : majeur **de méthode**, une
  colonne de table.

Suffisance : oui, avec ces trois ajouts ; aucune expérimentation ne peut entrer en production sans
porte, et une porte floue **bloque** plutôt qu'elle ne laisse passer (échec fermé) — c'est le bon
sens de la panne.

---

## 4. `coherence.md`

**Contradictions C01–C25 : résolues**, chacune par une règle nommée que j'ai retrouvée dans le texte
(C01 → NG3/G13 ; C05 → §9.11 ; C08 → ST « non déclarée satisfaite » ; C09 → ST-8 ; C13 → ER-1 ;
C16 → IG-1/IG-4 ; C17 → §6.11 ; C20 → `AD-47` rév. 3 ; C22 → `AD-13` rév. 3 ; C23 → spec:1513 ;
C24 → §15.5). Deux restes :

- **R3-m4 — C14 cite une contradiction qui n'existe plus** : `docs/ux-principles.md:14` dit déjà
  « palette (Ctrl/Cmd+Maj+P) » ; le code (`keymap.ts:16`) met `Mod-k` sur le lien. La ligne
  « principes UX règle 3 à corriger » et le point 6 du §6 sont périmés ; retirer, ou citer la vraie
  ligne fautive si elle est ailleurs. Mineur.
- **R3-m5 — `.inkwell/git/` « [à décider, AD-13] »** (spec:778) alors que `AD-13` est tranché en
  révision 3 : marquer `[cible, AD-13]`. Mineur.

**Déferrals justifiés** : M11 (réglages entre appareils), M12 (lien web vivant), ACP, client MCP
sortant, vecteurs, plusieurs projets, Mermaid complet — chacun avec sa raison (`AD-34`, `AD-01`,
P-FRAME) et sans dépendance du MVP sur eux. Bon.

**Carte produit** : calme et écriture d'abord, oui — aucun tiroir ouvert, trois destinations, deux
onglets à gauche, quatre éléments de barre d'état, un seul compteur de problèmes ; c'est conforme
aux GNOME HIG (« calme », chrome minimal) et à `ux-principles`. Un détail (**R3-m6**) : « Rien
d'affiché pour une fonction absente ou non installée » contre un bouton « Extensions » (pièce de
puzzle) **permanent** dans l'en-tête même sans extension tierce ; le montrer seulement quand une
extension tierce ou de projet existe, le gestionnaire restant accessible par le menu. Mineur.

**Phasage** : réaliste comme **ordre** (chaque lot = un usage complet, qualité dans chaque tranche),
ambitieux comme **volume** (`PROGRESS.md` : étapes 0 et 1 faites, étape 2 en cours ; 20 cases
cochées, 25 ouvertes ; le lot « Collaborer sans perdre » porte à lui seul la migration ADR 0034 et
l'admission ADR 0031). Deux corrections :

- **R3-M4 — N38 est placé dans « Distribution », donc en dernier.** Or §12.5 dit « valeurs fixées
  par la **première mesure de référence** » et `coherence.md` §5 dit « la qualité accompagne chaque
  lot ». Le code n'a aujourd'hui **aucun** cas `PERF-*` (grep `frontend/e2e` : rien ; `INKWELL_PERF`
  seulement dans un worktree) ; les bancs Go existent pour `HOT_PATHS` (blob, frame, mux, app,
  trust). La mesure de référence (`PERF-TYPE`, `PERF-SYNC-*`, `PERF-START`, `PERF-MEM`) doit entrer
  dans le **premier lot du MVP**, sinon tous les budgets « proposés » restent des souhaits jusqu'à
  la fin. Norme : `CLAUDE.md` (« prove perf changes »), N38. Gravité : **majeur** (phasage ; une
  ligne à déplacer).
- **R3-m7 — N34 absent du phasage** : la couche personnelle (§6.9 IG-2) n'apparaît ni au MVP ni en
  v1 ; la mettre en v1 « Modèles et écriture » ou « Distribution ». Mineur.

---

## 5. Sections nouvelles

### 5.1 §6.9 Fichiers non synchronisés — **bon**, deux précisions

Juste sur le fond : politique d'import/réception, pas de contrôle d'accès (IG-1), couche de projet
= décision signée via PD-1 (IG-2, exactement ce que Codex R2-11 demandait), jamais `.typstdeps/` ni
ressources référencées (IG-4, contre l'étude, conforme à N5/ADR 0024), parseur de go-git déjà lié +
conformité `git check-ignore` **à la construction** (compatible INV-12) + fuzz (IG-3).
- **R3-m8** — Un blob que cet appareil a choisi de **ne pas télécharger** ne doit jamais être
  annoncé comme disponible aux pairs (sinon un `fetch` échoue sans raison visible) ; une phrase dans
  IG-1 et un vecteur. Mineur.
- Sur-ingénierie : aucune ; six règles, un format existant.

### 5.2 §6.10 Projet chiffré — **bon**, un budget manquant

Juste : chiffrement par objet avec AAD (projet, type, nom, époque) et noms opaques (EN-1), identifiants
du protocole **dans** le chiffré (le protocole ne change pas — c'est la bonne frontière), enveloppe
au format d'en-tête **age** (C2SP, réutilisé plutôt que réinventé), Argon2id (RFC 9106), destinataire
de secours dérivé de ST-3, honnêteté EN-7, export lisible par `age -d`.
- **R3-M5 — EN-4 sans budget.** Le dossier de travail privé est un **tmpfs** sous Linux (RAM), et §2
  admet des projets de 2 Go (100 Mo par fichier binaire). Matérialiser « à la demande » un projet
  d'images pour une compilation peut dépasser à lui seul le budget global de 2,5 Gio ; sous macOS et
  Windows le dossier est sur disque (clair pendant la session, balayé au démarrage après plantage —
  EN-7 doit le dire). Le code a déjà le motif : `mount_linux.go:31,94` monte un tmpfs avec `size=`
  arrondi vers le bas. Correction : « tmpfs borné (`size=`, part du budget §2) ; matérialisation
  **par dépendances réelles** (`typst query`/`--deps` du principal) ; au-delà de la borne : aperçu
  refusé avec message, jamais de débordement silencieux sur disque » ; et la ligne EN-7 sur la
  rémanence après plantage. Norme : `engineering-principles` §3 (budgets), ADR 0033. Gravité :
  **majeur** (texte).
- **R3-m9** — EN-2 dit « DEK **par appareil** » puis l'enveloppe « pour les autres appareils de la
  même personne » : c'est donc une DEK **par personne et par projet**, par appareil seulement au sens
  où chaque appareil la déverrouille avec sa propre clé X25519. Dire l'un ou l'autre. Mineur.
- **Amélioration (hors constat)** : go-git lit et écrit à travers `billy.Filesystem` ; la projection
  Git (EN-5) peut vivre **dans** le coffre (billy sur le magasin chiffré ou en mémoire, puis objets
  chiffrés) sans matérialisation, ce qui retire une copie sensible de la liste.
- P-VAULT : numérique via §12.5 sauf « rechiffrement complet » (pas de budget : en donner un, par
  exemple ≥ 50 Mo/s par cœur). Mineur, inclus dans R3-M5.

### 5.3 §6.11 Historique et variantes — **bon sur l'intention**, une sémantique à fixer

Juste : vérité Automerge, Git = projection reconstructible par go-git (≥ 5.19 : CVE-2026-45022
corrigée ; de plus inkwell n'**analyse** jamais d'objets Git étrangers, il n'écrit que les siens),
jamais de binaire `git` (INV-12, AC-18 testé « sans `git` dans `PATH` »), restaurer = **nouvelle
modification** (HI-2, exactement la demande de Codex), versions = manifestes de têtes (HI-3, sur
`meta.versions` existant, `versions.ts:59`), vocabulaire sans Git, variantes sur l'existant
(`graph.ts` : voies, `forkFrom`, « merge » ; ADR 0031 : `DocId` dérivé avec `Slot`, variantes
bornées à `32 + 32 × grants`).
- **R3-M6 — HI-6 « Intégrer » mélange deux sémantiques.** « Fusion Automerge des modifications de la
  variante » (= `A.merge`, ce que fait `testkit.ts:44` et `versions.test.ts`) conserve les
  changements **avec leurs auteurs et signatures d'origine** ; « une modification, annulable,
  attribuée » décrit un **diff appliqué** par la personne qui intègre (une seule modification à son
  nom). Sous l'ADR 0031, les changements d'une variante ont été admis pour le `DocId` de la variante
  (dérivé de `Slot` et de l'ancre) : les faire entrer tels quels dans l'original exige une règle
  d'admission (« un changement admis dans une variante annoncée de D est admissible dans D ») que
  la revue byzantine doit voir — ou bien on intègre par diff, et l'attribution fine se perd (visible
  seulement dans l'histoire de la variante, conservée). Les deux sont défendables (Patchwork fait la
  première) ; le texte doit **choisir** et dire ce que « Annuler » annule. Gravité : **majeur**
  (sémantique d'une fonction du MVP).
- **R3-m10** — HI-1 « à n'importe quel moment » à travers **plusieurs** documents : il faut définir
  l'horloge du curseur temporel ; l'heure des changements Automerge est déclarée par l'auteur (pair
  byzantin) ; l'ordre de **réception locale** est honnête et déterministe par appareil ; dire « ordre
  de réception sur cet appareil, heure déclarée affichée comme telle ». Mineur.
- Budgets manquants pour HI-1 (`view` aux têtes d'un document de 50 000 opérations) et HI-4 (diff par
  mots sur 2 Mi caractères) dans §12.5 : voir R3-m11.

### 5.4 §9.11 Six plates-formes — **sain**, mécanismes à prouver comme annoncé

Table complète et honnête (« leur nom ne vaut pas preuve »). Vérifications : Seatbelt
(`sandbox_init`) déprécié depuis 10.8 mais fonctionnel et **hérité** par les enfants — c'est ce que
Chromium fait (`sandbox/mac/seatbelt.h`) ; `PROC_THREAD_ATTRIBUTE_CHILD_PROCESS_POLICY`,
`JOB_OBJECT_LIMIT_PROCESS_MEMORY`, `CPU_RATE_CONTROL` existent (Chromium `sandbox/win`) ; macOS
ignore largement `RLIMIT_AS`, et le texte s'en remet correctement au **parent** ; INV-13 (capacité
refusée) est la bonne règle de repli ; les sidecars passent par le même port (ADR 0033).
- Risque à écrire (mineur, **R3-m12**) : SBPL est une API privée d'Apple ; la porte P-SANDBOX-MAC
  doit se **rejouer à chaque majeure de macOS** (et AppContainer à chaque majeure de Windows), sinon
  une garantie prouvée une fois cesse de l'être en silence.
- Sur-ingénierie : non ; trois dorsaux derrière un port est le minimum pour N35.

### 5.5 §11.1 Ressources — **bon**

Conforme au modèle (`about:processes`) et aux crochets du tour 2 : table unique du cœur, mesures par
le **parent** (ce que l'hôte dit est indicatif), quatre verbes, limites réglables à la baisse,
aucune donnée de document (canari O-7), budget propre (2 Hz, ≤ 1 % d'un cœur, 0 fermée).
- **R3-m13** — Verbes **par classe** : « mettre en pause » ou « arrêter » le Worker Automerge ou la
  projection figerait l'édition ou la durabilité ; dire quelles lignes n'offrent qu'« annuler » (le
  cœur) et lesquelles offrent les quatre (greffons, IA, index, compilation). Mineur.

### 5.6 §12.5 Performance mesurée — **budgets numériques, bancs nommés, mesure encore à faire**

Oui, les budgets sont **chiffrés** (treize lignes, p95, mémoire, concurrence par ligne) et chaque
ligne nomme son banc. Sont **réellement benchmarkés aujourd'hui** : `BenchmarkRelay`
(`internal/app/wiring_bench_test.go:28`), blob (`internal/blob/bench_test.go`), frame, mux, trust ;
`mise run bench` existe. Ne le sont **pas** : tous les `PERF-*` d'interface et de réseau, l'index, le
coffre, Git, les greffons (expérimentations). Le texte le dit (« proposées, à fixer par la première
mesure ») ; le défaut est le **moment** (R3-M4). Sur « performance extrême par la concurrence Go » :
le texte est honnête — le chemin de la frappe est dans la webview (Automerge dans le Worker), la
concurrence Go sert la synchronisation, le hachage, l'écriture, l'index ; PF-5 à PF-10 (parallélisme
borné `errgroup.SetLimit`, `goleak`, `synctest`, PGO gardé seulement s'il gagne, `bench:compare` en
CI) sont les bonnes règles et correspondent à la consigne du dépôt.
- **R3-m11** — Lignes manquantes : HI-1 (`view` aux têtes, 50 000 opérations ≤ 200 ms), HI-4 (diff
  par mots, 2 Mi caractères ≤ 1 s hors fil d'affichage), §6.9 (motifs gitignore sur 10 000
  chemins ≤ 20 ms), déverrouillage du coffre (Argon2id ≤ 500 ms, paramètres fixés). Mineur.
- **R3-m14** — « BLAKE3 ≥ 500 Mo/s par cœur » : vrai sur amd64 avec AVX2 (lukechampine v1.4.1) ;
  sur arm64 en Go pur, ≈ 300–500 Mo/s : écrire le budget **par architecture** ou le mesurer avant de
  le fixer (ce que le texte prévoit). Mineur.
- Sur-ingénierie : PF-10 « go-git contre écrivain minimal » impose d'écrire un témoin de ≈ 200 lignes
  pour un banc ; acceptable (déjà dans `AD-13`), mais le témoin peut être **jetable** (dans
  `docs/plans/`, pas dans `internal/`).

### 5.7 §15.18 Index de recherche (RG-1…RG-5) — **bon**, porte à définir

Conforme aux crochets : instantané depuis la webview (jamais Automerge en Go, INV-1), lexical d'abord,
moteur choisi par la mesure sur six cibles, **provenance par extrait** (sans elle IA-2 serait
contournée), citations par nœud et révision (survivent aux renommages), `ai: never` et non
synchronisés exclus à l'indexation **et** à la requête, quota 200 Mio, index jetable. Constat
principal : R3-M1 (porte sans seuil). Deux remarques :
- **R3-m15** — « chiffré comme le projet » pour un index SQLite ou bleve : aucune de ces
  bibliothèques en Go pur ne chiffre ses fichiers ; le plus simple et cohérent avec EN-4 est de
  **loger l'index dans le dossier de travail privé** (tmpfs sous Linux), détruit à la fermeture — ce
  que RG-3 dit déjà (« effacé à la fermeture ») : alors « chiffré » devient « jamais sur disque »,
  plus fort et sans code. Mineur.
- FTS5 n'a pas de racinisation française native (tokenizer `unicode61`) ; bleve a Snowball ; c'est un
  critère du banc P-IDX à écrire explicitement (rappel en français).

---

## 6. Sur-ingénierie (à alléger, non bloquant)

- **Kit de secours régénéré à chaque sauvegarde** (ST-8) : si la personne doit le ré-enregistrer à
  chaque fois, elle ne le fera pas ; dire « proposé à la régénération tous les N jours ou sur geste »,
  l'ancre de fraîcheur restant celle du dernier kit gardé (ST-7 le dit honnêtement).
- **IPC-2 huit types de trames** dont `log` : un `host_call("log")` suffirait ; garder si le décodeur
  reste un seul `switch` fuzzé.
- **§11.1 « entrées et sorties »** par ligne : utile pour les sidecars, bruit pour le reste ; colonne
  optionnelle.
- Ce qui n'est **pas** de la sur-ingénierie et qu'il faut garder : un hôte par greffon (D-10), le
  démarrage en deux temps, le port `internal/sandbox` à trois dorsaux, le coffre par objet au format
  age, le kit de secours lui-même, JSON Lines, la table unique des ressources, la règle « restaurer
  = nouvelle modification ».

---

## 7. Ce qui est bien et qu'il ne faut pas toucher

§9.10 H-0/H-5 (enfin exécutable), §7.3 (codec complet, autorité par connexion), §15.20 ABI-4
dérivé et neutralisé, §6.7 (secret seul, kit, aucun ménage distant), PD-1 avec sa règle de pli et
sa transition honnête, PD-2 retiré, §0.5 et `AD-95`, §9.11 avec INV-13, §6.9, §6.10 EN-1/EN-2/EN-7,
§6.11 HI-2/HI-3/HI-5, §11.1, §12.5 PF-5…PF-10, la carte produit, le phasage par usages complets.

---

## Verdict : **ACCEPT**

Les onze bloquants et les huit majeurs du tour 2 sont fermés dans le texte, vérifiés ligne à ligne ;
les sept désaccords D-10…D-16 convergent (je rejoins Codex sur D-10, je révise ma position sur
D-15 vers « candidat 1.1 avec porte », aucun désaccord ouvert) ; le §0.5 est sain. Les majeurs
ci-dessous sont des **précisions de texte** à faire en révision 4 au fil de l'eau, sans nouveau
tour bloquant :

- **R3-M1** porte P-IDX mesurable (corpus, rappel@10, MRR) ;
- **R3-M3** repli nommé à l'avance pour chaque expérimentation (colonne du §0.5) ;
- **R3-M4** mesure de référence `PERF-*` dans le premier lot du MVP, pas en « Distribution » ;
- **R3-M5** budget du dossier de travail privé (tmpfs `size=`, matérialisation par dépendances,
  refus au-delà ; rémanence après plantage dans EN-7) ;
- **R3-M6** sémantique d'« Intégrer » (fusion d'opérations avec règle d'admission, ou diff appliqué)
  et de son « Annuler ».

Mineurs R3-m1…m15 au fil de l'eau. Acceptation conditionnée à rien d'autre.
