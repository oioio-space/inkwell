# Block RC1 — reconnect after restart (specification, step 1)

Rigour: **complete** (state, concurrency, bytes read from disk that a peer influenced, addresses of people on disk).
The formal model is replaced by a synchronous simulation (synctest, no relay) plus mutants; the written reason is in
section 9. Frozen gate: `gates/RC1.toml`. Decision row: `docs/autonomous-decisions.md`, 2026-11-12 RC1.

## 1. The need

Two people work on a project. One of them closes inkwell (or the machine reboots, or the app crashes) and opens it
again. **What they must see:** the other person's name comes back on its own, in seconds, with nothing to do and no
relay. **What they see today:** nobody comes back. The link stays "absent" until somebody sends a new invitation,
because after a restart nothing remembers where the other device was.

Root cause (diagnosed, reproduced): `Admission.dialTargets` (internal/app/wiring.go) gives `Node.Want` the members with
**no address** (`nil`); `Node.dialOnce` (internal/transport/p2p/dial.go) dials only the known addresses or the relays;
there is no default relay. The stage-2 plan promised persisted last addresses (docs/plans/2026-10-25-etape-2-reseau.md,
"T9"); it was never built. Second defect: the LAN probe (internal/app/invites.go) calls `Node.Drop` on a person it
cannot greet, and `Drop` also un-wants a member (node.go), so a probe can silently end a member's reconnection.

Repro (red now, kept as the permanent test): two nodes on the simulated NAT network without relay, connected, b quits,
b starts again with the same key and port: `appWant` never reconnects (2 min of virtual time), the control with known
addresses reconnects in 86 ms.

## 2. Design

1. **Address book** = what a device last saw of each other member: the dialable addresses of the member (as announced
   by the member itself through libp2p identify) and when it was last seen. It lives **inside `internal/transport/p2p`**
   (types `addrBook`, codec, policy), because it holds multiaddrs, it is fed by node-internal events (PeerUp, mDNS,
   Drop), and AC-3/AC-4 forbid any other package from importing libp2p or from being imported back by transport. Pure
   policy and codec, no I/O.
