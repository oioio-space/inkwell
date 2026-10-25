# Étape 1 — Noyau : plan d'implémentation

> **Pour les agents :** sous-skill requis : `superpowers:subagent-driven-development` ou
> `superpowers:executing-plans`. Étapes en cases à cocher (`- [ ]`).

**Objectif :** deux personnes tapent dans le même fichier, sur deux instances d'inkwell
reliées en TCP, curseurs visibles, undo qui ne défait que ses propres frappes.

**Architecture :** la webview (Svelte 5 + CodeMirror 6 + automerge-repo) tient le document.
Un `NetworkAdapter` TypeScript parle au Go par le Stream Wails `net` en enveloppes binaires ;
Go route ces octets opaques vers les pairs via l'interface `transport.Transport` (TCP à cette
étape, libp2p à l'étape 2). La poignée de main automerge-repo (arrive/welcome) voyage **dans**
les charges utiles : Go ne connaît que des pairs et des octets.

**Stack :** Go 1.27.3, Wails v3.0.0-beta.34 (GTK4), Svelte 5, Vite 8, TypeScript 6,
pnpm 12, Biome 2, Vitest, `@automerge/automerge-repo` 2.5.6, `@automerge/automerge-codemirror`
0.2.0, CodeMirror 6.

**Spécification :** `docs/conception.pdf` (§ Architecture, § Tests de convergence, § Plan
étape 1), ADR 0002, 0004, 0007, 0009, 0010, 0011, `docs/engineering-principles.md`.

## Contraintes globales

- Versions exactes (pas de `^`/`~`) dans `frontend/package.json` ; outils dans `mise.toml`.
- Aucun code Go n'importe Automerge ; seul `internal/transport/tcp` (puis `p2p`) ouvre des sockets.
- Toute trame venue d'un pair : taille bornée (`MaxFrame = 64 << 20`), décodeur fuzzé.
- Pas de variable globale mutable, pas d'`init()` à effet de bord ; `context.Context` en 1er paramètre.
- Couverture Go ≥ 85 % (gate existant) ; `go test -race` ; tests concurrents sous `testing/synctest`.
- UI : français par défaut, zéro jargon (`docs/glossary.md`) — « personne », « héberger », « rejoindre ».
- Couleurs des personnes : palette Okabe-Ito (8 teintes), nom toujours affiché au survol.
- TCP de l'étape 1 = **non chiffré, non authentifié** : l'UI l'indique (« réseau local de
  confiance uniquement ») ; remplacé par libp2p/Noise à l'étape 2.

## Review Focus

1. Pair qui se déconnecte au milieu d'une trame → aucune fuite de goroutine, `PeerDown` émis une fois.
2. Rechargement de la page (Stream fermé puis rouvert) → les pairs connus sont réannoncés, l'édition reprend.
3. Deux instances qui se connectent l'une à l'autre en même temps → un seul lien gardé, pas d'oscillation.
4. Frappe concurrente au même endroit, puis undo de chaque côté → chacun n'annule que son texte.
5. Trame JS malformée (enveloppe tronquée, version inconnue) → ignorée et journalisée, le pont reste ouvert.

---

## Structure des fichiers

