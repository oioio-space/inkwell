# Architecture d'inkwell — spécification normative

| | |
|---|---|
| Version | **1.0.0-brouillon.2** |
| Date | 2026-11-01 |
| Statut | proposé (exigence N18) ; à valider par la boucle de revue Fable + Codex, puis par l'utilisatrice |
| Remplace | [`docs/architecture.md`](../architecture.md) (devenu un renvoi) |
| Finalité | [document de conception](../conception.pdf) (24 oct. 2026) |
| Décisions | [ADR](../adr/README.md) ; doutes et options : [decisions.md](decisions.md) (`AD-xx`) |
| Protocole | [docs/protocol/spec.md](../protocol/spec.md) (2.0.0-brouillon.10, [ADR 0034](../adr/0034-protocole-unifie.md)) |
| Traçabilité | [traceability.md](traceability.md) |
| Étude | [research/plugins.md](research/plugins.md) (greffons) |

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

Une règle sans marque est *[en place]*.

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
| | extensibilité (prioritaire) | 100 % des fonctions visibles inscrites par le registre des contributions ; chaque point du §15.4 utilisé par au moins une extension intégrée ; 0 import interne depuis `src/ext/**` ; ajouter un type, une commande, un panneau, un exportateur = une contribution, sans toucher au cœur ; démarrage : ≤ 10 ms ajoutées par 50 extensions | P-EXT-1 à P-EXT-7 ; `BenchmarkRegistryBuild` ; ADR 0027, 0039 |
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

Seuls quatre flux sortent de l'appareil : libp2p vers les membres (et le relais), mDNS sur
le LAN, le rendez-vous Wormhole pendant une invitation, et la vérification de mise à jour.
Chacun se désactive (conception : « tout ce qui se voit se désactive »).

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
| Cadre de rendu isolé *[cible]* | `iframe` sandbox, origine opaque | rendre un diagramme pour les moteurs qui exigent une mise en page de navigateur (§15.14) | aucun accès à l'application, aucune connexion |

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
| Rendus partagés (diagrammes) | entrée `renders` du nœud texte dans l'index *[cible, `AD-70`]* + blob | `Repo` | idem + `.inkwell/blobs/` | `.renders/<clé>.svg`, `.renders/index.json` |
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

### 6.7 Stockage distant par extension (N22)

*[cible, ADR 0039 ; `AD-67`]* Une extension peut fournir un **dorsal de stockage** (nuage,
autre système de fichiers, disque réseau). L'**intégrité** du stockage reste fermée (K-6) :
le cœur l'impose au-dessus de tout dorsal, qu'il traite comme **byzantin**, comme le font un
dépôt restic ou borg, un distant Git ou un appareil Syncthing « non fiable ».

**Ce qui ne change pas.**

- **ST-1** La copie locale (`.inkwell/` et la projection) reste **la** copie de travail,
  toujours autoritative pour l'édition, hors ligne compris. Typst, la projection Git
  (`AD-13`), la surveillance des modifications externes et la recherche (`AD-42`) travaillent
  sur ce dossier local matérialisé, jamais sur le dorsal.
- **ST-2** Aucune frappe n'attend le dorsal ; aucun arrêt ne l'attend : les données sont
  durables localement avant tout envoi (I1, I2).

**Ce que fait le dorsal.** Il réplique le magasin (pas la projection) : sauvegarde distante et
restauration sur un autre appareil de la même personne en v1 ; rendez-vous partagé entre
membres plus tard (`AD-68`).

**Contrat minimal** (interface WIT `storage-backend`), inspiré du dorsal de restic
(`Save`, `Load`, `Stat`, `List`, `Remove` sur des noms opaques), des services d'OpenDAL
(opérations uniformes et drapeaux de capacité par service) et d'`io/fs` (lecture seule,
noms validés) :

| Opération | Sens | Exigence sur le dorsal |
|---|---|---|
| `capabilities()` | taille maximale d'objet, lecture par plage, liste paginée | — |
| `put(nom, octets)` | écrit un objet ; idempotent | aucune (le cœur revérifie tout) |
| `get(nom, début, longueur)` | lit un objet ou une plage | aucune |
| `list(préfixe, curseur)` | énumère, par pages ≤ 1 000 | peut être incomplète ou mentir |
| `delete(nom)` | appelé seulement par le ménage du cœur | peut ne pas effacer |

Pas de renommage ni de verrou : l'échange atomique des têtes n'est **pas nécessaire**, car
chaque appareil n'écrit que **sa** tête (`heads/<PeerID>`), et le CRDT fusionne les têtes de
tous ; un dorsal sans écriture conditionnelle suffit.

**Ce que le cœur garantit au-dessus** (K-6) :

