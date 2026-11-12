# /simplify — block RC1 (reconnect after restart without a relay) — DRAFT by go-dev (2026-11-12)

Draft written by the implementing agent; the session's own `/simplify` pass confirms or amends it before the
marker is armed. Spec: `docs/plans/2026-11-12-rc1-reconnect.md`. Frozen gate: `gates/RC1.toml`.

Scope (RC1 lines only; several files also hold uncommitted block C5S changes):

- new: `internal/transport/p2p/addrbook.go` (codec, policy, coalesced saves, `Node.member`/`remember`),
  `internal/app/addrbook_file.go` (the `BookStore` on the home directory);
- changed: `internal/transport/p2p/node.go` (`Options.Book`, load in `New`, the save goroutine, `Drop` forgets the
  entry and saves at once, new `Hangup`, `forgetAddrs`), `dial.go` (book fallback in `dialOnce`, `Want` records the
  wanted peers that are up), `stream.go` (record on PeerUp), `mdns.go` (record a member's sighting), `relay.go`
  (the test-only address rewrite also applies without relays; production behaviour unchanged: `rewrite` is set by
  tests only and RelayOnly needs relays), `internal/app/node.go` (open the store, pass it, close it after the node),
  `internal/app/invites.go` (the LAN probe calls `Hangup` and skips linked peers; `lobbyNode` gains `Hangup`), `internal/app/wiring.go` (`Admission.Scope`);
- tests: `addrbook_test.go`, `reconnect_test.go`, `addrbook_file_test.go`, `addrbook_file_unix_test.go`; `sim_test.go` (`nodeKeyAt`),
  `export_test.go` (`MemBook`, `BookAddrs`, `SightLAN`, `BookFile`, `snapshot`);
- `scripts/netsim/mutants.toml`: MC1a to MC1af (MC1o to MC1af beyond the gate's list, run by its `netsim:mutants -- RC1`); `gates/guard-overrides.json`: one entry (Windows skip of
  `TestAddrBookFileMode`).

## Reuse

- Atomic write: `store.WriteAtomic` + `store.SyncDir` (same primitive as the trust store), not a new helper.
- Peer ID validation: `KeyFromID`/`parseID` (canonical form, Ed25519, small-order check) instead of new checks.
- Circuit detection: the shape check of `bookAddrOK` already excludes circuits; `isCircuit` is not duplicated.
- Restart in the sim: one parameter split of the existing `nodeAt` (`nodeKeyAt`), no second fixture.
- `encoding/json/v2` gives duplicate-name, invalid-UTF-8 and trailing-byte refusal for free (no hand checks).

## Simplification (applied)

- `Want` lost its `defer Unlock`, so the recording of up peers runs after the lock without an unlock/relock pair.
- `snapshot` moved from production code to `export_test.go` (only tests use it); `cmp` import dropped.
- `flush`'s condition written positively (`due := …`), as staticcheck QF1001 asked.
- The save goroutine is started unconditionally and returns at once without a store, which keeps `New` under
  the gocognit bound instead of adding a branch.
- `dialOnce`: an earlier loop over `{known, book}` was replaced by a second explicit block, so the frozen line
  `n.dial(ctx, w.id, known, knownDialTimeout)` stays as it was (base guard) and the diff stays surgical.

## Efficiency

- Recording is O(addresses) per PeerUp, per `Want` of an up peer, per mDNS sighting; the book lock is a leaf (never
  held while calling libp2p or `Admit`).
- Disk: at most one save per 10 s on a change of the set, seen-only changes at most hourly or on Close; Drop saves
  at once (rare: revocation). `saveMu` serialises snapshot+Save so an older snapshot never overwrites a revocation.
- The dial schedule is unchanged; the book attempt reuses `knownDialTimeout`, and addresses already in `known` are
  not dialed twice. Not a HOT_PATHS change for `internal/app` (startup and close only): no benchmark needed.

## Altitude

- The book lives in `internal/transport/p2p` (multiaddrs, node events; AC-3/AC-4); persistence is injected
  (`BookStore`), the file store lives in `internal/app`. `mise run arch` is in the gate.

## Deviations from the plan, with reasons (to confirm)

1. `BookStore` has a third method, `Quarantine(reason string) error` (a category, see the security review, 10):
   the decoder (p2p) decides that a file is refused, but only the store can move it to `.bad`.
2. The peerstore is **not** seeded from the book in `New` (plan §2.4): `dialOnce` passes the book addresses to
   `h.Connect`, which adds them to the peerstore itself, so seeding was a second copy of the same data. It also
   lets `TestReconnectAfterLongOutage` catch MC1i (fallback reading the peerstore only), which a seeded peerstore
   would hide.
3. Seen times never refuse a book (security review, 3): no 2020 floor, a future time is brought back to now, a
   negative one is a bounds error.
4. `Want` records the addresses of wanted peers that are already up: on an invitation the PeerUp happens while the
   newcomer is not yet a member (`Admit` refuses it then), so the PeerUp hook alone would never record it; the list
   change that follows calls `Want`. Found by `TestRestartReconnectsMembers`.
4b. Source of "announced" addresses: the listen addresses of identify's `EvtPeerIdentificationCompleted` (kept per
   connected peer in `Node.heard`, dropped on disconnect), not `Peerstore().Addrs`: the peerstore also holds every
   address libp2p merely dialed (TempAddrTTL), so a `Connect` to forged mDNS addresses of a connected member would
   have been recorded as announced at the next `Want`. libp2p does not keep the signed peer record in the peerstore,
   hence the event. Loopback addresses are kept only for a peer reached over the loopback interface (identify's own
   filter). The subscription joins the existing connectedness one: `New` gains no branch.
