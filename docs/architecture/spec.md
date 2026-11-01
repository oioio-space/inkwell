# Architecture d'inkwell — spécification normative

| | |
|---|---|
| Version | **1.0.0-brouillon.3** (révision 2 après revue ; changements : [CHANGES.md](CHANGES.md)) |
| Date | 2026-11-01 |
| Statut | proposé (exigence N18) ; à valider par la boucle de revue Fable + Codex, puis par l'utilisatrice |
| Remplace | [`docs/architecture.md`](../architecture.md) (devenu un renvoi) |
| Finalité | [document de conception](../conception.pdf) (24 oct. 2026) |
| Décisions | [ADR](../adr/README.md) ; doutes et options : [decisions.md](decisions.md) (`AD-xx`) |
| Protocole | [docs/protocol/spec.md](../protocol/spec.md) (2.0.0-brouillon.10, [ADR 0034](../adr/0034-protocole-unifie.md)) |
| Traçabilité | [traceability.md](traceability.md) |
| Études | [research/plugins.md](research/plugins.md) (greffons), [research/editor-features.md](research/editor-features.md) (écriture), [research/ai.md](research/ai.md) (IA) |

## 0. Comment lire ce document

### 0.1 Mots normatifs

Les mots-clés **DOIT**, **NE DOIT PAS**, **OBLIGATOIRE**, **DEVRAIT**, **NE DEVRAIT PAS**,
**RECOMMANDÉ**, **PEUT** et **OPTIONNEL** ont le sens du BCP 14 ([RFC 2119], [RFC 8174])
quand, et seulement quand, ils sont écrits en gras.

### 0.2 Marques d'état

Chaque règle ou élément porte, si besoin, une marque :

- *[en place]* : le code le fait aujourd'hui (2026-11-01, `main` à `362cb4d`) ;
- *[cible]* : décidé, pas encore fait ; le chemin est indiqué (ADR, étape, `AD-xx`) ;
- *[à décider AD-xx]* : la règle dépend d'une décision ouverte de [decisions.md](decisions.md).

Une règle sans marque est *[en place]*. Une marque posée sur une section vaut pour toute la section ; une marque locale la précise sans la lever.

### 0.3 Ordre des sources

En cas de désaccord : une ADR acceptée l'emporte sur la spécification du protocole, qui
l'emporte sur ce document, qui l'emporte sur les plans (`docs/plans/`). Ce document ne
contredit aucune ADR acceptée ; là où il propose de changer une décision, il renvoie à une
`AD-xx` qui demande une nouvelle ADR. La spécification du protocole est normative pour
tout octet échangé entre appareils ; ce document l'est pour tout le reste.

### 0.4 Vocabulaire

« Pair » (code) = un appareil ; « personne » (interface) = un groupe d'appareils
([glossaire](../glossary.md)). « Projet » = un dossier partagé. « Modification » = un
changement Automerge signé. « Acteur » = une goroutine qui possède seule un état et le
modifie sur ordre reçu par une file.

## 1. Finalité, objectifs, non-objectifs

### 1.1 Finalité

> « Un éditeur de bureau en Go où plusieurs personnes éditent en direct des documents
> Typst, Markdown et texte, sans serveur, avec trace de qui a écrit quoi, historique de
> versions et export PDF. Les pairs se trouvent sur le LAN ou par un code court, et l'édition
> continue hors ligne. » — conception, p. 1.

Premier jalon mesurable : à la fin de l'étape 3, écrire le document de conception à deux dans
l'application ([PROGRESS.md](../../PROGRESS.md)).

### 1.2 Objectifs

| # | Objectif | Source |
|---|---|---|
| G1 | Édition à plusieurs, en direct, de Typst, Markdown, texte et tableaux CSV, avec curseurs et présence | conception p. 1, 32 ; U19–U22 |
| G2 | Aucun serveur : pas de compte, pas de cloud ; un relais optionnel ne voit que du chiffré | conception p. 1, 15, 19 |
| G3 | Hors ligne d'abord : chaque frappe gardée localement ; fusion au retour, sans perte | conception p. 1, 14 ; U32 |
| G4 | Qui a écrit quoi ; historique, versions nommées, retour arrière, divergences signalées et réglables | conception p. 6–11 ; U11, U12, U27 |
| G5 | Aperçu et export PDF déterministes, entièrement hors ligne | conception p. 5, 13 ; ADR 0024 |
| G6 | Confiance explicite : invitation, liste signée, rôles, suspension, retrait, zones verrouillées ; une personne malveillante ne gêne jamais les autres | conception p. 3–4, 20 ; N2–N4, N7 |
| G7 | Projet multi-fichiers : textes, binaires, paquets ; rejoindre = tout recevoir | conception p. 5, 12, 31 ; N5 ; ADR 0023 |
| G8 | Interface Adwaita, zéro jargon, accessible, fr/en, cohérente | conception p. 13, 37 ; U1–U3, U31 |
| G9 | Fiable et rapide, y compris par Internet et sur de très gros fichiers | U32, U33 ; ADR 0028, 0029 |
| G10 | Un seul exécutable autonome ; `.deb` et `.rpm` amd64 et arm64 ; SemVer ; signé | L1–L5 ; ADR 0025 |
| G11 | Un protocole public, versionné, réutilisable par d'autres applications | N12–N16 ; ADR 0034 |
| G12 | **Hautement extensible** : toute fonction visible est un point d'extension ; le cœur est livré en extensions sur la même API ; greffons WebAssembly multi-langages, de l'appareil ou du projet ; sans dénaturer l'application ni la ralentir | N20–N24 ; §15 ; ADR 0039 (proposée) |

### 1.3 Non-objectifs (décidés)

| # | Hors périmètre | Source |
|---|---|---|
| NG1 | Application mobile ; édition WYSIWYG du PDF ; édition native RTF/DOCX (import/export seulement) | conception p. 39 |
| NG2 | Synchronisation par un compte en ligne ; serveur de découverte global | conception p. 19, 39 |
| NG3 | Chiffrement au repos en v1 (le disque de l'appareil est de confiance ; chiffrement du disque par le système recommandé) | conception p. 4, 39 ; ADR 0005 |
| NG4 | Permissions par dossier ; capacités transitives (Keyhive) en v1 | conception p. 39 ; ADR 0005 |
| NG5 | Go qui lit ou fusionne Automerge | ADR 0002, 0011 |
| NG6 | Téléchargement à l'exécution (sidecars, paquets Typst, polices) | ADR 0024 |
| NG7 | Port réseau local pour le pont webview ↔ Go | ADR 0009 |
| NG8 | Télémétrie ; tout envoi automatique de journaux | conception p. 15 |
| NG9 | Consensus ou quorum (l'édition hors ligne passe avant ; l'auto-équivoque reste arbitrée par un admin) | ADR 0031 R1 |
| NG10 | v2, hors de ce document sauf mention : brouillons privés et branches, résumés par modèle, Tiptap, aperçu partagé pour un invité sans l'application, télécommande téléphone, `join` sans fenêtre | conception p. 7, 38 ; `AD-34`, `AD-35` |

### 1.4 Invariants

Ces invariants reprennent ceux de [`CLAUDE.md`](../../CLAUDE.md) ; les changer exige une ADR.

| # | Invariant | Vérifié par |
|---|---|---|
| INV-1 | Le CRDT vit dans la webview ; Go ne transporte que des octets opaques ; aucun code Go n'importe Automerge | `arch-check.sh` (graphe complet, tests compris) |
| INV-2 | Tout le réseau entre appareils passe par `internal/transport` ; seuls `internal/transport/p2p` et le module `protocol/libp2p` importent libp2p ; le rendez-vous Wormhole est un canal séparé limité à `internal/invite` (ADR 0032) | `arch-check.sh` (AC-4, AC-5 ; AC-10 *[cible]*) |
| INV-3 | Une seule source de vérité par chose (§6.1) | revue ; §6 |
| INV-4 | Tout octet venu d'un pair est borné, décodé strictement, vérifié (signature, rôle) avant usage, et son décodeur est fuzzé | cibles `Fuzz*` ; vecteurs ; P-FUZZ |
| INV-5 | Pont webview ↔ Go = Streams Wails ; aucun port local ; tout autre serveur local écoute sur `127.0.0.1`, port éphémère, jeton comparé en temps constant, jamais journalisé | revue ; `AD-22` |
| INV-6 | Typst : aucun téléchargement à l'exécution ; `typst` et `tinymist` épinglés, embarqués, revérifiés (SHA-256) avant chaque lancement | `arch-check.sh` (AC-6, AC-7) ; tests `SIDE-*` |
| INV-7 | cgo requis par Wails ; construction native par système et par architecture | CI ; ADR 0003 |
| INV-8 | « pair » dans le code, « personne » dans l'interface ; zéro jargon | glossaire ; garde i18n *[cible]* (§2) |
| INV-9 | Un paquet = un rôle, créé à l'étape qui en a besoin | §4 ; revue |
| INV-10 | Le module `protocol` n'importe rien d'inkwell ; inkwell n'a aucune copie privée de ses codecs | AC-9 *[cible, ADR 0034 étape 1]* |
| INV-11 | *[proposé, ADR 0039]* Le noyau fermé est la liste K-1 à K-10 (§15.3) ; tout le reste est un point d'extension ; une extension intégrée n'a aucun droit qu'un greffon ne puisse demander (sauf `ui.native`) ; aucune extension ne retire une zone protégée (ND-1 à ND-8) | AC-16, FE-3, P-EXT-*, `EXT-ND*` |

## 2. Attributs de qualité (ISO/IEC 25010:2023) et budgets

Chaque caractéristique du modèle a au moins une exigence mesurable, une mesure et une porte.
Les budgets de performance reprennent l'[ADR 0028](../adr/0028-performance-huge-files-and-wan.md)
(qui précise ceux de [engineering-principles.md](../engineering-principles.md) §3) ; ceux de
fiabilité l'[ADR 0029](../adr/0029-reliability.md). Un budget marqué *proposé* est nouveau ici.

**Attributs prioritaires.** Par ordre : sécurité (N7), fiabilité (U32), **extensibilité** (maintenabilité : modularité, modifiabilité ; flexibilité : adaptabilité — N20 à N24, exigence explicite « hautement extensible »), efficacité (U33), capacité d'interaction. Un arbitrage entre eux suit cet ordre et se note dans une ADR. L'extensibilité ne se paie ni en sécurité (noyau fermé, capacités, §15.3, §15.5) ni en performance (activation paresseuse, budgets par point, §15.4).

| Caractéristique | Sous-caractéristique | Exigence mesurable | Mesure, porte |
|---|---|---|---|
| Adéquation fonctionnelle | exactitude | après guérison et repos, N répliques ont textes et têtes identiques ; aucune modification non couverte admise ; toute frappe acquittée avant un arrêt est présente | propriétés fast-check (3 `Repo` réels, 20 graines CI, 200 la nuit) ; ADR 0029 §IV |
| | complétude | critère de sortie de chaque étape démontré | PROGRESS.md ; e2e, démo, tâche chronométrée |
| | pertinence | les cinq tâches (créer, partager, relire, présenter, exporter) réussies | tests en personne, temps consigné ([ux-principles](../ux-principles.md)) |
| Efficacité | temps de réponse | frappe → affichage local p95 ≤ 16 ms ; frappe → pair LAN p95 ≤ 100 ms (≤ 128 Ki car.), ≤ 1 s (≤ 2 Mi) ; par relais (RTT 100 ms, 1 % de pertes) p95 ≤ 300 ms ; frappe pendant un message de 6 Mo à 1 Mbit/s p95 ≤ 500 ms ; démarrage : fenêtre peinte ≤ 1 s, dernier projet modifiable ≤ 1,5 s ; document de 50 000 opérations ouvert < 2 s | cas `PERF-*` (`INKWELL_PERF=1`) ; e2e ; condition de mesure §12.3 (`AD-27`) |
| | | aperçu : p95 ≤ 1 s après la dernière frappe pour ≤ 20 pages *(proposé, `typst compile`)* ; ≤ 300 ms après l'aperçu incrémental *(cible, `AD-26`)* | e2e `preview.spec.ts` chronométré |
| | utilisation des ressources | webview ≤ 600 Mo (projet ≤ 2 Mi ouvert), alerte 1,5 Go ; WASM ≤ 1 Go par Worker ; processus Go ≤ 300 Mo RSS pour 2 Mi de texte + 20 Mo de blobs *(proposé)* ; 0 allocation par trame ≤ 64 Kio | `PERF-*` ; bancs `HOT_PATHS` (`-benchmem`) |
| | capacité | 64 appareils connectés par projet (128 connexions) ; 1024 opérations de membres ; 10 000 nœuds d'arbre ; 10 000 documents couverts ; 100 Mo par fichier binaire, 2 Go par projet (au-delà, confirmation) | tests de borne (P-BORNES) ; `AD-23` |
| Compatibilité | coexistence | un éditeur externe sur le même dossier : modification détectée, jamais écrasée sans accord | tests `EXT-*` ; règle de propriété (ADR 0020) |
| | interopérabilité | vecteurs du protocole passés à 100 % en Go et en TypeScript ; majeures N et N−1 parlées 12 mois ; fichiers projetés lisibles par `typst` en ligne de commande et par Git | P-VEC, P-COMPAT ; test de projection |
| Capacité d'interaction | apprentissage, opérabilité | première minute sans question réseau avant le premier partage ; trois accès à toute action (clic, raccourci, palette) | `ux-walkthrough` ; e2e |
| | inclusivité | WCAG 2.2 niveau AA ; 0 violation axe par écran ; cibles 44 px ; clavier complet ; fr et en | e2e `@axe-core/playwright` ; revue |
| | protection contre l'erreur | toute action destructive est réversible (toast « Annuler », corbeille ≥ 30 jours) | tests `FM-*` |
| | auto-description | aucun terme de la colonne « Ne jamais dire » du glossaire dans les chaînes d'interface | garde sur `frontend/src/i18n` *(cible)* |
| Fiabilité | absence de défaut | aucun test déclaré instable sans taux mesuré et cause prouvée ; un test corrigé passe 500 exécutions de suite | décision D14 de l'audit |
| | disponibilité | l'édition ne dépend ni d'un pair, ni d'un admin, ni du réseau | scénarios hors ligne ; ADR 0017 |
| | tolérance aux fautes | un pair lent ou malveillant ne ralentit ni ne bloque les autres | portes P-BYZ, P-BORNES (ADR 0034) |
| | récupérabilité | perte au plus 200 ms de frappe sur `kill -9` (I1), 500 ms sur coupure de courant (I2) ; jamais de fichier à moitié écrit (I3) ; disque plein sans perte, reprise automatique (I5) | `TestKillDuringWrite`, `TestKillAtRandomTimes`, LazyFS, `faultinject` |
| Sécurité | confidentialité | 100 % du trafic entre appareils chiffré (TLS 1.3 ou Noise) ; aucune donnée de document dans les journaux, la trace réseau ou un rapport | test canari *(cible, §11)* |
| | intégrité | tout octet stocké ou reçu vérifié avant usage (IKV1, Bao, signature) | `TestBitFlipQuarantined` ; fuzz |
| | non-répudiation, responsabilité | chaque modification signée par l'appareil de son auteur (G12) ; attribution par personne | ADR 0031 ; vecteurs |
| | authenticité | clés validées (ordre premier, encodage canonique) à l'entrée et au rechargement ; empreinte en 4 mots | vecteurs Chalkias ; N-1 |
| | résistance | chaque file, carte et octet disque borné avec propriétaire ; OWASP ASVS 5.0 niveau 2 (V2.4.1) ; chaque décodeur fuzzé 1 h sans plantage | table §16 du protocole ; P-FUZZ ; revue `security-auditor` |
| Maintenabilité | modularité | 0 violation des règles de dépendance (§4.5) | `mise run arch` |
| | réutilisabilité | module `protocol` utilisable seul (aucun import d'inkwell) | AC-9 *(cible)* |
| | analysabilité | complexité cognitive et cyclomatique ≤ 30 (échec), cible 15 | golangci-lint (gocognit, gocyclo) |
| | modifiabilité | aucun doublon ≥ 100 jetons | dupl |
| | testabilité | couverture ≥ 85 % par paquet (exemptions listées dans `scripts/cover.sh`) ; score de mutation ≥ 80 % sur `protocol/trust` et `protocol/session` *(proposé)* | `cover:check` ; `mutate:go` |
| Flexibilité | adaptabilité | Linux amd64 et arm64 en v1 ; macOS et Windows selon `AD-18` | CI native |
| | installabilité | `.deb` et `.rpm` installés puis e2e « fumée » sur Debian 13, Ubuntu 24.04, Fedora 44, amd64 et arm64 | `pkg:test` (`PKG-01…12`) |
| | remplaçabilité | chaque frontière (transport, stockage, compilateur, pont) a une doublure de test | `memnet`, `transporttest`, faux magasins |
| | modifiabilité et adaptabilité, au service de l'extensibilité (N20–N26) | 100 % des fonctions visibles inscrites par le registre des contributions ; chaque point du §15.4 utilisé par au moins une extension intégrée ; 0 import interne depuis `src/ext/**` ; ajouter un type, une commande, un panneau, un exportateur = une contribution, sans toucher au cœur ; démarrage : ≤ 10 ms ajoutées par 50 extensions | P-EXT-1 à P-EXT-7 ; `BenchmarkRegistryBuild` ; ADR 0027, 0039 |
| Sûreté | sécurité intégrée | sidecars enfermés (§9.5) ; échec fermé si l'enfermement manque | `TestRunNoNetwork`, `TestRunDirectoryBound` |
| | repli sûr | dans le doute : lecture seule plutôt que corruption ; refus d'écraser une modification externe ; quarantaine plutôt que suppression | tests `EXT-*`, quarantaine |
| | alerte | disque plein, liste scellée, lien lent, croissance anormale : bandeau qui dit quoi faire | e2e de reprise |

## 3. Vues C4

Notation [C4](https://c4model.com/) en texte. Les flèches vont de l'appelant vers l'appelé ;
le libellé dit ce qui passe.

### 3.1 Contexte

```
                    Personne (utilisatrice)
                           │ écrit, relit, invite, exporte
                           ▼
 Éditeur externe ──► ┌────────────────────────────┐ ──► Système de fichiers
 (VS Code…)  fichiers│ inkwell sur cet appareil   │     Documents/Inkwell/<Projet>/
                     └──┬────────┬────────┬───────┘
     libp2p (QUIC/TLS,  │  mDNS  │        │ texte d'invitation (PAKE)
     TCP/Noise)         │ (LAN)  │        ▼
                        │        │   Serveur de rendez-vous Wormhole
                        ▼        ▼   (public par défaut, configurable, auto-hébergeable)
         inkwell sur les appareils des autres personnes du projet
                        ▲
                        │ circuit v2, chiffré de bout en bout
         Relais inkwell-relay (optionnel, tenu par une personne du groupe — AD-43)

 Vérification de mise à jour (optionnelle, désactivable — AD-17) ──► page des versions publiées
 À la construction seulement (jamais à l'exécution) : archives officielles typst/tinymist (SHA-256 épinglés)
```

Flux qui sortent de l'appareil, tous faits par Go et chacun désactivable (conception : « tout ce
qui se voit se désactive ») : libp2p vers les membres (et le relais), mDNS sur le LAN, le
rendez-vous Wormhole pendant une invitation, la vérification de mise à jour ; et, *[cible]*,
seulement si la personne les active : le dorsal de sauvegarde (§6.7, chiffré), les hôtes
nommés d'un greffon (`net.hosts`, §15.5), le fournisseur d'IA autorisé (§15.18). Entrée locale
*[cible]* : le serveur MCP (socket Unix `0600`, ou `127.0.0.1` avec jeton en option).

### 3.2 Conteneurs

```
┌──────────────────────── processus inkwell (un exécutable Go, cgo, Wails v3) ─────────────────────────┐
│                                                                                                     │
│  ┌──────────────────── webview (processus web WebKitGTK 6.0, GTK4) ───────────────────────┐         │
│  │ fil d'affichage : Svelte 5, CodeMirror 6, Adwaita CSS, i18n                            │         │
│  │ Worker Automerge [cible ADR 0028 D2, prérequis ADR 0031] :                              │         │
│  │   Repo automerge-repo, WailsNetworkAdapter, GoStorageAdapter, admission, couverture,    │         │
│  │   garde G1–G12, @inkwell/protocol                                                       │         │
│  │ Workers d'analyse : historique, recherche, lecture CSV                                  │         │
│  └──────────┬───────────────────────────────────────────────────────────▲────────────────┘         │
│     Streams │ net · store · preview · present · lsp [cible] · view [cible]                           │
│     Bindings│ Session · Projects · PreviewService · PresentService                                   │
│  ┌──────────▼────────────────────────────────────────────────────────────┴────────────────┐         │
│  │ cœur Go : app, bridge, ledger [cible], transport/{p2p,session}, invite, identity,       │         │
│  │ store, project, blob, preview [cible], typst, sidecar, lsp [cible], textview [cible]    │         │
│  └───────┬──────────────────────────┬────────────────────────────┬────────────────────────┘         │
└──────────┼──────────────────────────┼────────────────────────────┼──────────────────────────────────┘
           │ fork/exec, enfermés       │ os.Root                    │ libp2p
   typst (une par compilation)   Dossier du projet              autres appareils, relais
   tinymist lsp [cible, AD-20]   <Projet>/ + .inkwell/ (0700)
                                 Configuration : <UserConfigDir>/inkwell/ (clé de l'appareil, réglages)
                                 Cache : <UserCacheDir>/inkwell/<version>/ (sidecars extraits)
                                 État : journaux [cible, AD-16]

 Autres exécutables : inkwell-relay (relais circuit v2) ; sidecar-fetch (construction seulement).
```

| Conteneur | Technologie | Responsabilité | Ne fait pas |
|---|---|---|---|
| Fil d'affichage | Svelte 5, CodeMirror 6 | afficher, saisir, panneaux, aperçu en images | aucun accès disque ni réseau ; aucun Automerge après ADR 0028 D2 |
| Worker Automerge *[cible]* | automerge 3.5, automerge-repo 2.5 | fusion, admission, couverture, garde, historique | aucun accès disque ni réseau direct (tout passe par les Streams) |
| Cœur Go | Go 1.27, Wails v3 | réseau, confiance, disque, binaires, Typst | ne décode jamais Automerge |
| `typst` | binaire épinglé | compiler un projet en PDF ou SVG | aucun réseau ; rien hors du projet ; borné |
| `tinymist lsp` *[cible]* | binaire épinglé, stdio | complétion, survol, diagnostics à la frappe | aucun port ; aucune écriture |
| Dossier du projet | fichiers | projection lisible, magasin, listes signées | — |
| `inkwell-relay` | Go + libp2p | relais circuit v2 | ne voit jamais le contenu |
| Hôte des greffons *[cible, ADR 0039]* | `inkwell --plugin-host`, wazero | exécuter les greffons WebAssembly ; un hôte global, un par projet ouvert avec greffons | aucun fichier, aucun réseau, aucun secret ; tout passe par Go (capacités) |
| Rendu à DOM *[différé, P-FRAME]* | processus web dédié, à prouver | rendre un diagramme pour les moteurs qui exigent une mise en page de navigateur (§15.14) | rien en v1 |

### 3.3 Composants du cœur Go

```
cmd/inkwell ── processus : instance unique, Wails, Streams, services, arrêt ordonné
   │
internal/app ── composition par projet (Runtime), cas d'usage (inviter, rejoindre, fichiers), services Wails
   │
   ├── bridge ────────── codecs et routage des Streams (net, store, present), crédits, époques
   ├── ledger [cible] ── acteur d'autorité : ensembles signés, rôles, décisions, annonces, attestations
   │      └─ protocol/{trust,policy,listing,sig}   (cœur pur)
   ├── invite ────────── salon d'invitation, ticket, rendez-vous Wormhole
   ├── transport ─────── port réseau (PeerID, Transport, Conn/Stream [cible AD-06])
   │      ├─ session [cible] ── acteur par pair : classes, générations, SET, LARGE (protocol/{sync2,session})
   │      ├─ p2p ────────────── hôte libp2p, filtre de connexion, vrac, salon (protocol/libp2p)
   │      └─ memnet, transporttest ── simulation et contrat
   ├── blob ──────────── magasin BLAKE3/Bao, quotas, échange en tirage (protocol/blob)
   ├── project ───────── dossier, project.json, propriété, arbre, corbeille ── store (KV atomique, os.Root)
   ├── preview [cible] ─ recompilation, pages visibles, export ── typst ── sidecar
   ├── lsp [cible] ───── tinymist sur stdio ── sidecar
   ├── textview [cible]─ très gros fichiers paginés (ADR 0028 D4)
   ├── plugin [cible] ── gestionnaire d'extensions : paquets, signatures, droits, hôte des greffons (wazero, processus enfant), assainisseur SVG
   ├── settings [cible] ─ réglages de l'appareil (AD-21)
   ├── identity ──────── clé de l'appareil, empreinte
   └── version
```

### 3.4 Composants de la webview

```
src/workspace ── racine de composition : Repo, adaptateurs, chargement, projets
   ├── src/net ───── WailsNetworkAdapter, admission, couverture, enveloppe (→ @inkwell/protocol)
   ├── src/store ─── GoStorageAdapter, projection, synchro externe, arbre, paliers de taille
   ├── src/history ─ analyse (Worker), modèle d'historique
   ├── src/editor ── extensions CodeMirror ; éditeurs par type (csv…) ; recherche ; mise en forme
   ├── src/preview ─ client du Stream preview, synchro source ↔ aperçu
   ├── src/present ─ fenêtres de présentation (Stream present)
   ├── src/filetypes [cible, ADR 0027] ── registre des types de fichiers
   ├── src/worker [cible] ── hôte du Repo dans le Worker
   └── src/ui ────── composants Svelte (coque, personnes, historique, tableur, barres)
src/extensions [cible] ── registre des contributions, hôte TypeScript des extensions, rendu des vues déclaratives
src/ext/<nom> [cible] ── extensions intégrées (éditeur, tableur, aperçu, historique, diagrammes…)
packages/extension-api [cible] ── @inkwell/extension-api : types générés du WIT
packages/protocol [cible] ── @inkwell/protocol : profil CBOR, schémas, garde, crédits, vecteurs
```

### 3.5 Trois parcours

**Une frappe chez Paul apparaît chez Marie (après la migration ADR 0034).**

1. CodeMirror applique la frappe localement (≤ 16 ms) et l'envoie au Worker.
2. Le Worker l'applique au document Automerge ; la fabrique signe le tuple d'auteur
   (Stream `net`, kind `Sign`) ; automerge-repo produit un message de synchronisation
   (au plus un toutes les 100 ms par document pendant une rafale).
3. Le message traverse le Stream `net` (kind `Message`, époque `(session, génération)`) sous
   crédit ; `bridge.Router` le remet à l'acteur de session du pair.
4. L'acteur de session l'écrit sur le flux `/inkwell/repo/2.0.0` (ou un flux LARGE s'il
   dépasse 64 Kio). QUIC le chiffre.
5. Chez Marie : `p2p` accepte le flux d'un membre admis ; l'acteur de session vérifie la
   trame (§12 du protocole) et la dépose, bornée, vers `bridge.Router`.
6. Le Worker de Marie : profil CBOR, admission, couverture, garde G1–G12, puis `Repo` ;
   les correctifs partent vers le fil d'affichage. Le crédit est rendu.
7. Le GoStorageAdapter de Marie persiste l'incrément par le Stream `store` (acquitté après
   écriture atomique ; `fsync` groupé, ADR 0029 I2).

**Une personne rejoint.** Code Wormhole ou texte d'invitation → ticket (15 min, usage
unique) → salon `/inkwell/lobby/1.0.0` → l'hôte accepte → op signée d'admission → la liste
pliée admet le nouvel appareil → sessions → SET réplique les ensembles signés → la webview
reçoit l'ensemble admis puis l'index, les textes, et les blobs voulus par le vrac.

**Exporter en PDF.** La webview a déjà projeté chaque texte sur le disque (300 ms après la
dernière frappe) → `PreviewService.Export` → `typst compile --root <projet>
--creation-timestamp` dans un processus enfermé → PDF vérifié par `os.Root` → boîte de
dialogue côté Go → écriture atomique.

## 4. Paquets et modules

### 4.1 Règles générales

- **P-1** Un paquet **DOIT** avoir un seul rôle, dit en une phrase dans son commentaire de
  paquet. Il **NE DOIT PAS** être créé avant l'étape qui en a besoin.
- **P-2** Les interfaces **DOIVENT** être petites et définies chez le consommateur ; un
  constructeur accepte des interfaces et rend des structures ; chaque implémentation porte
  `var _ I = (*T)(nil)`.
- **P-3** Aucune variable globale mutable ni `init()` à effet de bord ; la configuration passe
  par le constructeur.
- **P-4** Chaque état mutable a un propriétaire unique (§5.2) ; la documentation du type dit
  lequel.
- **P-5** Un paquet qui lit des octets venus d'un pair **DOIT** avoir une cible `Fuzz*` par
  décodeur ; un paquet du chemin chaud **DOIT** avoir des `Benchmark*` et figurer dans
  `HOT_PATHS:` de `CLAUDE.md`.
- **P-6** Les codecs du fil appartiennent au module `protocol` ; inkwell n'en garde aucune
  copie. Les alias de types qui renvoient vers `protocol` (`internal/trust/protocol.go` du
  chantier en cours) ne sont tolérés que pendant l'étape 1 de la migration et **DOIVENT**
  disparaître avant la fusion de l'étape 2 (`AD-03`).

### 4.2 Catalogue des paquets Go

Pour chaque paquet : rôle ; API publique (principaux identifiants exportés) ; dépendances
internes permises (le reste est interdit) ; état possédé et son propriétaire ; concurrence et
arrêt ; statut.

**`cmd/inkwell`** — le processus.
- API : `main` ; `--version`.
- Dépend de : tout `internal/*`, Wails.
- État : l'application Wails, les boucles de premier niveau (`shutdownLoops`).
- Concurrence : enregistre les Streams et les services ; possède l'arrêt (§5.4). Instance
  unique par dossier de configuration *[cible, `AD-12`]*.
- Statut : *[en place]* ; la composition par projet est dans `internal/app` (`AD-04`).

