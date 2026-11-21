# WC1 step 1 witnesses

Baseline `8980890` on `wc1`. Replayed on 2026-11-21 after the second Opus review and the
integrator’s decision to let the first close event consume the approved attempt’s permit.
The runs below execute the six decoded commands in [WC1.toml](../WC1.toml)
individually. TLC and Go ran through `heavy.sh`. The sandbox required reserving a host slot
with a read-only descriptor and giving the wrapper a writable private lock path.

## Execution

| Criterion | Exit | Wall time | Observed witness |
|---|---:|---:|---|
| `tla` | 0 | 27.31 s | TLC exhausted all three state spaces and checked liveness. Mixed had 821,216 distinct states at depth 33, healthy quit had 1,006 at depth 21, and absent renderers with explicit force had 15 at depth 9. All queues emptied. |
| `go` | 1 | 1.25 s | W-5 failed at the behavioral assertion with `StatusNotFound (1)`. The witness lets the old stream return its already-received save frame after the replacement attaches. The new stream reads from the same atomic store. The gate also rejected the five absent W-1 through W-4 and W-6 product tests → WC1. |
| `webkit-native-e2e` | 1 | 0.04 s | Preflight rejected both WebKit `fixme` cases and the absent native runner → WC1. The separate native probe reached the real close route below. |
| `mutants` | 0 | 6.19 s | TLC rejected all ten mutants on the named invariant or temporal property. |
| `docs` | 0 | 0.03 s | All six rules, both regression IDs, liveness guarantees, the consent invariant and the requirements entry were present. |
| `simplify-report` | 1 | 0.04 s | The step-3 review report is absent → WC1. |

The six commands took 34.86 s of wall time in total, outside slot wait and the native probes.
The configured caps total 1,560 s. With 60 s of kill grace the 30 min gate leaves 180 s
of margin. `pregate` passed, including the frozen-dependency guard. WC1 uses a dedicated
TLC runner and the shared pinned distribution. Inventory reported zero unfiled items.

## Formal counterexamples

| Mutant | Observed rejection |
|---|---|
| `WindowCloseSkipWait` | Request, type, prepare, authorize and close before writing violate `NoAcknowledgedKeystrokeLost`. |
| `WindowCloseTimeoutCloses` | The deadline fires before preparation. Closing with edits still only in the window violates `NoAcknowledgedKeystrokeLost`. |
| `WindowCloseNoGoodbye` | A durable save ACK and the launch handling complete. Authorizing and consuming without Goodbye violates `GoodbyeBeforeClose`. |
| `WindowCloseStaleAck` | The first attempt completes its ACK and then expires. Using that ACK after a fresh request and prepare violates `CloseWaitedForAck`. |
| `WindowCloseTypeWhileHeld` | Request, prepare, then accept another edit, violating `InputFrozen`. |
| `WindowCloseAckBeforeWrite` | After a dirty cut is prepared, ACK before Write violates `AckIsDurable`. |
| `WindowClosePartialQuit` | Request both windows, complete one and issue its permit while the other is still draining, violating `GroupWaitedForAll`. |
| `WindowCloseNoWrite` | A dirty healthy close stalls after prepare and launch handling, violating `HealthyCloseSucceeds`. |
| `WindowCloseForceNeedsLive` | A dead renderer aborts into the notice. The person chooses force, but permit issuance stalls, violating `ForceClosesCrashedWindow`. |
| `WindowCloseForceWithoutConsent` | Renderer loss leads to the notice. Force issues a permit before the person chooses force, violating `OnlyWithConsent`. |

## Native baseline and oracle checks

The earlier temporary Wails v3.0.0 fixture ran under headless Mutter 51.1 on Wayland,
GTK 4.24.1 and WebKitGTK 2.54.2, with a private session bus and populated temporary
`INKWELL_DOCS` and `INKWELL_HOME`. The page sent readiness over Wails Streams. An
external driver sent Alt+F4 to the compositor. The fixture observed GTK's
`close-request` as Wails' Linux delete event, followed by a common closing hook that
observed the event without vetoing it. The native loop returned after teardown, the
OS supervisor observed exit 1, and Go received no `pagehide` marker. The original
diagnostic with Go initiating `Close()` also replayed and exited 1 for the missing marker.

The replayed native report validator accepted a synthetic complete lifecycle and rejected
13 negative controls, including a reload or Go close substituted for the WM action,
missing GTK signal, veto, destruction or exit, a reused PID, an abnormal exit, a
mismatched close request, permit consumption before approval or twice, and skipped browser
or forced-close tests. These checks exercise the validator. Step 2 still owns the actual
desktop-app run with an approved close permit, OS-confirmed clean exit and relaunch into the
original project, plus the presence and recovery cases → WC1.
