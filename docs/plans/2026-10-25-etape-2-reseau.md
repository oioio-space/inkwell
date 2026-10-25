# Étape 2 — Réseau : plan d'implémentation

> **Pour les agents :** sous-skill requis : `superpowers:subagent-driven-development` ou `superpowers:executing-plans`. Étapes en cases à cocher (`- [ ]`). Modèle par défaut : Sonnet (`go-dev`). Revue Opus pour T6 et T10.

**Objectif :** hors ligne puis retour, sur deux réseaux différents, sans perte. Sur **un seul PC**, ce critère se démontre par trois choses. D'abord une simulation déterministe en Go (simnet + synctest). Ensuite un e2e sans root : relais sur boucle locale, que l'on tue puis relance. Enfin une topologie de namespaces réseau avec NAT nftables. Celle-ci tourne **sans root** sur Fedora (`unshare -Urnm`, vérifié sur cette machine le 25/10/2026) et en CI Ubuntu.

**Architecture :**
- Le transport TCP de l'étape 1 est remplacé par `transport/p2p`, basé sur go-libp2p : QUIC-v1 d'abord, TCP+Noise en secours, relais circuit v2, DCUtR, mDNS.
- Un `mux` découpe le transport en canaux : `repo` (opaque, vers le pont), `trust` (liste de membres) et `attest` (têtes signées).
- La clé privée ne sort jamais du Go. Go vérifie la qualité de membre et les signatures. La webview vérifie que chaque modification reçue est couverte par une attestation d'écrivain, car elle seule sait lire Automerge.
- Le stockage automerge-repo passe en Go (Stream `store`), dans un dossier projet sous `os.Root`.

**Stack :**
- `github.com/libp2p/go-libp2p v0.51.0` (23/10/2026, quic-go v0.64.1, exige Go ≥ 1.26)
- `github.com/psanford/wormhole-william v1.0.8`
- `github.com/fsnotify/fsnotify v1.10.1`
- `golang.org/x/crypto/argon2`
- tests uniquement : `github.com/marcopolo/simnet v0.0.7` et `go-libp2p/x/simlibp2p`
- frontend : aucune nouvelle dépendance (`decodeSyncMessage` et `decodeChange` existent déjà dans `@automerge/automerge` 3.5)

**Spécification :** ADR 0004, 0005, 0009, 0011, `docs/engineering-principles.md`, `docs/glossary.md`, PROGRESS § Étape 2.

## Contraintes globales

- Seul `internal/transport/p2p` importe `github.com/libp2p/...`. Le binaire relais appelle `p2p.RunRelay`.
- Seul `internal/invite` importe `wormhole-william`. Aucun code Go n'importe Automerge.
- Un test d'architecture (`go list -deps`) le vérifie en CI.
- La clé Ed25519 privée ne quitte jamais le processus Go. Elle ne va ni dans la webview, ni dans les logs, ni dans un rapport.
- Chaque décodeur d'octets venus d'un pair ou de la webview a un `Fuzz…`. Liste : ticket, op de confiance, attestation, messages lobby, protocole du Stream `store`, nouveaux kinds d'enveloppe.
- E/S disque uniquement via `os.Root` (Go 1.25+ : `Rename`, `MkdirAll`, `WriteFile`). Fichiers en `0o600`, dossiers en `0o700`. Pas de `renameio` : `os.Root.Rename` suffit.
- Concurrence sous `testing/synctest` partout où c'est possible, et `goleak` dans chaque `TestMain`.
- Dans l'interface : « personne », « inviter », « rejoindre », « relais », « empreinte », « hors ligne ». Jamais « pair », « NAT », « multiaddr » ni « peer ID » (sauf derrière « Détails techniques »).
- Le canal sécurisé n'est pas « Noise partout » : QUIC-v1 utilise TLS 1.3 natif, TCP utilise Noise. L'architecture dira « chiffrement authentifié de bout en bout (TLS 1.3 sur QUIC, Noise sur TCP) ».
- **Un seul PC.** Tout se vérifie en local et en CI. Rien ne suppose une seconde machine ni un GPU.

## Review Focus

1. **Passage relais → direct** (hole punching) pendant la frappe : aucun `PeerDown`, aucune trame perdue, le flux sortant est rouvert sur la connexion directe.
2. **Hors ligne 2 min, puis retour sur un autre réseau** (nouvelle IP) : reconnexion par l'adresse de circuit du relais en ≤ 30 s, un seul `PeerUp`, convergence.
3. **Liste de membres** : révocation concurrente, deux admins qui se retirent l'un l'autre, antidatage. Tous les pairs obtiennent le même résultat, quel que soit l'ordre d'arrivée (test de propriété sur les permutations).
4. **Isolation** : un non-membre ne fait jamais parvenir une trame à la webview. Une modification forgée par un lecteur n'atteint jamais le `Repo`.
5. **Arrêt brutal** (`kill -9`) entre l'écriture temporaire et le `Rename` : on retrouve l'ancienne ou la nouvelle valeur, jamais une valeur tronquée. Un bit inversé est détecté et mis en quarantaine.
6. **Ticket rejoué, expiré ou d'un autre projet** : refusé, comparaison en temps constant, rien de journalisé.

---

## Structure des fichiers