| Fichier | Rôle |
|---|---|
| `internal/transport/transport.go` | `PeerID`, `Kind`, `Event`, interface `Transport`, erreurs sentinelles |
| `internal/transport/memnet/memnet.go` | Réseau en mémoire avec fautes injectées (graine) |
| `internal/transport/tcp/frame.go` | Trames longueur-préfixées (lecture/écriture bornées) |
| `internal/transport/tcp/tcp.go` | `Node` : écoute, numérotation, poignée de main, pairs |
| `internal/bridge/envelope.go` | Enveloppe binaire webview ↔ Go (encode/decode) |
| `internal/bridge/router.go` | Routeur : une connexion Stream attachée ↔ `Transport` |
| `internal/bridge/testdata/envelopes.json` | Vecteurs dorés partagés Go/TS |
| `internal/app/session.go` | Service Wails lié : `LocalPeer`, `Host`, `Join`, `Peers` |
| `inkwell.go` | Paquet racine : `Assets` (frontend embarqué) |
| `cmd/inkwell/main.go` | Composition Wails : services, Stream `net`, fenêtre |
| `build/`, `Taskfile.yml` | Build Wails (Linux, macOS, Windows seulement) |
| `frontend/src/net/envelope.ts` | Miroir TS de l'enveloppe |
| `frontend/src/net/WailsNetworkAdapter.ts` | `NetworkAdapter` automerge-repo sur le Stream |
| `frontend/src/net/simnet.ts` | Réseau simulé à graine pour Vitest |
| `frontend/src/editor/*.ts` | CodeMirror : sync, historique local, curseurs distants |
| `frontend/src/ui/*.svelte` | Barre d'en-tête, panneau Héberger/Rejoindre, avatars |
| `frontend/src/theme/tokens.css` | Jetons Adwaita (primitifs → sémantiques), clair/sombre |

---

### Task 1 : cœur `transport` + `memnet`

**Files :** Create `internal/transport/transport.go`, `internal/transport/memnet/memnet.go`,
`internal/transport/memnet/memnet_test.go`. Delete `inkwell_test.go` (placeholder `Greet`).

**Interfaces — Produces :**

```go
package transport
type PeerID string            // opaque, non vide, ≤ 64 octets ASCII imprimables
type Kind uint8
const (PeerUp Kind = iota + 1; PeerDown; Message)
type Event struct { Kind Kind; Peer PeerID; Payload []byte } // Payload seulement pour Message
type Transport interface {
    Local() PeerID
    Send(ctx context.Context, to PeerID, payload []byte) error // ErrUnknownPeer, ErrClosed
    Recv(ctx context.Context) (Event, error)                   // un seul consommateur ; ErrClosed après Close
    Peers() []PeerID                                          // trié
    Close() error
}
var ErrUnknownPeer, ErrClosed, ErrFrameTooLarge error
const MaxFrame = 64 << 20
func ValidPeerID(id PeerID) bool
```

```go
package memnet
type Faults struct { Drop, Duplicate float64; MaxDelay time.Duration } // probabilités 0..1
func New(seed uint64, f Faults) *Network
func (n *Network) Join(id transport.PeerID) *Node      // *Node implémente transport.Transport
func (n *Network) Link(a, b transport.PeerID)          // émet PeerUp des deux côtés
func (n *Network) Partition(a, b transport.PeerID)     // émet PeerDown des deux côtés
```

- [ ] **Step 1 : tests (échouent)** — sous `synctest.Test` : `TestLinkEmitsPeerUpBothSides`,
  `TestSendDeliversToTargetOnly`, `TestPartitionEmitsPeerDownAndRejectsSend` (`errors.Is(err, transport.ErrUnknownPeer)`),
  `TestFaultsAreDeterministicForSeed` (deux réseaux même graine + même scénario ⇒ même suite
  d'événements), `TestRecvAfterCloseReturnsErrClosed`, `TestValidPeerID` (table : `""` faux,
  65 octets faux, `"a\x00"` faux, `"k7f3"` vrai). Ajouter `goleak.VerifyTestMain`.
- [ ] **Step 2 :** `mise run test` → FAIL (paquets absents).
- [ ] **Step 3 : implémenter.** RNG `math/rand/v2` PCG(seed) ; délais via `time.AfterFunc` (virtualisés par synctest) ;
  file par nœud non bornée côté réseau mais `Recv` respecte `ctx`.
- [ ] **Step 4 :** `mise run test` → PASS ; `go test -race ./internal/transport/...` → PASS.
- [ ] **Step 5 : commit** `feat(transport): Transport interface and fault-injecting memnet`.

