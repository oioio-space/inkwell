# Block OI — independent review (another model family)

Rigour complete (`docs/engineering-principles.md` §0): one review by a model family other than the implementer's.
Two Codex reviews (OpenAI), requested by the coordinating session, both adversarial; every finding was fixed at the
root, test first (red, then green), in the block's worktree. Details of each fix: `gates/OI/SIMPLIFY.md`, sections
« Codex review » and « Codex re-review ».

**Scope of the checks.** The first review read the whole diff of the block (code, tests, model, plan, ADR 0037). The
second (« re-review », native read) re-checked the six first findings (four resolved, the activation token kept as the
documented R1 limitation) and raised five new ones; **its fixes are checked by a narrow fix-diff review only**: no third
whole-block review has read the code after them. Their proof is the tests below and the gate's criteria.

## Review 1 (adversarial, whole block): « fix first »

| # | Severity | Finding | Fix | Test (red first) |
|---|---|---|---|---|
| C1 | High | requests acknowledged before the identity, project and network opened; a failed start lost them after their launches exited 0 | a launch is acknowledged only once the window applied its request (`Ticket`, webview `{"applied":id}`); held 2 s then « busy »; a failed or closing primary answers « closing » and the launch becomes the next primary | `TestStartupFailureAcksNothing`, `TestHandoffAckAfterApply` |
| C2 | High | socket candidates computed once: a launch that started before the private fallback folder existed never found the primary | candidates recomputed on every try | `TestHandoffFallbackCreatedLate` (red: « does not answer » after 5 s) |
| C3 | High | a clean shutdown dropped requests already acknowledged | same root fix as C1; model: `ExitOkImpliesApplied`, `AckedApplied` without an escape for a closing primary, mutant `AckDropOnClose` | `TestHandoffDroppedAtClosing`; `mise run tla` |
| C4 | Med | activation token discarded | **fixed 2026-11-13** (the limitation failed on the installed package): `internal/activate.Present` gives the token to GTK before `present` (plan §13, ADR 0037 amendment) | `TestSecondLaunchTokenReachesPresenter`; `mise run app:activation` |
| C5 | Med | invalid UTF-8 paths reached the webview as another path (JSON) | `launch` refuses such a file with its own notice (`badName`) | `TestResolveNonUTF8Name`; e2e « …whose name is not UTF-8 shows the toast » |
| C6 | Med | unbounded promise chain in the webview; Go drained on send | one request in flight, Go waits for the webview's answer; at most 8 pending in the webview | `TestOpenerOneInFlight`; Vitest « keeps at most MAX_PENDING… » |

## Review 2 (re-review, native read): 4 of 6 resolved, token = documented limitation, 5 new findings

| # | Severity | Finding | Fix | Test (red first) |
|---|---|---|---|---|
| R1 | High | an application error was logged, then `applied` sent: exit 0 with nothing shown | webview `{"failed":id}`; reply `failed` (status 5, also for a retry); the launch shows R19 and exits 5; `show` fails when the file was not shown | `TestHandoffFailedReported`; Vitest « reports a request the window could not show as failed… »; e2e « …whose file the window cannot show fails visibly » |
| R2 | High | no deadline on the webview's answer: a stuck webview blocked delivery for ever | 10 s per request, then failed, slot freed, next served; late answers ignored | `TestOpenerApplyDeadline`; Vitest « answers nothing while the workspace never becomes ready… » |
| R3 | Med | after « closing », a third launch could win the lock and the forwarded file became « another project » | rule D-FORWARD (plan, ADR 0037): the request is handed unchanged; a primary still starting with nothing of its own opens the forwarded request's project; model witness `NoForward` | `TestHandoffAcrossTakeover`, `TestTakeoverOpensForwardedProject`; `mise run tla` |
| R4 | Med | same-user queue flooding | accepted residual (ADR 0037 « Consequences »): the user is the trust boundary; bounds kept; FIFO retry order not done (needs an unbounded list or a second « busy ») | — |
| R5 | Med | weak oracles | takeover proved with a third contender and the displayed frame; the in-flight bound proved through the real webview; refusal and never-answer cases assert what is displayed | e2e « OI a burst of launches is applied one by one through the window », and the tests above |

