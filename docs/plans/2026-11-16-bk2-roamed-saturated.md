# BK2 — the roamed-member test asserted a bound the dial loop never promised

Rigour: light (a test defect in `internal/transport/p2p`; no product code changes).

## Symptom

`TestReconnectRoamedSaturated` (`internal/transport/p2p/reconnect_test.go`, block RC1, 23879bd) failed about once in
20 runs (9 in 200 with `-race`): « Recv(<peer>): context deadline exceeded » from the 5 s wait for `PeerUp` after a LAN
sighting. The test is not on real time: `forSeeds` runs each seed in a `synctest` bubble, so machine load cannot change
its outcome. Reproduced without `-race`: 23 failed seeds out of 8000.

## Cause, with evidence

A sighting wakes the dial loop only at its pause (`pause` in `dial.go`, `case <-wake`); a round already dialling the
book is not cut short, and that round read the LAN addresses before the sighting, so it does not dial them. The
sighted address is dialled after the round fails: the rest of the round plus a handshake. With 8 stale addresses a
round fails in 5.25 s (quic-go v0.64.1 `DefaultHandshakeIdleTimeout` = 5 s, `internal/protocol/params.go`, plus the
swarm's address staggering; under `knownDialTimeout` = 10 s), so the 5 s bound fails exactly when
a round starts less than ~0.3 s before the sighting. The jitter of the backoff comes from the unseeded global
`math/rand/v2`, hence intermittent. A probe (8000 seeds, not kept): every one of the 8 cases above 5 s (max 5.28 s) had
a stale round starting in (179.69 s, 180 s), and the LAN dial came 5.25 s after it.

## Product or test

Test. Not cutting an attempt short is deliberate: sightings can be forged by anyone on the local network and must not
cut the authenticated attempts (second review 1, `TestReconnectUnderSightingFlood`). Once sighted, the book holds a
valid address, and the product meets bound R of the RC1 plan §6 (`knownDialTimeout` + 5 s). The 5 s figure was copied
from `TestReconnectOnLANSighting`, where no address means no round is ever in flight. Set aside: cancelling the round
(starvation by forged sightings), dialling the LAN address alongside it (the swarm keeps one active dial per peer,
`p2p/net/swarm/dial_sync.go` of the pinned go-libp2p fork, and `dial` clears the peerstore per attempt: the two would
merge and break the attempt order of security review 9).

## Fix

The test takes both instants from the dial loop itself (`p2p.SetOnDial`), so every seed meets each case:
`inPause`, sighting `knownDialTimeout` after a round starts (round failed, loop sleeping): up within the handshake
allowance, 5 s, as before; `atRoundStart`, sighting as a round of stale addresses starts: up within
`reconnectBound` = `p2p.KnownDialTimeout` + 5 s (now derived from the product constant, exported in `export_test.go`).

## Proof

Red first: with 5 s in both cases, `atRoundStart` fails 20/20 seeds with the original message, `inPause` passes 20/20.
A mutant that drops the wake fails both cases, 20/20 each. Green (2026-11-16): `-count=200 -race` on the test, 200/200
(830 s); `-count=2000` without race, 2000/2000 (1512 s; a first attempt hit `go test`'s default 10-minute deadline,
not a failure); the whole package with `-race -timeout=40m`, ok (472 s); `mise run lint` clean. The 2000-run under
`-race` first planned was stopped: each seed now meets both cases deterministically, so it proved little more while
holding a heavy slot for hours.
