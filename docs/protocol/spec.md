# Protocole inkwell — spécification normative

| | |
|---|---|
| Version | **2.0.0-brouillon.10** (SemVer, §1.4) |
| Date | 2026-11-01 |
| Statut | proposé ([ADR 0034](../adr/0034-protocole-unifie.md)) ; les parties marquées *existant* décrivent des octets déjà émis par inkwell et sont figées |
| Révision | 10 : revues [01-codex](reviews/01-codex.md), [02-fable](reviews/02-fable.md), [03-codex](reviews/03-codex.md), [04-fable](reviews/04-fable.md), [05-codex](reviews/05-codex.md), [06-fable](reviews/06-fable.md), [07-codex](reviews/07-codex.md), [08-fable](reviews/08-fable.md), le débat [09-debate-D1](reviews/09-debate-D1.md) (accord), [10-codex](reviews/10-codex.md), [11-fable](reviews/11-fable.md), [12-codex](reviews/12-codex.md), [13-fable](reviews/13-fable.md), [14-codex](reviews/14-codex.md), [15-fable](reviews/15-fable.md), [16-codex](reviews/16-codex.md), [17-fable](reviews/17-fable.md), [18-codex](reviews/18-codex.md), [19-fable](reviews/19-fable.md) et le débat [20-debate-D2](reviews/20-debate-D2.md) (accord) intégrés ; correspondance dans [CHANGES.md](CHANGES.md) |
| Aperçu | [README.md](README.md) |
| Vecteurs, modèle | [vectors/](vectors/) (§21), [tla/](tla/) (§7.3) |
| Paquets | module Go `github.com/oioio-space/inkwell/protocol`, paquet TypeScript `@inkwell/protocol` (§1.6) |

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
acceptée. Ce choix suit [RFC 9413] : la tolérance n'est admise qu'aux points d'extension
prévus (TLV du HELLO §5.3, clés des maps CBOR ouvertes §9.2), jamais ailleurs.

### 1.4 Versions, compatibilité et évolution (exigence N14)

**But normatif.** Un nœud d'une version et un nœud d'une version plus récente ou plus
ancienne **DOIVENT** soit se comprendre, soit se refuser avec une raison définie, jamais se
mal comprendre ; on **DOIT** pouvoir ajouter des fonctions sans casser les anciens nœuds. Les
règles suivent [RFC 6709] (extensions), [RFC 9413] (tolérance seulement aux points
d'extension, extensions exercées), [RFC 8701] (GREASE) et la négociation de version de QUIC
([RFC 9000 §15], [RFC 9368]).

#### 1.4.1 Deux sortes de changement

| Sorte | Exemples | Version |
|---|---|---|
| **extension compatible** | TLV de HELLO, clé facultative d'une map CBOR ouverte, nouveau `type` de message de la couche dépôt, trame de la plage ignorable, nouvelle capacité qui ajoute des trames critiques, nouvel ensemble SET ou nouvelle version d'objet derrière une capacité | **mineure** (`2.1.0`) ; l'identifiant de protocole ne change pas |
| **changement incompatible** | mise en page d'une trame existante, sens d'un champ, retrait d'une fonction, règle de sécurité différente | **majeure** : nouveaux identifiants `/inkwell/<classe>/3.0.0` |

Correctif (`2.0.1`) : texte seulement.

#### 1.4.2 Ce qu'on fait de l'inconnu

| Inconnu | Règle | Erreur si refus |
|---|---|---|
| identifiant de protocole (majeure) | multistream-select : celui qui ouvre propose ses identifiants **du plus récent au plus ancien** ; le premier accepté est la plus haute majeure commune (symétrique) | aucune commune : message clair (§1.4.5) |
| TLV de HELLO | identifiant **pair** : ignoré ; identifiant **impair** : « à comprendre », refus | GOAWAY UNSUPPORTED_CRITICAL (`compat.json#tlv-unknown-odd-critical`) |
| capacité | ignorée (l'ensemble effectif est l'intersection) | — (`#cap-unknown`) |
| type de trame `0x40`–`0x7F` (plage **ignorable**) | sautée grâce à sa longueur, comptée (au plus 256 trames et 256 Kio par minute et par pair, puis OVERLOADED ; une extension périodique plus bavarde passe par une capacité, §20.1) | — (`#frame-ignorable-unknown`, `#frame-grease`) |
| type de trame `0x01`–`0x3F` (plage **critique**) non négocié | violation : une trame critique n'existe que par capacité | GOAWAY PROTOCOL_VIOLATION (`#frame-critical-unknown`) |
| classe de flux, ensemble SET, `kind` d'un SET | critiques : n'existent que par capacité | PROTOCOL_VIOLATION (`#set-unknown`) |
| version d'objet signé | l'objet est **ignoré sans sanction**, jamais lu comme une autre version (l'octet de version est dans les octets signés) | — (`#object-version-unknown`) |
| clé d'une map CBOR ouverte | ignorée et non conservée ; si son nom commence par `!`, « à comprendre » : message refusé | refus (`repo.json#compat-critical-key`) |
| `type` d'un message de la couche dépôt | ignoré sans erreur, ni violation ni règle L (`remote-*` restent refusés) | — (`repo.json#compat-unknown-type`) |
| clé ou valeur d'une map CBOR **fermée** (présence) | message jeté (classe à perte) | — |

Les extensions ne peuvent pas devenir une attaque par épuisement : tout ce qui est ignoré
est d'abord borné (taille de trame, TLV ≤ 1024 o au total, profil CBOR §9.2.1 : profondeur,
nombre d'éléments, taille des maps, **y compris dans les champs ignorés**), et les trames
ignorables sont comptées.

#### 1.4.3 Négociation authentifiée, sans rétrogradation

- multistream-select et HELLO passent **à l'intérieur** du canal chiffré et authentifié
  (TLS 1.3 sur QUIC, Noise sur TCP, de bout en bout même à travers un relais circuit v2) : un
  intermédiaire ne peut ni retirer un identifiant ni modifier le HELLO. Signer le HELLO
  n'ajouterait rien que le canal ne garantisse déjà.
- **Liaison de la négociation** (comme la *Version Information* de [RFC 9368]) : le HELLO
  porte `VERSIONS` (TLV `0x0004`), les majeures que l'émetteur parle. Le receveur **DOIT**
  vérifier que la majeure du flux est la plus haute commune entre ses majeures et `VERSIONS` ;
  sinon GOAWAY VERSION_DOWNGRADE (`compat.json#versions-downgrade`, `#versions-ok`).
- Ce qui porte la sécurité est dans `core/2` et ne se négocie pas (§5.4).

#### 1.4.4 Objets signés et données stockées

- La version d'un objet est liée à sa signature de deux façons : le domaine signé se termine
  par `/vN` (§3.3, pour tous les objets), et l'octet de version de l'objet, quand il en a un,
  fait partie des octets signés (après le `ProjectID` pour `trust-op`, `policy-op`,
  `final-ack` et `listing-op` ; en tête pour `attest`). Le tuple d'auteur (`change`) n'a pas
  d'octet de version : seul son domaine le versionne. Un vérificateur ancien et un récent ne
  peuvent donc pas lire le même octet-chaîne signé de deux façons. Une nouvelle version d'objet
  change l'octet de version, et le suffixe de domaine si la signature couvre autre chose.
- **Toujours lisible** : une implémentation lit et vérifie toutes les versions d'objet définies
  jusqu'à la sienne, pour toujours (un projet en dépend : son identifiant est l'empreinte de sa
  genèse). Le retrait d'une version n'arrête que son **émission**.
- **Récent → ancien** : une version d'objet nouvelle n'est émise ou relayée qu'aux pairs dont
  l'ensemble effectif contient sa capacité. Si elle change un pli (liste, décisions, ensemble
  admis), elle n'est émise qu'une fois que la décision d'admin `format = <mineure>` (§20.5)
  l'autorise pour le projet ; un nœud qui ne connaît pas cette mineure passe le projet en
  **lecture seule** et le dit, au lieu de calculer un autre pli que les autres.
- **Ancien → récent** : un nœud récent lit tout ce qu'un ancien a signé ; rien à faire.
- **Pas d'arrêt silencieux** : un nœud qui reçoit un objet de version inconnue ne le stocke ni
  ne le relaie (il ne peut pas le vérifier) ; il le compte et le **dit** à la personne (« une
  partie de la liste vient d'une version plus récente d'inkwell : mettez à jour »). Comme une
  version qui change un pli attend la décision `format`, ce cas ne survient qu'avant cette
  décision, chez les nœuds qui la refuseront de toute façon.

#### 1.4.5 Politique de support et de retrait

- Un nœud parle la majeure courante **et la précédente** pendant au moins **12 mois** après la
  publication de la nouvelle (exception : le passage 1 → 2, direct, décidé le 2026-11-01 parce
  qu'aucune version publiée n'avait d'utilisateurs qui se synchronisent).
- Une fonction se retire seulement à une majeure, après avoir été annoncée comme dépréciée
  pendant au moins une mineure (journal des changements du module).
- Face à un pair **trop ancien ou trop récent** (aucune majeure commune), le nœud lit la liste
  des protocoles du pair par libp2p *identify* et le **dit à la personne**, sans échec
  silencieux : « L'appareil de Béa utilise une version d'inkwell trop ancienne : il doit être
  mis à jour pour travailler avec vous » (ou « … plus récente : mettez inkwell à jour »).

#### 1.4.6 Anti-ossification (GREASE)

Un émetteur **DEVRAIT**, au hasard et de temps en temps (au moins une session sur huit),
envoyer : une capacité et un TLV GREASE (`0x?A?A`, identifiant pair) ; une trame GREASE
(`0x4A`, `0x5A`, `0x6A` ou `0x7A`, corps aléatoire ≤ 64 o) sur CONTROL ; une clé GREASE
`_g` + 4 chiffres hexadécimaux dans une map CBOR ouverte ; un message de couche dépôt de
`type` `grease-` + 4 chiffres hexadécimaux. Un receveur **DOIT** tous les ignorer. Les vecteurs
en contiennent (`compat.json`, `repo.json#compat-grease-*`).

#### 1.4.7 Versions des paquets (N13)

Le **module** `protocol` suit son propre SemVer, qui décrit son **API Go**, pas le fil :
`v0.x` jusqu'à l'acceptation de la spec, `v1.0.0` ensuite ; une mineure du module ajoute de
l'API (dont les fonctions d'une mineure du protocole), une majeure du module change l'API (et
le chemin d'import, `/v2`). Le fil est exposé par paquet de majeure (`protocol/sync2`, plus
tard `protocol/sync3`, côte à côte pendant la période de support) et par les constantes
`sync2.ProtocolID` et `sync2.Minor`. Le README du module tient la table « version du module →
identifiants de protocole et mineures ». L'exécuteur de conformité est paramétré par la
version de protocole visée. `@inkwell/protocol` suit les mêmes règles.

### 1.5 Périmètre

| Partie | Statut | Sections |
|---|---|---|
| Identités, cryptographie, objets signés, pli de la liste, ensemble admis | public | §3, §4 |
| Transport libp2p, `control`/`repo`/`presence`/`large` `2.0.0`, `bulk/1.0.0`, `lobby/1.0.0`, ticket | public | §2, §5–§8 |
| Messages automerge-repo, profil CBOR, admission, garde structurelle | public | §9 |
| Contrôle de flux, machines à états, erreurs, bornes | public (comportement observable) | §10–§16 |
| Pont Go ↔ webview (Streams Wails `net`, `store`, `preview`, `present`) | **interne**, hors contrat | Annexe A |
| Organisation du code (acteurs, paquets) | informatif | §1.6, §15 |

### 1.6 Paquets de référence (exigence N13)

La spécification a deux implémentations de référence, utilisables par d'autres
applications et utilisées par inkwell lui-même (aucune copie privée des codecs) :

| Paquet | Contenu | Dépendances permises |
|---|---|---|
| module Go `github.com/oioio-space/inkwell/protocol` (dossier `protocol/`, `go.mod` propre, SemVer `v0.x` tant que la spec est un brouillon, étiquettes `protocol/vX.Y.Z`) | cœur **pur** : types et codecs du fil (§4–§8), profil CBOR et schémas (§9), validation des clés et signatures (§3), pli de la liste (§4.9), ensemble admis (§4.10), machines à états (§7.3, §11) sans E/S ni goroutine, table des bornes en constantes (§16), vecteurs embarqués (`go:embed`) et exécuteur de conformité exporté (§21) | bibliothèque standard, `filippo.io/edwards25519`, `lukechampine.com/blake3` ; **rien** d'inkwell, ni libp2p, ni Wails, ni Automerge |
| module Go `github.com/oioio-space/inkwell/protocol/libp2p` (sous-module, `go.mod` propre) | liaison libp2p : flux par classe, préambules, flux LARGE, lecture au rythme du receveur, `bulk/1.0.0` | le cœur, go-libp2p |
| paquet TypeScript `@inkwell/protocol` (`frontend/packages/protocol`, espace de travail pnpm) | profil CBOR, schémas §9, garde (§9.7) et couverture, crédits du pont, mêmes vecteurs | `cbor-x` (décodage), `@automerge/automerge` (garde) |

Règles de dépendance vérifiées par `scripts/arch-check.sh` : `protocol/...` n'importe rien
de `github.com/oioio-space/inkwell/internal` ni de `cmd` ; l'application importe
`protocol` ; seuls `internal/transport/p2p` et `protocol/libp2p` importent libp2p ; aucun
code Go n'importe Automerge (ADR 0011 inchangée : le cœur Go valide les enveloppes CBOR du
§9, jamais le contenu Automerge). Choix d'un sous-module dans ce dépôt plutôt qu'un dépôt
séparé : la spec, les vecteurs, le code et l'application changent dans le même commit, la CI
les teste ensemble, et Go sait versionner un sous-module (`protocol/v0.3.0`). Un dépôt séparé
reste possible à la 1.0 sans changer le chemin d'import si le module est servi par un
`go-import` personnalisé ; ce n'est pas nécessaire avant.

## 2. Architecture en couches

```
 L4  dépôt        messages automerge-repo (CBOR, profil §9.2) · admission · couverture
                  · garde structurelle (G1–G12) · pages d'historique
 L3  classes      ÉTAT (SET : anti-entropie d'ensembles signés ; ATTEST : dernier gagne)
                  DÉPÔT (fiable, générations, flux LARGE) · PRÉSENCE (à perte)
 L2  session      un protocole libp2p par classe de flux : /inkwell/control/2.0.0 (HELLO,
                  capacités, GOAWAY, SET, ATTEST), /inkwell/repo/2.0.0 (génération dans le
                  préambule), /inkwell/presence/2.0.0, /inkwell/large/2.0.0 (> 64 Kio)
                  /inkwell/bulk/1.0.0 : blobs (WANT/SLICE/MISSING/BUSY, Bao, BLAKE3)
                  /inkwell/lobby/1.0.0 : une requête, une réponse (inconnus invités)
 L1  transport    libp2p : QUIC v1 (TLS 1.3) ou TCP + Noise + yamux ; relais circuit v2 ;
                  DCUtR ; mDNS « _inkwell._udp » ; contrôle de flux par flux et par connexion
 L0  identité     clé Ed25519 par appareil · PeerID · signatures à domaine séparé
```

Chaque couche ne fait confiance qu'à ce que la couche du dessous a vérifié, dans l'ordre du
§12. Le seul mécanisme de contre-pression du réseau est celui du transport, par flux
([RFC 9000 §4.1] : MAX_STREAM_DATA ; fenêtre par flux de yamux) : un receveur qui ne lit pas
un flux ne freine que ce flux.

## 3. Identités et cryptographie

### 3.1 Clés et PeerID

- Chaque appareil possède une paire Ed25519 ([RFC 8032]). La clé publique (32 octets) est
  son identité dans tous les objets signés.
- Le PeerID est l'identifiant libp2p de cette clé : base58btc du multihash *identité*
  (`0x00`, longueur `0x24`) de la clé encodée en protobuf libp2p (`0x08 0x01 0x12 0x20 ||
  clé`). Il fait 52 caractères et commence par `12D3KooW` (`crypto.json#peer-id-A`).
- Un PeerID **DOIT** être canonique (le ré-encodage de la clé extraite redonne la même
  chaîne), de 1 à 64 octets ASCII imprimables sans espace, et sa clé **DOIT** être valide
  (§3.2).

### 3.2 Validité d'une clé publique

Une clé publique Ed25519 de 32 octets est **valide** si et seulement si :

1. **elle se décode en un point** selon [RFC 8032 §5.1.3] : `y < p` ; `x² = (y² − 1) /
   (d·y² + 1)` a une racine carrée ; si `x = 0`, le bit de signe vaut 0. Un filtre par plage
   et par liste noire ne suffit pas : `02 00…00` (y = 2) a `y < p`, n'est sur aucune liste, et
   n'est pas un point (`crypto.json#off-curve-public-key`) ;
2. son encodage est **canonique** : le ré-encodage du point redonne les mêmes 32 octets
   (`#noncanonical-public-keys`) ;
3. le point est dans le **sous-groupe d'ordre premier** et n'est pas le neutre : `[L]A = O` et
   `A ≠ O`. Cela exclut les huit points d'ordre petit (`#small-order-keys`) et les points
   d'ordre mixte (`#speccheck-2` … `#speccheck-9`). Une clé honnête `A = [s]B` y est toujours.
   Calcul : `[L−1]A + A = O`.

Toute clé qui entre dans le protocole **DOIT** être validée ainsi, au plus tôt, **à chaque
point d'entrée** : filtre de connexion (clé du PeerID distant, avant la fin de la poignée de
main libp2p), décodage d'un PeerID, `author` / `subject` d'une op de membre, `signer` d'une
attestation, `author` d'une décision, d'un accusé ou d'une annonce, `key` d'un tuple
d'auteur ou d'une preuve, hôte d'un ticket, **et au rechargement** de tout objet stocké
(liste, journal, annonces, configuration, import d'un projet). Une clé invalide est refusée
comme une entrée mal formée ; un objet stocké qui en contient une est mis en quarantaine
(§14).

*Raison.* Avec la clé neutre et la signature `R = O, S = 0`, l'équation sans cofacteur est
vraie pour tout message (`#ed25519-small-order-key`) : `crypto/ed25519` de Go et go-libp2p
v0.51.0 l'acceptent, donc quiconque peut s'authentifier sous ce PeerID et signer en son nom
(constat N-1, [Chalkias 2020]). La règle 3, plus stricte que « pas d'ordre petit », rend
le verdict indépendant du choix cofacteur / sans cofacteur pour la clé.

### 3.3 Signatures

- Toute signature est Ed25519 pur sur `"inkwell/" || domaine || "/v1" || 0x00 || message`,
  `domaine` pris au registre §20.4 (`crypto.json#sig-domain-1`). Une signature d'un domaine
  ne vérifie jamais pour un autre.
- **Verdict exact** (un désaccord sur une signature est un désaccord sur la liste) : une
  signature `(R, S)` de `M` par `A` est acceptée si et seulement si (a) `A` est valide
  (§3.2) ; (b) `S < L` (`#ed25519-noncanonical-s`) ; (c) avec `k = SHA-512(R || A || M) mod
  L`, l'encodage du point `[S]B − [k]A` est **octet pour octet** égal à `R` (équation sans
  cofacteur de [RFC 8032 §5.1.7], comparaison des encodages). `R` n'est pas validé
  autrement. C'est `crypto/ed25519.Verify` de Go précédé de (a). Les douze cas de
  [Chalkias 2020] sont dans `crypto.json#speccheck-0` … `#speccheck-11` avec leur verdict ;
  une implémentation **DOIT** les reproduire tous.

