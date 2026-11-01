# Protocole inkwell — spécification normative

| | |
|---|---|
| Version | **2.0.0-brouillon** (SemVer, §1.4) |
| Date | 2026-11-01 |
| Statut | proposé ([ADR 0034](../adr/0034-protocole-unifie.md)) ; les parties marquées *existant* décrivent les octets déjà émis par inkwell et sont figées |
| Aperçu | [README.md](README.md) |
| Vecteurs | [vectors/](vectors/) (§21) |

## 1. Conventions

### 1.1 Mots normatifs

Les mots-clés **DOIT**, **NE DOIT PAS**, **OBLIGATOIRE**, **DEVRAIT**, **NE DEVRAIT PAS**,
**RECOMMANDÉ**, **PEUT** et **OPTIONNEL** ont le sens de MUST, MUST NOT, REQUIRED, SHOULD,
SHOULD NOT, RECOMMENDED, MAY et OPTIONAL du BCP 14 ([RFC 2119], [RFC 8174]) quand, et
seulement quand, ils sont écrits en gras comme ici.

### 1.2 Notation des octets

Les entiers sont non signés et **gros-boutistes** (`u8`, `u16`, `u32`, `u64`), sauf mention
contraire. `[n]` désigne n octets bruts ; `str8` une chaîne UTF-8 précédée de sa longueur en
`u8` ; `str16` la même avec une longueur `u16`. `a || b` est la concaténation. Une structure
se lit de haut en bas, sans remplissage. Les messages CBOR sont décrits en CDDL ([RFC 8610])
dans [cddl/](cddl/).

### 1.3 Décodage strict

Sauf mention contraire, un décodeur **DOIT** refuser une entrée tronquée, une entrée suivie
d'octets en trop, une longueur annoncée au-delà de la borne du champ (avant d'allouer quoi
que ce soit), une valeur hors de son domaine, une liste censée être strictement croissante qui
ne l'est pas, et un texte qui n'est pas de l'UTF-8 valide. Pour tout objet à mise en page
binaire de ce document, l'encodage est **unique** : `encode(decode(b)) = b` pour toute entrée
acceptée. Ce choix suit [RFC 9413] : la tolérance aux entrées inattendues n'est admise qu'aux
points d'extension prévus (§5.4, §9.2), jamais ailleurs.

### 1.4 Versions et compatibilité

- La spécification suit SemVer. **Majeure** : changement incompatible du fil ; elle se voit
  dans l'identifiant de protocole libp2p (`/inkwell/sync/2.0.0`). **Mineure** : nouvelle
  capacité (§5.4), nouveau TLV ou nouveau type de trame réservé à une capacité ; un pair qui
  ne la connaît pas n'est pas touché. **Correctif** : texte seulement.