4c. mDNS is unauthenticated (coordinator review): an entry keeps the authenticated addresses (stored) apart from the
   mDNS ones (memory only, at most 2, only in the room the authenticated ones leave, dialed after them); a peer seen
   by mDNS alone is evicted first and never evicts an authenticated entry. Proven by
   `TestBookLANCannotOutrankAuthenticated` and `TestBookLANSpoofKeepsAuthenticated`; mutants MC1o, MC1p.
4d. A sighting of a wanted, unconnected member wakes its dial loop (a per-loop channel that ends the pause without
   resetting the backoff), at most every 10 s (`wakeEvery`), so forged sightings cannot make it dial in a loop.
   `TestReconnectOnLANSighting`; mutant MC1q.
5. Observability: `Options.Log` (the app's logger) receives the book's diagnostics: entries loaded, refusal
   category, entries left out of a save, save failures, the clock hold; never an address nor a peer ID.
6. File format: each entry gains `"in"` (1 to 8 project names), see the security review, 2.

## Residual

- A member is recorded when identify's event is handled, asynchronously, a few milliseconds after its PeerUp: a node
  closed inside that window does not record it (found as a 1-in-100 flake of the restart tests, whose setup now
  waits for both books; in the app a session lasts far longer).

- A forged sighting can still fill the 2 mDNS places of a member's entry and cost one dial attempt per 10 s at most;
  the authenticated addresses are dialed first and are never displaced, and a dial authenticates the peer ID.

## Security review (independent, 2026-11-12): finding, fix, test

| # | Finding | Fix (root) | Proof |
|---|---|---|---|
| 1 | Any `/dns*` value recorded; an invalid UTF-8 name made `json.Marshal` fail and `flush` re-set `changed` forever: nothing saved for anyone | DNS only as ASCII LDH hostnames of 253 bytes at most (`validHostname`); `bookAddrOK` requires the text to parse back to the address; `encodeBook` encodes entry by entry, leaves a bad one out and counts it (logged as a count) | `FuzzBookNote` (note, encode, decode: whatever note takes is saved and read back; seeds: invalid UTF-8, `/`, NUL), 1.2 M execs clean; rows "dns not a hostname", "dns label hyphen"; mutant MC1r |
| 2 | Load dropped the peers the open project's Admit refuses, and Drop erased an entry: a per-device book lost the members of the other projects (switch, failed join) | Entries carry the projects (`Options.Scope`, the app's project ID) they were seen a member of; load filters nothing; Drop removes only the current project, only when Admit already refuses the peer (revocation), and the entry once no project keeps it; a Drop for a fault keeps it | `TestBookPerDevice`, `TestBookPerDeviceAcrossProjects` (two projects, a switch, a fault Drop, a revocation in one then the other); mutant MC1s |
| 3 | A clock set back quarantined the whole book; a jump forward of more than 90 days pruned and saved it empty | Per-entry rules: a future seen is brought back to now; if any was, or every entry would be pruned, the book is kept as it is and unsaved (`hold`) until a member is seen, then the usual rules apply; the 2020 floor is gone | `TestBookClockRules` (ten years back; 100 days forward); mutant MC1t |
| 4 | Each wake (every 10 s per peer) ran a full `dialOnce`, relays included | A woken loop dials only the addresses the sighting gave (`dialLAN`, no relay); a node-wide token bucket (4, then one per 10 s) on top of the per-peer bound | `TestReconnectOnLANSighting`, `TestBookLANWakeBucket`; mutants MC1q, MC1u |
| 5 | mDNS addresses unrestricted | `lanAddrOK`: private or link-local IPv4, unique-local IPv6 literals only; no loopback, public or DNS. The packet's source is not available through libp2p's mDNS notifee: not matched (residual) | `TestBookLANCannotOutrankAuthenticated` (refused rows); mutant MC1v |
| 6 | Identify's listen addresses recorded raw | The signed peer record's addresses when identify carried one (its signature checked by identify), else the listen addresses; filtered as identify does (`reachableFrom`: a peer reached publicly keeps its public addresses only, no loopback unless reached on loopback) | `TestBookReachableFrom`, `TestBookPrefersSignedRecord` (a sealed record of the peer wins over the listen addresses, one of another peer is ignored; `announced` is the pure part of `onIdentified`); mutants MC1w, MC1aa |
| 7 | The probe greeted connected members; a failed Hello hung up a working link | The probe skips linked peers (`probeTargets`) | `TestLANProbeKeepsMember` (AST + unit); mutant MC1x |
| 8 | `remember` checked membership outside the book's lock: Drop then forget could be undone by a racing note | `note`/`noteLAN` take the membership check as a func and call it under `b.mu` | `TestBookNoteRechecksMembership` (asserts the check runs under the lock, then revoke, forget, late note); deterministic, not synctest: the interleaving is a lock, not a blocking point; mutant MC1y |
| 9 | Authenticated and LAN addresses dialed in one attempt | Two attempts: authenticated, then LAN (`dialAddrs` returns them apart) | `TestDialFallbackOrder` (a test-only dial recorder, `Options.onDial`, set by `SetOnDial`: the book's authenticated address first, then the mDNS one, in two attempts); mutant MC1ab |
| 10 | `Quarantine(cause)` unused | `Quarantine(reason string)`: size, syntax, version or bounds; logged by the store; the cause text (it may quote an address) is never logged | `TestBookDecodeRejects` checks every row's category; `TestBookPerDevice` the quarantine's |
| 11 | `Load` opened without checking for a regular file: a FIFO hung Start | Opened `O_NONBLOCK`, then `Stat`: anything but a regular file is not read | `TestAddrBookFileNotRegular` (unix); mutant MC1z |

## Second review (Codex, adversarial, 2026-11-12): finding, fix, test

| # | Finding | Fix (root) | Proof |
|---|---|---|---|
| 1 (High) | Forged sightings starve authenticated attempts: each wake abandoned the backoff timer and restarted it | `pause` keeps an absolute deadline (`due`) for the next normal attempt across wakes; a wake runs one LAN attempt and the next pause ends at the same instant; cleared when a pause ends normally, on a kick, or when a dial succeeds. Found while proving it: libp2p's `Connect` dials every address its peerstore holds (earlier attempts leave theirs for 2 minutes), so a LAN-only attempt also dialed the authenticated address, which hid the starvation (mutant MC1ac survived) and made findings 4 and 9 hold in name only; `dial` now clears the peer's peerstore addresses first (it dials only peers that are not connected), so an attempt dials exactly what it names | `TestReconnectUnderSightingFlood` (a forged sighting every second for 3 min, the member then reachable at its authenticated address: up within 90 s + one LAN attempt, 20 seeds); mutant MC1ac. Guard override: the timer line is rewritten, same jittered backoff |
| 2 (Med) | A member with 8 authenticated addresses had no room for mDNS hints: after it moved, a valid sighting gave nothing to dial | mDNS hints have 2 places of their own (`bookMaxLAN`), never taken from nor evicting the authenticated 8 | `TestReconnectRoamedSaturated` (8 stale addresses, a sighting on the LAN: up within 5 s, 20 seeds), `TestBookLANCannotOutrankAuthenticated`; mutants MC1ad, MC1p |
| 3 (Med) | A failed save restored `changed` but scheduled no retry | `run` retries a failed save on its own: after 10 s, then doubling, up to 10 min (`bookRetryMax`); a failed save from `Drop` hands over to `run` | `TestBookSaveRetried` (two failures then success, no further note nor shutdown); mutant MC1ae |
| 4 (Med) | During a clock hold, a revocation's save and the shutdown save were skipped: the revoked peer came back | `forget` marks a revocation; `flush` saves it during the hold: the book as loaded (seen times unclamped: clamping now happens when the hold ends), minus the revoked entry, unpruned | `TestBookClockRules` (revocation + reload under both holds); mutant MC1af |

