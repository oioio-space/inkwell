---
status: proposed
date: 2026-10-26
decision-makers: [Mathieu]
---

# Performance : paliers par taille, Automerge hors du fil d'affichage, gros fichiers lus par Go, réseau lent

## Contexte et problème

Exigence U33 : « très performant, y compris par Internet, surtout sur les fichiers texte énormes à
afficher ». L'ADR 0026 a fixé un seuil de 10 Mo au-delà duquel un fichier s'ouvre en lecture
seule, **mais toujours dans un document Automerge**. Résultat : l'e2e SHEET-BIG (CSV de 12 Mo)
demande plus de 8 Go de mémoire dans le navigateur et échoue sous `scripts/heavy.sh`. Il fallait des
mesures plutôt que des suppositions.

### Mesures (2026-10-26, Node 22.23 / V8, Automerge 3.5.0, CodeMirror state 6.7.6, Go 1.27)

Texte de type prose (rapport gzip 3,5:1), construit par modifications de 64 Kio, sauf mention.
Les scripts de mesure entrent dans le dépôt avec PERF-18 ; tout est à remesurer dans WebKitGTK (PERF-01).
Convention : Ki et Mi comptent des caractères (1 024 et 1 048 576), Kio, Mio et Mo des octets.

| Texte | Ouverture (`load`) | Mémoire WASM après ouverture | Frappe locale p50 | **Modification distante appliquée** | Message de synchro initial |
|---|---|---|---|---|---|
| 100 Kio | – | – | 0,18 ms | **21 ms** | – |
| 256 Kio | – | – | 0,23 ms | **52 ms** | – |
| 512 Kio | – | – | 0,39 ms | **104 ms** (application 123 + différence 86) | – |
| 1 Mio | 0,48 s | 145 Mo | 0,66 ms | **240 à 420 ms** | 0,3 Mo, reçu en 0,6 s |
| 5 Mio | 2,6 s | 660 Mo | 1,9 à 7 ms | **3,3 s** | 1,6 Mo, reçu en 3,9 s |
| 10 Mio | 4,3 s | 1,3 Go | 3,6 à 5,6 ms | (tué : 2 copies > 4 Go) | 3,2 Mo, reçu en 5,5 s |
| 20 Mio | 9,4 s | 2,6 Go | 8 ms | – | 6,3 Mo, reçu en 13 s |

Constats :

1. **Appliquer une modification venue d'un pair coûte O(taille du texte)** : environ 0,2 à 0,4 ms par
   Kio, dans le cœur Rust (`applyChanges` seul : 123 ms à 512 Kio), alors que la frappe locale coûte
   moins d'une milliseconde. Un document de 1 Mio fige donc le fil d'affichage 0,25 à 0,4 s **à chaque
   lot reçu**. Rien de tel n'est signalé chez Automerge (recherche du 2026-10-26) ; c'est cohérent avec
   la représentation compressée gardée en mémoire depuis Automerge 3.
2. **Ce coût est par appel, pas par modification** : 1, 10 ou 50 frappes appliquées d'un coup coûtent
   108, 106 et 107 ms à 512 Kio. Regrouper est donc le levier principal.
3. **Mémoire à l'ouverture : environ 128 octets WASM par caractère**, jamais rendus (la mémoire
   linéaire WASM ne rétrécit pas). La limite de 4 Gio du WASM 32 bits tombe donc vers 30 M caractères
   **pour tous les documents chargés d'un coup**, puisqu'ils partagent une instance.
4. **Un collage en une seule modification coûte environ 300 octets par caractère** (5 Mio : 1,5 Go ;
   10 Mio : 3,2 Go), que le texte arrive en une ou en 80 `splice`. Découpé en modifications de 64 Kio, il
   coûte 7 octets par caractère (10 Mio : 70 Mo). **C'est la cause de SHEET-BIG** : 12 Mo collés d'un
   coup font environ 3,6 Go de WASM avant les copies du texte, de l'index et de la projection.