## Round 3: the narrow check of review 2's fixes found them incomplete → a design decision

By method v2 a third finding round is a design decision, not more patches. The coordinator decided D1-D3; they were
applied as one design, test first (plan section 4.1, « Third amendment »; ADR 0037, decision 6).

| # | Decision | Implementation | Test |
|---|---|---|---|
| D1 | one owner of the deadline: it starts when the webview reported ready; initial readiness has its own bound (60 s), then « busy »; on expiry `{"cancel":id}`, the webview aborts, frees its slot, never shows it later; late answers ignored on both sides | `{"ready":true}` from the webview; Go sends nothing before; status `starting` (6) renews the launch's patience while the window opens (`StartWait`); `answerExpired` → `Ticket.Failed` + `{"cancel":id}`; webview `AbortController` per request, `applyOpen` checks it before and after each step, the chain races it against the abort | `TestHandoffStartingPatience` (slow first open succeeds; never ready ends with R11), `TestOpenerWaitsForReady` (nothing before ready; deadline from ready), `TestOpenerApplyDeadline` (`{"cancel":1}` after expiry); Vitest « never shows a request Go cancelled… », « frees the slot of a request whose application never ends… » (9th accepted), « says it is ready only once the workspace is » |
| D2 | the starting primary chooses its project from the queue at the moment it decides, requests that arrived during its startup included | handover mark left by a primary that closed with pending requests (`TookOver`); the next primary, with nothing of its own, waits up to 500 ms for one before choosing | `TestHandoffTookOver`; `TestTakeoverOpensForwardedProject` (B arrives 100 ms into C's startup, after C opened its identity; C opens B's project and shows its file) |
| D3 | the burst oracle checks each request applied, in order | the webview logs each applied request (`__inkwellOpenApplied`, end-to-end build only) | e2e « OI a burst of launches is applied one by one through the window »: 8 entries, each file once, ids increasing |

## Round 4: the contract is simplified (golden rule)

Codex's check of round 3 still said « fix first » (`show()` not cancellable, the takeover wait accepting unrelated
launches, mark files following symlinks and going stale, a reload keeping old pending IDs, « busy » against
« starting » patience). By the golden rule the project stopped patching and simplified the contract (decision logged
in `docs/autonomous-decisions.md`, row « OI »): a launch exits 0 once the primary has received and queued its request;
the primary owns the outcome and shows it, or says why not, in its window. Every round-3 finding is about machinery
that no longer exists:

| Round-3 finding | Now |
|---|---|
| `show()` not cancellable | no per-request deadline, no cancellation: the window applies what it received, in order |
| the takeover wait accepts unrelated launches | no takeover wait; a forwarded request after a takeover follows the normal rows |
| mark files follow symlinks and go stale | no mark file |
| a reload keeps old pending IDs | no pending IDs: a request is the primary's once queued; a request taken when the stream fails goes back to the queue |
| « busy » against « starting » patience | no « starting »: « busy » until the queue opens (the primary can show a file), then only when it is full |

Kept, with their tests: lock, socket, peer check, wire codec and fuzz, validation, the bounded queue (`TestOpenQueueBounded`: « busy » until open, 64 then « busy »), busy until ready to queue (`TestStartupFailureAcksNothing`), the webview's bound with a visible toast for what it leaves (Vitest « keeps at most MAX_PENDING… »), the flood through the real webview (e2e burst, each request applied once, in order), a file the window cannot show said in a toast (Vitest, e2e « …is said in the window »), the non-UTF-8 refusal (`TestResolveNonUTF8Name`, e2e), takeover with retry (`TestHandoffAcrossTakeover`, `TestHandoffDuringClosing`), and the drain at close (`TestOpenQueueBounded`, model mutant `AckDropOnClose`).

## Round 5: file-by-file check of the simplified contract (4 local defects, no contract change)

