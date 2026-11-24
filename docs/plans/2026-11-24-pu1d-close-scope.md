# PU1d - what a failed sync opening closes (P-16), and openings without a peer event (P-17)

Status: **draft, step 1** (2026-11-24), cross-family review by DeepSeek V4 Pro « accepter avec corrections »
(`gates/PU1d/SPEC-REVIEW.md`): findings 1, 2 and 4 applied here, 3 and 5 need no change. Rigour: **complete** (wire and state between
peers). Worktree `../inkwell-pu1d` on `main` `abe7bac`. `L/` = `$(go env GOMODCACHE)/github.com/oioio-space/go-libp2p@v0.51.1-0.20261122194938-8ebee42bc17a`.

## 1. Defect

Spec §8.7.4 said that a refusal, a violation or the end of the 15 s budget of a session opening "ferme ses connexions au
pair (`ClosePeer`)" (also §8.7.5, §8.7.6, §8.7.8, §16). The code and the vectors agree with that text:
`internal/transport/p2p/stream.go:116-130` (`openOut`: `ClosePeer(ps.id)` at :129), `:336-341` (`outLost` after a
remote reset or a byte after the ACK: `ClosePeer` at :340); reference `protocol/syncack/syncack.go:204,213,243`
(`Opener` → `ClosePeer`), `order.go:144-147` (`OnRefusal` → `ClosePeerAction`); 9 vectors of `syncack.json` and 4 of
`syncorder.json` expect `close_peer`. The model disagrees: `tla/PeerUpAck.tla` `RecvReset` (:270-282) and `Timeout`
(:298-303) close only the connection of the stream (`CloseC`, :136-138); its comments still say "(ClosePeer)". The P-15
proofs of block HP1c (`Supersede.tla`, which extends PeerUpAck) were therefore made under the connection scope, while
the code runs the peer scope. HP1's plan routed this here (`2026-11-23-hp1-reformalize.md` §6).

## 2. Decision: the connection scope (rule P-16)

A failed session opening closes the connection it ran on, and nothing else. Only `Drop` (admission withdrawn) and
the node's own close close every connection to a peer. Evidence:

1. **What a failure proves.** The ACK proves the acceptance of one stream on one connection (P-4; model S2 by stream).
   Its absence proves no more. Among the responder's refusals (§8.7.3), "connection closed or closing" is a
   connection fact. The deadline also fires on path faults: a relay circuit that stops forwarding, or a dead older
   connection. libp2p prefers that dead connection because it carries the most streams (`L/p2p/net/swarm/swarm.go:595-625`
   `isBetterConn`; measured in HP1, plan §2).
2. **libp2p semantics.** `Swarm.ClosePeer` closes every connection (`swarm.go:711-736`). Closing one connection among
   several emits no connectedness event (`connection_events_emitter.go:201-225`, `notifyPeer` emits on change only).
   So the session must reopen by itself on a remaining connection (P-17, below).
3. **Consistency of §8.7.** P-15 keeps the older connection as the fallback *if the opening on the newer one fails*.
   `ClosePeer` on that failure closes the fallback too. §8.7.5 says two crossed openings are independent. `ClosePeer`
   breaks that when the peer's acknowledged session runs on another connection: TLC finds it at depth 13 (MC1 below).
   The same three causes already close nothing on the bulk lane (§8.1) and only abandon a move (§8.7.4).
4. **Threat model.** A connection carries one peer, so under either scope a Byzantine peer closes only its own
   connections. A relay cannot forge a stream reset: the two ends upgrade the relayed connection with their own security
   protocol and multiplexer (libp2p `relay/circuit-v2.md`). But a relay can stall a circuit. Under `ClosePeer`, a third
   party could then close a live direct connection, which cost DCUtR calls (12 per peer per hour, §8.6).
5. **Root cause of the E2E3 defects.** 123 of 125 failed openings in `TestLanAskAndAnswer` were lost connections
   misread as refusals, and they reached `ClosePeer` (`docs/autonomous-decisions.md`, 2026-11-23). Under P-16, a
   misclassification closes only a connection that has already failed. The refused-or-lost classification
   (`openingLost`, `outEndedByConn`) stops being safety-critical. No new heuristic is needed.