5. La « panique vers 20 Mo » de l'ADR 0026 est une affaire de mémoire, pas de taille : construit par
   morceaux, un texte de 20 Mio se crée (9,6 s) et s'ouvre (9,4 s, 2,6 Go).
6. **Le message de synchro initial est déjà compressé** : Automerge envoie l'instantané entier à un pair
   vide ([Merry Commitmas](https://automerge.org/blog/merry-commitmas/)), et gzip n'y gagne rien (0 %).
   Les messages de frappe font environ 200 octets.
7. **CodeMirror n'est pas le goulot** : un état de 100 Mio se crée en 219 ms, une modification coûte
   11 µs, `lineAt` 2,6 µs (arbre de lignes équilibré). L'analyse syntaxique est paresseuse et s'arrête
   hors de la zone visible ([exemple « million »](https://codemirror.net/examples/million/)).
8. **Go lit un gros fichier très vite** : un index creux (un repère toutes les 1 024 lignes) se
   construit en 35 ms pour 100 Mo et 0,33 s pour 1 Go (cache chaud), et pèse 11 Kio par 100 Mo. Une
   page de 64 Kio se lit en 8 µs, et une recherche littérale parcourt 100 Mo en 43 ms.
9. **Réseau** : `p2p` borne chaque envoi à `10 s + taille / (1 Mio/s)`. Sur un lien relayé à 1 Mbit/s,
   un message initial de 6 Mo (texte de 20 Mio) prend 48 s : l'envoi échoue, le flux est réinitialisé,
   automerge-repo renvoie tout, **sans fin**. Pendant ce temps, la trame occupe le flux de synchro du
   pair (`wsem`) et retient les frappes de **tous** les autres documents (blocage en tête de file).

## Facteurs de décision

- Le fil d'affichage ne se fige jamais plus d'une image (16 ms) à cause d'un autre pair.
- La mémoire est un budget de premier rang : aucune action d'un utilisateur ou d'un pair ne doit
  pouvoir faire dépasser 4 Gio au WASM, ni 1,5 Go à la webview.
- Une seule source de vérité par chose (CLAUDE.md) ; Go ne décode jamais Automerge (ADR 0011).
- Le réseau réel : environ 70 % des tentatives de perçage de NAT réussissent, en TCP comme en QUIC
  ([mesure DCUtR à grande échelle, 2025](https://arxiv.org/html/2510.27500v1)). Un tiers des liens
  restent donc relayés, souvent par la connexion montante d'un particulier.

## Options envisagées

- **A. Statu quo** : seuil à 10 Mo, lecture seule dans Automerge. Rejeté par les constats 3, 4 et
  SHEET-BIG.
- **B. Changer de CRDT** (Loro : Fugue et idées d'Eg-walker, instantanés « superficiels » sans
  l'historique ancien, ouverture en moins d'une milliseconde pour 1,6 M opérations,
  [Loro 1.0](https://loro.dev/blog/v1.0)). C'est prometteur, mais cela remet en cause les ADR 0002,
  0011 et 0018 (la couverture repose sur les hachages de modifications Automerge). **Gardé comme
  déclencheur de réexamen** (voir « Plus d'informations »).
- **C. Découper un gros texte en plusieurs documents** (segments par plages de lignes). Rejeté : deux
  redécoupages concurrents dupliquent le texte, et l'annulation, les curseurs et la recherche
  deviennent inter-documents.
- **D. Paliers par taille, Automerge dans un Worker, très gros textes hors d'Automerge, réseau
  fragmenté par priorité.** Retenue.

## Décision

Option D, en dix points.

### D1. Paliers, comptés en caractères du texte (`doc.length`)

| Palier | Taille | Stockage | Édition | Ce qui change |
|---|---|---|---|---|
| **Fluide** | ≤ 256 Ki | document Automerge | à plusieurs | rien |
| **Grand** | 256 Ki à 2 Mi | document Automerge | à plusieurs, avec mesures (D2, D7) | bandeau discret « Fichier volumineux : les modifications des autres peuvent arriver avec un léger retard » |
| **Lecture seule en mémoire** | 2 Mi à 64 Mi | **blob** BLAKE3 (ADR 0023) ; texte en simple chaîne dans la webview | aucune dans Inkwell ; une modification hors d'Inkwell donne une nouvelle version (comme une image, PRJ-13) | message de l'ADR 0026 ; vue texte et tableau en lecture seule |
| **Lecture seule paginée** | > 64 Mi | blob si ≤ quota de 100 Mo, sinon local seulement (« trop grand pour être partagé ») | aucune | pages servies par Go (D4) |

- **Aucun nouveau type de nœud.** Un très gros texte est un nœud `blob` dont l'extension est
  textuelle. Le type dit comment c'est stocké, l'extension comment c'est montré.
- **Seuil vérifié avant Automerge, jamais après.** À l'ouverture d'un fichier du disque, à l'import,
  au dépôt d'un fichier et au collage, la taille est connue avant de créer ou de modifier un document.
  `store/limits.ts` (`tooBigToEdit`) devient la porte d'entrée unique, avec ces seuils.
- **Croissance d'un document existant** : de 2 à 4 Mi, il reste modifiable, et le bandeau propose
  « Figer en lecture seule » ou une « nouvelle base » (D10). Au-delà de 4 Mi, un ajout local est refusé
  dans un `transactionFilter` CodeMirror : « Ce collage porterait le fichier à X Mo, au-delà de ce qui
  se modifie à plusieurs (2 Mo). » Une croissance venue d'un pair est appliquée (refuser casserait la
  convergence), puis le document passe en lecture seule chez soi avec le même message (REL-18).
- **Avant D2** (Automerge encore dans le fil d'affichage), le palier Grand s'arrête à **512 Ki** :
  100 ms de gel par lot reçu, au rythme de D7, est le pire que l'on accepte.

### D2. Le `Repo` dans un Web Worker dédié

- automerge-repo, les adaptateurs `WailsNetworkAdapter` et `GoStorageAdapter`, la couverture
  (ADR 0018) et l'historique local vivent dans un **Worker** de la webview. Le CRDT reste dans la
  webview : l'invariant tient.
- Le fil d'affichage ne garde que CodeMirror et un pont mince :
  - il envoie les modifications locales (`{from, to, insert}`, avec un numéro de version) ;
  - il reçoit des correctifs déjà traduits en changements CodeMirror ;
  - il réconcilie les deux comme `automerge-codemirror` le fait aujourd'hui.
- Les curseurs Automerge et la présence passent par le même pont. Les Streams Wails restent dans le
  fil principal, qui transfère les `ArrayBuffer` sans copie (`postMessage` avec transfert).
- La CSP ajoute `worker-src 'self'` ; `'wasm-unsafe-eval'` couvre déjà le Worker.
- **Gain :** une application de 0,4 s devient de la latence, plus un gel. **Bonus :** la mémoire WASM
  devient récupérable : on termine le Worker et on rouvre les documents visibles (D9).
- C'est la direction que décrit l'équipe Automerge
  ([automerge-repo#339](https://github.com/automerge/automerge-repo/issues/339)) : CRDT hors du fil
  d'affichage, vue matérialisée dans le fil d'affichage.

### D3. Gros ajouts découpés

Un ajout de plus de 64 Ki caractères (collage, import, intégration d'une modification externe) est
appliqué en **modifications successives de 64 Ki au plus** : un seul pas d'annulation (groupe dans
`localHistory`) et une seule attestation à la fin (ADR 0018). Le pic tombe ainsi d'environ 300 à
7 octets par caractère (constat 4).

### D4. Très gros textes : vue paginée servie par Go

Un nouveau paquet **`internal/textview`**, créé à la tâche PERF-10 :
- un index creux des lignes (repère toutes les 1 024 lignes, construit en flux et en arrière-plan) ;
- des pages de 64 Kio, découpées aux fins de ligne et à des frontières UTF-8 sûres ;
- la recherche littérale ou par expression régulière (`regexp`, linéaire), avec résultats en flux et
  annulable ;
- la détection d'encodage, réutilisée de A3.

Il passe par un Stream Wails `view` séparé, comme `preview` : une recherche longue ne bloque jamais le
Stream `store`, qui est séquentiel. Les lectures se font sous `os.Root`.

Dans la webview, CodeMirror en lecture seule tient une **fenêtre** de 2 Mio au plus autour de la zone
visible. La barre de défilement est proportionnelle au nombre de lignes de l'index, et « Aller à la
ligne » charge la page voulue (comme `less`, klogg ou un éditeur hexadécimal). La première page
s'affiche avant la fin de l'index.

Le tableau (ADR 0026) en lecture seule paginée lit ses lignes dans les mêmes pages ; trier et filtrer
plus de 64 Mi est hors périmètre et le dit.

### D5. Fil de synchro : fragments, priorités, échéance au progrès

Le multiplexeur (`internal/transport/mux`), qui reste opaque, change sur trois points :

- **Fragmentation.** Toute charge de plus de 64 Kio part en fragments de **32 Kio** (mesure du
  2026-10-26 : 256 Kio coûtent 2 s d'attente à 1 Mbit/s, contre un budget de 500 ms ; 32 Kio, 0,26 s). Le réassemblage est
  borné par `MaxFrame` et par le budget d'octets en file existant, et il est fuzzé.
- **Deux classes.**
  - *Interactive* : présence, attestations, messages ≤ 64 Kio.
  - *Fond* : les fragments.

  L'ordonnanceur sert l'interactive en priorité et garde au fond au moins 25 % du débit, pour ne pas
  l'affamer.
- **Ordre FIFO par voie.** La webview étiquette chaque enveloppe d'une clé de voie (hachage du
  document sur 4 octets ; Go ne la comprend pas). L'ordre est garanti dans une voie, jamais entre
  voies. Ainsi l'attestation précède toujours les modifications de son document.

L'échéance d'écriture n'est plus `10 s + taille/1 Mio/s` mais **« aucun progrès pendant 20 s »** :
elle est réarmée à chaque fragment écrit. Le flux bulk applique la même règle.

Les fragments portent la longueur totale. Go en tire des événements de progression (« Réception de
main.typ : 2,1 sur 6,3 Mo ») sans rien décoder.

### D6. Compression

- **Aucune pour la synchro** (constat 6).
- **zstd par tranche pour les blobs compressibles** : extension textuelle, ou essai sur les premiers
  64 Kio avec un rapport < 0,9. La bibliothèque est `github.com/klauspost/compress/zstd`, déjà dans le
  graphe et promue en dépendance directe (ADR 0008).
- Bao vérifie les octets **décompressés**. Le décodeur est borné (fenêtre ≤ 8 Mio, sortie ≤ longueur de
  la tranche) et fuzzé (`FuzzBlobWire` étendu).
- Gain attendu sur du texte : environ 3,4× (zstd -3 : 100 Mo → 31 Mo).

### D7. Rythme de synchro adaptatif

automerge-repo regroupe déjà les modifications d'un document toutes les 100 ms (`syncDebounceRate`
du `DocSynchronizer`). Inkwell le règle **par document** :
- Fluide : 100 ms ;
- Grand : `100 ms + 0,25 ms × Ki caractères` (2 Mi → 600 ms) ;
- la présence est espacée d'au moins 250 ms en Grand.

Le champ n'est pas une option du `Repo` : on proposera l'option en amont et, en attendant, on la posera
par un correctif `pnpm patch` épinglé (ADR 0008). La projection sur disque (300 ms) s'espace selon la
même règle, car réécrire 2 Mo toutes les 300 ms n'apporte rien.

### D8. Relais

- Le lien direct est toujours préféré (déjà fait). Le relais n'a pas de plafond de durée ni de volume
  (`relay.md`).
- **Tant qu'un lien est relayé**, un blob de plus de 25 Mo attend un lien direct : « En attente d'un
  lien direct — Télécharger quand même ». Le relais est la machine d'un membre du groupe ; on ne la
  sature pas sans le dire.
- Un pair relayé dont la file d'envoi dépasse 2 s de retard est signalé « Lien lent » dans le panneau
  d'état (REL-01).

### D9. Démarrage et mémoire

- **Démarrage paresseux.** Seuls l'index et le document ouvert se chargent. Les autres documents se
  chargent à l'ouverture ou pour une modification (remplacer partout).
- **Recherche, tâches et sommaire du projet** (B6, B10) : pour les documents non chargés, on lit les
  **projections** sur le disque, côté Go (Stream `view`). Seul le remplacement ouvre le document.
- **Déchargement.** Un document invisible depuis 60 s est déchargé (`DocHandle.unload`). Quand le WASM
  du Worker dépasse son budget, ou qu'un document Grand a été fermé, le Worker est **recyclé** :
  1. persister ;
  2. terminer ;
  3. relancer ;
  4. rouvrir les documents visibles.

### D10. Compaction de l'historique

- **Stockage** : automerge-repo compacte déjà (`A.save` quand les incréments dépassent l'instantané).
  Il le fait désormais dans le Worker : 0,35 s à 10 Mio ne fige plus rien.
- **CRDT** : Automerge 3 ne sait pas tronquer un historique. La compaction prend donc la forme d'une
  **nouvelle base**, action explicite d'un écrivain, proposée quand l'ouverture dépasse 1 s ou que
  l'historique pèse plus de 10 fois le texte :
  1. un nouveau document est créé depuis le texte courant ;
  2. l'index fait pointer le nœud vers lui, avec `rebasedFrom: {url, heads}` ;
  3. l'ancien document reste lisible dans l'historique (B2) ;
  4. une modification tardive de l'ancien document (un pair resté hors ligne) est fusionnée en trois
     voies, base = texte aux `heads` de la bascule, par la machinerie de B1 (`ExternalSync`). Les
     conflits partent en suggestion.

  Le même mécanisme sert à « Figer en lecture seule » (texte → blob) et à « Rendre modifiable à
  plusieurs » (blob ≤ 2 Mi → texte).

### Budgets (vérifiés par les cas PERF-*, `INKWELL_PERF=1` pour échouer, journalisés sinon)

| Mesure | Budget |
|---|---|
| Frappe locale → affichage, tout palier modifiable | p95 ≤ 16 ms |
| Image du fil d'affichage pendant qu'un pair tape dans un document Grand (après D2) | p95 ≤ 16 ms, aucune tâche > 50 ms |
| Frappe → affichage chez le pair, LAN | p95 ≤ 100 ms (≤ 128 Ki) · ≤ 250 ms (≤ 512 Ki) · ≤ 1 s (≤ 2 Mi) |
| Frappe → affichage chez le pair, **relais** (RTT 100 ms, 1 % de pertes) | p95 ≤ 300 ms (≤ 128 Ki) · ≤ 1,2 s (≤ 2 Mi) |
| Frappe pendant un message de 6 Mo sur un lien à 1 Mbit/s | p95 ≤ 500 ms ; le message aboutit |
| Arrivée d'une personne : projet de 2 Mi de texte + 20 Mo de blobs, relais à 10 Mbit/s | progression visible ≤ 1 s ; `main` modifiable ≤ 3 s ; tout ≤ 30 s |
| Ouverture d'un document local | ≤ 150 ms (≤ 256 Ki) ; ≤ 0,6 s par Mi au-delà |
| Ouverture d'un très gros fichier (toute taille) | première page ≤ 300 ms ; index de 100 Mo ≤ 1 s ; aller à une ligne ≤ 50 ms ; recherche sur 100 Mo ≤ 500 ms (premiers résultats ≤ 100 ms) |
| Mémoire de la webview (processus WebKit), projet dont les documents ouverts font ≤ 2 Mi | ≤ 600 Mo ; alerte à 1,5 Go |
| Fichier en lecture seule en mémoire | ≤ 4 octets par caractère + 50 Mo |
| Fichier en lecture seule paginé | + 64 Mo au plus, quelle que soit sa taille |
| WASM : aucun chemin ne dépasse | 1 Go par Worker (alerte et recyclage) |
| Démarrage : fenêtre peinte / dernier projet modifiable | ≤ 1 s / ≤ 1,5 s |
| Go : trames et mux, message ≤ 64 Kio | 0 allocation (bancs existants) ; fragmentation ≤ 5 % de CPU en plus (`benchstat`) |

Ces budgets remplacent, en les précisant, ceux de `engineering-principles.md` § 3. Le budget « document
de 50 000 opérations ouvert < 2 s » y reste valable.

### Conséquences

- Bien :
  - le fil d'affichage ne dépend plus de la taille des documents des autres ;
  - la mémoire a un plafond pour chaque chemin ;
  - un fichier de 1 Go s'affiche ;
  - un lien lent finit toujours ses envois et ne retient plus les frappes ;
  - SHEET-BIG passe sous 1 Go.
- Moins bien :
  - D2 est un gros chantier : pont CodeMirror ↔ Worker, curseurs, annulation, couverture déplacés ;
  - un fichier de 2 à 64 Mi ne se modifie plus à plusieurs (avant : jusqu'à 10 Mo) ;
  - la « nouvelle base » coupe la filiation Automerge (gardée par `rebasedFrom`) ;
  - un correctif local d'automerge-repo est à entretenir tant que l'option n'existe pas en amont.

### Confirmation

Cas PERF-01 à PERF-20 du plan T11 (« Fiabilité et performance »). Banc de référence de la courbe
Automerge (PERF-18), rejoué à chaque mise à jour d'`@automerge/*`. `scripts/arch-check.sh` : seul
`internal/textview` lit les gros fichiers pour la vue, et il n'importe ni `transport` ni `blob`.

## Plus d'informations

Amende, si elle est acceptée :
- l'ADR 0026 : seuil de 10 Mo → paliers D1 ; lecture seule hors d'Automerge ;
- l'ADR 0023 : un très gros texte est un blob ; zstd sur le flux bulk ;
- l'ADR 0020 : Stream `view`.

**Réexamen** : si une version d'Automerge ne rend pas l'application distante sous-linéaire avant
l'étape 5, faire un essai borné de Loro sur les mêmes mesures (PERF-18), avec le coût de migration de
la couverture (ADR 0018). Signaler le constat 1 en amont, avec le script de mesure.

Sources :
- [Automerge 3.0](https://automerge.org/blog/automerge-3/) ;
  [Merry Commitmas](https://automerge.org/blog/merry-commitmas/) (instantané pour un pair vide, gels
  au calcul des messages) ;
  [automerge-repo#339](https://github.com/automerge/automerge-repo/issues/339) (messages qui bloquent
  le fil principal) ; [#425](https://github.com/automerge/automerge-repo/issues/425) (mémoire des
  grands textes) ;
  [Sedimentree](https://github.com/automerge/beelay/blob/main/docs/sedimentree.md) (synchro par strates).
- [CodeMirror, un document de millions de lignes](https://codemirror.net/examples/million/) ;
  [VS Code, réécriture du tampon de texte](https://code.visualstudio.com/blogs/2018/03/23/text-buffer-reimplementation)
  (table de pièces, lecture par morceaux de 64 Kio, désactivation de la coloration au-delà d'un seuil) ;
  [Zed, rope et SumTree](https://zed.dev/blog/zed-decoded-rope-sumtree) ;
  [Zed, CRDT](https://zed.dev/blog/crdts) (travail hors du fil d'interface).
- [Mesure du perçage de NAT DCUtR](https://arxiv.org/html/2510.27500v1) ;
  [probelab](https://probelab.io/blog/can-libp2p-punch-through-nats/).
- [Loro 1.0](https://loro.dev/blog/v1.0) ;
  [Figma multiplayer](https://www.figma.com/blog/how-figmas-multiplayer-technology-works/) (copie
  complète à l'arrivée, puis deltas).