### Task 2 : transport TCP

**Files :** Create `internal/transport/tcp/{frame.go,tcp.go,frame_test.go,tcp_test.go}`.

**Interfaces — Consumes :** Task 1. **Produces :**

```go
package tcp
func WriteFrame(w io.Writer, payload []byte) error           // u32 big-endian + payload ; ErrFrameTooLarge
func ReadFrame(r io.Reader, max int) ([]byte, error)         // n'alloue jamais plus que max
type Options struct { HandshakeTimeout, WriteTimeout time.Duration } // défauts 5 s / 10 s
func New(local transport.PeerID, o Options) *Node            // *Node implémente transport.Transport
func (n *Node) Listen(ctx context.Context, addr string) (net.Addr, error)
func (n *Node) Dial(ctx context.Context, addr string) (transport.PeerID, error)
func NewPeerID() transport.PeerID                            // 16 octets crypto/rand, base32 minuscule sans padding
```

Poignée de main : chaque côté envoie une trame `"INKW" | version u8 = 1 | peerID`, lit celle de l'autre
avec `HandshakeTimeout`. Refus : version ≠ 1, peerID invalide, peerID = local. Lien en double
(connexion croisée) : on garde la connexion initiée par le plus petit `PeerID`, l'autre est fermée.

- [ ] **Step 1 : tests.** `FuzzReadFrame` (aucune panique, jamais > max alloué, aller-retour
  `WriteFrame`/`ReadFrame` identique) ; `TestReadFrameRejectsOversize` ; `TestTwoNodesExchange`
  (loopback `127.0.0.1:0`, `PeerUp` des deux côtés, message reçu intact) ;
  `TestHandshakeRejectsSelf`, `TestHandshakeTimeout` (pair muet) ; `TestSimultaneousDialKeepsOneLink`
  (Review Focus 3) ; `TestPeerDropMidFrameEmitsSinglePeerDown` (Review Focus 1) ; `goleak`.
- [ ] **Step 2 :** FAIL.
- [ ] **Step 3 : implémenter** — une goroutine lectrice par connexion, écritures sérialisées par
  pair (mutex), `SetWriteDeadline` ; `Close` ferme listener et connexions puis attend les goroutines (`sync.WaitGroup.Go`).
- [ ] **Step 4 :** PASS, `-race`, `go test -fuzz=FuzzReadFrame -fuzztime=30s ./internal/transport/tcp`.
- [ ] **Step 5 : commit** `feat(transport/tcp): length-prefixed TCP transport with handshake`.

### Task 3 : pont `bridge`

**Files :** Create `internal/bridge/{envelope.go,router.go,envelope_test.go,router_test.go}`,
`internal/bridge/testdata/envelopes.json`.

**Interfaces — Consumes :** Task 1 (`memnet` en test). **Produces :**

```go
package bridge
const StreamName = "net"
// Enveloppe : version u8 = 1 | kind u8 | len(peer) u8 | peer | payload.
type Envelope struct { Kind transport.Kind; Peer transport.PeerID; Payload []byte }
func Encode(e Envelope) ([]byte, error)
func Decode(b []byte) (Envelope, error)          // ErrMalformed (enveloppé)
type Conn interface { Send([]byte) error; Receive() ([]byte, error); Context() context.Context }
func NewRouter(t transport.Transport, log *slog.Logger) *Router
func (r *Router) Run(ctx context.Context) error  // unique lecteur de t.Recv ; transmet à la Conn attachée, sinon jette
func (r *Router) Serve(c Conn)                   // bloque ; à l'attache, envoie PeerUp pour t.Peers() ; détache à la sortie
```

JS → Go : seul `Kind == Message` est accepté (`Peer` = destinataire). Go → JS : `PeerUp`, `PeerDown`, `Message` (`Peer` = émetteur).
Une seule `Conn` attachée : une nouvelle remplace l'ancienne (rechargement de page).

