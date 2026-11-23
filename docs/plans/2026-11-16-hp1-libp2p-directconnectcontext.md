# HP1 - the fork change: `(*holepunch.Service).DirectConnectContext`

Patch: `docs/plans/2026-11-16-hp1-libp2p-directconnectcontext.patch` (`git diff` against
`oioio-space/go-libp2p@cae49c803966`, the pinned fork). **Landed in the fork at c37a183f** (2026-11-16, branch
`inkwell/v0.51.0-nilfix`, commit `c37a183f1f1e0230d9df7b26f9644979f6f69da2`, the holepunch package passing under
-race); `go.mod` pins it as `v0.51.1-0.20261116140521-c37a183f1f1e`, whose `p2p/protocol/holepunch` is identical to the
local copy the block was developed on. Filed upstream on 2026-11-23, after HP1 merged (the user's go-ahead): issue https://github.com/libp2p/go-libp2p/issues/3585, pull request https://github.com/libp2p/go-libp2p/pull/3586 (branch `holepunch-directconnect-context` of the fork, the commit cherry-picked onto upstream master 8582b880, v0.52.0; holepunch tests pass under -race). Same day: ping on #3576 (nil transport in `nonProxyAddr`, still unguarded on master) and support on #3547 (DCUtR destinations filtered to public addresses).

To land it (integrator; steps 1 and 2 done on 2026-11-16, step 3 waits for the user's go-ahead):

1. in a clone of `github.com/oioio-space/go-libp2p` at `cae49c803966`: `git apply` the patch, commit
   (« holepunch: add Service.DirectConnectContext »), push to the fork;
2. in `go.mod`, replace `../go-libp2p-hp1` by the fork's new pseudo-version
   (`go get github.com/oioio-space/go-libp2p@<commit>` then `go mod tidy`), and restore the comment;
3. file the upstream issue and pull request below.

## Upstream issue

Title: holepunch: DirectConnect cannot be cancelled and blocks for ever if the service closes first

`(*holepunch.Service).DirectConnect(p)` takes no context. A caller that wants to retry DCUtR (a peer that
restarted while relayed, #3420) cannot end a call towards the old process: the next call fails with
`ErrHolePunchActive` until the old one has run its three rounds (up to ~15 s of punching plus the stream
timeout). And `DirectConnect` waits on `hasPublicAddrsChan`, which is never closed if the service is closed
before the host gets a public address: the call then never returns.

## Upstream pull request

Title: holepunch: add Service.DirectConnectContext

Adds `(*Service).DirectConnectContext(ctx, p)`, `DirectConnect` bounded by `ctx`. When `ctx` is done the call
stops waiting for a public address, ends its direct dial, its hole-punching stream (reset) and its punch, and
returns `ctx.Err()`; the per-peer slot is released before it returns, so a new attempt can start at once. It
returns `ErrClosed` if the service closes while it waits for a public address (fixing the hang above).
`DirectConnect` now calls it with `context.Background()`; the automatic attempt on inbound relayed
connections is unchanged (it still ends with the service).

Tests: `TestDirectConnectContextWaitEnds` (context, close while waiting for a public address) and
`TestDirectConnectContextCancelReleasesPeer` (a stalled responder: the call returns within a second of the
cancel, and the next call does not get `ErrHolePunchActive`). The package's tests pass under `-race`.