### 3.4 Empreintes

| Usage | Fonction | Entrée |
|---|---|---|
| op de membre, décision, accusé, annonce, preuve, identifiant de projet | SHA-256 | encodage complet, signature comprise |
| têtes et modifications de document | empreinte de modification Automerge (SHA-256, 32 octets) | format Automerge |
| identifiant de document dérivé | `BLAKE3("inkwell/doc/v1" ‖ …)[:16]` | §4.6 |
| blob | BLAKE3-256, arbre Bao à groupes de 16 Kio | contenu |

Le `ProjectID` est le SHA-256 de l'encodage de l'op de genèse (§4.1). Il ne change jamais.

## 4. Objets signés

### 4.1 Op de membre, version 1 (*existant*, figé)

```
op_v1 {
  version   u8      = 1
  kind      u8      1 Genesis | 2 Add | 3 SetRole | 4 Remove | 5 Suspend | 6 Resume
  role      u8      0 aucun | 1 lecture | 2 écriture | 3 admin
  lamport   u64     0 pour Genesis ; sinon exactement 1 + max(lamport des parents)
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

- Domaine `trust-op`, message `ProjectID || op sans sig` ; la genèse est signée avec un
  `ProjectID` de 32 zéros (`trust.json#op-genesis`, `#op-add-writer` donne les octets signés).
- Contraintes par `kind` (refus sinon) : **Genesis** : `author = subject`, `role = 3`,
  `person = 0`, `nprev = 0`, `lamport = 0`. **Add** : rôle 1–3. **SetRole** : rôle 1–3,
  `person = 0`. **Remove** : `role = 0`, `person = 0`, `name` vide. **Suspend**, **Resume** :
  `author ≠ subject`, `role = 0`, `person = 0`, `name` vide. Toute op autre que la genèse :
  `root_url` vide, `nprev ≥ 1`, `lamport ≥ 1`.
- **Stockage** (vérifié sur l'op et ses ancêtres seuls, donc même verdict partout) : parents
  connus (sinon attente, §6.1), Lamport exact, signature, et l'auteur a reçu le rôle admin
  dans le passé causal de l'op (créateur, ou sujet d'un Add/SetRole admin parmi ses
  ancêtres, effectif ou non). Une liste de 1024 ops est **scellée** : elle ne stocke plus
  rien (§6.1).

### 4.2 Op de membre, version 2

Identique à la version 1 avec `version = 2` et, après `prev` :
`nseen u8 ≤ 64 || seen [32] × nseen || nseen_listing u8 ≤ 64 || seen_listing [32] × …`,
chaque liste strictement croissante, au moins une non vide. `seen` porte la frontière du
journal de décisions vue par l'auteur ([ADR 0030] D1) ; `seen_listing` les bouts de chaînes
d'annonces vus ([ADR 0031] §2, `SeenListing`). Une op qui coupe, suspend ou enregistre un
départ **DOIT** être émise en version 2 ; les autres **DEVRAIENT** rester en version 1.

### 4.3 Attestation, version 1 (*existant*, figé)

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

Le champ `doc` admet 1 à 256 octets (format figé), mais l'attestation n'est valable que pour un
document de l'ensemble admis, dont les identifiants ont au plus 64 caractères (§9.2.3) : en
pratique `doc` fait au plus 64 octets. **Valable** si `project` est celui de la session, la signature vérifie (§3.3), `signer` est
dans la liste courante un appareil de rôle écriture ou admin non suspendu, et `doc` est dans
l'ensemble admis (§4.10). Elle n'autorise que la couverture du §9.4. Plusieurs attestations
d'un même (document, signataire) peuvent compter à la fois : celles que le magasin garde
(§6.2) ; chacune couvre ses têtes et leurs ancêtres.

### 4.4 Décision d'admin, version 1

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

Journal : 8192 décisions, 1024 par auteur. Sémantique : [ADR 0030] (debout au passé de
`members`, `≺`-dernière en cas de conflit). Vecteur : `policy.json#policy-mode-approval`.

### 4.5 Accusé final, version 1 (*proposé*, ADR 0031 M8)

```
final_ack_v1 {
  version     u8    = 1
  author      [32]  clé valide d'un appareil admin
  nmembers    u8    1–64 ;  members   [32] × nmembers, strictement croissants
  ndecisions  u8    1–64 ;  decisions [32] × ndecisions, strictement croissants
  nentries    u8    1–96 ;  entries (doc [16] ; gen u32 ; purge [32]) × nentries,
                            strictement croissantes par (doc, gen)
  sig         [64]  domaine « final-ack », message = ProjectID || tout ce qui précède sig
}                    taille totale ≤ 8192 octets
```

`purge` est l'empreinte de la décision `doc/<id> = purged` accusée. Finalité : [ADR 0031] §2
(« Purge et restauration »), prédicat pur et monotone sur les accusés détenus.

### 4.6 Annonce de document, version 1 (*proposé*, ADR 0031 M3)

```
listing_op_v1 {
  version     u8    = 1
  sort        u8    1 variant | 2 adopt   (3 node : réservé à l'index de l'ADR 0023)
  author      [32]  clé valide
  position    u32   ≥ 1 : rang sur la chaîne de l'auteur (variantes et adoptions)
  slot        u16   variant : 1 ≤ slot ≤ Q (§4.10) ; adopt : slot de l'op adoptée
  nprev       u8    0 si position = 1, 1 sinon ;  prev [32] × nprev (annonce précédente de l'auteur)
  nmembers    u8    1–64 ;  members   [32] × nmembers   (frontière de la liste vue)
  ndecisions  u8    0–64 ;  decisions [32] × ndecisions (frontière du journal vue)
  anchor      [32]  variant : empreinte de la modification d'ancrage ; adopt : empreinte de l'op adoptée
  sig         [64]  domaine « listing-op », message = ProjectID || tout ce qui précède sig
}                    taille totale ≤ 4096 octets
```

`DocId = BLAKE3("inkwell/doc/v1" ‖ ProjectID ‖ author ‖ sort ‖ slot ‖ anchor)[:16]` pour une
variante (`slot` en `u16`) ; une adoption désigne le `DocId` de l'op adoptée. Le
`documentId` automerge-repo est le bs58check de ces 16 octets. Il est **toujours
recalculé**, jamais reçu. Règles d'admission L1–L5, équivoque, adoption : [ADR 0031] §2,
résumées au §4.10.

### 4.7 Signature d'auteur d'une modification (*proposé*, ADR 0031 M1, G12)

Chaque modification Automerge émise porte un tuple signé par l'appareil qui l'a écrite :

```
changesig = Ed25519(key, "inkwell/change/v1" || 0x00 || ProjectID || key [32] || epoch [16]
                         || seq u64 || hash [32])
actor     = BLAKE3("inkwell/actor/v2" || ProjectID || key || epoch)[:16]
```

`epoch` est un aléa de 16 octets tiré et persisté par l'appareil, un par génération de
poignée Automerge, jamais réutilisé. `seq` et `hash` sont ceux de la modification. Sur le
fil, les tuples voyagent groupés par (clé, epoch) dans la trame AUTHOR (§5.2), **avant** le
message REPO ou LARGE qui contient les modifications, sur le même flux.

```
author_batch { key [32] ; epoch [16] ; n u16 ≥ 1 ; n × ( seq u64 ; hash [32] ; sig [64] ) }
```

Le receveur vérifie chaque signature (§3.3) ; la garde vérifie que l'acteur de la
modification est `actor(key, epoch)`, que `seq` et `hash` correspondent (G12, §9.7).
L'autorisation reste la couverture (§9.4) : les deux sont exigées.

### 4.8 Preuve d'équivoque, version 1 (*proposé*, ADR 0031 M9)

```
evidence_v1 { version u8 = 1 ; key [32] ; epoch [16] ; seq u64 ;
              hash_a [32] ; sig_a [64] ; hash_b [32] ; sig_b [64] }     249 octets
```

`hash_a < hash_b` (ordre des octets), et les deux tuples vérifient (§4.7). Elle se vérifie
sans Automerge. Gardée : la première par (key, epoch), au plus 4 par `key`.

### 4.9 Pli de la liste des membres (normatif)

La liste est une **fonction de l'ensemble des ops stockées**, indépendante de l'ordre
d'arrivée ; deux implémentations qui détiennent le même ensemble **DOIVENT** obtenir la même
liste. Ce texte reprend `internal/trust/doc.go` (référence exécutable, testée dans tous les
ordres) et les [ADR 0005], [ADR 0017], [ADR 0021].

Notations. `≺` ordonne les ops par (lamport, author, empreinte) et prolonge la causalité ;
`pos(o)` est l'indice de `o` dans `≺`. « o voit p » : p est un ancêtre strict de o. Deux ops
sont concurrentes si aucune ne voit l'autre. Une **op de rôle** sur x est un Add, SetRole ou
Remove de sujet x, ou la genèse pour le créateur. C'est un **octroi** si elle donne admin, une
**coupe** si c'est un Remove ou un rôle inférieur. Niveaux : Remove < lecture < écriture <
admin.

1. **Ancienneté.** Le créateur a le rang 0. Le rang d'une autre clé est `pos(g)` du premier
   (selon `≺`) octroi debout qui survit (règle 4) sur elle ; sans octroi, la clé n'a pas de
   rang. Plus petit = plus ancien. Les rangs sont distincts et gardés après rétrogradation.
2. **Portée.** Une op de rôle de y sur une autre clé x ne compte que si x n'a pas de rang ou
   est plus récent que y. Exception : un Remove, ou un SetRole/Add vers un rôle inférieur
   sans `name` ni `person`, sur un plus ancien qui **n'est pas admin** à cette op (règle 3(a)
   évaluée pour lui). Un octroi ou un renommage ne compte jamais sur un plus ancien. Une clé
   peut agir sur elle-même, mais une coupe de soi ne compte que s'il existe un autre admin
   dans la liste calculée sur les seuls ancêtres de cette op : le dernier admin ne peut pas
   partir.
3. **Debout.** Une op o de y est debout si et seulement si : (a) y est admin à o (parmi les
   ops de rôle sur y que o voit et qui survivent, la dernière selon `≺` est un octroi ; la
   genèse est debout par elle-même) ; (b) aucune coupe debout de y par une autre clé n'est
   concurrente à o, et aucune **sortie** de y n'est concurrente à o, sauf si o est elle-même
   une sortie ou est épargnée par elle ; (c) la règle 2 l'admet. Une **sortie** est une coupe
   de y par y alors que y est admin avec un autre admin dans la liste de ses ancêtres, sans
   coupe debout de y par une autre clé qui lui soit concurrente. Une sortie debout termine la
   carrière de y ; elle épargne une op o de y concurrente quand une op d'une autre clé, dans
   l'ensemble plié, voit o sans voir la sortie. Une op qui n'est pas debout n'a aucun effet.
4. **Survie.** Une op de rôle debout sur x par une autre clé est écartée si une op de rôle
   debout concurrente sur x par une autre clé donne un niveau inférieur (la révocation bat un
   octroi concurrent). Les ops d'une clé sur elle-même survivent toujours et n'écartent rien ;
   les coupes qu'autorise l'exception de la règle 2 n'écartent rien et sont écartées par un
   octroi concurrent qui survit.
5. **Résultat.** Le rôle de x est celui de la dernière (selon `≺`) op de rôle qui survit sur
   x ; si c'est un Remove, ou s'il n'y en a pas, x n'est pas membre. Add et SetRole fixent le
   nom quand il n'est pas vide (le nom de la genèse est le titre du projet) ; un Remove
   oublie les noms antérieurs. `person` vient du dernier Add qui survit après le dernier
   Remove qui survit (son empreinte si `person` est nul), sinon de l'op qui a admis x. Ordre
   d'affichage : `≺` de cet Add, le créateur en premier.
6. **Suspension** (Suspend/Resume, par un admin, sur une autre clé ; pas une op de rôle).
   S1 portée : comme la règle 2 (exception comprise). S2 état : on garde les ops de
   suspension sur x qui voient chaque Remove qui survit de x par un plus ancien ; celle d'un
   plus récent sur un plus ancien est écartée par un octroi qui survit et qu'elle ne suit
   pas ; un Resume concurrent à un Suspend gardé est écarté ; x est suspendu si la dernière
   gardée est un Suspend. S3 effet : en plus de la règle 3, une op o de y n'est debout que si
   y n'est pas suspendu à o (S2 sur ce que o voit) et si aucun Suspend debout de y par une
   autre clé n'est concurrent à o.

Chaque règle sur les ops de y ne lit que les ops des clés plus anciennes que y et les
siennes : un seul balayage par rang décide de tout, en O(n²/64 + n·e). Une liste scellée
(1024 ops) dont deux répliques détiennent des ensembles différents (seulement si un ex-admin
les a inondées) **DOIT** le dire : « liste de membres saturée — créez un nouveau projet ».

### 4.10 Ensemble admis de documents (normatif)

L'**ensemble admis** est la seule liste des `documentId` qu'un pair accepte, partage, charge
et pour lesquels il garde des attestations. C'est une fonction pure des ensembles signés
(§6.1) :

```
admis = racine ∪ G ∪ annonces debout sous le point de gel ∪ documents adoptés
        ∪ { d : doc/d = kept et d issu d'une annonce détenue }
        ∪ { d : legacy/d }  ∪ documents de récupération des décisions equiv/
        \ { d : doc/d = purged }
```

- **racine** : document de `root_url` de la genèse ;
- **G**, l'index, calculé par chaque pair sans réseau, octet pour octet ([ADR 0030] §3) : une
  modification d'acteur `a0 = BLAKE3("inkwell/index-actor/v1" ‖ ProjectID)[:16]`, `seq 1`,
  `startOp 1`, `time 0`, `deps []`, qui pose à la racine `schema = 1`, `nodes = {}`,
  `packages = {}` et le nœud principal `main.typ` pointant vers `root_url`, d'identifiant
  `BLAKE3("inkwell/main-node/v1" ‖ root_url)[:16]` ; son `documentId` est
  `BLAKE3("inkwell/index/v1" ‖ empreinte(G))[:16]` (bs58check). Un vecteur doré figera
  l'empreinte de G (étape 8) ;
- **Annonce debout** (ADR 0031 L1–L5) : canonique et signée (L1) ; passé (`prev`,
  `members`, `decisions`) détenu (L2, sinon attente bornée par auteur, §16) ; `position`
  exacte et `slot` jamais répété sur la chaîne de l'auteur (L3) ; l'auteur écrit et n'est
  pas suspendu dans le pli de `members` (L4) ; nombre de variantes ≤ `Q = 32 + 32 × octrois`,
  où `octrois` compte les décisions `quota/<clé de l'auteur>` debout dans le passé de
  `decisions` (L5) ; aucune coupe de l'auteur ne lui est concurrente ; sous le **point de
  gel** : deux annonces d'un auteur à la même position gèlent sa chaîne à partir de là.
- **Adoption** : une op `adopt` vérifie L1, L4 et L5 de l'op adoptée à son émission ; une
  par (adoptant, auteur étranger, slot), jamais imbriquée.
- Un document d'un autre projet n'est jamais admis. Le pli est recalculé à chaque
  changement des ops de membres, de décisions ou d'annonces.

Le détail (purge, finalité, arbitrage `equiv/`) est dans [ADR 0031] §2 ; ce paragraphe en
est l'énoncé normatif minimal pour décider l'admission d'un `documentId`.

## 5. Session : `/inkwell/{control,repo,presence,large}/2.0.0`

### 5.1 Session, flux et préambule

- Une **session** est une connexion libp2p authentifiée entre deux membres. Un passage du
  relais au direct ouvre une **nouvelle** connexion, donc une nouvelle session ; l'ancienne
  se ferme (GOAWAY NO_ERROR) quand la nouvelle est Active. Il n'y a ni remplacement de flux
  ni ordre à préserver entre deux connexions.
- Dans une session, chaque pair ouvre vers l'autre des **flux unidirectionnels**, un par
  classe ; **la classe est l'identifiant de protocole libp2p** négocié par multistream-select
  (aucun octet de classe à nous) ; il n'écrit jamais sur un flux reçu et ne lit jamais un flux
  qu'il a ouvert (un octet reçu sur un flux ouvert est une violation).

| Classe | Identifiant de protocole | Préambule (premiers octets du flux) |
|---|---|---|
| CONTROL | `/inkwell/control/2.0.0` | aucun |
| REPO | `/inkwell/repo/2.0.0` | `gen u32` (génération ≥ 1, §7.3) |
| PRESENCE | `/inkwell/presence/2.0.0` | aucun |
| LARGE | `/inkwell/large/2.0.0` | `gen u32 ; id u32 ≥ 1 ; size u32` (65 537 ≤ size ≤ 4 194 304 = `r`, §10.6) |

- Tous les flux d'une session ont la majeure du flux CONTROL ; un flux d'une autre majeure est
  une violation. Les limites de nombre de flux par pair et par protocole sont celles du
  gestionnaire de ressources de libp2p (portées *protocol* et *peer*), réglé selon §16.