- [ ] **Step 1 : tests.** `TestEnvelopeVectors` lit `testdata/envelopes.json`
  (`[{name, kind, peer, payloadHex, wireHex}]`, ≥ 6 cas dont payload vide et peer de 64 octets) et vérifie
  `Encode`/`Decode` dans les deux sens ; `FuzzDecode` ; `TestDecodeRejects` (version 2, tronqué, peer invalide) ;
  `TestServeAnnouncesExistingPeers` ; `TestRouterForwardsBothWays` (memnet + `fakeConn`) ;
  `TestMalformedFrameIsDroppedAndConnStaysOpen` (Review Focus 5) ;
  `TestReattachReplacesConnAndReannounces` (Review Focus 2) ; `goleak`.
- [ ] **Step 2 :** FAIL. **Step 3 :** implémenter. **Step 4 :** PASS + fuzz 30 s.
- [ ] **Step 5 : commit** `feat(bridge): binary envelope and Stream↔Transport router`.

### Task 4 : application Wails + chaîne frontend

**Files :** Create `build/` (depuis `wails3 init -t svelte`, sans `ios/`, `android/`, `docker/`),
`Taskfile.yml`, `frontend/` (Svelte 5 TS, `package.json` exact, `pnpm-lock.yaml`, `biome.json`,
`vitest.config.ts`, `.npmrc` `minimum-release-age=10080`), `internal/app/session.go` + test,
`inkwell.go` (remplace `Greet`), `cmd/inkwell/main.go`. Modify `mise.toml`, `.gitignore`,
`.github/workflows/ci.yml`, `.github/dependabot.yml` (npm `/frontend`), `scripts/clean-artifacts.sh`.

**Interfaces — Produces :**

```go
package inkwell
//go:embed all:frontend/dist
var Assets embed.FS
```

```go
package app
type Session struct{ /* *tcp.Node */ }
func NewSession(n *tcp.Node) *Session
func (s *Session) LocalPeer() string
func (s *Session) Host(ctx context.Context, port int) ([]string, error) // écoute 0.0.0.0:port, renvoie "ip:port" des interfaces non loopback
func (s *Session) Join(ctx context.Context, addr string) error           // valide host:port avant de numéroter
func (s *Session) Peers() []string
```

