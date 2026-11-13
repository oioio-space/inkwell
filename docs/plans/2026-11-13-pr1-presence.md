# PR1 - presence correctness (R1, W46 item 9)

Rigour: standard (method v2 rule 4). Scope: stuck presence, duplicate colours, 45 s departure. Attribution and
"follow" stay deferred (docs/autonomous-decisions.md, row "release R1"). Study: docs/plans/2026-11-10-collab-presence.md.

## 1. Need

A first user judges collaboration by the avatar stack and the carets: a person who left must disappear, two people
must not look alike, and what is shown must be true. None of the three holds today.

## 2. Causes (read in code, 2026-11-13)

| # | Cause | Evidence |
|---|---|---|
| C1 | Silent loss is noticed only by the TTL: 3 x 15 s = 45 s, plus a pruning tick of 15 s, so up to 60 s. inkwell passes neither `heartbeatMs` nor `peerTtlMs`. The transport knows at once (`peer-disconnected`) but presence ignores it. | automerge-repo 2.5.6 `presence/constants.js:1-2`, `Presence.js:219-224`; `editor/presence.ts:30`, `ui/sheet/presence.ts:28`, `App.svelte:187`; `workspace.svelte.ts:295` |
| C2 | Stuck names and files: `names` / `peerFiles` change only on a Presence event; `stop()` never empties them. Only `ui/sheet/presence.ts:97-98` sends goodbye on `pagehide`; the editor and `App.svelte:184` presences do not, so closing a window costs a full TTL. | `workspace.svelte.ts:607-609`; `editor/presence.ts:94-98` |
| C3 | Up to three Presence objects share one handle and one sender id (editor, sheet, App projection); each `hearNames` call overwrites the others' map. | `App.svelte:184-198`, `workspace.svelte.ts:607-611` |
| C4 | Duplicate colours: slot = FNV-1a(peerId) mod 8, blind to who is present: 59 % chance of a collision among 4 people. Used by carets (`remoteCursors.ts:66`), sheet carets (`ui/sheet/Grid.svelte:161,606`), the stack (`PeopleStack.svelte:25-30`), the panel (`PeopleDialog.svelte:112-113`), blame (`history/people.ts:56`). | `ui/people.ts` `personIndex` |

## 3. Rules (testable)