- **Au plus deux sessions par pair** à la fois, pendant un passage. **Départage
  déterministe** (les deux côtés calculent le même résultat, sans message de plus) : chaque
  HELLO porte `SESSION_NONCE` (`u64` aléatoire, §5.3) ; la **clé** d'une session est le couple
  (nonce envoyé par le pair de plus petit PeerID, nonce de l'autre). Entre deux sessions vers
  le même pair, on garde (1) une connexion directe plutôt qu'une connexion relayée (tous deux
  voient `/p2p-circuit` dans l'adresse), puis (2) la plus grande clé, puis (3) à clés égales
  (nonce réutilisé par un pair byzantin), la session Active la plus ancienne. Dès que la gardée est
  Active, l'autre ferme ses flux REPO, LARGE et PRESENCE puis la connexion (GOAWAY NO_ERROR).
  Deux connexions directes simultanées (DCUtR) se résolvent donc en une seule, la même des deux
  côtés. Les limites du gestionnaire de ressources sont réglées à **deux fois** celles d'une
  session (portées *peer* × *protocol*) ; la règle « un flux par classe » s'applique **par
  session**. Chaque session a aussi un identifiant local `session` (`u32`, croissant pour la
  vie du processus) qui accompagne la génération partout côté local (règle T, Annexe A).
- **Fenêtre de connexion** ([RFC 9000 §4.1]) : sur QUIC, tous les flux partagent le crédit de
  la connexion ; un flux non lu en retient jusqu'à sa fenêtre. Valeurs effectives avec
  go-libp2p v0.51.0 et quic-go v0.64.1 : fenêtres **initiales** 512 Kio (flux) et 768 Kio
  (connexion), qui grandissent avec le débit lu jusqu'aux **maximums** 10 Mio et 15 Mio ; quic-go
  garde toujours la fenêtre de connexion ≥ 1,5 × celle de tout flux. Règles :
  (a) CONTROL et PRESENCE sont **toujours lus** ;
  (b) **un seul flux de données peut rester non lu** : le flux REPO courant ; un flux LARGE
  n'est ouvert qu'après LARGE_REPLY (§7.2) et se lit aussitôt ;
  (c) au changement de génération, l'ancien flux REPO est **annulé** (STOP_SENDING) ; un flux
  LARGE entrant de l'ancienne génération n'est **pas** annulé : il est lu jusqu'au bout (il est
  déjà dans une réservation) et ses octets sont jetés, il ne retient donc jamais de crédit. Le
  crédit du REPO annulé ne revient **pas** aussitôt : il revient
  quand la taille finale est connue, c'est-à-dire à l'arrivée du RESET_STREAM (ou du FIN) du
  pair, qu'un pair honnête **DOIT** envoyer ([RFC 9000 §3.5]) — au moins un aller-retour
  ([RFC 9000 §4.5]). D'où : **au plus un flux annulé en attente** par session (un nouveau
  changement de génération attend sa fin ; la cadence est de toute façon ≥ 2 s) ; et si le
  RESET_STREAM n'arrive pas dans les **10 s**, le pair viole RFC 9000 : GOAWAY
  PROTOCOL_VIOLATION et fermeture de **cette** session ;
  (d) **la garantie** : hors de cet intervalle, un seul flux de données retient du crédit et
  le contrôle en trouve toujours (connexion ≥ 1,5 × flux) ; pendant l'intervalle entre une
  annulation et le RESET_STREAM du pair, l'ancien flux et le nouveau (s'il est déjà lu puis mis
  en pause) peuvent ensemble prendre tout le crédit, et le contrôle peut alors attendre
  **jusqu'à ce RESET_STREAM** : au plus un aller-retour avec un pair honnête, au plus 10 s
  avant la fermeture de la session d'un pair qui ne répond pas. Cela ne touche jamais une
  autre session. Une implémentation **DOIT** avoir `fenêtre maximale de connexion ≥
  fenêtre maximale de flux + fenêtre initiale de flux + 1 Mio` (vérifié au démarrage, porte
  P-QUIC). yamux n'a pas de fenêtre de connexion. Modèle : [tla/ConnCredit.tla](tla/ConnCredit.tla).