- **ST-3** Objets adressés par contenu : `nom = BLAKE3 à clé(clé de stockage, clair)`, ce qui
  déduplique dans le projet sans révéler d'égalité entre projets. Grands objets découpés et
  vérifiés par Bao (ADR 0023).
- **ST-4** Chiffrement côté client **avant** l'extension : XChaCha20-Poly1305, clé de
  stockage par projet et par appareil, gardée par Go à côté de `device.key` ; l'extension ne
  voit que des octets chiffrés et des noms opaques. La clé ne quitte jamais Go (S-3).
- **ST-5** À la lecture : authentification AEAD, puis BLAKE3 du clair = nom attendu, puis,
  pour un objet signé, revérification complète (D-4). Un échec : objet ignoré, compteur,
  message « La sauvegarde distante contient des données abîmées » ; jamais de quarantaine
  locale écrasée.
- **ST-6** Tête = manifeste signé par l'appareil (Ed25519, domaine `inkwell/backup-head/v1`)
  et chiffré, portant un compteur monotone ; une tête plus ancienne que la dernière vue est
  refusée (retour arrière). Un retour arrière ne peut de toute façon rien corrompre : il
  apporte au pire des modifications déjà connues.
- **ST-7** Disponibilité seulement : un dorsal hostile peut refuser, ralentir, effacer ; il
  ne peut ni lire, ni modifier, ni rejouer sans être vu. L'état « Sauvegarde distante : à
  jour / en retard de N modifications / injoignable depuis … » est montré.

**Échecs et latence.** Envois et lectures passent par une file bornée hors du chemin
d'édition, délai de 30 s par appel, recul exponentiel, débit plafonné ; hors ligne, la file
s'allonge sur disque (bornée par le quota du projet) et repart au retour.

**Restauration.** Sur un appareil de la même personne : lire les têtes, vérifier, télécharger
les objets, reconstruire `.inkwell/`, puis matérialiser la projection (Typst et Git
retrouvent un vrai dossier). La clé de stockage passe d'un appareil à l'autre par une phrase de
récupération montrée une fois (v1) ; jamais par le dorsal.