**`internal/app`** — la couche application : compose les paquets pour un projet ouvert,
porte les cas d'usage qui en traversent plusieurs, et les expose au frontend.
- API : `Start(ctx, home, Options) (*Runtime, error)`, `Runtime.Close` ; services Wails
  `Session` (identité, inviter, rejoindre, personnes, visibilité), `Projects`,
  `PreviewService`, `PresentService` ; `OpenHome`, `OpenDocs`, `OpenProject` ; `CSPNonce`.
- Dépend de : tous les paquets internes sauf `cmd/*` ; **personne** n'importe `app` (AC-1).
- État : le `Runtime` d'un projet ; les invitations en cours ; l'orchestration des fichiers
  (`Files`).
- Concurrence : `Runtime.Close` attend chaque goroutine qu'il a lancée (`sync.WaitGroup.Go`).
- Statut : *[en place, à réduire]* — aujourd'hui aussi acteur d'autorité (`wiring.go`,
  `attest.go`, `persist.go`, `suspend.go`, `people.go`) et aperçu (`preview.go`) : ARC-M5 de
  l'audit. Cible : ces rôles partent dans `internal/ledger` et `internal/preview`
  (`AD-02`, `AD-05`) ; le hub `present` part dans `internal/bridge` (`AD-29`).

**`internal/ledger`** *[cible, `AD-02`]* — l'acteur d'autorité du projet : il possède les
ensembles signés (membres, décisions d'admin, annonces, accusés finals, preuves d'équivoque),
les attestations gardées, leur persistance, et en publie les vues.
- API : `Open(dir *os.Root, s Signer, o Options) (*Ledger, error)` ; commandes `Issue`,
  `Decide`, `Announce`, `Attest`, `Adopt` (envoyées à la boîte de l'acteur) ; lectures sans
  verrou `Snapshot() *View` (liste pliée, décisions, ensemble admis), `Admit(PeerID) bool`
  (filtre de connexion) ; `Events() <-chan Event` (notifications coalescées : rôles,
  attestations, retraits, suspensions, décisions, ensemble admis, tuples d'auteur, liste
  scellée) ; `Close`.
- Dépend de : `protocol/*`, `identity`, `store`, `transport` (pour `PeerID`). **NE DOIT PAS**
  dépendre de `app`, `bridge`, `blob`, `project`, `typst`, `sidecar`, `preview`, `invite`.
- État : tout ce que §15 du protocole attribue à « l'acteur `Wiring` » ; les attestations
  dans un magasin concurrent à parts par signataire.
- Concurrence : une goroutine d'acteur (boîte `do`) et un travailleur de persistance (§5.1
  C-6) ; « durable avant diffusé » (protocole §14.2).
- Statut : *[cible]* ; remplace `app.Wiring`, `app.opStore`, `app.attestStore`. Rejoint
  `HOT_PATHS:` avec `BenchmarkRelay`.

**`internal/bridge`** — les Streams Wails : un codec borné par Stream, le routage et la
contre-pression ; aucun sens donné aux octets.
- API : `Envelope`, `Kind`, `Encode`/`Decode*` (Stream `net`), `Router` (net ↔ port du
  dépôt), `StoreServer`, `StoreRequest`/`StoreResponse` (Stream `store`) ; hub de
  présentation *[cible]* ; ports `Source`, `Attester`, `KV`, `Projection`, `Tree`, `Conn`.
- Dépend de : `transport` (`PeerID`), `protocol` *[cible]*. **NE DEVRAIT PAS** dépendre de
  `project` ni de `store` (ports et types de transfert à lui, `AD-29`).
- État : crédits par (pair, époque, voie), notifications en attente coalescées, baux de
  stockage par voie ; propriétaire : la goroutine de chaque connexion de Stream.
