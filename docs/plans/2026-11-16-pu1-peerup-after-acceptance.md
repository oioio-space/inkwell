# PU1 — PeerUp only after the peer's acceptance

Date: 2026-11-16. Block PU1, rigour **complete** (wire and state between peers, method v2 closed list). Step 1 only:
rules, spec §8.7, vectors, model, gates; no product code. Origin: block HP1r (STATE « Session 5 »,
`gates/HP1r.toml` arm `closed_dropping_side`, 9/10 red) and the Codex debate of 2026-11-16 (decision: option 1, an
application-level acceptance proof after the peer's own checks; forgetting cached protocols is not a guarantee;
the gater check stays complementary).

## 1. The defect, stated

`transport.PeerUp` fires when our outbound sync stream's `NewStream` returns (`internal/transport/p2p/stream.go:90-117`).
go-libp2p's `BasicHost.NewStream` waits for identify, then, if the peerstore already lists the protocol, skips
multistream-select and returns a lazy stream (`basic_host.go:473-497` of the pinned fork; `go-multistream`
`lazyClient.go`: « It does not block writes waiting for the other end to respond »). The return then proves nothing
about the peer: in HP1r's trace, `a` closes a late connection in `ConnectedF` (rule H-19) before it accepts any
stream; `b` registers it, finds `/inkwell/sync/1.0.0` cached from the previous session, and announces `a` up, then
down ~10 ms later. The same flicker follows a one-sided admission (the peer's handler resets our stream after
`PeerUp`). Upstream knows the lazy path is unsound for data written before confirmation (go-multistream issue
#20: Stebalien, « the unsound part is writing arbitrary data before confirmation »); identify repopulates the
protocol list on every connection, so forgetting it (option 2) only narrows the window.

## 2. How other stacks decide that a session is up (research)

| Stack | What makes the session count | Source |
|---|---|---|
| libp2p connections spec | multistream-select is interactive: the responder echoes the id or answers `na`; nothing says the handler accepted | [libp2p/specs connections](https://github.com/libp2p/specs/blob/master/connections/README.md) |
| go-multistream lazy client | optimistic select: the client writes before the echo; the open issue proposes a `use` reply before data is trusted | [go-multistream #20](https://github.com/multiformats/go-multistream/issues/20) |
| Syncthing BEP v1 | Hello both ways, then authentication; « a Cluster Config message MUST be the first post authentication message »; a rejected device is dropped | [BEP v1](https://docs.syncthing.net/specs/bep-v1.html) |
| Hypercore Protomux | `onopen` runs only in `_fullyOpen()`, once the remote's open is paired with ours; a remote reject closes the channel | [protomux README](https://github.com/holepunchto/protomux), `index.js` `_fullyOpen` |
| Matrix federation | a transaction counts only on the receiver's `200`: « The sending server must wait and retry for a 200 OK » | [Server-Server API, Transactions](https://spec.matrix.org/latest/server-server-api/) |
| IPFS Bitswap (boxo) | `PeerConnected` on the connection event; the protocol support comes from identify, and an attempt to block until accepted (#435) was reverted (#444) — the weakness we have | [boxo #432](https://www.github.com/ipfs/boxo/issues/432) |
| inkwell 2.0 target (spec §11.1) | Sécurisé → Actif only on valid HELLOs from both sides | spec §11.1 |

Every robust design waits for a message *from the peer's application* before it counts the session (Syncthing's
ClusterConfig, Protomux's paired open, Matrix's 200, our own 2.0 HELLO). Beyond the existing: a full hello is not
needed here because identity and project are already bound by the authenticated connection and Admit; the only
missing fact is « the peer's handler accepted this stream on a connection it keeps ». One fixed byte carries it, sent
**before** the responder waits for anything, which is what removes the deadlock that a naive « reply after my own
PeerUp » would create (model mutant M3).

## 3. Rules

| Rule | Statement | Proof (step 2) | Mutant |
|---|---|---|---|
| P-1 | A node announces a peer up (`PeerUp`) only after reading the peer's ACK on the outbound sync stream that carries the session; the return of `NewStream` proves nothing (cached or negotiated path alike). | `TestPeerUpAfterAck` arms `closed_at_once_by_remote` (HP1r's trace on main's code: a raw libp2p host closes the connection at once), `one_sided_admission`; vectors `syncack.json` (opener) | TLA M1 (S3); Go MP1 (PeerUp on return) |
| P-2 | The **responder** (the handler of an inbound sync stream) writes the ACK only after, in this order: the node is not closed; `Admit` accepts the peer and it is not held (`admitted`); the stream's connection is not closed (a connection the node closes at its registration — a late link it does not keep, Admit refusing it — never reaches a handler). Otherwise it resets the stream without writing. | `TestPeerUpAfterAck` arms `not_admitted_remote`, `ack_needs_live_conn`; vectors (responder) | TLA M2, M5 (S1); Go MP2 (ACK before `admitted`) |
| P-3 | The responder writes the ACK **before** it waits for its own `PeerUp` of the opener (`waitUp`, `stream.go:238`); frames are still read only after that `PeerUp`. Crossed opens therefore never wait on each other. | `TestPeerUpAfterAck` arms `ack_before_own_up`, `both_sides_up` | TLA M3 (L1 BothUp) |
| P-4 | Wire: the ACK is exactly one byte `0x01`, the only byte the responder ever writes on that stream. The opener reads exactly one byte: `0x01` = accepted; any other value is a violation (stream reset, `ClosePeer`, as a hostile byte today); EOF or reset before it = refusal (`ClosePeer`); any byte after it = hostile (`watchOut`, unchanged). | vectors `sync_ack` decoder; fuzz `FuzzReadAck`; arms `wrong_ack_byte`, `byte_after_ack`; an ACK counts only for the stream it arrives on (model S2 by stream) | Go MP3 (any byte accepted); TLA M7 (acceptance by connection only) |
| P-5 | Deadline: `NewStream` and the ACK share **one** `openTimeout` (15 s, unchanged), counted from the open and not restarted when NewStream returns; it fires whatever is in flight, a responder stuck in its handler included. Without the ACK by then: reset, `ClosePeer`, no event (the connection is useless for syncing, as a failed open today). The responder's write of the ACK is bounded by the same 15 s, then reset. | arms `silent_remote` (simulated 15 s, no PeerUp, connection closed), `slow_negotiation_late_ack` (echo after 10 s, ACK 6 s later: closed at 15 s, no PeerUp) | TLA S4 Budget, S5 NoUpFromStuck, L2 OpenResolves (Stuck); M6 (deadline waits for silence), M8 (budget restarted at return) |
| P-6 | The move (the relay-to-direct upgrade, `stream.go:480`, block NI0) replaces the session's stream only after the ACK on the new stream; without it the new stream is reset and the session stays where it is, no event. Any rule that speaks of « the session is off the circuit » therefore means « after the ACKed swap ». A stream reopened while up (`outLost` with reopen) also waits for its ACK before carrying frames (Send already waits while `ps.out` is nil). | `TestPeerUpMoveAfterAck` arms `move_waits_for_ack`, `move_refused_keeps_circuit` | TLA M4 (S2) |
| P-7 | Version: the stream protocol becomes `/inkwell/sync/2.0.0` (incompatible change, spec §1.4); `/inkwell/sync/1.0.0` is no longer served nor opened (clean switch, no release yet). A peer without 2.0.0 answers `na`: the open fails, `ClosePeer`, no event. Bulk (`/inkwell/bulk/1.0.0`) is unchanged: it is opened only after PeerUp. | arm `old_peer_refused`; registry §20.6 | — |

**Guarantee and its limit.** P-1..P-3 give: *a PeerUp at A is preceded by B's handler accepting A's stream, on a
connection B had registered and not closed, while B admitted A* — **for a conforming B only**: a Byzantine B can
write `0x01` without applying Admit, so A's own local admission (checked before it opens the stream and before it
reads B's frames) stays the guarantee; the ACK adds that a conforming peer accepted *this* stream. It does not promise a long session: a peer that
accepts then closes yields PeerUp then PeerDown (a real, short session, not a phantom). A connection closed by one
side at its registration gives no PeerUp on either side, because no handler can run on it.

**Interactions.** *Frontend handshake* (`frontend/src/net/WailsNetworkAdapter.ts:279`, automerge-repo `arrive`): it
starts on PeerUp and is unchanged; it cannot be the acceptance proof (it runs above PeerUp, so using it would be the
deadlock P-3 avoids) and it stays the repo-level proof (spec §9.3). *Block HP1* (rebased onto PU1, not a
dependency): its H-19 closes late links at registration, which P-2 then covers, and its H-15 « off the circuit »
reads as P-6 says; HP1r's arm `closed_dropping_side` needs PU1. *Gater*: unchanged, complementary (it refuses a stranger
before any stream). *2.0 session (§5, §11.1)*: its HELLO already plays the ACK's role; §8.7 states that Actif is never
entered on a stream's return either.

**Cost.** At most one RTT, by construction: the ACK travels back once. `TestPeerUpLatency` measures it paired, in
one run on controlled time (synctest, simnet path of RTT R = 100 ms): the PeerUp delay of the same setup with the
ACK and without it (a test-only seam in `export_test.go` that skips the ACK read). Arm `cached`: the difference is
R to R + 2 ms (the lazy header now waits for the ACK: exactly one round trip). Arm `negotiated`: the difference is
0 to R + 2 ms (logged; reading the code suggests ≈ 0, the ACK following the multistream echo, but no bound is
frozen on that reading). Netsim (1-4 strict; 5, 8 characterize, `gates/PU1n.toml`, alone on the machine, later)
characterises real timings only.

## 4. Specification artefacts (step 1, this commit)

- Spec §8.7 (ten points of `docs/protocol/README.md`), §16.1 row, §20.6 registry, CHANGES.md entry.
- Vectors `protocol/vectors/syncack.json` (generator `protocol/cmd/vectors/syncack.go`): decoders `sync_ack`
  (opener's reading of the stream), `sync_ack_responder` (P-2's decision table), `sync_ack_opener` (event scripts:
  P-1, P-4, P-5, P-6). Status `proposed`, no `enforce` at step 1 (CI stays green); step 2 adds the reference
  (`protocol/syncack`: `Ack`, `ReadAck`, the two pure decision functions, `FuzzReadAck`), sets `enforce: step-3`,
  and inkwell imports it (no private copy).
- Model `docs/protocol/tla/PeerUpAck.tla`: Honest, Late (the closed-at-once trace; invariants), LateLive, Silent, Byzantine
  (invariants), Stuck, Reuse (invariants); a clock per opening stream (D ticks, absolute); acceptance by connection and
  stream; S1-S5, L1 BothUp, L2 Open/MoveResolves; mutants M1-M8, four witnesses (gates/PU1/WITNESSES.md).
- Gates: `gates/PU1.toml` (≤ 45 min), `gates/PU1n.toml` (netsim, exclusive); red witnesses `gates/PU1/WITNESSES.md`.

## 5. Open questions for the integrator

1. P-7: `/inkwell/sync/2.0.0` accepted by the integrator (2026-11-16; nothing released); no person-facing « too
   old » message (spec §1.4) until a release exists.
2. Go mutants MP1-MP3 go in `scripts/netsim/mutants.toml` (block PU1) at step 2 with their anchors; the gate checks
   the entries' kind, package, test, clause and expected text (main's `mutants.py`, unchanged).
3. Order (integrator): PU1 first on main, independent of HP1; HP1 and HP1r are rebased onto it, and HP1r's frozen
   arm `closed_dropping_side` is expected to pass then, which closes the finding of HP1r session 5.
4. Exigence and plan placement (`docs/exigences.md`, `docs/architecture/coherence.md` §5, `PROGRESS.md`) are the
   integrating session's to file.
