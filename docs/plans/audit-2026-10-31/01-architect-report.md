# Audit d'architecture inkwell — constats (agent architect, 2026-10-31)

Référentiel, seuils et barème : `docs/plans/2026-10-31-architecture-audit.md`. Lecture seule, aucun fichier du dépôt modifié.

## Outils, versions, commandes (reproductibles)

- go 1.27.3 linux/amd64 ; golangci-lint 2.15.0 (via `mise exec`) ; lukechampine.com/blake3 v1.4.1.
- Graphe : `go list -f '{{.ImportPath}}: {{join .Imports " "}}' ./internal/... ./cmd/...`
- Complexité / duplication : `mise exec -- golangci-lint run -c <scratch>/metrics.yml ./...` avec `gocognit`, `gocyclo` (min-complexity 15 ⇒ signale > 15) et `dupl` (threshold 100), fichiers `_test.go` exclus. Le `.golangci.yml` du dépôt n'active aucun des trois.
- Couverture par paquet : (a) `scripts/heavy.sh go test -short -cover -p 2 ./internal/...` ; (b) comme `scripts/cover.sh` : `go test -short -covermode=atomic -coverpkg=./... -coverprofile=cov.out ./...`, puis agrégation par paquet (blocs dédoublonnés).
- Chemins critiques : `scripts/heavy.sh go test -run '^$' -bench . -benchmem -count 3 -benchtime 200ms ./internal/blob ./internal/transport/frame ./internal/transport/mux` ; profil : `go test -c -o blob.test ./internal/blob && ./blob.test -test.bench 'Fetch|ReadSlice' -test.benchtime 20x -test.memprofile blob.mem && go tool pprof -top -sample_index=alloc_space`.
- `go vet ./internal/... ./cmd/...` : aucun constat.

## Synthèse

| Gravité | Nombre | IDs |
|---|---|---|
| Critique | 2 | ARC-C1, ARC-C2 |
| Majeur | 5 | ARC-M1 … ARC-M5 |
| Mineur | 11 | ARC-m1 … ARC-m11 |
| Écartés | 6 | voir fin |

---

## Critique

### ARC-C1 · Critique · arête interdite `internal/trust` → `internal/transport`
- **Règle** : `docs/architecture.md:74` (« `trust` et `identity` n'importent pas `transport` ») ; `engineering-principles.md` §2, ligne 3 (dépendances orientées vers le cœur, preuve « test d'architecture `go list -deps` en CI ») ; barème du référentiel : « toute arête interdite = critique ».
- **Preuve** : `internal/trust/sync.go:10` importe `internal/transport` ; usages : `sync.go:31,46,47,57,63,89,121,136,149,152` (uniquement le type `transport.PeerID`). Commande : `go list -f '{{join .Imports "\n"}}' ./internal/trust | grep inkwell` → `…/internal/transport`. Cause racine : `scripts/arch-check.sh` ne vérifie que libp2p, wormhole, `sidecar/fetch`, automerge et `net` dans `sidecar`/`typst` ; la règle de cœur n'a **aucune preuve automatisée**, donc l'arête est passée (introduite par `f304a59`, 2026-10-25).
- **Correctif proposé** (3 options) :
  - **A (recommandée)** : rendre `Sync` générique sur la clé de pair : `type Sync[P cmp.Ordered] struct`, `Outbound[P]`, `NewSync[P cmp.Ordered](l *List) *Sync[P]`. `app` instancie `trust.NewSync[transport.PeerID]`. Coût nul à l'exécution, `trust` ne connaît plus le vocabulaire réseau, `FuzzDecodeMessage` reste en place.
  - B : sortir `Sync` (réplication sur le réseau) vers la couche session (voir ARC-M5). C'est plus juste conceptuellement, mais plus gros ; à faire en même temps que ARC-M5 si ARC-M5 est retenu.
  - C : ADR qui assouplit `architecture.md:74` (« un type feuille de `transport` est permis »). Déconseillé : la règle protège justement le cœur de la confiance contre toute dérive vers le réseau.