| # | Severity | Defect | Fix at the root | Test (red first) |
|---|---|---|---|---|
| L1 | High | a clean close could lose acknowledged requests: Go dequeued before the window handled them, and the close wait looked only at the queue length | a request stays in Go's queue until the window reports it handled (`{"handled":id}`, an internal report, not an acknowledgement to the launch); closing waits at most 2 s for those reports, then logs what is left (the residual of a window that cannot answer while closing) | `TestCloseWaitsForHandled` (handled within the wait; silent window logged at the bound); `TestOpenerServe` (kept until handled; resent to the next window) |
| L2 | Med | the model assumed every request applied before close | `Take` (Go sends) and `Apply` (the window handles) split; the drain happens during « closing »; the bounded wait is the fault `Stuck`, which is `AckedApplied`'s only new escape; mutant `EarlyDequeue` (close once Go's queue is empty) is caught | `mise run tla` (sound models pass; `EarlyDequeue` and `AckDropOnClose` violate `AckedApplied`) |
| L3 | Med | the queue could exceed 64 when a failed send put a request back after its slot was freed | follows from L1: the slot stays reserved until handled; a failed send changes nothing in the queue | `TestOpenQueueBounded` (refill while 8 are in flight never exceeds 64) |
| L4 | Med | a rejected readiness lost the request without a toast; a step that never ends blocked later requests and the overflow toast | readiness, `prepare` and `show` are each bounded (60 s, 10 s, 10 s); a rejection or a bound gives the « n'a pas pu ouvrir » toast and frees the slot; every request is reported handled | Vitest « gives each request its toast when the workspace failed to open… », « gives up a step that never ends after its bound… », « bounds a workspace that never becomes ready », « reports each request handled… » |

The Go tests were written first and failed to build against the old queue (red); the webview's code was written just
before its tests (not red-first).

## Round 6: fix-diff check of round 5 rejected → a design decision (one owner at a time, by generation)

Codex's check of the round-5 fix diff: REJECT. Per method v2 rule 3, a major post-smoke finding is a design decision,
not a patch: rule D-OWNER (plan section 4, amendment 2026-11-13; ADR 0037 point 6, amended the same day).