- Flux par pair, par session et par sens : **1 CONTROL**, **1 REPO** (plus, pendant au plus un tour de
  génération, l'ancien en cours de fermeture), **1 PRESENCE**, **au plus 1 LARGE non lu**
  (§7.2). Un flux de plus ou un `gen` hors règle est une violation, sauf le cas « ancien » du
  §7.3 (flux réinitialisé sans sanction) ; un identifiant inconnu est refusé par
  multistream-select (`na`).
- CONTROL, REPO et PRESENCE portent ensuite une suite de **trames** ; LARGE porte exactement
  `size` octets bruts puis la fin du flux.
- Un pair qui n'est pas membre mais que la fenêtre d'invitation laisse se connecter ne peut
  ouvrir que `/inkwell/lobby/1.0.0` ; tout autre flux de sa part est réinitialisé.

*Pourquoi un flux par classe.* Le transport donne déjà un crédit **par flux** ([RFC 9000
§4.1], fenêtre de yamux) : un receveur qui cesse de lire le flux REPO (sa webview est
lente) ne bloque ni le contrôle, ni la présence, ni les autres pairs, et le contrôle n'est
jamais derrière des données (même règle que [RFC 9113 §5.2.1], qui exempte du contrôle de
flux les trames autres que DATA). Réinitialiser un flux jette atomiquement tout ce qu'il
contenait encore ([RFC 9000 §3.5]) : c'est le mécanisme de changement de génération (§7.3).

### 5.2 Trames

```
trame { length u32 (1 ≤ length ≤ 98 304, compte type + corps) ; type u8 ; corps [length − 1] }
```

Le récepteur **DOIT** comparer `length` à la borne avant d'allouer
(`session.json#frame-too-large`). Deux plages de types (§1.4.2) : **critique** `0x01`–`0x3F`
(un type inconnu, réservé à une capacité non négociée, retiré, ou reçu sur un flux d'une autre
classe est une violation : `#frame-wrong-stream`, `#retired-type`) et **ignorable**
`0x40`–`0x7F` (un type inconnu est sauté et compté ; GREASE `0x4A`, `0x5A`, `0x6A`, `0x7A`).

| Type | Nom | Flux | Corps | Taille du corps |
|---|---|---|---|---|
| 0x01 | HELLO | CONTROL (la première, et une seule) | liste TLV (§5.3) | ≤ 1024 |
| 0x02 | GOAWAY | CONTROL | `code u8` (§20.3) | 1 |
| 0x03 | SET | CONTROL | anti-entropie d'un ensemble signé (§6.1) | ≤ 98 303 |
| 0x04 | ATTEST | CONTROL (commérage, réponses) ou REPO (avant ce qu'elle couvre) | `attestation_v1` (§4.3) | ≤ 4096 |
| 0x05 | REPO | REPO | message de la couche dépôt, CBOR (§9) | 1 à 65 536 |
| 0x06 | PRESENCE | PRESENCE | message `ephemeral`, CBOR (§9.5) | 1 à 2048 |
| 0x07 | LARGE | REPO | `id u32 ; size u32 ; flags u8 (bit 0 : indivisible ; autres bits 0) ; root [32] ; change [32] ; doc str8 (1–64)` : annonce un objet à cette place (§7.2) | 75 à 138 |
| 0x08 | — | — | retiré (ancien REPO_RESET), jamais réattribué | — |
| 0x09 | AUTHOR | REPO (avant ce qu'elle couvre) | `author_batch` (§4.7) | ≤ 98 303 |
| 0x0A | ATTEST_WANT | CONTROL | `doc str16` (1–64) : « envoie-moi tes attestations de ce document » (§6.2) | ≤ 66 |
| 0x0B | LARGE_REPLY | CONTROL | `gen u32 ; id u32 ; verdict u8` : 1 = « j'ai réservé la place, ouvre le flux LARGE `id` de la génération `gen` », 0 = « ne l'ouvre pas » (refus, ou modification indivisible que je tire par le vrac) (§7.2) | 9 |

### 5.3 HELLO

- Le flux CONTROL commence par une trame HELLO, et par une seule. Avant d'avoir lu le HELLO
  du pair, un récepteur n'accepte aucun autre flux de ce pair (ils attendent, non lus).
- Corps : une suite `id u16 || len u16 || valeur [len]`, identifiants **strictement
  croissants** (`session.json#hello-unsorted-tlv`). Un identifiant inconnu **pair** est ignoré
  (`#hello-unknown-tlv`, point d'extension du HELLO) ; un identifiant inconnu **impair** est
  « à comprendre » : GOAWAY UNSUPPORTED_CRITICAL (§1.4.2).

| id | Nom | Valeur | Règle |
|---|---|---|---|
| 0x0001 | PROJECT | `[32]` ProjectID | OBLIGATOIRE ; différent du nôtre → GOAWAY PROJECT_MISMATCH |
| 0x0002 | CAPS | `u16` × k, strictement croissants, k ≥ 1 | OBLIGATOIRE ; registre §20.2 |
| 0x0004 | VERSIONS | `u16` × k, majeures parlées, strictement croissantes | OBLIGATOIRE ; liaison de la négociation (§1.4.3) |
| 0x0008 | SESSION_NONCE | `u64` aléatoire, tiré à chaque connexion | OBLIGATOIRE ; départage des sessions (§5.1) |
| 0x?A?A | GREASE | quelconque | ignoré |

- Le HELLO **DOIT** arriver dans les 10 s suivant l'ouverture de la connexion ; sinon
  GOAWAY HELLO_TIMEOUT.

### 5.4 Capacités

- L'**ensemble effectif** est l'intersection des CAPS des deux HELLO. **`core/2` DOIT y
  figurer, sinon GOAWAY NO_COMMON_CAPABILITY** (`#hello-without-core`).
- `core/2` désigne **tout** ce que décrit cette spécification, y compris ce qui porte la
  sécurité byzantine : ops de membre v2, décisions, accusés, annonces, preuves, AUTHOR,
  pages d'historique, garde structurelle. **Une garantie de sécurité ne se négocie pas** :
  un pair qui n'en implémente qu'une partie ne peut pas annoncer `core/2`, et un récepteur
  ne désactive jamais une vérification parce que le pair « ne sait pas ». Cela ferme toute
  rétrogradation par un pair (un tiers ne peut de toute façon pas toucher au HELLO, qui
  passe dans une session authentifiée).
- Une capacité future (mineure, §1.4) ne peut ajouter que des fonctions **sans effet sur la
  sécurité** de ce qui existe ; un décodeur reçoit l'ensemble effectif en paramètre et c'est
  le seul endroit où une trame ou une version nouvelle devient acceptable.
- Un émetteur **DEVRAIT** ajouter à CAPS, et en TLV, une valeur GREASE tirée dans
  `{0x0A0A, 0x1A1A, …, 0xFAFA}` ([RFC 8701]) ; un récepteur **DOIT** l'ignorer.

### 5.5 Classes et fiabilité

| Classe | Flux, trames | Sémantique | Perte |
|---|---|---|---|
| contrôle | CONTROL : HELLO, GOAWAY | session | perte du flux = fin de session |
| état | CONTROL : SET, ATTEST, ATTEST_WANT | anti-entropie ; dernier gagne | **permise** : réparée par l'anti-entropie (§6.1) ou la réconciliation (§6.2) |
| dépôt | REPO : REPO, LARGE, AUTHOR, ATTEST ; flux LARGE | fiable, ordonnée, par génération | **interdite** : règle L (transport), règle H (receveur) |
| présence | PRESENCE | dernier gagne par `sessionId` | permise |

**Règle L (aucune perte silencieuse dans la classe dépôt).** Une perte **en transport** — flux
REPO ou LARGE réinitialisé, flux LARGE sans progrès — ou une demande de la webview **DOIT**
faire changer de génération (§7.3). La raison est l'état de synchronisation d'Automerge : un
message « envoyé » ne sera jamais renvoyé (`sentHashes`), donc une perte non dite est une
divergence durable.

**Règle H (ce que le receveur ne garde pas, il le redemande).** Un message que le receveur a
bien reçu mais ne remet pas au `Repo` — refusé (admission, garde), non couvert ou sans tuple
d'auteur à l'expiration de son attente, document pas encore prêt et file d'attente pleine —
est un **refus** noté par (pair, document), **jamais** un changement de génération. Quand la
cause peut avoir disparu (nouvelle attestation ou nouveau tuple reçu, document prêt, file
vidée), et au plus une fois par (pair, document) et par 60 s, le receveur envoie
`history-request` (§9.6) avec ses têtes, d'abord au pair qui avait livré le message, puis, s'il
ne répond pas ou si la cause demeure, à un autre membre actif qui détient le document (à tour
de rôle) : celui-ci lui renvoie, en pages, **toute** modification hors du passé causal de ses
têtes, branches concurrentes comprises.
Ainsi un écrivain disparu ou byzantin qui atteste une tête fantôme coûte au plus une demande
par minute, sans boucle de générations entre pairs honnêtes ; et l'émetteur ne fait jamais de
règle L pour une file **locale** (§16.2).

### 5.6 Ordre

1. Les trames d'un flux sont traitées dans l'ordre d'émission ; aucun ordre n'existe entre
   deux flux.
2. Sur le flux REPO, ATTEST et AUTHOR précèdent le message qu'ils couvrent : le receveur a
   toujours la preuve avant les données. Le receveur traite une trame entièrement (vérifiée,
   enregistrée) avant la suivante du même flux.
3. Les messages REPO sont livrés dans l'ordre ; les messages LARGE aussi, entre eux. Un
   REPO lu **après** une annonce LARGE **PEUT** être livré avant le message LARGE (un gros
   transfert ne retarde pas la frappe) ; un récepteur **DOIT** le tolérer, ce qu'Automerge
   fait.

## 6. Classes d'état

### 6.1 SET : réplication des ensembles signés

Un seul mécanisme réplique les cinq ensembles d'objets signés, chacun **adressé par son
contenu** (SHA-256 de l'encodage) et **idempotent** :

| set | Ensemble | Objet | Structure | Bornes (§16) |
|---|---|---|---|---|
| 1 | membres | `op_v1`, `op_v2` (§4.1–§4.2) | graphe (`prev`) | 1024 ops, scellé |
| 2 | décisions | `policy_op_v1` (§4.4) | graphe (`prev`) | 8192, 1024 par auteur |
| 3 | accusés | `final_ack_v1` (§4.5) | plat | admins × entrées |
| 4 | annonces | `listing_op_v1` (§4.6) | graphe (chaîne par auteur) | Q* + 2 par auteur |
| 5 | preuves | `evidence_v1` (§4.8) | plat | 4 par clé |

```
set_msg  { set u8 ; kind u8 ; corps }
  kind 1 have    { n u16 ≤ 256 ; hash [32] × n }             frontière (sets à graphe)
  kind 2 objects { n u16 ≥ 1 ; n × ( len u16 ; objet [len] ) }
  kind 3 digest  { count u32 ; digest [32] }                  sets plats : SHA-256 des empreintes triées concaténées
```

- **Annonce.** À l'activation, puis toutes les 15 s, chaque pair envoie pour chaque set un
  `have` (sa frontière ; si elle dépasse 256 têtes, les 256 plus petites) ou un `digest`.
  Une liste scellée continue d'annoncer et de répondre (le scellement arrête seulement le
  stockage de nouvelles ops).
- **Réponse.** Un `have` reçu est répondu par les objets que le receveur détient hors du passé
  causal de cette frontière (têtes inconnues ignorées) ; un `digest` différent du sien, par
  tous les objets du set. Jamais par un `have` : aucun écho.
- **Lots, ordre causal, continuation.** La réponse est un **curseur** gardé par le pair qui
  répond : les objets sont émis dans l'ordre `≺` (lamport, auteur, empreinte ; parents
  d'abord, donc applicables dans l'ordre d'arrivée), en trames `objects` d'au plus 98 303
  octets, **à mesure que la file de sortie CONTROL du pair a de la place** ; rien n'est
  matérialisé d'avance (état : un curseur par (pair, set)). Un nouveau `have`/`digest` du
  même pair pour le même set **ne recommence pas** un curseur actif : il met seulement à jour
  le filtre (les objets désormais dans le passé de la nouvelle frontière sont sautés) et le
  curseur **reprend où il en était**, jusqu'à la fin de l'ensemble ; un lent ne peut donc pas
  affamer la fin. Un nouveau curseur ne démarre qu'une fois le précédent terminé, au plus **2
  par (pair, set) et par 15 s** (aucune amplification). Le curseur s'arrête à épuisement, à la
  sortie d'Actif, ou si le flux CONTROL tombe.
- **Achèvement.** Sans perte, une réponse amène le demandeur à l'ensemble du répondeur en un
  tour (le curseur parcourt un ensemble fini, borné §16). Avec pertes, chaque tour applique au
  moins le premier lot (parents d'abord), la frontière annoncée avance, et le tour suivant
  (15 s) ne renvoie que le reste : l'ensemble converge en un nombre fini de tours, scellé ou
  non.
- **Relais.** Un objet nouvellement stocké est envoyé (`objects`) aux autres pairs actifs.
- **Verdicts.** Un objet mal formé (§4) est une violation. Un objet bien formé mais refusé
  (parent inconnu, auteur non admin, liste scellée) est ignoré sans sanction ; un objet en
  attente de ses parents est gardé au plus deux périodes, dans la part du pair qui l'a livré
  (§10.6).

### 6.2 ATTEST : attestations

- **Avant les données.** Un pair qui émet sur le flux REPO un message portant des têtes d'un
  document **DOIT** y faire précéder ce message de l'attestation de ces têtes s'il écrit, et
  des attestations qu'il détient pour les têtes qu'il relaie et que ce pair n'a pas encore
  reçues de lui dans cette génération.
- **Garde.** Par (document, signataire), le receveur garde un ensemble d'attestations
  valables, dans la part du signataire (§10.6) :
  *acceptation* — une attestation est gardée si elle apporte au moins une tête que l'ensemble
  n'a pas, sinon elle est ignorée ; *éviction* — tant que l'ensemble couvre plus de 256 têtes
  distinctes, on retire l'attestation de plus petit `seq` (les plus anciennes partent d'abord) ;
  *usage* — toutes celles qui restent comptent pour la couverture (§9.4). Toute tête encore
  dans l'ensemble de couverture d'un pair (même borne de 256) a donc son attestation, qu'un
  relais, même un appareil qui lit seulement, peut redonner. Une attestation nouvellement
  gardée est relayée aux autres pairs actifs sur CONTROL ; une ignorée ne l'est pas.
- **Signataire pas encore connu.** Une attestation bien signée dont le signataire n'est pas
  (encore) un écrivain de la liste est **gardée en attente**, comme un objet SET orphelin : au
  plus deux périodes, dans la part du pair qui l'a livrée, revérifiée à chaque changement de la
  liste ; elle n'est ni relayée ni comptée avant.
- **Activation.** À l'entrée en Actif, chaque pair envoie sur CONTROL la dernière attestation
  de chaque (document admis, signataire) qu'il garde.
- **Réconciliation (tirée par le receveur).** Quand la couverture (§9.4) attend une tête
  attestée pour un document, le receveur envoie `ATTEST_WANT(doc)` au pair qui a livré le
  message, puis toutes les 15 s tant que l'attente dure (au plus 30 s, puis règle H) ; au
  plus 64 documents demandés par pair et par 15 s. Le pair répond sur CONTROL par toutes les
  attestations qu'il garde pour `doc` ; il sert au plus 64 demandes par
  pair et par 15 s, au-delà il les ignore. Ainsi une attestation perdue revient en au plus
  15 s + un aller-retour, sans attendre une reconnexion, et sans trafic périodique quand rien
  ne manque.
- Une révocation ou une suspension rend caduques les attestations du signataire : le
  récepteur les oublie et la couverture cesse de les compter.

### 6.3 PRESENCE

Le flux PRESENCE porte des messages `ephemeral` (§9.5). Le Go ne les interprète pas. Il
est **toujours lu** : une file de 16 entrées par pair garde **la dernière** trame par
(pair, `sessionId`), la plus ancienne est jetée quand elle est pleine. Une PRESENCE ne
bloque rien et n'a pas de crédit sur le pont (Annexe A).

## 7. Classe dépôt

### 7.1 REPO

Un message de la couche dépôt de 1 à 65 536 octets voyage en ligne dans une trame REPO.
Jusqu'à `r` = 4 194 304 octets (la part fixe, §10.6), il voyage par un flux LARGE (§7.2).
Plus grand, il n'est qu'annoncé (avis) et le receveur redemande le document par pages d'au
plus `r` octets (§9.6) ; une modification qui ne tient pas seule dans une page est tirée par
le vrac (§8.1). Un avis ne franchit pas le pont : la webview envoie seulement `Advice` (Annexe A) ; le pont
plafonne un message à 67 108 797 octets (2^26 − 67).

### 7.2 Flux LARGE

1. L'émetteur écrit sur le flux REPO, à la place du message,
   `LARGE{id, size, flags, root, change, doc}` : `id` strictement croissant dans la
   génération, à partir de 1 ; `size` la taille de l'objet ; `root` = BLAKE3 (racine Bao) de ses
   octets ; `doc` son document. Le bit **indivisible** n'est posé que dans une réponse à
   `history-request` (§9.6) dont la page ne contiendrait qu'**une seule modification** et
   dont l'**encodage** (enveloppe CBOR comprise) dépasse `max` = `r` = 4 194 304 octets : l'objet est alors
   les octets bruts de la modification, et `change` son empreinte Automerge, couverte par le
   tuple d'auteur qui précède (§4.7) ; sinon `change` est nul (32 zéros,
   `#large-indivisible-no-change`). Une annonce indivisible de plus de 16 777 216 octets est une
   violation (la garde G1 interdit une telle modification, `#large-indivisible-too-big`) : un
   refus définitif, jamais redemandé.
   Une annonce **ordinaire** de plus de `r` octets n'est qu'un **avis** : `root` et `change`
   y sont nuls (32 zéros), l'émetteur l'annonce d'après la seule taille, sans faire passer les
   octets par le pont ni les garder (un message de synchronisation de 200 Mio n'a donc pas
   besoin de franchir le plafond de 64 Mio du pont, `#large-advice`) ; le receveur répond
   `verdict = 0` et redemande le document par pages (règle H). Après un avis, l'émetteur
   réinitialise l'état de synchronisation Automerge de ce (pair, document), comme pour un
   message remplacé (§7.4). Une annonce ordinaire ≤ `r`
   **DOIT** avoir un `root` non nul (`#large-ordinary-zero-root`). Seuls les objets ordinaires
   ≤ `r` passent par un flux LARGE.
2. Le receveur répond sur CONTROL par `LARGE_REPLY{gen, id, verdict}` (la génération lie la
   réponse : une réponse d'une génération passée ne correspond à rien et est ignorée) :
   - objet ordinaire de taille ≤ `r` (la part fixe, §10.6) : il est **toujours** admis ;
     `verdict = 1` ; l'émetteur ouvre alors seulement le flux `/inkwell/large/2.0.0` de
     préambule `(gen, id, size)`, y écrit les `size` octets et ferme son écriture ; le receveur
     le lit aussitôt dans sa réservation et vérifie `root`. Un flux LARGE ne reste donc jamais
     non lu (§5.1) ;
   - objet ordinaire plus grand que `r` (avis) : `verdict = 0` ; le receveur redemande le
     document par pages d'au plus `max = r` octets (§9.6) ;
   - objet **indivisible** : `verdict = 0`, et le receveur le **tire par le vrac** (§8.1), par
     `root`, chez l'annonceur ou chez tout autre membre qui le détient, en tranches Bao
     vérifiées, reprenables, mises en attente sur disque (§8.1, « Modifications
     indivisibles »).
   Sur `verdict = 0`, l'émetteur oublie le flux LARGE ; une modification indivisible reste
   servie par son index persistant (§8.1, « qui sert »). Un objet annoncé sans
   réponse depuis **10 min** est oublié, et l'état de synchronisation de ce (pair, document)
   réinitialisé (§7.4) ; avec des parts fixes, un receveur honnête répond toujours aussitôt.
   Un flux LARGE non demandé, ou dont le préambule ne correspond pas à la demande, est une
   violation (`#large-unannounced`).
3. Le message est livré quand exactement `size` octets ont été lus, que le flux est fini et que
   BLAKE3 donne `root` ; sinon violation.
4. Un émetteur a **au plus un flux LARGE non terminé** par pair ; l'annonce suivante attend.
5. Échec (flux réinitialisé par l'un ou l'autre, connexion perdue, **aucun progrès pendant
   60 s après `LARGE_REPLY`**) d'un flux LARGE **de l'époque courante** : le message est perdu,
   règle L. La fin, la réinitialisation ou l'échec d'un flux d'une époque **passée** n'est pas
   une perte (règle T : on l'ignore) ; en particulier, le pair qui applique la règle I
   réinitialise son LARGE sortant, et notre LARGE entrant de l'ancienne génération finit en
   RESET_STREAM sans rien déclencher. Le délai de 60 s sans progrès s'applique **aussi** à un
   LARGE d'une époque passée (il est alors réinitialisé et sa réservation libérée) : seule la
   règle L est omise ; un pair qui « goutte » un tel flux ne retient que sa propre part. Attendre sa réponse n'est jamais un échec ; la lenteur
   seule non plus ; rien n'est jamais repris pour faire de la place.

### 7.3 Générations

La synchronisation automerge-repo avec un pair vit dans une **génération** `g` (`u32`). Par
pair distant, chaque côté tient `gen` (0 au début de la session), `mode ∈ {Bas, Attente,
Synchro}` et `conf` (vrai si `gen` a été confirmée par le pair). Côté local, l'**époque** d'un
élément est le couple `(session, gen)` (§5.1) : deux sessions vers le même pair ne se
confondent jamais, même si leurs générations repartent de 0. La génération est **portée
par le préambule du flux `/inkwell/repo/2.0.0`** : tout ce qu'un flux transporte appartient à sa génération,
sans étiquette par trame. Le modèle [tla/RepoGen.tla](tla/RepoGen.tla) vérifie ces règles
(TLC, §21).

- **Règle I (engager).** Depuis Synchro, sur une perte (règle L) : en un seul pas de l'acteur
  du pair, `gen := gen + 1`, `mode := Attente`, `conf := faux`, réinitialiser le flux REPO
  sortant courant **et** tout flux LARGE sortant (ce qu'ils contenaient est jeté
  atomiquement par le transport), **annuler** (STOP_SENDING) le flux REPO entrant (un flux LARGE
  entrant de l'ancienne génération est lu jusqu'au bout et jeté, §5.1 c), ouvrir un nouveau flux
  REPO sortant de préambule `gen`, et dire à la webview que le pair part.
- **Règle B (activer).** Entrer en Actif (début de session, reprise après Restreint) est un
  engagement vers `gen + 1` si `conf`, sinon vers `max(gen, 1)` (une génération jamais
  confirmée est rouverte telle quelle, jamais dépassée).
- **Règle A (accepter un flux REPO entrant de génération h).**

| Condition | Action |
|---|---|
| `mode = Bas`, ou `h < gen` | réinitialiser ce flux, sans sanction (`#repo-stale`) |
| `h = gen`, `mode = Attente` | confirmation : lire ce flux ; `mode := Synchro`, `conf := vrai` ; la webview voit le pair revenir en génération `h` |
| `h = gen`, `mode = Synchro` | le pair a rouvert une génération non confirmée, qui n'a donc porté aucune donnée : ce flux remplace l'entrant courant |
| `h = gen + 1` | suivre : comme la règle I vers `h`, mais `mode := Synchro`, `conf := vrai`, et lire ce flux (`#repo-open-next`) |
| `h > gen + 1` | violation (`#repo-gen-jump`) : un pair honnête n'a jamais plus d'une génération d'avance |

- **Règle R (réessayer).** En Attente, si le pair réinitialise notre nouveau flux (il est Bas
  ou nous croit en retard), on rouvre un flux de la **même** génération après le délai de
  cadence ; un réessai ne change jamais `gen`.
- **Règle T (génération à chaque passage).** Elle vaut pour les éléments **et pour les
  événements** (fin, réinitialisation, échec d'un flux) : un événement d'une époque passée est
  ignoré. Tout élément de la classe dépôt (trame lue, objet
  LARGE, message du pont dans les deux sens) porte l'**époque** `(session, gen)` du flux d'où il
  vient ; à chaque passage asynchrone (file d'entrée, pont, livraison à la webview, envoi), un
  élément dont l'époque n'est pas l'époque courante en Synchro est jeté. L'annulation atomique des files
  (règle I) est l'obligation de libérer la mémoire ; la règle T est ce qui garantit qu'aucun
  ancien message n'entre dans la nouvelle génération, quel que soit l'entrelacement.
- **Barrière.** Sur un flux, aucune trame d'une génération ne précède son préambule ; aucune
  trame d'une génération dépassée n'est lue après l'acceptation d'un flux plus récent. La
  barrière est le préambule lui-même ; elle est vérifiable par le receveur.
- **Cadence.** Un pair **NE DOIT PAS** engager (règle I) ni réessayer (règle R) plus d'une
  fois par 2 s vers un même pair ; l'intervalle double à chaque nouvel engagement dans les
  10 min (jusqu'à 60 s). Les deux partagent ce seul compteur. Un flux qui **suit** (règle A,
  ligne 4) répond à un flux que nous avons ouvert et n'use pas de jeton. Le receveur compte
  donc : flux REPO entrants en 2 s ≤ 1 + nombre de flux REPO **que nous avons ouverts** dans
  les 2 s précédentes ; au-delà, GOAWAY OVERLOADED.
- **Règle W (épuisement).** `gen ≤ 2^32 − 1`. Si un pas devait dépasser cette valeur, le pair
  ferme la session (GOAWAY NO_ERROR) ; la suivante repart de 0. À la cadence ci-dessus, il
  faudrait plus de 270 ans de réinitialisations ininterrompues.
- **Ce que le modèle vérifie, et seulement cela** ([tla/](tla/README.md)). `RepoGen`, **exhaustif**
  sur `RepoGenSmall.cfg` (3 générations, 1 message par génération, 1 perte, 1 suspension, file
  de 1) : deux pairs, une seule session, flux non ordonnés entre eux, pertes locales, demandes
  de la webview, perte du flux entrant ou sortant, suspensions, livraisons asynchrones avec
  contrôle de génération — aucune violation entre pairs honnêtes ; `|gen_A − gen_B| ≤ 1` ;
  deux côtés en Synchro ont la même génération ; ce qu'un côté livre dans une génération est un
  préfixe sans trou de ce que l'autre y a envoyé ; à l'arrêt, tout ce qui a été envoyé a été
  livré ; avec un nombre fini de pertes et de suspensions, retour à une même génération. La
  livraison n'y attend pas de crédit du pont. `ConnCredit`, exhaustif : un sens d'une
  connexion QUIC, fenêtre de connexion partagée, REPO non lu, LARGE après LARGE_REPLY,
  changements de génération dont le crédit ne revient qu'au RESET_STREAM du pair, un seul flux
  annulé en attente, webview qui peut s'arrêter pour toujours — le contrôle passe toujours
  (après au plus le RESET_STREAM d'un pair honnête), ou la session d'un pair qui ne répond pas
  est fermée ; retirer LARGE_REPLY ou la fermeture à 10 s le casse. **Non modélisés,
  argumentés** : plus de deux pairs (les machines sont par pair), le passage entre deux
  sessions (époques distinctes), les crédits du pont (§10.7).

### 7.4 Émetteur : machine à états par pair

| État du flux REPO sortant | Entrées | Sorties |
|---|---|---|
| Aucun | activation (règle B) | Ouvert(g) |
| Ouvert(g) en Attente | flux du pair de génération g (règle A) → Synchro ; réinitialisé par le pair → Aucun puis règle R | — |
| Ouvert(g) en Synchro | messages de la webview de génération g, dans l'ordre : ATTEST et AUTHOR d'abord, puis REPO ou LARGE ; un LARGE à la fois | perte → règle I ; suspension → Aucun ; sortie d'Actif → Aucun |

**Rythme à la source** (aucune file locale ne déclenche la règle L) : un pair fait entrer les
documents en synchronisation avec un pair **progressivement** (au plus 64 dont le premier
message n'a pas encore reçu de réponse ; dans automerge-repo, par un `sharePolicy` qui répond
à mesure ; une place se libère à la réponse, à un `doc-unavailable`, au refus du document ou
après 60 s sans réponse) ; il garde **au plus un** message en attente de crédit par (pair, document) — un
nouveau message pour ce (pair, document) remplace l'ancien, et l'état de synchronisation
Automerge de ce seul (pair, document) est réinitialisé, ce que le protocole de synchronisation
d'Automerge tolère (l'état de synchronisation peut être perdu à tout moment ; il est même
éphémère ici, §9.3) ; aussitôt après, l'adaptateur demande à automerge-repo un nouveau
message pour ce (pair, document) (reprise de la synchronisation de ce document avec ce pair),
qui prend la place libérée. La limite de 16 Mio vaut pour la voie en ligne ; un message LARGE
attend seul dans la voie LARGE, quelle que soit sa taille (≤ 64 Mio). Un démarrage avec
10 000 documents et un pair lent ne produit donc ni débordement ni réinitialisation.

Flux LARGE sortant (le délai de 60 s sans progrès court seulement **après** LARGE_REPLY) :
`Annoncé → Demandé (LARGE_REPLY 1) → Écrit → Fini`, ou `Annoncé → Refusé (LARGE_REPLY 0)`
(une modification indivisible reste alors servie par le vrac, §8.1), ou `Réinitialisé` (règle I, sortie d'Actif,
réinitialisation par le pair, 60 s sans progrès) → règle L. L'objet est gardé, dans la part
du pair (§10.6), jusqu'à `Fini` ou `Réinitialisé`.

## 8. Vrac et salon

### 8.1 `/inkwell/bulk/1.0.0` : blobs (*existant*, règles précisées)

Les blobs (fichiers joints, ADR 0023) peuvent venir de n'importe quel membre qui les détient :
on les vérifie contre leur empreinte. Un flux unidirectionnel par sens et par pair, trames
`u32 length` comme §5.2, `length ≤ 1 065 005`. Corps :

```
want    { kind u8 = 1 ; hash [32] ; offset u64 ; length u32 }   offset multiple de 16 384 ; length = 16 384 × k, 1 ≤ k ≤ 64
slice   { kind u8 = 2 ; hash [32] ; offset u64 ; length u32 ; bao [..] }   bao ≤ 1 064 960 octets
missing { kind u8 = 3 ; hash [32] }
busy    { kind u8 = 4 ; hash [32] ; retry_ms u32 ≤ 60 000 }
```

- **Bao** ([Bao], groupes de 2^4 blocs de 1 Kio = 16 Kio) : `bao` = taille totale du blob en
  `u64` **petit-boutiste**, puis, en préordre, les nœuds parents (64 octets : les deux valeurs
  de chaînage filles) du chemin qui mène à la plage, puis les octets des groupes de la plage ;
  il se vérifie contre le seul `hash` (`bulk.json#slice-final-group` : 8 + 64 + 4464 octets).
- **Dernier groupe.** Une demande est toujours en groupes entiers, même au-delà de la fin
  (`#want-final-group`). La réponse a `length` = la longueur demandée, ou moins **seulement si**
  `offset + length` = taille totale (`#slice-final-group`) ; une demande à la longueur exacte
  d'un dernier groupe partiel est mal formée (`#want-final-group-exact`).
- **Avant toute allocation**, le demandeur vérifie que la taille totale annoncée par `bao`
  égale la taille attendue (index du projet) et que `offset < taille` ; sinon faute
  (`#slice-size-mismatch`).
- **Appariement.** Au plus **une** demande en cours par (pair, hash) et au plus 2 par pair ;
  une réponse s'apparie par (pair, hash) ; un SLICE **DOIT** reprendre l'`offset` demandé et la
  `length` demandée, **sauf** pour la plage qui contient la fin du blob, où `length` = taille −
  `offset` (`#slice-final-group`). Un SLICE, MISSING ou BUSY sans demande en cours pour (pair, hash) est
  **non sollicité** : jeté sans vérifier Bao ; c'est une faute seulement si aucune demande de
  ce hash n'a été annulée (délai, génération, abandon) depuis 60 s (`#slice-unsolicited`).
  Demander plus de 2 à la fois est une violation.
- **Délais au progrès.** Une demande échoue si **aucun octet** de réponse n'est arrivé en
  30 s ; une réponse qui progresse n'échoue jamais. La taille des demandes s'adapte au débit
  mesuré du pair : de 64 Kio au départ, doublée quand une réponse arrive en moins de 2 s,
  divisée par deux au-delà de 8 s, entre 16 Kio et 1 Mio.
- **BUSY** : le serveur est à sa capacité pour ce pair ; le demandeur attend `retry_ms` et
  redemande. Cinq BUSY de suite sur la même plage comptent pour un échec ; un serveur envoie
  au plus 32 BUSY par pair et par minute, au-delà il se tait (et le délai au progrès joue).
- **Échecs et fautes.** Trois échecs sur une plage : on la demande à un autre détenteur.
  Une **faute** (Bao invalide, taille fausse, non sollicité fautif) compte contre le pair :
  3 fautes en 10 min, on ne l'interroge plus pendant 10 min. Une trame mal formée (décodage)
  est une **violation**, comme partout (§13).
- **Modifications indivisibles** (accord D1, [09-debate-D1](reviews/09-debate-D1.md)). Le même
  protocole sert une modification annoncée `LARGE` indivisible (§7.2), par son `root` :
  - **imputation** : tout ce qui n'est pas encore vérifié est imputé au **pair qui a fait
    l'annonce** (le livreur), toujours : le tuple d'auteur signe `change`, pas `root`, `size` ni
    `doc`, donc un relais peut associer un tuple honnête à une fausse racine ; c'est alors son
    emplacement à lui qu'il occupe (`repo.json#indivisible-tuple-false-root`). Le quota cumulé
    de l'**auteur** (N10) n'est crédité qu'**après** vérification de `change`. Il n'y a donc plus
    de principal provisoire ni de dépendance à l'étape 8 ;
  - **liaison** : le transfert est lié au document `doc`, à l'empreinte Automerge attendue
    `change`, à la taille `size` et à la racine `root` ; Bao vérifie chaque tranche contre
    `root` ; à la fin, l'empreinte Automerge des octets doit valoir `change`
    (`repo.json#indivisible-hash-mismatch`) et la longueur valoir `size`
    (`#indivisible-size-mismatch`) ; puis la modification passe la chaîne de réception normale
    (§9.4 : admission, couverture, auteur G12, garde) comme une page d'une seule modification,
    dont l'enveloppe CBOR est faite **localement** ;
  - **dédoublonnage** : une annonce d'un `root` déjà en cours de tirage **ajoute un détenteur**,
    jamais un second transfert ;
  - **qui sert** : l'annonceur, et tout membre qui détient la modification. Chacun tient un
    **index persistant** `root → (document, empreinte)` des modifications indivisibles qu'il a
    produites ou **vérifiées et admises** (jamais avant), et la sert tant que le document est
    admis ; une entrée est retirée quand le document n'est plus admis ou est purgé, ou quand la
    modification n'y est plus. **Borne** : `max` étant fixé à `r` (§9.6), une modification
    indivisible a une page singleton encodée de plus de 4 Mio, donc la modification elle-même
    fait plus de `r` moins l'enveloppe (≈ 4 Mio), et l'index a au plus une entrée par ≈ 4 Mio
    d'historique vérifié et stocké (`repo.json#page-route-*`, à `max = r`)
    (~70 octets par entrée), imputée au quota N10 de l'auteur avec sa modification ; aucune
    fenêtre de temps, ce qui couvre l'intervalle entre `LARGE_REPLY` et le premier WANT et un
    redémarrage. On la demande à l'annonceur d'abord, puis aux autres à tour de rôle ;
  - **attente sur disque** : les tranches reçues sont rangées sous le dossier de données du
    projet (§14, point 7), **par `root`**, avec la liste de ses détenteurs connus.
    **Emplacements.** Chez un receveur, chaque pair **connecté** (session Active) possède
    **un** emplacement de 16 Mio. Une entrée est **active** si et seulement si elle **occupe
    l'emplacement d'un pair connecté qui la détient** ; seules les entrées actives sont tirées.
    Une entrée dont tous les détenteurs connectés ont déjà leur emplacement pris est
    **inactive** et n'est pas tirée.
    **Attribution** (accord D2, [20-debate-D2](reviews/20-debate-D2.md)), toujours dans un
    emplacement **libre**, avec **réservation atomique** de la taille annoncée (≤ 16 Mio).
    Chaque entrée a un **rang d'attente** : l'ordre de sa première annonce chez ce receveur,
    persisté, conservé à travers les réannonces et les redémarrages. Une règle unique : **chaque
    fois qu'un emplacement devient libre ou disponible** (fin ou abandon du tirage, connexion du
    pair, redémarrage du receveur), il est réservé à l'entrée inactive **éligible** (que ce pair
    détient) **de plus petit rang**, avant toute nouvelle annonce ; s'il n'y en a pas, à la
    prochaine annonce de ce pair. Une annonce d'un `root` nouveau quand l'emplacement du pair est
    pris crée une entrée inactive, rangée en fin d'attente ; une annonce d'un `root` déjà connu
    (dédoublonnage) ajoute le détenteur, sans changer le rang. Quand le pair attribué se
    déconnecte ou est **retiré de la liste des membres**, l'entrée devient inactive (elle garde
    son rang) et suit la même règle chez ses autres détenteurs connectés ; un détenteur retiré
    de la liste des membres est **retiré de la liste des détenteurs**. Un tirage actif n'est
    **jamais** préempté. *(« Plus récemment avancée d'abord » au redémarrage, proposé par Fable,
    n'est pas retenu : une seule règle, l'ordre d'attente partout — à confirmer par les deux
    relecteurs.)*
    **Vivacité.** Hypothèse : l'entrée a au moins un détenteur **honnête, connecté et
    disponible**. Alors elle est tirée, et son attente est bornée par le nombre d'entrées de rang
    plus petit dans la file de ce détenteur, multiplié par la durée d'un transfert de 16 Mio à
    son débit ; un flux de nouvelles annonces ne passe jamais devant elle (scénario « R face à des
    annonces renouvelées », porte P-BYZ). Sans détenteur honnête connecté, aucune garantie n'est
    possible ; la règle H reste le recours.
    **Plafond global** : 1 Gio d'attente au total, entrées inactives comprises (pairs absents).
    Les entrées actives occupent au plus un emplacement par pair connecté, donc au plus
    64 × 16 Mio = 1 Gio ; pour réserver un emplacement, on retire d'abord les entrées finies ou
    plus voulues (document plus admis, modification obtenue autrement), puis les inactives **sans
    détenteur connecté**, de la moins récemment avancée à la plus récente, et seulement en dernier
    les inactives éligibles, **du plus grand rang au plus petit** (la plus ancienne en attente
    part la dernière) ; une entrée active n'est **jamais** retirée. Retirer une entrée inactive
    retire ses **octets**, jamais son rang d'attente (un enregistrement de ~100 octets, au plus 64
    par pair) : à son tour, elle repart de zéro, et la borne d'attente ci-dessus ne change pas.
    Une réservation dans l'emplacement d'un pair connecté réussit donc toujours, et ni 65
    membres successifs, ni un pair qui annonce plusieurs racines laissées par d'autres, ne
    dépassent le plafond (scénarios de la porte P-BYZ). Jamais sur un délai. Plusieurs relais
    peuvent faire tirer la même modification d'un même auteur chacun dans son emplacement (rien
    ne limite *par auteur réel* avant vérification), mais chacun reste borné par pair, et le
    dédoublonnage par `root` évite les copies d'une même racine ;
  - **plages doublées** : une plage en cours depuis plus de 30 s chez un détenteur **DOIT** être
    demandée aussi à un autre détenteur **connecté et membre**, s'il en existe un (la règle « une demande par (pair, hash) » vaut par
    pair) ; la première réponse complète gagne, les autres sont annulées. Un relais pressé qui
    annonce le premier une vraie racine puis « goutte » une plage ne retarde donc rien ;
  - **reprise** : l'état (groupes vérifiés) est persisté et repris après un redémarrage ou un
    changement de détenteur, comme les têtes de l'historique, sans limite de durée ;
  - **règle H non suspendue** : pendant un tirage pour le document D, le receveur peut
    toujours demander des pages de D à **d'autres** détenteurs ; un annonceur qui « goutte »
    ne retient donc que son propre emplacement, jamais l'accès du receveur à D ;
  - **ordonnancement** : les demandes sont servies et émises en DRR (§10.4) ; aucun quantum,
    aucune reprise pour lenteur ;
  - **échecs** : trois échecs sur une plage chez un détenteur → un autre détenteur ; plus aucun
    détenteur → règle H (le document est redemandé plus tard).
- **Service.** Un blob n'est servi que s'il est référencé par le projet, **ou** une modification
  indivisible **présente dans l'index du serveur** (ci-dessus), par son `root`, si elle appartient à un document admis et que
  le demandeur est un membre actif non suspendu ; sinon MISSING (`repo.json#indivisible-serve-unadmitted`) ;
  un blob de projet n'est servi que si le demandeur
  est un membre actif non suspendu ; sinon MISSING (indiscernable d'un blob absent). Chaque
  pair actif a une part réservée d'au moins une demande servie à la fois (§10.6), deux au plus.

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
  chaîne depuis la genèse, chaque clé (§3.2), et que l'empreinte de la genèse est le
  `project` de son ticket.
- Un refus est volontairement grossier : ticket faux, utilisé, expiré ou d'un autre projet
  donnent toujours `1 Invalid`.

### 8.3 Ticket d'invitation (*existant*)

Texte : `"inkwell1:"` suivi du base32 ([RFC 4648], alphabet standard, sans `=`) de :

```
ticket { version u8 = 1 ; flags u8 (bit 0 : person présent, autres bits = 0) ; role u8 1–3 ;
         project [32] ; [person [32] si bit 0] ; secret [16] ; expires u64 (secondes Unix) ;
         host str8 (PeerID, clé valide §3.2) ; naddr u8 ≤ 16 ; naddr × str8 (multiaddr sans « /p2p/<host> ») }
```

Le texte fait au plus 1024 octets ; à la lecture, la casse, les espaces et les retours à la
ligne sont ignorés. Durée de vie : 15 min. Vecteur : `lobby.json#ticket-text`. Le ticket est
un secret : quiconque le détient peut rejoindre le projet une fois.

### 8.4 Rendez-vous Wormhole (*existant*, [ADR 0032])

Le texte du ticket peut voyager par magic-wormhole ([Wormhole]) : `AppID =
"inkwell.app/invite/v1"`, texte seul, sans relais de transit ; code `^[0-9]+(-[a-z]+)+$`,
≤ 64 caractères. Ce canal ne transporte rien d'autre.

## 9. Couche dépôt : messages automerge-repo

### 9.1 Contenu

Les trames REPO et PRESENCE, et les flux LARGE, contiennent un message automerge-repo 2.x
(ou une page d'historique, §9.6) encodé en CBOR ([RFC 8949]) et décrit par
[cddl/repo.cddl](cddl/repo.cddl). Le champ `data` de `sync` et `request` est un message de
synchronisation Automerge ([Automerge format]) ; `changes` d'une page, des modifications
Automerge entières.

### 9.2 Décodage

#### 9.2.1 Profil de validité CBOR (receveur)

Ces messages ne sont **pas signés** : aucune forme canonique n'est nécessaire pour une
empreinte. Ce qu'il faut empêcher, ce sont les **désaccords d'analyse** (deux décodeurs qui
lisent deux messages différents dans les mêmes octets). Le receveur **DOIT** donc refuser,
avant tout décodage de schéma, exactement ce qui peut se lire de deux façons :

1. plus d'un élément CBOR, ou des octets après lui (`repo.json#trailing-byte`) ;
2. une longueur indéfinie (`#indefinite-map`), une information additionnelle réservée ;
3. une **étiquette** (type majeur 6), y compris le tag 64 sur une chaîne d'octets (`#tag-64`) ;
4. un **flottant** (`#float`), une valeur simple autre que `false`, `true`, `null` ;
5. un **entier négatif** (`#negative-int`) ou un entier > 2^53 − 1 (`#big-uint`), que
   JavaScript ne lit pas exactement ;
6. une clé de map qui n'est pas une chaîne de texte (`#non-text-key`), ou une **clé en
   double** (`#duplicate-key`) — `cbor-x`, par exemple, garde silencieusement la dernière.
   Deux clés sont en double si leurs **textes décodés** sont égaux, quel que soit leur
   encodage (`#duplicate-key-nonpreferred` : `64 'type'` puis `78 04 'type'`) ;
7. un texte qui n'est pas de l'UTF-8 valide (`#bad-utf8`) ;
8. une **profondeur** > 8 (l'élément racine est au niveau 1) (`#too-deep`), plus de 4096
   éléments au total, une map de plus de 64 paires, un tableau de plus de 256 éléments.

Le profil s'applique à tout le message, **champs inconnus compris**, et au CBOR imbriqué du
`data` des éphémères. Il n'exige **ni** la sérialisation préférée **ni** un ordre des clés : un
même message accepte plusieurs encodages qui donnent tous la même valeur, ce qui ne crée aucun
désaccord (`#liberal-cbor-x-map-header`). C'est une restriction de [RFC 8949 §5.3.1] (validité de base : clés en double, UTF-8) et de §5.6, pas un profil déterministe au sens de §4.2.

#### 9.2.2 Émission

Un émetteur **DOIT** produire des messages conformes au profil ; il **DEVRAIT** utiliser la
sérialisation préférée et l'ordre des clés de l'encodage déterministe de base
([RFC 8949 §4.2.1]), ce que font les vecteurs acceptés et l'encodeur de `@inkwell/protocol`.
Pour mémoire : `cbor-x` réglé comme automerge-repo (`tagUint8Array: false, useRecords:
false`) produit des messages conformes mais non déterministes (en-tête de map sur 2 octets,
ordre d'insertion) ; `encode` de `cbor-x` **sans réglage** étiquette les `Uint8Array`
(tag 64) et n'est donc **pas** conforme (constat N-12).

#### 9.2.3 Schéma et contexte

- Le message est ensuite lu contre le CDDL. Dans une map **ouverte** (`* tstr => any`), les
  clés inconnues sont ignorées et **jamais conservées ni retransmises** : le receveur
  reconstruit le message à partir des seuls champs connus (`#unknown-key-ignored`). Dans une
  map **fermée** (enveloppes et messages de présentation), une clé de plus fait refuser le
  message (`#present-extra-key`). C'est vrai à toute profondeur.
- **Une seule règle d'identité, pour tous les types** : `senderId` **DOIT** être le PeerID
  authentifié de l'émetteur et `targetId` celui du receveur, poignée de main comprise
  (`#wrong-sender`, `#wrong-target`, `#arrive-no-target`).
- Types traités : `arrive`, `welcome`, `sync`, `request`, `doc-unavailable`, `ephemeral`,
  `history-request`, `history-page`. `remote-subscription-change` et `remote-heads-changed`
  sont refusés (`#remote-heads-changed`) ; tout **autre** type est ignoré sans erreur
  (extension future ou GREASE, §1.4.2 ; `#compat-unknown-type`).
- Une clé inconnue dont le nom commence par `!` est « à comprendre » : le message est refusé
  (`#compat-critical-key`, `#compat-nested-critical-key`). Aucune clé de cette version ne
  commence par `!`.
- Bornes exprimées dans le CDDL par des contrôles (`.size`, intervalles, occurrences) ;
  le couplage des coordonnées du pointeur (toutes deux nulles ou toutes deux entières) est
  un choix de types (`#laser-half`).

### 9.3 Poignée de main

`arrive` puis `welcome` (≤ 1024 octets chacun, `#handshake-too-big`) ouvrent la génération
côté dépôt. `peerMetadata` est ignorée : tout pair est éphémère et son `storageId` jamais
utilisé (aucun état de synchronisation persistant par pair). `projectUrl` (≤ 80 caractères,
URL Automerge valide) annonce le document racine. Un pair peut renvoyer `arrive` au plus 3
fois, puis 1 fois par 20 s.

### 9.4 Chaîne de réception, admission et couverture

Un message de document n'atteint le `Repo` qu'après **toutes** ces étapes, dans cet ordre
([ADR 0031] §1, [ADR 0030] §4) :

1. **profil** (§9.2.1), **schéma et contexte** (§9.2.3) ;
2. **admission** : l'émetteur est un appareil membre actif non suspendu (la table des pairs
   de la webview ne contient que des membres) ; `documentId` est dans l'ensemble admis
   (§4.10), sinon le message est jeté sans réponse (`#not-admitted`) ; budgets de la poignée
   de main et des éphémères ;
3. **couverture** ([ADR 0018], T10) : pour `sync`, `request` et `history-page`, chaque
   modification apportée est une tête attestée (§4.3) par un appareil qui écrit, actuel et non
   suspendu, ou un ancêtre d'une telle tête. Une modification non encore couverte **attend**
   (au plus 4 messages par document et par pair, 8 Mio par pair, 30 s) pendant la
   réconciliation (§6.2) ; à l'expiration, c'est un refus, et la règle H la redemandera ;
4. **garde structurelle** (§9.7), y compris **G12** : chaque modification a un tuple d'auteur
   vérifié (§4.7) dont l'acteur dérivé est le sien ; un tuple manquant fait attendre (4 par
   auteur, 30 s, puis refus et règle H) ;
5. **`Repo`**.

Un message pour un document admis pas encore chargé **PEUT** attendre (4 par document et par
pair, 8 Mio par pair) ; au-delà, refus et règle H.

### 9.5 Éphémère

`ephemeral` porte la présence (`__presence`) ou le pilotage de présentation (`__present`),
≤ 1024 octets de `data` (`#ephemeral-too-big`), sur le flux PRESENCE. Budget par pair : 128
messages en rafale, 64 par seconde ensuite ; au-delà, jetés. Aucun relais sortant. Le
pointeur de présentation est en **dix-millièmes entiers** de la diapositive
(`#present-laser`) : le protocole n'a aucun flottant.

### 9.6 Pages d'historique (ADR 0031 M5)

- Un receveur qui n'a aucune copie d'un document admis, qui refuse un message de
  synchronisation au-delà des bornes de la garde avec une erreur récupérable, ou qui applique
  la règle H, envoie `history-request {documentId, have, max}` (`have` : ses têtes, ≤ 256 ;
  `max` : la taille maximale d'une page, enveloppe CBOR comprise, **fixée à `r` = 4 194 304**
  dans cette version — une autre valeur est refusée, `#history-request-wrong-max` ; le champ
  reste pour qu'une mineure future puisse l'assouplir, §1.4).
- Le destinataire répond par **une** page `history-page {documentId, changes, last}` : des
  modifications entières (0 à 256), formant un **préfixe causalement clos** (chaque dépendance
  d'une modification est dans le passé causal de `have` ou plus tôt dans la même page), dans
  l'ordre topologique, choisies parmi **toutes celles
  qui ne sont pas dans le passé causal de `have`** (les branches concurrentes comprises, pas
  seulement les descendants), dont l'**encodage** fait ≤ `max` octets (enveloppe CBOR
  comprise) et ≤ 16 Mio décompressés. Une modification est **indivisible** pour cette demande
  quand la page qui ne contiendrait qu'elle, **encodée**, dépasse `max` (`repo.json#page-route-*`) :
  elle part seule, annoncée `LARGE` indivisible et tirée par le vrac (§7.2, §8.1). Quand il
  n'a plus rien,
  il répond par une page **terminale** `changes = [], last = true` (`repo.json#history-page-terminal`).
- L'émetteur fait précéder chaque page, sur le flux REPO, des ATTEST et AUTHOR qui la couvrent
  (§5.6) ; une page de plus de 64 Kio passe par LARGE.
- C'est aussi le moyen de la **règle H** (§5.5) : un receveur qui a refusé ou laissé expirer
  des modifications d'un document les redemande ainsi, une fois la cause disparue.
- Le receveur passe la page par la chaîne §9.4, l'applique, la persiste, puis redemande avec
  ses nouvelles têtes, jusqu'à `last = true`. Le rythme est celui du receveur ; une reprise
  après redémarrage repart des têtes persistées. Il **PEUT** demander la page suivante dès la
  réception de la précédente, avant de l'appliquer, avec pour `have` ses têtes réunies à celles
  des modifications reçues — c'est une frontière valable **parce que** chaque page est un
  préfixe causalement clos : au plus **deux** pages en vol par (pair, document).
- *Coût.* Avec la part fixe de 4 Mio, une synchronisation initiale d'un historique de `H` octets
  demande `⌈H / 4 Mio⌉` pages successives (50 pour 200 Mio) ; à deux pages en vol, le coût en
  allers-retours est divisé par deux, sans dépasser la part.

### 9.7 Garde structurelle (normatif, [ADR 0030] §4 et [ADR 0031])

La garde décide si une modification Automerge `c` peut entrer, en fonction de `c` et de son
passé `before(c)` seulement (donc même verdict partout, sauf G8). Un refus fait jeter le
message entier. Deux implémentations **DOIVENT** rendre le même verdict.

| Règle | Contenu |
|---|---|
| G1 bornes | `c` se décode ; acteur de 1 à 32 octets ; modification ≤ 16 Mio décompressés et ≤ 65 536 opérations (analyse bornée **avant** Automerge) ; `deps` distincts ; `1 ≤ startOp`, `startOp + |ops| ≤ 2^53` ; actions `set`, `del`, `inc`, `make{Map,List,Text,Table}`, `markBegin`, `markEnd` ; types de donnée connus |
| G2 dépendances | dépendance inconnue : attendre (quota par émetteur) ; dépendance refusée : refuser |
| G3 chaîne d'acteur | `seq = 1` si l'acteur est absent de `before(c)`, sinon `(acteur, seq − 1) ∈ before(c)` |
| G4 Lamport | `startOp > maxOp(before(c))` |
| G5 objet | `obj` vaut `_root` ou désigne un `make*` de `before(c)` ou plus tôt dans `c` ; map/table : `key` ; liste/texte : `elemId` |
| G6 élément | `elemId` vaut `_head` ou désigne une insertion du même objet, dans `before(c)` ou plus tôt dans `c` |
| G7 `pred` | entrées distinctes, chacune une op `set`, `make*` ou insertion de `before(c)` (ou plus tôt dans `c`) sur le même objet et la même clé ou élément, jamais un `del` ni un `inc` |
| G8 unicité | un `(acteur, seq)` déjà admis avec une autre empreinte est refusé et prouve une équivoque (§4.8) |
| G9 schéma | pour l'index G (§4.10) : **R-root** — une op dont `obj` est `_root` n'est admise que dans G ; **R-nodes** — sur la map `nodes`, seule `makeMap` est permise, avec une clé de 32 caractères hexadécimaux minuscules et `pred` vide (ni `del`, ni scalaire, ni remplacement) (`repo.json#g9-*`) |
| G10 document complet | chargement isolé, puis chaque modification dans l'ordre topologique passe G1–G9 |
| G11 ancrage | rien d'un document dérivé avant son ancre (§4.6) ; ensuite, l'ancre est dans le passé de chaque modification |
| G12 auteur | tuple vérifié (§4.7), acteur = `actor(key, epoch)` ; exceptions : la genèse exacte G et le passé `legacy/<d>` |

## 10. Contrôle de flux et équité

### 10.1 Le principe unique

> Toute file a **un propriétaire, une borne et un crédit**. Un producteur sans crédit attend
> sans rien bloquer d'autre. Une boucle partagée par plusieurs pairs **NE DOIT** jamais
> attendre un pair. Un dépassement ne coûte qu'à celui qui l'a causé. Tout plafond partagé a
> des **parts réservées** (§10.6).

### 10.2 Sur le réseau

- **Crédit = celui du transport, par flux.** Un receveur dont la file d'un flux est pleine
  **DOIT** cesser de lire ce flux, sans le réinitialiser ; le transport freine l'émetteur, sur
  ce flux seulement.
- **Fenêtre de connexion** : voir §5.1 ; un seul flux de données (REPO) peut rester non lu
  par connexion.
- **Ce qui est toujours lu** : CONTROL (son traitement ne fait jamais attendre la lecture :
  une réponse qui ne trouve pas de place en sortie est différée par son curseur §6.1, ou
  ignorée pour ATTEST_WANT) et PRESENCE (on remplace). **Ce qui est lu au rythme du
  receveur** : REPO (place dans la file d'entrée), LARGE (réservation, §7.2).
- **Aucun délai sur la lenteur.** Une écriture bloquée parce que le receveur ne lit pas est de
  la contre-pression, pas une panne ; il n'y a pas de délai d'écriture sur CONTROL, REPO et
  PRESENCE. Une connexion morte est détectée par le transport (délai d'inactivité de QUIC,
  maintien de yamux, ping libp2p). Seul **l'arrêt complet** d'un transfert que le receveur a
  demandé ou réservé (LARGE §7.2, vrac §8.1) déclenche un échec, jamais un débit faible
  (ADR 0028 : un pair derrière un relais lent doit finir, même lentement).

### 10.3 Sur le pont interne

Le Stream Wails n'a pas de contrôle de flux : l'Annexe A définit des crédits explicites par
(pair, génération, voie), avec la règle de [RFC 9000 §4.1] (limites absolues,
idempotentes). Deux voies séparées : **en ligne** (messages REPO, fenêtre 4 Mio) et **LARGE**
(un objet à la fois, quelle que soit sa taille). Un gros objet n'occupe donc jamais la
capacité de la frappe. La présence n'a pas de crédit : à perte, coalescée par Go, jetée par la
webview si elle déborde.

### 10.4 Équité

Une boucle qui sert plusieurs pairs (livraison à la webview, traitement des SET, tirage du
vrac) choisit le pair suivant par **Deficit Round Robin** ([DRR]) : chaque pair qui a du
travail reçoit un quantum de 64 Kio par tour ; un pair inondant obtient au plus sa part. Les
pairs sans crédit sont sautés sans attendre. Dans la webview, le coût du décodage, de la
garde et de l'application est imputé à l'**auteur** (tuple §4.7), le transfert au pair qui
livre ([ADR 0031] F-A1).

### 10.5 Ordre d'émission vers un pair

Un écrivain par flux. Les flux étant séparés, l'ordre entre classes est l'affaire du
transport ; dans un flux, l'ordre d'origine. Sur CONTROL : HELLO, puis GOAWAY avant toute
autre trame en attente, puis SET et ATTEST dans l'ordre. Sur REPO : ATTEST et AUTHOR avant le
message qu'ils couvrent.

### 10.6 Plafonds partagés : parts réservées

> **Règle.** Un plafond `C` partagé par des **principaux** (pairs actifs, ou signataires
> debout) garantit à chacun une part `r = ⌊C / n⌋`, où `n` est le nombre courant de principaux
> éligibles. Un principal **sous sa part** est toujours admis : si `C` est atteint, on reprend
> à un principal **au-dessus** de sa part (le plus en excès, son élément le plus ancien),
> jamais à un principal sous sa part. Un principal au-dessus de sa part n'est admis que sur la
> place libre. Ce qu'on reprend suit la sémantique de sa classe : un élément d'état est oublié
> (il reviendra par l'anti-entropie ou la réconciliation) ; un élément de la classe dépôt est
> libéré avec la règle L.

**Parts fixes pour la mémoire réservée d'avance.** Quand une réservation n'est pas reprenable
(objet LARGE en cours de lecture), la part est **fixe** : `r = C / N_max`, avec `N_max` le
nombre maximal de principaux (64 pairs actifs, §16.1). La somme des réservations ne dépasse donc
jamais `C`, et tout nouvel objet ≤ `r` est admis aussitôt, même quand des pairs arrivent : il
n'y a rien à renégocier. Les parts dynamiques `⌊C / n⌋` restent pour ce qui est reprenable
(attestations, objets en attente).

Ainsi aucun principal, ni aucune coalition de `k` principaux, ne peut priver un principal
honnête de sa part ; un principal qui retient de la capacité (lecture au compte-gouttes) ne
retient que la sienne, plus du libre qu'on lui reprend dès qu'un autre en a besoin.

| Plafond `C` (§16) | Principaux | Reprise |
|---|---|---|
| attestations gardées : 65 536 clés, 64 Mio | signataires debout (écrivains) | oubli de sa plus ancienne (réconciliation §6.2) |
| objets LARGE réservés par le receveur : 256 Mio | **part fixe** `r = 256 Mio / 64 = 4 Mio` (64 = nombre maximal de pairs actifs) | jamais : un objet ≤ `r` est toujours admis ; plus grand, il est refusé et repris par pages ≤ `r`, ou tiré par le vrac s'il est indivisible (§7.2) |
| objets en attente de parents (SET) : 256 | pairs qui les ont livrés | oubli (tour suivant) |
| messages en attente de couverture ou d'auteur | pairs qui les ont livrés | refus, règle H |
| demandes de vrac servies : 2 par pair, `max(8, 2 × n)` au total | pairs actifs | BUSY |

Une borne qui est déjà **par principal sans total commun** (files d'un pair, budgets
éphémères, journal par auteur) n'a pas besoin de cette règle.

### 10.7 Pas d'auto-blocage

Le graphe « attend » est acyclique, donc aucune combinaison de files pleines ne bloque :

- lire CONTROL ou PRESENCE n'attend rien (§10.2) ;
- lire REPO attend la place dans la file d'entrée du pair, qui attend le crédit du pont, qui
  attend que la webview consomme ; la consommation de la webview n'attend **jamais** (a) une
  trame plus loin sur le même flux (les preuves précèdent les données, §5.6), (b) le flux
  CONTROL (une modification non couverte attend à côté, sans arrêter la consommation, et
  expire en refus, règle H), (c) un crédit **sortant** vers quelque pair que ce soit (la file
  sortante est bornée par document et remplace, §16.2 ; elle n'attend jamais), (d) un message
  LARGE (§5.6 point 3) ;
- un flux LARGE n'existe qu'après réservation (LARGE_REPLY) et se lit toujours ; la
  réservation suivante attend la livraison du précédent (voie LARGE du pont, même argument) ;
- sur QUIC, le crédit de connexion est partagé : le seul flux qui peut rester non lu (REPO)
  retient au plus sa fenêtre, plus, pendant un aller-retour après un changement de génération,
  l'ancien flux annulé (§5.1 c–d) ; CONTROL et PRESENCE trouvent donc du crédit, au pire après
  le RESET_STREAM d'un pair honnête, et un pair qui ne l'envoie pas voit sa session fermée en
  10 s (modèle `ConnCredit`) ;
- écrire n'attend que la lecture du pair ; aucun lecteur n'attend son propre écrivain.

Entre deux pairs, la seule dépendance croisée passerait par « la webview de A attend son envoi
vers B », exclue par (c). Les modèles vérifient : files bornées et générations (`RepoGen`),
crédit de connexion et webview arrêtée (`ConnCredit`) ; les crédits du pont sont argumentés
ici, non modélisés (§7.3).

## 11. Machines à états

### 11.1 Pair distant

```
          connexion authentifiée            HELLO valides des deux côtés
 Absent ───────────────────────► Sécurisé ─────────────────────────────► Actif ◄──────┐
   ▲      (clé valide §3.2,         │  HELLO absent/faux                   │   │        │ reprise
   │       membre ou fenêtre)       ▼                                      │   ▼        │ (règle B)
   │                              Fermé ◄──── GOAWAY / violation ─────── Restreint ────┘
   │                                │            (suspendu : CONTROL seul) │ suspension
   └──────── attente écoulée ──── Tenu (attente 2 s → 60 s) ◄──────────────┘ retrait → Fermé (refusé ensuite)
```

| État | Flux acceptés en entrée | Émis | Sortie |
|---|---|---|---|
| Sécurisé | CONTROL (HELLO d'abord) | CONTROL | Actif ou Restreint ; Fermé (10 s, PROJECT_MISMATCH, NO_COMMON_CAPABILITY) |
| Actif | tous | tous | Restreint (suspension de l'un des deux) ; Fermé (retrait, violation, fermeture) |
| Restreint | CONTROL (SET, ATTEST_WANT, GOAWAY) ; REPO, LARGE et PRESENCE réinitialisés sans sanction | CONTROL | Actif (reprise) ; Fermé |
| Fermé | aucun | GOAWAY éventuel | Tenu (violation, surcharge) ou Absent (retrait) |
| Tenu | — | — | Absent après l'attente (2 s, doublée à chaque incident dans les 10 min, 60 s au plus) |

Entrer en Actif applique la règle B (§7.3), envoie les `have`/`digest` (§6.1) et les
attestations gardées (§6.2). Quitter Actif ferme les flux REPO, LARGE et PRESENCE, libère
les objets retenus et les réservations, vide les files du pair, et la webview voit le pair
partir ; `gen` et `conf` sont gardés jusqu'à la fin de la session.

### 11.2 Objet LARGE

| Côté | États | Transitions |
|---|---|---|
| émetteur | Annoncé → Demandé (`LARGE_REPLY` 1) → Écrit → Fini ; Annoncé → Refusé (`LARGE_REPLY` 0) ; Annoncé → Oublié (10 min sans réponse) | Réinitialisé : règle I, sortie d'Actif, réinitialisation par le pair, 60 s sans progrès après `LARGE_REPLY` → règle L |
| receveur | Annoncé → Réservé (`LARGE_REPLY` 1) → Lu → Livré ; Annoncé → Refusé (`LARGE_REPLY` 0 : avis > `r`, ou indivisible tirée par le vrac) | Réinitialisé : idem ; Lu attend le crédit de la voie LARGE du pont |

### 11.3 Génération

§7.3 (règles I, B, A, R, T, W) et §7.4.

### 11.4 Pair vu de la webview

`Annoncé(s, g)` (PeerUp de l'époque (session s, génération g)) → `Poignée de main`
(arrive/welcome) → `Synchronise` → `Parti` (PeerDown, ou PeerUp d'une autre époque). Un
message du pont dont l'époque n'est pas celle du dernier PeerUp est jeté (règle T) : un
message tardif d'une ancienne session ne passe donc jamais, même si sa génération coïncide.

## 12. Ordre de vérification des octets non fiables

Chaque saut a **un seul point d'entrée**, qui applique les étapes dans cet ordre ; une
entrée qui échoue n'atteint jamais l'étape suivante.

| Étape | Réseau → Go (lecteur du flux) | Pont → webview (adaptateur, `@inkwell/protocol`) |
|---|---|---|
| 1. qui | clé valide (§3.2) ; membre, ou fenêtre d'invitation pour le salon seul ; limites de ressources libp2p | pair membre annoncé ; génération du message = génération courante (règle T) |
| 2. flux | identifiant de protocole (multistream-select), majeure de la session, nombre de flux (gestionnaire de ressources), préambule et règle A (§5.1, §7.3) | — |
| 3. taille | `length` ≤ borne, avant allocation ; LARGE : `size` réservée | taille ≤ borne du type (§16.2) |
| 4. droit de parler | type admis sur ce flux, dans cet état (§11.1), dans l'ensemble effectif | type admis (§9.2.3) |
| 5. place | file du flux : sinon on cesse de lire (REPO), on remplace (PRESENCE), on diffère (réponses CONTROL) | budgets du pair (§9.3, §9.5) |
| 6. forme | décodage strict (§1.3) | profil CBOR (§9.2.1), puis schéma et contexte (§9.2.3) |
| 7. preuve | signatures (§3.3) des objets, attestations, tuples d'auteur ; projet | — (fait par Go) |
| 8. admission | standing dans la liste (§4.9) ; journal ; ensemble admis (§4.10) | appareil membre ; document admis (§4.10) |
| 9. couverture | — | T10 (§9.4), attente bornée et réconciliation (§6.2) |
| 10. garde | — | G1–G12 (§9.7) |
| 11. effet | stocker, relayer, mettre en file avec sa génération | remettre au `Repo` |

## 13. Erreurs et réinitialisations

| Portée | Cause | Effet | Reprise |
|---|---|---|---|
| objet | objet signé bien formé mais refusé (auteur inconnu, rôle, ancien) ; modification refusée par la couverture définitive ou la garde | ignoré ou jeté, compté ; **pas** de règle L | anti-entropie |
| objet (vrac) | Bao invalide, taille fausse, réponse non sollicitée fautive | faute du pair (3 → 10 min sans l'interroger) | autre détenteur |
| génération | règle L (§5.5) : perte en transport, demande de la webview | règle I (§7.3) | nouvelle génération, Automerge resynchronise |
| document | règle H (§5.5) : message reçu mais non remis (refus, attente expirée, file pleine) | refus noté par (pair, document) | `history-request` (§9.6), au plus 1 par minute |
| flux | flux REPO ancien, flux en état Restreint | flux réinitialisé sans sanction | — |
| connexion | **toute trame ou tout préambule mal formé, sur tout flux** (sync, vrac) ; type interdit ; borne dépassée ; HELLO faux ou absent ; règle A violée ; trop de flux ; octet sur un flux ouvert | GOAWAY PROTOCOL_VIOLATION (ou code précis), fermeture, Tenu | après l'attente |
| connexion | cadence dépassée (§7.3), inondation | GOAWAY OVERLOADED, fermeture, Tenu | après l'attente |
| membre | retrait | fermeture, connexion refusée ensuite | réadmission |
| membre | suspension | Restreint | reprise |

Règle de portée : une **forme** fausse est une violation de la connexion (un pair honnête
n'en émet jamais) ; un **contenu** faux venu d'ailleurs (un blob relayé) est une faute ; une
**perte** est une génération ; un **refus** n'est rien de plus qu'un refus. GOAWAY est une
courtoisie : le récepteur **NE DOIT PAS** en dépendre.

## 14. Persistance et atomicité

1. **Remplacement atomique** : fichier temporaire du même dossier, `fsync`, renommage,
   `fsync` du dossier ([ADR 0020], `store.WriteAtomic`) ; un ensemble de fichiers se prépare
   dans `next/` et s'installe par renommage.
2. **Durable avant diffusé** : un objet signé localement (op, décision, accusé, annonce,
   attestation à retrouver, `epoch`) **DOIT** être persisté avant sa première émission.
3. **Vérifié avant stocké** : rien de ce qu'envoie un pair n'est écrit avant l'étape 8 du §12.
4. **Revérifié au chargement** : recharger un objet stocké repasse le décodage strict, la
   validation des clés (§3.2) et la signature ; un fichier qui échoue est mis en quarantaine
   (`.bad`), jamais effacé.
5. **Chargement = pli** : recharger un ensemble donne le même état quel que soit l'ordre.
6. **Caches** : sous le cache utilisateur (`inkwell/…`), `0700`, confinés par `os.Root`,
   nettoyés à la fin, à l'abandon et après 24 h ; jamais dans un `/tmp` en mémoire.
7. **Attente des modifications indivisibles** (§8.1) : sous le dossier de données du projet
   (`.inkwell/staging/<root>`, avec la liste de ses détenteurs et la carte des groupes vérifiés écrite par
   remplacement atomique), `0700`, `os.Root` ; **pas** un cache : conservée à travers les
   redémarrages et sans limite de durée, retirée seulement quand la modification est obtenue ou
   n'est plus voulue, ou, au plafond global de 1 Gio, si elle est inactive (§8.1) ; ces règles priment le point 6. Une reprise après plus de 24 h est
   testée (porte P-BORNES).

## 15. Propriété (informatif : organisation retenue par inkwell)

| État | Propriétaire unique | Accès des autres |
|---|---|---|
| connexions, flux, état §11.1 | `internal/transport/p2p` sur `protocol/libp2p` | instantané immuable publié atomiquement |
| files d'un pair, génération §7.3, curseurs §6.1 | acteur du pair (`internal/transport/session`, machines pures de `protocol/session`) | dépôt non bloquant |
| liste, journal, accusés, annonces, preuves, persistance | acteur `Wiring` | lecture concurrente (verrou court) |
| attestations gardées | magasin concurrent (parts par signataire) | lecteurs de flux (écriture), routeur (lecture) |
| objets LARGE, blobs, tirages | `blob.Service` et l'acteur du pair | API non bloquante |
| livraison à la webview, crédits du pont | `bridge.Router` | état coalescé |
| état Automerge, admission, couverture, garde | webview (`@inkwell/protocol`, adaptateur) | — |

Une goroutine partagée n'appelle jamais une écriture bloquante d'un pair ; elle dépose dans
une file bornée et continue.

## 16. Table des bornes

Chaque ligne donne une borne **normative** (sauf « impl. » : valeur d'inkwell, qu'une autre
implémentation peut choisir plus stricte sans gêner un pair honnête), sa part réservée
(§10.6), le comportement à la borne et la reprise. Les bornes sont des constantes exportées
de `protocol` ; un test par ligne vérifie la borne, la borne + 1 et la reprise.

### 16.1 Réseau

| Objet | Borne | Part réservée | À la borne | Reprise |
|---|---|---|---|---|
| connexions | 2 par pair (passage) ; 128 entrantes, 128 sortantes au total (impl., 64 membres × 2) | — | refusée | à la fermeture d'une autre |
| flux par pair et par sens (gestionnaire de ressources libp2p, portées *protocol* × *peer*) | control 1, repo 1 (+1 en fermeture), presence 1, large 1 non lu ; vrac 1 ; salon 4 | — | violation (salon : réinitialisé) | Tenu |
| trame sync | 98 304 o | — | violation | Tenu |
| trame vrac | 1 065 005 o | — | violation | Tenu |
| salon | requête 4096 o / 10 s ; réponse 262 144 o ; traitement 3 min | — | flux fermé | nouvelle requête |
| HELLO | 1024 o ; 10 s | — | GOAWAY | Tenu |
| file d'entrée REPO d'un pair | 4 Mio et 1024 trames (impl.) | par pair | lecture du flux suspendue | dès que la webview consomme |
| file d'entrée CONTROL d'un pair | 512 Kio (impl.) | par pair | lecture suspendue **seulement** le temps que l'acteur `Wiring` traite (CPU et disque locaux) ; jamais à cause du réseau ni de la webview — « toujours lu » au sens de §5.1 (a) | immédiate |
| flux annulés en attente de RESET_STREAM | 1 par session ; 10 s | par session | GOAWAY PROTOCOL_VIOLATION, session fermée | reconnexion |
| modifications indivisibles en attente (disque, §14 point 7) | par `root` ; **un emplacement de 16 Mio par pair connecté** ; active ⇔ occupe l'emplacement d'un pair connecté qui la détient ; attribution seulement dans un emplacement libre, réservation atomique, **à l'entrée éligible de plus petit rang d'attente** (FIFO, persisté) avant toute nouvelle annonce ; rangs : ≤ 64 par pair ; **plafond global 1 Gio, pairs absents compris** (retrait : finies ou plus voulues, puis inactives en LRU ; jamais une active) ; jamais remplacé, jamais sur un délai ; quota N10 de l'auteur crédité après vérification | par pair annonceur | l'annonce suivante du même pair attend ; la règle H continue auprès des autres détenteurs | reprise des groupes vérifiés, même après 24 h |
| index des modifications indivisibles | entrées vérifiées seulement ; ≤ 1 par ≈ 4 Mio d'historique stocké ; retirée avec le document ou la modification | par auteur (N10) | — | — |
| annonce indivisible | ≤ 16 777 216 octets (G1) | — | violation | — |
| flux LARGE | ≤ `r` = 4 194 304 octets ; une annonce ordinaire plus grande n'est qu'un avis (`LARGE_REPLY` 0, pages) | — | violation si un flux dépasse `r` | pages d'historique |
| PRESENCE par pair | 16 entrées, dernière par `sessionId` | par pair | la plus ancienne remplacée | — |
| file de sortie CONTROL d'un pair | 1 Mio (impl.) | par pair | curseurs SET différés ; réponses ATTEST_WANT ignorées ; commérage ATTEST jeté | tour suivant (15 s) |
| curseurs SET | 1 par (pair, set) ; 2 nouveaux par 15 s | par pair | demande ignorée | tour suivant |
| ATTEST_WANT | 64 demandes émises et 64 servies par pair et par 15 s ; attente 30 s | par pair | demande non émise / ignorée | 15 s ; règle H à 30 s |
| sortie dépôt d'un pair (pont) | voie en ligne 4 Mio ; voie LARGE 1 objet | par pair | la webview attend | crédit rendu |
| objets LARGE annoncés (émetteur) | 1 par pair, en attente de `LARGE_REPLY`, ≤ 10 min (dans la voie LARGE de la webview) | par pair | oublié au refus ou après 10 min ; état de synchronisation de ce document réinitialisé | règle H chez le receveur |
| objets LARGE réservés (receveur) | 1 par pair ; ≤ part fixe `r` = 4 Mio ; plus grand : refus (`LARGE_REPLY` 0), ou vrac si indivisible | 4 Mio (fixe) | refus | pages d'historique ≤ `r` (§9.6) |
| sessions par pair | 2 pendant un passage ; limites du gestionnaire de ressources = 2 × celles d'une session | — | la plus ancienne fermée | — |
| fenêtres QUIC | maximum de connexion ≥ maximum de flux + fenêtre initiale de flux + 1 Mio (go-libp2p v0.51.0 et quic-go v0.64.1 : 15 ≥ 10 + 0,5 + 1 Mio ; initiales 512 / 768 Kio) | — | démarrage refusé (porte P-QUIC) | — |
| trames ignorables | 256 trames et 256 Kio par minute et par pair | par pair | OVERLOADED | — |
| progrès d'un flux LARGE | 60 s sans octet alors que réservé et lu | — | flux réinitialisé | règle L |
| génération | engagements et réessais 1 / 2 s par pair, doublé jusqu'à 60 s (mémoire 10 min) ; flux REPO entrants ≤ 2 / 2 s ; `gen ≤ 2^32 − 1` | — | retardé ; OVERLOADED ; fin de session (règle W) | — |
| vrac (demandeur) | 1 demande par (pair, hash), 2 par pair ; 30 s sans octet ; 3 échecs par plage ; 5 BUSY de suite | par pair | échec | autre détenteur |
| vrac (serveur) | 2 par pair ; `max(8, 2n)` au total ; 32 BUSY par pair et par minute ; 3 fautes → 10 min | ≥ 1 par pair | BUSY, puis silence | délai |
| transferts de blobs | 4 à la fois (impl.) | — | file d'attente | — |
| connexion réinitialisée | attente 2 s, doublée jusqu'à 60 s (mémoire 10 min) | — | refusée | après l'attente |
| liste des membres | 1024 ops (scellée) ; `have` ≤ 256 têtes | — | refus (scellée) | nouveau projet |
| objets en attente de parents | 256, 2 périodes | par pair livreur | oubli | tour suivant |
| journal de décisions | 8192, 1024 par auteur | par auteur | refus | compaction (S2-R2) |
| annonces | `Q* + 2` par auteur (ADR 0031) ; attente L2 : 4 par auteur | par auteur | refus ; plus ancienne de l'auteur évincée | — |
| preuves | 1 par (key, epoch), 4 par key | par clé | ignorée | — |
| op / attestation / décision / accusé / annonce | 4096 / 4096 / 8192 / 8192 / 4096 o ; 64 parents ; 64 têtes ; 96 entrées | — | refus | — |
| attestations gardées | par (doc, signataire) : acceptée si elle apporte une tête nouvelle, éviction par plus petit `seq` au-delà de 256 têtes distinctes ; 65 536 clés et 64 Mio au total (impl. ≥) | ⌊C / signataires debout⌋ | reprise (§10.6) | réconciliation |
| attestations en attente de leur signataire | 2 périodes | par pair livreur | oubli | réconciliation |
| refus notés (règle H) | 1 drapeau par (pair, document admis) ; `history-request` ≤ 1 par (pair, document) et par 60 s | par pair | — | page d'historique |
| ticket | 1024 caractères ; 15 min ; usage unique | — | refus | nouveau ticket |

### 16.2 Webview (couche dépôt)

| Objet | Borne | Part réservée | À la borne | Reprise |
|---|---|---|---|---|
| profil CBOR | profondeur 8 ; 4096 éléments ; map 64 ; tableau 256 ; entiers ≤ 2^53 − 1 | — | refus | — |
| poignée de main | 1024 o ; 3 `arrive` puis 1 / 20 s | par pair | jetée | attente |
| identifiants | PeerID, DocumentId ≤ 64 ; `projectUrl` ≤ 80 | — | refus | — |
| éphémère | `data` ≤ 1024 o ; 128 en rafale, 64/s | par pair | jeté | seau |
| attente document non prêt | 4 par document, 8 Mio par pair | par pair | refus, règle H | page d'historique |
| attente de couverture | 4 messages par document et par pair, 8 Mio par pair, 30 s | par pair | refus, règle H | idem |
| attente de tuple d'auteur (G12) | 4 par auteur, 30 s | par auteur | refus, règle H | idem |
| attente de dépendances (G2) | quota par émetteur, ses plus anciennes évincées | par pair | refus, règle H | idem |
| couverture | 256 têtes par (doc, signataire) ; 1024 documents par signataire ; 10 000 documents | par signataire | les plus anciennes oubliées | réconciliation |
| garde | ≤ 65 536 opérations et 16 Mio décompressés par modification ; Worker 256 Mio | — | refus (avant Automerge) | pages d'historique |
| file de sortie d'un pair | **au plus 1 message en attente de crédit par (pair, document)** ; au plus 64 documents en synchronisation initiale par pair (les autres entrent à mesure) ; 16 Mio (impl.) | par pair | un nouveau message pour un (pair, document) qui en a déjà un en attente le **remplace**, et l'état de synchronisation Automerge de ce seul (pair, document) est réinitialisé (correctif de cycle de vie, ADR 0031) ; au-delà de 16 Mio, plus aucun document n'entre en synchronisation ; **jamais de règle L** | à mesure des crédits |

### 16.3 Pont interne (Annexe A)

| Objet | Borne | À la borne |
|---|---|---|
| enveloppe `net` | 64 Mio (limite Wails) ; pair ≤ 64 o | refus |
| crédit par (pair, époque, voie) | en ligne : fenêtre 4 Mio ; LARGE : 1 objet | violation : session du pont fermée, la webview se recharge |
| présence | pas de crédit ; 16 par pair coalescées côté Go | la plus ancienne remplacée |
| notifications d'état (rôles, attestations, suspension, ensemble admis) | coalescées : dernière valeur par clé | — |
| `store` | 1026 voies ; page ≤ 1 Mio ; valeur ≤ 32 Mio ; clé ≤ 4 composants de 128 o | refus |
| `preview` | requête ≤ 1 Kio ; 40 pages ; trame ≤ 48 Mio | refus |
| `present` | trame ≤ 8 Mio ; 6 fenêtres ; 256 trames / 32 Mio par fenêtre | trame suivante jetée |

## 17. Considérations de sécurité

- **Modèle de menace** (N7) : n'importe quel membre, y compris un appareil qui écrit ou un
  admin retiré, peut être malveillant ; il ne doit jamais gêner les autres. Les inconnus ne
  parlent qu'au salon, pendant une fenêtre d'invitation. Les admins en place sont de confiance
  ([ADR 0017], résiduel R7 de l'[ADR 0031]).
- **Authentification** : TLS 1.3 (QUIC) ou Noise (TCP) lie chaque connexion à une clé
  Ed25519 ; le relais voit qui parle à qui et combien, jamais le contenu. Sans la validation
  du §3.2, une clé d'ordre petit enregistrée comme membre rend son identité publique (N-1).
- **Autorisation** : la liste signée (§4.9) décide qui se connecte et avec quel rôle ; les
  objets signés sont rejouables mais idempotents ; une attestation compte tant qu'elle est
  gardée (§6.2, au plus 256 têtes par document et par signataire) et que son signataire écrit
  encore. L'écriture exige la couverture (§9.4),
  l'auteur exact de chaque modification (G12) et la garde (§9.7) ; aucune de ces vérifications
  ne se négocie (§5.4).
- **Épuisement** : chaque file, carte et octet disque a une borne et un propriétaire (§16) ;
  chaque plafond partagé a des parts réservées (§10.6) ; l'attente des modifications
  indivisibles est plafonnée à 1 Gio sur disque, pairs absents compris, avec un emplacement
  garanti par pair connecté (§8.1) ; rien n'est lu sans crédit (§10.2) ;
  aucune boucle partagée n'attend un pair ; aucune combinaison de files pleines ne bloque
  (§10.7).
- **Résiduel de volume (Majeur jusqu'au quota N10)** : un appareil qui écrit peut produire
  beaucoup de modifications **valides** (couvertes, signées, conformes à la garde). Un CRDT
  byzantin garantit la convergence, pas la résistance au volume : chaque pair paie le CPU de
  la garde et le stockage. Le DRR imputé à l'auteur (§10.4) borne sa part de CPU ; le quota
  disque cumulé par personne qui écrit (exigence N10, ADR 0031 « Décision de l'utilisatrice »)
  bornera le stockage ; d'ici là, la détection et l'alerte de D-3 (256 Mio/jour par auteur)
  s'appliquent. L'auto-équivoque (R1) est un résiduel Critique accepté.
- **Rétrogradation** : impossible par un tiers (HELLO authentifié) et sans effet par un pair
  (§5.4).
- **Horloges** : l'expiration du ticket et le `seq` des attestations (millisecondes Unix)
  dépendent de l'horloge locale ; un appareil **DEVRAIT** prendre `max(dernier seq connu + 1,
  maintenant)`.
- **Secrets** : le secret du ticket est comparé en temps constant, à usage unique, jamais
  journalisé ; aucun octet de charge n'est journalisé.

## 18. Considérations de confidentialité

Selon [RFC 6973] :

- **Relais** : il connaît les PeerID et adresses des deux bouts, l'heure et le volume. Le
  mode « relais seulement » cache l'adresse IP aux autres membres.
- **Réseau local** : mDNS (`_inkwell._udp`) annonce le PeerID ; pendant une fenêtre visible,
  `Hello` du salon révèle le nom de la machine et le titre du projet. La découverte locale se
  désactive.
- **Ticket** : il contient le `ProjectID`, le PeerID et les adresses de l'hôte ; il se
  partage comme un mot de passe. Le serveur Wormhole public voit un code et un horaire.
- **Entre membres** : HELLO révèle le `ProjectID` ; attestations et tuples d'auteur révèlent
  l'heure et le volume des modifications ; la présence, curseur et nom affiché. Rien ne sort
  du projet.

## 19. Écarts avec le code au 2026-11-01

Les parties *existantes* sont émises telles quelles. Tout le reste est à implémenter selon
l'[ADR 0034] ; l'[inventaire](inventaire.md) relie chaque écart à `fichier:ligne`. Écarts
notables de l'existant : clé non validée (N-1) ; l'adaptateur webview encode avec `encode` de
`cbor-x` sans réglage (tag 64, refusé par §9.2.1) et décode avec `cbor-x`, qui accepte les clés
en double (N-12) ; le pointeur de présentation est en flottants (§9.5) ; `handshake` sans
`targetId`.

## 20. Registres

### 20.1 Types de trame (majeure 2)

`0x00` réservé ; `0x01`–`0x07`, `0x09`–`0x0B` : §5.2 ; `0x08` retiré ; `0x0B`–`0x3F`
(critiques) : à attribuer par une mineure, chacun lié à une capacité et à une classe de flux ;
`0x40`–`0x7F` (ignorables) : à attribuer par une mineure, sans capacité, dont GREASE `0x4A`,
`0x5A`, `0x6A`, `0x7A` ; `0x80`–`0xFF` : réservés à une majeure. Une extension ignorable doit
tenir dans le budget des trames ignorables (§16.1) ; plus bavarde, elle prend un type
critique et une capacité.

### 20.2 Capacités (CAPS) et TLV du HELLO

| id | Capacité | État |
|---|---|---|
| 0x0001 | `core/2` : toute cette spécification | **obligatoire** |
| 0x0002–0x0005 | anciennement `member-op/2`, `policy/1`, `history/1`, `changesig/1` : incluses dans `core/2` | retirées, jamais réattribuées |
| 0x?A?A | GREASE (16 valeurs) | à ignorer |

TLV du HELLO : `0x0001` PROJECT, `0x0002` CAPS, `0x0004` VERSIONS, `0x0008` SESSION_NONCE, `0x?A?A` GREASE ; le reste à
attribuer (pair : ignorable ; impair : à comprendre).

### 20.3 Codes GOAWAY

`0x01` NO_ERROR · `0x02` PROTOCOL_VIOLATION · `0x03` HELLO_TIMEOUT · `0x04` PROJECT_MISMATCH ·
`0x05` NO_COMMON_CAPABILITY · `0x06` NOT_MEMBER · `0x07` OVERLOADED · `0x08`
UNSUPPORTED_CRITICAL · `0x09` VERSION_DOWNGRADE. Un code inconnu se lit comme
PROTOCOL_VIOLATION.

### 20.4 Domaines de signature

`trust-op` (§4.1, §4.2) · `attest` (§4.3) · `policy-op` (§4.4) · `final-ack` (§4.5) ·
`listing-op` (§4.6) · `change` (§4.7) · réservés : `actor`, `recovery`. Un domaine ne contient
ni `/` ni octet nul. (Les dérivations BLAKE3 `inkwell/doc/v1` et `inkwell/actor/v2` ne sont
pas des domaines de signature.)

### 20.5 Espaces de noms des décisions (§4.4)

`mode` (`approval` | `free`) · `main` (hex de 32 caractères) · `doc/<hex32>` et `node/<hex32>`
(`kept` | `trashed` | `purged`) · `quota/<hex64>` (`grant`) · `legacy/<hex32>` (têtes hex64
triées, séparées par des virgules, ≤ 64) · `equiv/<hex32>/<seq décimal ≥ 1>` (hex64) ·
`format` (mineure décimale, ≥ 0 : la plus petite mineure de la majeure 2 que le projet exige,
§1.4.4 ; absente = 0).

### 20.6 Identifiants de protocole

`/inkwell/control/2.0.0` · `/inkwell/repo/2.0.0` · `/inkwell/presence/2.0.0` ·
`/inkwell/large/2.0.0` · `/inkwell/bulk/1.0.0` · `/inkwell/lobby/1.0.0`. `/inkwell/sync/1.0.0`
est retiré (bascule nette). Une nouvelle classe s'ajoute par une mineure, liée à une capacité.

### 20.7 Raisons de refus du salon

`1` Invalid · `2` Declined · `3` Busy · `4` Failed · `5` TooLarge.

## 21. Conformité, vecteurs et modèle

- **Vecteurs.** Les fichiers [vectors/](vectors/) sont normatifs :
  `{spec, spec_version, generator, keys?, vectors: [{id, status, enforce?, description,
  decoder, context?, input_hex | inputs_hex | (input_sha256, recipe), verdict, expect?, current_go?}]}`. Un vecteur trop gros pour être recopié (≥ 4 Mio) donne l'empreinte SHA-256 de son entrée et la recette exacte pour la régénérer avec le générateur. `verdict` : `accept` ou
  `reject` ou `ignore` (jeté sans erreur ni sanction) ; `expect` : ce qu'une implémentation
**DOIT** reproduire ; `context` : l'état du
  receveur dont le verdict dépend (pair authentifié, receveur, ensemble admis, génération et
  mode, annonces lues) ; `status` : `normative` (octets déjà émis par inkwell) ou `proposed` ;
  `enforce` : l'étape de migration de l'[ADR 0034] à partir de laquelle les exécuteurs
  l'exigent (avant, ils l'exécutent et signalent sans échouer) ; `current_go` : comportement
  actuel quand il diffère.
- **Générateurs.** `vectors/generate.go.txt` (Go, vrais codecs : `trust`, `blob`, `invite`,
  `policy`, `bridge`, `crypto/ed25519`, `filippo.io/edwards25519`) et
  `vectors/generate-repo.cjs.txt` (Node : `cbor-x` 1.6.7, `@automerge/automerge` 3.5.0) ; les
  vecteurs `proposed` d'un format qui n'a pas encore de codec sont produits par des encodeurs
  écrits d'après cette spec, dans ces mêmes fichiers. Les cas `speccheck-*` viennent de
  `novifinancial/ed25519-speccheck` (`cases.json`, SHA-256 `08e47a36…3540450`).
- **Décodeurs nommés** : `public_key`, `signature`, `signature+key`, `peer_id`, `trust_op`,
  `trust_op+apply`, `trust_sync`, `attestation`, `attestation+verify`, `policy_op`, `bulk`,
  `bulk+expect`, `ticket_text`, `lobby_request`, `lobby_reply`, `sync2_stream` (dont
  `compat.json`), `trust_fold` (`fold.json`), `author_batch+verify`, `evidence+verify`,
  `listing_op+verify`, `final_ack+verify`, `listing_admit` (`objects.json`), `guard_schema`,
  `repo_message`, `bridge_envelope` (interne).
- **Matrice de compatibilité** (N14) : `compat.json` (session) et `repo.json#compat-*`
  (couche dépôt) disent ce qu'un décodeur 2.0 fait d'octets d'une mineure future ou d'une
  autre majeure (extension ignorée, « à comprendre » refusé, rétrogradation détectée, GREASE
  ignoré) ; à chaque nouvelle mineure, ses propres vecteurs y sont ajoutés avec le verdict
  qu'un décodeur 2.0 **doit** rendre, et un test croisé fait décoder les vecteurs de N+1 par
  le décodeur N (paquets `protocol/sync2` de deux versions du module).
- **Modèles.** [tla/RepoGen.tla](tla/RepoGen.tla) (`RepoGenSmall.cfg` exhaustif, `RepoGen.cfg`
  long) et [tla/ConnCredit.tla](tla/ConnCredit.tla) ; `mise run tla` (TLC épinglé, SHA-256) ;
  c'est une porte de l'ADR 0034 avant la bascule. §7.3 dit exactement ce qu'ils couvrent.
- **Conformité.** Une implémentation est conforme si elle donne le verdict attendu sur tous les
  vecteurs de la version qu'elle annonce, reproduit chaque `expect`, et passe l'exécuteur
  `protocol/conformance` (Go, vecteurs embarqués, interface à implémenter par la cible) ou son
  équivalent `@inkwell/protocol/conformance` (TypeScript). inkwell s'y soumet lui-même : ses
  codecs **sont** ceux de ces paquets.
- Les clés de test sont celles de [RFC 8032 §7.1] (TEST 1 et TEST 2) : jamais de vraies clés.

## 22. Réutilisation des standards (exigence N15)

Règle : on réutilise tel quel tout standard ou bibliothèque compatible ; on adapte quand il
manque une pièce ; on n'écrit une couche à nous que si rien ne convient, avec la raison. À
égalité, le plus simple gagne.

| Mécanisme | Existant examiné | Choix | Justification |
|---|---|---|---|
| Transport, chiffrement, authentification | libp2p : QUIC v1 (TLS 1.3), TCP + Noise + yamux | **tel quel** | déjà retenu (ADR 0002) |
| Typage des flux, négociation de majeure | multistream-select | **tel quel** | un identifiant par classe (§5.1) remplace l'octet de classe de la révision 2.0 ; propose plusieurs majeures (§1.4) |
| Contrôle de flux | crédit par flux de QUIC ([RFC 9000 §4.1]) et de yamux | **tel quel** | un flux par classe et par gros message (revue 02-fable) ; remplace OFFER/WANT/Bao et le cadrage par fragments |
| Changement de génération | réinitialisation de flux (QUIC RESET_STREAM [RFC 9000 §3.5], yamux RST) | **tel quel** | jette atomiquement le contenu ; remplace la trame REPO_RESET |
| Vivacité | ping libp2p, délai d'inactivité de QUIC, maintien de yamux | **tel quel** | remplace les délais d'écriture « 20 s + taille / 16 Kio/s » |
| Joignabilité, NAT | circuit relay v2, DCUtR, AutoNAT v2, mDNS | **tel quel** | — |
| Version du pair trop ancienne / récente | identify (liste des protocoles) | **tel quel** | message à la personne (§1.4.5) |
| Limites de connexions, de flux, mémoire par flux | gestionnaire de ressources de go-libp2p (portées system, peer, protocol, stream) | **tel quel** pour les nombres et les tampons de flux ; **à nous** pour les parts dynamiques (§10.6) | le gestionnaire plafonne mais ne garantit pas de part minimale reprise à la demande ; les attestations ne sont pas une ressource réseau |
| Cadences (générations, ATTEST_WANT, éphémères) | seaux à jetons (`golang.org/x/time/rate`) | **bibliothèque**, valeurs à nous | la valeur est sémantique (§16) |
| Équité entre pairs | Deficit Round Robin ([DRR]) | **algorithme tel quel** | — |
| Synchronisation des documents | automerge-repo 2.x (messages, poignée de main, synchronisation Automerge, éphémères) | **tel quel**, filtré | on ne spécifie que ce qu'il faut pour l'interopérabilité et l'admission (§9) ; la référence des messages est automerge-repo et le CDDL de son adaptateur WebSocket |
| Gros historiques | — (automerge-repo n'a pas de pagination) | **à nous** (extension de deux types, §9.6) | aucun standard ; à retirer si automerge-repo en offre une |
| Encodage, schéma | CBOR [RFC 8949] + CDDL [RFC 8610] (couche dépôt, imposé par automerge-repo) ; `cbor-x` | **tel quel** + profil de validité (§9.2.1) | le profil n'invente rien : il interdit ce que [RFC 8949 §5.3.1, §5.6] déclare invalide ou laisse au choix |
| Trames et préambules | protobuf (libp2p), CBOR | **à nous**, minimal | 3 à 5 champs fixes ; un décodeur binaire strict de 30 lignes, fuzzé, sans dépendance |
| Objets signés | COSE_Sign1 ([RFC 9052], EdDSA [RFC 9053] ; `veraison/go-cose` v1.3) | **garder le binaire fixe** (§4) | les objets existants sont figés (le `ProjectID` est l'empreinte de la genèse, les données stockées en dépendent) ; deux enveloppes coexisteraient, ce qui est moins simple ; chemin de migration prêt : un octet de version `0x84` (en-tête d'un tableau CBOR de 4) désignera sans ambiguïté un COSE_Sign1 non étiqueté |
| Signatures | Ed25519 [RFC 8032] ; séparation de domaine par préfixe (pratique de libp2p) | **tel quel** + règles de validité (§3.2–§3.3) | les vecteurs de [Chalkias 2020] fixent le verdict |
| Liste des membres, rôles | MLS [RFC 9420] ; Keyhive (Ink & Switch, alpha) ; p2panda-auth 0.7 | **garder la liste signée** ; reprendre des idées | MLS exige un ordre total des commits (service de distribution), incompatible avec l'édition hors ligne sans serveur, et traite des clés de groupe, pas des droits ; Keyhive (niveaux read/edit/relay/admin, graphe de délégations, E2EE) est la cible la plus proche mais en alpha et sans implémentation Go ; p2panda-auth (CRDT d'ops, « strong removal ») ressemble à notre pli mais son API n'est pas stable et il est en Rust. À réexaminer quand Keyhive sera stable (le niveau « relay » est une idée à reprendre) |
| Anti-entropie des ensembles signés | réconciliation par intervalles (RBSR, negentropy NIP-77), échange de frontières (Hypercore, SSB) | **frontières** (existant, §6.1) | ensembles ≤ 8192, graphes : la frontière est plus simple ; RBSR si les ensembles plats grandissent |
| Blobs et modifications indivisibles | BLAKE3 + Bao (`lukechampine.com/blake3` v1.4.1), groupes de 16 Kio comme `bao-tree` d'iroh ; bitswap (IPFS) | **tel quel** (format) ; requêtes à nous (`bulk/1.0.0`, existant, étendu aux modifications indivisibles par leur racine, accord D1) | bitswap : blocs et CID SHA-256, plus lourd ; iroh-blobs : hors libp2p |
| Invitation | magic-wormhole (ADR 0032) | **tel quel** | — |
| Ticket | tickets iroh | **à nous** (existant) | pas de standard pour libp2p ; format figé (§8.3) |
| Versionnement, extensibilité | [RFC 6709], [RFC 9413], [RFC 8701], [RFC 9368] | **méthodes reprises** | §1.4 |
| Modèle formel | TLA+ / TLC | **tel quel** | §7.3 |
| Crédits du pont Go ↔ webview | — (les Streams Wails n'ont pas de contrôle de flux) | **adapté** de [RFC 9000 §4.1] | interne |

Ce qui a été **remplacé par un standard** dans cette révision : OFFER/WANT/Bao pour les
messages et le cadrage par fragments (→ un flux libp2p par classe et par gros message), la
trame REPO_RESET et son renvoi (→ réinitialisation de flux), l'octet de classe (→
multistream-select), les délais d'écriture au débit (→ ping et délais d'inactivité du
transport), les compteurs de flux par pair (→ gestionnaire de ressources), le profil
déterministe envisagé (→ validité de [RFC 8949 §5.3.1]).

## 23. Références

- [RFC 2119], [RFC 8174] : mots-clés BCP 14.
- [RFC 3552], [RFC 6973] : sécurité, confidentialité. [RFC 4648] : base32.
- [RFC 7301] : ALPN. [RFC 9113] : HTTP/2 (§5.2.1 contrôle de flux des seules trames DATA,
  §6.5 SETTINGS).
- [RFC 8032] : EdDSA (§5.1.3 décodage d'un point, §5.1.7 vérification, §7.1 vecteurs).
- [RFC 8610] : CDDL. [RFC 8949] : CBOR (§4.2.1 encodage déterministe, §5.3.1 validité de base, §5.6
  clés en double).
- [RFC 8701] : GREASE. [RFC 9000] : QUIC (§3.5 réinitialisation de flux, §4.1 crédit par flux,
  §15 versions). [RFC 9368] : négociation de version compatible de QUIC. [RFC 6709] :
  considérations de conception des extensions.
  [RFC 9413] : protocoles robustes.
- [Chalkias 2020] : K. Chalkias, F. Garillot, V. Nikolaenko, *Taming the many EdDSAs*, SSR
  2020 ; vecteurs <https://github.com/novifinancial/ed25519-speccheck>.
- [DRR] : M. Shreedhar, G. Varghese, IEEE/ACM ToN 4(3), 1996.
- [Bao] : J. O'Connor, spécification Bao ; `lukechampine.com/blake3/bao` v1.4.1 ; `bao-tree`.
- [RFC 9052], [RFC 9053] : COSE. [RFC 9420] : MLS.
- Keyhive : <https://www.inkandswitch.com/keyhive/notebook/>, <https://automerge.org/docs/keyhive/ark-api-guide/> ; p2panda-auth : <https://docs.rs/p2panda-auth> ; gestionnaire de ressources : <https://github.com/libp2p/go-libp2p/blob/master/p2p/host/resource-manager/README.md> ; go-cose : <https://github.com/veraison/go-cose> ; CDDL des messages automerge-repo : <https://www.npmjs.com/package/@automerge/automerge-repo-network-websocket>.
- [TLA+] : L. Lamport, *Specifying Systems* ; TLC (tla2tools 1.8.0).
- [Automerge format] ; messages automerge-repo ; libp2p specs ; [Wormhole].
- [ADR 0005], [ADR 0017], [ADR 0018], [ADR 0020], [ADR 0021], [ADR 0023], [ADR 0028],
  [ADR 0030], [ADR 0031], [ADR 0032], [ADR 0034].

[RFC 2119]: https://www.rfc-editor.org/rfc/rfc2119
[RFC 8174]: https://www.rfc-editor.org/rfc/rfc8174
[RFC 3552]: https://www.rfc-editor.org/rfc/rfc3552
[RFC 6973]: https://www.rfc-editor.org/rfc/rfc6973
[RFC 4648]: https://www.rfc-editor.org/rfc/rfc4648
[RFC 7301]: https://www.rfc-editor.org/rfc/rfc7301
[RFC 9113]: https://www.rfc-editor.org/rfc/rfc9113
[RFC 9113 §5.2.1]: https://www.rfc-editor.org/rfc/rfc9113#section-5.2.1
[RFC 9113 §6.5]: https://www.rfc-editor.org/rfc/rfc9113#section-6.5
[RFC 8032]: https://www.rfc-editor.org/rfc/rfc8032
[RFC 8032 §5.1.3]: https://www.rfc-editor.org/rfc/rfc8032#section-5.1.3
[RFC 8032 §5.1.7]: https://www.rfc-editor.org/rfc/rfc8032#section-5.1.7
[RFC 8032 §7.1]: https://www.rfc-editor.org/rfc/rfc8032#section-7.1
[RFC 8610]: https://www.rfc-editor.org/rfc/rfc8610
[RFC 8949]: https://www.rfc-editor.org/rfc/rfc8949
[RFC 8949 §4.2.1]: https://www.rfc-editor.org/rfc/rfc8949#section-4.2.1
[RFC 8949 §5.3.1]: https://www.rfc-editor.org/rfc/rfc8949#section-5.3.1
[RFC 8701]: https://www.rfc-editor.org/rfc/rfc8701
[RFC 9000]: https://www.rfc-editor.org/rfc/rfc9000
[RFC 9000 §3.5]: https://www.rfc-editor.org/rfc/rfc9000#section-3.5
[RFC 9000 §4.1]: https://www.rfc-editor.org/rfc/rfc9000#section-4.1
[RFC 9413]: https://www.rfc-editor.org/rfc/rfc9413
[RFC 9368]: https://www.rfc-editor.org/rfc/rfc9368
[RFC 9052]: https://www.rfc-editor.org/rfc/rfc9052
[RFC 9053]: https://www.rfc-editor.org/rfc/rfc9053
[RFC 9420]: https://www.rfc-editor.org/rfc/rfc9420
[RFC 6709]: https://www.rfc-editor.org/rfc/rfc6709
[RFC 9000 §15]: https://www.rfc-editor.org/rfc/rfc9000#section-15
[Chalkias 2020]: https://eprint.iacr.org/2020/1244
[DRR]: https://doi.org/10.1109/90.502236
[Bao]: https://github.com/oconnor663/bao/blob/master/docs/spec.md
[TLA+]: https://lamport.azurewebsites.net/tla/book.html
[Automerge format]: https://automerge.org/automerge-binary-format-spec/
[Wormhole]: https://magic-wormhole.readthedocs.io/
[ADR 0005]: ../adr/0005-trust-model-v1.md
[ADR 0017]: ../adr/0017-member-list-seniority.md
[ADR 0018]: ../adr/0018-signed-heads-write-control.md
[ADR 0020]: ../adr/0020-storage-in-go-store-stream.md
[ADR 0021]: ../adr/0021-suspension.md
[ADR 0023]: ../adr/0023-projet-multi-fichiers-blobs.md
[ADR 0028]: ../adr/0028-performance-huge-files-and-wan.md
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
| 1 | PeerUp | Go → webview | `session u32 ; generation u32` : début d'une époque §7.3 |
| 2 | PeerDown | Go → webview | `session u32 ; generation u32` : fin de cette époque (un PeerDown, un Credit ou un Message d'une autre époque que la courante est ignoré) |
| 3 | Message | les deux | `session u32 ; generation u32 ; message` (REPO ou objet LARGE) |
| 4 | Attest | webview → Go | `Heads` : `doc str16 (1–256) ; têtes [32] × ≤ 64` |
| 5 | Attested | Go → webview | `Heads` d'un signataire `peer` valable |
| 6 | Revoked | Go → webview | vide : les attestations de `peer` sont caduques |
| 7 | Sealed | Go → webview | vide : liste pleine |
| 8 | Roles | Go → webview | `n u16 ≤ 1024 ; n × (peer str8 ; role u8 1–3 ; flags u8 bit 0 suspendu)` |
| 9 | Suspended | Go → webview | `u8` : 1 suspendu, 0 repris ; `peer` = admin auteur |
| 10 | Decide | webview → Go | intention d'admin |
| 11 | Policy | Go → webview | état plié des décisions |
| 12 | Credit | les deux | `session u32 ; generation u32 ; lane u8 (0 en ligne, 1 LARGE) ; consumed u64 ; window u32` |
| 13 | Presence | les deux | message `ephemeral` (sans crédit) |
| 14 | Reset | webview → Go | `session u32 ; generation u32` : la webview demande la règle I |
| 15 | Sign | webview → Go | lot de (`seq`, `hash`) à signer pour un `epoch` (§4.7) |
| 16 | Authored | Go → webview | tuples d'auteur vérifiés (§4.7) |
| 17 | Listed | Go → webview | ensemble admis (§4.10), avec projet et époque |
| 18 | WantAttest | webview → Go | `doc str16` : demander ATTEST_WANT au pair `peer` (§6.2) |
| 19 | Advice | webview → Go | `session u32 ; generation u32 ; size u32 ; doc str16 (1–64)` (`bridge-internal.json#bridge-advice`) : annoncer un avis (§7.2) sans en envoyer les octets |

- **Crédits** : pour chaque (pair, génération, voie), l'émetteur compte les octets de
  `message` envoyés (`sent`) ; le receveur annonce `consumed` et `window`. Voie 0 :
  `sent + taille ≤ consumed + window`. Voie 1 : un objet à la fois (`sent = consumed` avant
  le suivant), quelle que soit sa taille. Un Credit d'une autre génération ou dont `consumed`
  recule est ignoré ; une voie inconnue est refusée (`bridge-internal.json#bridge-credit-bad-lane`).
  « Consommé » : côté Go → webview, remis au `Repo` après la garde ; côté webview → Go, écrit
  sur le flux du pair.
- **Époque** : un Message dont `(session, generation)` n'est pas celui du dernier PeerUp est
  jeté des deux côtés (règle T, `#bridge-message-gen`).
- **Ordre** : avant de livrer un Message, le routeur envoie les notifications d'état en
  attente (Roles, Attested, Revoked, Suspended, Listed, Authored), coalescées.
- **Validation tardive** : `Decide` et `Policy` sont validés par leur gestionnaire, après
  l'aiguillage par `kind` et le contrôle de taille.

### A.2 Stream `store` ([ADR 0020])

v1 : requêtes `id u32 ; op u8 ; nkey u8 ; clé ; data`, réponses `id u32 ; status u8 ;
payload` ; v2 ajoute voies, générations, Cancel/Released, pages ≤ 1 Mio. La version **DOIT**
se choisir une fois, par une première trame à l'ouverture, et non par un préfixe magique.

### A.3 Streams `preview` et `present`

`preview` : requêtes JSON ≤ 1 Kio, réponses ≤ 48 Mio. `present` : trames JSON ≤ 8 Mio dont
`type` est un mot de 1 à 16 lettres minuscules, relayées entre les fenêtres d'une même machine
(≤ 6), 256 trames / 32 Mio en attente par fenêtre, la suivante jetée au-delà.
