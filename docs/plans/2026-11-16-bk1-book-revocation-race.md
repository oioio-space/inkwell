# BK1 — the per-device book test switched projects before the note of the connection

Rigour: light (a test defect in `internal/transport/p2p`; no product code changes).

## Symptom

`TestBookPerDeviceAcrossProjects` (`internal/transport/p2p/reconnect_test.go`, block RC1, 23879bd) failed about once in
200 runs with `-race`: « revoked in project B, the member of project A left the book (stored: {"v":1,"peers":[]}) ».

## Cause, with evidence

The test opens project A, waits for b's `PeerUp`, then switches the scope to B. But `PeerUp` is pushed under `n.mu` in
`stream.go` (`pushLocked`) and the book note comes after, outside the lock (`n.remember(ps.id)`, which reads
`n.scope()` when it runs, `addrbook.go` `remember`); the identify path (`onIdentified` → `remember`) is asynchronous
too. `synctest` controls time, not the order of runnable goroutines: when the test goroutine runs first, every note of
the connection reads scope « B ». b's entry is then `in: [B]` only, the revocation in B empties it (`forget`, correctly),
and the book is flushed empty — exactly the message above.

Forced schedule (a probe, not kept): a `Scope` callback that blocks every call until the test has switched to B
reproduces the failure on every run, with the exact message and `{"v":1,"peers":[]}`; two scope reads (the two notes)
were pending when `PeerUp` arrived. The same probe releasing the notes and calling `synctest.Wait()` before the switch
passes.

## Product or test

Test. The book records a project only together with a membership check made for it (`note` takes the scope and
`member` at the same moment); a note that ran after the switch has verified B, not A, so it must not record A. Nothing
the book had recorded is lost. `PeerUp` never promised the book was written. Residual, noted, not a defect: a project
switch within the instant a connection is being noted records the new project only; the next `Want` under A records A
again (`Want` notes every connected wanted peer).

## Fix

`synctest.Wait()` after the `PeerUp`, before the switch (`reconnect_test.go`), so the connection's notes complete under
A, as the test's premise (« b a member of both ») requires. No bound or delay changed. `synctest.Wait` « blocks until
every goroutine within the current bubble, other than the current goroutine, is durably blocked »
(https://pkg.go.dev/testing/synctest#Wait): the host's goroutines live in the bubble (the net is simulated), so on
return both notes have run. Set aside: a product hook « book written » (an API for a test only), or polling `Data()`
(the in-memory note is not flushed until the save timer).

## Proof

- Forced schedule above: red on the old ordering on every run, green with the notes settled first.
- `go test -count=2000 -race -run '^TestBookPerDeviceAcrossProjects$'`: 2000/2000 pass (134 s), machine at rest.
- Whole `internal/transport/p2p` with `-race` (105 top-level tests, 611 passes with subtests): ok in 604 s; it needs
  `-timeout=40m` because the race build of the full, non-short package exceeds `go test`'s default 10 min (CI runs it
  `-short -timeout=8m`). `mise run lint`: clean.
