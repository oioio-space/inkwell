# Revue Fable n°1 de l'architecture (spec 1.0.0-brouillon.2, 2026-11-01) — verdict : **NOT ACCEPTED**

Revue indépendante de `docs/architecture/{spec.md, decisions.md, traceability.md, README.md,
research/plugins.md}` contre `docs/conception.pdf`, `CLAUDE.md`, `docs/engineering-principles.md`,
les ADR (0017, 0020, 0023, 0024, 0028–0034), `docs/protocol/spec.md` (2.0.0-brouillon.10,
acceptée) et `docs/exigences.md` (N18–N26). État du code vérifié sur `main` (`d0e4d59`).

**Méthode.**
- Chaque règle du §15 et du §6.6–6.7 confrontée au protocole accepté (registres §20, HELLO §5.3,
  `policy_op_v1` §4.4, modèle de menace §17) et au code (`frontend/src`, `internal/sidecar`,
  `internal/typst`, `internal/blob`).
- Sources externes revérifiées le 2026-11-01 : wazero v1.12.0 (`WithCloseOnContextDone`,
  `WithMemoryLimitPages`, `WithDebugInfoEnabled`, `NewCompilationCacheWithDir` : présents,
  pkg.go.dev) ; mermaid #3886 (fermé « not planned », mise en page par le navigateur, confirmé) ;
  CVE-2026-41148 / 41149 (injections CSS et HTML par `classDef`, corrigées en 11.15.0 ; NVD,
  INCIBE) ; restriction AppArmor des espaces de noms utilisateur sans privilège sur Ubuntu 24.04
  (`kernel.apparmor_restrict_unprivileged_userns=1` ; note de l'équipe noyau Ubuntu sur oss-security,
  2024-04-19 ; discourse.ubuntu.com « Understanding AppArmor User Namespace Restriction »).
- Grille : chaque constat cite une norme ou une règle du projet, une preuve (fichier:ligne ou
  source) et une gravité (bloquant / majeur / mineur). Aucun constat « au ressenti ».

**Bilan.** 4 bloquants, 8 majeurs, 9 mineurs. Le document est de très bonne tenue : le modèle
multi-fichiers (§6.6), le noyau fermé (§15.3), la grille « ne pas dénaturer » (§15.11), le dorsal
byzantin (§6.7) et la franchise sur le mode débogage (§15.15) sont au niveau attendu. Les
bloquants portent sur quatre endroits où la spécification contredit son propre modèle de menace
ou le protocole accepté, précisément sur le chemin qui s'exécute **sans geste** (greffons de
projet, N23).

---

## 1. Le choix clé : processus enfant (spec, option 4) contre dans le processus (étude, option 3)

