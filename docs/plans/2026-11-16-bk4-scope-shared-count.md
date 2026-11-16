# BK4 — the W58a test sent before the sender had announced its peer

Rigour: light (a test defect in `frontend/src/net`; no product code changes).

## Symptom

`WailsNetworkAdapter.test.ts > W58a a document outside the admitted scope is not counted as shared` (block W58a,
fdfd030) failed once in a full `fe:check` under load: after `send("d1")`, `heardHeads("b").has("d1")` was false.

## Cause, with evidence

The adapter records a document as shared with a peer only for a peer it has announced (`#shared`:
`!this.#announced.has(peer)` returns early). The test waited on `b.connected.has("a")`, which is **b's** view: b
announces a as soon as a's « arrive » reaches it, while b's own « arrive » (or « welcome ») is still in flight to a. The
two other conditions of the wait were vacuous (`heardHeads` and `syncCounts` never return `undefined`). When the test's
5 ms poll ran in that gap (a starved event loop spreads the two handshake timers over different milliseconds), `send`
went to a peer a had not announced yet, so nothing was recorded.

Ruled out by the code: a reconnect (SimNet never closes a socket; `onclose` comes only from `close`), buffering
(`#sendTo` writes at once, there is no queue), a changing key (keyed by peer id). Measured: 0 failures in 200 loops on a
quiet machine; with the frames to a held (forced gap), the old wait fails every time.

Product or test: test. automerge-repo only sends to peers it learned through `peer-candidate`, which the adapter emits
exactly when it adds the peer to `#announced`, and `#gone` removes it with `peer-disconnected`; so the repo never sends a
sync message to an unannounced peer, and the indicator cannot miscount this way.

## Fix

The test holds the peer messages delivered to a (`holdPeerMessages`, control frames pass), waits for b to see a, asserts
that a has not announced b yet (the interleaving is reached), releases, and waits on the sender's own announcement
(`adapter.announcedPeers().includes("b")`) before sending. No assertion weakened, no delay added. Proof: the red variant
(old wait, frames held) fails; the fixed test passes 500 repeats; full Vitest suite, Biome and svelte-check green.