`frontend/dist/.gitkeep` versionné (l'`embed` compile sans build frontend) ; le script `build`
le recrée. Tâches mise : `fe:install`, `fe:check` (biome + svelte-check + vitest), `app:dev`
(`wails3 dev`), `app:build` (`wails3 build`) ; `ci` dépend de `fe:check`.
CI : `apt-get install libgtk-4-dev libwebkitgtk-6.0-dev` avant `mise-action`, `CGO_ENABLED=1`, job
`app:build` Linux.

- [ ] **Step 1 :** tests `internal/app` : `TestJoinRejectsBadAddress` (`""`, `"host"`, `"a:99999"`),
  `TestHostReturnsReachableAddrs` (port 0 ⇒ ≥ 1 adresse `ip:port`), `TestLocalPeerStable`.
- [ ] **Step 2 :** FAIL. **Step 3 :** implémenter + échafaudage Wails/frontend.
- [ ] **Step 4 :** `mise run ci` PASS ; `mise run app:build` produit `bin/inkwell` ; la fenêtre s'ouvre.
- [ ] **Step 5 : commit** `feat(app): Wails v3 shell, Session service, frontend toolchain, CI`.

### Task 5 : adaptateur réseau + simulation de convergence (TS)

**Files :** Create `frontend/src/net/{envelope.ts,WailsNetworkAdapter.ts,simnet.ts}` et tests
`*.test.ts`.

**Interfaces — Consumes :** enveloppe de Task 3 (mêmes vecteurs, importés depuis
`../../internal/bridge/testdata/envelopes.json`). **Produces :**

```ts
export function encodeEnvelope(e: Envelope): Uint8Array
export function decodeEnvelope(b: Uint8Array): Envelope           // lève MalformedEnvelope
export class WailsNetworkAdapter extends NetworkAdapter {
  constructor(socket: SocketLike, opts: { projectUrl?: AutomergeUrl })
  onProject(cb: (url: AutomergeUrl) => void): void                // url reçue d'un hôte via arrive/welcome
}
export function createSimNet(seed: number, faults: { drop: number; dup: number }): SimNet
```

Charge utile = CBOR (`cbor-x`, déjà dépendance d'automerge-repo) de `{type: "arrive"|"welcome", peerMetadata, projectUrl?}`
ou du `Message` automerge-repo. `peer-candidate` n'est émis qu'après arrive/welcome ; `PeerDown` ⇒ `peer-disconnected`.
Le `peerId` du `Repo` = `Session.LocalPeer()`.

- [ ] **Step 1 : tests.** `envelope vectors` (mêmes cas que Go) ; adaptateur avec `fakeSocket` :
  `announces peer only after welcome`, `peer down emits peer-disconnected`, `reconnect re-runs handshake`,
  `drops malformed frames without closing` ; simulation : `3 repos converge under 10% drop, 50 seeds`
  (même texte et `getHeads` identiques après drainage), `concurrent typing at same position converges`.
- [ ] **Step 2 :** FAIL. **Step 3 :** implémenter. **Step 4 :** `mise run fe:check` PASS.
- [ ] **Step 5 : commit** `feat(frontend/net): Wails network adapter and seeded convergence sim`.

### Task 6 : éditeur, présence, interface

**Files :** Create `frontend/src/editor/{setup.ts,localHistory.ts,remoteCursors.ts}` + tests,
`frontend/src/ui/{HeaderBar.svelte,ShareDialog.svelte,PeopleStack.svelte}`,
`frontend/src/theme/tokens.css`, `frontend/src/App.svelte`, `frontend/src/i18n/fr.json`, `en.json`.

**Interfaces — Produces :**

```ts
export const localHistoryOnly: Extension   // addToHistory=false pour tout changement sans userEvent
export function remoteCursors(presence: Presence<CursorState>): Extension
type CursorState = { name: string; color: string; anchor?: string; head?: string } // curseurs Automerge
```

- [ ] **Step 1 : tests.** `undo reverts only local edits` (EditorState + `history()` + `localHistoryOnly` :
  insertion locale, insertion « distante » sans userEvent, `undo` ⇒ seul le texte local disparaît —
  Review Focus 4) ; `remote cursor maps through concurrent edit` ; `okabe-ito palette assigns stable color per peer`.
- [ ] **Step 2 :** FAIL. **Step 3 :** implémenter : barre d'en-tête (titre, état « Seul » / « Synchronisé »,
  pile d'avatars), dialogue Partager (Héberger → affiche les adresses à copier ; Rejoindre → champ
  `ip:port`, avertissement réseau de confiance), éditeur plein cadre, thème clair/sombre, `prefers-reduced-motion`.
- [ ] **Step 4 :** `mise run fe:check` PASS ; démonstration manuelle : deux instances (`bin/inkwell`
  ×2, l'une héberge, l'autre rejoint `127.0.0.1:port`) — frappe croisée, curseurs, undo.
- [ ] **Step 5 : commit** `feat(frontend): collaborative editor with presence and local undo`.

### Task 7 : critère de sortie et clôture

- [ ] Démo sur deux machines du LAN (ou deux instances locales en CI manuelle) consignée dans `PROGRESS.md`.
- [ ] `PROGRESS.md` : cocher l'étape 1, journal ; `docs/architecture.md` à jour ; ADR si une frontière a bougé.
- [ ] Revue de branche complète (`go-reviewer` + `security-auditor` sur `transport/tcp` et `bridge`).
- [ ] Commit `docs: close stage 1`.