**Qui a raison.** La spécification, sous conditions. L'étude (research §5.1, raison 2) écrit que
les capacités « vivent là où sont les secrets et `os.Root` » : c'est un argument **contre**
l'option 3, pas pour. Dès que N23 fait charger sans geste du code reçu d'un pair, la surface
d'attaque n'est plus « le moteur wazero est en Go sûr » mais « le code natif que son compilateur
génère » (bogues d'élision de bornes, cf. CVE-2023-26489 et CVE-2022-39393 chez Wasmtime, même
classe de défaut chez tout compilateur AOT). Une évasion dans le processus principal donne
`device.key` (K-1) : usurpation de toutes les modifications de la personne, non détectable par
le protocole (§17 : « chaque modification signée par l'appareil »). L'enfant sans fichier, sans
réseau, sans secret ramène l'évasion à un déni de service local. Norme : défense en profondeur,
NIST SP 800-53 SC-39 (isolation de processus) ; OWASP ASVS 5.0 V1 (architecture) ; `CLAUDE.md`
« tout octet venu d'un pair est hostile ».

**À quel coût.** Un aller-retour de plus par appel. Sur Linux, un ping-pong par tube ou socket
Unix coûte de l'ordre de 10 à 30 µs, la copie de 64 Kio quelques µs : contre ≈ 1,6 µs d'appel
wazero mesuré par son journal CI (research §3.3). Soit un facteur 10 à 20 **sur l'appel**, mais
≤ 50 µs en absolu, négligeable devant les budgets du §15.4 (2 ms par appel, 50 ms coloration,
300 ms rendu) et devant le trajet webview → Go déjà présent (Stream `ext`). Le coût réel est
ailleurs : (a) un processus Go de plus par projet ouvert (RSS de base ≈ 10–30 Mo + code wazero
mappé), absent des budgets du §2 (« processus Go ≤ 300 Mo ») ; (b) un troisième saut pour tout
ce qui est interactif (vue déclarative : clic → Go → enfant → vue → Go → webview), non
budgété ; (c) la surveillance (recul, `Pdeathsig`) déjà payée par les sidecars.

**Ce qui manque pour que la décision soit « mesurée, pas ressentie »** (engineering-principles §3,
ADR 0028) : P-PLUG-BENCH (annexe, *Confirmation*) mesure l'« aller-retour entre processus »
mais **sans témoin en processus** (option 3). Sans ce témoin, le banc ne prouve pas le surcoût de
l'option 4, il le constate. → **M-1**.

Verdict sur ce point : option 4 **retenue**, à condition de fermer B-3 (l'enfermement de l'enfant
est aujourd'hui une affirmation, pas une règle vérifiable) et M-1.

---

## 2. Bloquants

### B-1 — Le cadre de rendu `frame` (Mermaid) fait tourner du JavaScript tiers dans le processus web, sans capacité, et se charge sans geste au niveau « projet »

- **Norme.** Modèle de menace §9.2 et K-8 (« un défaut de la webview ne doit donner aucun pouvoir
  sur Go », B-2) ; AD-57, *Pourquoi pas l'option 2* : « le code tiers tournerait dans l'origine
  qui tient le pont (Stream `net`, kind `Sign`) » ; CWE-829 (inclusion de fonctionnalité d'une
  sphère non maîtrisée) ; ASVS 5.0 V14.
- **Preuve.** spec §15.14 point 1 : hôte `frame` = `iframe sandbox="allow-scripts"` dans la
  webview, « seul cas où du JavaScript d'extension tourne » ; §15.4 ligne `renderers` :
  capacités « **aucune** (contrat pur) » ; §15.10 niveau « projet » (chargé sans question) :
  « types de fichiers et leurs vues, coloration et décorations… » et rien n'exclut un
  `renderer` `frame`. Donc : un admin ajoute un greffon Mermaid, tous les membres exécutent du
  JavaScript tiers dans **le processus web qui tient le pont et le CRDT**, sans accord. L'`iframe`
  à origine opaque protège du DOM parent, pas d'un défaut du moteur JavaScript ou du moteur de
  rendu WebKit (même processus web : WebKitGTK n'isole pas les cadres d'origine opaque dans un
  processus à part sans l'isolation de site, non activée ici). C'est exactement le risque que
  l'AD-57 refuse pour l'option 2, réintroduit par la porte des diagrammes. La mitigation citée
  (`securityLevel: 'strict'`, ≥ 11.15) protège contre un **diagramme** hostile, pas contre un
  **greffon** hostile.
- **Correction demandée.** (1) `frame` exige une capacité nommée (`ui.canvas` ou une capacité
  propre `render.frame`) hors du niveau « projet » : accord local explicite, phrase « exécuter du
  code d'affichage de X dans une zone isolée ». (2) Isolation de processus : rendre dans une
  fenêtre Wails cachée dédiée (son propre processus web WebKitGTK, aucun binding, aucun Stream,
  CSP du cadre), ou documenter pourquoi l'isolation de site de WebKitGTK suffit (à prouver, pas à
  affirmer). (3) Le cas de référence livré reste Graphviz en `wasm` ; Mermaid en `frame` est un
  exemple **installé par geste**, jamais un greffon de projet au niveau « projet ». (4) Ajouter
  SC-11 à la liste SC-* (le texte y renvoie déjà, « SC-11 », mais la table s'arrête à SC-10).

### B-2 — L'espace de décisions `plugin/<id>` n'est pas une « extension compatible » du protocole accepté

- **Norme.** `docs/protocol/spec.md` §0.3 de la spec (« la spécification du protocole est
  normative pour tout octet échangé ») ; protocole §4.4 : `policy_op_v1.entries` « clés
  strictement croissantes, **espaces de noms du registre §20.5** » ; §20.5 = liste fermée
  (`mode`, `main`, `doc/`, `node/`, `quota/`, `legacy/`, `equiv/`, `format`) ; §1.4 et ligne 104 :
  une mineure n'est émise qu'après la décision `format = <mineure>` ; INV-4 (décodage strict).
- **Preuve.** spec §15.10 « Vérité » : « nouvel espace de noms de décisions, extension compatible
  du protocole (§1.4), à inscrire au registre §20.5 ». Le protocole ne définit **aucune** règle de
  tolérance pour un espace de noms inconnu dans `policy_op_v1` (seule la règle `!` des maps CBOR
  §9.2 existe, elle ne s'applique pas à ce codec binaire). Un appareil 2.0 qui décode strictement
  **refuse** l'op entière (et sanctionne ?), un appareil 2.1 l'applique : le pli des décisions
  diverge entre membres, ce que l'ADR 0030 interdit (« le pli doit être identique partout »,
  K-2). Idem pour les décisions qui portent le drapeau « principal » si elles cohabitent avec un
  `plugin/` dans la même op.
- **Correction demandée.** Une **mineure du protocole** (2.1) avec : règle de tolérance explicite
  pour `policy_op_v1` (espace inconnu → entrée ignorée, op acceptée, ou op refusée **sans**
  sanction — à trancher et à vectoriser `policy.json#unknown-namespace`), inscription de
  `plugin/<id>` au §20.5 avec la borne de `id` (clé ≤ 160 o, §4.4), le schéma de la valeur
  (≤ 4096 o) et la capacité `plugins/1` dans CAPS (§20.2) ; émission gardée par `format ≥ 1` ;
  ligne dans la table §16 du protocole ; même traitement pour la carte `renders` (§15.14 point 3 :
  « règle de garde à ajouter ») et pour les domaines de signature `inkwell/plugin/v1` et
  `inkwell/backup-head/v1`, absents du §20.4. La spec d'architecture doit **lister** ces deltas
  du protocole dans une table numérotée (PD-1…n) avec leur vecteur, au lieu de les disséminer.

### B-3 — L'enfermement de l'hôte des greffons, seule justification de l'option 4, n'est pas spécifié de façon exécutable

- **Norme.** engineering-principles §4 (« échec fermé si l'enfermement manque ») ; ADR 0033 ;
  annexe SC-1 ; AD-18 (Linux v1, parité d'enfermement avant d'autres OS) ; ISO/IEC 25010:2023
  sûreté / « sécurité intégrée ».
- **Preuves.**
  1. Annexe *Décision* point 1 : « Landlock **sans aucun accès** au système de fichiers » et
     §3.2 : « aucun fichier ». Mais le même point exige « wazero (compilateur, **cache de
     compilation sur disque**) » et §4.2 `internal/plugin` : « cache de compilation wazero
     (`<UserCacheDir>`) ». Un enfant sans aucun accès ne peut ni lire les modules
     (`<UserDataDir>/inkwell/plugins/<id>/<v>/`) ni lire ou écrire ce cache. Rien ne dit que les
     modules et le code compilé passent par des descripteurs hérités (`memfd`, tube) ni qui
     écrit le cache. Contradiction interne.
  2. `internal/sidecar/netns_linux.go:22–34` : l'isolation réseau repose sur
     `CLONE_NEWUSER | CLONE_NEWNET` et `probeIsolation` **se replie** quand le noyau ou AppArmor
     la refuse (commentaire : « AppArmor on Ubuntu 24.04 »). Ubuntu 24.04 est une cible de
     `pkg:test` (§2, PKG-01…12). Avec `apparmor_restrict_unprivileged_userns=1`, un exécutable
     sans profil AppArmor ne crée pas d'espace de noms utilisateur. Pour `typst`, le repli
     journalisé est accepté (§9.5, AD-28) ; pour l'hôte des greffons, le texte dit « échec
     fermé » mais ne dit ni ce que voit la personne (aucun greffon, pourquoi, quoi faire), ni
     que le `.deb` doit livrer un profil AppArmor avec `userns`, ni ce qui se passe hors Linux
     pour les **extensions intégrées en WebAssembly** (Tâches, §15.4 : livrée en greffon
     WebAssembly) — « greffons tiers refusés » ne couvre pas une fonction du produit.
  3. `RLIMIT_AS` sur un processus qui héberge un compilateur AOT : wazero mappe des régions de
     code par module et `WithMemoryLimitPages` borne la mémoire linéaire, pas le tas Go de l'hôte
     ni le code compilé ; la valeur de `RLIMIT_AS`, le plafond mémoire **par hôte** (16 greffons ×
     256 Mio = 4 Gio possibles, PP-7 ne borne que la taille des paquets) et le comportement au
     dépassement (qui meurt : un greffon ou l'hôte ?) ne sont pas fixés.
- **Correction demandée.** Un §9.5 bis « enfermement de l'hôte des greffons » : table comme §9.5
  (mesure, Linux, autres OS), transfert des modules et du cache par descripteurs hérités (le
  parent lit, vérifie le hachage, passe un `memfd` scellé ; le cache est écrit par le parent à
  partir d'un `memfd` rendu par l'enfant, ou abandonné en v1 avec le coût mesuré), budget
  mémoire par hôte et règle d'éviction, profil AppArmor livré dans les paquets (ou constat
  mesuré que le repli `unshare`/`setpriv` de `mount_linux.go` suffit), message et état montrés
  quand l'enfermement manque, et sort des extensions intégrées WebAssembly hors Linux.

### B-4 — Le niveau « projet » (chargé sans geste) permet de modifier ce que les membres voient et collent

- **Norme.** ISO/IEC 25010:2023 sécurité / intégrité et non-répudiation ; CWE-451 (représentation
  trompeuse de l'interface) ; ND-2, PP-2, PP-5 ; protocole §17 : les admins en place sont de
  confiance, **mais l'auteur du paquet qu'un admin ajoute ne l'est pas** (confiance au premier
  usage, annexe point 6) ; G6 (« une personne malveillante ne gêne jamais les autres »).
- **Preuves.** §15.10 niveau « projet » inclut « types de fichiers et leurs vues » alors que
  `viewers` est un point à verbe **remplacer** (§15.4) : un greffon de projet remplace l'éditeur
  de `.typ` chez tout le monde, sans geste, et peut montrer un texte qui n'est pas le document
  (PP-5 encadre la vue d'un badge ; il n'empêche pas un contenu mensonger). « Coloration et
  décorations » : une décoration CodeMirror peut **remplacer** une plage par un widget, donc
  cacher du texte à la lecture ; un écrivain byzantin qui glisse une clause et un greffon qui la
  masque chez les relecteurs n'est pas couvert par ND-1 (zones protégées) ni ND-7. `transforms`
  au verbe **envelopper (collage)** modifie ce que la personne colle, sur un geste (coller) qui
  n'est pas une commande du greffon : PP-2 ne s'applique pas, « via X » non plus (réservé aux
  propositions).
- **Correction demandée.** Niveau « projet » = **contribuer seulement** : jamais « remplacer »
  ni « envelopper » sans accord local ; `decorations` limitées aux marques qui n'enlèvent ni ne
  recouvrent du texte (classes de style, gouttière, soulignement ; aucun `replace`, aucun widget
  bloc qui masque) et cette borne vérifiée par l'hôte (propriété fast-check `EXT-ND2` étendue au
  contenu) ; `transforms` au niveau « projet » uniquement par commande invoquée ; une vue de type
  **contribuée** par un greffon de projet ne devient la vue par défaut que par choix de la
  personne (repli : éditeur du cœur). À inscrire dans PP-1…PP-7 et dans la liste des « décisions
  demandées à l'utilisatrice » (point 3), car N23 change de sens.

---

## 3. Majeurs

### M-1 — P-PLUG-BENCH sans témoin : la décision option 4 n'est pas mesurée contre l'option 3
- engineering-principles §3 ; ADR 0028 ; `CLAUDE.md` « prove perf changes ». Annexe *Confirmation* :
  le banc mesure « aller-retour entre processus » mais pas le même appel en processus. Ajouter le
  témoin (option 3) et l'appel complet webview → Go → enfant → retour, p50/p95/p99, `-count ≥ 10`,
  plus la mémoire résidente **par hôte** et le coût d'un hôte par projet. Ajouter un budget
  « interaction de vue déclarative ≤ 100 ms » (clic → mise à jour), absent du §15.4.

### M-2 — Le protocole hôte ↔ enfant n'existe pas dans le document
- §7.1 décrit le Stream `ext` (webview ↔ Go) ; rien ne décrit le canal Go ↔ `inkwell
  --plugin-host` : transport (tube, socket Unix), cadrage, bornes, crédits, multiplexage de
  16 greffons, annulation d'un appel en cours, **modèle de concurrence** (wazero ne garantit pas
  l'appel concurrent d'une même instance ; un greffon sert-il un appel à la fois ?). B-1 de la
  spec exige pour tout canal « codec borné, vecteurs dorés, `Fuzz*` » : à appliquer, avec une
  ligne au §7.1 et une entrée au plan de `docs/plugin-api/` (ce canal est interne, mais ses
  bornes conditionnent ND-5).

### M-3 — DX-5 (≤ 60 fonctions, ≤ 40 types) est incompatible avec le catalogue, et le catalogue avec une v1
- ISO/IEC 25010:2023 maintenabilité ; règle du projet « créé à l'étape qui en a besoin » (P-1,
  INV-9). §15.4 + §15.12 + §15.13 énumèrent **29** interfaces WIT (`commands` … `language-server`,
  `windows`, `gutters`, `inline-widgets`, `welcome`, `mirrors`…). 60 fonctions pour 29
  interfaces, 40 types pour l'arbre de vues, les propositions, les erreurs, les événements et les
  menus : la porte `api:surface` échouera dès la première implémentation, ou l'API sera
  tronquée. S'y ajoutent, en 1.0 : quatre kits typés, chaque exemple « dans les quatre
  langages », kit de conformité, matrice de compatibilité, GREASE dans les messages hôte (un seul
  hôte, pas d'écosystème d'implémentations : RFC 8701 vise des implémentations indépendantes),
  rejeu sous Wasmtime. Demander une **API 1.0 étagée** : `commands`, `menus`, `panels`
  (`ui.view`), `file-types`, `viewers` (`ui.view`), `tokens`, `decorations` (bornées, B-4),
  `transforms`, `renderers` (`wasm`), `events`, `themes`, `settings`, `storage` ; 1.1 :
  `exporters`, `importers`, `windows`, `gutters`, `inline-widgets`, `hover`, `completion` ; 1.2 :
  `storage-backend`, `mirrors`, `language-server`, points déclarés par un greffon. Deux kits en
  1.0 (Rust, Go), GREASE réservé à ce qui est négocié. Recalculer DX-5 depuis ce découpage.

### M-4 — ST-7 surestime la détection du retour arrière ; ST-3 contredit la clé par appareil
- Norme : restic (dépôt non fiable), TUF (fraîcheur : un client **neuf** ne peut détecter un
  retour arrière sans ancre de fraîcheur), ADR 0029. §6.7 ST-6 refuse une tête « plus ancienne que
  la dernière vue » : sur un appareil qui **restaure** (cas d'usage v1 !), il n'y a pas de
  « dernière vue » : un dorsal hostile sert une tête ancienne, valide, signée, et l'appareil
  restauré repart d'un état périmé sans le savoir. ST-7 (« ni rejouer sans être vu ») est donc
  faux pour la restauration. Corriger : résiduel explicite, et/ou ancre de fraîcheur (compteur de
  tête porté par la présence ou une attestation vers les autres membres ; ou, en v1, la phrase de
  récupération transporte aussi le dernier compteur vu). ST-3 : « clé de stockage par projet et
  **par appareil** » rend la déduplication « dans le projet » fausse entre deux appareils de la
  même personne (deux clés, deux noms) ; la restauration suppose de toute façon le transfert de la
  clé (§6.7 *Restauration*) : trancher « par projet et par personne, transportée par la phrase ».
  Ajouter le résiduel « tailles d'objets visibles » (pas de rembourrage).

### M-5 — PV-6 suppose un champ du HELLO qui n'existe pas
- Protocole §5.3 : TLV `VERSIONS` = **majeures du protocole**, pas la version d'inkwell ni
  l'intervalle d'API des greffons. §15.16 PV-6 : « d'après la version annoncée dans leur HELLO ».
  Il faut un TLV **pair** (ignorable) `PLUGIN_API` (intervalles parlés) inscrit au §20 du protocole
  et vectorisé, ou retirer la promesse. Cohérence avec B-2 (même mineure 2.1).

### M-6 — Racines du ménage des blobs et cycle de vie des paquets et rendus
- ADR 0023 ; `internal/blob/gc.go:43` : `GC(referenced func(Hash) bool, …)` ne garde que ce que
  l'index référence. Un paquet de greffon de projet est « un blob du projet » référencé par une
  **décision** (§15.10), un rendu par la carte `renders` d'un nœud texte (§15.14) : la spec ne
  dit pas que ces deux sources alimentent `referenced` ; sinon le paquet est effacé par le ménage
  et le greffon disparaît chez les nouveaux membres (N5 « rejoindre = tout recevoir »). Idem
  pour la purge d'un nœud texte : ses rendus partent avec lui ? Et le retrait d'un greffon global :
  son magasin privé (`storage`, 10 Mio) et son cache de compilation sont-ils effacés ?

### M-7 — G-0 : le correctif n'a ni étape ni chemin minimal
- N19 ; §6.6 et §16 constatent le bogue (`App.svelte:197` ; confirmé :
  `onOpen={() => session.focus()}` jette `path`, `Sidebar.svelte:49` l'émet, `workspace.open()`
  prend une `AutomergeUrl`, `workspace.svelte.ts:247`) mais le renvoient à `AD-46`–`AD-56` sans
  étape de `PROGRESS.md`, sans ordre vis-à-vis de la migration Worker (ADR 0028 D2) ni de
  l'ADR 0034, et sans test e2e nommé. Exigé : une ligne au §16 « étape X, avant le Worker, e2e
  `files.spec.ts` FM-xx (clic → document ouvert, sous-titre, présence) », et la liste minimale
  MF-1, MF-2, MF-5, MF-12 pour fermer G-0 sans attendre MF-6/MF-7.

### M-8 — Budgets et surveillance de la coloration par greffon (`tokens`) sur le chemin de frappe
- ADR 0028 (fil d'affichage ≤ 16 ms), §15.4 `tokens` ≤ 50 ms « plages de la zone visible ». Le
  trajet est frappe → webview → Go → enfant → retour, à chaque modification de la zone visible :
  la spec doit fixer l'anti-rebond (≥ 150 ms ?), l'annulation du calcul périmé, la stratégie
  pendant l'attente (dernières plages conservées, jamais de clignotement) et le dégradé au-delà du
  palier « Grand » (coloration coupée à 2 Mi, AD-53) ; sinon ND-5 n'est pas prouvable.

---

## 4. Mineurs

- **m-1** Conception p. 28 : les diagrammes de la finalité sont **fletcher/CeTZ** (Typst natif,
  déterministes, dans X-8) et l'import SVG (draw.io, Excalidraw). §15.14 ne les cite pas : dire
  que les `renderers` complètent ce chemin (fidélité à la conception, §0.3).
- **m-2** G5 et texte SVG : `engine.go:463` passe `--ignore-system-fonts --font-path` (bon :
  rendu déterministe). Mais Mermaid mesure le texte avec les polices du cadre ; si elles ne sont
  pas les mêmes que celles du `--font-path`, les étiquettes se chevauchent dans le PDF. Exiger du
  contrat `renderers` que les polices déclarées soient celles fournies par le projet ou embarquées,
  et que le cadre les charge depuis `/ext/<id>/`.
- **m-3** CSP du cadre : « scripts du paquet autorisés par empreinte SHA-256 » — l'empreinte
  d'un script **externe** (CSP3) n'est pas universellement honorée par WebKit ; préciser
  `script-src` par chemin servi + intégrité, ou scripts inlinés et hachés.
- **m-4** ND-4 impose fr **et** en ; l'exemple « Bonjour en dix lignes » (`ctx.toast("Bonjour !")`)
  ne passe pas ND-4. Montrer le modèle avec `t("hello")` et deux chaînes, ou admettre une langue
  seule avec repli signalé pour les greffons non publiés.
- **m-5** §15.4 « Tâches livrée en greffon WebAssembly » dépend de l'hôte, donc de l'enfermement
  Linux ; sur macOS et Windows (AD-18) une fonction du produit disparaîtrait. Dire le repli.
- **m-6** §2 : ISO/IEC 25010:2023 n'a pas de sous-caractéristique « extensibilité » ; la
  rattacher explicitement à modifiabilité (maintenabilité) et adaptabilité (flexibilité), comme le
  fait le paragraphe d'en-tête, et retirer « (prioritaire) » de la ligne Flexibilité.
- **m-7** TH-2 : SC 2.4.13 est AAA ; « visé » est juste, mais la porte doit dire que seul AA
  refuse un thème.
- **m-8** §15.12 « greffon de projet ne dépend que de greffons du même projet ou d'extensions
  intégrées » (API 1.1) : préciser que la décision `plugin/<id>` porte alors les dépendances et
  que le pli refuse un graphe incomplet (PP-6).
- **m-9** README « Les greffons tiers … ne demandent que des droits nommés » et §15.5 : la
  phrase montrée pour `document.read` (« lire vos documents ») ne distingue pas projet courant et
  tous les projets ; dire « les documents de ce projet » (PP-1).

---

## 5. Cohérence avec le protocole accepté (synthèse)

| Élément de la spec | Protocole | État |
|---|---|---|
| `plugin/<id>` dans `policy_op_v1` | §4.4, §20.5 liste fermée, pas de tolérance | **B-2** |
| carte `renders` sur un nœud texte | garde G1–G12, table §16 : aucune borne | B-2 (même delta) |
| domaines `inkwell/plugin/v1`, `inkwell/backup-head/v1` | §20.4 : absents | B-2 |
| version annoncée « dans le HELLO » (PV-6) | §5.3 : `VERSIONS` = majeures protocole | **M-5** |
| N et N−1 pendant 12 mois (PV-4) | §1.4.4 | cohérent |
| « admins de confiance » (§17) vs greffons de projet | l'auteur du paquet n'est pas l'admin | **B-4** |
| Stream `ext`, route `/ext/<id>/` | pont hors protocole (AD-31) | cohérent, règle B-1 de la spec à appliquer |

## 6. Sur-ingénierie et manques (synthèse)

- Sur-ingénierie : M-3 (29 interfaces, 4 kits, GREASE hôte, rejeu Wasmtime, matrice) ; deux
  hôtes dès la v1 alors qu'un seul greffon de projet n'existe pas encore (acceptable si PP-1 est
  la raison — elle l'est ; le dire).
- Manques : M-2 (canal hôte ↔ enfant), B-3 (enfermement exécutable), M-6 (racines du ménage,
  nettoyage à la désinstallation), M-8 (anti-rebond `tokens`), budget d'interaction des vues
  (M-1), a11y des vues déclaratives renvoyée à l'étude §8 sans règle normative dans §15.5
  (ajouter : libellé accessible obligatoire, refus à l'installation).

## 7. decisions.md — recommandations contestées

- AD-57 : d'accord (option 4), sous B-3 et M-1.
- AD-69 : le niveau « projet » proposé est trop large (B-4) ; la question 3 posée à l'utilisatrice
  doit être reposée avec la liste réduite.
- AD-70 : `frame` doit sortir du niveau sans geste (B-1) ; Graphviz `wasm` reste le cas de
  référence.
- AD-67 : d'accord, corriger ST-6/ST-7 (M-4) ; la portée « sauvegarde personnelle » doit
  apparaître dans `traceability.md` N22 comme réinterprétation de « remplacer le stockage », à
  confirmer par l'utilisatrice (c'est le cas, « oui (portée v1) » — bien).
- AD-71 : DX-5 à recalculer (M-3).
- AD-72 : d'accord ; ajouter au plan de `docs/plugin-api/` le chapitre « canal hôte ↔ enfant »
  (M-2) et la liste PD-* des deltas du protocole (B-2).
- AD-74 : honnête et acceptable ; le prototype de rejeu sous Wasmtime reste « à prouver » et ne
  doit pas conditionner la v1.
- AD-46–AD-56 : d'accord ; manque l'étape (M-7).

## 8. Ce qui est bien et qu'il ne faut pas toucher

Modèle de documents par nœud (MF-1–MF-7) ; principal toujours compilé (MF-8) ; projection de
tous les textes admis (MF-12) ; registre de types partagé Go/TS avec reniflage WHATWG (MF-14,
MF-15) ; grammaires embarquées sans téléchargement (MF-18) ; noyau fermé énuméré (K-1–K-10) ;
« politique ouverte, vérification fermée » (X-4) ; trois verbes sans priorité numérique (X-5) ;
registre lu sans exécuter (X-6) ; X-8 (rien dans la compilation) ; ND-1–ND-8 avec preuves
nommées ; dorsal byzantin chiffré avant l'extension (ST-3–ST-5) ; bus d'événements sans contenu
de document ; mode sans extensions ; franchise sur l'absence de débogueur dans wazero.

---

## Verdict : **NOT ACCEPTED**

Bloquants à lever pour la révision 2 : **B-1** (`frame` sans capacité ni isolation de
processus, chargé sans geste), **B-2** (`plugin/<id>` et `renders` ne sont pas des extensions
compatibles du protocole accepté : mineure 2.1, tolérance, registres, vecteurs), **B-3**
(enfermement de l'hôte : modules et cache sans fichier, AppArmor Ubuntu 24.04, plafond par hôte,
échec fermé visible), **B-4** (niveau « projet » = contribuer seulement ; décorations qui ne
masquent rien ; collage non enveloppé). Majeurs M-1 à M-8 attendus dans la même révision ;
mineurs au fil de l'eau.