**Miroir lisible** (point distinct, `mirrors`) : copie à sens unique de la projection lisible
vers un dossier distant (par exemple un partage Nextcloud), sans `.inkwell/`, explicitement non
chiffrée et signalée comme telle.

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
| Route `/ext/<id>/` *[cible]* | Go → webview | icônes, polices des thèmes, page du cadre de rendu (§15.14), servies par le gestionnaire d'actifs de Wails depuis le paquet vérifié | CSP propre au cadre ; aucun port |
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
| TB-9 | extensions | greffons (code tiers), greffons de projet, cadres de rendu | hôte enfermé, capacités, règle d'exclusion, vues déclaratives, assainisseur SVG, ND-1 à ND-8 (§15) |
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
| Greffon malveillant (installé par la personne) | ce que ses capacités accordées permettent, dans ses bornes | lire puis faire sortir du clair (règle d'exclusion) ; agir hors de ses capacités ; cacher l'interface du cœur ; ralentir l'application | ce qu'une capacité accordée permet ; une évasion du moteur reste enfermée dans l'hôte |
| Admin qui ajoute un greffon de projet hostile | lui faire lire le projet chez chacune (ce qu'il lit déjà) et proposer des modifications | écrire au nom d'une personne sans son geste (PP-2) ; atteindre un autre projet, les fichiers, le réseau, les clés | interface trompeuse dans son cadre attribué (PP-5) ; R7 (admins de confiance) |
| Dorsal de stockage hostile | refuser, ralentir, effacer, rejouer d'anciennes têtes | lire ou modifier sans être vu | indisponibilité de la sauvegarde distante |
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
| Panneau latéral, onglet du tiroir | `panels` | contribuer | `ui.view` | Historique, Commentaires, Activité, Rechercher, Sommaire (U30), Tâches (U29, livrée en **greffon WebAssembly**) | `onView` | premier rendu ≤ 100 ms ; mises à jour ≤ 10/s |
| Barre d'état, bandeau, toast | `status` | contribuer | `ui.view` | mots, « Page X sur Y », encodage, fin de ligne (U1) | `onFileType` | ≤ 4 mises à jour/s par élément, coalescées |
| Type de fichier : reconnaissance | `file-types` (données) | contribuer | — | Typst, Markdown, tableaux, texte, code, images, PDF (§6.6) | aucune | registre construit ≤ 10 ms |
| Vue, éditeur d'un type | `viewers` | remplacer (par type) | `document.read`, `document.propose` ou `blob.read` ; `ui.view` ou `ui.native` | éditeur CodeMirror, grille CSV (ADR 0026), visionneuse d'images, PDF.js (`AD-39`) | `onFileType` | ouverture d'un fichier ≤ 1 Mio : activation + premier rendu ≤ 200 ms ; démarrage à froid d'un greffon ≤ 150 ms (module compilé en cache) |
| Coloration | `grammars` (Lezer intégrées) ; `tokens` (greffons) | contribuer | `document.read` | Typst, Markdown, langages de la liste fermée (`AD-54`) | `onLanguage` | intégrée : incrémentale, ≤ 5 ms par mise à jour de la zone visible ; greffon : plages de la zone visible, ≤ 50 ms, rendu ≤ 4 ms |
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
| Miroir lisible | `mirrors` | contribuer | `project.read`, `mirror.dir` (un dossier local, par exemple monté par le client du nuage ; jamais `net.hosts`, règle d'exclusion) | aucune en v1 | `onSaved` | idem |
| Cycle de vie, événements | `events` (§15.9) | contribuer | celle de chaque événement | Activité (U12), « Pendant votre absence… », toasts de fin d'export | `onEvent:<nom>` | émission ≤ 1 µs sans abonné ; livraison asynchrone, coalescée ; arrêt : 1 s au plus pour tous les greffons |
| Rendu de diagramme | `renderers` (§15.14) | contribuer (par langage) | aucune (contrat pur) | « Diagrammes » : Graphviz ; exemple : Mermaid | `onLanguage` | p95 ≤ 300 ms (≤ 200 nœuds), plafond 5 s, hors du fil d'affichage |
| Fenêtres, dialogues, menus contextuels, gouttières, widgets, accueil | `windows`, `menus`, `gutters`, `inline-widgets`, `welcome` (§15.13) | contribuer | `ui.view` | fenêtre public de présentation (U28) ; menus contextuels de la liste des fichiers ; bande des modifications | `onCommand`, `onView` | ouverture d'une fenêtre ≤ 300 ms ; menus : 0 appel d'extension pour s'afficher |

### 15.5 Capacités

Aucune capacité ambiante : un greffon n'a ni système de fichiers, ni réseau, ni horloge non
déterministe, ni processus. Les capacités sont **nommées dans le manifeste, montrées en
phrases simples avant l'installation, accordées par appareil et par projet**, et revérifiées
par Go pour tout ce qui sort de la webview (B-2).

| Capacité | Donne | Contrôle | Phrase montrée |
|---|---|---|---|
| `self.read`, `people.read`, `presence.read`, `extensions.read` | lectures du §15.3 | registre | « voir les personnes du projet » |
| `document.read` | plages des documents texte du projet, à la demande (jamais un document entier copié d'office) | webview, porte de taille | « lire vos documents » |
| `document.propose` | proposer des modifications ancrées (étude, §5.2) ; la webview les valide puis les applique en **une** modification annulable, signée par la personne, étiquetée « via X » | webview : bornes, garde, attribution | « proposer des modifications » |
| `document.create` | créer un fichier texte par l'index (MF-4) | webview | « créer des fichiers » |
| `project.read`, `blob.read` | lire la projection et les blobs, par `os.Root` | Go | « lire les fichiers du projet » |
| `compile.run`, `compile.read` | obtenir un PDF, des pages SVG, `typst query` du projet ; lire les ancres de l'aperçu | Go, sidecar enfermé | « compiler le projet » |
| `export.write` | écrire **un** fichier choisi par la personne dans une boîte de dialogue de Go | Go | « enregistrer un fichier que vous choisissez » |
| `import.read` | lire **un** fichier choisi par la personne | Go | « ouvrir un fichier que vous choisissez » |
| `history.read` | historique des documents (auteurs, dates, différences) | webview | « voir l'historique » |
| `storage` | magasin clé-valeur privé, 10 Mio | Go, `os.Root` | — |
| `ui.view` | vues déclaratives (arbre de composants Adwaita validé, étude §8) | webview | — |
| `ui.canvas` | une vue libre en `iframe` `sandbox` sans `allow-same-origin`, CSP sans réseau | webview | « afficher un dessin libre » |
| `ui.native` | monter du code dans le DOM | **extensions intégrées seulement** | — |
| `net.hosts` | requêtes HTTPS vers les hôtes nommés dans le manifeste, faites par Go (pas de cookie, pas de redirection hors liste) | Go | « se connecter à : nuage.example » |
| `storage.backend` | recevoir et rendre des objets **chiffrés** (§6.7) | Go | « garder une copie chiffrée de vos projets chez … » |
| `mirror.dir` | écrire la projection lisible dans **un** dossier choisi par la personne | Go, `os.Root` | « copier vos fichiers lisibles dans … » |

**Règle d'exclusion** (sécurité, *à auditer*) : un greffon qui a `net.hosts` n'a aucune capacité qui lit du clair (`document.read`, `project.read`, `blob.read`, `history.read`, `people.read`, `compile.run`) ; un dorsal de stockage ne voit que du chiffré. Ainsi rien ne sort de l'appareil en clair par un greffon. Une version future qui lèverait cette règle exige une ADR.

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
  texte ≥ 4,5:1 (SC 1.4.3), composants et anneau de focus ≥ 3:1 (SC 1.4.11, 2.4.13 visé). Une
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

Un greffon peut **appartenir à un projet** : il se charge automatiquement chez chaque membre
qui ouvre ce projet, sans installation globale. Le précédent canonique de ce motif est la
macro d'Office qui s'exécute à l'ouverture, au point que Microsoft bloque depuis 2022 les macros
des fichiers venus d'Internet. On l'accepte donc **borné**, pas tel quel. Décision : `AD-69`.

**Vérité.** La liste des greffons d'un projet est une **décision d'admin** du journal signé
(ADR 0030), espace de noms `plugin/<id>` : `{version, hash BLAKE3 du paquet, majeure d'API,
niveau, ajouté par, date}`. Aucune nouvelle vérité ; nouvel espace de noms de décisions,
extension compatible du protocole (§1.4), à inscrire au registre §20.5. Les octets du paquet
sont un blob du projet (ADR 0023), répliqué par le vrac et vérifié (BLAKE3, Bao) avant tout
chargement. Une op non signée ou d'un non-admin est refusée comme toute op (K-2).

**Niveau « projet »** (chargé sans question) : contributions déclaratives (commandes,
menus, barre d'outils, panneaux `ui.view`, barre d'état), types de fichiers et leurs vues,
coloration et décorations, propositions de modification, lecture du projet (`document.read`,
`project.read`, `blob.read`, `compile.read`), magasin privé **par projet**, événements du
projet. **Tout le reste** (`net.hosts`, `storage.backend`, `mirror.dir`, `export.write`,
`import.read`, `ui.canvas`, `people.read`, `app.*`) exige l'accord local de la personne, une
fois par version et par différence de capacités (motif *Workspace Trust* de VS Code ; le
greffon reste en mode restreint tant qu'il n'est pas accordé).

**Règles de sûreté** (toutes *à auditer*) :

- **PP-1** Isolé par projet : un hôte de greffons **par projet ouvert**, distinct de celui des
  greffons globaux ; aucun accès à un autre projet, aux fichiers de la personne, au réseau, aux
  clés ; déchargé à la fermeture du projet.
- **PP-2** Pas d'écriture au nom de la personne sans son geste : les propositions d'un greffon
  de projet ne s'appliquent que sur une commande invoquée par la personne, jamais sur un
  événement ni en arrière-plan ; la modification porte « via X (ajouté par Marie) ».
- **PP-3** Provenance visible : la première ouverture montre un bandeau non bloquant « Ce projet
  utilise 2 extensions ajoutées par Marie. **Voir** · **Désactiver pour moi** » ; la liste dit
  qui a ajouté quoi, quand, l'auteur et l'empreinte du paquet.
- **PP-4** Interrupteur : chacune peut désactiver un greffon de projet **pour elle**, à tout
  moment ; le projet reste lisible et modifiable sans lui (repli MF-14 : texte ou carte de
  blob).
- **PP-5** Aucun déguisement : une vue de greffon est toujours encadrée de son nom et du badge
  « Extension du projet » ; elle ne s'affiche jamais dans un emplacement de dialogue du cœur.
- **PP-6** Concurrence de décisions : deux versions décidées en même temps → décision
  `contested` (ADR 0030 D2) → la plus prudente : aucune des deux chargée, carte aux admins.
- **PP-7** Bornes : paquet ≤ 16 Mio, module WebAssembly ≤ 8 Mio décompressé, 16 greffons par
  projet, 64 Mio au total, comptés dans le quota N10 de l'admin qui les ajoute.

**Versions entre appareils** (N14) : le manifeste porte la majeure et la mineure d'API ; un
appareil charge un greffon si sa majeure est N ou N−1 et sa mineure ≤ la sienne ; sinon :
« Mettez Inkwell à jour pour utiliser X » et repli PP-4. Une seule version debout par `id`
(pli du journal) ; mettre à jour = une nouvelle décision ; l'historique du journal permet le
retour arrière.

**Antécédents** : macros d'Office bloquées par la marque du Web (2022) ; *Workspace Trust* de
VS Code (mode restreint, capacité `untrustedWorkspaces`, qui ne peut pas empêcher une extension
malveillante d'ignorer le mode — d'où notre bac à sable) ; dossier `.obsidian/plugins`
synchronisé (code arbitraire sans bac à sable : le contre-exemple) ; paquets Typst qui
voyagent avec le projet (purs, sans pouvoir : le modèle) ; carnets Jupyter « de confiance »
(signature HMAC locale ; sortie non fiable jamais exécutée) ; greffons d'organisation de Figma
(liste approuvée par un admin, exécutée dans un bac à sable).

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
| ND-2 | Aucune imitation de l'interface du système | vues rendues par l'hôte depuis un arbre déclaratif, toujours encadrées du nom et de l'icône de l'extension (badge « Extension du projet » pour PP-5) ; aucun accès aux composants de dialogue d'accord ni aux champs de mot de passe ; pas de superposition | propriété fast-check `EXT-ND2` : aucun arbre généré ne produit un élément hors de son conteneur ni sans cadre ; capture : un dialogue de greffon se distingue d'un dialogue d'accord |
| ND-3 | Aucun raccourci du cœur pris en silence | raccourcis déclarés dans le manifeste ; liste réservée du cœur ; conflit → raccourci de l'extension inactif, signalé dans « Extensions » et dans l'éditeur de raccourcis, la personne tranche | `EXT-ND3` (unitaire et e2e) |
| ND-4 | Les principes UX et le glossaire valent pour les extensions | contrôle à l'hôte au chargement : chaînes fr et en présentes, longueurs par emplacement (titre de commande ≤ 40 caractères, info-bulle ≤ 80, toast ≤ 120), aucun terme « Ne jamais dire » du glossaire, couleurs par jetons seulement, icônes d'Adwaita ou du paquet | `EXT-ND4` ; même garde que l'i18n du cœur (§2) |
| ND-5 | Une extension lente ne ralentit pas l'application | budgets du §15.4 mesurés par appel ; au-delà : cadence réduite, puis mise en pause avec le message « X est trop lent et a été mis en pause » ; aucun travail d'extension tierce sur le fil d'affichage ; vue ≤ 2 000 nœuds, ≤ 10 mises à jour/s ; mémoire plafonnée | `PERF-EXT-*`, `EXT-ND5` (greffon lent de test : ralenti puis mis en pause, frappe toujours ≤ 16 ms) |
| ND-6 | Tout désactiver rend l'application d'origine | **mode sans extensions** : `inkwell --safe-mode`, menu « Redémarrer sans extensions », proposé d'office après deux démarrages interrompus (marqueur d'exécution, `AD-12`) ; aucun greffon global ni de projet chargé, extensions intégrées telles que livrées | e2e `EXT-ND6` : avec des greffons hostiles installés, le mode sans extensions donne le même registre et les mêmes captures qu'une installation neuve |
| ND-7 | Aucun document ne devient illisible sans son extension | contenu toujours standard (texte, blobs) ; aucun format propre à une extension dans le CRDT en v1 (`AD-66`) ; repli MF-14 | `EXT-ND7` : projet de test, greffons de projet désactivés, chaque fichier s'ouvre |
| ND-8 | Annuler reste maître | toute proposition appliquée = **un** pas d'annulation, nommé ; Ctrl+Z l'annule toujours | `EXT-ND8` |

### 15.12 Au-delà du catalogue : retenu, différé, écarté

| Proposition | Verdict | Raison et conditions |
|---|---|---|
| Points d'extension **déclarés par un greffon**, que d'autres étendent (graphe de dépendances, intervalles SemVer) | **retenu, API 1.1** | modèle d'Eclipse et des dépendances d'extensions de VS Code ; points nommés `<id>/<point>` décrits en WIT dans le paquet ; résolution à l'installation et au chargement d'un projet : cycle, conflit d'intervalles, profondeur > 4 ou > 32 dépendances → refus avec la raison ; appels entre greffons **par l'hôte** seulement ; la règle d'exclusion (§15.5) s'applique à **toute la composante** du graphe d'appel (pas d'adjoint complaisant : un greffon sans réseau ne peut pas faire sortir des données par un greffon qui en a) ; un greffon de projet ne dépend que de greffons du même projet ou d'extensions intégrées |
| Réglages par schéma, rendus par l'hôte | **retenu, API 1.0** | point `settings` (§15.4) : sous-ensemble de JSON Schema (booléen, énumération, nombre borné, texte borné) ; valeurs dans les réglages de l'appareil, espace de noms de l'extension |
| i18n des extensions | **retenu, API 1.0** | tables fr et en dans le manifeste, contrôlées par ND-4 ; langue absente → l'autre, signalée |
| Commandes composées en macros | **retenu, API 1.1** | une macro est une **donnée** (suite de commandes et d'arguments), écrite ou enregistrée par la personne, dans ses réglages ; chaque pas repasse les capacités ; lancer la macro est le geste (PP-2) |
| Automatisations déclenchées par un événement (« formater à l'enregistrement ») | **retenu, borné** | seulement celles que la personne crée elle-même, pour des greffons globaux ; jamais proposées ni lancées par un greffon de projet (PP-2 : c'est la macro d'Office) |
| Transformations de document | **retenu** | point `transforms` (§15.4), par proposition |
| Étapes du pipeline de compilation (avant `typst`) | **écarté** | X-8 : le PDF doit être le même chez tous ; un appareil sans le greffon compilerait autre chose. Remplacé par : paquets Typst fournis par le projet (greffons Typst compris), étapes **après** compilation (`preview-overlays`, `exporters`) |
| Langages fournis par un greffon, contrat proche de LSP | **retenu, API 1.1** | point `language-server` : sous-ensemble de LSP 3.17 (complétion, survol, diagnostics, définition, symboles, formatage) en messages WIT, adapté à CodeMirror par l'hôte comme tinymist (`AD-20`) ; le serveur est un module WebAssembly (aucun processus lancé) |
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

Mermaid, Graphviz (DOT), D2, Vega-Lite, PlantUML : le parcours de bout en bout valide le
modèle et ajoute un point d'extension, `renderers`. Décision : `AD-70`.

**1. Contrat de rendu, pur.** `render(langage, source, options) → svg | diagnostics` : aucune
capacité, aucune horloge, aucun réseau, sortie bornée ; déterministe à version fixée. Le paquet
déclare où il tourne :

| Hôte | Pour | Exemples |
|---|---|---|
| `wasm` (hôte des greffons, défaut) | moteurs compilés en WebAssembly | Graphviz (le paquet Typst `diagraph` l'embarque déjà en greffon Typst ; Emscripten), D2 (écrit en Go, construit en WebAssembly par D2.js, encore avant stable), Vega-Lite (rendu SVG sans DOM, dans un moteur JavaScript compilé en WebAssembly) |
| `frame` (cadre de rendu isolé) | moteurs qui exigent un vrai moteur de mise en page | **Mermaid** : il mesure le texte par le moteur du navigateur ; un rendu sans navigateur n'est pas possible d'après ses mainteneurs (ticket mermaid #3886) |

Le **cadre de rendu isolé** : `iframe` `sandbox="allow-scripts"` sans `allow-same-origin`
(origine opaque, donc aucun accès aux liaisons Wails ni au stockage de l'application), hors
écran, page servie par Go sous `/ext/<id>/render` avec sa propre CSP (`default-src 'none'`,
scripts du paquet autorisés par empreinte SHA-256, polices fournies seulement, aucune
connexion) ; il reçoit la source par `postMessage` et rend une chaîne SVG. C'est le seul cas où
du JavaScript d'extension tourne, et seulement pour le contrat pur de rendu (SC-11). Pour
Mermaid : `securityLevel: 'strict'`, `htmlLabels: false` (aucun `foreignObject`), version ≥ 11.15
(correctifs des injections CSS et HTML de 2026, CVE-2026-41148, CVE-2026-41149).

**2. Assainir avant tout affichage.** Tout SVG produit par une extension passe par
l'assainisseur de Go (lecture XML en flux, liste blanche d'éléments et d'attributs : ni
`script`, ni `foreignObject`, ni gestionnaire `on*`, ni lien externe — seulement `#fragment` —,
`style` sans `url()` externe ni `@import` ; ≤ 4 Mio, ≤ 50 000 éléments, profondeur ≤ 64 ;
cible `Fuzz*`). Puis l'affichage se fait en `<img>` seulement (B-7). Un SVG reçu d'un pair est
réassaini avant affichage.

**3. Jusqu'au PDF.** Typst n'inclut que des images : le rendu devient un **blob** du projet
(magasin BLAKE3, ADR 0023), sous la clé `BLAKE3(greffon@version ‖ empreinte du paquet ‖
options ‖ source)`. Le nœud texte qui contient le diagramme porte, dans l'index, une carte
`renders` (`clé → {hash, greffon, version}`, ≤ 256 entrées, schéma additif V-5, règle de garde à
ajouter à l'ADR 0030) ; la projection écrit `.renders/<clé>.svg` et `.renders/index.json`
(source → clé). Un paquet Typst fourni, `inkwell-diagrams` (copié dans `.typstdeps/` à la
première insertion), contient la règle `show raw.where(lang: "mermaid")` (et `dot`, `d2`…) qui
lit `index.json` et appelle `image(...)`. Le même chemin sert le Markdown (`cmarker` passe les
blocs de code à Typst) et l'export Markdown ou HTML (image à côté de la source). X-8 tient : le
greffon n'entre pas dans la compilation, son **résultat** est un fichier du projet comme une
image.

**4. Collaboration.** Seule une personne qui écrit et vient de modifier la source publie le
rendu (une modification attribuée « via Mermaid » : blob plus entrée `renders`). Une personne
sans le greffon voit le dernier rendu partagé, dans l'aperçu et dans le PDF : repli ND-7. Avec un
greffon de projet (N23), il se charge chez chacune. Deux rendus concurrents d'une même clé :
même nom de fichier, homonymes résolus par l'index (ADR 0023), le même partout. Un rendu qui ne
correspond pas à sa source (écrivain malveillant) se voit chez qui a le greffon : « Le dessin
partagé ne correspond pas à sa source — Redessiner » ; c'est le même risque qu'une image
trompeuse, attribuée.

**5. Performance.** Rendu 500 ms après la dernière frappe dans le bloc, hors du fil
d'affichage, le plus récent annule le précédent ; cache local LRU par clé puis cache partagé
(blob) ; budget du point `renderers` : p95 ≤ 300 ms pour ≤ 200 nœuds, plafond 5 s
(« Diagramme trop complexe ») ; remplacement de l'image ≤ 16 ms.

**6. Édition.** Markdown : bloc ` ```mermaid ` ; Typst : bloc brut ` ```mermaid ` (même règle
`show`). Coloration du langage du bloc par le registre (N20 ; langage imbriqué de
`lang-markdown`, grammaire fournie par le greffon en jetons, §15.4). Aperçu : dans l'aperçu
Typst, et en ligne sous le bloc par un widget (`inline-widgets`) qui affiche le SVG en `<img>`.
« Insérer › Diagramme » propose les langages des greffons de rendu présents.

**7. Ce qu'on livre.** Extension intégrée « Diagrammes » : Graphviz en `wasm` (preuve du point
`renderers` et du chemin jusqu'au PDF) ; Mermaid comme greffon d'exemple en `frame`, installable
ou ajouté à un projet.

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
- **DX-5** Surface mesurée : API 1.0 ≤ **60 fonctions** et ≤ **40 types** ; la porte
  `mise run api:surface` compte depuis le WIT et échoue sur toute croissance non revue.
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

#[command(title = "Dire bonjour")]
fn hello(ctx: &Context) -> Result<()> {
    ctx.toast("Bonjour !")
}

export_plugin!(hello);
```

Langages pris en charge en 1.0, chacun avec un kit typé et un modèle : **Rust**, **Go**
(`GOOS=wasip1` ou TinyGo), **TypeScript/JavaScript** (compilé en WebAssembly avec un moteur
JavaScript embarqué, à la manière de Javy), **AssemblyScript** (proche de TypeScript, peu
coûteux). Chaque kit offre des aides pour les cas courants : inscrire une commande ; ajouter
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

**Exemples qui portent la documentation** (chacun complet, testé en CI, dans les quatre
langages quand c'est raisonnable) :

| Exemple | Points utilisés | Lignes de code visées (hors manifeste) |
|---|---|---|
| Bonjour | commande, toast | ≤ 10 |
| Diagrammes Graphviz et Mermaid | `renderers`, `inline-widgets`, coloration | ≤ 40 par moteur (le moteur lui-même mis à part) |
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
  Valeurs « graisse » (GREASE, RFC 8701, leçons de RFC 9413) : l'hôte en émet dans les messages
  et les kits de test en injectent, pour que personne ne fige une liste fermée par erreur. Pas de
  graisse là où elle changerait une décision de sécurité (capacités, signatures).
- **PV-6** Greffons de projet et appareils de versions différentes : un appareil dont l'hôte ne
  parle pas l'API demandée ne le charge pas, l'indique (« Ce projet utilise une extension qui
  demande Inkwell 1.5 ou plus récent ») et garde le repli ND-7 (dernier rendu partagé, texte,
  carte) ; un admin qui ajoute un greffon de projet voit avant de confirmer quels appareils
  membres, d'après la version annoncée dans leur HELLO, ne pourront pas le charger.

**Plan de `docs/plugin-api/`** (à écrire) :

1. Conventions (BCP 14, notation, décodage strict) ; 2. Versions et évolution (PV-1 à PV-6,
   politique de soutien, matrice) ; 3. Paquet (`.inkwell-plugin` : zip, bornes, signature,
   continuité de clé) ; 4. Manifeste (schéma JSON normatif, champs, contributions, activation,
   alias des capacités en mots simples) ; 5. ABI (exports, imports, mémoire, `alloc`, appels,
   annulation, imports WASI tolérés) ; 6. Encodage des messages (profil CBOR déterministe du
   protocole, §9.2) ; 7. Modèle d'erreur (codes, champs) ; 8. Capacités (noms, portée, règle
   d'exclusion, niveaux « projet » et accord) ; 9. Interfaces WIT par point d'extension (§15.4,
   §15.13, §15.14) ; 10. Événements (§15.9) ; 11. Vues déclaratives (composants, propriétés,
   bornes, accessibilité) ; 12. Propositions de modification (ancres, base, bornes) ;
   13. Contrat de rendu et assainissement SVG ; 14. Dorsal de stockage (§6.7) ; 15. Thèmes
   (contrat de jetons, contrôle des contrastes) ; 16. Bornes (table) ; 17. Sécurité et vie privée ;
   18. Conformité : vecteurs JSON (`enforce` par version comme le protocole), exécuteur Go
   (hôte) et kit des greffons (`inkwell plugin test`) ; 19. Registres (codes, capacités,
   événements, emplacements de menu, composants).

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
   en processus enfant, sous l'enfermement des sidecars (§9.5 : espace de noms réseau privé,
   Landlock sans aucun accès au système de fichiers, `RLIMIT_AS`, `Pdeathsig`, groupe de
   processus, superviseur avec recul). Un hôte pour les greffons globaux, un hôte par projet
   ouvert pour ses greffons de projet (PP-1). Moteur : wazero (compilateur, cache de compilation
   sur disque), un module instancié par greffon, réutilisé entre appels.
   *Pourquoi pas l'option 3 telle que l'étude la recommande :* une évasion du moteur (défaut du
   code natif généré) dans le processus principal donnerait `device.key`, le réseau et
   `os.Root` ; dans l'enfant, elle ne donne qu'un processus sans fichier, sans réseau et sans
   secret. *Pourquoi pas l'option 2 :* le code tiers tournerait dans l'origine qui tient le pont
   (Stream `net`, kind `Sign`) ; jco en navigateur est encore expérimental ; le mode sans fenêtre
   (`AD-35`) serait exclu. *Pourquoi pas l'option 1 :* l'API composants de wasmtime-go n'est pas
   publiée (tickets #280, #290, #291 ouverts) et ajoute du cgo.
2. **ABI.** Octets entrants, octets sortants : le greffon exporte `alloc`, `call(fonction,
   entrée) → sortie` ; l'hôte offre au plus six fonctions (`host_call`, `log`, `now` si la
   capacité l'accorde, `random`, `cancelled`, `yield`). Les messages sont en CBOR déterministe
   (profil du §9.2 du protocole, déjà fuzzé), décrits en **WIT** (`inkwell:plugin@1.0.0`), d'où
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
   blanche, rendu par Svelte) ; `ui.canvas` en `iframe` `sandbox` sans `allow-same-origin` pour
   l'exceptionnel ; jamais de HTML, de CSS ni de JavaScript de greffon dans l'origine de
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
| **P-PLUG-BENCH** | banc `benchstat` (`-count ≥ 10`) sur trois greffons types (Rust, Go `wasip1`, TinyGo) : compilation, instanciation (froide, depuis le cache), appel vide, appel 64 Kio aller et retour, aller-retour entre processus, mémoire résidente de l'hôte, taille ajoutée à l'exécutable. Seuils : appel p95 ≤ 2 ms tout compris pour 64 Kio ; démarrage à froid d'un greffon ≤ 150 ms ; 0 ms ajoutée avant la première peinture. **Prérequis de l'acceptation.** |
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
| SC-5 | Vues déclaratives : liste blanche, aucun `{@html}`, `ui.canvas` sans même origine ni réseau ; aucun déguisement en dialogue du cœur (PP-5) |
| SC-6 | Règle d'exclusion `net.hosts` / lecture du clair ; dorsal de stockage : chiffrement avant le greffon, clé jamais transmise (§6.7) |
| SC-7 | Paquets : zip (taille décompressée, chemins, liens), signature, continuité de clé, version monotone, revérification avant chargement |
| SC-8 | Greffons de projet : décision d'admin vérifiée, blob vérifié, niveau « projet », accord local au-delà, interrupteur, isolation par projet (PP-1 à PP-7) |
| SC-9 | Thèmes : données typées seulement, contrôle des contrastes, aucune URL |
| SC-10 | Bus d'événements : aucune donnée de document dans une charge ; files bornées |

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
3. Niveau « projet » chargé sans question (N23) : confirmer la liste du §15.10.