6. **Prior art.** HTTP/2 scopes an error to the unit it corrupts: a stream error "does not affect processing of other
   streams", while a connection error renders "the entire connection unusable" (RFC 9113 §5.4). QUIC does the same
   (RFC 9000 §11). P-16 applies this one level up: connection, not peer.

Alternatives considered:

- (A) `ClosePeer` for all causes (the old text and code): rejected, see points 1-5.
- (C) Hybrid, with `ClosePeer` on a violation only: a violation is the peer's doing. But `ClosePeer` neither bans the
  peer (it redials, and we redial a wanted member) nor protects anyone else. It would add a second scope for no
  guarantee. Left as a question for the review.
- (D) Reset only, keeping the connection: libp2p would pick the same connection again, one 15 s budget after another.
- (E) Mark the connection bad and route around it: this needs a connection-state table, and leaves a half-dead
  connection that DCUtR and identify still choose (the H-20 problem).

A peer that refuses on its own account (it does not admit us, it lacks `/inkwell/sync/2.0.0`) refuses on every
connection. They close one by one, one opening each: at most the connections to the peer (2 in steady state, §16.1).

## 3. Rule text (applied in this step, French)

§8.7.4 gets **« Portée d'un échec (règle P-16) »**. The session's stream opens on a connection the node names
beforehand: the one whose arrival causes the opening, else the one libp2p would choose. It opens *on that connection
itself*, so a failure before any stream exists (deadline, refused negotiation) still names what to close (review
finding 1). A refusal, a violation or the deadline, before or after the ACK, closes that connection alone and MUST NOT
close the others. **P-17**: any other connection still open after that close opens the session, with no event
(review finding 4, also in the §8.7.5 rows). §8.7.6, §8.7.8, §8.7.9 and the §16.1 row are aligned; `CHANGES.md` has a
« Bloc PU1d » entry.

## 4. Model (applied; TLC 1.8.0, 2 workers, `heavy.sh`)

`tla/CloseScope.tla` extends `PeerUpAck` without editing it, as `Supersede` does. `PeerUpAck` is frozen by
`gates/PU1b.toml` and `gates/PU1c.toml`, so the runs are named `PU1d*` and PU1's 19 `^PeerUpAck` runs keep their
count. The session-failure branches close `CloseNamed`: the named connection (`FailClose`), or, if none is named, all of
them (`CloseAll`, the code before PU1d). `Names(st)` is the connection an opening names. `StartOpen` is guarded by
`OpenGuard`. `evt` abstracts libp2p's connectedness event. The comment on §8.7.3 step 3, the responder's reset on a
connection it is closing, is in the module header (finding 2).

| Run | Result |
|---|---|
| `PU1dCloseScopeAny` (safety: S1-S4, S6 `CrossedIndependent`, S7 `OpeningNamed`) | exhaustive, 7,407,601 states, 1,345,391 distinct, depth 34, 33 s |
| `PU1dCloseScopeNegotiated` (the same, protocol not cached: a deadline before NewStream returns) | exhaustive, 17,257,071 states, 2,484,923 distinct, depth 37, 83 s (with a Drop: 14.8 M distinct, 9 min, no error) |
| `PU1dCloseScopeLive` (S1-S4, S6, S7, L3 `NeverStranded`, L2 `OpenResolves`) | exhaustive, 63,469 states, 19,957 distinct, depth 28, 7 s |
| MC1 `ClosePeer` (`FailClose <- FailClosePeer`) | `CrossedIndependent` violated, depth 13 |
| MC2 opening only on a peer event (`OpenGuard <- OpenOnEvent`) | `NeverStranded` violated |
| MC3 named only at NewStream's return (`Names <- NamesAtReturn`) | `OpeningNamed` violated, depth 4; its consequence run, `CrossedIndependent`, depth 14 |
| witnesses `NoFailBeside`, `NoQuietOpen`, `NoUnreturnedFail` | violated (S6, L3, S7 not vacuous) |

## 5. Vectors (applied)