| # | Severity | Finding | Decision | Proof (spec first, then red-first tests) |
|---|---|---|---|---|
| G1 | High | `openRequests.ts`: a reconnection stranded slots: what Go sent again went to `#dropped` (the old chain still counted 8 pending), the old chain flushed them on the old socket, `#handled` suppressed them, and `#dropped` was cleared: Go kept 8 « sent » for ever | the webview starts each connection from a fresh `Session`: retiring it aborts its signal, its queued requests are skipped, its results and reports discarded; Go sends them again on the new stream | Vitest « starts afresh after a reconnection… » (8 pending, reconnect, all 8 applied and reported on the new stream, none on the old) |
| G2 | Med | `open.go`: reports had no stream ownership: any stream's end reset every entry, and a report removed an id whatever stream it was delivered on | Go numbers each connection (generation); a new one supersedes the old one, which Go closes, and only the old generation's unreported requests go back; a report is accepted only from the generation holding the id (others ignored, logged at debug level); the end of a stream gives back only what it held | model: actions `Connect`, `Reconnect`, `EndStream`, `Send`, `Apply`, `Deliver`; `SlotsAgree`, `RemovedWasShown`, `NoStrandedSlot`; mutants `StaleReportAccepted` and `ResetAllOnDisconnect` caught by `SlotsAgree`; witness `NoStaleReport` violated. Go: `TestSupersedeGivesBackOnlyOldGeneration`, `TestOpenerSupersedes`, `TestOpenQueueOwnerHistories` (400 generated histories of launches, connections, late ends, deliveries and stale, duplicate or forged reports: model invariants checked at every step, then no stranded slot) |
| G3 | Med | the plan's residual summary still said F3 removed, contradicting ADR 0037 and this file | F3 is an accepted residual of the simplified contract; the plan's F3 row and its residual summary say so, with the close residual | plan sections 8 (F3) and 10 (residuals) |
| G4 | Low | `openRequests.ts`: a `show` past its bound was not cancelled; its late completion could overlap the next request | (the bound's cancellation is superseded by G5) the bound aborts the step's `AbortSignal` (the window's `prepare` and `show` check it) and the next request starts only once the step has really ended: never two at once; a step that ignores the cancellation and never ends is a window that stops answering (the close residual) | Vitest « never starts a request while the previous one's show is still running » |
| G5 | Med (R-trust) | G4's bound announced « n'a pas pu ouvrir » for a step that might still succeed (round-6 report, item 7): a late success showed the file after a failure toast | coordinator's decision: at the bound the window says « L'ouverture de « {nom} » prend plus de temps que prévu. » (same toast, catalogue key `open.notice.slow`) and the request keeps its slot until the step really ends; a late success opens the file with no failure toast, a late rejection gives « n'a pas pu ouvrir »; the next request still waits for the step to settle, bounded only by the window's close (the close residual, plan and ADR 0037) | Vitest, red first: « says a late success is slow, then opens the file, with no failure toast », « says a late failure is slow, then that the file could not be opened », « says a step past its bound is slow and keeps its slot until it ends… », « says a workspace slow to become ready is slow… » |
| G6 | Med (rule 16) / Low | Codex's round-6 check (code and model accepted on substance): `TestOpenQueueOwnerHistories` took admissions, generations and deliveries from the implementation's own results; the plan still said a never-ending step frees its slot (section 4) and that an acknowledgement implies application, with the old liveness (section 6.2) | the test now runs a reference model computed from the history alone (`ownerModel`: admission under the 64 bound and its numbering, one generation per connect, FIFO delivery per generation with the 8 in flight, give-back on supersede and on end, reports accepted only from the holding generation), compared with the queue at every step, refusals and order included; half of the histories end by draining on the current connection with no reconnection (which would give back, and hide, a stranded slot), and the run requires refusals, ignored reports of held requests and such drains to occur; plan sections 4 and 6.2 restated (S4 `ExitOkImpliesQueued`; `AckedApplied` and `NoStrandedSlot` with their three escapes: primary crash, hang, window stuck). Generation wrap after 2^64 connections: negligible (one per page load or dropped stream), no code | hand-applied mutants of `open.go`, each red on this test alone, then restored: LIFO delivery, no give-back at a stream's end, a stale report accepted, every request given back at any end; gremlins cannot mutate this `main` package (every mutant « lived » without a test run, `SIMPLIFY.md`), so these hand mutants are the mutation evidence for `open.go` |
| G7 | Sev 2, 2, 1 (UX walkthrough, `gates/OI/UX.md`) | UX-OI-1: requests opened in a row replaced each other's toast; UX-OI-2: « Et 1 autre. » named neither the other file nor its problem; UX-OI-3: failure toasts had `role="status"` | coordinator's decision (interface coherence, the existing `Toast` and catalogue): Go sends a notice per argument (`launch.Outcome.Notices`, R8 amended); the window says each burst (800 ms quiet) in one toast per kind naming up to three files then « et N autre(s) », failures in their own `role="alert"` toast with each file and why, a single item keeping its sentence; toasts are queued one after another (AdwToastOverlay), and a request is reported handled once its burst is said | red first: `frontend/e2e/ux/oiDefects.spec.ts` (3 red, now green), 6 new Vitest cases (failing before the code) and a queue test; `TestPlanOneToastPerRequest` and `TestResolveSeveralPaths` red (build) then green; mutation `internal/launch` rerun |

Trust (ADR 0037 point 6, with ADR 0009): the reports arrive only on the Wails stream between the application's own
webview and Go, with no local port, so their sender is our window; the generation guards against our own stale
connections, not against another sender.

## Verdict

Rounds 1 and 2 were fixed; round 3 was answered by a design decision whose check found it incomplete; round 4
replaces that machinery by the simpler contract above. Codex checked the simplified contract file by file (round 5:
four local defects, fixed) and rejected the round-5 fix diff (round 6: answered by rule D-OWNER, specified in the
model before the code). The OI criteria other than `regression` and `review` pass on the round-6 code; the
round-6 diff has not been checked by another model family yet. Open: a Codex check of the round-6 diff, the GNOME
check of the token limitation and the user's session (`gates/OI/UX.md`).