| Fichier | Rôle |
|---|---|
| `internal/transport/transporttest/suite.go` | Contrat comportemental partagé (memnet, mux, p2p) |
| `internal/transport/frame/{frame,budget}.go` | Trames bornées et budget d'octets, **déplacés** de `tcp` (et leurs tests/fuzz) |
| `internal/transport/mux/mux.go` | Découpe un `Transport` en sous-transports par octet de canal |
| `internal/transport/p2p/{node,stream,dial,gate,lobby,mdns,relay,keys}.go` | go-libp2p derrière `transport.Transport` |
| `internal/identity/{device,fingerprint}.go`, `wordlist/{fr,en}.txt` | Clé de la machine, empreinte en 4 mots (BIP39, MIT) |
| `internal/trust/{op,list,attest,sync}.go` | Liste signée (DAG d'ops), rôles, attestations, synchronisation sans E/S |
| `internal/invite/{ticket,wormhole,lobby,flow}.go` | Ticket, code Wormhole, demande LAN, acceptation |
| `internal/store/{atomic,kv,backup}.go` | KV atomique et vérifié sur `os.Root`, sauvegardes |
| `internal/project/{project,projection,watch}.go` | Dossier projet, projection `.typ`, règle de propriété fsnotify |
| `internal/bridge/{kinds,storestream}.go` | Kinds `Attest`/`Attested`/`Revoked`, protocole du Stream `store` |
| `internal/app/{session,people,settings}.go` | Services Wails : inviter, rejoindre, personnes, réglages |
| `cmd/inkwell-relay/main.go` | Relais auto-hébergeable, pur Go, sans cgo |
| `frontend/src/store/GoStorageAdapter.ts`, `projection.ts` | Stockage via Go, projection sur disque |
| `frontend/src/net/coverage.ts` | Vérifie la couverture par attestations avant le `Repo` |
| `frontend/src/ui/{ShareDialog,JoinPrompt,PeoplePanel,StatusIndicator}.svelte` | Interface |
| `scripts/netsim/{topology.sh,run.sh}`, `frontend/e2e/{offline,persistence}.spec.ts` | Critère de sortie local |
| **Supprimé** : `internal/transport/tcp/` | Après T4 |

---

### Task 1 : contrat partagé + paquet `frame`

**Files :**
- Create `internal/transport/transporttest/suite.go`.
- Move `tcp/frame.go` et `tcp/budget.go` vers `internal/transport/frame/`, avec `FuzzReadFrame` et les tests de budget.
- Modify `memnet_test.go` et `tcp_test.go` pour qu'ils appellent la suite.

```go
package transporttest
type Pair struct {
    A, B    transport.Transport
    Connect func(t testing.TB) // relie A↔B ; idempotent
    Cut     func(t testing.TB) // perte brutale du lien, sans Close
}
type Options struct{ MaxFrame int; Ordered bool } // Ordered : ordre FIFO par émetteur
func Run(t *testing.T, newPair func(t *testing.T) Pair, o Options)
```

- [ ] **Step 1 : extraire la suite.** On reprend les tests comportementaux de `tcp_test.go` sous forme de sous-tests :
  - `PeerUpBothSides`, `DeliversIntactToTargetOnly`, `OrderedPerSender` (si `Ordered`)
  - `SendErrors` (pair inconnu, trop gros, fermé), `CloseNotifiesPeerOnce`, `CutEmitsSinglePeerDown`
  - `ReconnectAfterCut`, `ConcurrentConnectNoFlap`, `BackpressureNoLoss` (2× budget, récepteur lent)
  - `RecvHonorsContext`, `RecvBlockedThenClose`, `PeersSorted`

  Les tests propres à TCP (octets du hello, `NewPeerID`) restent dans `tcp`.
- [ ] **Step 2 :** `memnet` et `tcp` passent la suite. La suite est ainsi validée sur deux implémentations avant p2p. `go test -race`, fuzz `frame` 30 s.
- [ ] **Step 3 : commit** `refactor(transport): shared contract suite and frame package`.

### Task 2 : `identity`

**Files :** `internal/identity/{device.go,fingerprint.go,wordlist/fr.txt,wordlist/en.txt,*_test.go}`.

```go
type Device struct{ /* ed25519.PrivateKey non exportée */ }
func LoadOrCreate(dir *os.Root) (*Device, error) // "device.key" PEM PKCS#8 (x509), 0o600 ; refuse un fichier lisible par groupe/autres
func (d *Device) Public() ed25519.PublicKey
func (d *Device) Sign(domain string, msg []byte) []byte // préfixe "inkwell/<domain>/v1\x00"
func (d *Device) Libp2pKey() ed25519.PrivateKey       // seul usage : app → p2p.New
func Fingerprint(pub ed25519.PublicKey, lang string) [4]string
func MachineName() string                             // os.Hostname nettoyé, ≤ 40 runes
```

**Choix et justification :**
- **Fichier `0o600` sous `os.UserConfigDir()/inkwell`.** `INKWELL_HOME` le surcharge (e2e, netsim).
  - Écartés pour la v1 : le trousseau du système (Secret Service absent en CI et sans session), et le stockage dans la webview (ce que fait Keyhive ARK, mais la clé y serait exposée).
  - Une interface interne garde la porte ouverte au trousseau.
- **Empreinte = 4 mots BIP39** (2048 mots, 11 bits chacun, donc 44 bits). La liste française officielle exclut les homophones et se reconnaît aux 4 premières lettres. Elle est sous licence MIT.
- **Dérivation étirée** : `Argon2id(pub, sel = "inkwell-fp-v1", t = 1, m = 64 MiB)`. Signal étire pour la même raison (5200 × SHA-512). Fabriquer une clé dont l'empreinte imite celle d'une personne passe de quelques jours-GPU à un coût hors de portée. Le résultat est mis en cache par clé.
- Alternatives écartées : la liste PGP (32 bits seulement, anglais) et la liste courte de l'EFF (anglais seulement).
- Mots affichés dans la langue de l'interface ; « Détails techniques » montre l'autre langue et l'hexadécimal.

- [ ] **Step 1 : tests.**
  - `TestLoadOrCreateStable` (deux appels donnent la même clé)
  - `TestRejectsLoosePermissions` (`0o644` donne une erreur)
  - `TestSymlinkEscapeRefused`
  - `TestFingerprintGolden` (vecteurs fr/en figés)
  - `TestWordlists2048Unique4Prefix`
  - `TestSignDomainSeparated`
- [ ] **Step 2 :** FAIL. **Step 3 :** implémenter. **Step 4 :** PASS.
- [ ] **Step 5 : commit** `feat(identity): per-machine Ed25519 key and 4-word fingerprint`.

### Task 3 : `transport/p2p` (cœur)

**Files :** `internal/transport/p2p/{node.go,stream.go,keys.go,export_test.go,node_test.go,contract_test.go}`.

```go
const ProtocolSync = "/inkwell/sync/1.0.0"
type Options struct {
    Port   int                       // UDP/TCP 7419 de préférence, sinon 0
    Admit  func(transport.PeerID) bool // nil = tous (tests) ; consulté par la porte et à chaque flux entrant
    Relays []string                  // multiaddrs /p2p/… (T5)
    RelayOnly, LANDiscovery bool     // T5, T8
    extra  []libp2p.Option           // tests (simnet), via export_test.go
}
func New(ctx context.Context, key ed25519.PrivateKey, o Options) (*Node, error) // *Node implémente transport.Transport
func (n *Node) Addrs() []string
func (n *Node) Connect(ctx context.Context, peer transport.PeerID, addrs []string) error
func (n *Node) Drop(peer transport.PeerID) // ferme et oublie (révocation)
func IDFromKey(ed25519.PublicKey) transport.PeerID
func KeyFromID(transport.PeerID) (ed25519.PublicKey, error)
```

**Construction de l'hôte :**
- `libp2p.New` avec : `Identity`, `NoTransports`, `Transport(quic.NewTransport)`, `Transport(tcp.NewTCPTransport)`, `Security(noise.ID, noise.New)`, `DefaultMuxers`.
- Écoute : `/ip4/0.0.0.0/udp/P/quic-v1`, `/ip6/::/udp/P/quic-v1`, `/ip4/0.0.0.0/tcp/P`.
- Gestionnaire de ressources : `rcmgr` avec limites explicites (64 pairs, 4 flux sync par pair, 256 MiB de mémoire).
- `connmgr` 32/64, et `Protect(peer, "member")` pour les membres.
- Pas de WebTransport, WebRTC ni WebSocket. Ces transports servent aux navigateurs ; les retirer réduit la surface d'attaque et la taille du binaire (pion).

**Un flux unidirectionnel par sens.** Chaque côté écrit sur le flux qu'il a ouvert et lit les flux que l'autre a ouverts. C'est le modèle de go-libp2p-pubsub (`comm.go`).
- Avantages : pas d'arbitrage d'ouverture simultanée (la danse « plus petit PeerID » de TCP disparaît), ordre FIFO par émetteur, contre-pression gratuite par le contrôle de flux QUIC/yamux.
- Écarté : un flux par message (négociation multistream à chaque fois, aucun ordre garanti).

**Trames et réception :**
- Trames `frame.Write`/`frame.Read`, bornées par `MaxFrame`.
- `frame.Budget` côté réception, comme pour TCP.
- Écritures sérialisées par pair, avec délai `10 s + 1 s/MiB`.
- Ouverture avec `network.WithAllowLimitedConn`.

**`PeerUp` / `PeerDown` :**
- `PeerUp` quand le flux sortant est ouvert **et** que `Admit(peer)` est vrai. Le relais lui-même n'est jamais un pair.
- `PeerDown` quand le dernier flux sortant meurt et que `EvtPeerConnectednessChanged` passe à `NotConnected`.
- Si une nouvelle connexion apparaît (migration), on rouvre le flux sortant sans émettre d'événement.

- [ ] **Step 1 : tests.**
  - `contract_test.go` : `transporttest.Run` avec des paires construites par `simlibp2p.QUICSimnet` sous `synctest.Test`. QUIC, TLS et yamux sont réels, mais l'UDP et l'horloge sont virtuels. `Cut` coupe le routeur simnet.
  - `TestLoopbackSmoke` : vraie boucle locale `127.0.0.1`, hors synctest.
  - `TestAdmitRefusesStranger` (aucun `PeerUp`, flux réinitialisé).
  - `TestKeyIDRoundTrip`.
- [ ] **Step 2 :** FAIL. **Step 3 :** implémenter. **Step 4 :** PASS, plus `-race`.
- [ ] **Step 5 : commit** `feat(transport/p2p): libp2p transport passing the shared contract`.

### Task 4 : bascule sur p2p, suppression de `tcp`

**Files :** Modify `internal/app/session.go`, `cmd/inkwell/main.go`, `frontend/src/workspace/workspace.svelte.ts`, `frontend/e2e/collab.spec.ts`, `playwright.config.ts`. Create `scripts/arch-check.sh` (tâche mise `arch`, dépendance de `ci`). Delete `internal/transport/tcp/`.

**Session provisoire :**
- `Host()` renvoie une « adresse d'invitation » : la liste des multiaddrs avec `/p2p/<id>`.
- `Join(text)` accepte cette adresse.
- `Admit` vaut nil (tout le monde), ce qui équivaut à l'étape 1.
- Pourquoi l'adresse **doit** porter l'identifiant du pair : go-libp2p ne peut composer un numéro sans lui, car le canal sécurisé authentifie ce pair. Un `ip:port` nu serait donc impossible, ou bien, en confiance au premier usage, ouvert aux attaques de l'homme du milieu.

**Identité séparée par instance :** `INKWELL_HOME` (une valeur par instance dans `playwright.config.ts`).

- [ ] **Step 1 : tests.**
  - `session_test` mis à jour (texte invalide refusé).
  - `scripts/arch-check.sh` : pour chaque paquet, si `github.com/libp2p` figure dans `go list -deps` hors de `internal/transport/p2p`, c'est un échec. Pareil pour `wormhole-william` hors de `internal/invite`, et pour toute dépendance `automerge`.
- [ ] **Step 2 :** e2e `collab.spec` vert sur p2p. Ensuite `git rm -r internal/transport/tcp`. `mise run ci` vert, couverture ≥ 85 %.
- [ ] **Step 3 : commit** `feat(app): switch to libp2p transport; remove stage-1 TCP`.

### Task 5 : joignabilité (relais, hole punching, reconnexion) + `inkwell-relay`

**Files :** `p2p/{dial.go,gate.go,relay.go,links.go,sim_test.go}`, `cmd/inkwell-relay/main.go`, `docs/relay.md`, tâche mise `relay:run`.

```go
func (n *Node) Want(peers map[transport.PeerID][]string) // pairs à garder connectés + adresses connues
type LinkKind uint8 // LinkLAN, LinkDirect, LinkRelayed
func (n *Node) Links() []Link                            // {Peer, Kind, RTT}
func RunRelay(ctx context.Context, key ed25519.PrivateKey, o RelayOptions) error // utilisé par cmd/inkwell-relay
```

**Boucle de numérotation :**
- Pour chaque pair voulu et absent, tentatives avec backoff de 1 s à 60 s, avec gigue.
- Relance immédiate sur `EvtLocalAddressesUpdated` (changement de réseau) et sur une découverte mDNS.
- Adresses essayées : les dernières connues, **puis l'adresse de circuit construite** `/<relais>/p2t-circuit/p2p/<pair>`. Cette adresse ne dépend pas de l'IP courante du pair : c'est elle qui garantit le « retour sur un autre réseau ».
- Les dernières adresses connues sont persistées par l'app (T9).

**Options libp2p :**
- `EnableHolePunching()`.
- `EnableAutoRelayWithStaticRelays(relays)` + `ForceReachabilityPrivate()` lorsqu'un relais est configuré. Un poste de bureau est presque toujours derrière un NAT, et AutoNAT v2 suppose plusieurs serveurs publics qu'un petit réseau privé n'a pas. C'est exactement le raccourci que la documentation de `EnableHolePunching` recommande.
- `NATPortMap` désactivé en v1 (moindre privilège ; on réévaluera sur mesures).

**`RelayOnly` (« Masquer mon adresse aux autres personnes ») :**
- La porte refuse toute connexion qui n'est pas un circuit, sauf vers le relais. Pas de hole punching.
- C'est une vraie fonction de confidentialité, et c'est aussi le mode e2e sans root.

**Relais :**
- `Resources` circuit v2 réglées : le défaut de go-libp2p (2 min et 128 KiB par connexion relayée) est fait pour le hole punching, pas pour synchroniser.
- Nous prenons `Limit = nil`, `MaxReservations = 256`, `MaxCircuits = 16`, `MaxReservationsPerIP = 8`, plus une liste blanche facultative de PeerIDs (`WithACL`).
- Le relais ne voit que du chiffré de bout en bout.
- Binaire statique, `CGO_ENABLED=0`, `--key`, `--listen`, `--print-addr`. « En une commande » : `inkwell-relay --key relay.key`.

- [ ] **Step 1 : tests sous synctest.** On s'inspire de `holepunch_test.go` de go-libp2p (`simnet.SimpleFirewallRouter`).
  - `TestRelayThenDirect` : deux hôtes privés et un relais public. Lien relayé, puis `LinkDirect`, sans `PeerDown` (Review Focus 1).
  - `TestPartitionThenHeal` : coupure plus longue que le délai d'inactivité QUIC, ce qui donne un `PeerDown`. Après réparation, un seul `PeerUp` en ≤ 30 s virtuelles.
  - `TestRoamToNewAddress` : une nouvelle `SimConn` avec une autre IP publique. Reconnexion par l'adresse de circuit (Review Focus 2).
  - `TestRelayOnlyRefusesDirect`.
  - Chaque test tourne sur 20 graines de latence.
- [ ] **Step 2 :** FAIL. **Step 3 :** implémenter. **Step 4 :** PASS.
- [ ] **Step 5 : commit** `feat(p2p): reconnect loop, relay, hole punching; inkwell-relay`.

### Task 6 : `trust` (pur, sans E/S) — revue Opus

**Files :** `internal/trust/{op.go,list.go,attest.go,sync.go,*_test.go}`.

```go
type Role uint8 // RoleReader=1, RoleWriter, RoleAdmin
type Hash [32]byte
type ProjectID Hash                   // = SHA-256 de l'op Genesis : identifiant auto-certifié
type Op struct{ Kind OpKind; Subject ed25519.PublicKey; Person Hash; Role Role; Name string
    Prev []Hash; Lamport uint64; Author ed25519.PublicKey; Sig [64]byte } // Kind: Genesis|Add|SetRole|Remove
type Signer interface{ Public() ed25519.PublicKey; Sign(domain string, msg []byte) []byte }
func Genesis(s Signer, name, rootURL string) (Op, ProjectID)
func New(id ProjectID) *List
func (l *List) Apply(op Op) error                      // vérifie signature, structure, Prev connus (sinon en attente) ; idempotent
func (l *List) Issue(s Signer, k OpKind, subject ed25519.PublicKey, r Role, name string) (Op, error)
func (l *List) Role(pub ed25519.PublicKey) Role       // 0 = non membre
func (l *List) Members() []Member
func (l *List) Frontier() []Hash
func (l *List) Since(have []Hash) []Op
func Encode(Op) []byte; func Decode([]byte) (Op, error) // binaire canonique, ≤ 4 KiB
type Attestation struct{ Project ProjectID; Doc string; Heads []Hash; Signer ed25519.PublicKey; Seq uint64; Sig [64]byte }
func (l *List) Verify(a Attestation) error             // signature + signataire écrivain/admin courant
type Sync struct{ /* machine d'état : OnPeerUp, OnMessage → messages sortants */ }
```

**Raffinement de l'ADR 0005.** Lamport seul ne permet pas de détecter la concurrence, et donc ni « la révocation l'emporte » ni l'antidatage. Chaque op porte `Prev` (la frontière vue par son auteur), ce qui forme un DAG de hachages (Kleppmann, *Making CRDTs Byzantine Fault Tolerant*, 2022). L'ordre total reste (tri topologique, puis Lamport, puis clé publique, puis hachage).

**Règles du repli** (déterministes, testées par permutations) :
- Une op n'est valide que si son auteur est admin dans **son passé causal**.
- `Remove(S)` annule toute op **écrite par** S qui ne l'a pas dans son passé causal. C'est la parade à l'antidatage.
- `Remove(S)` l'emporte sur toute `Add`/`SetRole(S)` concurrente. Une réadmission exige une `Add` qui a vu le `Remove`.
- Deux `Remove` croisés et concurrents entre admins : seul celui de l'admin le plus ancien (ordre d'ajout depuis la genèse) s'applique.
- Il reste toujours au moins un admin ; un `Remove` qui viderait les admins est nul.