2. **Persistence is injected**: `p2p.Options.Book BookStore`, `type BookStore interface { Load() ([]byte, error);
   Save([]byte) error }`. Nil = no persistence (the book still works in memory, which serves the process-lifetime
   case). `internal/app` provides the file store (`addresses.json` next to `device.key` in the home profile
   `*os.Root`, mode 0600, atomic write with `store.SyncDir`), because `internal/store` helpers must not be imported by
   transport (AC-3). The home dir stays the single place of per-device state; this is a per-device file, not
   per-project (a device's addresses do not depend on the project; members of several projects share a line).
3. **Feeding**: on `PeerUp` of a peer admitted by `Options.Admit` (identify has completed: the peerstore holds the
   addresses the peer announced), record its non-relay addresses; on an mDNS sighting (`lanNotifee.HandlePeerFound`) of
   a peer admitted by `Admit`, record the sighted addresses. Never record for a peer `Admit` refuses (an unknown
   person's address is not kept). Addresses `/p2p-circuit` (relay paths are rebuilt from `Options.Relays`), unspecified
   and multicast addresses are skipped.
4. **Use**: `New` loads the book and seeds the peerstore (so libp2p can dial any listed peer). `dialOnce`: the addresses
   of `w.known` first; **if `known` is empty (or all failed), the book's addresses**; then the relays, as today.
   The fallback reads the book, not only the peerstore: libp2p expires identify addresses 15 minutes after a
   disconnect, so a peerstore-only fallback would fail after any longer outage. RelayOnly mode still removes every
   non-circuit address, book included (nothing direct is dialed, nothing recorded from direct links).
5. **Want** is unchanged for the app: `dialTargets` keeps passing `nil` addresses; a peer wanted without address is
   dialed at the book's. A wanted peer with no book entry and no relay stays `errNoAddress` (visible, see 7).
6. **Drop** (revocation) un-wants, closes, clears the peerstore **and deletes the book entry**. A new
   `Node.Hangup(who)` closes the connections of a peer **without** un-wanting it and without touching the book; if the
   peer is not wanted it also clears its peerstore entries (what `Drop` did for a stranger). The LAN probe calls
   `Hangup`. `Reset` is unchanged.
7. **Observability**: one `slog` line per load (entries kept/dropped), per quarantine, per save failure; the existing
   `errNoAddress` for a wanted peer without address is surfaced in the diagnostics the same way as today's dial errors.

## 3. File format `addresses.json` (version 1)

UTF-8 JSON read with `encoding/json/v2` (duplicate names and invalid UTF-8 rejected; unknown names rejected):

```json
{"v":1,"peers":[{"id":"12D3KooW…","seen":1789000000,"addrs":["/ip4/192.168.1.20/udp/7419/quic-v1","/ip4/192.168.1.20/tcp/7419"]}]}
```

- `v` MUST be 1; any other value → file refused (not interpreted by a newer or older reader).
- File size ≤ 96 KiB **before** parsing (64 peers x 8 addresses x ~160 B ≈ 80 KiB + framing). Larger → refused.
- `peers` ≤ 64 (the protocol's per-project capacity of 64 devices, architecture §2 "capacité"); each `id` a valid
  libp2p peer ID from an Ed25519 key (the only key kind inkwell uses), no duplicates, not our own; `addrs` 1..8, each a
  valid multiaddr, ≤ 128 bytes, dialable (ip4/ip6/dns over udp-quic-v1 or tcp), not /p2p-circuit, not unspecified,
  without a `/p2p/` suffix (the peer ID is the entry's), no duplicates; `seen` a Unix second in
  `[2020-01-01, now + 1 day]`.
- Any violation: the **whole file** is refused (it is not a file this code wrote or someone edited it), moved to
  `addresses.json.bad` (one slot, overwritten), and the device starts with an empty book. Starting is never blocked;
  the cost of refusal is one reconnection by invitation or mDNS or by the other side dialing us.
- Atomic write: temp file in the same directory (mode 0600), `fsync`, `rename`, `fsync` of the directory. A reader
  sees the old or the new file, never a mix (same primitive as the trust store). Mode is forced to 0600 on write; a
  wider mode on read is corrected, not refused (the file is not a secret key but exposes network location; see 5).
- Writes are coalesced: the in-memory book is flushed at most once per 10 s, and only when the **set** of addresses
  or peers changed (a refreshed `seen` alone is flushed on `Close` and at most once per hour). A failed save is
  logged and retried at the next change; the process never stops for it.

## 4. Eviction

| Rule | Value | Why |
|---|---|---|
| max peers | 64 | member capacity; beyond it the entry seen longest ago is evicted |
| max addresses per peer | 8, the most recently announced win | identify lists a few per interface, bounded |
| age | an entry not seen for **90 days** is dropped at load and at each flush | a stale address is harmless (peer ID is checked by the handshake) but it is location data we have no reason to keep |
| member removed | `Node.Drop(who)` deletes the entry and the line is rewritten at once (not coalesced) | revocation must not leave the removed person's address on disk |
| refused by `Admit` | never recorded; an entry whose peer `Admit` refuses at load is dropped | an old member whose removal happened while we were off |

## 5. Privacy

The file holds **IP addresses (private LAN and public) of the people one works with**, plus the time last seen. It is
location-sensitive data kept in plaintext in the user's home profile. Mitigations: mode 0600 and directory 0700 (same
as the identity), only members are recorded, 90 days maximum, deleted on revocation, never logged (logs say counts),
never sent to a peer, never in a diagnostic or crash report (the file is outside the reports' allow-list, whose own test stays the proof), removed with the profile. A device-wide "forget everything" action removes the file;
the settings screen entry is out of scope here and filed as a follow-up in `docs/exigences.md`. Full-disk encryption
is the person's choice, as for the project files.

Security of the use: a recorded address is a **hint**. The handshake authenticates the peer ID; an address that now
belongs to another device fails the handshake (test `TestReconnectStaleAddressWrongPeer`). The file is local state but
its decoder is fuzzed anyway (CLAUDE.md: bytes that a peer influenced are verified, decoder fuzzed): a peer chooses
the addresses it announces.

## 6. The reconnection bound

Constants (dial.go, pinned): `minBackoff` 1 s, `maxBackoff` 60 s, jitter up to +25 %, `knownDialTimeout` 10 s,
`stableAfter` 30 s.

**Bound R (normative for this block):** let T0 be the instant a restarted device is listening and has installed
`Want` for a member (its book already loaded). If the other device is up and **at least one of the two books holds a
currently valid address of the other**, the link is up within **R = knownDialTimeout + 5 s = 15 s** of T0.

Derivation:
1. The restarted device's dial loop starts with no prior pause: its first `dialOnce` runs at T0 (the loop begins with
   `backoff = minBackoff` and dials before it ever pauses). That attempt targets the book's addresses; it either
   connects or fails within `knownDialTimeout` = 10 s.
2. If its book address is valid and the other side is listening, the QUIC/TCP handshake completes in a few round trips
   (≤ 1 s on the simulated network, < 5 s allowed for any realistic WAN) → connected by T0 + 10 s + 5 s.
3. If only the **other** side holds a valid address (the restarted one changed address), the restarted device's
   `PeerUp` is not needed: the other side's dial loop is in one of two states: (a) it saw the connection drop after
   `stableAfter` → it redials at once (fails within 10 s: the old address is dead), then pauses ≤ 1 s x 1.25, and so on;
   its pause is capped by `maxBackoff` = 60 s x 1.25 = 75 s; but the restarted device, whose book holds the **other**
   side's address, takes branch 2. So R needs only one valid address among the two books, **provided that device is
   the restarted one or its loop is not sleeping**; the symmetric case (only the surviving device holds a valid
   address, its loop sleeping up to 75 s) is bounded by 75 s + 10 s + 5 s = **90 s** and is **not** inside R: it
   requires the survivor's dial loop to be kicked, which `Node` already does on address change and on `PeerUp` of any
   peer; the test covers the case with a kick-free sleeping loop and asserts the 90 s bound.
4. No valid address in either book, no relay, no mDNS: no bound (section 7).

Detection of a crashed peer (kill -9, power cut) is outside R: it is bounded by the transport's own liveness
(yamux keepalive 30 s, QUIC idle timeout) and already specified in the link-probe/WB blocks; R starts at T0 of the
restarted device, which for a crash is when the person reopens the app, long after detection.

## 7. Failure modes

| Situation | Behaviour |
|---|---|
| no entry, no relay, no mDNS (first start, or every address changed on both sides) | `errNoAddress`; the link comes back when either side is reached by an invitation or by mDNS or when the other side dials us; documented residual, not a regression |
| stale address, another device now there | handshake fails (peer ID mismatch), attempt counted as a failure, backoff as today; never an accepted link to the wrong peer |
| book file corrupt, too large, wrong version | quarantine as `.bad`, empty book, app starts (section 3) |
| book directory read-only / disk full | save fails, logged; the in-memory book keeps working; next change retries |
| peer removed while we are off | `Admit` refuses it at load: entry dropped; it cannot reconnect (Admit still gates every connection) |
| clock goes backwards / forward | `seen` beyond now + 1 day refused at load; age compares with the current clock only (a jump forward can evict early: acceptable, costs one reconnection by other means) |
| two instances of the app on one profile | excluded by the single-instance lock (ADR 0037); the file is still written atomically |
| RelayOnly | nothing direct is dialed or recorded |
| a hostile peer announces 10 000 addresses | capped at 8 per peer, 64 peers, 128 B each, checked before storing |
| a member of two projects | one entry (keyed by peer ID) |

## 8. Placement, import rules, public spec

- Code: `internal/transport/p2p/addrbook.go` (+ `Options.Book`, `Hangup`, `dialOnce` fallback, hooks in the
  PeerUp/mDNS/Drop paths); `internal/app/addrbook_file.go` (the `BookStore` on the home `*os.Root`, uses
  `internal/store`); `internal/app/invites.go` (`Drop` → `Hangup`); `internal/app/node.go` (pass `Options.Book`).
  Rules: AC-1 (no import of `app` by internal), AC-3/AC-4 (libp2p stays in p2p; transport imports no other internal
  package; the file store lives in app, which may import both), AC-2 untouched. `mise run arch` is in the gate.
- **Not in the public spec** (`docs/protocol/spec.md`): nothing crosses the wire. The identify addresses are libp2p's
  own protocol; the file is local state of one implementation; the bound R describes this client's dial schedule.
  Another implementation is free to do otherwise. Consequently: no protocol version, no CHANGES entry, no vectors.
  The file format and R are normative **here** (a frozen test names each rule) and summarized in
  `docs/architecture/spec.md` §3 "tolérance aux fautes" only if the architecture review asks for it at step 7.
- Invariants checked: one source of truth (the book is a cache of what members announced; members stay in the signed
  list; texts untouched); bytes from a peer verified (peer ID by handshake; admission by `Admit`; decoder fuzzed);
  libp2p only in p2p; no new server, no port, no download.

## 9. Test plan

All in `internal/transport/p2p` unless noted; sim tests use the existing `natNet` + `synctest` fixtures (no relay,
virtual time, every latency seed of `sim_test.go`).

| Test | Rule it proves |
|---|---|
| `TestReconnectAfterRestartNoRelay` (subtests `samePort`, `restartedMovedPort`, `survivorSleeping`) | R: 15 s when the restarted device holds a valid address (same port; or it moved and the survivor's address is in its book); 90 s when only the sleeping survivor holds it. The repro made permanent, driven through the real `dialTargets` path (`Want` with nil addresses) |
| `TestReconnectAfterLongOutage` | peer down for 1 h (> the 15 min peerstore expiry): still reconnects: the fallback uses the book |
| `TestReconnectStaleAddressWrongPeer` | the recorded address now answers as another peer ID: no link, no event for the wanted peer |
| `TestDialFallbackUsesBook` / `TestDialFallbackRelayOnly` | `known` empty → book addresses; RelayOnly dials none of them |
| `TestHangupKeepsWant` | after `Hangup` a wanted member reconnects; a stranger's peerstore is cleared |
| `TestDropUnwantsAndForgetsBook` | `Drop` un-wants, deletes the entry, triggers an immediate save |
| `TestBookRecordsAdmittedOnly` | no entry for a peer `Admit` refuses |
| `TestBookRecordsMDNSSighting` | a sighting of an admitted peer updates addresses and `seen` |
| `TestBookSkipsCircuitAndUnspecified` | relay, unspecified, multicast, non-dialable addresses never stored |
| `TestBookBounds` | 64 peers (oldest evicted), 8 addresses (latest win), 128 B |
| `TestBookEvictsOld` | exactly 90 days: kept at 90 d, dropped past it, at load and at flush |
| `TestBookSaveCoalesced` | at most one save per 10 s; `seen`-only changes not flushed; flushed on Close |
| `TestBookRoundTrip`, `TestBookDecodeRejects` (table: wrong `v`, duplicate name, unknown name, bad ID, own ID, 0/9 addresses, circuit, `/p2p/` suffix, future `seen`, 97 KiB, 65 peers, invalid UTF-8, trailing bytes) | section 3, one row per rule |
| `FuzzDecodeBook` | decoder never panics, never accepts beyond the bounds, accepted → re-encodes to an equal book; seeds = the table; 120 s in the gate |
| `internal/app`: `TestAddrBookFileMode` (0600, wider mode corrected) | section 3 |
| `internal/app`: `TestAddrBookFileAtomic` (a concurrent reader of 2000 saves never sees a partial file) | atomic write |
| `internal/app`: `TestAddrBookCorruptQuarantined` | `.bad`, empty book, start succeeds |
| `internal/app`: `TestLANProbeKeepsMember` | a probe failing on a member leaves it wanted; `Drop` is no longer used by the probe (architecture test on the call) |
| `internal/app`: `TestRestartReconnectsMembers` | two real runtimes (existing `app` fixtures, loopback), restart one with the same `INKWELL_HOME`, no relay: the member list shows the other person again within R |

**Mutants** (`scripts/netsim/mutants.toml`, block `RC1`, kind `go`, each must turn its named test red by a real
`--- FAIL`; to be added in step 2 together with the code, listed here so the gate can name them):
`MC1a` dialOnce ignores the book; `MC1b` `Hangup` un-wants; `MC1c` recording skips `Admit`; `MC1d` per-peer cap
removed; `MC1e` age compared with `>=` instead of `>` or removed; `MC1f` `Drop` leaves the entry; `MC1g` decoder
accepts `v` != 1; `MC1h` circuit addresses recorded; `MC1i` fallback reads the peerstore only (the 1 h test); `MC1j`
save writes in place; `MC1k` file created 0644; `MC1l` `Drop` replaced by `Hangup` is undone in the probe;
`MC1m` the first `dialOnce` of a new loop pauses before dialing (breaks R); `MC1n` no whole-file refusal (a bad entry
skipped, the rest kept).

Why no TLA+ model: the book adds no protocol state and no message; its only concurrency is one mutex around a map, and
the dial loop it feeds is unchanged (its liveness is already modelled/measured by the NI0/WB/C5S blocks). The property
of interest, the bound R, is arithmetic over pinned constants and is checked end to end by the synctest test under
every latency seed, with mutant `MC1m` proving the test can see a violation. If step 2 reveals interleavings between
`Drop`, `Hangup`, `Want` and the flush goroutine that the tests cannot cover, stop and write a small TLA+ model first
(rule "flatten when hard").

Order for step 2: tests red → `addrBook` + codec (+ fuzz) → hooks → fallback → `Hangup` → app file store and wiring →
mutants → `/simplify` (`gates/RC1/SIMPLIFY.md`) → verifications → `mise run gate -- RC1`.