- Deux pairs négocient la majeure par libp2p (multistream-select : chaque côté propose les
  identifiants qu'il connaît) et les mineures par HELLO (§5.3). C'est le même partage que
  HTTP/2 (ALPN [RFC 7301] puis SETTINGS [RFC 9113 §6.5]).
- Les **objets signés** (§4) sont figés pour toujours sous leur octet de version : un projet
  existant en dépend (son identifiant est l'empreinte de sa genèse). Une implémentation
  **DOIT** lire toutes les versions d'objet signé qu'elle a déjà acceptées ; elle **NE DOIT
  PAS** émettre ni accepter d'un pair une version nouvelle tant que la capacité
  correspondante n'est pas négociée avec ce pair (§5.4).
- Une capacité ne se retire qu'à une majeure.

### 1.5 Périmètre

| Partie | Statut | Sections |
|---|---|---|
| Identités, cryptographie, objets signés | public | §3, §4 |
| Transport libp2p, `sync/2.0.0`, `bulk/1.0.0`, `lobby/1.0.0`, ticket | public | §2, §5–§8 |
| Messages automerge-repo et règles d'admission | public | §9 |
| Contrôle de flux, machines à états, erreurs, bornes | public (le comportement observable) | §10–§16 |
| Pont Go ↔ webview (Streams Wails `net`, `store`, `preview`, `present`) | **interne**, hors contrat | Annexe A |
| Organisation du code (acteurs, paquets) | informatif | §15 |

## 2. Architecture en couches

```
 L4  dépôt        messages automerge-repo (CBOR) · admission · couverture par attestations
 L3  classes      TRUST (anti-entropie) · ATTEST (dernier gagne) · REPO (fiable, générations)
                  PRESENCE (à perte) · objets offerts et blobs (tirés par le receveur)
 L2  session      /inkwell/sync/2.0.0 : trames, HELLO, capacités, GOAWAY, REPO_RESET
                  /inkwell/bulk/1.0.0 : WANT/SLICE/MISSING/BUSY (Bao, BLAKE3)
                  /inkwell/lobby/1.0.0 : une requête, une réponse (inconnus invités)
 L1  transport    libp2p : QUIC v1 (TLS 1.3) ou TCP + Noise + yamux ; relais circuit v2 ;
                  DCUtR ; mDNS « _inkwell._udp » ; contrôle de flux par flux et par connexion
 L0  identité     clé Ed25519 par appareil · PeerID · signatures à domaine séparé
```

Chaque couche ne fait confiance qu'à ce que la couche du dessous a vérifié, dans l'ordre du
§12.

## 3. Identités et cryptographie

### 3.1 Clés et PeerID

- Chaque appareil possède une paire Ed25519 ([RFC 8032]). La clé publique (32 octets) est
  l'identité de l'appareil dans tous les objets signés.
- Le PeerID est l'identifiant libp2p de cette clé : base58btc du multihash *identité*
  (`0x00`, longueur `0x24`) de la clé publique encodée en protobuf libp2p
  (`0x08 0x01 0x12 0x20 || clé`). Il fait 52 caractères et commence par `12D3KooW`
  (vecteur `crypto.json#peer-id-A`).
- Un PeerID **DOIT** être sous forme canonique (le ré-encodage de la clé extraite redonne la
  même chaîne), de 1 à 64 octets ASCII imprimables sans espace.

### 3.2 Validité d'une clé publique

Une clé publique Ed25519 est **valide** si et seulement si :

1. son encodage est canonique : le point se décode, et le ré-encodage du point redonne les
   mêmes 32 octets ;
2. elle n'est pas d'ordre petit : `[8]A ≠ O`. Les huit encodages canoniques à refuser sont
   listés dans `crypto.json#small-order-keys`.

Toute clé qui entre dans le protocole **DOIT** être validée ainsi, au plus tôt : filtre de
connexion (clé du PeerID distant), décodage d'un PeerID, `Author` et `Subject` d'une op de
membre, `Signer` d'une attestation, `Author` d'une décision, hôte d'un ticket. Une clé
invalide est refusée comme une entrée mal formée.

*Raison.* Avec la clé neutre et la signature `R = O, S = 0`, l'équation de vérification sans
cofacteur est vraie pour **tout** message (vecteur `crypto.json#ed25519-small-order-key`).
`crypto/ed25519` de Go et go-libp2p v0.51.0 l'acceptent aujourd'hui : quiconque peut alors
s'authentifier sous ce PeerID (`crypto.json#peer-id-small-order`) et signer en son nom
(voir l'inventaire, constat N-1, et [Chalkias 2020]).

### 3.3 Signatures

- Toute signature de ce protocole est Ed25519 pur sur l'octet-chaîne
  `"inkwell/" || domaine || "/v1" || 0x00 || message`, où `domaine` est l'une des valeurs du
  registre §20.4. Une signature faite pour un domaine ne vérifie donc jamais pour un autre
  (vecteur `crypto.json#sig-domain-1`).
- La vérification **DOIT** suivre [RFC 8032 §5.1.7] avec l'équation **sans cofacteur**
  `[S]B = R + [k]A`, refuser `S ≥ L` (`crypto.json#ed25519-noncanonical-s`), comparer `R` octet
  par octet au point recalculé, et n'être appliquée qu'à une clé valide (§3.2). Ces règles
  sont celles de `crypto/ed25519` de Go plus le §3.2 ; une autre implémentation **DOIT** donner
  le même verdict sur tous les vecteurs, car un désaccord sur une signature est un désaccord
  sur la liste des membres.

### 3.4 Empreintes

| Usage | Fonction | Entrée |
|---|---|---|
| op de membre, décision, identifiant de projet | SHA-256 | encodage complet, signature comprise |
| têtes de document | empreinte de changement Automerge (32 octets) | définie par le format Automerge |
| blob, objet offert | BLAKE3-256, arbre Bao à groupes de 16 Kio | contenu |

Le `ProjectID` est le SHA-256 de l'encodage de l'op de genèse (§4.1). Il ne change jamais.

## 4. Objets signés (*existant*, figés)

### 4.1 Op de membre, version 1

```
op_v1 {
  version   u8      = 1
  kind      u8      1 Genesis | 2 Add | 3 SetRole | 4 Remove | 5 Suspend | 6 Resume
  role      u8      0 aucun | 1 lecture | 2 écriture | 3 admin
  lamport   u64
  author    [32]    clé valide (§3.2)
  subject   [32]    clé valide
  person    [32]    personne dont subject est un autre appareil ; zéro sinon
  name      str16   UTF-8, ≤ 128 octets
  root_url  str16   UTF-8, ≤ 512 octets
  nprev     u8      ≤ 64
  prev      [32] × nprev, strictement croissants (ordre des octets)
  sig       [64]
}                    taille totale ≤ 4096 octets
```

- Message signé, domaine `trust-op` : `ProjectID || op sans sig`. La genèse est signée avec un
  `ProjectID` de 32 zéros (vecteur `trust.json#op-genesis` ; `op-add-writer` donne les
  octets signés exacts).
- Contraintes par `kind` (refus sinon) : **Genesis** : `author = subject`, `role = 3`,
  `person = 0`, `nprev = 0`, `lamport = 0`. **Add** : rôle 1–3. **SetRole** : rôle 1–3,
  `person = 0`. **Remove** : `role = 0`, `person = 0`, `name` vide. **Suspend**, **Resume** :
  `author ≠ subject`, `role = 0`, `person = 0`, `name` vide. Toute op autre que la genèse :
  `root_url` vide, `nprev ≥ 1`, `lamport ≥ 1`.
- Le sens des ops (ancienneté, dernier admin, suspension, plafond de 1024 ops) est celui de
  [ADR 0005], [ADR 0017] et [ADR 0021] ; la liste est un pli déterministe de l'ensemble des
  ops, indépendant de l'ordre d'arrivée.

### 4.2 Op de membre, version 2 (capacité `member-op/2`, inerte)

Identique à la version 1 avec `version = 2` et, après `prev` :
`nseen u8 ≤ 64 || seen [32] × nseen || nseen_listing u8 ≤ 64 || seen_listing [32] × …`,
chaque liste strictement croissante, au moins une non vide. **Refusée** tant que
`member-op/2` n'est pas négociée (`trust.json#op-version-2-not-negotiated`).

### 4.3 Attestation, version 1

```
attestation_v1 {
  version   u8      = 1
  project   [32]    ProjectID
  signer    [32]    clé valide
  seq       u64     croissant par (doc, signer) ; le récepteur garde le plus grand
  doc       str16   identifiant de document, UTF-8, 1 à 256 octets
  nheads    u16     ≤ 64
  heads     [32] × nheads, strictement croissantes
  sig       [64]    domaine « attest », message = tout ce qui précède sig
}                    taille totale ≤ 4096 octets
```

Une attestation est **valable** si `project` est celui de la session, la signature vérifie et
`signer` est, dans la liste courante, un membre de rôle écriture ou admin non suspendu
(`attest.json`). Elle n'autorise rien d'autre que la couverture du §9.4.

### 4.4 Décision d'admin, version 1 (capacité `policy/1`, inerte)

```
policy_op_v1 {
  version   u8      = 1
  lamport   u64     ≥ 1
  author    [32]    clé valide d'un admin (ADR 0030 D1)
  nprev     u8      ≤ 64 ;  prev    [32] × nprev, strictement croissants
  nmembers  u8      1–64 ;  members [32] × nmembers, strictement croissants (frontière de la liste)
  nentries  u8      1–96 ;  entries (key str16 ≤ 160, value str16 ≤ 4096) × nentries,
                            clés strictement croissantes, espaces de noms du registre §20.5
  sig       [64]    domaine « policy-op », message = ProjectID || tout ce qui précède sig
}                    taille totale ≤ 8192 octets
```

Bornes du journal : 8192 décisions au total, 1024 par auteur. Sémantique : [ADR 0030].
Vecteur : `policy.json#policy-mode-approval`.

## 5. Session : `/inkwell/sync/2.0.0`

### 5.1 Flux

- Chaque pair ouvre **un flux unidirectionnel sortant** `/inkwell/sync/2.0.0` vers chaque
  membre connecté ; il ne lit jamais ce flux et n'écrit jamais sur un flux entrant. Un octet
  reçu sur son propre flux sortant est une violation.
- Un flux peut être remplacé (passage d'un relais à une connexion directe) : l'émetteur ferme
  l'ancien en écriture après sa dernière trame ; le récepteur **DOIT** lire l'ancien jusqu'à
  sa fin avant de lire le nouveau (au plus 60 s d'attente), ce qui conserve l'ordre.
- Un flux remplacé après un **échec d'écriture** a pu perdre des trames : l'émetteur **DOIT**
  alors engager un `REPO_RESET` (§7.3).
- Un pair qui n'est pas membre mais que la fenêtre d'invitation laisse se connecter ne peut
  ouvrir que `/inkwell/lobby/1.0.0` ; tout autre flux de sa part est réinitialisé.

### 5.2 Trames

```
trame {
  length    u32     1 ≤ length ≤ 98 304 (96 Kio) ; compte type + corps
  type      u8      registre §20.1
  corps     [length − 1]
}
```

Le récepteur **DOIT** comparer `length` à la borne avant d'allouer
(`session.json#frame-too-large`). Un type inconnu, ou réservé à une capacité non négociée, est
une violation de protocole (`session.json#unknown-type`) : il n'y a pas d'extension silencieuse
au niveau des trames.

| Type | Nom | Corps | Taille du corps | Capacité |
|---|---|---|---|---|
| 0x01 | HELLO | liste TLV (§5.3) | ≤ 1024 | toujours |
| 0x02 | GOAWAY | `code u8` (§20.3) | 1 | toujours |
| 0x03 | TRUST | message de synchronisation de la liste (§6.1) | ≤ 81 920 | core/2 |
| 0x04 | ATTEST | `attestation_v1` (§4.3) | ≤ 4096 | core/2 |
| 0x05 | REPO | message automerge-repo, CBOR (§9) | 1 à 65 536 | core/2 |
| 0x06 | PRESENCE | message automerge-repo `ephemeral`, CBOR (§9.5) | 1 à 2048 | core/2 |
| 0x07 | OFFER | `hash [32] \|\| size u32` (§7.2) | 36 | core/2 |
| 0x08 | REPO_RESET | `generation u32 \|\| reason u8` (§7.3, §20.6) | 5 | core/2 |
| 0x09–0x0B | réservés | POLICY, HISTORY, CHANGESIG | — | policy/1, history/1, changesig/1 |

### 5.3 HELLO

- Chaque flux sync commence par une trame HELLO, et par une seule. Avant d'avoir lu le HELLO
  du pair, un récepteur ne lit aucune autre trame de ce flux.
- Corps : une suite d'éléments `id u16 || len u16 || valeur [len]`, identifiants
  **strictement croissants** (`session.json#hello-unsorted-tlv` est refusé). Un identifiant
  inconnu est ignoré (`session.json#hello-unknown-tlv`) : c'est le point d'extension du HELLO.

| id | Nom | Valeur | Règle |
|---|---|---|---|
| 0x0001 | PROJECT | `[32]` ProjectID | OBLIGATOIRE ; différent du nôtre → GOAWAY PROJECT_MISMATCH |
| 0x0002 | CAPS | `u16` × k, strictement croissants, k ≥ 1 | OBLIGATOIRE ; registre §20.2 |
| 0x?A?A | GREASE | quelconque | ignoré (§5.4) |

- Le HELLO du pair **DOIT** arriver dans les 10 s suivant l'ouverture de son flux ; sinon
  GOAWAY HELLO_TIMEOUT. Un HELLO ultérieur sur un flux de remplacement **DOIT** porter le même
  projet et les mêmes capacités, sinon violation.

### 5.4 Capacités

- L'**ensemble effectif** d'une session est l'intersection des CAPS des deux HELLO.
  `core/2` **DOIT** y figurer, sinon GOAWAY NO_COMMON_CAPABILITY.
- Un émetteur **NE DOIT PAS** envoyer une trame, une version d'objet ou une entrée que
  l'ensemble effectif n'autorise pas. Un récepteur **DOIT** traiter une telle trame comme une
  violation. Les décodeurs reçoivent l'ensemble effectif en paramètre : c'est le seul endroit
  où une version nouvelle devient acceptable.
- Un objet n'est **relayé** qu'aux pairs dont l'ensemble effectif contient sa capacité.
- Un émetteur **DEVRAIT** ajouter à CAPS, et en TLV, une valeur GREASE tirée au hasard dans
  `{0x0A0A, 0x1A1A, …, 0xFAFA}` ([RFC 8701]) ; un récepteur **DOIT** l'ignorer. Cela garde le
  point d'extension en état de marche.

### 5.5 Classes et fiabilité

| Classe | Trames | Sémantique | Perte |
|---|---|---|---|
| session | HELLO, GOAWAY, REPO_RESET | contrôle | interdite (la connexion tombe sinon) |
| état | TRUST, ATTEST | anti-entropie, dernier gagne | permise : réparée par l'annonce suivante (15 s) ou le renvoi à la connexion |
| dépôt | REPO, OFFER | fiable, ordonnée, par génération | **interdite** : toute perte DOIT déclencher REPO_RESET |
| présence | PRESENCE | dernier gagne par `sessionId` | permise |

### 5.6 Ordre

1. Les trames d'un flux sont traitées dans l'ordre d'émission.
2. Une ATTEST est entièrement traitée (vérifiée, enregistrée) avant la trame qui la suit :
   une attestation précède toujours les messages qu'elle couvre.
3. Les REPO en ligne sont livrés dans l'ordre ; les objets offerts aussi, entre eux. Un REPO
   reçu **après** un OFFER **PEUT** être livré avant l'objet offert (un gros transfert ne
   retarde pas la frappe ; même propriété qu'aujourd'hui, `mux.go:28-31`). Un récepteur
   **DOIT** le tolérer, ce qu'Automerge fait.

## 6. Classes d'état

### 6.1 TRUST : réplication de la liste des membres (*existant*)

```
have { kind u8 = 1 ; n u16 ≤ 256 ; hashes [32] × n }          frontière de l'émetteur
ops  { kind u8 = 2 ; n u16 ; n × ( len u16 ; op [len] ) }     ops (§4.1), ≤ 4096 o chacune
```

Message ≤ 81 920 octets, décodage strict (`trust.json#sync-have`, `#sync-ops`,
`#sync-have-too-many`).

- À l'activation d'une session, puis toutes les 15 s, chaque pair envoie `have` avec sa
  frontière (les 256 premières têtes). Une liste scellée (1024 ops) n'annonce plus.
- Un `have` reçu est répondu par `ops` (ce que l'émetteur n'a pas), jamais par un `have`, et au
  plus **2 fois par pair et par période de 15 s** : aucun écho, aucune amplification.
- Une op nouvellement appliquée est relayée à tous les autres pairs actifs.
- Une op mal formée (§4.1) est une violation de protocole. Une op bien formée mais refusée
  (auteur inconnu car arrivée dans le désordre, liste pleine, scellée) est ignorée sans
  sanction : un pair honnête peut en envoyer. Les ops en attente de leurs parents sont
  gardées au plus deux périodes (256 au plus).

### 6.2 ATTEST : attestations (*existant*)

- Un pair qui écrit **DOIT** envoyer, avant tout message REPO ou OFFER portant de nouvelles
  têtes d'un document, une attestation de ces têtes (§4.3) à ce même pair, sur le même flux.
- Un récepteur garde, par (document, signataire), l'attestation de plus grand `seq` ; une
  attestation reçue nouvelle et valable est relayée aux autres pairs actifs ; une égale ou
  plus ancienne n'est ni gardée ni relayée (fin du commérage).
- À l'activation d'une session, chaque pair envoie la dernière attestation de chaque
  (document, signataire) qu'il garde.
- Une révocation ou une suspension rend caduques les attestations du signataire ; le
  récepteur les oublie et la couche dépôt cesse de les compter (§9.4).
- Une attestation reçue d'un pair connecté mais pas encore actif **PEUT** être gardée
  (≤ 40 000 par pair) puis vérifiée à son activation.

### 6.3 PRESENCE

Trame PRESENCE = un message `ephemeral` automerge-repo (§9.5). Le Go ne l'interprète pas.
Côté émetteur et côté récepteur, une file de présence garde **la dernière** trame par
(pair, `sessionId`) et en remplace une plus ancienne ; quand la file est pleine (16 entrées
par pair), la plus ancienne est jetée. Une PRESENCE ne bloque jamais la lecture d'un flux.

## 7. Classe dépôt

### 7.1 REPO

Un message automerge-repo de 1 à 65 536 octets voyage en ligne dans une trame REPO. Un
message plus grand voyage par OFFER (§7.2). Au-delà de 67 108 797 octets
(2^26 − 67, borne du pont) il n'existe pas.

### 7.2 OFFER et tirage par le receveur

```
offer { hash [32] (BLAKE3-256 du message) ; size u32, 65 537 ≤ size ≤ 67 108 797 }
```

1. L'émetteur garde le message (« objet offert ») et envoie OFFER sur le flux sync.
2. Le receveur le tire sur `/inkwell/bulk/1.0.0` par des WANT (§8.1) de groupes alignés de
   16 Kio, d'au plus 1 Mio chacun, avec au plus **2 WANT en vol par pair** et **8 au total**.
   Il vérifie chaque SLICE contre `hash` ; l'objet complet **DOIT** faire exactement `size`
   octets.
3. L'émetteur ne sert un objet offert qu'au pair à qui il l'a offert ; à tout autre il
   répond MISSING.
4. L'émetteur oublie l'objet quand le receveur en a tiré le dernier groupe, ou après **120 s
   sans WANT** ; dans ce second cas le message est perdu et l'émetteur **DOIT** engager
   REPO_RESET (raison OFFER_EXPIRED).
5. Le receveur abandonne après 3 échecs sur un même groupe (délai de 30 s, SLICE invalide,
   MISSING) et **DOIT** engager REPO_RESET (raison PULL_FAILED).
6. Un pair a **au plus un objet offert non tiré** vers un pair donné : il n'émet pas de
   deuxième OFFER avant que le premier soit tiré ou abandonné. Un deuxième OFFER en attente
   est une violation.

*Pourquoi tirer plutôt que pousser.* Le receveur ne reçoit que ce qu'il a demandé, quand il
a la place : un pair lent ou hostile n'immobilise que ses propres demandes, jamais la mémoire
ou le tour des autres ; l'intégrité de chaque tranche est vérifiée sans confiance
(arbre Bao). C'est le modèle des blobs d'inkwell (ADR 0023) et d'iroh-blobs ; la
fragmentation dans le flux, le réassemblage, le débordement sur disque et l'éviction
d'aujourd'hui disparaissent.

### 7.3 REPO_RESET : générations de synchronisation

La synchronisation automerge-repo avec un pair vit dans une **génération** `g` (u32,
commence à 1 à l'activation). Les deux côtés tiennent, pour ce pair, l'état
`Synced(g)` ou `Resetting(g)`.

| État | Événement | Action | Nouvel état |
|---|---|---|---|
| Synced(g) | perte locale (§7.2, §5.1, file locale pleine, demande de la webview) | envoyer REPO_RESET(g+1) ; la webview voit le pair partir | Resetting(g+1) |
| Synced(g) | reçu REPO_RESET(h), h > g | envoyer REPO_RESET(h) ; la webview voit le pair partir puis revenir en génération h | Synced(h) |
| Synced(g) | reçu REPO_RESET(h), h ≤ g | ignorer | Synced(g) |
| Resetting(x) | reçu REPO_RESET(y), y ≥ x | si y > x : envoyer REPO_RESET(y) ; la webview voit le pair revenir en génération y | Synced(y) |
| Resetting(x) | reçu REPO_RESET(y), y < x | ignorer | Resetting(x) |
| Resetting(x) | reçu REPO, OFFER ou PRESENCE | jeter (ancienne génération) ; libérer tout objet offert | Resetting(x) |
| Resetting(x) | 10 s sans réponse | renvoyer REPO_RESET(x) | Resetting(x) |
| tout | la session quitte l'état Actif | oublier | — |

- Un pair **NE DOIT PAS** engager plus d'un REPO_RESET par 2 s vers un même pair ; l'intervalle
  double à chaque nouvelle réinitialisation dans les 10 min (jusqu'à 60 s). Cela interdit les
  boucles de réinitialisation.
- Propriétés (vérifiées par le modèle TLA+ de l'ADR 0034) : aucune trame REPO d'une
  génération g n'est livrée après que le récepteur est passé à g' > g ; deux pairs qui
  engagent en même temps convergent vers la même génération ; sans nouvelle perte, les deux
  finissent en `Synced` avec la même génération.

## 8. Vrac et salon

### 8.1 `/inkwell/bulk/1.0.0` (*existant*, sémantique étendue aux objets offerts)

Un flux unidirectionnel par sens et par pair, mêmes trames `u32 length` que §5.2, longueur
≤ 1 065 005. Corps :

```
want    { kind u8 = 1 ; hash [32] ; offset u64 ; length u32 }      offset et length multiples de 16 384, 1 ≤ length ≤ 1 048 576
slice   { kind u8 = 2 ; hash [32] ; offset u64 ; length u32 ; bao [..] }   bao ≤ 1 064 960 octets
missing { kind u8 = 3 ; hash [32] }
busy    { kind u8 = 4 ; hash [32] ; retry_ms u32 ≤ 60 000 }
```

`offset ≤ 2^40`. `bao` est l'encodage « slice » de Bao ([Bao]) avec des groupes de 2^4 blocs
(16 Kio), longueur totale en `u64` petit-boutiste en tête : il se vérifie contre le seul
`hash` (`bulk.json`). Un blob de projet n'est servi que s'il est référencé par le projet et
que le demandeur est un membre actif non suspendu ; sinon MISSING (indiscernable d'un blob
absent). Un serveur sert au plus 8 demandes à la fois, 2 par pair, et répond BUSY au-delà
(au plus 32 BUSY par pair). Un pair qui envoie 3 tranches fausses ou messages mal formés
n'est plus interrogé pendant 10 min.

### 8.2 `/inkwell/lobby/1.0.0` (*existant*)

Un flux **bidirectionnel** ouvert par l'inconnu : une trame requête (≤ 4096 octets, lue en
moins de 10 s), puis une trame réponse (≤ 262 144 octets) ; le traitement côté hôte dure au
plus 3 min (il peut attendre une personne).

```
requête  { kind u8 : 1 Hello | 2 Ask (name str8) | 3 Redeem (project [32] ; secret [16] ; name str8) }
réponse  { kind u8 : 1 Hello (machine str8 ; title str8)
                   | 2 Welcome (root_url str16 ≤ 512 ; n u16 ≤ 1024 ; n × (len u32 ; op))
                   | 3 Refused (reason u8, registre §20.7) }
```

- Hello et Ask ne sont servis que pendant que l'hôte est visible ; Ask attend l'accord d'une
  personne. Redeem présente le secret d'un ticket ; l'hôte le compare **en temps constant**,
  ne l'accepte qu'une fois et avant expiration, puis signe l'ajout du nouvel appareil.
- Welcome porte la liste dans l'ordre d'application. Le nouvel appareil **DOIT** vérifier la
  chaîne depuis la genèse et que l'empreinte de la genèse est le `project` de son ticket :
  un hôte ne peut pas lui donner un autre projet.
- Une raison de refus est volontairement grossière : un ticket faux, utilisé, expiré ou d'un
  autre projet donne toujours `1 Invalid`.

### 8.3 Ticket d'invitation (*existant*)

Texte : `"inkwell1:"` suivi du base32 ([RFC 4648], alphabet standard, sans `=`) de :

```
ticket { version u8 = 1 ; flags u8 (bit 0 : person présent, autres bits = 0) ; role u8 1–3 ;
         project [32] ; [person [32] si bit 0] ; secret [16] ; expires u64 (secondes Unix) ;
         host str8 (PeerID) ; naddr u8 ≤ 16 ; naddr × str8 (multiaddr sans « /p2p/<host> ») }
```

Le texte fait au plus 1024 octets ; à la lecture, la casse, les espaces et les retours à la
ligne sont ignorés. Durée de vie : 15 min. Vecteur : `lobby.json#ticket-text`. Le ticket est
un secret : quiconque le détient peut rejoindre le projet une fois.

### 8.4 Rendez-vous Wormhole (*existant*, [ADR 0032])

Le texte du ticket peut voyager par magic-wormhole ([Wormhole]) : `AppID =
"inkwell.app/invite/v1"`, transfert de texte seul, sans relais de transit ; code de la forme
`^[0-9]+(-[a-z]+)+$`, ≤ 64 caractères. Le PAKE garantit que seul le détenteur du code reçoit
le ticket. Ce canal ne transporte rien d'autre.

## 9. Couche dépôt : messages automerge-repo

### 9.1 Contenu

Les trames REPO et PRESENCE (et les objets offerts) contiennent un message automerge-repo 2.x
encodé en CBOR ([RFC 8949]), décrit par [cddl/repo.cddl](cddl/repo.cddl). Le champ `data` des
messages `sync` et `request` est un message de synchronisation Automerge ([Automerge format]).

### 9.2 Décodage

- Un seul élément CBOR, sans octet après ; ni longueur indéfinie, ni étiquette, ni clé de map
  en double ; sinon refus.
- Les clés inconnues sont **ignorées et non conservées** : le récepteur reconstruit le message
  à partir des seuls champs qu'il connaît. C'est le point d'extension de cette couche.
- `senderId` **DOIT** être le PeerID authentifié de l'émetteur, `targetId` celui du
  récepteur. Les types acceptés sont `arrive`, `welcome`, `sync`, `request`,
  `doc-unavailable` et `ephemeral` ; tout autre type (y compris
  `remote-subscription-change` et `remote-heads-changed`) est refusé.

### 9.3 Poignée de main

`arrive` puis `welcome` (≤ 1024 octets chacun) ouvrent une génération (§7.3). `peerMetadata`
est ignorée : tout pair est traité comme éphémère et son `storageId` n'est jamais utilisé
(aucun état de synchronisation persistant par pair). `projectUrl` (≤ 80 caractères, URL
Automerge valide) annonce le document racine du projet. Un même pair peut renvoyer `arrive`
au plus 3 fois, puis 1 fois par 20 s.

### 9.4 Admission et couverture

Un message de document n'est remis au dépôt Automerge que si :

1. l'émetteur est un membre actif, non suspendu, et ce pair-ci n'est pas suspendu ;
2. `documentId` appartient à l'**ensemble admis** du récepteur ([ADR 0031]) ;
3. pour `sync` et `request`, chaque changement qu'il apporte est **couvert** : il est une tête
   attestée (§4.3) par une personne qui écrit, actuelle et non suspendue, ou un ancêtre d'une
   telle tête (par les dépendances des changements) [ADR 0018]. Un changement non couvert
   n'atteint pas le dépôt.

Un message pour un document pas encore chargé **PEUT** être mis en attente (4 par document et
par pair, 8 Mio par pair) ; au-delà, le récepteur engage REPO_RESET avec ce pair.

### 9.5 Éphémère

`ephemeral` porte la présence (`__presence`) ou le pilotage de présentation (`__present`),
≤ 1024 octets de `data`, et voyage en trame PRESENCE. Budget par pair : 128 messages en
rafale, 64 par seconde ensuite ; au-delà, ils sont jetés. Une trame PRESENCE dont le contenu
n'est pas un `ephemeral` valide est jetée.

## 10. Contrôle de flux et équité

### 10.1 Le principe unique

> Toute file a **un propriétaire, une borne et un crédit**. Un producteur sans crédit attend
> sans rien bloquer d'autre. Une boucle partagée par plusieurs pairs **NE DOIT** jamais
> attendre un pair. Un dépassement est la faute de celui qui l'a causé et ne coûte qu'à lui.

### 10.2 Sur le réseau

- **Crédit** : celui de libp2p. QUIC donne un crédit par flux et par connexion
  ([RFC 9000 §4] : MAX_STREAM_DATA, MAX_DATA) ; yamux, une fenêtre par flux. Un récepteur dont
  la file d'un pair est pleine **DOIT** cesser de lire le flux de ce pair, sans le
  réinitialiser : le transport freine alors l'émetteur, et lui seul.
- **Gros messages** : tirés par le receveur (§7.2), donc jamais reçus sans place.
- **Écriture** : chaque trame a un délai fondé sur le progrès : 20 s sans octet écrit, et au
  total 20 s + taille / 16 Kio/s. Au-delà, le flux est remplacé (§5.1) ; trois échecs en 10 min
  font réinitialiser la connexion avec attente (§13).

### 10.3 Sur le pont interne

Le Stream Wails n'a pas de contrôle de flux : l'Annexe A définit des crédits explicites, avec
la même règle que [RFC 9000 §4.1] (limites absolues, idempotentes) et une exception : un
message plus grand que la fenêtre **PEUT** partir quand rien n'est en vol, sinon il ne
partirait jamais.

### 10.4 Équité

Une boucle qui sert plusieurs pairs (livraison à la webview, traitement des TRUST, tirage du
vrac) choisit le pair suivant par **Deficit Round Robin** ([DRR]) : chaque pair qui a du
travail reçoit un quantum de 64 Kio par tour ; un pair inondant obtient au plus sa part. Les
pairs sans crédit sont sautés sans attendre.

### 10.5 Ordre d'émission vers un pair

Un écrivain par flux sortant. Il sert, dans l'ordre : (1) HELLO, GOAWAY, REPO_RESET ; (2) TRUST ;
(3) ATTEST, REPO et OFFER, dans leur ordre relatif d'origine ; (4) PRESENCE, qui obtient au
moins une trame sur quatre quand (3) attend. Les volumes de (1) et (2) sont bornés par
construction (§16), donc (3) n'est jamais affamé.

## 11. Machines à états

### 11.1 Pair distant (une par appareil distant)

```
          connexion authentifiée            HELLO valides des deux côtés
 Absent ───────────────────────► Sécurisé ─────────────────────────────► Actif ◄──────┐
   ▲      (clé valide §3.2,         │  HELLO absent/faux                   │   │        │ reprise
   │       membre ou fenêtre)       ▼                                      │   ▼        │
   │                              Fermé ◄──── GOAWAY / violation ─────── Restreint ────┘
   │                                │            (suspendu : TRUST seul)   │ suspension
   └──────── attente écoulée ──── Tenu (attente 2 s → 60 s) ◄──────────────┘ retrait → Fermé (sans attente, refusé ensuite)
```

| État | Trames acceptées en entrée | Émises | Sortie |
|---|---|---|---|
| Sécurisé | HELLO seulement | HELLO | Actif ou Restreint (HELLO valides, selon la liste) ; Fermé (10 s, PROJECT_MISMATCH, NO_COMMON_CAPABILITY) |
| Actif | toutes celles de l'ensemble effectif | toutes | Restreint (suspension de l'un des deux) ; Fermé (retrait, violation, fermeture) |
| Restreint | HELLO (remplacement), GOAWAY, TRUST | idem | Actif (reprise) ; Fermé |
| Fermé | aucune | GOAWAY éventuel | Tenu (violation ou surcharge) ou Absent (retrait : la connexion est refusée tant que la liste ne le réadmet pas) |
| Tenu | — | — | Absent après l'attente (2 s, doublée à chaque nouvel incident dans les 10 min, 60 s au plus) |

Entrer en Actif ouvre la génération 1 (§7.3), envoie `have` (§6.1) et les attestations
gardées (§6.2). Quitter Actif ferme la génération : la webview voit le pair partir, les
objets offerts sont libérés, les files du pair sont vidées.

### 11.2 Objet offert

| Côté | États | Transitions |
|---|---|---|
| émetteur | Offert → Tiré → Libéré | Libéré aussi par REPO_RESET, sortie d'Actif, ou 120 s sans WANT (→ REPO_RESET OFFER_EXPIRED) |
| receveur | Annoncé → En tirage → Prêt → Livré | En tirage → Échoué après 3 échecs sur un groupe (→ REPO_RESET PULL_FAILED) ; Prêt attend le crédit de la webview |

### 11.3 Génération de synchronisation

Voir §7.3.

### 11.4 Pair vu de la webview (admission, Annexe A et §9)

`Annoncé(g)` (PeerUp de génération g reçu) → `Poignée de main` (arrive/welcome) →
`Synchronise` → `Parti` (PeerDown, ou PeerUp d'une génération plus récente). Un pair en
attente de réinitialisation est `Refroidi` (15 s, doublé jusqu'à 10 min) et ses messages sont
jetés.

## 12. Ordre de vérification des octets non fiables

Chaque saut a **un seul point d'entrée**, qui applique les étapes dans cet ordre ; une
entrée qui échoue n'atteint jamais l'étape suivante.

| Étape | Réseau → Go (lecteur du pair) | Pont → webview (adaptateur) |
|---|---|---|
| 1. qui | clé valide (§3.2) ; membre, ou fenêtre d'invitation pour le salon seul ; limites de ressources libp2p | enveloppe d'un pair annoncé, génération courante |
| 2. taille | `length` ≤ borne du flux, avant allocation | taille ≤ borne du type |
| 3. droit de parler | type admis dans l'état (§11.1) et dans l'ensemble effectif (§5.4) | type de message admis (§9.2) |
| 4. place | file du pair et de la classe : sinon on cesse de lire (REPO, TRUST), on remplace (PRESENCE) | budgets du pair (§9.3, §9.5) |
| 5. forme | décodage strict (§1.3) | décodage strict (CDDL, §9.2) |
| 6. preuve | signature, projet, rôle (§3.3, §4) | — (fait par Go) |
| 7. admission | standing dans la liste ; journal de décisions | document admis ; couverture (§9.4) |
| 8. effet | appliquer, relayer, mettre en file | remettre au dépôt Automerge |

## 13. Erreurs et réinitialisations

| Portée | Cause | Effet | Reprise |
|---|---|---|---|
| objet | op ou attestation bien formée mais refusée (auteur inconnu, rôle, ancienne) | ignorée, comptée | anti-entropie |
| objet | tranche Bao invalide, message vrac mal formé | faute du pair (3 → 10 min sans l'interroger) | autre pair |
| génération | perte d'un message de dépôt (§7.2, §5.1), file locale pleine, demande de la webview | REPO_RESET (§7.3) | nouvelle génération, Automerge resynchronise |
| connexion | trame mal formée, type interdit, borne dépassée, HELLO faux ou absent, op mal formée, deuxième OFFER en attente, octet reçu sur son flux sortant | GOAWAY PROTOCOL_VIOLATION (ou code précis), fermeture, état Tenu | après l'attente |
| connexion | écritures sans progrès répétées (§10.2) | GOAWAY OVERLOADED, fermeture, Tenu | après l'attente |
| membre | retrait | fermeture, connexion refusée ensuite | réadmission par un admin |
| membre | suspension | Restreint | reprise |

GOAWAY est un avertissement de courtoisie : le récepteur **NE DOIT PAS** en dépendre et
**DOIT** traiter de même une connexion fermée sans GOAWAY.

## 14. Persistance et atomicité

1. **Remplacement atomique** : écrire dans un fichier temporaire du même dossier, `fsync` du
   fichier, renommage, `fsync` du dossier ([ADR 0020], `store.WriteAtomic`). Un ensemble de
   fichiers se prépare dans un dossier `next/` et s'installe par renommage (D6 de l'audit).
2. **Durable avant diffusé** : un objet signé localement (op, attestation que l'on doit
   pouvoir retrouver, décision) **DOIT** être persisté avant sa première émission.
3. **Vérifié avant stocké** : rien de ce qu'envoie un pair n'est écrit avant l'étape 7 du §12.
   Un fichier stocké corrompu est mis en quarantaine (`.bad`), jamais effacé.
4. **Chargement = pli** : recharger un ensemble d'objets donne le même état quel que soit
   l'ordre des fichiers.
5. **Caches** (objets en tirage, tampons) : sous le cache utilisateur
   (`inkwell/…`), dossier privé `0700`, confiné par `os.Root`, nettoyé à la fin, à l'abandon
   et après 24 h d'inactivité ; jamais dans un `/tmp` en mémoire.

## 15. Propriété (informatif : organisation retenue par inkwell)

| État | Propriétaire unique | Accès des autres |
|---|---|---|
| connexions, flux, état §11.1 d'un pair | `p2p.Node` (un lecteur et un écrivain par flux) | instantané immuable publié atomiquement |
| files d'un pair (par classe, entrée et sortie), génération §7.3 | acteur du pair (`internal/transport/session`) | dépôt non bloquant ; refus ou attente du seul producteur |
| liste des membres, journal de décisions, persistance | acteur `Wiring` (une goroutine) | lecture concurrente (verrou court) |
| attestations gardées | magasin concurrent (verrou court, O(1)) | lecteurs de pair (écriture), routeur (lecture) |
| objets offerts, blobs, tirages | `blob.Service` | API non bloquante |
| livraison à la webview, crédits du pont | `bridge.Router` (une goroutine) | état coalescé (rôles, attestations) |
| état Automerge, admission, couverture | webview (`WailsNetworkAdapter`, `admission.ts`, `coverage.ts`) | — |

Une goroutine partagée n'appelle jamais `Send` d'un pair ni une attente sans délai ; elle
dépose dans une file bornée et continue (§10.1).

## 16. Table des bornes

Chaque ligne donne une borne **normative** (sauf « impl. », valeur d'inkwell qu'une autre
implémentation peut choisir plus stricte), son propriétaire, le comportement à la borne et la
reprise. Un test par ligne vérifie la borne, la borne + 1 et la reprise (ADR 0034).

### 16.1 Réseau

| Objet | Borne | Propriétaire | À la borne | Reprise |
|---|---|---|---|---|
| connexions | 64 pairs entrants, 64 sortants (impl.) | `p2p` (gestionnaire de ressources libp2p) | connexion refusée | à la fermeture d'une autre |
| flux par pair | sync 4, vrac 4, salon 4 (impl.) | `p2p` | flux réinitialisé | — |
| trame sync | 98 304 o | lecteur du pair | violation | Tenu |
| trame vrac | 1 065 005 o | lecteur du pair | violation | Tenu |
| salon | requête 4096 o / 10 s ; réponse 262 144 o ; traitement 3 min | `p2p`, `invite` | flux fermé | nouvelle requête |
| HELLO | 1024 o ; 10 s | session | GOAWAY | Tenu |
| file d'entrée REPO d'un pair | 4 Mio et 1024 messages (impl.) | acteur du pair | lecture du flux suspendue | dès que la webview consomme |
| file d'entrée TRUST d'un pair | 512 Kio (impl.) | acteur du pair | lecture suspendue | dès que `Wiring` traite |
| PRESENCE par pair (entrée, sortie) | 16 entrées, dernière par `sessionId` | acteur du pair | la plus ancienne remplacée | — |
| file de sortie TRUST d'un pair | 1 Mio (impl.) | acteur du pair | trame jetée | `have` suivant (15 s) |
| attestations à renvoyer à un pair | ensemble de clés (doc, signataire), sans copie | acteur du pair | — (borné par le magasin) | — |
| sortie dépôt d'un pair | crédit du pont : 4 Mio + 1 objet offert | `bridge.Router` | la webview attend | crédit rendu à l'écriture |
| objets offerts (émetteur) | 1 par pair ; 256 Mio au total ; 120 s sans WANT | `blob.Service` | la webview attend le crédit ; à 120 s, libéré | REPO_RESET |
| tirages (receveur) | 2 WANT par pair, 8 au total ; 30 s par WANT ; 3 échecs ; objets partiels ≤ 512 Mio | `blob.Service` | demande retardée (DRR) | REPO_RESET au 3e échec |
| service du vrac | 8 demandes, 2 par pair ; 32 BUSY par pair ; 3 fautes → 10 min | `blob.Service` | BUSY, puis silence | délai |
| transferts de blobs | 4 en même temps ; 8 essais par plage ; 5 BUSY attendus ≤ 5 s | `blob.Service` | file d'attente | — |
| écriture d'une trame | 20 s sans progrès ; 20 s + taille / 16 Kio/s au total | écrivain du flux | flux remplacé | REPO_RESET si perte |
| REPO_RESET | 1 par 2 s par pair, doublé jusqu'à 60 s (mémoire 10 min) ; renvoi 10 s | acteur du pair | réinitialisation retardée | — |
| connexion réinitialisée | attente 2 s, doublée jusqu'à 60 s (mémoire 10 min) | `p2p` | connexion refusée | après l'attente |
| liste des membres | 1024 ops (scellée) ; 256 en attente, 2 périodes ; `have` ≤ 256 têtes ; 2 réponses par pair et période | `trust` | refus (scellée) ; oubli | nouveau projet (scellée) |
| op / attestation / décision | 4096 / 4096 / 8192 o ; 64 parents ; 64 têtes ; 96 entrées | décodeurs | refus | — |
| journal de décisions | 8192 ops, 1024 par auteur | `policy` | refus | compaction (à venir, S2-R2) |
| attestations gardées | dernière par (doc, signataire) ; ≤ 65 536 clés (impl.) | magasin d'attestations | nouvelle clé refusée, journalisée | ménage à la révocation |
| attestations d'un pair non actif | 40 000 (impl., aujourd'hui `maxHeld`) | `Wiring` | refus | activation ou départ |
| signatures en attente (non membre) | 10 000 documents (`maxPending`) | `Wiring` | refus | admission |
| ticket | 1024 caractères ; 15 min ; usage unique | `invite` | refus | nouveau ticket |

### 16.2 Webview (couche dépôt)

| Objet | Borne | Propriétaire | À la borne | Reprise |
|---|---|---|---|---|
| poignée de main | 1024 o ; 3 `arrive` puis 1 / 20 s | `admission.ts` | jetée | attente |
| identifiants | PeerID, DocumentId ≤ 64 ; `projectUrl` ≤ 80 | `admission.ts` | message refusé | — |
| éphémère | `data` ≤ 1024 o ; 128 en rafale, 64/s | `admission.ts` | jeté | seau qui se remplit |
| attente par document non prêt | 4 par document, 8 Mio par pair, message ≤ 64 Mio | `admission.ts` | REPO_RESET (via le pont) ; refroidi 15 s → 10 min | nouvelle génération |
| couverture | 256 têtes par (doc, signataire) ; 10 000 documents ; 1024 par signataire ; 1024 refusés ; 256 réécrits par pair, 4096 au total | `coverage.ts` | les plus anciennes oubliées | réattestation |
| file de sortie d'un pair (en attente de crédit) | 16 Mio et 4096 messages (impl.) | `WailsNetworkAdapter` | demande de REPO_RESET | nouvelle génération |

### 16.3 Pont interne (Annexe A)

| Objet | Borne | À la borne |
|---|---|---|
| enveloppe `net` | 64 Mio (limite Wails) ; pair ≤ 64 o | refus |
| crédit par (pair, sens) | fenêtre 4 Mio ; dépassement autorisé si rien n'est en vol | violation : session du pont fermée, la webview se recharge |
| notifications d'état (rôles, attestations, suspension) | coalescées : dernière valeur par clé | — |
| `store` | 1026 voies ; page ≤ 1 Mio ; valeur ≤ 32 Mio ; clé ≤ 4 composants de 128 o | `StatusTooLarge` / refus |
| `preview` | requête ≤ 1 Kio ; 40 pages ; trame ≤ 48 Mio | refus, échec « tooLarge » |
| `present` | trame ≤ 8 Mio ; 6 fenêtres ; 256 trames / 32 Mio par fenêtre | trame suivante jetée |

## 17. Considérations de sécurité

- **Modèle de menace** (N7) : n'importe quel membre, y compris une personne qui écrit ou un
  admin retiré, peut être malveillant ; il ne doit jamais gêner les autres. Les inconnus ne
  parlent qu'au salon, pendant une fenêtre d'invitation. Les admins en place sont de confiance
  ([ADR 0017], résiduel R7 de l'ADR 0031).
- **Authentification** : libp2p (TLS 1.3 sur QUIC, Noise sur TCP) lie chaque connexion à une
  clé Ed25519. Le relais voit qui parle à qui et combien, jamais le contenu. La validation des
  clés (§3.2) est indispensable : sans elle, une seule clé d'ordre petit enregistrée comme
  membre rend son identité publique.
- **Autorisation** : la liste signée décide qui se connecte et avec quel rôle ; une op est
  rejouable mais idempotente (adressée par son contenu) ; les attestations sont rejouables
  mais seule la plus récente compte, et elles meurent avec le rôle de leur signataire.
  L'écriture est contrôlée par la couverture (§9.4) ; l'auteur exact de chaque changement
  (ChangeSig, capacité `changesig/1`) et l'équivocation (R1 accepté par N10) relèvent de
  l'[ADR 0031].
- **Épuisement de ressources** : chaque file, chaque carte et chaque octet disque a une borne
  et un propriétaire (§16) ; un pair ne peut retenir que ce qu'il a lui-même envoyé et que le
  receveur a accepté ; aucun gros message n'est reçu sans avoir été demandé (§7.2) ; aucune
  boucle partagée n'attend un pair (§10.1). Le quota disque cumulé par personne qui écrit
  (N10) borne la croissance d'historique.
- **Rétrogradation** : la négociation passe dans une session déjà authentifiée ; un tiers ne
  peut pas retirer une capacité. Un pair malveillant qui en annonce moins ne gêne que sa propre
  session.
- **Horloges** : l'expiration du ticket et le `seq` des attestations (millisecondes Unix au
  moment de signer) dépendent de l'horloge locale. Un appareil dont l'horloge a reculé émet
  des `seq` plus petits, ignorés jusqu'à ce qu'il réapprenne son dernier `seq` par commérage ;
  il **DEVRAIT** donc prendre `max(dernier seq connu + 1, maintenant)` après avoir reçu les
  attestations de la session (ce qu'inkwell fait).
- **Secrets** : le secret du ticket est comparé en temps constant, à usage unique, et n'est
  jamais journalisé ; aucun octet de charge n'est journalisé.

## 18. Considérations de confidentialité

Selon [RFC 6973] :

- **Relais** : il connaît les PeerID et adresses des deux bouts, l'heure et le volume. Le
  mode « relais seulement » cache l'adresse IP aux autres membres.
- **Réseau local** : mDNS (`_inkwell._udp`) annonce le PeerID de l'appareil ; pendant une
  fenêtre visible, `Hello` du salon révèle le nom de la machine et le titre du projet à
  quiconque sur le réseau local. La découverte locale se désactive.
- **Ticket** : il contient le `ProjectID`, le PeerID et les adresses IP de l'hôte ; il se
  partage comme un mot de passe. Le serveur Wormhole public voit un code et un horaire, pas le
  ticket.
- **Entre membres** : HELLO révèle le `ProjectID` (déjà connu d'un membre) ; les attestations
  révèlent l'heure de chaque série de modifications ; la présence révèle curseur et nom
  affiché. Rien de cela ne sort du projet.

## 19. Écarts avec le code au 2026-11-01

Les parties *existantes* sont émises telles quelles par inkwell. Le reste (`sync/2.0.0`,
HELLO, OFFER, REPO_RESET, crédits du pont, validation §3.2) est à implémenter selon les étapes
de l'[ADR 0034] ; l'[inventaire](inventaire.md) relie chaque écart à `fichier:ligne`.

## 20. Registres

### 20.1 Types de trame `sync/2.0.0`

`0x00` réservé (jamais émis) ; `0x01`–`0x08` : §5.2 ; `0x09` POLICY (`policy/1`) ; `0x0A`
HISTORY (`history/1`) ; `0x0B` CHANGESIG (`changesig/1`) ; `0x0C`–`0x7F` : à attribuer par une
version mineure, chacun lié à une capacité ; `0x80`–`0xFF` : réservés à une majeure.

### 20.2 Capacités (CAPS) et identifiants TLV du HELLO

| id | Capacité | État |
|---|---|---|
| 0x0001 | `core/2` : trames 0x01–0x08 | obligatoire |
| 0x0002 | `member-op/2` : op de membre version 2 (§4.2) | réservée, inerte |
| 0x0003 | `policy/1` : décisions (§4.4), trame POLICY | réservée, inerte |
| 0x0004 | `history/1` : pages d'historique (ADR 0031 M5) | réservée |
| 0x0005 | `changesig/1` : signature par changement (ADR 0031 M1) | réservée |
| 0x?A?A | GREASE (16 valeurs) | à ignorer |

TLV du HELLO : `0x0001` PROJECT, `0x0002` CAPS, `0x?A?A` GREASE ; le reste à attribuer.

### 20.3 Codes GOAWAY

`0x01` NO_ERROR · `0x02` PROTOCOL_VIOLATION · `0x03` HELLO_TIMEOUT · `0x04` PROJECT_MISMATCH ·
`0x05` NO_COMMON_CAPABILITY · `0x06` NOT_MEMBER · `0x07` OVERLOADED. Un code inconnu se lit
comme PROTOCOL_VIOLATION.

### 20.4 Domaines de signature

`trust-op` (§4.1, §4.2) · `attest` (§4.3) · `policy-op` (§4.4) · réservés : `change`
(ADR 0031 M1), `actor`, `recovery`. Un domaine ne contient ni `/` ni octet nul.

### 20.5 Espaces de noms des décisions (§4.4)

`mode` (`approval` | `free`) · `main` (hex de 32 caractères) · `doc/<hex32>` et `node/<hex32>`
(`kept` | `trashed` | `purged`) · `quota/<hex64>` (`grant`) · `legacy/<hex32>` (têtes hex64
triées, séparées par des virgules, ≤ 64) · `equiv/<hex32>/<seq décimal ≥ 1>` (hex64).

### 20.6 Raisons de REPO_RESET

`0x01` LOCAL (demande de la webview) · `0x02` PULL_FAILED · `0x03` OFFER_EXPIRED ·
`0x04` QUEUE_OVERFLOW · `0x05` STREAM_LOST · `0x06` MEMBERSHIP. Une raison inconnue est
acceptée (elle ne sert qu'au diagnostic).

### 20.7 Raisons de refus du salon

`1` Invalid · `2` Declined · `3` Busy · `4` Failed · `5` TooLarge.

## 21. Conformité et vecteurs

- Les fichiers [vectors/](vectors/) sont normatifs. Chacun contient
  `{spec, spec_version, keys?, vectors: [{id, status, description, decoder, input_hex,
  verdict, expect?, current_go?}]}`. `verdict` vaut `accept` ou `reject` ; `expect` donne les
  champs ou octets qu'une implémentation **DOIT** reproduire (empreinte, octets signés,
  signature, PeerID, texte) ; `status` vaut `normative` (octets déjà émis) ou `proposed` (v2,
  à implémenter) ; `current_go` signale un vecteur que le code actuel ne passe pas encore.
- Décodeurs nommés : `public_key`, `signature`, `peer_id`, `trust_op`, `trust_op+apply`,
  `trust_sync`, `attestation`, `attestation+verify`, `policy_op`, `bulk`, `ticket_text`,
  `lobby_request`, `lobby_reply`, `sync2_frame`, `bridge_envelope` (interne).
- Une implémentation est **conforme** si elle donne le verdict attendu sur tous les vecteurs
  `normative` et `proposed` de la version qu'elle annonce et reproduit chaque `expect`. inkwell
  exécute ces fichiers dans ses tests Go et Vitest ; ils sont produits par les vrais codecs
  (`vectors/generate.go.txt`, à transformer en commande de régénération, ADR 0034 étape 1).
- Les clés de test sont celles de [RFC 8032 §7.1] (TEST 1 et TEST 2) : jamais de vraies clés.

## 22. Références

- [RFC 2119], [RFC 8174] : mots-clés BCP 14. <https://www.rfc-editor.org/rfc/rfc8174>
- [RFC 3552] et [RFC 6973] : considérations de sécurité et de confidentialité.
- [RFC 4648] : base32.
- [RFC 7301] : ALPN. [RFC 9113] : HTTP/2 (§5.2 contrôle de flux, §6.5 SETTINGS).
- [RFC 8032] : EdDSA ; §5.1.7 vérification ; §7.1 vecteurs. <https://www.rfc-editor.org/rfc/rfc8032>
- [RFC 8610] : CDDL. [RFC 8949] : CBOR (§4.2.1 encodage déterministe, §5.6 clés en double).
- [RFC 8701] : GREASE. [RFC 9000] : QUIC (§4 contrôle de flux). [RFC 9413] : protocoles robustes.
- [Chalkias 2020] : K. Chalkias, F. Garillot, V. Nikolaenko, *Taming the many EdDSAs*, SSR 2020,
  <https://eprint.iacr.org/2020/1244>.
- [DRR] : M. Shreedhar, G. Varghese, *Efficient Fair Queuing using Deficit Round Robin*,
  IEEE/ACM ToN 4(3), 1996.
- [Bao] : J. O'Connor, spécification Bao, <https://github.com/oconnor663/bao/blob/master/docs/spec.md> ;
  implémentation `lukechampine.com/blake3/bao` v1.4.1 (groupes de 16 Kio) ; même découpage
  que `bao-tree` d'iroh.
- [Automerge format] : <https://automerge.org/automerge-binary-format-spec/> ; messages
  automerge-repo : <https://automerge.org/automerge-repo/types/_automerge_automerge-repo.RepoMessage.html>.
- libp2p : <https://github.com/libp2p/specs> (noise, tls, quic, yamux, relay/circuit-v2, dcutr, mdns).
- [Wormhole] : magic-wormhole, <https://magic-wormhole.readthedocs.io/>.
- [ADR 0005], [ADR 0017], [ADR 0018], [ADR 0020], [ADR 0021], [ADR 0023], [ADR 0030],
  [ADR 0031], [ADR 0032], [ADR 0034] : `docs/adr/`.

[RFC 2119]: https://www.rfc-editor.org/rfc/rfc2119
[RFC 8174]: https://www.rfc-editor.org/rfc/rfc8174
[RFC 3552]: https://www.rfc-editor.org/rfc/rfc3552
[RFC 6973]: https://www.rfc-editor.org/rfc/rfc6973
[RFC 4648]: https://www.rfc-editor.org/rfc/rfc4648
[RFC 7301]: https://www.rfc-editor.org/rfc/rfc7301
[RFC 9113]: https://www.rfc-editor.org/rfc/rfc9113
[RFC 9113 §6.5]: https://www.rfc-editor.org/rfc/rfc9113#section-6.5
[RFC 8032]: https://www.rfc-editor.org/rfc/rfc8032
[RFC 8032 §5.1.7]: https://www.rfc-editor.org/rfc/rfc8032#section-5.1.7
[RFC 8032 §7.1]: https://www.rfc-editor.org/rfc/rfc8032#section-7.1
[RFC 8610]: https://www.rfc-editor.org/rfc/rfc8610
[RFC 8949]: https://www.rfc-editor.org/rfc/rfc8949
[RFC 8701]: https://www.rfc-editor.org/rfc/rfc8701
[RFC 9000]: https://www.rfc-editor.org/rfc/rfc9000
[RFC 9000 §4]: https://www.rfc-editor.org/rfc/rfc9000#section-4
[RFC 9000 §4.1]: https://www.rfc-editor.org/rfc/rfc9000#section-4.1
[RFC 9413]: https://www.rfc-editor.org/rfc/rfc9413
[Chalkias 2020]: https://eprint.iacr.org/2020/1244
[DRR]: https://doi.org/10.1109/90.502236
[Bao]: https://github.com/oconnor663/bao/blob/master/docs/spec.md
[Automerge format]: https://automerge.org/automerge-binary-format-spec/
[Wormhole]: https://magic-wormhole.readthedocs.io/
[ADR 0005]: ../adr/0005-trust-model-v1.md
[ADR 0017]: ../adr/0017-member-list-seniority.md
[ADR 0018]: ../adr/0018-signed-heads-write-control.md
[ADR 0020]: ../adr/0020-storage-in-go-store-stream.md
[ADR 0021]: ../adr/0021-suspension.md
[ADR 0023]: ../adr/0023-projet-multi-fichiers-blobs.md
[ADR 0030]: ../adr/0030-decisions-signees-garde-structurelle.md
[ADR 0031]: ../adr/0031-frontiere-admission-chaines-documents.md
[ADR 0032]: ../adr/0032-canal-invitation-wormhole.md
[ADR 0034]: ../adr/0034-protocole-unifie.md

## Annexe A — Pont interne Go ↔ webview (hors contrat public)

Ces Streams Wails relient le Go et la webview **d'un même exécutable** : aucun port local
([ADR 0009]), aucun pair distant. Les deux côtés sont construits ensemble ; un octet de version
différent ferme le Stream. Ces règles sont normatives pour inkwell seulement.

[ADR 0009]: ../adr/0009-bridge-wails-streams-gtk4.md

### A.1 Stream `net`

```
enveloppe { version u8 = 1 ; kind u8 ; peer str8 (PeerID, 1–64) ; payload [reste] }   ≤ 64 Mio
```

| kind | Nom | Sens | payload |
|---|---|---|---|
| 1 | PeerUp | Go → webview | `generation u32` (nouveau) : début d'une génération §7.3 |
| 2 | PeerDown | Go → webview | vide |
| 3 | Message | les deux | message automerge-repo (REPO, objet offert) |
| 4 | Attest | webview → Go | `Heads` : `doc str16 (1–256) ; têtes [32] × ≤ 64` |
| 5 | Attested | Go → webview | `Heads` d'un signataire `peer` valable |
| 6 | Revoked | Go → webview | vide : les attestations de `peer` sont caduques |
| 7 | Sealed | Go → webview | vide : liste pleine |
| 8 | Roles | Go → webview | `n u16 ≤ 1024 ; n × (peer str8 ; role u8 1–3 ; flags u8 bit 0 suspendu)` |
| 9 | Suspended | Go → webview | `u8` : 1 suspendu, 0 repris ; `peer` = admin auteur |
| 10 | Decide | webview → Go | intention d'admin (inerte) |
| 11 | Policy | Go → webview | état plié des décisions (inerte) |
| 12 | Credit | les deux | `generation u32 ; consumed u64 ; window u32` (nouveau) |
| 13 | Presence | les deux | message `ephemeral` (nouveau ; trame PRESENCE) |
| 14 | Reset | webview → Go | `generation u32` : la webview demande REPO_RESET (nouveau) |

- **Crédits** : pour chaque (pair, génération, sens), l'émetteur compte les octets de
  `payload` des Message et Presence envoyés (`sent`). Le receveur annonce `consumed` (octets
  dont il a fini le traitement) et `window` ; l'émetteur **DOIT** respecter
  `sent + taille ≤ consumed + window`, ou `sent = consumed` (rien en vol, un seul message
  plus gros que la fenêtre). Un Credit dont la génération n'est pas courante est ignoré ; un
  `consumed` qui recule est ignoré. Côté Go → webview, « traité » signifie remis au dépôt
  Automerge puis, quand les reçus de cycle de vie existeront (étapes 2.4/2.5b du plan
  d'admission), traitement terminé ; côté webview → Go, écrit sur le flux du pair ou objet
  offert entièrement tiré.
- **Ordre** : avant de livrer un Message d'un pair, le routeur envoie les notifications d'état
  en attente (Roles, Attested, Revoked, Suspended), coalescées à leur dernière valeur.
- **Validation tardive** : `Decide` et `Policy` sont validés par leur gestionnaire, après
  l'aiguillage par `kind` et le contrôle de taille, pas par le décodeur générique d'enveloppe.

### A.2 Stream `store` ([ADR 0020])

Requêtes `id u32 ; op u8 ; nkey u8 ; clé ; data`, réponses `id u32 ; status u8 ; payload`
(v1) ; v2 ajoute voies, générations, Cancel/Released et pages d'au plus 1 Mio
(`internal/bridge/storev2.go`). La version **DOIT** se choisir une fois, par une première
trame à l'ouverture du Stream, et non à chaque trame par un préfixe magique.

### A.3 Streams `preview` et `present`

`preview` : requêtes JSON ≤ 1 Kio, réponses (pages, nombre de pages, diagnostics) ≤ 48 Mio.
`present` : trames JSON ≤ 8 Mio dont `type` est un mot de 1 à 16 lettres minuscules, relayées
telles quelles entre les fenêtres d'une même machine (≤ 6), 256 trames / 32 Mio en attente par
fenêtre, la suivante jetée au-delà.