**Rotation de la clé de session (point 5 de l'ADR 0005) : sans objet en v1.** Il n'y a ni clé de groupe ni chiffrement au repos. Une révocation se traduit par : retrait, `Drop`, refus à la porte, invalidation des tickets en cours, rattestation (T10). Le sujet reviendra avec le chiffrement de groupe (Keyhive).

- [ ] **Step 1 : tests.**
  - `FuzzDecodeOp`, `FuzzDecodeAttestation`.
  - Scénarios nommés : `ConcurrentRevocation`, `TwoAdminsRemoveEachOther`, `Antedating`, `ReaddAfterRemove`, `LastAdminCannotLeave`, `WrongProjectRejected`.
  - `TestFoldOrderIndependent` : `pgregory.net/rapid`, permutations de 30 ops.
  - `TestSyncConverges` : 3 `Sync` sur `memnet` avec fautes, sous synctest.
- [ ] **Step 2 :** FAIL. **Step 3 :** implémenter. **Step 4 :** PASS + fuzz 30 s.
- [ ] **Step 5 : commit** `feat(trust): signed member list as a hash-DAG CRDT, attestations`.

### Task 7 : `mux` + câblage de la confiance

**Files :** `internal/transport/mux/{mux.go,mux_test.go}`, `internal/app/{wiring.go,attest.go}`.

```go
package mux
const Overhead = 1
func Split(t transport.Transport, channels ...byte) map[byte]transport.Transport // un lecteur interne, une file bornée par canal
```

- Canaux : `0x01` repo, `0x02` trust, `0x03` attest.
- L'ordre est préservé entre canaux d'un même pair, puisqu'ils partagent un même flux. C'est ce qui garantit qu'une attestation précède les modifications qu'elle couvre.
- Les sous-transports passent `transporttest.Run` avec `MaxFrame = transport.MaxFrame - Overhead`.

**Câblage dans `app` :**
- `Admit = trust.Role(p2p.KeyFromID(id)) > 0`, ou bien « ticket en cours ».
- `Want` reçoit les membres.
- Une révocation déclenche `Drop`.
- Le canal 3 relaie les attestations : à la connexion, Go envoie la dernière de chaque (doc, signataire) ; il retransmet toute attestation nouvelle **avant** le relais des messages repo ; il élimine les doublons par `Seq`.

- [ ] **Step 1 : tests.**
  - mux : suite de contrat, `FuzzSplitFrame`, `TestChannelIsolation`.
  - app : `TestNonMemberNeverReachesBridge` et `TestRevokeDropsAndRefuses`, sur `memnet` (Review Focus 4).
- [ ] **Step 2–4 :** FAIL → implémenter → PASS.
- [ ] **Step 5 : commit** `feat(app): channel mux, member admission, attestation gossip`.

### Task 8 : `invite` (code, texte, LAN)

**Files :** `internal/invite/{ticket.go,wormhole.go,lobby.go,flow.go,*_test.go}`, `p2p/{lobby.go,mdns.go}`.

```go
type Ticket struct{ Project trust.ProjectID; Host transport.PeerID; Addrs []string
    Secret [16]byte; Role trust.Role; Person *trust.Hash; Expires time.Time } // Person ≠ nil : « un de mes appareils »
func (t Ticket) Text() string            // "inkwell1:" + base32 ; ≤ 1 KiB
func ParseText(string) (Ticket, error)   // fuzzé
type Wormhole struct{ RendezvousURL string } // vide = serveur public magic-wormhole ; AppID "inkwell.app/invite/v1"
func (w Wormhole) Offer(ctx context.Context, t Ticket) (code string, done <-chan error, err error) // SendText
func (w Wormhole) Fetch(ctx context.Context, code string) (Ticket, error)                         // Receive
// p2p : protocole "/inkwell/lobby/1.0.0" requête/réponse ≤ 4 KiB ; HandleLobby, AskLobby, SetOpen(until), Nearby()
```

**Une seule notion, le ticket** (comme les « tickets » d'iroh). Il voyage de trois façons :
1. **Code court Wormhole** (PAKE).
2. **Texte d'invitation** : copier-coller, utile hors Internet.
3. **Rien du tout sur le LAN**, où une acceptation explicite le remplace.

**Wormhole à sens unique.** `SendText` transporte le ticket, qui est authentifié par le PAKE. Le nouvel arrivant se connecte ensuite en libp2p et présente `Redeem{Secret}` sur le protocole lobby. L'hôte compare en temps constant, signe `Add(arrivant, rôle)`, puis répond `Welcome{rootURL, ops}`. L'arrivant vérifie la chaîne d'ops jusqu'à la genèse et `ProjectID` : on ne peut pas lui servir un faux projet. Ticket à usage unique, expiration 15 min, et seul un admin en émet.

**LAN :**
- mDNS sur `_inkwell._udp`, réglage « Découverte sur le réseau local ».
- Un inconnu ne voit que ce que publie mDNS de libp2p (PeerID et adresses, que libp2p ne permet pas de cacher), plus `Hello{Machine, Titre}`.
- `Hello` n'est servi **que pendant la fenêtre « Visible pour rejoindre », 10 min**, sur le modèle de « Tout le monde pendant 10 minutes » d'AirDrop. Hors de cette fenêtre, le flux lobby est réinitialisé et la porte refuse les inconnus.
- `Ask{Nom}` déclenche l'invite « Marie (empreinte : lampe · orage · …) demande à rejoindre » → `Accepter (écriture)` / `Lecture seule` / `Refuser`.

- [ ] **Step 1 : tests.**
  - `FuzzParseText`, `FuzzLobbyDecode`.
  - `TestWormholeRoundTrip` avec `wormhole-william/rendezvous/rendezvousservertest`, en processus.
  - `TestRedeemOnceExpiresWrongProject` (Review Focus 6).
  - `TestHelloOnlyWhileVisible`.
  - `TestLanAskAcceptReject`.
  - `TestDeviceTicketKeepsPerson`.
- [ ] **Step 2–4.** **Step 5 : commit** `feat(invite): tickets via wormhole code, text, and LAN acceptance`.

### Task 9 : `store` + `project` + Stream `store`

**Files :** `internal/store/{atomic.go,kv.go,backup.go}`, `internal/project/{project.go,projection.go,watch.go}`, `internal/bridge/storestream.go`, `frontend/src/store/{GoStorageAdapter.ts,projection.ts}` + tests.

```go
func WriteAtomic(r *os.Root, name string, data []byte, perm fs.FileMode) error // temp O_EXCL → write → Sync → Close → Rename → Sync du dossier
type KV struct{}  // Open(r *os.Root) ; Load/Save/Remove/LoadRange/RemoveRange(key []string) ; composants ^[A-Za-z0-9._-]{1,128}$, profondeur ≤ 4
func (k *KV) Backup(now time.Time, keep int) error // .inkwell/backups/<RFC3339>/ (liens durs), garde 10
type Project struct{}  // Create(base *os.Root, title, rootURL string) / Open ; Meta{ID, Title, RootURL}
func (p *Project) WriteProjection(rel string, data []byte) error
func (p *Project) Watch(ctx context.Context, onExternal func(rel string)) error
```

**Disposition du dossier projet :**
- Emplacement : `<XDG Documents>/Inkwell/<Titre>/`, créé sans poser de question (UX règle 8).
- Contenu : `main.typ` (la projection), et `.inkwell/` en `0o700` avec `project.json`, `repo/…`, `trust/<hash>.op`, `attest/…`, `backups/`, `quarantine/`.
- On tranche la question ouverte du nom : `.inkwell/`.

**Intégrité :**
- Chaque valeur est stockée avec un en-tête `"IKV1" | SHA-256(data)`.
- Au chargement, une divergence donne `ErrCorrupt` : le fichier va en quarantaine et on retente avec la dernière sauvegarde.
- Pour les clés `incremental`, on vérifie en plus que le hachage de la clé égale `SHA-256(data)` (c'est la convention de `keyHash` d'automerge-repo).

**Pourquoi un Stream `store` séparé :**
- Écartés : les bindings (`[]byte` en base64 JSON, +33 %) et le partage du Stream `net` (blocage en tête de file).
- Protocole : `id u32 | op u8 | clé | data`, traité **séquentiellement**, ce qui rend la cohérence lecture-après-écriture triviale.
- Go peut aussi pousser `ExternalChange{path}` (id 0).

**Règle de propriété :**
- Inkwell possède les fichiers listés dans `project.json`.
- Une modification externe, c'est-à-dire un hachage différent du dernier écrit, déclenche le toast « main.typ a été modifié en dehors d'Inkwell — Importer / Ignorer ». « Importer » applique `Automerge.updateText`.
- Les autres fichiers ne sont jamais touchés.

**Côté TS :** `GoStorageAdapter` remplace `IndexedDBStorageAdapter`. `projection.ts` envoie le texte 300 ms après la dernière frappe.

- [ ] **Step 1 : tests.**
  - `TestKillDuringWrite` : un sous-processus est tué entre l'écriture et le `Rename` (Review Focus 5).
  - `TestBitFlipQuarantined`, `TestKeyComponentRejected`, `TestSymlinkEscape`.
  - `FuzzStoreRequest`.
  - `TestProjectionIgnoresOwnWrites`, `TestExternalEditNotified`.
  - TS : conformité de `GoStorageAdapter` (load/save/range) sur un socket factice.
- [ ] **Step 2–4.** **Step 5 : commit** `feat(store,project): atomic verified storage, projection, store stream`.

### Task 10 : contrôle d'écriture (attestations) côté webview — revue Opus

**Files :** `internal/bridge/kinds.go` (+ vecteurs dorés), `frontend/src/net/{envelope.ts,coverage.ts,WailsNetworkAdapter.ts}`, `frontend/src/net/coverage.test.ts`, scénarios de simulation.

**Nouveaux kinds du pont** (`bridge.Kind`, sur-ensemble de `transport.Kind`, version 1 inchangée) :
- `Attest` (JS→Go) : `doc | têtes`.
- `Attested` (Go→JS) : `peer` = signataire, `doc | têtes`.
- `Revoked` (Go→JS).

**Émission :**
- Dans `send()`, si le `heads` de `decodeSyncMessage` diffère de la dernière attestation de ce document, l'adaptateur envoie d'abord `Attest`, **sur le même Stream**.
- Go signe, puis diffuse sur le canal 3 avant de relayer le message repo. L'ordre est donc garanti sans attente.
- Pour un lecteur, Go refuse de signer.

**Réception (`coverage.ts`), avant `emit("message")` :**
- Pour les modifications : BFS depuis les têtes attestées par les écrivains courants (connues via `Attested`) à travers `deps`. Les modifications non couvertes sont retirées, et le message est réencodé avec `encodeSyncMessage`.
- Pour un fragment de document complet, que les messages de synchro V2 d'Automerge 3 peuvent contenir : on charge le fragment, puis on exige que toutes ses modifications soient couvertes. Sinon le message entier est jeté.
- Type de fragment : on lit l'octet 8 de l'en-tête, qui vaut 0 pour un document et 1 ou 2 pour une modification.

**Révocation :** les attestations du révoqué sont ignorées. Chaque écrivain rattestera ses têtes en apprenant la révocation, ce qui couvre l'historique déjà accepté. Les modifications hors ligne d'un révoqué que personne n'a acceptées sont refusées, et elles sont signalées. La question ouverte est tranchée ainsi.

Les messages éphémères (curseurs, présence) restent acceptés pour les lecteurs.

- [ ] **Step 1 : tests.**
  - Go : vecteurs dorés des kinds, `FuzzDecode` étendu, `TestReaderCannotAttest`.
  - TS, scénarios nommés de simulation (10 graines chacun) :
    - « un lecteur forge une modification » : elle est retirée, et les honnêtes convergent ;
    - « fragment de document complet couvert » ;
    - « écrivain révoqué hors ligne » ;
    - « nouveau pair après révocation se synchronise ».
- [ ] **Step 2–4.** **Step 5 : commit** `feat(net): signed-heads write enforcement`.

### Task 11 : interface (partager, rejoindre, personnes, état)

**Files :**
- `internal/app/{session.go,people.go,settings.go}`. Bindings : `Me`, `Invite(role)`, `Join(codeOrText)`, `SetVisible`, `Nearby`, `Answer`, `People`, `SetRole`, `Remove`, `Settings`, `SetSettings`. Événements Wails : `join-request`, `people-changed`.
- `frontend/src/ui/*`, `i18n/{fr,en}.json`.

**Comportement :**
- **Partager** affiche le code (« 7-guitare-orage », copiable), puis « Copier l'invitation » (le texte), puis l'interrupteur « Visible sur ce réseau (10 min) ». Si le code Wormhole est indisponible : « Pas d'accès Internet : copiez l'invitation ».
- **Rejoindre** a un seul champ (code ou texte) et une liste « À proximité ».
- **Invite d'acceptation** (`JoinPrompt`) : ce n'est pas une boîte modale bloquante, mais une bannière avec ses actions.
- **Panneau Personnes** : nom, empreinte au survol, rôle, et lien « direct (réseau local) » / « direct » / « via relais ».
- **Indicateur global** : « Synchronisé », « Hors ligne — vos modifications sont gardées », ou « Reconnexion… ».
- **Réglages** : relais (liste modifiable, relais par défaut), « Découverte sur le réseau local », « Masquer mon adresse aux autres personnes ».

- [ ] **Step 1 : tests.** Go : `session_test` (validation des entrées, erreurs traduites en phrase + action). TS : `people.test`, i18n complet.
- [ ] **Step 2–4.** **Step 5 : commit** `feat(ui): invite, join, people and status panels`.

### Task 12 : critère de sortie sur un seul PC, CI, documentation

**Files :** `frontend/e2e/{collab,persistence,offline}.spec.ts`, `scripts/netsim/{topology.sh,run.sh}`, `mise.toml` (`e2e`, `e2e:netsim`), `.github/workflows/ci.yml`, `PROGRESS.md`, `docs/architecture.md`, ADR.

**e2e en boucle locale, sans root** (`mise run e2e`) :
- `collab` : le nouvel arrivant rejoint avec le texte d'invitation.
- `persistence` : frappe, redémarrage de l'instance, le texte est intact, et `main.typ` sur disque est identique au texte.
- `offline` :
  - le test lance lui-même `bin/inkwell-relay` ;
  - A et B tournent en `RelayOnly` (`INKWELL_RELAYS`, `INKWELL_NET=relay-only`) ;
  - on tue le relais : les deux passent « hors ligne » et tapent chacun une ligne ;
  - on relance le relais (même clé, même port) ;
  - attendu : convergence en ≤ 30 s.

**netsim** (`mise run e2e:netsim`) :
- `run.sh` se relance lui-même sous `unshare -Urnm --fork` si l'uid n'est pas 0, puis monte un tmpfs sur `/run` et crée `/run/netns`. **Aucun root sous Fedora.**
- Topologie :
  - `wan` 203.0.113.0/24 ;
  - relais 203.0.113.10 ;
  - routeurs `nat-a`, `nat-b`, `nat-c` en masquerade nftables (`random` pour simuler un NAT symétrique) ;
  - hôtes `alice` (10.1.0.2, avec liens vers `nat-a` et `nat-c`) et `bob` (10.2.0.2).
- Accès pour Playwright : `socat` unix↔tcp. Aucune interface de gestion, qui court-circuiterait le NAT.
- Scénarios :
  1. NAT cône : lien direct après hole punching.
  2. NAT symétrique : le lien reste relayé.
  3. Coupure de `alice` pendant 60 s, les deux tapent, retour : convergence.
  4. `alice` passe de `nat-a` à `nat-c` (nouvelle IP publique) : convergence.
- Le scénario 4 est littéralement « sur deux réseaux différents ».

**CI** (job `netsim`, ubuntu-24.04) :
- `sudo apt-get install socat nftables` ;
- `sudo sysctl -w kernel.apparmor_restrict_unprivileged_userns=0` (Ubuntu 24.04 bride les userns non privilégiés) ;
- puis `mise run e2e:netsim`, le même chemin sans root qu'en local.

**Ce qui exige root :**
- Rien sous Fedora 44 (userns activés, `nf_tables` déjà chargé par firewalld).
- En CI, seulement `apt` et `sysctl`.
- Si `nft` échoue dans le userns (module absent), `sudo modprobe nf_tables` une fois, ou bien se rabattre sur `mise run e2e` (relais en boucle locale), qui couvre « hors ligne puis retour » sans le NAT.

**Clôture :**
- [ ] PROGRESS : critère démontré par `p2p/sim_test`, `e2e offline` et `e2e:netsim` scénarios 3 et 4, avec le journal de l'exécution. La démonstration physique devient facultative.
- [ ] `architecture.md` : TCP disparaît, les paquets `mux` et `frame` apparaissent, la phrase sur le chiffrement est corrigée.
- [ ] Revue de branche complète : `go-reviewer`, puis `security-auditor` sur `p2p`, `trust`, `invite`, `store`, `bridge`.
- [ ] Commit `docs: close stage 2`.

**Ordre et parallélisme :**
- Chemin critique : T1 → T3 → T4 → T5. Le risque est traité en premier : TCP disparaît dès T4.
- T2 se fait en parallèle de T1.
- T6 en parallèle de T3–T5 (sans E/S).
- Puis T7 → T8.
- T9 en parallèle de T6–T8.
- Puis T10 → T11 → T12.

---

---

## ADR à écrire

| N° | Titre | Décision en une ligne |
|---|---|---|
| 0014 | Transport p2p : QUIC d'abord, un flux par sens | QUIC-v1 (TLS 1.3) + TCP/Noise en secours, sans WebTransport/WebRTC/WS ; un flux sortant unidirectionnel par pair (modèle pubsub) ; `ForceReachabilityPrivate` + relais statiques + DCUtR ; pas d'UPnP en v1 |
| 0015 | Relais auto-hébergé `inkwell-relay` | Circuit v2 sans limite par connexion (`Limit=nil`), plafonds de réservations par IP, liste blanche facultative ; pur Go ; reconnexion par adresse de circuit construite |
| 0016 | Invitation = ticket, trois porteurs | Ticket (PeerID + adresses + secret à usage unique + ProjectID + rôle) via code Wormhole (PAKE, sens unique), texte, ou acceptation LAN pendant une fenêtre de visibilité de 10 min ; l'adresse doit porter le PeerID |
| 0017 | Liste de membres en DAG de hachages (amende 0005) | `Prev` + Lamport/clé pour l'ordre ; révocation l'emporte sur toute op concurrente du révoqué et sur toute op le concernant ; l'admin le plus ancien gagne ; ProjectID = hachage de la genèse ; pas de rotation de clé de session en v1 (pas de clé de groupe) |
| 0018 | Contrôle d'écriture par têtes signées | Go signe (la clé reste en Go) et vérifie les attestations ; la webview vérifie que chaque modification reçue est couverte avant le `Repo` ; un message non couvert est filtré ou jeté |
| 0019 | Empreinte : 4 mots BIP39 étirés par Argon2id | Listes fr/en officielles (MIT, sans homophones) ; Argon2id 64 MiB contre le broyage des 44 bits |
| 0020 | Stockage en Go par un Stream `store` | KV atomique (`os.Root.Rename` + fsync), en-tête SHA-256, quarantaine, sauvegardes en liens durs ; dossier `.inkwell/` ; remplace IndexedDB |
| 0021 | Critère réseau vérifié sur un seul PC | simnet + synctest en Go ; e2e relais tué puis relancé ; namespaces réseau sans root (`unshare -Urnm`) avec NAT nftables, mêmes scripts en CI |

## Risques ouverts

1. **`x/simlibp2p` est expérimental.** Son API peut bouger à chaque version de go-libp2p. On épingle, et `mocknet` (`p2p/net/mock`) sert de plan B pour la suite de contrat.
2. **wormhole-william est peu actif** (dernier commit en août 2025). Il dépend de `gospake2` (2021, hébergé sur salsa.debian.org). Le serveur de rendez-vous public est un tiers en `ws://` (contenu chiffré, mais les métadonnées sont visibles). Le porteur est isolé derrière `invite`. Plans B : le fork `rymdport/wormhole` (déjà sur `coder/websocket`), ou un rendez-vous hébergé sur `inkwell-relay`. Les mots du code restent anglais (liste PGP) en v1.
3. **Messages de synchro V2 d'Automerge 3** (document complet) : la vérification de couverture charge le fragment, ce qui coûte O(document) à la première synchro. Il faut le mesurer sur 50 000 ops (budget < 2 s).
4. **Un pair malveillant renvoie sans cesse des modifications filtrées** (boucle de synchro). Parade : limite de débit, puis `Drop` après N rejets. À vérifier en simulation.
5. **mDNS publie un PeerID stable sur chaque réseau rejoint** (pistage possible). Le réglage le désactive ; des identifiants tournants viendront plus tard.
6. **Empreinte en langues différentes** : deux personnes avec une interface fr et une interface en ne voient pas les mêmes mots. « Détails techniques » montre les deux.
7. **Ubuntu 24.04 et userns** : AppArmor impose le `sysctl` en CI. Sous Fedora, `nf_tables` doit déjà être chargé, ce qu'assure firewalld.
8. **Relais sans limite** : risque d'abus de bande passante sur un relais public. Parades : plafonds par IP, liste blanche facultative, et métriques côté relais.

## Sources

**go-libp2p et ses tests :**
- [go-libp2p v0.51.0](https://github.com/libp2p/go-libp2p/releases/tag/v0.51.0) et [v0.50.0](https://github.com/libp2p/go-libp2p/releases/tag/v0.50.0) (Go 1.26, quic-go 0.62)
- [defaults.go](https://github.com/libp2p/go-libp2p/blob/v0.51.0/defaults.go) (TLS puis Noise par défaut, transports par défaut)
- [options.go](https://github.com/libp2p/go-libp2p/blob/v0.51.0/options.go) (`EnableHolePunching`, `ForceReachabilityPrivate`, `EnableAutoNATv2`)
- [relay/resources.go](https://github.com/libp2p/go-libp2p/blob/v0.51.0/p2p/protocol/circuitv2/relay/resources.go) (défaut de 2 min / 128 KiB)
- [mdns.go](https://github.com/libp2p/go-libp2p/blob/v0.51.0/p2p/discovery/mdns/mdns.go)
- [x/simlibp2p](https://github.com/libp2p/go-libp2p/tree/v0.51.0/x/simlibp2p), [holepunch_test.go](https://github.com/libp2p/go-libp2p/blob/v0.51.0/p2p/protocol/holepunch/holepunch_test.go), [marcopolo/simnet](https://github.com/marcopolo/simnet)
- [go-libp2p-pubsub comm.go](https://github.com/libp2p/go-libp2p-pubsub/blob/master/comm.go)

**Invitation :**
- [psanford/wormhole-william](https://github.com/psanford/wormhole-william) (v1.0.8, mars 2025)
- [rymdport/wormhole](https://beta.pkg.go.dev/github.com/rymdport/wormhole), [webwormhole](https://github.com/saljam/webwormhole), [gowormhole](https://github.com/bingoohuang/gowormhole), [paquet Debian wormhole-william](https://dyn.manpages.debian.org/trixie/wormhole-william/wormhole-william.1.en.html), [sujet GitHub magic-wormhole](https://github.com/topics/magic-wormhole)

**Empreinte :**
- [Listes de mots BIP39 (règles de la liste française)](https://github.com/bitcoin/bips/blob/master/bip-0039/bip-0039-wordlists.md)
- [libsignal fingerprint](https://github.com/signalapp/libsignal/blob/main/rust/protocol/src/fingerprint.rs)

**Automerge et confiance :**
- [Automerge sync.rs (messages V2 avec document complet)](https://github.com/automerge/automerge/blob/main/rust/automerge/src/sync.rs)
- [Guide de l'API Keyhive ARK (alpha)](https://automerge.org/docs/keyhive/ark-api-guide/)
- [Kleppmann, Making CRDTs Byzantine Fault Tolerant (PaPoC 2022)](https://martin.kleppmann.com/papers/bft-crdt-papoc22.pdf)

**Stockage :** [fsnotify v1.10.1](https://github.com/fsnotify/fsnotify/releases). La disponibilité de `os.Root.Rename` a été vérifiée avec `go doc` sous Go 1.27.3. Les types de `StorageAdapterInterface` et `keyHash` ont été lus dans `frontend/node_modules`.