- **Test** : étendre `scripts/arch-check.sh` avec une liste d'interdits par `go list -deps` (pas seulement les imports directs). `internal/transport` (racine) n'importe aucun `internal/*`. `internal/trust` et `internal/identity` n'atteignent ni `internal/transport…` ni `app`, `bridge`, `project`, `store`, `blob`, `typst`, `sidecar`, `invite`. Ajouter un cas négatif au test du script. Je préfère étendre le script plutôt qu'ajouter go-arch-lint ou arch-go : le script ne coûte aucune dépendance (ADR 0008) et la règle se lit au même endroit que les autres.

### ARC-C2 · Critique (remède documentaire) · trafic réseau d'invitation hors de `transport.Transport`
- **Règle** : invariant `CLAUDE.md:45` (« Tout le réseau passe par l'interface `internal/transport.Transport` ») ; ADR 0004, titre et « Décision » (« go-libp2p + wormhole-william, **derrière** … `Transport` »).
- **Preuve** : `internal/invite/wormhole.go:49` (`w.client().SendText`) et `:78` (`w.client().Receive`) parlent au serveur de rendez-vous et au relais Wormhole directement, derrière le type `invite.Wormhole` et pas derrière `transport.Transport`. Le référentiel se contredit : `docs/architecture.md:76` et `scripts/arch-check.sh:35-38` **autorisent** wormhole-william dans `internal/invite`.
- **Correctif proposé** : une ADR qui remplace ADR 0004 sur ce point, et la mise à jour de l'invariant. Options :
  - **a (recommandée)** : garder le code tel quel et reformuler. Le trafic **entre pairs** passe par `Transport`. Le canal de rendez-vous d'invitation (un texte unique, PAKE) est isolé dans `internal/invite`, derrière une petite interface du consommateur (`invite.Lobby`, `flow.go:441`).
  - b : envelopper Wormhole comme un `transport.Transport`. Mauvaise forme : Wormhole n'a ni `PeerID` ni flux continu, c'est un échange unique protégé par un code.
  - c : déplacer le code dans `internal/transport/wormhole` avec sa propre interface. Cela ne change rien à la sécurité et brouille le rôle de `transport`.
- **Test** : le contrôle existant d'`arch-check.sh` (wormhole seulement dans `invite`) devient la preuve de l'invariant reformulé. Ajouter : aucun paquet hors `transport/p2p`, `invite` et `sidecar/fetch` (construction seulement) n'importe `net` en dehors de `net/http` côté serveur d'assets (aujourd'hui seul `app/csp.go` importe `net/http`, pour un middleware).

---

## Majeur

### ARC-M1 · Majeur · allocations prouvées dans le chemin critique des blobs (service et téléchargement)
- **Règle** : `engineering-principles.md` §3 (« Pas d'allocation dans les boucles chaudes (… hachage de blobs) ») ; `internal/blob` figure dans `HOT_PATHS:`.
- **Preuve** (bench ci-dessus, i7-1370P) :
  - `BenchmarkReadSlice` : 3 237 7xx B/op, 39 allocs/op pour une tranche de 1 Mio (`MaxSlice`, `wire.go:20`), soit ≈ 3,1 × la charge utile.
  - `BenchmarkFetch` : 119,3 Mo/op pour un blob de 16 Mio, soit ≈ 7,1 × les octets transférés.
  - pprof `alloc_space` : `bao.DecodeSlice` pèse 1 328 Mo cumulés sur 2 404 Mo (55 %), dont `bytes.growSlice` à 69 % en plat. Ces allocations viennent de `bao.VerifySlice`, qui décode dans un `bytes.Buffer` non dimensionné (bao.go:333-339 de la bibliothèque).
  - Côté service : `internal/blob/store.go:609` appelle `VerifySlice` et **jette** les octets vérifiés (`_`). Côté réception : `service.go:755`. En plus, `store.go:623` relit l'outboard entier (taille/256 octets, ≈ 400 Kio pour un fichier de 100 Mo, `DefaultMaxFile`) à **chaque** tranche servie.
  - `bytes.Clone` (333 Mo) vient de `memnet` (transport de test) : ce n'est pas un constat.
- **Correctif proposé** :
  1. `ReadSlice` : remplacer `VerifySlice` par `bao.DecodeSlice(io.Discard, rd, groupLog, offset, n, h)` puis vérifier `rd.Len()==0`. La sémantique est la même que `VerifySlice` et la copie disparaît.
  2. `checkSlice` : décoder dans un tampon pré-dimensionné (`bytes.NewBuffer(make([]byte, 0, n))`), ou mieux, écrire directement dans le fichier partiel.
  3. Réutiliser le tampon de 1 Mio de `ReadSlice` par un `sync.Pool`, comme `mux` le fait déjà (`mux.go` `bufs`).
  4. Passer l'outboard au lecteur séquentiel par un `bufio.Reader` sur le fichier ouvert dans `os.Root`, au lieu de `ReadFile`. `ExtractSlice` lit l'outboard séquentiellement.
- **Test** : `mise run bench:baseline` → correctif → `bench:compare` (`-count 10`). Attendu : B/op de `ReadSlice` ≤ 1,1 Mio et B/op de `Fetch` divisé par ≥ 2, sans régression de ns/op. `FuzzCheckSliceForgery`, `FuzzBlobWire` et les tests de corruption restent verts.

### ARC-M2 · Majeur · une allocation de `context` par événement dans la boucle de relais des updates
- **Règle** : `engineering-principles.md` §3 (« Pas d'allocation dans les boucles chaudes (relais d'updates …) »).
- **Preuve** : `internal/app/wiring.go:340-349`. `poll` crée puis annule un `context.WithCancel(context.Background())` à chaque appel. Il est appelé 3 fois par tour (`:358`, `:372`, `:376`), donc au moins 3 fois par trame relayée, plus 3 fois au tour à vide avant de bloquer. Micro-bench de la construction exacte (programme jetable du scratchpad, `go test -bench . -benchmem -count 3`) : **75 ns, 96 B, 2 allocs par `poll`**, soit au moins 6 allocs et 288 B par trame relayée, sans compter le reste. `internal/app` ne figure pas dans `HOT_PATHS` et n'a aucun `Benchmark`.
- **Correctif proposé** : créer **un** contexte annulé dans `Wire` (champ `polled context.Context`) et le réutiliser. Mieux : exposer dans `mux` un `Poll(t) (Event, bool)` non bloquant, voisin de `mux.Wake`, qui évite tout contexte.
- **Test** : `BenchmarkRelay` (memnet, 2 pairs, N trames, `-benchmem`), avant/après avec benchstat ; ajouter le paquet qui porte `Wiring` à `HOT_PATHS:`.

### ARC-M3 · Majeur · complexité cognitive > 30
- **Règle** : seuil du référentiel (gocognit > 30 = majeur) ; Google Go Style, best-practices.
- **Preuve** (gocognit, commande ci-dessus) :
  - `internal/project/watch.go:27` `(*Project).Watch` **57**
  - `internal/trust/fold.go:416` `(*sweep).members` **52**
  - `internal/trust/fold.go:270` `(*sweep).decide` **39**
  - `internal/typst/packages.go:337` `readArchive` **35**
  - `internal/typst/watch.go:32` `(*Engine).Watch` **35**
  - `internal/trust/list.go:452` `(*List).capPrev` **34**
- **Correctif proposé** :
  - `project.Watch` : en faire une petite machine d'état, un type `watcher` (champs `dirty`, `held`, `dirs`, `known`, `timer`) avec les méthodes `refresh`, `onEvent` et `flush`. La boucle `select` ne fait plus que répartir.
  - `trust` : extraire les prédicats de `decide`/`members` (rang, ancienneté, révocation) en fonctions pures. C'est du code de sécurité : refactorer sous `mise run mutate:go -- ./internal/trust`, sans baisse du score de mutation.
  - `typst.Engine.Watch` n'a **aucun appelant de production** (ARC-m2) : le supprimer plutôt que le refactorer.
  - `readArchive` : séparer la lecture d'en-tête, la validation de nom et la copie bornée.
- **Test** : gocognit ≤ 30 sur ces fonctions. Activer `gocognit` (bloquant au-delà de 30, signalé au-delà de 15), `gocyclo` et `dupl` dans `.golangci.yml` ; les tests existants, les fuzz et les mutants restent verts.

### ARC-M4 · Majeur · couverture par paquet < 85 % et gate seulement global
- **Règle** : seuil du référentiel (« Couverture par paquet < 85 % = majeur »).
- **Preuve** :
  - `internal/sidecar/fetch` : **83,8 %** avec les deux méthodes (a) et (b).
  - `internal/transport/transporttest` : **77,7 %** en `-coverpkg=./...` (0 % en propre). C'est un paquet de support de test, mais un paquet non-test.
  - `scripts/cover.sh:28-36` ne calcule que le **total**, donc un paquet peut passer sous 85 % sans faire échouer `cover:check`.
  - Les autres paquets sont ≥ 85 % (blob 87,1, store 86,7, invite 87,3, p2p 88,5, app 89,2, etc.).
- **Correctif proposé** : tester les branches d'échec de `fetch` (archive tronquée, hachage faux, renommage refusé). Pour `transporttest`, soit l'exclure explicitement dans `cover.sh` avec une justification écrite (comme `cmd/`), soit le couvrir. Enfin, rendre `cover.sh --check` **par paquet** : `go tool cover -func` agrégé par répertoire, échec si l'un d'eux est < `COVER_MIN`.
- **Test** : `mise run cover:check` échoue si un paquet est < 85 % (cas négatif scripté).

### ARC-M5 · Majeur · `internal/app` cumule quatre rôles, dont la couche session critique pour la sécurité
- **Règle** : `engineering-principles.md` §2 (« Un paquet = un rôle ») ; `docs/architecture.md:36-41` (couche « Session : identité, invitations, liste signée, relais des updates » distincte du pont et des services) et `:51` (« `app` = services Wails … la composition vit dans `cmd/inkwell` »). Aucune ADR ne place la session dans `app` ; seul le plan `docs/plans/2026-10-25-etape-2-reseau.md:288` le fait.
- **Preuve** :
  - `internal/app` compte 16 fichiers et 4 178 lignes. Il contient à la fois :
    - la composition : `Start`, `node.go:156`, ce qui contredit `architecture.md:51` ;
    - la couche session : `Wiring`, `Admission`, `attestStore`, `opStore` (`wiring.go`, `attest.go`, `persist.go`, `people.go`, `suspend.go`), qui appliquent le contrôle d'écriture de l'ADR 0018 ;
    - le pipeline d'aperçu : `preview.go`, 825 lignes ;
    - les façades Wails.
  - Impact mesuré : `go list -deps ./internal/app | wc -l` = **610** contre 111 pour `./internal/trust`, et le binaire de test `app.test` pèse **104 Mo** contre 13 Mo pour `trust.test`. Chaque test de la couche session lie donc libp2p, `typst` et les 57 Mo de sidecars embarqués (`internal/sidecar/bin`).
  - L'ADR 0030 (proposée) ajoute `internal/policy`, dont le consommateur sera `Wiring`, ce qui aggraverait le couplage.
- **Correctif proposé** : extraire `internal/session` (voir la restructuration). `app` garde les services liés au frontend et l'aperçu. La composition va soit dans `cmd/inkwell`, soit `architecture.md:51` est corrigé : c'est une décision documentaire, sans ADR existante à remplacer.
- **Test** : règle `arch-check.sh` : `session` n'atteint ni `typst`, ni `sidecar`, ni `blob`, ni `project`, ni `app`. Taille de `session.test` < 20 Mo ; tests de convergence et de contrôle d'écriture déplacés et verts.

---

## Mineur

### ARC-m1 · Mineur · variables globales mutables servant de crochets de test
- **Règle** : `engineering-principles.md` §2 (« Aucune variable globale mutable … injection explicite par constructeur »).
- **Preuve** :
  - `internal/sidecar/run.go:207` `isolationOverride`, réécrit par `limits_test.go:124` ;
  - `internal/typst/attachments.go:17,20` `maxAttachmentBytes`/`maxTotalAttachmentBytes`, réécrits par `attachments_test.go:135,179` et `unit_test.go:172` ;
  - `internal/typst/engine.go:60` `maxScanFiles`, réécrit par `unit_test.go:236` ;
  - `internal/typst/engine.go:65` `DefaultTimestamp`, **exporté** et réassignable, lu par `internal/app/preview.go:245` ;
  - `internal/identity/fingerprint.go:104` `fpState`, remis à zéro par `export_test.go:23`.
  - Aucune course aujourd'hui : aucun `t.Parallel` dans ces paquets (grep). Impact non mesuré.
- **Correctif proposé** :
  - les limites deviennent des champs (`typst.Config`, `sidecar.Spec`), avec valeur par défaut à zéro ;
  - `DefaultTimestamp` devient non exporté avec un accesseur, ou un défaut de `Config` ;
  - les tests de `fingerprints` utilisent une instance locale, `newFingerprints(f)`, au lieu de réinitialiser le global.
- **Test** : ces tests passent avec `t.Parallel()` sous `-race`.

### ARC-m2 · Mineur · API exportée sans appelant de production, code spéculatif
- **Règle** : `engineering-principles.md` §2 (« API exportée minimale ») ; `CLAUDE.md` (« pas de … "pour plus tard" »).
- **Preuve** (grep des appels hors `_test.go`) :
  - `typst.(*Engine).Watch` (`watch.go:32`, gocognit 35, 2 goroutines, fsnotify) ;
  - `sidecar.NewSupervisor` (`supervisor.go:145`, 264 lignes : le LSP tinymist n'est pas branché) ;
  - `sidecar.Licenses` (`licenses.go:16`) ;
  - `typst.Attachments` (`attachments.go:58`, analyseur PDF de 464 lignes, utilisé seulement pour vérifier l'export en test) ;
  - `project.Convert` (`text.go:128`) ;
  - `app.StartNode` (`node.go:69`) ;
  - `bridge` : 10 codecs exportés sans appelant hors tests (`DecodeBlobInfo`, `DecodeFileList`, `DecodeRange`, `DecodeTreeResult`, `EncodeReferenced`, `EncodeStoreRequest`, `EncodeTree`, `EncodeWriteAt`, `DecodeFileInfo`, `DecodeStoreResponse`).
- **Correctif proposé** : ce qui sert aux tests passe dans `export_test.go`. Le code spéculatif (`Engine.Watch`, `Supervisor`) est supprimé, ou branché à l'étape qui en a besoin. Ajouter `deadcode` (golang.org/x/tools/cmd/deadcode) à `mise run ci`.
- **Test** : `deadcode ./cmd/...` ne rapporte rien d'exporté hors liste d'exceptions motivées.

### ARC-m3 · Mineur · `tinymist.gz` (32,8 Mo) embarqué mais jamais lancé
- **Règle** : ISO/IEC 25010 efficacité (utilisation des ressources) ; décision d'ADR 0024 (deux binaires embarqués).
- **Preuve** : `ls -l internal/sidecar/bin/linux_amd64/` donne `tinymist.gz` 32 814 010 octets et `typst.gz` 25 656 310 octets. `sidecar.Tinymist` n'est référencé que par `manifest.go:23`, et `Supervisor` n'a pas d'appelant (ARC-m2). Cela fait 56 % de la charge utile embarquée sans usage.
- **Correctif proposé** (frontière ADR 0024) :
  - a : laisser tel quel jusqu'au LSP ;
  - b : n'embarquer tinymist que sous une étiquette de construction tant que le LSP n'est pas branché, par amendement de l'ADR 0024 ;
  - c : supprimer tinymist du manifeste jusqu'à l'étape LSP.
  - Recommandation : b, si une version sort avant le LSP ; sinon a.
- **Test** : taille du binaire de version mesurée en CI, plus un test que `manifest.json` ne liste que des sidecars lancés.

### ARC-m4 · Mineur · interface `bridge.Storage` de 17 méthodes
- **Règle** : `engineering-principles.md` §2 (« Interfaces petites ») ; Go Code Review Comments, section Interfaces.
- **Preuve** : `internal/bridge/storestream.go:364-389`, un seul consommateur (`StoreServer`) dont `handle` a un gocyclo de 22 (`:490`) et `validate` de 19 (`:168`). Le seuil « à signaler » est dépassé, pas le seuil majeur.
- **Correctif proposé** : trois ports, `KV` (Load/Save/Remove/LoadRange/RemoveRange), `Projection` (Read/Write/Ack/SyncPoint/ReadText/List) et `Tree` (SetTree/SetReferenced/PutBlob). `StoreServer` compose trois gestionnaires ; `app.Files` continue d'implémenter les trois (assertions `var _`).
- **Test** : gocyclo ≤ 15 sur `handle`/`validate` ; `FuzzStoreRequest` et les vecteurs `storestream.json` restent verts.

### ARC-m5 · Mineur · complexité au-delà de 15 (signalement)
- **Règle** : seuils du référentiel (gocognit et gocyclo > 15 à signaler).
- **Preuve** : 36 fonctions sont entre 16 et 30 en gocognit, dont `mux.(*muxer).run` 30, `p2p.New` 30, `typst.Compile` 23, `trust.(*List).apply` 23, `op.validate` 23, `blob.(*Store).scan` 27 et `typst.skipValue` 26. En gocyclo : `storestream.go:168` 19, `:490` 22, `fragment.go:232` 18, `attachments.go:404` 18, `text.go:240` 16, `fetch.go:256` 16. Liste complète : sortie de la commande ci-dessus.
- **Correctif proposé** : activer les linters (voir ARC-M3) et traiter ces fonctions au fil de l'eau.
- **Test** : gate golangci.

### ARC-m6 · Mineur · code dupliqué dans `transport/p2p`
- **Règle** : seuil `dupl` ≥ 100 jetons.
- **Preuve** : `p2p/bulk.go:100-121` ≡ `p2p/node.go:669-690` (file `Recv`) ; `p2p/bulk.go:162-183` ≡ `p2p/stream.go:230-251` (boucle de lecture avec `ReadReserve` et budget).
- **Correctif proposé** : un type non exporté `inbox` (file, réveil, budget ; `push`/`pop`) et une fonction `readLoop(s, budget, deliver)` partagés par `Node` et `Bulk`.
- **Test** : `dupl` sans constat ; `transporttest` suite verte pour les deux.

### ARC-m7 · Mineur · arrêt sans attente des goroutines lancées par `main`
- **Règle** : référentiel Concurrence (propriétaire et condition de sortie) ; Google Go Style, best-practices, section « Goroutine lifetimes ».
- **Preuve** : `cmd/inkwell/main.go:129-145` lance `proj.Watch` et `router.Run` sans `WaitGroup`. `OnShutdown` (`:114-121`) annule `ctx` puis ferme `rt` et `proj` sans attendre ces goroutines. Un `p.check` en cours peut donc lire un `os.Root` fermé, et le rappel peut appeler `Preview.Changed` après `rt.Close`. Ce n'est pas démontré en exécution, d'où la gravité mineure.
- **Correctif proposé** : `rt` possède ces goroutines, par exemple `Runtime.Run(ctx, proj, router, storeServer)` avec un `sync.WaitGroup`. L'ordre devient : annuler, attendre, `rt.Close`, `proj.Close`.
- **Test** : test d'arrêt avec `goleak.VerifyNone` sur le `Runtime`.

### ARC-m8 · Mineur · `internal/blob` lance des goroutines sans vérification goleak
- **Règle** : référentiel Concurrence (goleak).
- **Preuve** : `internal/blob/service.go:239,247` (`wg.Go`). `internal/blob` est le seul paquet qui lance des goroutines sans `goleak` dans ses tests (boucle `grep -l goleak` par paquet ; les autres sans goleak, `transport` et `version`, n'en lancent pas).
- **Correctif proposé** : `TestMain` avec `goleak.VerifyTestMain(m)`.
- **Test** : le paquet passe sous goleak.

### ARC-m9 · Mineur · E/S Go sur le dossier projet hors `os.Root` dans le moteur Typst
- **Règle** : `engineering-principles.md` §4 (« E/S fichier du projet uniquement via `os.Root` »).
- **Preuve** : `internal/typst/engine.go:311` (`os.Stat(root/fonts)` suit les liens), `:393` et `:421` (`os.ReadDir` de `.typstdeps`), `watch.go:63`. Ces accès sont atténués par `scanLinks(root)` (`engine.go:300`) et par le type `DirEntry` (Lstat), mais une fenêtre TOCTOU reste ouverte entre le scan et l'usage. Un pair ne peut pas créer de lien (le projet est écrit via `os.Root`), d'où l'absence d'exploit démontré.
- **Correctif proposé** : ouvrir un `os.Root` du projet par tâche (`statInRoot` le fait déjà, `:338`) et utiliser `root.Stat` et `fs.ReadDir(root.FS(), …)`.
- **Test** : test d'évasion par lien posé entre `scanLinks` et `packageView` (crochet injecté) → refus.

### ARC-m10 · Mineur · flux `preview` et `present` sans vecteurs dorés partagés Go/TS
- **Règle** : `engineering-principles.md` §4 (« chaque message du pont validé et typé … preuve : test du pont »).
- **Preuve** : `net` et `store` partagent des vecteurs (`frontend/src/net/envelope.test.ts:2`, `frontend/src/store/protocol.test.ts:2` → `internal/bridge/testdata/*.json`). Le JSON d'aperçu est tapé à la main des deux côtés : `internal/app/preview.go:354-385` et les étiquettes `json` de `typst` d'un côté, `frontend/src/preview/protocol.ts` de l'autre ; pas de vecteurs pour `present.go:220`. Aucun écart aujourd'hui (TS normalise les champs `omitzero` : `protocol.ts:153,162,169,208`).
- **Correctif proposé** : `internal/app/testdata/preview.json` et `present.json` écrits par un test Go doré (`-update`), lus par Vitest.
- **Test** : un renommage d'étiquette côté Go casse le test TS.

### ARC-m11 · Mineur · écarts de documentation d'architecture
- **Règle** : règles du projet (« une seule source de vérité » ; `architecture.md` fait partie du référentiel).
- **Preuve** :
  - le tableau des paquets omet `internal/transport/mux`, `internal/transport/transporttest` et `internal/version` ;
  - `:51` place la composition dans `cmd/inkwell`, alors que `app.Start` (`node.go:156`) compose ;
  - ADR 0004 décrit `Recv() <-chan Msg` alors que le code a `Recv(ctx) (Event, error)` (`transport.go`).
- **Correctif proposé** : mettre la documentation à jour en même temps qu'ARC-C2 et ARC-M5.
- **Test** : revue ; plus une ligne dans `arch-check.sh` qui compare `go list ./internal/...` aux paquets cités dans `architecture.md`.

---

## Écartés (pas de règle ou pas de preuve) — à noter, non retenus

1. **Couches du frontend** : `editor/remoteCursors.ts:3`, `history/people.ts:4` et `history/csvdiff.ts:1` importent depuis `ui/`. Il y a aussi un cycle de types `net` ↔ `store` (`net/coverage.ts:2` ↔ `store/GoStorageAdapter.ts:2`). Le référentiel n'a **aucune règle de dépendance frontend**, donc pas de constat. Recommandation : écrire la règle (par exemple domaine → jamais `ui/`) et la prouver en CI avec dependency-cruiser (graphe résolu, CI) ou eslint-plugin-boundaries (retour dans l'éditeur). À vérifier : la prise en charge des fichiers `.svelte` par l'outil choisi.
2. **Découverte d'interfaces optionnelles** dans `bridge.NewRouter` (`router.go:66-67`) : idiome Go (à la `io.WriterTo`), qui échoue fermé (`router.go:262`).
3. **`context` stocké dans des structs** (`Wiring`, `Preview`, `muxer`) : ce sont des boucles de travail avec propriétaire et `Close` qui attend ; idiomatique.
4. **`frame.ReadReserve`** (2,03 Mo/op pour une trame de 1 Mio, croissance par doublement) : défense documentée contre la préallocation (`frame.go:56-58`). Piste d'amélioration à prouver par benchstat : quand `reserve` a déjà accepté la taille (le budget est donc débité), allouer `n` d'un coup. Le pire cas mémoire est inchangé (il est égal au budget) et la copie disparaît.
5. **`mux.Split`** sur `context.Background()` (`mux.go:98`) : la durée de vie est possédée par `Close` des sous-transports.
6. **Lint, race, govulncheck, gosec** : hors du périmètre structurel. `go vet` est propre ; le reste est à confier à `quality-runner`.

---

## Restructuration proposée (ordonnée par dépendances)

1. **Gates d'abord.** Ils préviennent les récidives et prouvent les étapes suivantes.
   - `scripts/arch-check.sh` : interdits de cœur par `go list -deps`, plus les règles `session` de l'étape 4.
   - `.golangci.yml` : `gocognit` (échec au-delà de 30), `gocyclo` et `dupl`.
   - `cover.sh --check` par paquet.
   - `deadcode` dans `ci`.
   - `goleak` dans `blob`.
2. **ARC-C1** : `trust.Sync[P cmp.Ordered]`. Le gate de l'étape 1 passe au vert.
3. **ARC-C2 et ARC-m11** : ADR qui remplace ADR 0004 sur le canal d'invitation, reformulation de `CLAUDE.md:45`, mise à jour d'`architecture.md`. Documentation seulement.
4. **ARC-M5** : extraire `internal/session`.
   - Contenu : `Wiring`, `Admission`, `attestStore`, `opStore`, la logique « personnes » et suspension.
   - Dépendances autorisées : `transport`, `transport/mux`, `trust`, `identity`, `bridge` (pour `Envelope` et les ports `Source`/`Attester` qu'il implémente).
   - Interdit : `app`, `typst`, `sidecar`, `blob`, `project`.
   - `app` garde les façades Wails et `Preview`, et importe `session`.
   - Ce découpage prépare l'ADR 0030 : `internal/policy` sera consommé par `session`, pas par `app`. Dépend de 2, qui stabilise la signature de `Sync`.
5. **Chemins critiques avec benchstat** : ARC-M1 (`blob`) et ARC-M2 (`session.Wiring.poll`, puis `internal/session` ajouté à `HOT_PATHS:` avec `BenchmarkRelay`).
6. **Code mort avant refactor** (ARC-m2) : supprimer `typst.Engine.Watch` (retire une des fonctions d'ARC-M3), passer les codecs de test de `bridge` en `export_test.go`, décider de `Supervisor` et tinymist (ARC-m3, ADR 0024).
7. **Complexité** (ARC-M3) : `project.Watch` en type `watcher`, prédicats de `trust` sous mutation testing, `readArchive`.
8. **Nettoyages mineurs** : variables globales vers config (m1), découpe de `Storage` (m4), `inbox` de p2p (m6), attente à l'arrêt via `Runtime.Run` (m7), `os.Root` dans `typst` (m9), vecteurs dorés preview et present (m10).

Sources (état de l'art des contrôles d'architecture) : [go-arch-lint](https://pkg.go.dev/github.com/fe3dback/go-arch-lint), [arch-go](https://pkg.go.dev/github.com/fdaines/arch-go), [eslint-plugin-boundaries](https://npmjs.com/package/eslint-plugin-boundaries), [dependency-cruiser (Xebia)](https://xebia.com/blog/taking-frontend-architecture-serious-with-dependency-cruiser/), Google Go Style Guide, Go Code Review Comments (interfaces), API `lukechampine.com/blake3/bao` v1.4.1 (`DecodeSlice`, `VerifySlice`).