- Concurrence : par connexion de Stream, un lecteur et un écrivain ; `StoreServer` : un
  travailleur par voie (≤ écrivains + 2, ADR 0031) *[cible ; aujourd'hui séquentiel]*.
- Statut : *[en place]* ; version 1 des enveloppes ; crédits et époques à l'étape 6 d'ADR 0034.

**`internal/transport`** — la frontière réseau : identifiants de pairs et port vers le réseau.
- API : `PeerID` (alias de `protocol.PeerID` *[cible]*), `ValidPeerID`, `Transport`
  (`Local`, `Send`, `Recv`, `Peers`, `Close`), `Event`, `Kind`, `ErrClosed`,
  `ErrUnknownPeer`, `ErrFrameTooLarge` ; port de session `Conn`/`Stream` *[cible, `AD-06`]*.
- Dépend de : rien d'interne (AC-3), `protocol` *[cible]*.
- État : aucun.
- Statut : *[en place]*.

**`internal/transport/session`** *[cible, ADR 0034 étape 4]* — l'acteur d'une session avec un
pair : flux par classe, générations, files, curseurs SET, objets LARGE, HELLO, GOAWAY.
- API : `Run(ctx, conn transport.Conn, deps Deps) error` ; dépôt non bloquant
  `Deposit(Message) bool`.
- Dépend de : `transport`, `protocol/{sync2,session}` ; jamais de libp2p (la connexion lui
  est donnée par le port `Conn`).
- État : §11.1 et §7.3 du protocole, pour une session.
- Concurrence : une goroutine par session plus un lecteur par flux entrant ; sortie sur
  GOAWAY, fin de connexion ou annulation.
- Statut : *[cible]* ; remplace `internal/transport/mux` à l'étape 10. Rejoint `HOT_PATHS:`.

**`internal/transport/p2p`** — l'hôte libp2p.
- API : `New`, `Node` (implémente `Transport`), `Node.Bulk()`, salon (`LobbyHandler`),
  `Link`/`LinkKind`, `Nearby`, `RunRelay`, `IDFromKey`, `KeyFromID`.
- Dépend de : `transport`, `protocol/libp2p` *[cible]*, libp2p (seul paquet d'inkwell qui
  l'importe, AC-4).
- État : hôte, liens, instantané immuable des connexions.
- Concurrence : boucles internes de libp2p plus une boucle de liens ; `Close` arrête les
  services de numérotation **avant** le swarm (décision D13 de l'audit, F3).
- Statut : *[en place]*.

**`internal/transport/memnet`** — réseau en mémoire, fautes injectées, déterministe sous
`testing/synctest`. Implémentera le port `Conn`/`Stream` avec un crédit par flux (pour tester
l'absence de blocage en tête de file) *[cible, `AD-06`]*.

**`internal/transport/transporttest`** — suite de contrat partagée par les transports.

**`internal/transport/frame`, `internal/transport/mux`** — *[en place, supprimés à l'étape 10
d'ADR 0034]* ; leurs bancs passent à `protocol/sync2` et `internal/transport/session`.

**`internal/identity`** — la clé Ed25519 de l'appareil et son affichage.
- API : `LoadOrCreate(*os.Root) (*Device, error)`, `Device.Public`, `Device.Sign(domain,
  msg)`, `Device.Libp2pKey`, `Fingerprint`, `MachineName`, `CleanName`,
  `ValidatePublicKey` (→ `protocol/sig` *[cible]*).
- Dépend de : `protocol/sig` *[cible]* ; ni `transport` (AC-2) ni rien d'applicatif.
- État : `device.key` (0600), lu une fois ; la clé privée ne sort que par `Libp2pKey`.

**`internal/invite`** — inviter et être invité : salon, ticket, rendez-vous Wormhole.
- API : `Host`, `HostConfig`, `Joiner`, `Joined`, `Lobby`, `Request`/`Reply`, `Ticket`
  (→ `protocol/invite`), `Wormhole`, `LooksLikeCode`.
- Dépend de : `identity`, `transport`, `protocol/{invite,trust}` ; seul importateur de
  wormhole-william (AC-5).
- État : fenêtre de visibilité, tickets (15 min, usage unique), demandes en attente.

**`internal/store`** — un magasin clé-valeur atomique et vérifié sur `os.Root`.
- API : `KV`, `Entry`, `WriteAtomic`, `WriteAtomicFrom`, `SyncDir`, `ValidKey`,
  `ValidPrefix` ; validation groupée des `fsync` *[cible, ADR 0029 I2]*.
- Dépend de : rien d'interne.
- État : fichiers `IKV1` ; quarantaine ; sauvegardes.

**`internal/project`** — le projet comme dossier visible.
- API : `Project` (ouvrir, créer, `Latest`, `Meta`, `Watch`, `Acknowledge`, `ApplyTree`,
  `WriteBlob`, corbeille, `Backup`), `ValidateName`, `ValidatePath`, `CollisionKey`,
  `TreeEntry`, `Text`.
- Dépend de : `store`.
- État : `project.json` (fichiers possédés et leur SHA-256), surveillance fsnotify ; verrou
  du projet *[cible, `AD-12`]*.
- Concurrence : une goroutine de surveillance, sortie à l'annulation.

**`internal/blob`** — les binaires par empreinte.
- API : `Store` (mettre, lire, tranche, quotas, ménage), `Service` (tirage par le vrac),
  `Ref`, `Hash`, `QuotaError`, `GCResult`.
- Dépend de : `store`, `transport`, `protocol/blob` *[cible]*.
- État : `.inkwell/blobs/`, arbres Bao, cartes de reprise ; transferts en cours.
- Concurrence : une pompe, au plus 4 transferts, un ménage périodique ; `Run` attend tout.
- Statut : *[en place]* ; chemin chaud (`HOT_PATHS:`).

**`internal/sidecar`** — lancer un binaire embarqué, vérifié et enfermé.
- API : `Spec`, `Run`, `Output`, `Supervisor`, `SupervisorConfig`, `Backoff`, `Store`
  (extraction et vérification), `Manifest`, `Embedded`, `Licenses`, `NetworkIsolation`.
- Dépend de : rien d'interne ; **NE DOIT PAS** importer `net`, `net/http`, `crypto/tls` (AC-7).
- État : cache d'extraction (`<UserCacheDir>/inkwell/<version>/`, lecture seule).
- Concurrence : un processus enfant par `Run` dans son groupe ; `Supervisor` relance avec
  recul.

**`internal/sidecar/fetch`** — téléchargement des archives épinglées, **à la construction
seulement** ; importé par `cmd/sidecar-fetch` seul (AC-6).

**`internal/typst`** — compiler un projet Typst hors ligne.
- API : `Engine`, `Config`, `Project`, `Result`, `Diagnostic`, `CompileError`, `SVG`,
  `Deck`, `Anchor`, `Attachment`, `PageCount`, `DefaultTimestamp`.
- Dépend de : `sidecar` (jamais l'inverse) ; mêmes interdits réseau (AC-7).
- État : cache des paquets ; nombre de compilations simultanées borné par le CPU.

**`internal/preview`** *[cible, `AD-05`]* — l'aperçu et l'export : anti-rebond, annulation des
compilations périmées, pages visibles, export PDF.
- API : `New(Options) *Preview`, `Changed(rel)`, `Serve(conn)`, `Export(ctx, dst)`, port
  `Compiler`.
- Dépend de : `typst`. **NE DOIT PAS** dépendre de `transport`, `ledger`, `blob`.
- Concurrence : une minuterie, une compilation active, la précédente annulée.

**`internal/lsp`** *[cible, `AD-20`]* — tinymist sur stdio relayé par un Stream `lsp`.
- API : `Serve(conn)` ; cadrage JSON-RPC borné (taille d'en-tête et de corps).
- Dépend de : `sidecar`.

**`internal/textview`** *[cible, ADR 0028 D4, tâche PERF-10]* — index de lignes et pages de
64 Kio pour les fichiers trop gros pour Automerge ; Stream `view`.

**`internal/milestone`** *[à décider, `AD-13`]* — projection Git déterministe des versions
nommées.

**`internal/plugin`** *[cible, ADR 0039]* — le gestionnaire d'extensions et l'hôte des greffons.
- API : `Manager` (installer depuis un fichier, activer, désactiver, mettre à jour, retirer, droits par appareil et par projet), `Host` (lance et supervise `inkwell --plugin-host`, appels bornés), `Grant`, `Manifest`, `Package` (zip vérifié, signature, continuité de clé), `SanitizeSVG`.
- Dépend de : `sidecar` (enfermement, superviseur), `store`, `identity` (vérification de signature seulement), `protocol/sig` ; wazero **seulement** dans le sous-paquet `internal/plugin/host` exécuté par le processus enfant (AC-16). **NE DOIT PAS** dépendre de `ledger`, `transport`, `bridge`, `blob`.
- État : `<UserDataDir>/inkwell/plugins/` (paquets vérifiés), droits et épinglages dans les réglages ; cache de compilation wazero (`<UserCacheDir>`).
- Concurrence : un superviseur par hôte (global, et un par projet ouvert avec greffons) ; appels par file bornée ; arrêt §15.9.
- Fuzz : manifeste, paquet, sorties des greffons (propositions, vues, jetons), assainisseur SVG.
- Retirer un greffon global efface son dossier, son magasin privé (`storage`), ses droits, ses
  épinglages et son journal ; un greffon de projet désactivé pour soi garde son magasin par projet
  jusqu'à la fermeture du projet, puis l'efface s'il a été retiré par décision.

**`internal/settings`** *[cible, `AD-21`]* — réglages de l'appareil (`settings.json`, versionné).

**`internal/version`** — `Current() Info` (version, révision, date injectées par
`-ldflags`).

**`cmd/inkwell-relay`** — relais circuit v2 auto-hébergeable (`docs/relay.md`).
**`cmd/sidecar-fetch`** — `mise run sidecar:fetch`.

### 4.3 Module `protocol` (public, N13)

Conforme au §1.6 du protocole. *[en cours, ADR 0034 étape 1, worktree `proto1`]*.

| Paquet | Contenu | Dépendances permises |
|---|---|---|
| `protocol` | `PeerID`, `MaxFrame`, bornes du §16 en constantes | bibliothèque standard |
| `protocol/sig` | validation des clés (§3.2), verdict de signature (§3.3) | `filippo.io/edwards25519` |
| `protocol/trust` | ops de membres, pli (§4.9), attestations, `Sync` | `sig` |
| `protocol/policy` | journal de décisions, pli | `sig`, `trust` |
| `protocol/listing` | annonces, ensemble admis (§4.10), accusés, preuves | `sig`, `trust`, `policy`, `lukechampine.com/blake3` |
| `protocol/blob` | messages du vrac, hachage | `blake3` |
| `protocol/invite` | ticket, messages du salon | `sig` |
| `protocol/sync2` | préambules, trames, HELLO, règles de l'inconnu | — |
| `protocol/session` | machines à états pures (§7.3, §11), sans E/S ni goroutine | `sync2` |
| `protocol/conformance`, `protocol/vectors` | exécuteur de conformité, vecteurs embarqués | — |
| `protocol/libp2p` (sous-module) | liaison libp2p : un protocole par classe, gestionnaire de ressources | le cœur, go-libp2p |

Le paquet TypeScript `@inkwell/protocol` (`frontend/packages/protocol`) porte le profil
CBOR, les schémas du §9, la garde, les crédits du pont et les mêmes vecteurs ; il n'importe
rien de `frontend/src`.

### 4.4 Modules de la webview

| Module | Rôle | Peut importer |
|---|---|---|
| `src/workspace` | racine de composition : `Repo`, adaptateurs, chargement, projet courant | tout sauf `src/ui` |
| `src/worker` *[cible]* | hôte du `Repo` dans le Worker ; pont mince vers le fil d'affichage | `net`, `store`, `@inkwell/protocol` |
| `src/net` | adaptateur réseau, admission, couverture, enveloppe | `@inkwell/protocol`, automerge |
| `src/store` | adaptateur de stockage, projection, synchro externe, arbre, paliers | `net` (types), automerge |
| `src/history` | modèle et analyse d'historique (Worker d'analyse) | `store` (types) |
| `src/editor` | extensions CodeMirror ; éditeurs par type ; recherche ; mise en forme | `store`, `history` (types), `filetypes` |
| `src/filetypes` *[cible, ADR 0027]* | registre : extension → type, icône, éditeur chargé à la demande, syntaxe des commentaires, palier | rien d'autre que ses modules |
| `src/preview`, `src/present` | clients des Streams `preview` et `present` | `i18n` |
| `src/ui` | composants Svelte | tout sauf `net`, `worker` et les Streams |
| `src/i18n`, `src/theme` | chaînes fr/en ; jetons Adwaita | rien |
| `src/extensions` *[cible]* | registre des contributions (§15.2 X-2), hôte TypeScript des extensions intégrées, rendu des vues déclaratives, pont vers l'hôte des greffons | `ui/kit`, `@inkwell/extension-api`, Streams `ext` |
| `src/ext/<nom>` *[cible]* | extensions intégrées : éditeur, tableur, aperçu, historique, recherche, diagrammes, thèmes… | **seulement** `@inkwell/extension-api` et `src/ui/kit` (FE-3) |
| `packages/extension-api` *[cible]* | types et aides générés du WIT, partagés par les extensions intégrées et le kit TypeScript des greffons | rien |

Règles : seul `src/workspace` (puis `src/worker`) crée un `Repo` et ouvre les Streams `net`
et `store` ; `src/ui` ne touche jamais automerge-repo ni un Stream ; aucun `{@html}` sur une
donnée (seulement des constantes, `ui/icons`). Vérifiées par Biome `noRestrictedImports`
(≥ 2.2) dans `fe:check` *[cible, `AD-30`]*.

### 4.5 Règles de dépendance

| # | Règle | Vérification |
|---|---|---|
| AC-1 | aucun paquet `internal/*` n'importe `internal/app` | `arch-check.sh` *[en place]* |
| AC-2 | `trust`, `identity` (et sous-paquets) n'atteignent pas `internal/transport/**`, même transitivement | `arch-check.sh` *[en place]* |
| AC-3 | `internal/transport/**` (hors `p2p`) n'atteint aucun autre paquet `internal/*` | `arch-check.sh` *[en place]* |
| AC-4 | seul `internal/transport/p2p` importe `github.com/libp2p/*` (tests compris) | `arch-check.sh` *[en place]* ; étendu à `protocol/libp2p` (AC-10) |
| AC-5 | seul `internal/invite` importe wormhole-william | `arch-check.sh` *[en place]* |
| AC-6 | seul `cmd/sidecar-fetch` importe `internal/sidecar/fetch` | `arch-check.sh` *[en place]* |
| AC-7 | `internal/sidecar` et `internal/typst` n'importent ni `net`, ni `net/http`, ni `crypto/tls` | `arch-check.sh` *[en place]* |
| AC-8 | aucun paquet Automerge Go dans le graphe complet, tests compris | `arch-check.sh` *[en place]* |
| AC-9 | `protocol/...` n'importe rien de `internal/` ni de `cmd/` ; dépendances limitées au tableau §4.3 | à ajouter (étape 1, ADR 0034) |
| AC-10 | seuls `internal/transport/p2p` et `protocol/libp2p` importent libp2p | à ajouter (étape 1) |
| AC-11 | `internal/ledger` n'atteint ni `app`, `bridge`, `blob`, `project`, `typst`, `sidecar`, `preview`, `invite` | à ajouter avec `AD-02` |
| AC-12 | `internal/preview` n'atteint ni `transport/**`, ni `ledger`, ni `blob` | à ajouter avec `AD-05` |
| AC-13 | `internal/store` n'importe rien d'interne ; `internal/project` n'importe que `store` | à ajouter |
| AC-14 | l'exécutable publié n'est construit ni avec l'étiquette `faultinject` ni avec `server` | à ajouter (job de publication : `go version -m`) (`AD-22`) |
| AC-15 | plus aucun `internal/trust` (alias) après l'étape 2 d'ADR 0034 | à ajouter (`AD-03`) |
| FE-1 | `packages/protocol` n'importe rien de `src/` | Biome *[cible]* |
| FE-2 | règles du §4.4 | Biome *[cible]* |
| FE-3 | `src/ext/**` n'importe que `@inkwell/extension-api` et `src/ui/kit` | Biome *[cible, ADR 0039]* |
| AC-16 | seul `internal/plugin/host` importe wazero ; aucun paquet du noyau fermé (§15.3) n'importe `internal/plugin` | `arch-check.sh` *[cible, ADR 0039]* |

## 5. Concurrence, cycle de vie, arrêt

### 5.1 Règles

- **C-1** Chaque goroutine **DOIT** avoir un propriétaire nommé, un `context.Context`, une
  condition de sortie, et être attendue par le `Close` de son propriétaire
  (`sync.WaitGroup.Go`).
- **C-2** Chaque état mutable partagé a un seul propriétaire (un acteur). Les autres le lisent
  par instantané immuable (`atomic.Pointer`) ou passent par la boîte de l'acteur. Un verrou
  ne protège qu'une lecture courte, jamais une E/S.
- **C-3** Une goroutine partagée (acteur, routeur) **NE DOIT JAMAIS** faire d'écriture
  bloquante vers un pair, la webview ou un sous-processus : elle dépose dans une file bornée
  et continue (protocole §15, §10.7).
- **C-4** Toute file, carte ou mémoire tampon **DOIT** avoir une borne, un comportement à la
  borne et une reprise (table §16 du protocole) ; un plafond partagé a des parts réservées
  (protocole §10.6).
- **C-5** Un rappel exécuté dans un acteur **NE DOIT PAS** réentrer dans la boîte de ce même
  acteur (blocage assuré).
- **C-6** Une E/S disque lente (`fsync`, réseau de fichiers) **NE DOIT PAS** s'exécuter sur
  la goroutine d'un acteur : un travailleur de persistance par magasin, file bornée ; l'acteur
  n'émet un objet signé qu'après l'acquittement durable (`AD-25`).
- **C-7** Chaque sous-processus appartient à un superviseur : groupe de processus propre,
  `Pdeathsig` sous Linux, SIGTERM puis SIGKILL, attente de sortie.
- **C-8** Le temps est injecté ; la logique n'appelle pas `time.Sleep` ; les tests de
  concurrence tournent sous `testing/synctest`.
- **C-9** Chaque paquet qui lance des goroutines vérifie leur fin (`goleak` dans `TestMain`)
  et passe `go test -race`.

### 5.2 Acteurs et propriétaires

| Acteur ou magasin | Goroutines | Possède | Entrées | Sortie |
|---|---|---|---|---|
| `p2p.Node` | internes à libp2p + boucle de liens | hôte, liens, instantané des connexions | connexions | `Close` (D13) |
| session *[cible]* | 1 par session + 1 lecteur par flux | génération, files, curseurs SET, LARGE | flux du pair, dépôts | GOAWAY, fin de connexion, contexte |
| ledger *[cible]* (aujourd'hui `app.Wiring`) | 1 acteur + 1 persistance | ensembles signés, vues, persistance | commandes, objets reçus | `Close` vide la persistance |
| attestations | aucune (magasin concurrent) | attestations gardées, parts par signataire | lecteurs de flux | — |
| `bridge.Router` | lecteur + écrivain par Stream | crédits, époques, notifications | Stream `net`, dépôts | fin du Stream, contexte |
| `bridge.StoreServer` | 1 par voie (≤ W+2) *[cible]* | baux, ordre par voie | Stream `store` | `Cancel`/`Released`, contexte |
| `blob.Service` | pompe + ≤ 4 transferts + ménage | transferts, reprises | références voulues | contexte + attente |
| aperçu | minuterie + 1 compilation | dernière compilation publiée | `Changed` | `Close` tue le groupe de processus |
| superviseur LSP *[cible]* | 1 | processus `tinymist` | — | SIGTERM → SIGKILL |
| `project.Watch` | 1 | hachages des fichiers possédés | fsnotify | contexte |
| `invite.Host` | par requête du salon (≤ 4 par pair) | fenêtre, tickets, demandes | salon | expiration, contexte |
| hôte des greffons *[cible]* | processus enfant ; 1 superviseur par hôte | instances wazero, files d'appels | appels bornés venus de Go | SIGTERM puis SIGKILL après `app.stopping` (§15.9) |

### 5.3 Démarrage

Ordre normatif (chaque étape échoue fermée et libère les précédentes par `errors.Join`) :

1. instance unique pour le dossier de configuration ; verrou `device.lock` *[cible, `AD-12`]* ;
2. `OpenHome` (0700, `os.Root`) ; `identity.LoadOrCreate` (clé validée au rechargement) ;
3. `OpenDocs`, ouverture du projet ; verrou `.inkwell/lock` *[cible]* ; migration de format si
   nécessaire (§6.4) ; sauvegarde par liens durs (10 gardées) ;
4. ledger : reprise d'une bascule interrompue (décision D6), rechargement vérifié des
   ensembles signés, pli ;
5. nœud `p2p` avec le filtre `ledger.Admit` ; sessions ;
6. service des blobs ; aperçu ; LSP *[cible]* ;
7. application Wails, Streams, services, fenêtre.
8. *[cible]* registre des extensions construit depuis les manifestes (≤ 10 ms, aucun code exécuté) avant la peinture ; extensions activées ensuite, à leur premier événement ; `app.ready` après que le dernier projet est modifiable (§15.9).

Budget : fenêtre peinte ≤ 1 s ; dernier projet modifiable ≤ 1,5 s (§2). Le travail lent
(ménage, vérification de fond) part après la fenêtre.

### 5.4 Arrêt ordonné

Déclenché par `OnShutdown` de Wails ou par un signal. Ordre normatif :

1. les services refusent toute nouvelle commande ;
2. annulation du contexte racine ;
3. réseau : GOAWAY NO_ERROR à chaque session ; arrêt des écoutes ; arrêt des services de
   numérotation de libp2p **avant** le swarm (D13) ; attente des sessions (≤ 2 s) ;
4. Streams fermés ; service des blobs attendu ; groupes de processus des sidecars tués ;
5. ledger : file de persistance vidée (`fsync`) ;
6. `store` : dernier `fsync` groupé ; projet : verrou rendu, `os.Root` fermés.
7. *[cible]* `app.stopping` aux greffons globaux abonnés, 1 s au total, puis arrêt des hôtes de greffons (§15.9) : **après** que les données sont durables.

Durée totale bornée à 5 s ; au-delà, chaque boucle restante est nommée dans le journal et le
processus sort. La sûreté des données **NE DOIT PAS** dépendre d'un arrêt propre : elle
repose sur I1–I4 (§10).

### 5.5 La webview

- Fil d'affichage : CodeMirror, Svelte, rendu ; aucune tâche > 50 ms (ADR 0028).
- Worker Automerge *[cible]* : seul propriétaire du `Repo` et de l'état Automerge ; reçoit les
  Streams `net` et `store` du fil d'affichage par transfert d'`ArrayBuffer` sans copie ;
  recyclé (persister, terminer, relancer, rouvrir les documents visibles) au-delà de son
  budget mémoire ; deux permis bornent les allocations vivantes (ADR 0031).
- Workers d'analyse : historique, recherche, lecture CSV ; jetables, annulables.
- Rechargement de la webview (plantage du processus web, mise à jour) : Go le voit à la
  fermeture des Streams ; à la reconnexion, chaque Stream repart à une nouvelle époque (règle
  T) et le `Repo` se recharge depuis Go. Ce qui n'était pas acquitté est perdu (≤ 200 ms, I1).

## 6. Données

### 6.1 Une source de vérité par chose

| Chose | Vérité | Propriétaire en mémoire | Persistance | Vues dérivées (jamais écrites ailleurs) |
|---|---|---|---|---|
| Texte d'un fichier texte, ses commentaires, ses verrous (ADR 0022) | un document Automerge par fichier (`{text, meta}`) | `Repo` (Worker) | `.inkwell/repo/` par le Stream `store` | fichier projeté sur le disque, aperçu, recherche, sommaire |
| Arbre du projet, références des blobs, paquets `@preview`, demandes de suppression | document racine Automerge (index, `schema: 1`, ADR 0023/0030) | `Repo` | idem | arborescence matérialisée (fonction pure TS, vecteurs partagés avec Go) |
| Versions nommées | entrée Automerge (nom, note, auteur, date, têtes par document) | `Repo` | idem | projection Git *[à décider, `AD-13`]* ; PDF daté |
| Membres, rôles, suspensions, groupes d'appareils | ops signées (ADR 0017, 0021) | ledger | `.inkwell/trust/<hash>` | liste pliée, filtre de connexion, kind `Roles` |
| Décisions d'admin (mode de suppression, fichier principal, corbeille sur accord, purge, quotas, arbitrage `equiv/`) | journal de décisions signé (ADR 0030) | ledger | `.inkwell/sets/decisions/` *[cible, `AD-11`]* | état plié (kind `Policy`) |
| Annonces de documents, accusés finals, preuves d'équivoque | ensembles signés (ADR 0031) | ledger | `.inkwell/sets/<ensemble>/` *[cible]* | ensemble admis (kind `Listed`) |
| Attestations de têtes | objets signés, le dernier gagne par (document, signataire) | magasin d'attestations | celles de l'appareil, avant diffusion (protocole §14.2) | couverture dans la webview |
| Octets d'un binaire | magasin BLAKE3 (ADR 0006, 0023) | `blob.Store` | `.inkwell/blobs/` + arbre Bao | copie lisible sous son chemin (fichier possédé) |
| Clé de l'appareil | `device.key` | `identity.Device` | `<UserConfigDir>/inkwell/device.key` (0600) | PeerID, empreinte |
| Réglages de l'appareil | fichier de réglages *[cible, `AD-21`]* | `app` | `<UserConfigDir>/inkwell/settings.json` | interface |
| Rendus partagés (diagrammes) | fichiers ordinaires du projet : nœuds `blob` `.renders/<clé>.svg` et document `.renders/index.json` *[cible, `AD-70`]* | `Repo`, `blob.Store` | comme tout fichier | aperçu, PDF |
| Modèles de projet | bibliothèque locale de l'appareil *[cible, `AD-78`]* | `app` | `<UserDataDir>/inkwell/templates/` | galerie, vignettes (cache) |
| Politique IA | par appareil ; par projet après PD-1 (décisions `ai/`) *[cible, `AD-82`]* | `settings` ; ledger | `settings.json` ; journal signé | en-tête « IA : … », « Qui voit quoi » |
| Greffons de projet | décisions `plugin/<id>` du journal signé *[cible, `AD-69`]* | ledger | `.inkwell/sets/decisions/` ; paquet en blob | liste « Extensions du projet » |
| Extensions installées, droits, épinglages, mode développeur | réglages de l'appareil + dossier des greffons *[cible, ADR 0039]* | `internal/plugin` | `<UserDataDir>/inkwell/plugins/`, `settings.json` | gestionnaire d'extensions |
| Clé de stockage distant | par projet et par appareil *[cible, `AD-67`]* | `internal/plugin` (côté Go, jamais l'hôte) | `<UserConfigDir>/inkwell/` (0600) | — |
| Présence, curseurs, sélection | aucune : éphémère | `Repo` (éphémère) | **jamais** persistée | — |
| État de synchronisation Automerge | aucune (pairs éphémères, ADR 0031) | — | **jamais** ; les anciens sont effacés | — |

Règle : une fonctionnalité **NE DOIT PAS** créer une nouvelle vérité. Elle lit l'une de
celles-ci, ou en dérive une vue recalculable. Toute nouvelle ligne de ce tableau exige une ADR.

### 6.2 Disposition sur le disque

```
<Documents>/Inkwell/<Titre>/              dossier visible du projet
  main.typ, chapitres/…, figures/…        projection lisible (fichiers possédés)
  .typstdeps/                             paquets @preview fournis par le projet
  .renders/                               rendus de diagrammes, projetés depuis l'index [cible, AD-70]
  .inkwell/                               0700, os.Root
    project.json                          id, titre, URL racine, fichiers possédés + SHA-256, layout [cible]
    lock                                  verrou du projet [cible, AD-12]
    repo/<docId>/…                        incréments et instantanés automerge-repo (IKV1)
    trust/<hash>                          ops de membres (puis sets/members/ [cible, AD-11])
    sets/<ensemble>/<hash>                décisions, annonces, accusés, preuves [cible]
    blobs/…                               magasin BLAKE3, arbres Bao, cartes de reprise
    staging/<root>/                       modifications indivisibles en attente (protocole §14.7)
    git/                                  projection Git des versions [à décider, AD-13]
    backups/<horodatage>/                 liens durs, 10 gardés
    quarantine/                           octets abîmés, jamais effacés automatiquement
<UserConfigDir>/inkwell/                  0700 : device.key, device.lock [cible], settings.json [cible]
<UserDataDir>/inkwell/plugins/<id>/<v>/    paquets de greffons vérifiés [cible, ADR 0039]
<UserCacheDir>/inkwell/<version>/         sidecars extraits (lecture seule), caches régénérables (24 h)
$XDG_STATE_HOME/inkwell/logs/             journaux [cible, AD-16]
```

### 6.3 Formats et atomicité

- **D-1** Toute écriture d'un fichier d'inkwell **DOIT** passer par `store.WriteAtomic` (ou
  `WriteAtomicFrom`) : temporaire `O_EXCL` dans le même dossier, écriture, `Sync`, `Close`,
  `Rename`, `Sync` du dossier.
- **D-2** Un ensemble de fichiers qui change ensemble se prépare dans `<nom>.next/` et
  s'installe par renommages ; le point de validation est le renommage d'installation ; une
  bascule interrompue est reprise à l'ouverture (décision D6 de l'audit).
- **D-3** Les valeurs du magasin portent l'en-tête `IKV1 | SHA-256(données)` ; un écart est
  `ErrCorrupt` : quarantaine, puis dernière sauvegarde saine, puis les pairs.
- **D-4** Les objets signés sont stockés sous leur hachage, dans leur encodage canonique, et
  **revérifiés au chargement** (décodage strict, clés, signature) ; un échec part en
  quarantaine (`.bad`), jamais effacé (protocole §14.4).
- **D-5** « Durable avant diffusé » : un objet signé localement est persisté avant sa
  première émission (protocole §14.2).
- **D-6** « Vérifié avant stocké » : rien de ce qu'envoie un pair n'est écrit avant l'étape 8
  du §12 du protocole.
- **D-7** Les blobs sont écrits après vérification Bao complète, puis renommés (I3).
- **D-8** Les `fsync` des petits incréments sont groupés (≤ 500 ms ou 4 Mio par projet) ;
  instantanés, `project.json`, listes signées, clés et fichiers possédés ont un `fsync` complet
  avant acquittement (ADR 0029 I2) *[cible]*.
- **D-9** Disque plein ou erreur d'E/S : aucune écriture partielle ; l'état reste en mémoire
  et chez les pairs ; reprise avec recul (ADR 0029 I5).

### 6.4 Versions des données stockées et migration

*[cible, `AD-10`, ADR 0035 à rédiger]*

- **V-1** `project.json` **DOIT** porter un entier `layout` (version de la disposition du
  dossier `.inkwell/`). Absent = 1 (disposition actuelle).
- **V-2** Inkwell lit les dispositions qu'il connaît. Face à une disposition plus récente, il
  ouvre le projet **en lecture seule** avec le message « Ce projet a été modifié par une
  version plus récente d'Inkwell » ; il n'écrit rien.
- **V-3** Une migration **DOIT** : faire d'abord une sauvegarde par liens durs ; écrire la
  nouvelle disposition dans `.inkwell.next/` ; installer par renommage ; être idempotente et
  reprise si interrompue ; être testée par injection de plantage à chaque étape.
- **V-4** Les objets signés restent lisibles pour toujours (protocole §1.4.4) ; une nouvelle
  version d'objet s'ajoute, l'ancienne ne change jamais.
- **V-5** Le schéma des documents Automerge (`schema` de l'index, forme de `meta`) n'évolue
  que par ajout : un champ inconnu est ignoré et conservé ; un champ n'est jamais réutilisé
  avec un autre sens. Les majeures N et N−1 coexistent 12 mois comme le protocole.
- **V-6** Le format est figé à la 1.0.0 (ADR 0025) ; jusque-là, toute rupture passe par V-3.

### 6.5 Bornes de stockage

Valeur ≤ 32 Mio ; page du Stream `store` ≤ 1 Mio ; clé ≤ 4 composants de 128 octets
(`^[A-Za-z0-9._-]{1,128}$`) ; binaire ≤ 100 Mo, projet ≤ 2 Go puis confirmation ;
modifications indivisibles en attente ≤ 1 Gio (protocole §16) ; quota disque cumulé par
personne qui écrit (N10) *[valeur à décider, `AD-24`]* ; téléchargements de blobs suspendus
sous 1 Gio libre ou 5 % du disque (ADR 0029 I5).

**Racines du ménage des blobs** (`internal/blob`, `GC(referenced …)`) : un blob est gardé s'il est
référencé par l'index admis (fichiers, paquets, rendus `.renders/`, ADR 0023), **ou** par une
décision `plugin/<id>` debout (paquet d'un greffon de projet, §15.10, après PD-1) ; sinon il part
après 30 jours. La purge d'un nœud texte ne retire pas les rendus qui en viennent : « Nettoyer les
rendus » s'en charge (§15.14).

### 6.6 Projet multi-fichiers : documents, document actif, fichier principal, types

*[cible ; aujourd'hui un seul document]* Le code ne connaît qu'un document : la racine est
le texte de `main.typ` (`workspace.svelte.ts:30,42`), `workspace.open()` ouvre un **projet**
(`:247`), la projection et la synchronisation externe visent `main.typ` (`:270–310`), et un
clic dans la liste des fichiers ne fait rien (`App.svelte:197` jette le chemin). Cette
section définit le modèle qui manque. Décisions et preuves : `AD-46` à `AD-56`.

**Modèle de documents** (ADR 0023, ADR 0030 §3, protocole §4.10 ; `AD-46`)

- **MF-1** Un document Automerge par fichier texte ; un document d'**index** par projet,
  distinct : l'index `G` dont l'identifiant est dérivé du `ProjectID` et de `root_url`
  (ADR 0030 §3). La racine (`root_url`) reste le texte du `main.typ` de la genèse. Le schéma
  « index = racine » de l'ADR 0023 est remplacé par l'ADR 0030 (acceptée).
- **MF-2** Un fichier est un **nœud** de l'index (`id` stable sur 128 bits, `type`
  `text | blob | dir`, `name`, `parent`, `url` ou `hash`). Tout ce qui désigne un fichier dans
  l'application (éditeur, onglet, présence, commentaire, aperçu, recherche) le désigne par
  son `id` de nœud, **jamais par son chemin** : le chemin est une vue, recalculée par
  `materialize(index, décisions)`.
- **MF-3** `materialize` est une fonction pure en TypeScript, avec vecteurs partagés avec Go
  (ADR 0023) ; son résultat part vers Go par l'opération `Tree` du Stream `store` (codec déjà
  présent : `store/tree.ts`, `bridge/tree.go`) ; le drapeau « principal » y est porté.
- **MF-4** Créer un fichier texte = créer le document, l'annoncer (op signée, durable avant
  diffusée, ADR 0031) **puis** poser son nœud dans l'index. Un nœud qui désigne un document
  pas encore admis s'affiche « en attente », jamais vide.

**Session de travail** (`AD-47`)

| Notion | Définition | Propriétaire | Persistance |
|---|---|---|---|
| documents chargés | poignées `DocHandle` en mémoire : actif, récents, en cours de projection ou de réception | `src/workspace` (puis le Worker) | aucune |
| documents ouverts | ceux que la personne a ouverts, dans l'ordre (liste de récents ; onglets si `AD-47` les retient) | `src/workspace` | réglages de l'appareil, état de reprise (`AD-21`) |
| document actif | un seul : celui que montre la zone d'édition | `src/workspace` | idem |
| fichier principal | entrée de compilation du projet | décision d'admin (ADR 0030) ; repli : nœud principal de la genèse | journal de décisions |

- **MF-5** Ouvrir un fichier = `open(nodeId)` : retrouve le nœud, charge la poignée (ou la
  vue de son type, MF-14), la rend active ; aucune réouverture du projet. Ouvrir un projet
  reste `openProject(rootUrl)`.
- **MF-6** Documents chargés bornés : le document actif, les ouverts visibles, plus au plus
  64 autres en LRU ; un document invisible depuis 60 s est déchargé (ADR 0028 D9). La
  réception et la projection chargent un document le temps du travail, puis le rendent.
- **MF-7** La présence porte le nœud actif de chaque personne (sur l'index) et le curseur
  (sur le document) : la liste des fichiers montre qui est où (U16).

**Fichier principal et fichier édité** (`AD-48`)

- **MF-8** L'aperçu et l'export compilent **toujours** le fichier principal, quel que soit le
  document actif (`typst compile --root <projet> <principal>`) ; toute projection d'un fichier
  du projet relance la compilation (`Changed(rel)`). Go connaît le principal par le drapeau du
  `Tree`, pas par une option fixée au démarrage (aujourd'hui `preview.go:85,179`).
- **MF-9** Si le document actif n'est pas atteint par le principal (absent des dépendances
  rendues par la compilation), l'aperçu le dit en une ligne et propose « Aperçu de ce
  fichier » : compilation temporaire avec ce fichier pour entrée, jamais un changement du
  principal (qui reste une décision d'admin).
- **MF-10** Le saut source ↔ aperçu traverse les fichiers : une ancre désigne (nœud, position)
  et ouvre le bon document.

**Enregistrement par fichier** (ADR 0020, ADR 0029 ; `AD-49`)

- **MF-11** La persistance Automerge (Stream `store`, I1) vaut pour chaque document, ouvert
  ou non ; elle ne dépend pas du mode d'enregistrement.
- **MF-12** La projection sur le disque est une vue de **tous** les documents texte admis, pas
  seulement de l'actif : un document changé par un pair est reprojeté même s'il n'est pas
  ouvert. Une projection et une synchronisation externe (B1) par nœud, créées à la demande,
  détruites avec le déchargement.
- **MF-13** Le mode (automatique ou manuel) est un réglage de l'appareil, le même pour tous les
  fichiers ; l'état « non enregistré » est par fichier (pastille dans la liste) ; Ctrl+S
  enregistre tous les fichiers modifiés du projet ; fermer ne pose aucune question bloquante
  (SAV-03).

**Renommer, supprimer un fichier ouvert ailleurs** (ADR 0023 ; `AD-50`)

| Événement venu d'un pair | Effet chez la personne qui l'a ouvert |
|---|---|
| renommage, déplacement | l'éditeur suit le nœud ; le sous-titre change ; la projection déplace le fichier possédé (`ApplyTree`, renommage atomique) ; frappe jamais interrompue |
| renommages concurrents | registre Automerge, plus grand opId ; l'autre nom reste dans l'historique |
| mise à la corbeille | bandeau « Paul a mis ce fichier à la corbeille. **Restaurer** » ; la frappe continue (le document vit) |
| purge | lecture seule, bandeau, « Enregistrer une copie » ; rien n'est perdu de ce qui était tapé |
| fichier principal mis à la corbeille | impossible (FM-18) |

**Types de fichiers** (ADR 0027 à rédiger, N20, U25, U26 ; `AD-51` à `AD-55`)

- **MF-14** Un seul registre décrit chaque type ; sa partie données
  (`filetypes.json` : extensions, noms de fichiers, type MIME, sorte `text | table | blob`,
  vue, mode d'édition, langage de coloration, aperçu, icône, éditable, palier de taille) est
  **embarquée par Go et importée par TypeScript** ; la partie code (éditeur, vue) est un module
  TypeScript chargé à la demande. Aucun `switch` sur une extension ailleurs (AC à ajouter
  avec ADR 0027).

| Type | Extensions (MIME) | Vérité | Vue, édition | Aperçu, compilation | Éditable |
|---|---|---|---|---|---|
| Typst | `.typ` (`text/x-typst`) | texte Automerge | CodeMirror, coloration Typst, barre de mise en forme, LSP (`AD-20`) | principal ou « Aperçu de ce fichier » | oui |
| Markdown | `.md`, `.markdown` (`text/markdown`, RFC 7763) | texte Automerge | CodeMirror, coloration Markdown, barre de mise en forme | par Typst et `cmarker` (mise en page U18) | oui |
| Tableau | `.csv` (`text/csv`, RFC 4180), `.tsv`, `.tab` (`text/tab-separated-values`), `.psv` | texte Automerge (ADR 0026, `AD-52`) | grille virtualisée ⇄ vue texte | lu par Typst (`csv()`) depuis la projection | oui |
| Texte | `.txt` et tout texte non reconnu (`text/plain`) | texte Automerge | CodeMirror, sans coloration | — | oui |
| Code et configuration | liste fermée (`AD-54`) | texte Automerge | CodeMirror, coloration chargée à la demande, sans complétion | — | oui (« visualisation agréable », pas un éditeur de développement) |
| Image | `.png`, `.jpg`, `.gif`, `.webp`, `.svg` | blob BLAKE3 | visionneuse `<img>` | utilisée par Typst | non (remplacer) |
| PDF | `.pdf` | blob | visionneuse PDF.js (`AD-39`) | joint ou inclus par Typst | non |
| Autre binaire, inconnu | — | blob | carte (nom, taille, qui, quand), « Afficher dans le dossier » ; jamais lancé | — | non |

- **MF-15** Classement d'un fichier nouveau (ajouté dans l'application, reçu du disque par la
  surveillance, ou importé) : l'extension connue décide ; sinon reniflage des 8 premiers Kio
  (octet nul ou octet de contrôle « binaire » au sens du WHATWG MIME Sniffing → blob ; UTF-8
  valide, ou encodage détecté par Go, → texte brut). Le même reniflage en Go et en TS, avec
  vecteurs partagés (`AD-55`).
- **MF-16** Paliers de taille **par type** (§12.2 ; `AD-53`) vérifiés **avant** de créer ou
  de modifier un document (porte unique `store/limits.ts`) : textes et tableaux suivent les
  paliers en caractères ; un tableau en lecture seule reste une grille virtualisée sur un
  index des débuts de ligne (jamais un objet par cellule) ; une image n'est décodée que si son
  en-tête annonce au plus 100 mégapixels.
- **MF-17** Ajouter un binaire = `PutBlob` par le Stream `store` → `blob.Store` (vérifié,
  ≤ 100 Mo) → copie lisible sous son chemin → nœud `blob` (`hash`, `size`) dans l'index ;
  remplacer garde le nœud et change le `hash` ; une modification du fichier sur le disque
  propose un remplacement (comme pour un texte, B1).
- **MF-18** Grammaires de coloration : modules découpés par Vite, embarqués dans l'exécutable
  avec le reste de `frontend/dist`, chargés par `import()` au premier fichier du langage ;
  **aucun téléchargement à l'exécution** (même règle que Typst, NG6) ; la CSP (`script-src
  'self'`) les sert depuis l'origine de l'application (`AD-54`).

**Bornes et propriétaires de la session** (`AD-46`, `AD-77`)

- **MF-19** Plafonds globaux, quel que soit le nombre de documents ouverts : poignées chargées
  ≤ 96 (actif + ouverts visibles + 64 en LRU + travaux) ; caractères chargés cumulés ≤ 8 Mi
  (le palier « Grand » d'un seul document est 2 Mi) ; travaux de projection simultanés ≤ 4 ; au
  plafond, on décharge d'abord ce qui n'est ni visible, ni sale, ni en travail.
- **MF-20** Un **état par nœud**, possédé par `src/workspace` (puis le Worker) : poignée,
  projection, synchronisation externe, état « non enregistré », annulation et sélection locales.
  Décharger une poignée **ne détruit pas** cet état : la projection et le registre des
  modifications externes vivent dans une table par nœud indépendante de la visibilité.
- **MF-21** File de projection indépendante de la visibilité : toute modification admise d'un
  document texte (local ou d'un pair) met son nœud dans la file ; un travailleur charge la poignée
  si besoin, écrit, puis la rend. En mode manuel, un nœud sale **n'est jamais déchargé** (il compte
  dans les plafonds) ; à la fermeture, il reste sale dans Automerge, sans question (SAV-03).
- **MF-22** Modifications externes d'un fichier déchargé : la surveillance de Go les inscrit au
  registre par nœud ; la fusion B1 se fait au prochain chargement ou par la file, jamais perdue.
- **MF-23** Ouvertures concurrentes : chaque `open(nodeId)` porte un numéro croissant ; seule la
  plus récente peut devenir active (« dernier clic gagne ») ; une réponse plus ancienne est
  ignorée. Renommer un nœud pendant son chargement ne change rien (l'identité est le nœud).
- **MF-24** Annulation et sélection : par nœud et par personne, jamais partagées ; gardées tant
  que l'état du nœud vit (MF-20).
- **MF-25** Tableaux (ADR 0026, `AD-52`) : à chaque fusion de modifications distantes, l'adaptateur
  du tableur compare, **au niveau des opérations**, les lignes que la personne a touchées depuis la
  dernière synchronisation aux plages que les modifications concurrentes ont supprimées ou
  déplacées ; une intersection est un **conflit structurel**, même si le CSV reste bien formé :
  carte attribuée « Paul a supprimé la ligne que vous modifiiez » avec « Restaurer ma ligne »
  (proposition) ou « Garder la suppression » ; le texte orphelin est retiré de la ligne voisine
  dans la même proposition. Oracle : test de propriété à graine contre un modèle de référence à
  identités de lignes (chaque cellule garde sa ligne d'origine ou est signalée), remplaçant
  l'assertion actuelle `record(r).length > 0` de `collab.test.ts`.

**Chemin minimal pour corriger G-0** (clic sans effet, N19 ; `AD-77`)

Le modèle complet exige l'index `G` et les annonces de documents (ADR 0031, étape 8 d'ADR 0034).
On ne l'attend pas pour rendre le clic utile :

| Lot | Contenu | Dépend de | Preuve |
|---|---|---|---|
| **G-0a** (immédiat, avant le Worker) | `onOpen(path)` ouvre vraiment : `main.typ` → éditeur ; tout autre fichier texte → **lecture seule** depuis la projection (Go la sert déjà) avec le bandeau « Ce fichier n'est pas encore partagé : il ne se modifie pas encore à plusieurs » ; tableau → grille en lecture seule ; image → visionneuse ; autre → carte ; sous-titre et présence suivent ; suppression de `?file=` | rien | e2e `files.spec.ts` FM-OPEN-1 (clic → contenu du bon fichier), FM-OPEN-2 (sous-titre), FM-OPEN-3 (txt, csv, md, typ distincts, sans `?file=`) |
| **G-0b** | MF-1, MF-2, MF-5, MF-12 : index `G` matérialisé, un document par fichier, `open(nodeId)`, projection de tous les textes | ADR 0031 (annonces), étape 8 d'ADR 0034 | FM-OPEN-4 (ouvrir A, B, A), FM-OPEN-5 (renommer A pendant son chargement), FM-OPEN-6 (modification de B reçue, B fermé, écrite sur le disque), FM-OPEN-7 (mode manuel, éviction refusée), FM-OPEN-8 (redémarrage : ouverts et actif retrouvés) |
| **G-0c** | MF-6, MF-7, MF-19 à MF-24, onglets si `AD-47` | Worker (ADR 0028 D2) | `PERF-*` avec 10 000 documents |

### 6.7 Stockage distant par extension (N22)

*[cible, ADR 0039 ; `AD-67`]* Une extension peut fournir un **dorsal de stockage** (nuage,
autre système de fichiers, disque réseau). En v1, « remplacer le stockage » est **réinterprété**
en *sauvegarde distante et restauration pour la même personne* (à confirmer, `AD-67`) : la copie
locale reste la copie de travail. L'**intégrité** reste fermée (K-6) : le cœur l'impose au-dessus
de tout dorsal, traité comme **byzantin**, comme un dépôt restic, un distant Git ou un appareil
Syncthing « non fiable ».

**Ce qui ne change pas.**

- **ST-1** La copie locale (`.inkwell/` et la projection) est **la** copie de travail, toujours
  autoritative pour l'édition, hors ligne compris. Typst, la projection Git (`AD-13`), la
  surveillance des modifications externes et la recherche (`AD-42`) travaillent sur ce dossier
  local matérialisé, jamais sur le dorsal.
- **ST-2** Aucune frappe n'attend le dorsal ; aucun arrêt ne l'attend : les données sont
  durables localement avant tout envoi (I1, I2).

**Contrat du dorsal** (interface `storage-backend`), inspiré du dorsal de restic (`Save`,
`Load`, `Stat`, `List`, `Remove` sur des noms opaques), des services d'OpenDAL (opérations
uniformes, drapeaux de capacité) et d'`io/fs` :

| Opération | Sens | Exigence sur le dorsal |
|---|---|---|
| `capabilities()` | taille maximale d'objet, lecture par plage, liste paginée | — |
| `put(nom, octets)` | écrit un objet ; idempotent | aucune (le cœur revérifie tout) |
| `get(nom, début, longueur)` | lit un objet ou une plage | aucune |
| `list(préfixe, curseur)` | énumère, par pages ≤ 1 000 | peut être incomplète ou mentir |
| `delete(nom)` | appelé seulement par le ménage du cœur | peut ne pas effacer |

**Clés** (ST-3) : une **clé de stockage par projet et par personne**, aléatoire (256 bits),
gardée par Go à côté de `device.key` (0600), transportée d'un appareil de la personne à un autre
**seulement** par la phrase de récupération (ST-8), jamais par le dorsal ni par un pair. Clés
dérivées par HKDF-SHA-256 à domaines séparés : `inkwell/backup/obj-name/v1` (nommage),
`inkwell/backup/obj-aead/v1` (chiffrement), `inkwell/backup/head-aead/v1`. Une seule clé par
personne rend la déduplication réelle entre ses appareils.

**Objets** (ST-4) : `nom = hex(BLAKE3 à clé(clé de nommage, clair))[:32]` ; contenu =
XChaCha20-Poly1305, nonce aléatoire de 24 octets en tête, **AAD** = `"inkwell/backup/obj/v1" ‖
ProjectID ‖ nom` (un objet ne peut pas être déplacé sous un autre nom ou un autre projet) ;
grands objets découpés en morceaux de 1 Mio, chaque morceau un objet. L'extension ne voit que des
octets chiffrés et des noms opaques ; la clé ne quitte jamais Go (S-3).

**Graphe et ordre d'écriture** (ST-5) : objets (valeurs du magasin, blobs, ensembles signés) →
**manifeste** (liste des noms d'objets d'un instantané, chiffré, AAD de manifeste) → **tête**
`heads/<PeerID>/<compteur>` = `{ProjectID, PeerID, compteur, date, nom du manifeste,
compteur précédent}` signée par l'appareil (domaine `inkwell/backup-head/v1`) puis chiffrée.
Ordre : objets, puis manifeste, puis tête, chacun écrit **après** la confirmation du précédent ;
la tête précédente n'est retirée qu'après la vérification de lecture de la nouvelle (on garde
toujours deux têtes). Une tête n'est jamais réécrite (nom neuf à chaque compteur) : aucune
écriture déchirée ni réordonnancement ne peut produire une tête qui désigne un manifeste absent
sans être vu (lecture de vérification).

**Lecture** (ST-6) : AEAD, puis BLAKE3 du clair = nom attendu, puis revérification complète des
objets signés (D-4) ; une tête dont le manifeste ou un objet manque est **incomplète** : refusée,
la précédente est essayée. Liste incomplète ou mensongère : on ne voit que des têtes valides,
jamais un mélange.

**Fraîcheur, honnêtement** (ST-7) :

- Un appareil **déjà synchronisé** refuse une tête plus ancienne que son dernier compteur vu
  (retour arrière détecté).
- Un appareil **neuf** qui restaure ne peut **pas** savoir seul si la tête la plus récente que
  le dorsal lui montre est la plus récente qui existe (même limite que TUF sans ancre de
  fraîcheur). Ancres prévues : (a) la phrase de récupération porte le compteur et la date au moment
  où elle est montrée, et l'écran de restauration dit « Sauvegarde du 3 mars, 14 h 02 ; la phrase
  date du 12 mars : des modifications plus récentes ont pu ne pas être sauvegardées » ; (b) l'appareil
  restauré est un **nouvel appareil** de la personne, admis au projet comme tout appareil (groupe
  d'appareils, ADR 0017) : dès qu'un membre est joignable, la synchronisation normale apporte ce que
  le dorsal a caché.
- **Résiduel accepté**, écrit à l'écran : sans membre joignable, un dorsal hostile peut faire
  repartir un appareil neuf d'un état ancien ; il ne peut ni modifier, ni forger, ni faire lire.
- Restaurer n'impose jamais un état d'autorité ancien : les ensembles signés se **plient**
  (protocole §14.5) ; une op ancienne ne retire rien de ce que l'appareil apprendra ensuite ; une
  purge reçue plus tard s'applique normalement.
- **Résiduel** : tailles et nombre d'objets visibles du dorsal (pas de rembourrage en v1).

**Phrase de récupération** (ST-8) : 24 mots (BIP-39, liste anglaise et française), encodant la
clé de stockage, le `ProjectID`, le dernier compteur et sa date, avec somme de contrôle ; montrée
une fois à l'activation de la sauvegarde, puis sur demande (geste, mot de passe de session du
système si disponible).

**Échecs et latence.** Envois et lectures passent par une file bornée hors du chemin d'édition,
délai de 30 s par appel, recul exponentiel, débit plafonné ; hors ligne, la file s'allonge sur
disque (bornée par le quota du projet) et repart au retour. L'état « Sauvegarde distante : à jour
au … / en retard de N / injoignable depuis … » est montré.

**Ménage** : le cœur garde les objets des deux dernières têtes de chaque appareil de la personne
et de toutes les têtes de moins de 30 jours ; le reste est supprimé par `delete` (le dorsal peut ne
pas obéir : coût de stockage, pas d'intégrité).

**Miroir lisible** (point distinct, `mirrors`) : copie à sens unique de la projection lisible vers
un dossier local choisi (par exemple monté par le client d'un nuage), sans `.inkwell/`,
explicitement non chiffrée et signalée comme telle.

### 6.8 Modèles de projet (N27)

*[cible ; `AD-78`]* **Aucun modèle n'est livré** : les modèles sont créés et gérés par les
personnes. Un modèle **est un projet**, marqué comme modèle, décrit par la section standard
`[template]` de `typst.toml` (format des modèles de Typst Universe : `path`, `entrypoint`,
`thumbnail`), plus un nom et une description obligatoires.

- **TM-1** Bibliothèque locale : `<UserDataDir>/inkwell/templates/<id>/` (un dossier de projet
  complet, `.typstdeps/` compris) ; l'index des modèles est une vue de ce dossier.
- **TM-2** Gestes : « Enregistrer comme modèle » (depuis un projet : copie de la projection et de
  `.typstdeps/`, **sans** `.inkwell/`, donc sans historique ni membres) ; modifier (ouvrir le
  modèle comme projet local) ; dupliquer ; supprimer (corbeille de 30 jours) ; importer et
  exporter (archive du `AD-38`) ; partager (envoyer l'archive, ou ouvrir le modèle comme projet et
  inviter).
- **TM-3** « Nouveau projet depuis un modèle » = copie locale, champs à remplir (titre, auteur)
  substitués dans les fichiers déclarés ; **jamais** `typst init` (qui télécharge depuis
  Universe, contraire à NG6).
- **TM-4** Vignette : compilée **localement** (première page, SVG ou PNG) par le moteur enfermé,
  mise en cache ; recalculée quand le modèle change.
- **TM-5** Un même modèle peut servir plusieurs sorties (document officiel, présentation) : le
  modèle déclare plusieurs points d'entrée dans `typst.toml` (`entrypoint` par variante, extension
  `[tool.inkwell] variants`) ; la galerie montre une vignette par variante.
- **TM-6** Modèles Markdown : un projet Markdown dont la mise en forme est un gabarit **Typst**
  appliqué par `cmarker` (même chaîne que l'aperçu et le PDF, déterministe). Le CSS ne sert qu'à
  l'export HTML (Typst ≥ 0.15) ; un « type de document » défini **par CSS seul** pour le PDF exigerait
  un second moteur de mise en page (navigateur) : écarté en v1 (`AD-78`, à confirmer).
- **TM-7** Les modèles ne sont pas des greffons et n'exécutent rien hors de Typst (et de ses
  greffons Typst purs, X-8).

## 7. Pont Go ↔ webview

### 7.1 Principe

Le pont relie deux moitiés d'**un même exécutable** : pas de port, pas de pair distant, pas
de négociation de version au-delà d'un octet (protocole, annexe A). Il transporte ; il ne
comprend pas.

| Canal | Sens | Usage | Bornes |
|---|---|---|---|
| Stream `net` | les deux | messages de dépôt, présence, attestations, rôles, décisions, crédits (19 kinds, annexe A.1) | enveloppe ≤ 64 Mio ; pair ≤ 64 o ; crédit par (pair, époque, voie) |
| Stream `store` | requêtes webview → Go | stockage d'automerge-repo, projection, arbre, blobs (ADR 0020) | valeur ≤ 32 Mio ; page ≤ 1 Mio ; 1026 voies *[v2]* |
| Stream `preview` | requêtes → pages | pages SVG visibles, nombre de pages, diagnostics | requête ≤ 1 Kio ; 40 pages ; trame ≤ 48 Mio |
| Stream `present` | entre fenêtres | présentation sur deux fenêtres | trame ≤ 8 Mio ; 6 fenêtres |
| Stream `lsp` *[cible]* | les deux | JSON-RPC de tinymist | en-tête et corps bornés |
| Stream `view` *[cible]* | requêtes → pages | très gros fichiers (ADR 0028 D4) | page 64 Kio |
| Stream `ext` *[cible, ADR 0039]* | les deux | appels vers l'hôte des greffons, réponses, événements, propositions | message ≤ 4 Mio ; crédit par extension |
| Stream `ai` *[cible, `AD-82`]* | les deux | requêtes au fournisseur (par Go), flux de réponse, propositions | message ≤ 4 Mio ; flux annulable |
| Socket MCP *[cible, `AD-82`]* | `inkwell mcp` → instance | outils MCP relayés vers le registre de commandes | socket Unix `0600` ; option `127.0.0.1` + jeton + `Origin` |
| Route `/ext/<id>/` *[cible]* | Go → webview | icônes et polices des thèmes, servies par le gestionnaire d'actifs de Wails depuis le paquet vérifié | aucun port |
| Bindings (services) | appel → réponse | contrôle : identité, inviter, rejoindre, personnes, projet, export, fenêtres | entrées bornées et validées en Go |

### 7.2 Règles

- **B-1** Chaque Stream **DOIT** avoir : un codec borné en Go et en TypeScript, des vecteurs
  dorés partagés (`internal/bridge/testdata/*.json`), une cible `Fuzz*` côté Go, et une
  version choisie par la première trame (pas de préfixe magique).
- **B-2** Go valide tout ce qui vient de la webview comme s'il venait d'ailleurs : taille,
  type, domaine. La webview affiche du contenu venu des pairs ; un défaut de la webview ne
  **DOIT** donner à un pair aucun pouvoir sur Go.
- **B-3** Le crédit est la seule contre-pression : « consommé » côté Go → webview = remis au
  `Repo` après la garde ; côté webview → Go = écrit sur le flux du pair (annexe A.1).
- **B-4** Les notifications d'état (rôles, attestations, retraits, suspensions, ensemble
  admis, tuples d'auteur) sont coalescées et envoyées avant tout message qui en dépend.
- **B-5** Une erreur rendue par un service **DOIT** être un code stable plus des arguments
  (`{code, args}`), traduit côté TypeScript ; jamais un texte Go brut à l'écran *[cible]*.
- **B-6** Les services Wails sont minces : ils valident, appellent un cas d'usage de `app`,
  traduisent l'erreur. Aucune logique de domaine dans un service.
- **B-7** La CSP est stricte : `default-src 'self'`, styles par nonce par lancement, seul
  `'wasm-unsafe-eval'` pour Automerge, `worker-src 'self'` avec le Worker ; aucun contenu
  distant ; aucun `eval`. Le SVG d'aperçu est affiché en `<img>`, jamais inséré comme balisage.
- **B-8** La construction `-tags server` (e2e sans fenêtre) écoute sur `127.0.0.1` et **NE
  DOIT JAMAIS** être publiée (AC-14).

### 7.3 Canal Go ↔ hôte des greffons (interne)

*[cible, ADR 0039 ; `AD-86`]* Interne (comme le pont), mais soumis à B-1 : codec borné, vecteurs
dorés, `Fuzz*`. L'enfant est traité comme **compromis** : le parent impose tout, indépendamment du
contenu des messages.

- **IPC-1 Transport.** Une paire de sockets Unix (`socketpair`, `SOCK_SEQPACKET`) créée par le
  parent avant le lancement, seul descripteur réseau de l'enfant ; plus des descripteurs `memfd`
  scellés (`F_SEAL_WRITE|F_SEAL_SHRINK|F_SEAL_GROW`) pour les modules (§9.10). Aucun autre
  descripteur hérité (`close_range` avant `execve`).
- **IPC-2 Cadrage.** Trame = `type u8 ‖ id d'appel u32 ‖ compartiment u16 ‖ génération u32 ‖
  longueur u32 ‖ charge CBOR` ; charge ≤ 4 Mio (propositions et SVG ≤ 4 Mio, §15.4) ; au-delà :
  erreur `resource-exhausted`, la trame est jetée, la connexion n'est pas réinitialisée.
- **IPC-3 Autorité.** C'est le **parent** qui attribue `compartiment` et `génération` à chaque
  activation et qui garde la table (compartiment, génération) → (greffon, droits, projet, mandat).
  Un appel hôte venu de l'enfant n'est honoré que si sa génération est courante et que **les droits
  du compartiment** le permettent ; l'enfant ne peut rien revendiquer de plus (§9.10, H-4).
- **IPC-4 Concurrence.** Une instance WebAssembly sert **un appel à la fois** (wazero ne garantit
  pas l'appel concurrent d'une même instance) ; file ≤ 8 appels par greffon, au-delà
  `resource-exhausted` ; ≤ 16 appels en vol par hôte ; équité par tourniquet entre greffons.
- **IPC-5 Annulation.** Trame `cancel(id)` ; l'enfant annule le contexte de l'appel (interruption
  wazero) ; une réponse arrivée après l'annulation, ou d'une génération périmée, est ignorée ;
  rien de ce qu'elle porte ne prend effet (mandat révoqué, §15.10).
- **IPC-6 Vie.** Le parent supervise (recul, `Pdeathsig`) ; arrêt : `app.stopping` (§15.9), puis
  SIGTERM, puis SIGKILL après 1 s.

## 8. Réseau

Le réseau entre appareils est entièrement décrit par la [spécification du protocole](../protocol/spec.md).
Cette section dit seulement qui l'implémente et comment l'application s'y branche.

| Couche du protocole | Implémentée par | Remarque |
|---|---|---|
| L0 identité, signatures à domaine séparé | `identity`, `protocol/sig` | clé par appareil ; ordre premier exigé |
| L1 transport (QUIC v1/TLS 1.3, TCP/Noise/yamux, relais circuit v2, DCUtR, mDNS `_inkwell._udp`) | `internal/transport/p2p` (+ `protocol/libp2p`) | lien direct toujours préféré ; mode « relais seulement » |
| L2 session (`/inkwell/{control,repo,presence,large}/2.0.0`, vrac, salon) | `internal/transport/session` *[cible]*, `p2p` (vrac, salon) | session = connexion ; passage relais → direct = nouvelle session |
| L3 classes (SET, ATTEST, DÉPÔT, PRÉSENCE) | session (curseurs, files) + ledger (ensembles) | |
| L4 dépôt (CBOR automerge-repo, admission, couverture, garde) | webview (`@inkwell/protocol`, adaptateur) | Go ne lit jamais l'intérieur |
| Invitation (Wormhole) | `internal/invite` | canal séparé (ADR 0032) |

Règles d'application :

- **N-1** Un seul hôte libp2p par processus, lié au projet ouvert (le HELLO porte le
  `ProjectID`) *[à décider pour plusieurs projets : `AD-01`]*.
- **N-2** Le filtre de connexion **DOIT** n'admettre que les appareils membres non retirés,
  plus, pendant une fenêtre d'invitation, les inconnus limités au salon.
- **N-3** Découverte : mDNS sur le LAN (désactivable) ; adresses du ticket ; relais
  configurables (`AD-43`) ; message clair si aucun pair n'est vu après 10 s (conception, zone
  d'ombre 9).
- **N-4** Les réglages réseau (découverte locale, relais, mode relais seulement, serveur de
  rendez-vous) se règlent dans l'interface, à côté de la fonction ; les variables
  d'environnement `INKWELL_*` restent pour les tests *[cible, `AD-21`]*.
- **N-5** Le port de l'application vers le réseau est `internal/transport` ; avec la session
  v2, il expose des connexions et des flux par classe (`AD-06`), que `memnet` implémente aussi
  pour que la simulation reste déterministe.

## 9. Sécurité

### 9.1 Frontières de confiance

| # | Frontière | Ce qui la traverse | Contrôle principal |
|---|---|---|---|
| TB-1 | réseau → `p2p` | connexions, flux | TLS 1.3/Noise lié à la clé ; filtre de connexion (membres) ; salon seul pour les inconnus ; gestionnaire de ressources |
| TB-2 | `p2p` → session, ledger | trames, objets signés | ordre de vérification du §12 du protocole ; bornes §16 ; décodage strict |
| TB-3 | Go → webview (contenu des pairs) | messages de dépôt | profil CBOR, admission, couverture, garde G1–G12, Worker borné |
| TB-4 | webview → Go | enveloppes, requêtes de stockage, bindings | codecs bornés, validation en Go (B-2) |
| TB-5 | Go → sidecars | sources Typst écrites par des pairs | enfermement (§9.5) |
| TB-6 | Go ↔ disque | dossier du projet (que d'autres programmes peuvent modifier) | `os.Root` ; règle de propriété ; liens symboliques sortants refusés |
| TB-7 | invitation | texte d'invitation via un serveur tiers | PAKE, usage unique, 15 min, tailles bornées (ADR 0032) |
| TB-8 | construction | dépendances, sidecars | versions exactes, SHA-256 épinglés, govulncheck, SBOM, grype, âge minimal npm 7 jours |
| TB-9 | extensions | greffons (code tiers), greffons de projet, cadres de rendu | hôte enfermé, capacités, règle de flux (§15.5), vues déclaratives, assainisseur SVG, ND-1 à ND-8 (§15) |
| TB-10 | dorsal de stockage | objets répliqués hors de l'appareil | chiffrement avant l'extension, adressage par contenu, revérification (§6.7) |

### 9.2 Modèle de menace (N7)

| Acteur | Peut | Ne doit jamais pouvoir | Résiduel accepté |
|---|---|---|---|
| Inconnu sur Internet | se connecter si l'adresse est connue | ouvrir autre chose que le salon, et seulement pendant une fenêtre ; épuiser la mémoire | voir que l'appareil écoute |
| Inconnu sur le LAN | voir le nom de la machine et le titre pendant une fenêtre visible | voir le contenu | annonce mDNS du PeerID (désactivable) |
| Lecteur malveillant | lire ; présence bornée | écrire du texte ; occuper des ressources au-delà de sa part | — |
| Personne qui écrit, malveillante | écrire du contenu valide, attribué, réversible | gêner les autres : bloquer, ralentir, faire diverger sans arbitre, épuiser mémoire ou disque au-delà de son quota | R1 : auto-équivoque jusqu'à l'arbitrage `equiv/` (ADR 0031, accepté) ; volume valide jusqu'au quota (`AD-24`) |
| Ancien membre, admin retiré | garder sa copie | antidater, revenir, sceller la liste pour prendre le contrôle | sceller la gestion des membres par antidatage d'un ancien admin compromis (ADR 0017 : gestion gelée, édition continue) |
| Admin en place | gérer les personnes et les décisions | — (de confiance, R7) | un admin malveillant peut retirer des personnes : c'est son rôle |
| Relais | voir qui parle à qui, quand, combien | lire ou modifier | métadonnées (protocole §18) |
| Serveur Wormhole | voir un code haché et deux adresses ; refuser le service | lire le ticket | indisponibilité de l'invitation par code (le texte reste possible) |
| Document hostile (Typst) | consommer CPU, mémoire, sortie dans ses bornes | lire hors du projet, joindre le réseau, écrire hors de sa sortie, survivre à inkwell | — |
| Dépendance compromise | — | entrer sans revue (Dependabot sans fusion automatique) | risque de chaîne d'approvisionnement résiduel |
| Greffon malveillant (installé par la personne) | ce que ses capacités accordées permettent, dans ses bornes | lire puis faire sortir du clair (règle de flux (§15.5)) ; agir hors de ses capacités ; cacher l'interface du cœur ; ralentir l'application | ce qu'une capacité accordée permet ; une évasion du moteur reste enfermée dans l'hôte |
| Admin qui ajoute un greffon de projet hostile | lui faire lire le projet chez chacune (ce qu'il lit déjà) et proposer des modifications | écrire au nom d'une personne sans son geste (PP-2) ; atteindre un autre projet, les fichiers, le réseau, les clés | interface trompeuse dans son cadre attribué (PP-5) ; R7 (admins de confiance) |
| Dorsal de stockage hostile | refuser, ralentir, effacer, rejouer d'anciennes têtes | lire ou modifier sans être vu | indisponibilité de la sauvegarde distante |
| Fournisseur d'IA (distant ou de l'entreprise) | lire ce que la politique et l'aperçu lui envoient ; répondre n'importe quoi | écrire sans proposition acceptée ; obtenir des outils hors classe | ce qui est envoyé avec accord |
| Personne qui écrit, malveillante, par injection de prompt | placer des consignes dans le texte que l'IA d'une autre lit | faire agir l'IA d'une autre au-delà de `lire` et `proposer` (IA-2) ; obtenir une écriture sans relecture | une proposition trompeuse, relue et attribuée |
| Client MCP | appeler les outils de sa portée après accord | écrire directement, lire un autre projet, relayer un jeton | ce que sa portée permet |
| Voleur de l'appareil | tout lire (pas de chiffrement au repos, NG3) | — | documenté ; chiffrement du disque par le système recommandé |
| Autre programme de la même session | lire le dossier du projet | lire le pont (aucun port) | hors périmètre |

### 9.3 Contrôles

- **S-1** Tout octet d'un pair passe, dans cet ordre, la taille, le décodage strict, la
  validité des clés, la signature, le rôle, puis seulement l'usage (protocole §12).
- **S-2** Rien de sécuritaire ne se négocie (capacité unique `core/2`, protocole §5.4).
- **S-3** Les secrets (clé privée, secret du ticket, jeton d'un serveur local) ne sont jamais
  journalisés, comparés en temps constant, et ne quittent pas leur paquet.
- **S-4** Fichiers d'inkwell `0600`, dossiers `0700` ; aucune élévation de privilège.
- **S-5** Aucune donnée de document dans un journal, une trace ou un rapport (§11).
- **S-6** Un défaut de la webview ne donne aucun pouvoir sur Go (B-2) ; la clé reste en Go.

### 9.4 Gestion des clés

| Clé | Création | Stockage | Usage | Rotation, perte |
|---|---|---|---|---|
| Clé Ed25519 de l'appareil | `crypto/rand` au premier lancement | `device.key` 0600, écriture atomique, revalidée (ordre premier) au chargement | identité libp2p, signatures à domaine séparé (`inkwell/<domaine>/v1\0`) | pas de rotation en v1 : la clé **est** l'appareil. Perte ou vol : un admin retire l'appareil ; la personne ajoute un nouvel appareil à son groupe. Perte du seul appareil admin sans second admin : nouveau projet (ADR 0017) |
| Clés de session | éphémères, TLS 1.3 / Noise | mémoire | chiffrement d'une connexion | à chaque connexion (confidentialité persistante) |
| Secret du ticket | `crypto/rand` | mémoire de l'hôte | admission par le salon | 15 min, usage unique |
| Clé PAKE | SPAKE2 (wormhole-william) | mémoire | chiffrer le texte d'invitation | une par code |
| Signature des versions publiées | cosign sans clé (OIDC du workflow) ; GPG pour `.deb`/`.rpm` | GitHub (environnement `release`) | vérification par la personne et par l'outil de mise à jour | rotation GPG documentée dans `docs/releasing.md` |

Il n'existe **aucune** clé symétrique de groupe : la « rotation de la clé de session » de
l'ADR 0005 (point 5) n'a pas d'objet ; un retrait prend effet par le filtre de connexion et
la fin de la couverture (`AD-09`). Le stockage de la clé dans le trousseau du système est
écarté en v1 (dépendance, cgo ou D-Bus selon le système) et réexaminé avec le chiffrement au
repos.

### 9.5 Enfermement des sidecars (F5)

| Mesure | Linux | macOS, Windows |
|---|---|---|
| Binaire vérifié (SHA-256) avant chaque lancement ; extrait en lecture seule | oui | oui |
| Environnement construit de zéro ; proxys vers un port refusé | oui | oui |
| Pas de réseau : espace de noms réseau et utilisateur privé, repli journalisé sur les proxys refusés (`Engine.NetworkMode`) | oui | non *[`AD-18`]* |
| `--root` sur le projet ; liens symboliques sortants refusés avant lancement | oui | oui |
| Lecture limitée au projet, au binaire et aux paquets fournis (Landlock) | *[cible, `AD-28`]* | — |
| Sortie bornée par le noyau : tmpfs privé de taille fixe dans un espace de noms de montage, fichiers déclarés seuls inscriptibles (Landlock), `RLIMIT_FSIZE` ; repli Landlock ABI ≥ 3 à parts fixes ; sinon échec fermé (ADR 0033) | oui | lancement borné refusé |
| Mémoire (`RLIMIT_AS`), délai (`context.WithTimeout`, 30 s pour un export), CPU (compilations simultanées bornées) | oui | délai et CPU seulement |
| Mort avec le parent : `Pdeathsig` (fil verrouillé), groupe de processus | oui | *[`AD-18`]* |
| Relance bornée (tinymist) : recul exponentiel, jamais plus d'une relance par minute sans message | *[cible, `AD-20`]* | — |

Un `tinymist lsp` résident lit le projet mais n'écrit rien : toute écriture lui est refusée
(Landlock) *[cible]*.

### 9.6 Fichiers

- **F-1** Toute E/S dans le projet, la configuration ou le cache passe par `os.Root`.
- **F-2** Inkwell n'écrit que les fichiers qu'il possède (`project.json`) ; un fichier dont
  le SHA-256 a changé est une modification externe : signalée, jamais écrasée avant accord
  (ADR 0020) ; fusion en trois voies (B1).
- **F-3** Les noms reçus d'un pair sont validés (`ValidateName`, `ValidatePath` : portables
  Windows/macOS/Linux, NFC, pas de `..`, pas de nom réservé) avant tout usage.

### 9.7 Chaîne d'approvisionnement

Versions exactes (`go.mod`, `pnpm-lock.yaml`, `mise.lock`) ; npm : âge minimal 7 jours,
scripts de construction listés ; Dependabot propose, n'intègre jamais seul (ADR 0008) ;
`govulncheck`, `gosec`, `gitleaks`, SBOM syft par artefact et `grype` dans `mise run ci` ;
sidecars : URL officielle, SHA-256 de l'archive et du binaire épinglés dans
`internal/sidecar/manifest.json` ; licences vérifiées (MPL-2.0 compatible).

### 9.8 Confidentialité

| Flux | Qui voit quoi | Réglage |
|---|---|---|
| mDNS | les machines du LAN voient le PeerID ; pendant une fenêtre visible, le nom de la machine et le titre | « Découverte sur le réseau local », désactivable |
| Rendez-vous Wormhole | le serveur voit un code haché, deux adresses, l'heure | serveur configurable ; codes désactivables (texte seul) |
| Relais | adresses, PeerID, volume, heure ; jamais le contenu | relais configurables ; mode relais seulement ; LAN seulement |
| Entre membres | `ProjectID`, heure et volume des modifications, présence | « Me rendre invisible » coupe curseur et sélection |
| Vérification de mise à jour | numéro de version et plate-forme | désactivable *[`AD-17`]* |
| Paquets Typst | rien à l'exécution | — |

La promesse de confidentialité (une page, dans l'application et sur le site) est un livrable
de l'étape 5.

### 9.9 Résiduels acceptés

R1 (auto-équivoque, ADR 0031) ; R7 (admins en place de confiance) ; NG3 (pas de chiffrement
au repos) ; métadonnées visibles du relais et du serveur de rendez-vous ; scellement de la
gestion des membres par un ancien admin compromis (ADR 0017) ; risque de dépendance
compromise après revue.

### 9.10 Enfermement de l'hôte des greffons

*[cible, ADR 0039 ; `AD-87`]* Exécutable et vérifiable, comme §9.5. L'enfermement est fait
**avant** tout code tiers : l'enfant le pose lui-même au démarrage (`inkwell --plugin-host`),
avant de lire le premier module ; le parent vérifie qu'il est posé (H-6) avant d'envoyer un
module.

| # | Mesure | Linux | Ailleurs |
|---|---|---|---|
| H-1 | **Aucun accès au système de fichiers** : Landlock, tous les droits gérés, aucune règle (ABI ≥ 1) | oui ; échec fermé | greffons tiers refusés (`AD-18`) |
| H-2 | Modules reçus par `memfd` scellé, hachage vérifié **par le parent** avant envoi ; l'enfant n'ouvre aucun chemin | oui | — |
| H-3 | **Pas de cache de compilation sur disque en v1** : compilation en mémoire, cache en mémoire pour la vie de l'hôte (un hôte vit autant que le projet) ; coût mesuré par P-PLUG-BENCH ; un cache disque géré par le parent est une décision ultérieure (`AD-87`) | oui | — |
| H-4 | **Compartiments** : un hôte = un compartiment de confiance. Hôtes distincts pour : (a) chaque projet ouvert (greffons de projet) ; (b) greffons globaux **sans** canal de sortie (§15.5, classe O0) ; (c) **chaque** greffon qui a un canal de sortie réseau ou `storage.backend`, seul dans son hôte. Une évasion n'obtient que les droits de son compartiment | oui | — |
| H-5 | **seccomp** (liste blanche) : `read`, `write`, `recvmsg`, `sendmsg` sur les descripteurs hérités, `mmap`, `mprotect`, `munmap`, `madvise`, `futex`, `clock_gettime`, `nanosleep`, `getrandom`, `sched_yield`, signaux du runtime Go, `exit_group` ; **refusés** : `socket`, `connect`, `open*`, `execve*`, `ptrace`, `process_vm_*`, `memfd_create` | oui ; échec fermé | — |
| H-6 | Réseau : `socket()` refusé par H-5 ; en plus, espace de noms réseau privé quand le système le permet. Sur Ubuntu 24.04 (`kernel.apparmor_restrict_unprivileged_userns=1`), les paquets `.deb` installent un profil AppArmor `inkwell` qui permet `userns` ; sans profil (archive autonome), l'espace de noms manque, H-5 suffit, l'état est montré dans « Détails techniques » | oui | — |
| H-7 | Mémoire : chaque module plafonné (`WithMemoryLimitPages`, 64 Mio par défaut, 256 Mio au plus) ; **budget par hôte** 512 Mio de mémoire linéaire cumulée, contrôlé à l'activation (refus avec message, déchargement d'abord du greffon inactif le plus ancien) ; `debug.SetMemoryLimit` à 640 Mio et `RLIMIT_AS` à 4 Gio (le compilateur réserve des régions de code) ; dépassement : l'hôte entier meurt, le superviseur le relance, ses greffons se réactivent à la demande | oui | — |
| H-8 | `Pdeathsig`, groupe de processus, superviseur avec recul (C-7) | oui | — |

**Échec fermé, visible.** Si H-1 ou H-5 ne peut pas être posé : aucun greffon tiers ni de projet ne
se charge ; bandeau unique « Les extensions sont désactivées : ce système ne permet pas de les isoler
— Détails » ; les extensions intégrées en TypeScript restent. En v1 (Linux seulement, `AD-18`), les
extensions intégrées en WebAssembly (Tâches) suivent la même règle : sans isolation, Tâches est
indisponible et le dit.

**Honnêteté sur l'isolation.** Une instance WebAssembly qui plante ne touche pas les autres (piège
wazero) ; une **évasion native** donne tout l'hôte, donc tous les greffons de son compartiment
(H-4), jamais un autre compartiment ni le processus principal. Preuve exigée : test d'un **enfant
simulé compromis** (programme de test qui parle IPC en revendiquant d'autres compartiments,
générations, tailles et appels hôte) : tout est refusé par le parent (`PLUG-COMPROMISED-*`).

## 10. Erreurs et reprise

### 10.1 Classes d'erreurs

| # | Classe | Exemple | Traitement | Ce que voit la personne |
|---|---|---|---|---|
| E-1 | octets d'un pair invalides | trame tronquée, signature fausse | jetés, compteur par pair, sanction du protocole (§13) ; jamais de panique | rien, ou l'état « Attention » agrégé |
| E-2 | refus de droit | un lecteur écrit ; un révoqué envoie | modification non admise ; règle H | « Attention : X tente d'écrire sans droit » |
| E-3 | environnement | `ENOSPC`, `EIO`, `EROFS`, permission | aucune écriture partielle ; état gardé en mémoire ; recul ; reprise automatique | bandeau qui dit quoi faire (ADR 0029 I5) |
| E-4 | corruption | `IKV1` faux, Bao faux, signature fausse au rechargement | quarantaine, sauvegarde, puis pairs | « Une copie abîmée de X a été remplacée… » |
| E-5 | sidecar | erreur Typst ; délai, mémoire ou sortie dépassés ; plantage | erreur Typst = diagnostics ; bornes = message ; relance avec recul | panneau « Problèmes » ; « Document trop lourd à compiler » |
| E-6 | action de la personne | nom invalide, fichier trop grand, invitation expirée | refus avec code | une phrase, une action (i18n) |
| E-7 | défaut interne | invariant violé | panique seulement au démarrage ; ensuite erreur journalisée et composant relancé par son propriétaire si c'est sûr | message générique avec « Copier les détails » |

Règles Go : erreurs sentinelles par paquet (`ErrX`), enveloppées par `%w`, testées par
`errors.Is`/`errors.As` ; `errors.Join` dans les `Close` ; un texte d'erreur ne contient
jamais de donnée de document.

### 10.2 Invariants de non-perte

I1 à I8 de l'[ADR 0029](../adr/0029-reliability.md) sont normatifs : ≤ 200 ms de frappe
perdue sur `kill -9` ; ≤ 500 ms sur coupure de courant ; jamais de fichier à moitié écrit ;
rien ne s'efface sans copie durable ; disque plein sans écriture partielle ; octet abîmé
détecté avant usage, vérification de fond hebdomadaire ; aucune boucle sans fin ; un pair ne
peut pas épuiser la mémoire d'un autre.

### 10.3 Scénarios de reprise

| Situation | Détection | Reprise |
|---|---|---|
| Plantage ou `kill -9` | marqueur d'exécution resté dans `.inkwell/lock` *[cible]* | réouverture ; « Inkwell s'est fermé brusquement. Rien n'est perdu : N modifications récupérées. » |
| Coupure de courant | idem | idem ; au pire 500 ms perdues localement, souvent revenues des pairs |
| Disque plein | `ENOSPC` | bandeau ; écritures retentées ; téléchargements de blobs suspendus |
| Écriture partielle | impossible par D-1 ; un temporaire orphelin est nettoyé à l'ouverture | — |
| Fichier abîmé | `ErrCorrupt`, Bao, signature | quarantaine → sauvegarde → pairs |
| Bascule de liste interrompue | `trust.next/` présent | reprise (D6) ; préparation orpheline : échec fermé |
| Migration interrompue | `.inkwell.next/` présent | reprise (V-3) |
| Hors ligne long | aucun lien | « Hors ligne, N changements en attente » ; au retour, bannière « Pendant votre absence… » |
| Plantage de la webview | fermeture des Streams | nouvelle époque ; rechargement depuis Go |
| Plantage d'un sidecar | sortie du processus | relance bornée ; message au-delà d'une par minute |
| Deuxième instance | verrou *[cible, `AD-12`]* | la seconde passe la main à la première et sort |
| Dossier `.inkwell/` supprimé à chaud | surveillance | état gardé en mémoire ; proposition de recréer le dossier |
| Horloge décalée | — | aucune décision de sécurité ne dépend de l'horloge sauf l'expiration du ticket et `seq` des attestations (`max(dernier + 1, maintenant)`) |

### 10.4 Divergences et concurrences

| Concurrence | Règle | Interface |
|---|---|---|
| texte | Automerge fusionne toujours ; zone concurrente marquée (étape 4) | liseré, carte « Garder celle de… » |
| arbre (renommer, déplacer) | registre Automerge, plus grand opId ; cycles et homonymes déterministes (ADR 0023) | entrée dans l'Activité |
| binaire remplacé deux fois | les deux gardés (« plan (version de Paul).png ») | carte |
| membres (deux admins) | pli déterministe (ADR 0017, 0021) | carte aux admins |
| décisions d'admin | valeur la plus prudente, frontière `contested` (ADR 0030 D2) | carte aux admins |
| modification externe et frappe | fusion en trois voies depuis le dernier accord avec le disque (B1) | lignes marquées, « Voir » / « Annuler » |
| auto-équivoque | arbitrage `equiv/` par un admin, document de récupération (ADR 0031) | nommée dans l'interface |

## 11. Observabilité, sans donnée de document

- **O-1** Journaux structurés `log/slog`. Développement : sortie d'erreur. Production :
  fichier JSON tournant (5 × 4 Mio) dans `$XDG_STATE_HOME/inkwell/logs/` (0600), écrit par un
  petit gestionnaire maison sans dépendance *[cible, `AD-16`]*.
- **O-2** Interdit dans tout journal : texte, octets de charge, contenu d'un message, secret,
  titre de projet. Permis au niveau `Debug` seulement, et localement : chemins relatifs,
  PeerID. L'export les remplace par des pseudonymes stables (HMAC à clé locale).
- **O-3** Journal circulaire réseau des 1 000 derniers événements : **métadonnées seulement**
  (heure, pseudonyme du pair, classe, type de trame, taille, génération, verdict) ; jamais de
  charge (`AD-15`). Il sert à rejouer un incident en simulation.
- **O-4** Compteurs en mémoire (par pair : trames, refus, octets ; par Stream : crédits ;
  disque : latence de `fsync`) affichés dans « Détails techniques ». Rien ne quitte
  l'appareil.
- **O-5** Enregistreur de vol Go (`runtime/trace.FlightRecorder`, Go ≥ 1.25) armé à la
  demande et quand la boucle d'un acteur ne répond plus depuis 2 s ; la trace ne contient
  aucune donnée de document (aucune étiquette ni tâche nommée d'après un contenu).
- **O-6** Rapport de bug : archive **locale** (journaux expurgés, journal circulaire,
  version, plate-forme, trace éventuelle) ; aucun envoi automatique (`AD-16`).
- **O-7** Preuve : un test e2e « canari » tape une chaîne unique et vérifie qu'elle n'apparaît
  dans aucun journal, aucun journal circulaire, aucune trace, aucun rapport *[cible]*.

## 12. Performance

### 12.1 Chemins chauds

| # | Chemin | Où | Garde-fou |
|---|---|---|---|
| HP-1 | frappe → affichage local | CodeMirror, fil d'affichage | Automerge hors du fil (ADR 0028 D2) ; aucune tâche > 50 ms |
| HP-2 | modification locale → pair | Worker → Stream `net` → `bridge.Router` → session → libp2p | 0 allocation par trame ≤ 64 Kio ; crédits ; flux par classe |
| HP-3 | réception → `Repo` | p2p → session → pont → Worker (profil, admission, couverture, garde) | parts réservées ; DRR imputé à l'auteur ; garde avant WASM |
| HP-4 | binaires | `blob` (BLAKE3, Bao, tranches) | flux plutôt que `ReadAll` ; `BenchmarkPut/Read/Fetch` |
| HP-5 | persistance | Stream `store` → `store.KV` | `fsync` groupé (D-8) ; voies parallèles |
| HP-6 | aperçu | projection → `typst compile` → SVG des pages visibles | annulation des compilations périmées ; pages bornées |
| HP-7 | extensions | registre (démarrage), appels vers l'hôte des greffons, rendus | activation paresseuse ; budgets du §15.4 ; `BenchmarkRegistryBuild`, P-PLUG-BENCH |

`HOT_PATHS:` de `CLAUDE.md` **DOIT** lister les paquets Go de ces chemins : aujourd'hui
`internal/blob internal/transport/frame internal/transport/mux` ; après la bascule d'ADR 0034 :
`internal/blob internal/transport/session internal/ledger`, plus les bancs de `protocol/sync2`
lancés par `bench:*` dans le sous-module (`AD-40`).

### 12.2 Paliers de taille (ADR 0028 D1)

Fluide ≤ 256 Ki caractères ; Grand jusqu'à 2 Mi (avant le Worker : seuil de lenteur à 512 Ki, et 0,5 à 2 Mi restent modifiables avec un bandeau, décision U34 qui précise l'ADR 0028) ;
au-delà, lecture seule en mémoire (blob) jusqu'à 64 Mi, puis lecture seule paginée par Go.
Le seuil est vérifié **avant** de créer ou modifier un document (`store/limits.ts`, porte
unique). Ce palier remplace le seuil de 10 Mo de l'ADR 0026 (`AD-07`).

### 12.3 Budgets et conditions de mesure

Les budgets sont ceux du §2. Conditions normatives de mesure :

- « frappe → pair » se mesure avec une frappe régulière de 5 caractères par seconde (ADR 0029
  §V) ; une rafale continue peut ajouter au plus la cadence de synchronisation
  d'automerge-repo (100 ms en Fluide, ADR 0028 D7) (`AD-27`) ;
- les budgets se mesurent sur les machines de CI (amd64 et arm64) avec `INKWELL_PERF=1` ;
  une mesure hors budget fait échouer le cas `PERF-*` ;
- toute optimisation d'un paquet de `HOT_PATHS:` est prouvée par `benchstat`
  (`-count ≥ 10`, p < 0,05, sans régression d'allocations) ; sinon elle est retirée.

### 12.4 Règles

- **PF-1** Pas d'allocation dans les boucles chaudes ; `sync.Pool` seulement avec une preuve
  `benchstat`.
- **PF-2** Flux plutôt que `ReadAll` pour tout ce qui vient d'un pair ou du disque.
- **PF-3** Démarrage paresseux : index et document ouvert seulement ; documents invisibles
  depuis 60 s déchargés (ADR 0028 D9).
- **PF-4** Profil PGO (`default.pgo`) à l'étape 6, tiré d'un profil réel (sans donnée de
  document).

## 13. Tests : ce que chaque niveau prouve

| Niveau | Outil | Où | Prouve | Porte |
|---|---|---|---|---|
| Unitaire | `go test`, Vitest | partout | chaque règle locale ; cas limites (borne, borne + 1) | `test:ci`, `fe:check`, couverture ≥ 85 % par paquet |
| Propriétés | `pgregory.net/rapid`, fast-check | `protocol/*`, `store`, `blob`, TS | décodage strict, `encode(decode(b)) = b`, pli identique dans tout ordre, bornes | `test:ci` |
| Modèle | rapid `StateMachine` | session, génération, LARGE, crédits, curseur SET | l'implémentation suit les mêmes règles que le modèle TLA+ | `test:ci` |
| Formel | TLA+ (TLC épinglé) | `docs/protocol/tla/` | sûreté et vivacité des générations et du crédit de connexion | `mise run tla` (P-TLA) *[cible]* |
| Fuzz | `go test -fuzz` | chaque décodeur (≈ 30 cibles aujourd'hui) | aucun plantage ni allocation non bornée sur entrée hostile | fuzz court en CI ; 1 h par cible avant la bascule (P-FUZZ) |
| Conformité | vecteurs JSON, exécuteurs Go et Vitest | `protocol/vectors`, `internal/bridge/testdata` | Go et TypeScript lisent et écrivent les mêmes octets ; matrice N/N−1 | P-VEC, P-COMPAT |
| Contrat | `transporttest`, tests de contrat automerge-repo | transports ; bibliothèque épinglée | une implémentation respecte le port ; la bibliothèque se comporte comme supposé | `test:ci` |
| Simulation Go | `memnet` + `testing/synctest` | transport, ledger, session | livraison et routage sous pertes, doublons, partitions, déterministe | `test:ci` |
| Convergence TS | `simnet` + 3 `Repo` réels | webview | convergence, couverture, non-perte après arrêt | 20 graines CI, 200 la nuit |
| Pannes | étiquette `faultinject`, LazyFS, netem | `store`, `blob`, `project` ; e2e | I1–I8 | job de nuit |
| Bout en bout | Playwright/WebKit, deux instances | `frontend/e2e` | le pont, l'interface, les parcours réels | `mise run e2e`, `e2e:netsim` |
| Installation | conteneurs Debian, Ubuntu, Fedora | `pkg:test` | le paquet s'installe et l'e2e « fumée » passe | publication |
| Mutation | gremlins, Stryker | prédicats de sécurité | les tests tuent les mutants | `mutate:go` (score, §2) |
| Performance | bancs Go, cas `PERF-*` | `HOT_PATHS`, frontend | budgets du §2 | `bench:compare`, `INKWELL_PERF=1` |
| Interface | `ux-walkthrough`, axe | chaque écran | cohérence, accessibilité, zéro jargon | avant fusion d'interface |
| Architecture | `arch-check.sh`, Biome | dépôt | règles du §4.5 | `mise run arch`, `fe:check` |
| Extensions | suite de contrat de l'API, tests `CAP-*` et `EXT-ND*`, kit de conformité des greffons, `PERF-EXT-*` | `internal/plugin`, `src/extensions`, `src/ext` | chaque point a un client intégré ; chaque capacité refuse sans droit ; ND-1 à ND-8 ; budgets | P-EXT-1 à 7, P-PLUG-BENCH |

Règles : un test prouve une règle nommée, sinon il n'a pas lieu d'être ; un test n'est jamais
déclaré instable sans taux mesuré et cause prouvée (D14) ; les tests lourds passent par
`scripts/heavy.sh` (cgroup 8 Go, deux à la fois).

## 14. Construction, paquets, publication

1. **Outils** épinglés par mise (`mise.toml`, `mise.lock`) ; Go 1.27.x ; Node et pnpm exacts.
2. **Sidecars** : `mise run sidecar:fetch` télécharge les archives officielles, vérifie deux
   SHA-256, compresse en gzip déterministe sous `internal/sidecar/bin/<os>_<arch>/` (hors
   Git), embarqués par `go:embed` avec étiquettes de plate-forme (ADR 0024).
3. **Frontend** : Vite (production) → `frontend/dist`, embarqué (`inkwell.Assets`).
4. **Exécutable** : Wails v3 (cgo, GTK4/WebKitGTK 6.0), `-trimpath`, version injectée
   (`-ldflags -X internal/version…`), date reproductible (`SOURCE_DATE_EPOCH`).
5. **Paquets** : goreleaser + nFPM, une exécution native par architecture
   (`ubuntu-24.04`, `ubuntu-24.04-arm`) ; `.deb`, `.rpm`, `tar.gz` ; entrée de bureau,
   AppStream, type MIME `text/x-typst`, icônes (ADR 0025).
6. **Traçabilité** : SBOM syft par artefact ; `grype` ; `checksums.txt` signé par cosign sans
   clé ; `.deb`/`.rpm` signés GPG si la clé existe.
7. **Vérification** : installation et e2e « fumée » en conteneurs propres (Debian 13,
   Ubuntu 24.04, Fedora 44), amd64 et arm64 ; AC-14 (aucune étiquette de test dans
   l'exécutable publié).
8. **Version** : le tag `vX.Y.Z` est la seule source ; `0.x` tant que le format des projets
   peut changer (§6.4).
9. **Mises à jour** : selon `AD-17` (gestionnaire de paquets pour `.deb`/`.rpm` ; outil de
   Wails v3 vérifié par signature pour l'exécutable autonome).
10. **Wails** : `v3.0.0-beta.34` aujourd'hui ; passage à la v3.0.x stable par la procédure de
    l'ADR 0008 (`AD-19`).

## 15. Évolution et extensibilité

*Section entière [cible] (ADR 0039 proposée) : aucune extension n'existe dans le code au 2026-11-01 ; les marques locales précisent, elles ne lèvent pas cette marque.*

**Inkwell est hautement extensible : c'est un attribut de qualité de premier rang** (§2), au
même niveau que la sécurité et la fiabilité (exigences N20, N21). Règle : **ouvert par
défaut**. Toute fonction visible par la personne est un point d'extension ; les fonctions du
cœur sont livrées comme **extensions intégrées** écrites contre la même API que les greffons
tiers (« on mange ce qu'on sert ») ; seul un **noyau fermé**, court et énuméré (§15.3), ne
l'est pas, chaque exclusion justifiée par le modèle de menace. Étude de référence :
[research/plugins.md](research/plugins.md) ; décision : brouillon d'ADR 0039 en
[annexe](#annexe--brouillon-dadr-0039--greffons-webassembly) ; doutes : `AD-57` à
`AD-76`.

### 15.1 Ajouter une fonction : la liste à cocher

Avant toute fonctionnalité, cocher dans l'ordre :

1. **Exigence** : une ligne dans `docs/exigences.md` (date, où, état).
2. **Point d'extension** : à quel point du §15.4 elle contribue. S'il n'existe pas, on crée
   le point (avec son contrat, sa capacité, son budget) **avant** la fonction ; une fonction
   qui a besoin d'un droit qu'aucune capacité ne donne révèle un point manquant, traité comme
   un défaut de l'API (règle « un seul chemin »).
3. **Vérité** : quelle source du §6.1 elle lit ou écrit. Une nouvelle vérité exige une ADR.
4. **Place** : l'extension intégrée (`src/ext/<nom>`) ou, si elle touche le noyau fermé, le
   paquet dont le rôle la contient (§4) ; un nouveau paquet seulement s'il a un seul rôle et
   que l'étape en a besoin ; il entre alors au §4.2, dans `arch-check.sh` et, s'il est chaud,
   dans `HOT_PATHS:`.
5. **Octets entre appareils** : une modification de la spécification du protocole selon ses
   règles d'évolution (§1.4 : extension compatible, inconnu ignoré ou « à comprendre »,
   GREASE), avec vecteurs, fuzz et ligne dans la table des bornes.
6. **Nouveau Stream ou kind** : codec borné Go + TS, vecteurs dorés, fuzz, ligne au §7.1.
7. **Nouvel état** : propriétaire (§5.2), persistance, atomicité, version (§6).
8. **Nouvelle file ou carte** : borne, comportement à la borne, reprise, propriétaire (C-4).
9. **Nouveau type de fichier** : une contribution `file-type` (§6.6 MF-14), sans `switch`
   ailleurs.
10. **Nouveau réglage** : une contribution `settings` à côté de la fonction (« tout ce qui se
    voit se désactive »), persisté dans les réglages de l'appareil ou, s'il est partagé,
    décidé par un admin (journal de décisions).
11. **Nouvelle dépendance** : justifiée, comparée, épinglée, licence vérifiée (ADR 0008).
12. **Interface** : composants et jetons Adwaita existants, vocabulaire du glossaire,
    raccourci dans la palette, revue `ux-walkthrough` claire et sombre.
13. **Budget** : la ligne du §15.4 ; activation paresseuse ; mesure `PERF-EXT-*`.
14. **Preuve** : un test par règle ; une ligne dans [traceability.md](traceability.md).
15. **ADR** si une frontière, un invariant, une décision acceptée ou le noyau fermé change.

### 15.2 Principes

- **X-1 Ouvert par défaut.** Commandes, menus et palette, barre d'outils, panneaux latéraux,
  barre d'état, vues et éditeurs de types de fichiers, extensions de l'éditeur (coloration,
  complétion, décorations, survol), étapes d'affichage de l'aperçu, exportateurs et
  importateurs, thèmes et jeux d'icônes, pages de réglages, transformations de document : tous
  sont des points d'extension (§15.4).
- **X-2 Un seul registre.** Le registre des contributions (`src/extensions`) est l'unique
  endroit où l'application apprend qu'une commande, un panneau, un type ou un exportateur
  existe. Le cœur s'y inscrit comme un greffon : manifeste, capacités, événements
  d'activation. Aucune liste codée en dur ailleurs (palette, menus, barre d'outils, liste des
  exportateurs, registre des types de fichiers de l'ADR 0027).
- **X-3 Un seul chemin.** Une extension intégrée n'a aucun droit qu'un greffon ne puisse
  demander par une capacité, sauf `ui.native` (§15.5), réservé au code compilé dans
  l'exécutable pour le chemin de frappe (CodeMirror, grille). Toute autre exception exige une
  ADR.
- **X-4 Politique ouverte, vérification fermée.** Une extension peut proposer, afficher,
  transformer, ordonner ; elle ne peut ni valider, ni signer, ni admettre, ni décider d'un
  droit (étude, §6bis, règle 3).
- **X-5 Trois verbes, une règle de conflit chacun.** *Contribuer* (tout le monde s'ajoute ;
  conflit de raccourci tranché par la personne), *envelopper* (chaîne ordonnée par la personne
  ; le cœur a des positions fixes), *remplacer* (un seul fournisseur actif par créneau, le
  défaut du cœur toujours disponible). Jamais de priorité numérique déclarée par l'extension
  (étude, §6.2).
- **X-6 Paresseux et mesuré.** Le registre se construit en lisant les manifestes **sans
  exécuter de code** ; une extension s'active à son premier événement (`onCommand`,
  `onFileType`, `onView`, `onExport`…) ; « au démarrage » est interdit aux greffons et
  justifié pour une extension intégrée. Chaque point a un budget (§15.4) ; « Détails
  techniques » montre, par extension, le temps d'activation, le nombre d'appels et le temps
  consommé.
- **X-7 Désactivable.** Toute extension, intégrée comprise, se désactive dans « Extensions »,
  sauf celles marquées essentielles (éditeur de texte de base, aperçu Typst, gestionnaire
  d'extensions), dont on peut seulement remplacer le fournisseur.
- **X-8 Rien de ce qui change le PDF.** Une extension n'entre jamais dans la compilation :
  l'entrée de `typst` est le dossier du projet, pareil chez tous (G5). Ce qui doit changer le
  rendu passe par un **paquet Typst** fourni par le projet (ADR 0024), y compris ses propres
  greffons WebAssembly Typst, purs et répliqués avec le projet (`AD-64`).

### 15.3 Le noyau fermé

Liste exhaustive. Une extension ne peut ni le remplacer, ni l'envelopper, ni l'appeler hors
des lectures listées ; `arch-check.sh` et Biome l'imposent (AC-16, FE-3).

| # | Élément fermé | Paquets, modules | Raison (modèle de menace §9.2) | Ce qu'une extension peut en lire |
|---|---|---|---|---|
| K-1 | Identité et clés de l'appareil | `identity`, `protocol/sig` | qui tient la clé signe au nom de la personne : usurpation de toutes ses modifications | `self.read` : PeerID, empreinte, nom, couleur |
| K-2 | Liste des membres, rôles, suspensions, décisions d'admin | `ledger`, `protocol/{trust,policy,listing}` | élévation de droits ; le pli doit être identique partout | `people.read` : personnes, rôles, présence (lecture) |
| K-3 | Signatures et ordre de vérification des octets venus d'un pair | `protocol/*`, session, `src/net` (admission, couverture, garde G1–G12) | un pair byzantin passerait par une extension complaisante | aucun octet de pair brut ; seulement l'état admis |
| K-4 | Réseau : transport, session, relais, découverte, invitation | `transport/**`, `invite` | fuite de contenu, déni de service, contournement du filtre de connexion | `presence.read` : qui est connecté, type de lien |
| K-5 | Invariants du CRDT : application des modifications, porte de taille, attribution | `src/workspace`, `src/store/limits.ts`, Worker | une modification non bornée ou non attribuée casse N7 | `document.read` ; écrire seulement par proposition (§15.5) |
| K-6 | Intégrité du stockage : magasin `IKV1`, blobs BLAKE3/Bao, quarantaine, écriture atomique, règle de propriété, `os.Root`, chiffrement et vérification au-dessus de tout dorsal (§6.7) — le **dorsal** lui-même est un point d'extension | `store`, `blob`, `project` | perte ou corruption silencieuse (I1–I8) | `project.read`, `blob.read` |
| K-7 | Compilation Typst et sidecars, leur enfermement | `typst`, `sidecar`, `preview` (cœur) | évasion d'un document hostile ; PDF différent selon les appareils | `compile.run` : demander un PDF ou un SVG du projet |
| K-8 | Pont, CSP, Streams | `bridge`, `src/net`, `index.html` | un défaut de la webview ne doit donner aucun pouvoir sur Go (B-2) | — |
| K-9 | Gestionnaire d'extensions : installation, signatures, droits, bornes, hôte des greffons | `internal/plugin`, `src/extensions` (hôte) | une extension qui s'octroie des droits | `extensions.read` (liste, état) |
| K-10 | Mises à jour d'inkwell | `AD-17` | substitution de l'exécutable | — |

### 15.4 Points d'extension : contrat, capacité, extension intégrée qui l'utilise, budget

Les interfaces sont décrites en WIT (`inkwell:plugin@1.0.0`, annexe, *Décision* point 2) ; l'API
TypeScript des extensions intégrées est générée depuis la même description.
« Intégrée » nomme l'extension du cœur qui **doit** passer par ce point (preuve du
dogfooding). Budgets mesurés par les cas `PERF-EXT-*` sur les machines de CI.

| Point | Interface WIT | Verbe | Capacités | Extension intégrée qui l'utilise | Activation | Budget |
|---|---|---|---|---|---|---|
| Commande | `commands` | contribuer | selon la commande | toutes : mise en forme, partager, exporter, versions, rechercher | `onCommand` | palette ouverte ≤ 50 ms avec 1 000 commandes ; appel d'un greffon : aller-retour p95 ≤ 2 ms + son travail, plafond 1 s |
| Menu, palette, raccourci | `menus` (données) | contribuer | — | menu Insérer (U9), palette (Ctrl+Maj+P), raccourcis | aucune (déclaratif) | 0 appel d'extension pour afficher |
| Barre d'outils par type | `toolbar` (données) | contribuer | — | barres Typst et Markdown (U4, U5), barre du tableur | aucune | 0 appel pour afficher ; état « actif » ≤ 4 mises à jour/s |
| Panneau latéral, onglet du tiroir | `panels` | contribuer | `ui.view` | Historique, Commentaires, Activité, Rechercher, Sommaire (U30), Tâches (U29, livrée en **greffon WebAssembly**) | `onView` | premier rendu ≤ 100 ms ; mises à jour ≤ 10/s ; **interaction** (clic dans une vue déclarative → vue mise à jour, aller-retour webview → Go → hôte compris) p95 ≤ 100 ms |
| Barre d'état, bandeau, toast | `status` | contribuer | `ui.view` | mots, « Page X sur Y », encodage, fin de ligne (U1) | `onFileType` | ≤ 4 mises à jour/s par élément, coalescées |
| Type de fichier : reconnaissance | `file-types` (données) | contribuer | — | Typst, Markdown, tableaux, texte, code, images, PDF (§6.6) | aucune | registre construit ≤ 10 ms |
| Vue, éditeur d'un type | `viewers` | remplacer (par type) | `document.read`, `document.propose` ou `blob.read` ; `ui.view` ou `ui.native` | éditeur CodeMirror, grille CSV (ADR 0026), visionneuse d'images, PDF.js (`AD-39`) | `onFileType` | ouverture d'un fichier ≤ 1 Mio : activation + premier rendu ≤ 200 ms ; activation d'un greffon : ≤ 150 ms module déjà compilé dans l'hôte, ≤ 500 ms compilation comprise (pas de cache disque en v1, H-3) |
| Coloration | `grammars` (Lezer intégrées) ; `tokens` (greffons) | contribuer | `document.read` | Typst, Markdown, langages de la liste fermée (`AD-54`) | `onLanguage` | intégrée : incrémentale, ≤ 5 ms par mise à jour de la zone visible ; greffon : plages de la zone visible, anti-rebond 150 ms après la dernière frappe, calcul périmé annulé, dernières plages gardées pendant l'attente (jamais de clignotement), calcul ≤ 50 ms, rendu ≤ 4 ms ; coupée au-delà de 2 Mi caractères (`AD-53`) |
| Complétion, survol, aide de signature | `completion`, `hover` | contribuer | `document.read` | LSP tinymist (`AD-20`) | `onLanguage` | p95 ≤ 100 ms, sinon résultats abandonnés |
| Diagnostics, décorations | `diagnostics`, `decorations` | contribuer | `document.read` | problèmes Typst, bande des modifications (U11), « qui a écrit quoi » | `onFileType` | ≤ 1 mise à jour par 300 ms par extension ; application ≤ 4 ms |
| Transformation de document, de sélection, avant collage | `transforms` | envelopper (collage) ou contribuer (commande) | `document.read`, `document.propose` | tri et collage du tableur, « remplacer tout », mise en forme | `onCommand` | proposition ≤ 4 Mio, appliquée en une modification ≤ 50 ms |
| Affichage de l'aperçu | `preview-overlays` | contribuer | `compile.read` | saut source ↔ aperçu, surlignage de la page courante | `onView` | ≤ 16 ms par page dessinée ; jamais dans la compilation (X-8) |
| Exportateur | `exporters` | remplacer (par format) | `compile.run`, `project.read`, `blob.read`, `export.write` | PDF (et PDF/UA, `AD-44`), archive autonome (`AD-38`) | `onExport` | en tâche de fond, annulable, progression ; plafond 120 s |
| Importateur | `importers` | remplacer (par format) | `import.read`, `document.create` | CSV → tableau, pandoc (`AD-33`) | `onImport` | idem ; résultat soumis à la porte de taille |
| Thème, jeu d'icônes | `themes`, `icons` (données, contrat de jetons §15.8) | remplacer | — | Adwaita clair, sombre, contraste élevé ; icônes des types (U6) | aucune | validation des contrastes ≤ 20 ms ; changement de thème ≤ 50 ms |
| Page de réglages | `settings` (schéma) | contribuer | — | Préférences : enregistrement (U8), éditeur, aperçu | `onView` | rendu ≤ 50 ms |
| Diff et historique | `history-views` | contribuer | `history.read` | différences lisibles (U27), « Qui a écrit quoi » | `onView` | ≤ 100 ms pour 1 000 modifications |
| Stockage distant | `storage-backend` | remplacer (un par projet et par appareil) | `net.hosts`, `storage.backend` (octets chiffrés seulement, §6.7) | aucune en v1 (le dossier local est le défaut) ; une extension intégrée « Dossier de sauvegarde » (disque externe) | `onBackup` | jamais sur le chemin d'édition ; appel ≤ 30 s ; file bornée |
| Miroir lisible | `mirrors` | contribuer | `project.read`, `mirror.dir` (un dossier local, par exemple monté par le client du nuage ; O1, RF-4) | aucune en v1 | `onSaved` | idem |
| Cycle de vie, événements | `events` (§15.9) | contribuer | celle de chaque événement | Activité (U12), « Pendant votre absence… », toasts de fin d'export | `onEvent:<nom>` | émission ≤ 1 µs sans abonné ; livraison asynchrone, coalescée ; arrêt : 1 s au plus pour tous les greffons |
| Rendu de diagramme | `renderers` (§15.14) | contribuer (par langage) | entrée I2 seulement (contrat pur) | « Diagrammes » : Graphviz (`wasm`) ; exemple : D2 ; Mermaid différé (P-FRAME) | `onLanguage` | p95 ≤ 300 ms (≤ 200 nœuds), plafond 5 s, hors du fil d'affichage |
| Fenêtres, dialogues, menus contextuels, gouttières, widgets, accueil | `windows`, `menus`, `gutters`, `inline-widgets`, `welcome` (§15.13) | contribuer | `ui.view` | fenêtre public de présentation (U28) ; menus contextuels de la liste des fichiers ; bande des modifications | `onCommand`, `onView` | ouverture d'une fenêtre ≤ 300 ms ; menus : 0 appel d'extension pour s'afficher |
| Fournisseur d'IA | `ai-providers` (§15.18) | remplacer (un par projet et par appareil) | adaptateurs **compilés dans le cœur** ; greffon : `ai.complete` seulement | socle compatible OpenAI ; profils de l'entreprise | `onCommand` | premier jeton affiché ≤ 1 s sur un serveur local ; jamais sur le fil d'affichage |
| Outils et surfaces d'IA | `ai-tools`, `ai-surfaces` (§15.18) | contribuer | classe d'outil déclarée (`lire`, `proposer`, `régler`) | « Reformuler », « Corriger », panneau de discussion | `onCommand`, `onView` | proposition affichée en diff ≤ 100 ms après la fin du flux |

### 15.5 Capacités

Aucune capacité ambiante : un greffon n'a ni système de fichiers, ni réseau, ni horloge non
déterministe, ni processus. Les capacités sont **nommées dans le manifeste, montrées en
phrases simples avant l'installation, accordées par appareil et par projet**, et revérifiées
par Go pour tout ce qui sort de la webview (B-2).

| Capacité | Donne | Contrôle | Phrase montrée |
|---|---|---|---|
| `self.read`, `people.read`, `presence.read`, `extensions.read` | lectures du §15.3 | registre | « voir les personnes du projet » |
| `document.read` | plages des documents texte **du projet ouvert**, à la demande (jamais un document entier copié d'office) | webview, porte de taille | « lire les documents de ce projet » |
| `document.propose` | proposer des modifications ancrées (étude, §5.2) ; la webview les valide puis les applique en **une** modification annulable, signée par la personne, étiquetée « via X » | webview : bornes, garde, attribution | « proposer des modifications » |
| `document.create` | créer un fichier texte par l'index (MF-4) | webview | « créer des fichiers » |
| `project.read`, `blob.read` | lire la projection et les blobs, par `os.Root` | Go | « lire les fichiers du projet » |
| `compile.run`, `compile.read` | obtenir un PDF, des pages SVG, `typst query` du projet ; lire les ancres de l'aperçu | Go, sidecar enfermé | « compiler le projet » |
| `export.write` | écrire **un** fichier choisi par la personne dans une boîte de dialogue de Go | Go | « enregistrer un fichier que vous choisissez » |
| `import.read` | lire **un** fichier choisi par la personne | Go | « ouvrir un fichier que vous choisissez » |
| `history.read` | historique des documents (auteurs, dates, différences) | webview | « voir l'historique » |
| `storage` | magasin clé-valeur privé, 10 Mio | Go, `os.Root` | — |
| `ui.view` | vues déclaratives (arbre de composants Adwaita validé, étude §8) | webview | — |
| `ui.canvas` | *différée*, comme `render.frame` : une vue libre exécuterait du code de greffon dans le processus web qui tient le pont ; rouverte seulement après la porte P-FRAME (§15.14), alors dans un processus web dédié | — | « afficher un dessin libre » |
| `ui.native` | monter du code dans le DOM | **extensions intégrées seulement** | — |
| `net.hosts` | requêtes HTTPS vers les hôtes nommés dans le manifeste, faites par Go (pas de cookie, pas de redirection hors liste) | Go | « se connecter à : nuage.example » |
| `storage.backend` | recevoir et rendre des objets **chiffrés** (§6.7) | Go | « garder une copie chiffrée de vos projets chez … » |
| `mirror.dir` | écrire la projection lisible dans **un** dossier choisi par la personne | Go, `os.Root` | « copier vos fichiers lisibles dans … » |
| `ai.complete` | envoyer à un modèle **ce que le greffon a déjà le droit de lire**, par Go, sous la politique IA du projet et après l'aperçu de ce qui part ; le greffon ne voit ni clé ni hôte (O4, RF-3) | Go | « envoyer du texte à l'IA autorisée pour ce projet » |
| `time.read` | horloge monotone (`clock_time_get`) ; sans elle, 0 | hôte | — |
| `render.frame` | *différée* : moteur de rendu à DOM dans un processus web dédié (§15.14, P-FRAME) | — | « exécuter du code d'affichage de X dans une zone isolée » |

**Règle de flux, propriété fermée** (`AD-58`, sécurité, *à auditer*). Chaque capacité est classée
par ce qu'elle **fait entrer** (source) et ce qu'elle **fait sortir** (puits) :

| Classe | Sources | | Classe | Puits |
|---|---|---|---|---|
| I0 | rien (réglages propres, `storage`) | | O0 | aucun hors de l'appareil : interface, `storage`, propositions et rendus **visibles** dans le projet |
| I1 | métadonnées : `self.read`, `people.read`, `presence.read`, `extensions.read`, événements sans contenu | | O1 | fichier choisi par la personne : `export.write`, `mirror.dir` (clair, local, montré) |
| I2 | contenu du projet : `document.read`, `project.read`, `blob.read`, `history.read`, `compile.run`, `compile.read`, entrée d'un `renderer`, événements `document.*` | | O2 | réseau : `net.hosts` |
| I3 | fichier choisi par la personne : `import.read` | | O3 | dorsal : `storage.backend` (le cœur ne lui donne que du chiffré) |
| | | | O4 | modèle d'IA : `ai.complete` (envoyé **par Go**, §15.18) |

- **RF-1** Un **compartiment** (§9.10 H-4 : le greffon et tous ceux qu'il peut appeler, §15.12)
  qui a O2 n'a **aucune** source au-delà de I0, sauf les secrets de son propre dorsal, **injectés
  par Go** dans la requête (le greffon nomme `secret:<nom>`, ne le voit jamais).
- **RF-2** O3 ne reçoit que des octets chiffrés par le cœur ; un dorsal (O3) qui a aussi O2 reste
  sans source I1–I3.
- **RF-3** O4 est le seul cas où du contenu I2 sort : jamais par le greffon, toujours par Go,
  après la **politique IA du projet** et l'**aperçu de ce qui part** (§15.18) ; un greffon qui a
  `ai.complete` n'a ni O2 ni O3.
- **RF-4** O1 avec I2 est permis (exporter, c'est le but) : le fichier est choisi par la personne
  dans une boîte de dialogue de Go, une fois par geste.
- **RF-5** Un greffon de **projet** n'a que I0–I2 et O0 (§15.10).
- Ce que la règle promet : **aucun contenu du projet ne quitte l'appareil par un greffon** sauf
  O1 (geste, fichier choisi) et O4 (politique et aperçu). Elle ne promet rien contre une personne
  qui copie le texte : le modèle de menace l'accepte (§9.2).

Bornes par appel, fixées par l'hôte : mémoire (pages WebAssembly plafonnées, 64 Mio par
défaut, 256 Mio au plus, demandés dans le manifeste et montrés), temps (1 s pour une
interaction, 120 s pour un export ; dépassement → interruption), taille de sortie, appels hôte
par seconde, opérations par proposition. Trois dépassements → greffon mis en pause, message
« X est trop lent et a été mis en pause ».

### 15.6 Deux sortes d'extensions, une seule API

| | Extension intégrée | Greffon |
|---|---|---|
| Code | TypeScript dans l'exécutable (`src/ext/<nom>`), ou WebAssembly fourni (Tâches) | module WebAssembly signé (annexe) |
| Où il tourne | webview (TypeScript) ; hôte des greffons (WebAssembly) | hôte des greffons, processus enfant enfermé (annexe, *Décision* point 1) |
| Manifeste, capacités, activation | oui, mêmes règles | oui |
| `ui.native` | permis (chemin de frappe) | jamais |
| Mise à jour | avec inkwell | gestionnaire d'extensions |
| Désactivable | oui, sauf essentielles | oui |

Règle vérifiée : un module de `src/ext/**` n'importe que `@inkwell/extension-api` et la
bibliothèque de composants (`src/ui/kit`), jamais `src/net`, `src/store`, `src/workspace` ni
un Stream (FE-3).

### 15.7 Portes de l'extensibilité

| # | Porte | Preuve |
|---|---|---|
| P-EXT-1 | aucune liste codée en dur de commandes, panneaux, types ou exportateurs hors du registre | Biome + revue ; test qui désactive toutes les extensions non essentielles et vérifie que l'interface ne garde que la coque |
| P-EXT-2 | FE-3 et AC-16 (noyau fermé) | `fe:check`, `mise run arch` |
| P-EXT-3 | chaque point du §15.4 a au moins une extension intégrée qui l'utilise et un test de contrat | suite de contrat de l'API |
| P-EXT-4 | chaque capacité a un test de refus (sans droit → refus, sans effet) | tests `CAP-*` |
| P-EXT-5 | budgets du §15.4 | `PERF-EXT-*` |
| P-EXT-6 | décodeurs des sorties de greffon (propositions, vues, jetons) fuzzés | cibles `Fuzz*` Go, fast-check TS |
| P-EXT-7 | banc wazero (annexe, *Confirmation*) | `benchstat`, `-count ≥ 10` |

### 15.8 Thèmes : un contrat de jetons

- **TH-1** Un thème est une **donnée** : un sous-ensemble des jetons de
  [`docs/design/tokens.json`](../design/tokens.json) (couleurs, rayons, ombres, polices
  fournies, mouvement), pour chaque mode (`light`, `dark`, `hc-light`, `hc-dark`). Aucun CSS,
  aucun sélecteur, aucune URL : les valeurs sont typées (couleur sRGB, longueur bornée, nom de
  police de la liste fournie ou fichier `woff2` du paquet), et tout jeton absent garde la
  valeur d'Adwaita.
- **TH-2** Au chargement, l'hôte calcule **chaque paire** de `tokens.json#/contrast/pairs`
  avec la méthode déclarée (`WCAG 2.2`, luminance relative, translucides composés sur `over`) :
  texte ≥ 4,5:1 (SC 1.4.3), composants et anneau de focus ≥ 3:1 (SC 1.4.11) : ces seuils AA **refusent** un thème ; SC 2.4.13 (AAA) est visé et seulement signalé. Une
  paire en échec : le thème est refusé avec la liste des paires et leurs valeurs ; rien n'est
  appliqué à moitié. Les modes à contraste élevé ne peuvent pas être affaiblis.
- **TH-3** Les jeux d'icônes sont des SVG du paquet, servis par l'origine de l'application et
  affichés en `<img>` (jamais insérés dans le DOM), un par nom d'icône connu ; absent → icône
  d'Adwaita.
- **TH-4** Les couleurs des personnes (`person*`, conception p. 14) gardent leurs 8 teintes
  distinguables : un thème peut les ajuster, l'hôte vérifie leur distance (ΔE) en plus du
  contraste.

### 15.9 Cycle de vie et bus d'événements

Un seul contrat, versionné avec l'API (WIT `events`), gardé par capacités. Livraison
**asynchrone**, au plus une fois, dans l'ordre par source, **coalescée** ; file de 256
événements par abonné, l'excédent remplacé par un événement `lost{n}` ; un émetteur n'attend
jamais un abonné. Abonnement déclaré dans le manifeste (`onEvent:<nom>`), donc paresseux.

| Événement | Charge (jamais de contenu de document) | Capacité |
|---|---|---|
| `app.ready` | version, plate-forme | — |
| `app.stopping` | délai restant | — (greffons globaux seulement) |
| `project.opened`, `project.closed` | identifiant de projet | — |
| `document.opened`, `document.closed`, `document.activated` | nœud, type | `document.read` |
| `document.changed` | nœud, auteur, nombre de modifications ; ≤ 1 par 300 ms | `document.read` |
| `document.saved` | nœud, chemin relatif | `project.read` |
| `document.external-change` | nœud | `project.read` |
| `file.created`, `file.renamed`, `file.trashed`, `file.restored` | nœud, chemin relatif | `project.read` |
| `peer.joined`, `peer.left`, `people.changed` | personne (pseudonyme stable), rôle | `presence.read` ou `people.read` |
| `sync.state` | en ligne, hors ligne, à jour, en retard | `presence.read` |
| `compile.done` | durée, nombre de pages, nombre de diagnostics | `compile.read` |
| `export.done` | format, taille ; jamais le chemin | `export.write` |
| `version.named` | nom, auteur | `history.read` |
| `backup.state` | état du dorsal (§6.7) | `storage.backend` |
| `theme.changed`, `settings.changed` | mode ; clés de l'extension seulement | — |

**Démarrage** : aucune extension ne s'exécute avant que la fenêtre soit peinte et que le
dernier projet soit modifiable (budgets §2 inchangés) ; `app.ready` part ensuite, aux abonnés
en parallèle, chacun avec 2 s pour répondre. Aucun crochet « avant le démarrage ».

**Arrêt** (insère l'étape 5 bis dans §5.4) : le cœur rend **d'abord** ses données durables
(ledger, `store`, projection), **puis** envoie `app.stopping` aux greffons globaux abonnés, en
parallèle, avec **1 s** au total ; ensuite l'hôte des greffons reçoit SIGTERM puis SIGKILL. Un
crochet d'arrêt ne peut plus proposer de modification (les documents sont fermés) ; il peut
écrire dans son magasin privé (`storage`). L'arrêt ne dépend jamais d'un greffon.

**Isolation** : un greffon qui plante ou dépasse ses bornes est arrêté seul ; l'hôte relance
avec recul (C-7) ; le cœur et les autres greffons continuent.

**Budget prouvé** : le démarrage avec 50 extensions installées (dont toutes les intégrées) ne
coûte pas plus de **10 ms** avant la première peinture (lecture des manifestes, construction
du registre), mesuré par `BenchmarkRegistryBuild` (Go) et `PERF-EXT-START` (e2e), `benchstat`
et `-count ≥ 10` ; l'arrêt complet reste ≤ 5 s avec un greffon qui ne répond pas
(`PERF-EXT-STOP`).

### 15.10 Greffons de projet (N23)

*[cible ; dépend de PD-1, §15.19 ; `AD-69`]* Un greffon peut **appartenir à un projet** : il se
charge automatiquement chez chaque membre qui ouvre ce projet, sans installation globale. Le
précédent canonique de ce motif est la macro d'Office qui s'exécute à l'ouverture, au point que
Microsoft bloque depuis 2022 les macros des fichiers venus d'Internet. On l'accepte donc
**borné** : un admin du projet est de confiance (R7), **l'auteur du paquet qu'il ajoute ne l'est
pas**.

**Vérité.** La liste des greffons d'un projet est une **décision d'admin** du journal signé
(ADR 0030), espace de noms `plugin/<id>`. Cet espace **n'existe pas** dans le protocole accepté
(registre §20.5 fermé, aucune règle de tolérance) : il exige la mineure proposée PD-1 (§15.19).
**Tant qu'elle n'est pas acceptée, N23 n'est pas implémentable** ; rien n'est émis avant la
décision `format` correspondante. Les octets du paquet sont un blob du projet (ADR 0023), répliqué
par le vrac et vérifié avant tout chargement ; le paquet est **racine du ménage** des blobs tant que
la décision est debout (§6.5, M-6).

**Niveau « projet »** (chargé sans question) = **contribuer seulement** (classes I0–I2, O0,
§15.5) :

| Permis | Interdit sans accord local |
|---|---|
| commandes, entrées de menu, actions (non épinglées), panneaux `ui.view`, barre d'état | tout point au verbe **remplacer** (vue par défaut d'un type, thème, dorsal, exportateur) ou **envelopper** (collage, export) |
| types de fichiers **nouveaux** et leur vue ; une vue contribuée pour un type existant n'est proposée que comme **autre vue**, choisie par la personne | `viewers` pour un type du cœur en vue par défaut |
| coloration (`tokens`), diagnostics ; **décorations de marque seulement** : classe de style, soulignement, gouttière, info-bulle ; **aucun** `replace`, aucun widget qui recouvre ou masque du texte (vérifié par l'hôte, `EXT-ND2`) | décorations `replace`, widgets bloc, `inline-widgets` |
| `transforms` **seulement** par une commande invoquée par la personne | `transforms` en envelopper (collage) |
| `document.read`, `project.read`, `blob.read`, `compile.read` ; `storage` par projet ; événements du projet | `net.hosts`, `storage.backend`, `mirror.dir`, `export.write`, `import.read`, `ui.canvas`, `people.read`, `ai.complete`, `app.*` |
| `renderers` (`wasm`) : rendu **local** seulement | publication d'un rendu partagé (§15.14) |

Au-delà : accord local, une fois par version et par différence de capacités (motif *Workspace
Trust* de VS Code ; le greffon reste en mode restreint tant qu'il n'est pas accordé).

**Mandat** (PP-2, contrat d'autorisation). Une proposition de greffon de projet ne prend effet que
sous un **mandat** créé par l'hôte quand la personne invoque une commande du greffon :
`{compartiment, génération, projet, commande, documents visés (ceux de la commande), date,
expiration 30 s, propositions ≤ 1 par défaut (≤ 8 si le manifeste le déclare)}`. Chaque effet est
contrôlé contre le mandat ; le mandat est **révoqué** (et toute réponse en vol ignorée, IPC-5) à la
désactivation, à la mise à jour, à une nouvelle décision `plugin/<id>`, à la fermeture du projet, à
l'expiration. Aucune proposition d'un greffon de projet sur un événement, en arrière-plan, ou sous
un mandat d'une autre commande.

**Règles de sûreté** (toutes *à auditer*) :

- **PP-1** Isolé par projet : un hôte par projet ouvert (§9.10 H-4), distinct de ceux des
  greffons globaux ; aucun accès à un autre projet, aux fichiers de la personne, au réseau, aux
  clés ; déchargé à la fermeture du projet.
- **PP-2** Pas d'écriture au nom de la personne sans son geste : mandat ci-dessus ; la
  modification porte « via X (ajouté par Marie) ».
- **PP-3** Provenance visible : à la première ouverture, bandeau non bloquant « Ce projet utilise 2
  extensions ajoutées par Marie. **Voir** · **Désactiver pour moi** » ; la liste dit qui a ajouté
  quoi, quand, l'auteur et l'empreinte du paquet.
- **PP-4** Interrupteur : chacune peut désactiver un greffon de projet **pour elle**, à tout
  moment ; le projet reste lisible et modifiable sans lui (repli MF-14).
- **PP-5** Aucun déguisement : une vue de greffon est toujours encadrée de son nom et du badge
  « Extension du projet » ; jamais dans un emplacement de dialogue du cœur.
- **PP-6** Décisions concurrentes : deux versions décidées en même temps → décision `contested`
  (ADR 0030 D2) → aucune des deux chargée, carte aux admins. Les dépendances d'un greffon de projet
  (API 1.2, §15.12) sont portées par sa décision ; un graphe incomplet n'est pas chargé.
- **PP-7** Bornes : paquet ≤ 16 Mio, module ≤ 8 Mio décompressé, 16 greffons par projet, 64 Mio au
  total, comptés dans le quota N10 de l'admin qui les ajoute.

**Versions entre appareils** (N14, PV-6) : un appareil charge le greffon si l'intervalle d'API du
manifeste recoupe le sien ; sinon il l'indique (« Ce projet utilise une extension qui demande une
version plus récente d'Inkwell ») et garde le repli PP-4. Aucune promesse préalable sur ce que
peuvent charger les autres appareils (la version annoncée par un pair ne prouverait rien).

**Antécédents** : macros d'Office bloquées par la marque du Web (2022) ; *Workspace Trust* de VS
Code (mode restreint, capacité `untrustedWorkspaces`, qui ne peut pas empêcher une extension
malveillante d'ignorer le mode — d'où notre bac à sable) ; dossier `.obsidian/plugins` synchronisé
(code arbitraire sans bac à sable : le contre-exemple) ; paquets Typst qui voyagent avec le projet
(purs, sans pouvoir : le modèle) ; carnets Jupyter « de confiance » (signature HMAC locale ; sortie
non fiable jamais exécutée) ; greffons d'organisation de Figma (liste approuvée par un admin,
exécutée dans un bac à sable).

### 15.11 Ne pas dénaturer l'application (N24)

Une extension **ajoute, décore ou enveloppe** ; elle n'enlève ni ne cache jamais ce qui fait
l'identité et la sûreté d'inkwell. Chaque règle a sa preuve (engineering-principles.md).

**Zones protégées** : barre d'en-tête (nom du projet, pile des personnes présentes, état de
synchronisation et d'enregistrement, « Partager »), qui modifie quel fichier, bandeaux et
dialogues de confiance et de droits, dialogues d'accord, Annuler et Rétablir, panneau
Personnes, gestionnaire d'extensions et entrée « Redémarrer sans extensions ».

| # | Invariant | Mécanisme | Preuve |
|---|---|---|---|
| ND-1 | Aucune extension ne retire, ne cache, ne recouvre ni ne remplace une zone protégée | le registre n'a **aucune** opération de retrait ; le verbe « remplacer » ne s'applique à aucun emplacement protégé ; les vues de greffon vivent dans leur emplacement, sous la coque dans l'ordre d'empilement | test d'API `EXT-ND1-api` (toute contribution visant une zone protégée est refusée) ; e2e `EXT-ND1` : un greffon hostile essaie chaque API, chaque zone reste visible (DOM et capture) |
| ND-2 | Aucune imitation de l'interface du système | vues rendues par l'hôte depuis un arbre déclaratif, toujours encadrées du nom et de l'icône de l'extension (badge « Extension du projet » pour PP-5) ; aucun accès aux composants de dialogue d'accord ni aux champs de mot de passe ; pas de superposition | propriété fast-check `EXT-ND2` : aucun arbre généré ne produit un élément hors de son conteneur ni sans cadre ; aucune décoration d'un greffon de projet ne remplace, ne recouvre ni ne masque un caractère du document (B-4) ; capture : un dialogue de greffon se distingue d'un dialogue d'accord |
| ND-3 | Aucun raccourci du cœur pris en silence | raccourcis déclarés dans le manifeste ; liste réservée du cœur ; conflit → raccourci de l'extension inactif, signalé dans « Extensions » et dans l'éditeur de raccourcis, la personne tranche | `EXT-ND3` (unitaire et e2e) |
| ND-4 | Les principes UX et le glossaire valent pour les extensions | contrôle à l'hôte au chargement : chaînes fr et en présentes, longueurs par emplacement (titre de commande ≤ 40 caractères, info-bulle ≤ 80, toast ≤ 120), aucun terme « Ne jamais dire » du glossaire, couleurs par jetons seulement, icônes d'Adwaita ou du paquet | `EXT-ND4` ; même garde que l'i18n du cœur (§2) |
| ND-5 | Une extension lente ne ralentit pas l'application | budgets du §15.4 mesurés par appel ; au-delà : cadence réduite, puis mise en pause avec le message « X est trop lent et a été mis en pause » ; aucun travail d'extension tierce sur le fil d'affichage ; vue ≤ 2 000 nœuds, ≤ 10 mises à jour/s ; mémoire plafonnée | `PERF-EXT-*`, `EXT-ND5` (greffon lent de test : ralenti puis mis en pause, frappe toujours ≤ 16 ms) |
| ND-6 | Tout désactiver rend l'application d'origine | **mode sans extensions** : `inkwell --safe-mode`, menu « Redémarrer sans extensions », proposé d'office après deux démarrages interrompus (marqueur d'exécution, `AD-12`) ; aucun greffon global ni de projet chargé, extensions intégrées telles que livrées | e2e `EXT-ND6` : avec des greffons hostiles installés, le mode sans extensions donne le même registre et les mêmes captures qu'une installation neuve |
| ND-7 | Aucun document ne devient illisible sans son extension | contenu toujours standard (texte, blobs) ; aucun format propre à une extension dans le CRDT en v1 (`AD-66`) ; repli MF-14 | `EXT-ND7` : projet de test, greffons de projet désactivés, chaque fichier s'ouvre |
| ND-8 | Annuler reste maître | toute proposition appliquée = **un** pas d'annulation, nommé ; Ctrl+Z l'annule toujours | `EXT-ND8` |

### 15.12 Au-delà du catalogue : retenu, différé, écarté

| Proposition | Verdict | Raison et conditions |
|---|---|---|
| Points d'extension **déclarés par un greffon**, que d'autres étendent (graphe de dépendances, intervalles SemVer) | **retenu, API 1.2** | modèle d'Eclipse et des dépendances d'extensions de VS Code ; points nommés `<id>/<point>` décrits en WIT dans le paquet ; résolution à l'installation et au chargement d'un projet : cycle, conflit d'intervalles, profondeur > 4 ou > 32 dépendances → refus avec la raison ; appels entre greffons **par l'hôte** seulement ; la règle de flux (§15.5) s'applique à **toute la composante** du graphe d'appel (pas d'adjoint complaisant : un greffon sans réseau ne peut pas faire sortir des données par un greffon qui en a) ; un greffon de projet ne dépend que de greffons du même projet ou d'extensions intégrées |
| Réglages par schéma, rendus par l'hôte | **retenu, API 1.0** | point `settings` (§15.4) : sous-ensemble de JSON Schema (booléen, énumération, nombre borné, texte borné) ; valeurs dans les réglages de l'appareil, espace de noms de l'extension |
| i18n des extensions | **retenu, API 1.0** | tables fr et en dans le manifeste, contrôlées par ND-4 ; langue absente → l'autre, signalée |
| Commandes composées en macros | **retenu, API 1.1** | une macro est une **donnée** (suite de commandes et d'arguments), écrite ou enregistrée par la personne, dans ses réglages ; chaque pas repasse les capacités ; lancer la macro est le geste (PP-2) |
| Automatisations déclenchées par un événement (« formater à l'enregistrement ») | **retenu, borné** | seulement celles que la personne crée elle-même, pour des greffons globaux ; jamais proposées ni lancées par un greffon de projet (PP-2 : c'est la macro d'Office) |
| Transformations de document | **retenu** | point `transforms` (§15.4), par proposition |
| Étapes du pipeline de compilation (avant `typst`) | **écarté** | X-8 : le PDF doit être le même chez tous ; un appareil sans le greffon compilerait autre chose. Remplacé par : paquets Typst fournis par le projet (greffons Typst compris), étapes **après** compilation (`preview-overlays`, `exporters`) |
| Langages fournis par un greffon, contrat proche de LSP | **retenu, API 1.2** | point `language-server` : sous-ensemble de LSP 3.17 (complétion, survol, diagnostics, définition, symboles, formatage) en messages WIT, adapté à CodeMirror par l'hôte comme tinymist (`AD-20`) ; le serveur est un module WebAssembly (aucun processus lancé) |
| Importateurs, exportateurs, stockage, thèmes, cycle de vie, greffons de projet | **retenus** | §15.4, §6.7, §15.8–§15.10 |
| Rechargement à chaud pendant le développement | **retenu, mode développeur** | réglage « Mode développeur » : greffon non empaqueté lu dans un dossier choisi, rechargé à chaque modification ; non signé permis **seulement** là, avec un bandeau rouge permanent ; jamais pour un greffon de projet |
| Kit de développement et modèles par langage | **retenu** | générés depuis le WIT : Rust et Go (`wasip1`, TinyGo) d'abord, AssemblyScript ensuite ; outil `inkwell plugin new | pack | sign | test` (sans fenêtre, compatible `AD-35`) |
| Kit de conformité des greffons | **retenu** | comme les vecteurs du protocole : jeux d'entrées de l'hôte et réponses attendues (forme, bornes, déterminisme : même entrée → même sortie), budgets mesurés ; `inkwell plugin test` le lance ; exigé pour un futur catalogue ; l'hôte passe les mêmes vecteurs en CI |
| Accès au DOM ou au CSS de l'application | **écarté** | leçons d'Obsidian et de Firefox (XUL) : interdit la refonte et toute sécurité ; ND-1, ND-2 |
| Données propres à une extension dans le document partagé (`ext/<id>`) | **différé** (`AD-66`) | exige une règle de garde et des bornes dans le protocole ; v1 : magasin privé et contenu standard |

### 15.13 Surfaces d'interface : ajouter, modifier, ne pas toucher

Forme unique, reprise de l'existant : une **contribution de menu** = `{commande, emplacement,
groupe, ordre, quand}`, comme les `menus` de VS Code (identifiants d'emplacement, `group`
`"nom@ordre"`, clauses `when`), les `menuContributions` d'Eclipse (`locationURI`
`menu:…?after=…`), les groupes d'actions de JetBrains (`add-to-group`, ancre `before`/`after`)
et le couple GMenu/GAction de GNOME (sections ordonnées qui désignent des actions par nom).
Les éléments ne contiennent pas de code : ils désignent une commande (§15.4). L'ordre se
résout toujours de la même façon : **groupes du cœur à leurs places fixes** ; à l'intérieur
d'un groupe, `ordre` puis identifiant de l'extension (stable) ; un groupe propre à une
extension se place après ceux du cœur ; la personne peut réordonner ou masquer un élément
d'extension (jamais un élément protégé, ND-1). `quand` est une expression bornée sur un
contexte fermé (`fileType`, `selection`, `role`, `online`, `focus`…), évaluée par l'hôte sans
appeler l'extension.

| Surface | Ajouter | Modifier (insérer, réordonner, décorer) | Intouchable | Conflits, ordre |
|---|---|---|---|---|
| Nouvelle fenêtre | oui : fenêtre Wails v3 rendue par l'hôte depuis un arbre déclaratif (`ui.view`) ; non modale ; ouverte seulement sur un geste ; ≤ 2 par extension, ≤ 6 en tout ; fermée quand l'extension est désactivée, mise en pause ou retirée ; taille et place gardées | — | fenêtres du cœur (principale, public de présentation) | aucune ne prend le focus sans geste |
| Dialogue | oui, **dans** la fenêtre, `<dialog>` encadré et attribué (ND-2) ; modal seulement s'il répond à un geste | — | dialogues d'accord, de droits, d'invitation | un seul dialogue d'extension à la fois ; file sinon |
| Menu de l'application (en-tête) | dans les groupes `extensions`, `insert`, `export`, `view` | ordre dans son groupe | groupes `project`, `people`, `about`, « Extensions », « Redémarrer sans extensions » | règle ci-dessus |
| Menus contextuels : sélection de l'éditeur, fichier ou dossier de la liste, onglet, aperçu, ligne d'une personne, lien | oui, emplacements `editor/context`, `files/context`, `tabs/context`, `preview/context`, `people/context`, `link/context` | ordre dans son groupe | entrées de droits et de confiance du menu d'une personne ; « Supprimer », « Restaurer » | idem ; ≤ 8 entrées d'extensions par menu, au-delà sous-menu « Extensions » |
| Popovers | oui, attachés à un bouton d'extension | — | popovers de la coque (personnes, partage, état) | — |
| Bouton d'extension (zone dédiée, comme le bouton « Extensions » unifié de Firefox depuis la 109) | une **action** par extension : icône, libellé, info-bulle, badge (texte ≤ 4 caractères, couleur parmi les jetons d'état, ≤ 1 changement par seconde), clic → commande ou popover déclaratif | rangée dans le menu « Extensions » (icône pièce de puzzle de la barre d'en-tête) ; **seule la personne** l'épingle dans la barre d'en-tête, ou la désépingle ; ordre des épinglées choisi par glisser | une extension ne peut ni s'épingler, ni se placer hors de la zone (ND-1) | épinglage global pour un greffon global ; **par projet** pour un greffon de projet (N23), gardé dans les réglages de l'appareil sous l'identifiant du projet, jamais imposé par l'admin ; ≤ 6 épinglées, au-delà le menu |
| Barre d'outils par type | oui (groupes par type) | ordre dans le groupe | Annuler, Rétablir, état d'enregistrement | débordement dans « Plus » |
| Panneaux et onglets du tiroir, panneau de gauche | oui | ordre des onglets d'extension | onglets Personnes ; Fichiers comme panneau par défaut | ordre choisi par la personne |
| Barre d'état | oui (côté droit, groupe `extensions`) | ordre | état de synchronisation et d'enregistrement, présence | ≤ 3 éléments d'extension visibles, les autres dans un débordement |
| Écran d'accueil | oui : modèles de projet, cartes « Démarrer avec » | ordre | « Nouveau projet », « Ouvrir », « Rejoindre » | ≤ 4 cartes d'extensions |
| Préférences | oui, une page par extension (schéma, §15.12) | — | pages Réseau, Confidentialité, Extensions | ordre alphabétique |
| Gouttières de l'éditeur | oui, une colonne par extension (`gutters`) | — | bande des modifications et curseurs des personnes (U11) | à droite des gouttières du cœur ; ≤ 2 colonnes d'extensions |
| Widgets en ligne, décorations, soulignements | oui (`decorations`, `inline-widgets`) | — | marques des autres personnes, zones verrouillées (ADR 0022), conflits | précédence CodeMirror : cœur `Prec.high`, extensions `Prec.default`, dans un `Compartment` par extension |
| Info-bulles, cartes de survol | oui (`hover`) | — | carte d'une personne, carte de conflit | plusieurs fournisseurs : sections empilées, ordre par extension |
| Notifications : toast, bandeau | oui | — | bandeaux de confiance, de disque plein, de liste scellée | ≤ 1 toast par extension toutes les 10 s ; bandeau seulement pour une action requise, ≤ 1 à la fois ; jamais de question dans un toast (design TO-2) |
| Cibles de glisser-déposer | oui : types de données acceptés sur l'éditeur ou la liste des fichiers | — | glisser d'un fichier dans le projet (cœur) | un seul gestionnaire choisi par la personne si plusieurs acceptent |
| Raccourcis | oui, déclarés | — | raccourcis réservés du cœur (ND-3) | conflit montré, la personne tranche |
| Palette de commandes | toutes les commandes y entrent d'office | — | — | tri par usage récent, puis titre |

### 15.14 Cas de référence : des greffons qui dessinent des diagrammes

*[cible ; `AD-70`]* Mermaid, Graphviz (DOT), D2, Vega-Lite, PlantUML : le parcours de bout en bout
valide le modèle et ajoute un point d'extension, `renderers`. Il **complète** le chemin de la
conception (p. 28), qui reste le premier : diagrammes Typst natifs (fletcher, CeTZ), purs et
déterministes, et SVG importés (draw.io, Excalidraw).

**1. Contrat de rendu, pur.** `render(langage, source, options) → {svg, description} |
diagnostics` : entrée de classe I2 (§15.5), aucune autre capacité, aucune horloge, aucun réseau,
sortie bornée ; déterministe à version fixée ; **polices** : seulement celles du paquet ou celles
que le projet fournit à Typst (`--font-path`), pour que les mesures de texte du rendu soient celles
du PDF. `description` = texte alternatif court (exigé, ND-4), repris par `image(alt: …)`.

**2. Où il tourne.** Un seul hôte en v1 : `wasm` (hôte des greffons, §9.10). Cas de référence
**livré** : Graphviz (le paquet Typst `diagraph` l'embarque déjà en greffon Typst ; Emscripten) ;
D2 (écrit en Go, construit en WebAssembly par D2.js, encore avant stable) en exemple.

**Mermaid : différé.** Il mesure le texte par le moteur d'un navigateur ; ses mainteneurs disent
qu'un rendu sans navigateur n'est pas possible (ticket mermaid #3886). Un cadre `iframe` dans la
webview ferait tourner du JavaScript tiers dans le **processus web qui tient le pont et la demande
de signature** (contraire à AD-57) ; l'origine opaque ne protège ni d'un défaut du moteur, ni d'une
boucle infinie qui fige l'éditeur, ni à coup sûr d'une navigation qui porte la source. Conditions
pour le rouvrir (porte **P-FRAME**, prototype sur WebKitGTK) : un processus web **dédié**, sans
liaisons ni Streams, tuable seul, réseau et navigation refusés par l'hôte (politique de navigation
de WebKitGTK, pas seulement la CSP), canal `MessagePort` par instance avec identifiants et tailles
bornées ; preuves : boucle infinie tuée sans gêner la frappe, tentative de navigation avec une
source canari jamais vue hors de l'appareil. Même alors : capacité nommée `render.frame`, **hors**
du niveau « projet », installée par geste. Alternative suivie : un moteur Mermaid sans DOM
(sous-ensemble) ou une version WebAssembly, s'il en paraît une.

**3. Assainir avant tout affichage.** Tout SVG produit par une extension passe par l'assainisseur
de Go (lecture XML en flux, liste blanche d'éléments et d'attributs : ni `script`, ni
`foreignObject`, ni gestionnaire `on*`, ni lien externe — seulement `#fragment` —, `style` sans
`url()` externe ni `@import` ; ≤ 4 Mio, ≤ 50 000 éléments, profondeur ≤ 64 ; cible `Fuzz*`).
Affichage en `<img>` seulement (B-7). Un SVG reçu d'un pair est réassaini avant affichage.

**4. Jusqu'au PDF, sans toucher au protocole.** Typst n'inclut que des images. Le rendu devient un
**fichier ordinaire du projet** : un nœud `blob` de l'index sous `.renders/<clé>.svg` (ADR 0023,
schéma existant), `clé = hex(BLAKE3(greffon@version ‖ empreinte du paquet ‖ options ‖
source))[:32]` ; plus un document texte ordinaire `.renders/index.json` (source normalisée → clé,
description). Aucun champ nouveau dans l'index, aucune règle de garde nouvelle : la validation se
fait **à l'interprétation** (lecture stricte et bornée de `index.json`, SVG réassaini) ; c'est le
choix demandé, justifié par « ne pas toucher au protocole accepté pour une vue ». Un paquet Typst
fourni, `inkwell-diagrams` (copié dans `.typstdeps/` à la première insertion), contient la règle
`show raw.where(lang: "dot")` (et `d2`…) qui lit `index.json` et appelle `image(..., alt: …)`. Même
chemin pour le Markdown (`cmarker` passe les blocs de code à Typst) et l'export HTML. X-8 tient : le
greffon n'entre pas dans la compilation, son **résultat** est un fichier du projet.

**5. Qui publie.** Publier = écrire dans le projet (blob, nœud, `index.json`), donc une
modification attribuée. Permis : à l'extension intégrée « Diagrammes » quand la personne vient de
modifier la source du bloc (« via Diagrammes ») ; à un greffon **installé par la personne** qui a
reçu `document.create` ; jamais à un greffon de projet sans accord (§15.10 : rendu local seulement).
Une personne sans le greffon voit le dernier rendu partagé, dans l'aperçu et dans le PDF (ND-7).
Deux rendus concurrents d'une même clé : homonymes résolus par l'index (ADR 0023), le même partout.
Rendus orphelins (clé absente d'`index.json`) : retirés par la commande « Nettoyer les rendus »
(écrivain ; en mode `approval`, par décision d'admin) ; leur taille compte dans le quota N10. Un
rendu qui ne correspond pas à sa source se voit chez qui a le greffon (« Le dessin partagé ne
correspond pas à sa source — Redessiner ») ; même risque qu'une image trompeuse, attribuée.

**6. Performance.** Rendu 500 ms après la dernière frappe dans le bloc, hors du fil d'affichage, le
plus récent annule le précédent ; cache local LRU par clé puis cache partagé (blob) ; p95 ≤ 300 ms
pour ≤ 200 nœuds, plafond 5 s (« Diagramme trop complexe ») ; remplacement de l'image ≤ 16 ms.

**7. Édition.** Markdown : bloc ` ```dot ` ; Typst : bloc brut ` ```dot ` (même règle `show`).
Coloration par le registre (N20). Aperçu en ligne sous le bloc (`inline-widgets`, extension
intégrée) et dans l'aperçu Typst. « Insérer › Diagramme » propose les langages des moteurs
présents ; la vue accessible de l'aperçu montre `description` (§15.17).

### 15.15 Expérience des développeuses : une API claire et pratique

L'expérience de développement est un attribut de premier rang de l'extensibilité (`AD-71`).
Grille : *Google API Design Guide* et AIP (nommage cohérent, méthodes standard, modèle
d'erreur unique — AIP-193 —, opérations longues — AIP-151 —), documentation de l'API des
extensions de VS Code (événements d'activation, points de contribution, guides par cas), et
listes de contrôle façon *Rust API Guidelines* (nommage, prévisibilité, documentation,
interopérabilité) pour chaque kit.

**Une seule API, petite et régulière.**

- **DX-1** Une seule source : le WIT `inkwell:plugin@1.x`. On en **génère** la référence
  (site et aide hors ligne dans l'application), les liaisons Go, TypeScript, Rust et
  AssemblyScript, le schéma JSON du manifeste et les types de l'hôte. Aucune documentation
  écrite à la main ne décrit une signature : elle ne peut pas diverger du code. Porte :
  100 % des éléments WIT documentés (`mise run api:doc`).
- **DX-2** Nommage : `ressource.verbe` (`document.read`, `document.propose`, `panel.open`,
  `command.register`) ; mêmes verbes partout (`get`, `list`, `create`, `update`, `delete`,
  `open`, `close`, `propose`, `render`) ; identifiants en `kebab-case` dans le WIT, convertis
  par chaque kit à l'usage du langage.
- **DX-3** Un seul modèle d'erreur : `error { code, message, field?, capability?, hint }`, codes
  fermés pris d'AIP-193 (`invalid-argument`, `not-found`, `permission-denied`,
  `failed-precondition` — par exemple base périmée d'une proposition —, `resource-exhausted`,
  `deadline-exceeded`, `cancelled`, `unavailable`, `internal`). Chaque erreur nomme le champ
  du manifeste ou la capacité en cause et dit quoi faire (« Ajoutez `document.read` à
  `capabilities` dans inkwell-plugin.json »).
- **DX-4** Un seul modèle d'asynchronisme : en API 1, chaque appel hôte est bloquant du point
  de vue du greffon, borné en temps, annulable (`cancelled()`) ; ce qui dure (export, rendu,
  stockage) est une **opération** avec progression et annulation (AIP-151). En API 2
  (composants WASI 0.3), les mêmes fonctions deviennent `async` sans changer de nom.
- **DX-5** Surface mesurée, **par étape** (§15.21) : API 1.0 (13 interfaces) ≤ **60 fonctions** et ≤ **45 types** (arbre de vues : 12 composants comptés comme un type variant) ; 1.1 ≤ 85 et ≤ 60 ; 1.2 ≤ 110 et ≤ 75 ; la porte `mise run api:surface` compte depuis le WIT et échoue sur toute croissance non revue au-delà du plafond de l'étape.
- **DX-6** Manifeste `inkwell-plugin.json` avec schéma JSON publié (`$schema`) : complétion et
  contrôle dans tout éditeur ; capacités en mots simples (`"capabilities": ["lire les
  documents"]` accepté comme alias de `document.read`, normalisé à l'empaquetage). Les
  contributions déclarées par annotations dans le code sont **fusionnées** dans le manifeste par
  `inkwell plugin pack` : une seule écriture, pas de doublon à tenir.
- **DX-7** Stabilité : SemVer de l'API ; dépréciation annoncée une mineure à l'avance, retrait
  seulement à la majeure suivante, N et N−1 pris en charge 12 mois (comme N14) ; **matrice de
  compatibilité** (API des greffons × versions d'inkwell) générée en CI à partir des vecteurs
  du kit de conformité, publiée avec chaque version.

**Bonjour en dix lignes, en cinq minutes.** `inkwell plugin new hello --lang rust` produit le
manifeste, ce code et une tâche de construction :

```rust
use inkwell_plugin::prelude::*;

#[command(title = "hello.title")]          // clé ; textes fr et en dans strings.toml
fn hello(ctx: &Context) -> Result<()> {
    ctx.toast(t!("hello.message"))         // « Bonjour ! » / « Hello! »
}

export_plugin!(hello);
```

`strings.toml` (créé par le modèle) porte les deux langues exigées par ND-4 ; un greffon en
développement peut n'en avoir qu'une, repli signalé dans l'Inspecteur.

Langages, par étape : **API 1.0 : Rust** (`wasm32-wasip1`, `cdylib`) et **Go** (Go ≥ 1.24, `GOOS=wasip1`, `-buildmode=c-shared`, exports `//go:wasmexport` : module « réacteur ») ; **1.1** : TypeScript/JavaScript (moteur JavaScript compilé en WebAssembly, à la manière de Javy) et AssemblyScript ; tout langage qui produit un module réacteur `wasm32` conforme à l'ABI (§15.20) fonctionne sans kit. Chaque kit offre des aides pour les cas courants : inscrire une commande ; ajouter
une entrée de menu ou une action (§15.13) ; afficher un panneau déclaratif ; proposer une
modification ; lire un fichier du projet ; rendre un bloc de code (diagramme, §15.14).

**Outils.** `inkwell plugin new | dev | pack | sign | test` (sans fenêtre) ; mode développeur
avec rechargement à chaud (§15.12) ; panneau **Inspecteur d'extensions** (journal du greffon,
appels, temps, mémoire, refus de capacité, événements reçus) ; kit de conformité local
(§15.12).

**Mode débogage (N25).** Faisable aujourd'hui, vérifié le 2026-11-01 :

| Besoin | Ce qui existe | Choix |
|---|---|---|
| Pile symbolisée sur un piège | wazero : `WithDebugInfoEnabled` (vrai par défaut) donne des piles DWARF si le module garde ses sections DWARF ; aucun débogueur ni point d'arrêt documenté | piles DWARF dans l'hôte en mode débogage (les kits gardent DWARF en construction de développement) |
| Points d'arrêt, pas à pas | **pas dans wazero.** Wasmtime : `-D debug-info` sous gdb ou lldb ; débogage de l'invité seul par `gdbstub` (`wasmtime run -g <port>`, puis LLDB `process connect --plugin wasm`), inclus dans les binaires publiés | **rejeu** d'une session enregistrée (ci-dessous) dans un outil de développement : (a) `inkwell plugin replay --debug` lance le Wasmtime de la développeuse avec `gdbstub` et un module de rejeu qui fournit les fonctions hôte depuis l'enregistrement (`--preload`, *à prouver par prototype*) ; LLDB, ou `lldb-dap` dans un éditeur (DAP) ; (b) cible **native** du kit : le même code compilé pour la machine, l'hôte simulé par l'enregistrement, débogué par `dlv`, `rust-gdb` ou l'éditeur. Wasmtime n'est jamais embarqué dans inkwell |
| Extensions côté webview (intégrées TypeScript, cadre de rendu §15.14) | inspecteur Web de WebKitGTK | ouvert seulement dans une construction de développement ou en mode débogage |

Toujours disponibles en mode débogage, dans l'**Inspecteur d'extensions** : piles
symbolisées ; console du journal par greffon ; trace de **chaque** appel hôte (fonction,
arguments résumés, durée, résultat ou erreur) ; refus de capacité expliqués (capacité
manquante, champ du manifeste à modifier) ; temps par point d'extension comparé à son budget
(§15.4, ND-5) ; jauge mémoire ; inspecteur du bus d'événements (§15.9 : abonnements, files,
pertes).

**Enregistrer et rejouer.** En mode débogage, l'hôte enregistre les entrées de chaque appel du
greffon et les réponses de chaque fonction hôte (CBOR, avec `now` et `random`, qui sont des
fonctions hôte) dans un anneau ≤ 64 Mio sous le dossier du greffon. Comme un greffon n'a aucune
autre source de non-déterminisme, le kit de conformité rejoue la session à l'identique
(`inkwell plugin test --replay <trace>`), puis en fait un cas de test. L'enregistrement
contient des données de documents : il reste local, n'entre jamais dans un rapport de bug
d'office (O-2), et peut être expurgé (`--redact`) avant d'être partagé.

**Sûreté.** Mode débogage activé **par greffon**, sur cet appareil, dans le mode développeur ;
**jamais** pour un greffon de projet reçu d'un pair ni pour un greffon signé installé tel quel
(il faut une copie de développement) ; bandeau permanent « Débogage de X » ; sans effet sur les
autres personnes (ce que le greffon propose passe par les mêmes règles).

**Exemples qui portent la documentation** (chacun complet, testé en CI, dans les langages de l'étape, §15.21) :

| Exemple | Points utilisés | Lignes de code visées (hors manifeste) |
|---|---|---|
| Bonjour | commande, toast | ≤ 10 |
| Diagrammes Graphviz (et D2) | `renderers`, `inline-widgets`, coloration | ≤ 40 par moteur (le moteur lui-même mis à part) |
| Graphique depuis un CSV | `transforms` (insère un bloc Vega-Lite), `renderers`, menu contextuel du tableur | ≤ 60 |
| Thème | `themes` (données seulement) | 0 |
| Stockage distant dans un nuage | `storage-backend`, réglages, `net.hosts` | ≤ 150 |
| Greffon de projet | commande, panneau, `document.propose` | ≤ 30 en plus de sa fonction |

**Mesures** (objectives, revues à chaque mineure) :

| Mesure | Cible | Comment |
|---|---|---|
| Temps jusqu'au premier greffon (de `new` au bouton qui marche dans inkwell) | ≤ 5 min par langage | tâche chronométrée par trois personnes qui découvrent inkwell, comme les tests en personne de l'interface ([ux-principles](../ux-principles.md)) |
| Lignes des exemples | tableau ci-dessus | compteur en CI sur `examples/` |
| Surface de l'API | ≤ 60 fonctions, ≤ 40 types | `mise run api:surface` |
| Erreurs actionnables | 100 % nomment champ ou capacité | test sur le catalogue d'erreurs du WIT |
| Documentation | 100 % des éléments documentés et un exemple par point d'extension | `mise run api:doc` |

### 15.16 Le protocole des greffons : frontière, versions, spécification à venir (N26)

Le contrat entre l'hôte et un greffon est un **protocole**, traité comme celui du réseau
(ADR 0034) : une spécification normative à part, `docs/plugin-api/`, en langage BCP 14, avec
vecteurs et kit de conformité. Elle sera rédigée ensuite et passera sa propre boucle de revue
(Fable + Codex) ; ce document en fixe la frontière, les décisions et le plan (`AD-72`).

**Frontière.** Normatif dans `docs/plugin-api/` : tout octet et tout champ qui traversent la
frontière hôte ↔ greffon (manifeste, paquet, ABI, messages, erreurs, capacités, événements,
vues déclaratives, propositions). Hors de cette spécification : l'implémentation de l'hôte
(`internal/plugin`, `src/extensions`), régie par ce document.

**Deux axes de version, indépendants.**

| Axe | Porté par | Règle |
|---|---|---|
| version de l'**API des greffons** | `inkwell:plugin@MAJ.MIN.CORR` (WIT, ABI, manifeste) | SemVer ; l'hôte annonce l'intervalle qu'il parle (par exemple `1.0`–`1.4` et `2.0`–`2.1`) |
| version **du greffon** | `version` du manifeste | SemVer, monotone pour un même `id` et une même clé de signature |

- **PV-1** Le manifeste déclare l'API voulue par un intervalle (`"api": "^1.2"`). Hors de
  l'intervalle de l'hôte, le greffon est **refusé** avec un message qui nomme les deux versions
  et l'action (« Cette extension demande l'API 1.5 ; Inkwell 1.3 parle 1.0 à 1.4. Mettez Inkwell
  à jour. »).
- **PV-2** Une mineure n'**ajoute** que : fonctions, champs optionnels, valeurs d'énumération
  marquées extensibles, événements, points, capacités. Rien n'est retiré ni ne change de sens
  dans une majeure.
- **PV-3** Ce qui est optionnel se **détecte à l'exécution**, pas par comparaison de versions :
  `host.features()` rend l'ensemble des fonctions offertes (et les capacités accordées) ; un kit
  expose `if host.has("renderers.inline-widget")`. Une fonction absente rend `unimplemented`
  (code d'erreur stable), jamais un plantage.
- **PV-4** Une majeure garde une fenêtre de soutien : N et N−1 pendant au moins 12 mois après la
  sortie de N ; N−1 passe par un **adaptateur** (cales qui traduisent l'ancienne ABI vers la
  nouvelle) ; dépréciations annoncées avec leur date de retrait dans la référence générée et dans
  l'Inspecteur ; une **matrice de compatibilité** (API × versions d'inkwell × vecteurs) est
  publiée avec chaque version (DX-7).
- **PV-5** Tolérance de l'inconnu, là où elle est sûre : champ inconnu du manifeste ou d'un
  message → ignoré ; champ marqué « à comprendre » (`critical`) ou **capacité inconnue** → refus.
  Valeurs « graisse » (GREASE, RFC 8701, leçons de RFC 9413) **seulement là où une liste est
  négociée** : `host.features()` contient une fonction de graisse que les kits doivent ignorer ; le
  kit de conformité en injecte dans les champs optionnels des messages. Pas de graisse ailleurs (un
  seul hôte, pas d'écosystème d'hôtes) ni là où elle changerait une décision de sécurité.
- **PV-6** Greffons de projet et appareils de versions différentes : un appareil dont l'hôte ne
  parle pas l'API demandée ne le charge pas, l'indique (« Ce projet utilise une extension qui
  demande une version plus récente d'Inkwell ») et garde le repli ND-7 (dernier rendu partagé,
  texte, carte). Aucune vérification préalable des autres appareils : le HELLO du protocole ne porte
  que les majeures du protocole (§5.3), et une version annoncée par un pair ne prouverait rien.
- **PV-7** Interprétation unique des intervalles : `"api": "^1.2"` = ≥ 1.2.0 et < 2.0.0 (SemVer
  2.0.0, sémantique npm du `^`) ; l'hôte publie ses intervalles parlés (`1.0`–`1.4`, et `2.0`–`2.1`
  quand la 2 existe) ; un greffon `^1.y` est servi par l'adaptateur tant que N−1 est soutenue.
**Plan de `docs/plugin-api/`** (à écrire) :

1. Conventions (BCP 14, notation, décodage strict) ; 2. Versions et évolution (PV-1 à PV-6,
   politique de soutien, matrice) ; 3. Paquet (`.inkwell-plugin` : zip, bornes, signature,
   continuité de clé) ; 4. Manifeste (schéma JSON normatif, champs, contributions, activation,
   alias des capacités en mots simples) ; 5. ABI (exports, imports, mémoire, `alloc`, appels,
   annulation, imports WASI tolérés) ; 6. Encodage des messages (CBOR à émission déterministe, RFC 8949 §4.2.1, décodage strict selon le
   protocole, §9.2) ; 7. Modèle d'erreur (codes, champs) ; 8. Capacités (noms, portée, règle
   de flux, niveaux « projet » et accord) ; 9. Interfaces WIT par point d'extension (§15.4,
   §15.13, §15.14) ; 10. Événements (§15.9) ; 11. Vues déclaratives (composants, propriétés,
   bornes, accessibilité) ; 12. Propositions de modification (ancres, base, bornes) ;
   13. Contrat de rendu et assainissement SVG ; 14. Dorsal de stockage (§6.7) ; 15. Thèmes
   (contrat de jetons, contrôle des contrastes) ; 16. Bornes (table) ; 17. Sécurité et vie privée ;
   18. Canal Go ↔ hôte des greffons (§7.3, interne mais vectorisé) ; deltas du protocole (PD-*, §15.19) ;
   19. Conformité : vecteurs JSON (`enforce` par version comme le protocole), exécuteur Go
   (hôte) et kit des greffons (`inkwell plugin test`) ; 20. Registres (codes, capacités,
   événements, emplacements de menu, composants).

### 15.17 Outils d'écriture : erreurs, aide à la saisie, confort (N28, N29, N30)

*[cible ; `AD-79`, `AD-80`, `AD-81`]* Études : [research/editor-features.md](research/editor-features.md)
(§1–§7 : tinymist et outils d'écriture ; §8–§11 : addendum N30). Tout est livré en **extensions
intégrées** sur les points du §15.4 (dogfooding) ; rien n'est un outil de développement : les
libellés suivent le glossaire.

**Service de langage Typst (N29, tinymist).** Remplace et précise `AD-20` :

- **LSP-1** `tinymist lsp` sur stdio, enfermé comme un sidecar (§9.5 : lecture du projet et des
  paquets fournis seulement, pas de réseau, mémoire bornée) ; relayé par le Stream `lsp` ; liste
  blanche des méthodes (complétion, aide de signature, survol, définition, références, symboles du
  document et de l'espace, liens, diagnostics, actions de code, formatage, renommage) ;
  `workspace/executeCommand` refusé sauf commandes listées. Hors ligne : chemin des paquets réglé
  sur les paquets fournis et `.typstdeps/` ; un test d'intégration en espace de noms réseau vide
  prouve qu'aucun accès n'est tenté.
- **LSP-2** Espace de travail multi-fichiers : racine = dossier du projet ; les documents **ouverts**
  arrivent par `didOpen`/`didChange` incrémentaux **depuis la webview** (le texte Automerge, pas le
  disque) ; les autres sont lus sur la projection. Conversion des positions UTF-16 du LSP ↔ indices
  Automerge testée par propriété.
- **LSP-3** Versions : chaque `didChange` porte la version du document ; chaque requête porte la
  version vue ; **toute réponse dont la version est dépassée par une modification locale ou d'un
  pair est abandonnée** sans affichage. Budget : p95 ≤ 100 ms (complétion, signature, survol),
  sinon abandon.
- **LSP-4** Survol et signature : docstrings `///` (syntaxe Typst, conventions de tinymist) des
  fonctions **de la personne et de sa charte** ; rendus en texte et listes sûrs, jamais en HTML brut
  (un docstring vient d'un fichier partagé, donc d'un pair) ; type technique replié sous « Détails ».
- **LSP-5** Gros fichiers : au-delà du palier « Grand », bascule automatique en mode
  `syntaxOnly` de tinymist.

**Erreurs comme un IDE (N28).**

- **ER-1** Un seul onglet « Problèmes à corriger » dans le tiroir : diagnostics de compilation
  (`typst`), du service de langage, du tableur (CSV), de Markdown (`cmarker`) et du CSS des modèles
  (TM-6), groupés par fichier ; compteur dans la barre d'en-tête.
- **ER-2** Chaque diagnostic est ancré par un curseur Automerge (il suit les modifications
  concurrentes jusqu'à l'actualisation) ; clic, ou **F8 / Maj+F8**, ouvre le bon fichier (MF-10),
  place le curseur et sélectionne la plage, prête à corriger.
- **ER-3** Messages : table **déterministe** de motifs (message du compilateur → phrase simple +
  action), le texte brut sous « Détails techniques » ; corrections rapides quand elles sont
  mécaniques (« Hint: » de Typst, actions de code de tinymist).
- **ER-4** Un diagnostic n'est jamais du HTML ni une commande (texte venu d'un pair).

**Confort d'écriture (N30)** — recommandations 11 à 18 de l'étude, chacune par un point du
§15.4 : orthographe fr et en (Hunspell, dictionnaires **embarqués**), dictionnaire personnel et
**du projet** (texte partagé) ; onglet « Relecture » à la demande (style, répétitions,
typographie), jamais de note chiffrée ; typographie française en deux niveaux (règle d'affichage du
modèle, saisie optionnelle) ; focus et machine à écrire, mots et temps de lecture, objectifs
**locaux** sans culpabilisation ; recherche et remplacement de projet ; extraits personnels et du
projet ; notes, citations (import `.bib`), renvois, table des matières, figures, tableaux, maths,
caractères par le menu « Insérer » et la palette ; comparaison **par mots** entre versions ;
préréglages d'export partagés ; plan cliquable avec présence ; saut croisé aperçu ↔ source ;
commentaires ancrés. LanguageTool : greffon optionnel en processus séparé (pas livré). Écartés :
réécriture par IA **livrée par défaut** hors `AD-82`, séries et notifications de gamification,
connexion à Zotero dans le cœur.

**Vue accessible de l'aperçu** (`AD-44`, revu). L'aperçu en `<img>` reste inerte ; il a un
**équivalent** : la « Vue lecture », produite par l'export HTML de Typst (≥ 0.15, MathML compris)
du même projet, **assaini** par Go (liste blanche, aucun script, aucune ressource distante),
affiché dans un `iframe` `sandbox` **sans** `allow-scripts`, navigable au clavier et au lecteur
d'écran, synchronisé avec la page courante ; chaque diagramme y porte sa `description`
(§15.14). Preuve : parcours clavier et lecteur d'écran (Orca) dans `ux-walkthrough`, en plus d'axe.

### 15.18 Intelligence artificielle de premier rang (N31)

*[cible ; ADR 0040 et 0041 à rédiger ; `AD-82`]* Étude : [research/ai.md](research/ai.md). L'IA
est une **extension intégrée** (`ext/ai`) sur la même API : elle n'a aucun droit qu'un greffon ne
puisse demander. Invariants tenus : le CRDT reste dans la webview, aucun port sauf `127.0.0.1` avec
jeton, aucun téléchargement à l'exécution (les modèles locaux sont installés par la personne ou
l'entreprise, jamais tirés par inkwell).

**Une API, quatre surfaces.** Le registre de commandes (§15.4) alimente la palette, les greffons,
les **outils de l'agent** et les **outils MCP** ; une commande déclare sa **classe** pour l'IA :
`lire`, `proposer`, `régler` (réglages, thèmes, modèles), `jamais` (supprimer, inviter, partager,
droits, clés, politique, greffons : non exposées).

**Fournisseurs** (point `ai-providers`, verbe *remplacer*, un actif par projet et par appareil) :
socle **compatible OpenAI** (`/v1/chat/completions`, flux SSE, appels d'outils), adaptateurs
natifs minces ; **appels faits par Go seulement** (la webview n'a pas de réseau), URL de base
libre, AC interne, mTLS, jeton ou OIDC, proxy, aucune télémétrie. Sert le cas « serveur d'IA de
l'entreprise » (NVIDIA NIM, vLLM, SGLang, Ollama, llama.cpp parlent ce dialecte ; à vérifier par
fournisseur). Client écrit en interne (`net/http`, analyseur SSE **fuzzé**), sans SDK de
fournisseur. Nouveau puits O4 et capacité `ai.complete` (§15.5, RF-3).

**Politique IA du projet** (signée, effet prospectif) : `off` · `local` (boucle locale) · `org`
(profils nommés par un admin : URL, empreinte d'AC, classe) · `libre`. Défaut d'un projet partagé :
**rien ne part vers un hôte distant sans décision d'admin** ; projet solo : la personne décide.
Fichier marqué `ai: never` honoré. Limite écrite à l'écran : c'est un contrôle des clients
honnêtes, pas une confidentialité cryptographique (quiconque lit peut copier). La décision vit au
journal signé : espace `ai/` → **delta du protocole PD-1** (§15.19) ; avant son acceptation, seule
la politique **par appareil** existe, et l'IA est **désactivée par défaut dans tout projet
partagé**.

**Consentement à l'envoi** (sécurité, *à auditer*). Avant le premier envoi d'une session :
« voici exactement le texte qui part, et vers où » (plage, fournisseur, classe) ; le texte
d'**autres personnes** y est marqué comme tel ; rien ne part sans geste.

**Éditer = proposer.** Toute sortie qui touche un document est une **proposition** (§15.5) : diff
lisible par morceau, « Accepter / Refuser / Tout accepter », appliquée en **une** modification
signée par la personne qui accepte, étiquetée « via IA » ; le texte en flux vit dans une couche
**fantôme** locale, jamais dans le CRDT. Validateur de contenu : liens et chemins nouveaux, `#read`,
`#include` hors projet, caractères invisibles (zéro largeur, balises Unicode U+E0000, bidi) mis en
évidence. Marques d'autorat calculées depuis l'historique, jamais écrites dans le fichier ; l'UI dit
« marqué IA par Camille » (une déclaration signée, pas une preuve).

**Injection de prompt par des pairs byzantins** (sécurité, *à auditer*). On ne filtre pas, on
**borne** (« The Attacker Moves Second », 2025 : les défenses par détection sont contournées) :

- **IA-1** Provenance par plage, calculée dans la webview depuis les acteurs Automerge : `soi`,
  `autrui`, `externe` (MCP, import, collage).
- **IA-2** Règle des deux (Meta, 2025) et CaMeL allégé (2025) : dès que le contexte contient
  `autrui` ou `externe`, l'agent ne garde que `lire` (le fichier courant) et `proposer` ; tout le
  reste est **retiré par Go et la webview**, pas par le prompt.
- **IA-3** Mode sans outil (quarantaine) pour reformuler, résumer, traduire du texte d'autrui.
- **IA-4** `régler` exige une confirmation modale qui montre l'effet exact ; `jamais` n'est pas
  exposé ; aucune image ni lien distant rendu dans la discussion.
- **IA-5** Preuves : test de propriété « aucune séquence d'outils depuis un contexte `autrui`
  n'atteint une classe retirée » ; corpus d'injections fuzzé.

**Discuter pour modifier l'application** : la discussion ne produit que des **artefacts
déclaratifs** validés — patch de réglages (schémas `settings`), jetons de thème (§15.8), modèle de
projet (§6.8), invocation d'une commande du registre — avec aperçu, accord et Annuler. Jamais de
code exécuté ; la génération de greffons en code est différée.

**Serveur MCP** (révision 2026-07-28 de la spécification MCP, vérifiée : requêtes sans état,
ressources, prompts, outils, élicitation ; échantillonnage, racines et journalisation dépréciés —
on ne s'y appuie pas) :

- **MCP-1** Par défaut, `inkwell mcp` (stdio, lancé par le client) relaie vers l'instance en cours
  par une socket Unix `0600` dans le dossier d'exécution de la personne : **aucun port**.
- **MCP-2** En option, désactivé par défaut : Streamable HTTP sur `127.0.0.1`, port éphémère,
  jeton comparé en temps constant, jamais journalisé, `Origin` validé (exigence de la spec MCP),
  aucun jeton tiers accepté ni relayé.
- **MCP-3** Portées : `lecture`, `compilation`, `proposition` (vers la file de revue humaine,
  « via <nom du client> ») ; jamais d'écriture directe, de suppression, de partage, de réglage, de
  politique ; consentement **par client** à la première connexion ; politique du projet appliquée
  avant tout (`ia.mcp: off | lecture | proposition`).
- **MCP-4** Go est un **relais** : chaque outil est une commande du registre exécutée dans la
  webview (CRDT) ; sans fenêtre, lecture seule de la projection, signalée.
- **MCP-5** Dépendance : `modelcontextprotocol/go-sdk` (≥ v1.7.0, qui prend en charge 2026-07-28),
  épinglé, derrière `internal/mcpserver` ; son gestionnaire Streamable HTTP reste **à confirmer**
  avant MCP-2.

**Agents externes et autres interfaces** (étude §9–§11 ; catalogue de 17 interfaces) :

- **ACP-1** Client ACP (Agent Client Protocol de Zed ; v1 stable, v2 en brouillon, vérifié) :
  **option** de la dernière étape, désactivée par défaut, décision d'admin par **empreinte de la
  commande** que la personne fournit (aucun registre, aucun téléchargement) ; l'agent est un
  processus tiers à pleins droits, lancé dans un dossier vide et, quand le système le permet, un bac
  à sable (résiduel affiché). inkwell ne déclare ni `fs` ni `terminal` ; l'agent reçoit **notre
  serveur MCP** de session (jeton propre à la session) : il lit et **propose** par nos outils ; un
  diff ACP (`oldText`, `newText`) devient une proposition ancrée, périmée si le texte a changé ;
  `execute`, `fetch`, suppression et déplacement refusés ; « toujours autoriser » ne vaut que pour
  la session. Une session ACP est une extension intégrée virtuelle `ext/acp:<agent>` (lecture,
  proposition, `compile.read`), soumise à la règle des deux.
- **UTCP** : **pas de client** (ses gabarits d'appel sont des ordres exécutables, `cli`, `tcp`,
  `udp` compris) ; **MCP reste l'interface unique** ; un manuel UTCP **exporté** depuis le
  registre (fichier `0600`, mêmes portées) est une option de la dernière étape, désactivée par
  défaut, avec un test de non-divergence.
- Écartés ou différés (raisons dans l'étude §10) : A2A, ACP d'IBM, WebMCP, `lsp-ai`, temps réel
  dans le nuage ; MCP Apps (`iframe`, réseau par l'hôte) différé ; client MCP sortant (outils de
  l'entreprise) en dernière étape, réduit par la règle des deux.
- **Règles de projet** pour l'IA (`.inkwell/ai-rules.md`, ton, glossaire) : n'ont force de
  consigne que si un admin les a signées (décision au journal, PD-1) ; sinon texte `autrui`.

**Feuille de route** (étude, §1 et §11) : fournisseurs, propositions, commandes « reformuler, corriger » → politique,
marques, panneau de discussion → MCP stdio → MCP HTTP, client ACP et manuel UTCP en option → discussion « application », client MCP sortant, voix locale.

### 15.19 Ce que l'extensibilité demande au protocole (PD-*)

Le protocole accepté (2.0.0-brouillon.10) ne contient **aucun** de ces ajouts. Ils sont proposés
ici pour une **mineure 2.1**, à écrire et à faire accepter par la boucle du protocole
(`docs/protocol/`) ; cette spécification ne la modifie pas. Tout ce qui en dépend est bloqué
d'ici là et le dit.

| # | Delta proposé | Contenu exact demandé | Vecteurs | Bloque |
|---|---|---|---|---|
| PD-1 | Espaces de décisions `plugin/` et `ai/` | (a) **règle de tolérance** de `policy_op_v1` pour un espace inconnu : l'entrée est **conservée et ignorée** par le pli, l'op reste acceptée, sans sanction (le pli reste identique entre 2.0 et 2.1 puisqu'une entrée ignorée n'a aucun effet) ; (b) inscription au §20.5 de `plugin/<id>` (clé ≤ 160 o, `id` en notation inverse de domaine) et `ai/policy`, `ai/profile/<nom>`, `ai/mcp` ; (c) schémas des valeurs (≤ 4096 o, CBOR à clés fixes) ; (d) émission seulement après la décision `format ≥ 1` ; (e) ligne au §16 du protocole | `policy.json#unknown-namespace`, `#plugin-entry`, `#ai-policy` | N23 (§15.10), politique IA de projet (§15.18) |
| PD-2 | Domaines de signature | inscrire `inkwell/plugin/v1` (signature des paquets de greffons, vérifiée par chaque pair pour un greffon de projet) et `inkwell/backup-head/v1` (têtes de sauvegarde, jamais échangées entre pairs, inscrites pour éviter toute collision) au §20.4 | `sig.json#plugin-package`, `#backup-head` | N23, §6.7 |

**Retirés de la révision 1** (plus besoin de delta) : la carte `renders` dans l'index (les rendus
sont des fichiers ordinaires, §15.14) ; un TLV `PLUGIN_API` dans le HELLO (PV-6 ne promet plus de
vérification préalable).

### 15.20 ABI noyau des greffons (normatif dès maintenant)

*[cible ; ADR 0039]* Le reste (interfaces de chaque point, schéma complet du manifeste,
registres) va dans `docs/plugin-api/` (§15.16) ; ce noyau est fixé ici pour qu'un greffon minimal
et l'hôte puissent être écrits indépendamment.

- **ABI-1 Module.** WebAssembly 2.0 cœur, `wasm32`, **réacteur** : exporte `_initialize` (appelé
  une fois par l'hôte, s'il existe) et jamais `_start` ; une seule mémoire exportée `memory`,
  maximum déclaré ≤ 256 Mio (refus sinon, avant instanciation).
- **ABI-2 Exports** : `inkwell_abi_version() -> i32` (rend `1`) ;
  `inkwell_alloc(len: i32) -> i32` (rend un pointeur aligné sur 8, ou 0) ;
  `inkwell_free(ptr: i32, len: i32)` ;
  `inkwell_call(fn_ptr, fn_len, in_ptr, in_len: i32) -> i64` : `fn` est le nom UTF-8 de la
  fonction WIT (`commands.run`, `renderers.render`…), `in` sa charge CBOR ; le résultat code
  `(ptr << 32) | len` d'une **réponse** CBOR que l'hôte lit puis libère par `inkwell_free`. Le
  greffon ne conserve aucun pointeur de l'hôte après le retour.
- **ABI-3 Imports** (module `inkwell`) : `host_call(fn_ptr, fn_len, in_ptr, in_len, out_ptr_ptr,
  out_len_ptr) -> i32` (même convention, la sortie est allouée par l'hôte **dans** la mémoire du
  greffon via `inkwell_alloc`) ; `log(niveau, ptr, len)` ; `cancelled() -> i32`. Fonctions de
  `wasi_snapshot_preview1` **tolérées** (pour les chaînes d'outils) : `fd_write` sur 1 et 2
  (redirigé vers le journal du greffon), `random_get` (graine fournie par l'hôte, enregistrée en
  mode débogage), `clock_time_get` (horloge monotone **seulement** si `time.read` est accordé,
  sinon 0), `proc_exit`, `environ_*` et `args_*` (vides), `fd_close`/`fd_fdstat_get` sur 0–2 ;
  toute autre importation → refus à l'installation.
- **ABI-4 Réponse** : CBOR `{ "ok": <valeur> }` ou `{ "err": {code, message, field?, capability?,
  hint} }` (DX-3) ; codes du registre ; émission en encodage déterministe (RFC 8949 §4.2.1) ;
  décodage strict des deux côtés (clés en double, étiquettes, flottants non canoniques refusés),
  tailles bornées (§16 du futur `docs/plugin-api/`, 4 Mio par défaut).
- **ABI-5 Négociation** : `host.features()` (premier appel conseillé) rend les fonctions
  disponibles et les capacités accordées ; une fonction absente rend `unimplemented`.
- **ABI-6 Annulation** : l'hôte interrompt l'exécution (contexte wazero) ; un greffon long appelle
  `cancelled()` entre deux étapes ; une réponse après annulation est jetée (IPC-5).
- **ABI-7 Paquet signé** : `.inkwell-plugin` = zip **sans compression variable** (entrées
  `STORE` ou `DEFLATE` niveau fixé, triées, dates nulles) ; octets signés = `"inkwell/plugin/v1\0"
  ‖ BLAKE3(manifeste canonique) ‖ BLAKE3(module) ‖ BLAKE3(liste triée des autres fichiers et de
  leurs BLAKE3)` ; signature Ed25519 dans `signature.cbor` (`{clé, sig}`), clé validée comme au §3.2
  du protocole.
- **ABI-8 Vecteurs** (dans `docs/plugin-api/vectors/`, dès la première version) : un module
  « bonjour » Rust et Go (sources, octets, hachage), un appel `commands.run` et sa réponse, une
  proposition annulable (entrée, sortie, effet attendu), un paquet signé et ses octets signés, une
  importation refusée, une réponse mal formée refusée.

### 15.21 Étapes de livraison de l'API

Livrer progressivement, sans dépendre des étapes suivantes (M-3, M5 des revues) :

| Étape | Contenu | Preuves avant l'étape suivante |
|---|---|---|
| **API 1.0** | registre, `commands`, `menus`, `actions`, `panels` (`ui.view`), `status`, `file-types`, `viewers` (autre vue seulement pour un type du cœur), `tokens`, `decorations` (marques seulement), `transforms` (par commande), `renderers` (`wasm`), `events`, `themes`, `settings`, `storage` ; kits Rust et Go ; extensions intégrées portées dessus ; Tâches et Graphviz en WebAssembly | P-PLUG-BENCH, P-PLUG-PROTO, P-EXT-1…7, `PLUG-COMPROMISED-*`, revue `security-auditor` |
| **API 1.1** | `exporters`, `importers`, `windows`, `gutters`, `inline-widgets`, `hover`, `completion`, `ai-tools` ; macros ; kits TypeScript et AssemblyScript | mêmes portes sur les nouveaux points |
| **API 1.2** | `storage-backend`, `mirrors`, `language-server`, points déclarés par un greffon ; greffons de projet si PD-1 est acceptée | P-BYZ des greffons de projet ; fuzz du dorsal |

Ordre d'implémentation dans 1.0 : registre et extensions intégrées → commande, panneau,
proposition de bout en bout → Graphviz jusqu'au PDF → reste.

## 16. Écarts avec le code au 2026-11-01

| Écart | Cible | Décision |
|---|---|---|
| `internal/app` porte aussi l'autorité et l'aperçu (ARC-M5) | `internal/ledger`, `internal/preview` | `AD-02`, `AD-05` |
| alias `internal/trust` → `protocol/trust` (chantier `proto1`) | imports directs de `protocol` | `AD-03` |
| `mux`, `frame`, file commune de `p2p.Node` | session par classe (ADR 0034) | étapes 4–10 |
| `Transport` par messages seulement | port `Conn`/`Stream` | `AD-06` |
| `Repo` dans le fil d'affichage | Worker | ADR 0028 D2, prérequis ADR 0031 (`AD-07`) |
| Stream `store` séquentiel | voies (≤ W+2), pages ≤ 1 Mio | ADR 0031, ADR 0034 étape 11 |
| aucun verrou d'instance ni de projet | `device.lock`, `.inkwell/lock` | `AD-12` |
| `project.json` sans version de disposition | `layout` + migration | `AD-10` |
| journaux sur la sortie d'erreur seulement | fichier tournant, export expurgé | `AD-16` |
| réglages en `localStorage` et variables d'environnement | fichier de réglages en Go | `AD-21` |
| `bridge` importe `project` et `store` | ports à lui | `AD-29` |
| `tinymist` embarqué mais jamais lancé | LSP par Stream `lsp` | `AD-20` |
| Wails `v3.0.0-beta.34` (stable publiée le 2026-10-26) | v3.0.x | `AD-19` |
| aucune règle de couches dans le frontend | Biome `noRestrictedImports` | `AD-30` |
| statuts d'ADR incohérents (index, 0021–0029 proposées mais livrées) | acceptations | `AD-07` |
| `files_test.go:713` ignoré (suspension et blobs) | test actif | `AD-36` |
| CI sous Linux seulement | selon la décision de plates-formes | `AD-18` |
| un seul document : clic dans la liste des fichiers sans effet (`App.svelte:197`), projection et synchronisation externe sur `main.typ` seulement (`workspace.svelte.ts:42,270–310`), aucun index matérialisé côté TypeScript | modèle multi-fichiers §6.6 | `AD-46` à `AD-50` |
| aperçu compilé sur un principal fixé au démarrage (`preview.go:85,179`) | principal suivi par le `Tree` (MF-8) | `AD-48` |
| deux langages colorés (Typst, Markdown), aucun registre de types | registre des contributions, grammaires à la demande | `AD-51`, `AD-54` |
| aucune extension, palette, menus et barres codés en dur | registre unique, extensions intégrées, greffons | ADR 0027, ADR 0039 (`AD-57` à `AD-72`) |
| `protocol/bridge` dans le module public (chantier `proto1`) | enveloppe du pont dans `internal/bridge` | `AD-31` |
| tests du sous-module `protocol` hors de `go test ./...` racine | tâches sur les trois modules | `AD-40` |
| conception p. 13 : plusieurs projets en fenêtres ; code et design : un seul | un projet à la fois en v1 | `AD-01` |
| `tinymist` embarqué mais son service de langage n'est pas branché (N29) ; aucun onglet « Problèmes », pas de F8 (N28) | §15.17 | `AD-79`, `AD-80` |
| aucune bibliothèque de modèles (N27) | §6.8 | `AD-78` |
| aucune IA, aucun serveur MCP (N31) | §15.18 | `AD-82` |
| clic dans la liste des fichiers sans effet (G-0) | chemin minimal G-0a puis G-0b, G-0c (§6.6) | `AD-77` |
| lecteurs : la conception permet leurs commentaires, la couverture (ADR 0018) les refuse | mécanisme dédié | `AD-14` |

## 17. Références

- Document de conception (`docs/conception.pdf`), ADR 0001–0034, `docs/protocol/spec.md`.
- ISO/IEC 25010:2023, *Product quality model*.
- S. Brown, *The C4 model for visualising software architecture*, <https://c4model.com/>.
- [RFC 2119], [RFC 8174] (BCP 14).
- OWASP ASVS 5.0 ; NIST SP 800-218 (SSDF) ; CWE Top 25.
- Go : [os.Root](https://go.dev/blog/osroot) ; [flight recorder](https://go.dev/blog/flight-recorder) ;
  [Go Code Review Comments](https://go.dev/wiki/CodeReviewComments) ; Google Go Style Guide.
- Wails v3 : [v3.0.0](https://github.com/wailsapp/wails/releases/tag/v3.0.0),
  guide *Streams* et *Updater* (`docs/mpress/content/guides/` du dépôt Wails).
- Biome `noRestrictedImports` (option `patterns`, Biome ≥ 2.2).
- A. Cockburn, *Hexagonal architecture* ; M. Fowler, *Event Sourcing*, *CQRS*.
- Extensions : [research/plugins.md](research/plugins.md) et ses sources ; wazero <https://pkg.go.dev/github.com/tetratelabs/wazero> ; WASI 0.3 <https://wasi.dev/releases/wasi-p3> ; Wasmtime, débogage <https://docs.wasmtime.dev/examples-debugging-guest.html> ; VS Code, points de contribution et *Workspace Trust* <https://code.visualstudio.com/api/references/contribution-points>, <https://code.visualstudio.com/api/extension-guides/workspace-trust> ; Eclipse `menuContributions` ; JetBrains, groupes d'actions <https://plugins.jetbrains.com/docs/intellij/basic-action-system.html> ; GNOME `GMenu` <https://docs.gtk.org/gio/class.Menu.html> ; Firefox, bouton « Extensions » unifié (Firefox 109) ; Figma <https://www.figma.com/blog/how-we-built-the-figma-plugin-system/> ; macros d'Office bloquées <https://learn.microsoft.com/en-us/deployoffice/security/internet-macros-blocked> ; Jupyter <https://github.com/jupyter/notebook/blob/4.4.1/docs/source/security.rst> ; Google AIP <https://google.aip.dev/193>, <https://google.aip.dev/151> ; Rust API Guidelines <https://rust-lang.github.io/api-guidelines/checklist.html>.
- Stockage distant : restic (`internal/backend/backend.go`), OpenDAL <https://opendal.apache.org/docs/concepts>, `io/fs`.
- Diagrammes : mermaid #3886 <https://github.com/mermaid-js/mermaid/issues/3886> ; CVE-2026-41148, CVE-2026-41149 <https://nvd.nist.gov/vuln/detail/CVE-2026-41149> ; diagraph <https://typst.app/universe/package/diagraph> ; D2.js <https://www.npmjs.com/package/@terrastruct/d2>.
- WCAG 2.2 (SC 1.4.3, 1.4.11, 2.4.13) ; RFC 8701 ; RFC 9413.

[RFC 2119]: https://www.rfc-editor.org/rfc/rfc2119
[RFC 8174]: https://www.rfc-editor.org/rfc/rfc8174

## Annexe — Brouillon d'ADR 0039 : greffons WebAssembly

> **Statut : proposée**, non acceptée. Elle sera copiée dans `docs/adr/0039-greffons-webassembly.md`
> après la revue `security-auditor` (points **SC-*** ci-dessous) et la preuve P-PLUG-BENCH.
> Étude de référence : [research/plugins.md](research/plugins.md) (ci-après « l'étude ») ;
> ce brouillon ne reprend pas sa comparaison, il tranche. Les affirmations marquées
> **[mémoire]** dans l'étude ne servent pas de preuve ici : celles dont la décision dépend ont été
> vérifiées le 2026-11-01 (liste en fin d'annexe), les autres restent ouvertes.

### Contexte et problème

N20, N21, N22, N23 : inkwell doit être **hautement extensible** — types de fichiers, interface,
stockage distant, thèmes, cycle de vie, greffons de projet — dans plusieurs langages, sans
perdre la sécurité byzantine (N7), la fiabilité (ADR 0029) ni la performance (ADR 0028).
Contraintes : Go n'importe pas Automerge (INV-1) ; toute modification passe par le CRDT dans la
webview ; aucun téléchargement à l'exécution sans geste (NG6) ; Linux amd64 et arm64 (`AD-18`).

### Facteurs de décision

Isolation forte et capacités (S), langages disponibles aujourd'hui (L), performance (P),
adéquation à Go et à arm64 (C), taille (T), versions de l'API (V), et, propre à inkwell : la
clé de l'appareil vit dans le processus Go.

### Options (détail : l'étude, §4)

1. Composants WIT dans Go par wasmtime-go (cgo).
2. Composants dans la webview par jco, un Worker par greffon.
3. Modules WebAssembly *cœur* par wazero (Go pur), **dans le processus** d'inkwell.
4. Option 3 **dans un processus enfant enfermé** (« hôte des greffons »).
5. Extism (sur wazero) au lieu d'une ABI à nous.

### Décision

**Option 4, avec l'ABI minimale de l'option 3 et le schéma WIT comme seule description.**

1. **Hôte.** Les greffons tournent dans `inkwell --plugin-host`, l'exécutable lui-même relancé
   en processus enfant, sous l'enfermement des sidecars défini au §9.10 (H-1 à H-8 : aucun fichier — modules
   par `memfd` scellé —, seccomp, Landlock, réseau coupé, mémoire par hôte, échec fermé visible),
   par **compartiments** (H-4 : un hôte par projet ouvert, un pour les greffons globaux sans sortie,
   un par greffon qui a une sortie réseau ou un dorsal). Canal avec le parent : §7.3 (le parent
   impose autorité, tailles, générations). Moteur : wazero (compilateur, **cache en mémoire
   seulement** en v1), un module instancié par greffon, un appel à la fois, réutilisé.
   *Pourquoi pas l'option 3 telle que l'étude la recommande :* une évasion du moteur (défaut du
   code natif généré) dans le processus principal donnerait `device.key`, le réseau et
   `os.Root` ; dans l'enfant, elle ne donne qu'un processus sans fichier, sans réseau et sans
   secret. *Pourquoi pas l'option 2 :* le code tiers tournerait dans l'origine qui tient le pont
   (Stream `net`, kind `Sign`) ; jco en navigateur est encore expérimental ; le mode sans fenêtre
   (`AD-35`) serait exclu. *Pourquoi pas l'option 1 :* l'API composants de wasmtime-go n'est pas
   publiée (tickets #280, #290, #291 ouverts) et ajoute du cgo.
2. **ABI.** Octets entrants, octets sortants : le greffon exporte `alloc`, `call(fonction,
   entrée) → sortie` ; l'hôte offre au plus six fonctions (`host_call`, `log`, `now` si la
   capacité l'accorde, `random`, `cancelled`, `yield`). Les messages sont en CBOR **émis** en encodage déterministe (RFC 8949 §4.2.1) et
   **décodés** strictement (profil de validité du §9.2 du protocole, côté réception ; ce profil ne
   rend pas la réception déterministe) ; contrat exact au §15.20, décrits en **WIT** (`inkwell:plugin@1.0.0`), d'où
   l'on génère les types Go, TypeScript et les kits de développement (Rust et Go d'abord ; tout
   langage qui produit un module `wasm32` peut suivre l'ABI). Imports WASI tolérés :
   seulement les fonctions de `wasi_snapshot_preview1` dont les chaînes d'outils ont besoin,
   neutralisées (aucun descripteur ouvert, aucune socket).
   *Pourquoi pas Extism :* même forme, mais une membrane plus large à auditer et un SDK Go dont
   la dernière version date du 2025-03-19 (v1.7.1) ; rien n'empêche un kit compatible plus tard.
3. **Où passent les données.** Le cœur ne copie jamais un document entier vers un greffon
   (leçon Figma) : le greffon demande des plages (`document.read`). Il modifie un document en
   rendant une **proposition** (opérations ancrées sur des curseurs stables, empreinte de la
   version lue) que la webview valide (bornes, porte de taille, garde) et applique en **une**
   modification annulable, signée par la personne, étiquetée « via X ». Go ne la décode pas
   au-delà de son enveloppe (INV-1 intact). Les pairs reçoivent une modification Automerge
   ordinaire : aucun greffon n'est requis chez eux.
4. **Interface.** Vues déclaratives (arbre fermé de composants Adwaita, validé par liste
   blanche, rendu par Svelte) ; `ui.canvas` **différée** (porte P-FRAME, §15.14) ; jamais de HTML, de CSS ni de JavaScript de greffon dans l'origine de
   l'application.
5. **Capacités, points, cycle de vie, thèmes, stockage, greffons de projet** : §15.3 à §15.10
   et §6.7 de cette spécification sont la partie normative de cette ADR.
6. **Paquet et confiance.** `.inkwell-plugin` = zip (manifeste, module, ressources), ≤ 16 Mio ;
   signé Ed25519 par son auteur (domaine `inkwell/plugin/v1`), empreinte en 4 mots montrée à la
   première installation (confiance au premier usage), **continuité de clé** exigée pour une mise
   à jour ; version monotone (pas de retour arrière silencieux) ; installé dans
   `<UserDataDir>/inkwell/plugins/<id>/<version>/`, hachage revérifié avant chaque chargement
   (comme les sidecars). Installation depuis un **fichier** en v1 ; un catalogue signé, consulté
   seulement sur geste, est une décision à part (`AD-61`). Greffons de projet : §15.10.
7. **Versions de l'API.** Comme le protocole (N14) : règles PV-1 à PV-6 du §15.16, détaillées dans la future spécification `docs/plugin-api/` ; majeure entière par greffon, mineures
   additives, N et N−1 pris en charge 12 mois, champ inconnu ignoré sauf marqué « à comprendre »,
   capacité inconnue → refus d'installer. Trajectoire : `inkwell:plugin@2` en composants WASI 0.3
   quand un hôte Go pur les exécutera **et** que les chaînes d'outils 0.3 seront stables pour au
   moins quatre langages (critères de l'étude, §5.6).

### Conséquences

- Bien : extensible partout sauf le noyau fermé ; un défaut du moteur ne touche pas la clé ;
  aucun format nouveau entre appareils sauf l'espace de décisions `plugin/` ; le cœur sert de
  premier client de l'API (P-EXT-3).
- Moins bien : un aller-retour entre processus par appel (budget §15.4) ; un processus de plus
  par projet ouvert avec greffons ; l'API WIT est à nous tant que les composants ne sont pas
  exécutables en Go pur ; Linux seulement tant que l'enfermement n'existe pas ailleurs
  (`AD-18`) — ailleurs, greffons tiers refusés (échec fermé), extensions intégrées
  TypeScript disponibles.

### Confirmation (portes avant acceptation)

| Porte | Preuve |
|---|---|
| **P-PLUG-BENCH** | banc `benchstat` (`-count ≥ 10`, p50, p95, p99), sur amd64 **et** arm64, trois greffons types (Rust, Go `wasip1`, TinyGo), avec **témoin en processus** (option 3) et **cible** en processus enfant (option 4) : compilation, instanciation (module déjà compilé dans l'hôte ; compilation comprise), appel vide, appels 64 Kio, 1 Mio et 4 Mio aller et retour, **trajet complet** webview → Go → enfant → retour, appels réentrants (`host_call` pendant un appel), 16 greffons en concurrence, mémoire résidente **par hôte** et coût d'un hôte par projet, taille ajoutée à l'exécutable. Seuils : appel complet p95 ≤ 2 ms pour 64 Kio ; interaction de vue p95 ≤ 100 ms ; activation ≤ 150 ms (compilé) et ≤ 500 ms (compilation comprise) ; hôte au repos ≤ 40 Mio RSS ; 0 ms avant la première peinture. Si la cible dépasse le témoin au-delà des budgets, la décision est revue (pas les budgets). **Prérequis de l'acceptation.** |
| P-PLUG-PROTO | prototype de bout en bout : un diagnostic et une proposition (greffon → hôte → Go → webview → modification Automerge → Annuler) |
| P-EXT-1 … 7 | §15.7 |
| P-SEC | revue `security-auditor` (Opus, xhigh) des points SC-* |
| P-FUZZ | décodeurs des sorties de greffon (propositions, vues, jetons, manifestes, paquets) fuzzés 1 h chacun |

### Points critiques pour la revue de sécurité

| # | Point |
|---|---|
| SC-1 | Enfermement de l'hôte des greffons : aucun fichier, aucun réseau, aucun secret ; échec fermé si l'enfermement manque |
| SC-2 | Fonctions hôte : chaque entrée bornée et validée ; chemins par `os.Root` côté Go ; aucune capacité accordée implicitement |
| SC-3 | Interruption (`WithCloseOnContextDone`) et plafond mémoire (`WithMemoryLimitPages`) sur chaque appel ; pas de *fuel* dans wazero : le délai le remplace |
| SC-4 | Propositions : décodage strict, bornes, ancres, porte de taille, garde, attribution ; jamais appliquées sans geste pour un greffon de projet (PP-2) |
| SC-5 | Vues déclaratives : liste blanche, aucun `{@html}`, `ui.canvas` et `render.frame` différées (P-FRAME) ; aucun déguisement en dialogue du cœur (PP-5) |
| SC-6 | Règle de flux RF-1 à RF-5 (§15.5) ; dorsal de stockage : chiffrement avant le greffon, clé jamais transmise (§6.7) |
| SC-7 | Paquets : zip (taille décompressée, chemins, liens), signature, continuité de clé, version monotone, revérification avant chargement |
| SC-8 | Greffons de projet : décision d'admin vérifiée, blob vérifié, niveau « projet », accord local au-delà, interrupteur, isolation par projet (PP-1 à PP-7) |
| SC-9 | Thèmes : données typées seulement, contrôle des contrastes, aucune URL |
| SC-10 | Bus d'événements : aucune donnée de document dans une charge ; files bornées |
| SC-11 | Rendu à DOM (`render.frame`) : **différé** ; P-FRAME exigée (processus web dédié, tuable, sans liaisons, réseau et navigation refusés par l'hôte) avant toute réouverture |
| SC-12 | Enfant compromis : le parent impose compartiment, génération, tailles, crédits et droits indépendamment des messages (§7.3) ; tests `PLUG-COMPROMISED-*` |
| SC-13 | Mandats des greffons de projet (§15.10) : effet contrôlé contre le mandat, révocation, réponses tardives jetées |
| SC-14 | IA : règle des deux et CaMeL allégé (IA-2), aperçu de l'envoi, politique du projet, MCP (portées, consentement par client, `Origin`, jeton) (§15.18) |

### Vérifié le 2026-11-01 (et ce qui reste ouvert)

- wazero v1.12.0 (2026-05-29) : `RuntimeConfig.WithCloseOnContextDone`, `WithMemoryLimitPages`,
  `NewCompilationCacheWithDir` existent (pkg.go.dev) ; Component Model non prévu (ticket #2200,
  fermé). Dépôt : `wazero/wazero`.
- wasmtime-go v50 : cgo ; prise en charge des composants demandée et non publiée (tickets #280,
  #290, #291 ouverts).
- Extism go-sdk : dernière version v1.7.1 du 2025-03-19.
- WASI 0.3.0 publié le 2026-06-11 (wasi.dev) ; Wasmtime ≥ 46 l'exécute ; jco : prise en charge
  0.3 expérimentale hors Node.
- WebKitGTK 6.0 : bac à sable du processus web toujours actif (guide de migration de WebKit).
- **Ouverts** : état de *wacogo* et *gravity* ; TUF ou Sigstore pour un futur catalogue ; garanties
  mémoire d'Extism (sans objet si l'option 5 reste écartée).

### Décisions demandées à l'utilisatrice

1. Accepter cette ADR (après P-PLUG-BENCH et la revue de sécurité).
2. Catalogue de greffons : fichier seulement en v1, ou aussi un catalogue signé (`AD-61`) ?
3. Niveau « projet » chargé sans question (N23) : **contribuer seulement** (liste réduite du §15.10, B-4 de la revue Fable) ; N23 attend la mineure PD-1 du protocole.
4. Mermaid différé (P-FRAME) ; Graphviz est le cas livré.