- P-1 Clean leave: a goodbye removes the person from roster, stack and carets at once (bounded by the message, not by any tick); `pagehide` sends it from the editor, sheet and App presences.
- P-2 Silent loss: removed within TTL + heartbeat of the last message. Values 5 s heartbeat, TTL = max(15 s, 3 x measured worst WebKitGTK hidden-window heartbeat interval) (section 6), so 20 s if the measure allows 15 s. The library's pruning is the only clock: the roster mirrors `getPeerStates()` and its `pruning` / `goodbye` events, no second TTL. Sources: Yjs awareness drops a peer after 30 s (https://docs.yjs.dev/api/about-awareness); automerge-repo `start({heartbeatMs, peerTtlMs})`, defaults 15 s / 45 s (https://automerge.org/docs/reference/repositories/ephemeral/).
- P-3 Link loss: `peer-disconnected` removes that person at once. Presence reaches `Presence` only through `WailsNetworkAdapter`, which delivers messages of peers it announced (`#announced`, `WailsNetworkAdapter.ts:66`, dropped at `:318`); the roster therefore never blocks a peer that was never disconnected (no full-mesh assumption: a relay-routed peer is in the roster as soon as the adapter announces it). That a relay-routed peer is announced by the adapter was not confirmed; P-3 is written so that an unannounced peer is never blocked. After a disconnect, nothing from that peer counts until its next `peer-candidate`.
- P-3b Quick reconnect: on `peer-candidate`, every live Presence re-broadcasts all its local channels (public `broadcast(channel, value)` over `getLocalState()`; the library's `broadcastLocalState` is private) so an idle person reappears at once instead of at the next typing.
- P-4 No presence survives its owner: after `stop()` of every Presence of a document, roster, `names`, `peerFiles` and carets hold nobody that no live Presence reports; the roster is the union of live Presences (C3).
- P-5 Colours: home slot = `personIndex(peerId)` (unchanged, FNV-1a mod 8, also blame/history). The live slot is a pure function of the set of present ids (me included): people are placed in ascending peer-id order; each takes its home slot or, if taken, the next free slot (wrapping). Among at most 8 present all slots differ, and two replicas holding the same present set compute the same slots whatever the order they heard them in. Divergence: replicas with different present sets (unsynchronised pruning, one-sided link loss) may show different colours for the same person until the sets agree, bounded by P-2 when links work.
- P-6 Stability, algorithm-independent: the people whose live slot changes at a join or leave are a subset of those whose live slot differed from their home slot before or after the change. A displaced person returns to the home slot as soon as it is free (follows from the placement rule).
- P-7 At 9 or more present, once all 8 slots are taken each further person (in ascending id order) takes their home slot; slots repeat and the name stays the second cue (docs/ux-principles.md).

Decision table (roster per event; "ignore" = state unchanged):

| Event | In roster? | Result |
|---|---|---|
| snapshot / update from a connected peer | any | add or merge |
| any message after `peer-disconnected`, before `peer-candidate` | any | ignore (P-3) |
| heartbeat | yes / no | refresh (library) / ignore (library rule) |
| goodbye | yes | remove (P-1) |
| library `pruning` (silence > TTL) | yes | remove (P-2) |
| `peer-disconnected` | yes | remove (P-3) |
| `peer-candidate` | - | live Presences re-broadcast (P-3b) |
| a Presence stopped | its peers only | remove those no other live Presence reports (P-4) |

## 4. Design and change list

Pure modules `frontend/src/editor/colours.ts` (`placeSlots(ids)`; the only user of `personIndex` besides blame) and `frontend/src/editor/roster.ts` (`createRoster`, constants `PRESENCE_HEARTBEAT_MS`, `PRESENCE_TTL_MS`, `slots()`), importing nothing outside the two files. Change list: `editor/presence.ts`, `ui/sheet/presence.ts`, `App.svelte` create Presences through one `startPresence(handle, initialState)` helper (heartbeat/TTL, `pagehide` goodbye, `peer-candidate` re-broadcast, feed the roster); `workspace.hearNames` reads the roster; `PeopleStack.svelte`, `PeopleDialog.svelte`, `remoteCursors.ts`, `ui/sheet/Grid.svelte` (161, 606) take colours from `slots()`; `history/people.ts` keeps `personIndex`. No new string or component; "has left" wording stays deferred.

Contract change (recorded, not a weakening): colours shown for displaced people change, so `e2e/people.spec.ts-snapshots/` baselines may be regenerated; each regenerated image is reviewed in the ux-walkthrough.

## 5. Test plan (cheapest layer first)

Real automerge-repo `Presence` over a fake handle, `vi.useFakeTimers` (the library's own clock), except the e2e.

| Rule | Layer | Named test | Mutant (in-test, tied to the rule) |
|---|---|---|---|
| P-2 | unit | `PR1 presence is started with heartbeat 5 s and ttl 15 s`; `PR1 silence removes by 20 s and not before 15 s` | TTL 15000 -> 45000 |
| P-1 | unit | `PR1 goodbye removes within one tick`; `PR1 pagehide says goodbye` | goodbye case removed |
| P-3 | unit | `PR1 transport loss removes at once and later updates are ignored` | disconnect handler removed |
| P-3b | unit | `PR1 peer-candidate re-broadcasts local state` | re-broadcast removed |
| P-4 | unit | `PR1 stopping a presence removes only what no other presence reports` | stop clears all |
| P-5 | property, fast-check, generated join/leave/lose/rejoin histories of 1-12 peers, ids random and some forced to collide, seeds 1-3 | `PR1 property: slots are distinct while at most 8 are present`; `PR1 property: two replicas with the same present set compute the same slots` (second replica hears the set in a permuted order) | collision step removed; ascending-id order -> arrival order |
| P-6, P-7 | property | `PR1 property: slot changes are within the displaced people`; `PR1 property: a displaced person returns home when the slot is free`; `PR1 property: at 9 or more present, further people take their home slot`; `PR1 property: no person shown after owner absent` | home slot not restored on leave |
| wiring | component test `PeopleStack.test.ts` with colliding peer ids (same `personIndex`) | `PR1 colliding ids show distinct colours in stack and panel` | slot lookup replaced by `personIndex` |
| wiring | grep criterion | `personIndex(` / `personColor(` appear in non-test source only in `ui/people.ts`, `editor/colours.ts`, `history/people.ts` | - |
| wiring | e2e `frontend/e2e/presence.spec.ts`, retries 0, `test.setTimeout(180_000)` (join about 60 s + bounds) | `PR1-E2E page close leaves within 3 s`; `PR1-E2E frozen server leaves within TTL + heartbeat + 3 s` (SIGSTOP via `signalOwned`, SIGCONT in `finally`; the page stays open and silent) | - |

Mutants follow the pattern of `highlight.equivalence.test.ts` ("LF0 the window-guard mutant is caught"): `roster.mutants.test.ts` reads `roster.ts` / `colours.ts`, asserts each anchor occurs exactly once, writes the mutated copy under `test-results/pr1-mutant/`, imports it, requires the rule's check to pass on the original and fail on the mutant; one `PR1 mutant <rule> is caught` test each. No runner extension (method v2 rule 12).

## 6. Timer throttling (measured, not assumed)

Browsers throttle timers in hidden pages (Chrome: once per minute after 5 min hidden, https://developer.chrome.com/blog/timer-throttling-in-chrome-88), which would stop heartbeats and show a live person as gone. The measurement is made during implementation and is the evidence the `timers` criterion checks (it is not a precondition of the freeze). Procedure: a heartbeat-gap logger in the app (dev build only, logs the gap between consecutive 5 s heartbeats) runs on the real WebKitGTK webview with the window minimised or fully covered for at least 6 min (past Chrome's 5 min threshold), machine idle; done by hand in the user session if the harness cannot hide a Wails window. The worst gap is recorded in `gates/PR1/timers.md` as `WORST_GAP_MS=` and `TTL_MS=max(15000, 3*WORST_GAP_MS)`. The gate checks `PRESENCE_TTL_MS` in code equals `TTL_MS`. If the gap exceeds 5 s the P-2 bound is TTL + heartbeat and the e2e reads it from the constants.

## 7. Gate time estimate (<= 45 min)

liveness 1 min; colours-model 3; mutants 5 (9 imports); usage-grep < 1 s; timers record check < 1 s (measurement done during implementation, not in the gate); component test 1; e2e presence 5; regression `fe:check` 10 + collab/people e2e 8; ux/simplify checks < 1 s. Total about 35 min.

Out of scope: attribution, follow, joined/left announcements, caret label fade, off-screen indicator, multi-file caret display. Sources: https://automerge.org/docs/reference/repositories/ephemeral/ ; https://docs.yjs.dev/api/about-awareness ; https://developer.chrome.com/blog/timer-throttling-in-chrome-88 ; automerge-repo 2.5.6 source `presence/`.