The generator (`protocol/cmd/vectors/syncack.go`, `syncorder.go`) now writes `close_conn` in place of `close_peer`
in 13 vectors: `sync-ack-opener-{eof,reset,bad-byte,byte-after,deadline,slow-negotiation,deadline-before-return,silent,
reopen-refused}`, `sync-refusal-{busy-out-of-budget,other-code,negotiation-code,eof}`. `TestGenerateMatchesCommittedCorpus`
holds after regeneration. The two `TestCorpus` stay red until step 2, on exactly these 13 (`close_peer, want close_conn`).

## 6. Code expected in step 2 (not made)

- `protocol/syncack`: `ClosePeer` becomes `CloseConn` ("close_conn"), and `ClosePeerAction` becomes `CloseConnAction`.
- **Named connection (finding 1).** Today `openOut` calls `BasicHost.NewStream(ctx, p, proto)`
  (`L/p2p/host/basic/basic_host.go:441-520`). The swarm picks the connection (`swarm.go:475-492`, `bestConnToPeer`).
  On an identify or negotiation failure the stream is reset and nil is returned (« connection failed » when no
  connection), so `on` is nil. Step 2 chooses the connection first: `onConn`'s `c`, else from
  `Network().ConnsToPeer(p)` (`swarm.go:582`) in `isBetterConn`'s order. `isBetterConn` is unexported, so its three
  keys are rebuilt from `Conn.Stat().Limited` (`core/network/network.go:121-124`), `isCircuit` and `len(Conn.GetStreams())`
  (`core/network/conn.go:85`). Step 2 then opens on that connection through `network.Conn.NewStream(ctx)`
  (`conn.go:82`, `swarm_conn.go:212`), as `newStreamOn` already does (`stream.go:603-619`: `SetProtocol`, then
  `multistream.SelectProtoOrFail`). `openAcked` always gets a route, so `on` is never nil. Negotiating gives up the
  optimistic path but should add no round trip: the responder writes the ACK right behind its echo. PU1's
  `TestPeerUpLatency` in `regression` checks this.
- On failure: reset, `on.Close()`, then `startOpenLocked` on another live connection, the lost path of :118-125.
  `outLost` with `!reopen`: close `s.Conn()`, reopen elsewhere, the peer stays up. The Drop paths
  (`node.go:797,933,955,985`) keep `ClosePeer`.
- Log P-15 ping and close events (HP1 residual → PU1d).

## 7. Gate (draft `gates/PU1d.toml`, frozen by the integrator; witnesses in `gates/PU1d/WITNESSES.md`)

`docs`; `tla` (10 runs, 139 s, PeerUpAck byte-identical to `c7c90c7`); `vectors` (step 1's digests, no `close_peer`, three
corpus tests); `go`, seven synctest tests: `TestFailedOpeningClosesOnlyItsConnection`,
`TestDeadlineBeforeReturnClosesNamedConnection`, `TestDeadlineOnStalledCircuitKeepsDirect`,
`TestResetBeforeConnCloseKeepsOther` (finding 2: a reset that arrives before the opener learns its connection
closed), `TestPeerRefusingEveryConnection`, `TestByteAfterAckClosesItsConnection`, `TestRemainingConnectionOpens`;
`mutants`, Go mutants MD1 (ClosePeer back), MD2 (host NewStream back), MD3 (no reopening); `mutation` of
`protocol/syncack` ≥ 95 %; `regression` (`internal/transport/p2p`, `internal/app`); `netsim`, scenario 9 crash, seed 20771,
strict, C1d/C2d/C3k; `simplify-report`. Caps: 2700 s = 45 min (G-1). **Contract change to record** (method v2 rule 9):
the `vectors` criteria of `gates/PU1.toml` and `gates/PU1b.toml` pin the old digests and are superseded by PU1d's.

Still routed here by HP1: the exhaustive TLC check of P-15 on three connections (only a seeded simulation today).

Sources: `L/` files above; RFC 9113 §5.4, RFC 9000 §11; libp2p `relay/circuit-v2.md`
(github.com/libp2p/specs); `2026-11-23-hp1-reformalize.md` §2; `docs/autonomous-decisions.md` 2026-11-23.
