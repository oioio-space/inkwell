# SB2a1 - /simplify review of the block's diff and review of the Codex-written tests (2026-11-15)

Diff read against `main` c822d52 (`git add -N . ; git diff HEAD`, 83 files). Two parts: the review of what Codex wrote
(the eight `internal/sidecar/*_contract_linux_test.go`, `escape_lifecycle_linux_test.go`, the helper dispatch in
`tier_modes_linux_test.go`, the Landlock ABI seam and mutant MS16), read blind against the rules of
`docs/plans/2026-11-14-sb2-sandbox-tiers.md`; then reuse, simplification, efficiency, altitude, dead code, stale
comments and modern Go on the whole diff (the non-test files were swept by a Sonnet reviewer, every finding re-checked
in the code before it was applied or skipped).

## Verdict on the Codex-written tests and the seam

**Accepted: each inventory test is an oracle for its rule; no test passes for a reason foreign to its rule that would
let its named mutant through.** Every name and subtest path of `gates/inventories/SB2a1.toml` exists exactly: checked by
the gate's own `go` criterion (`scripts/gojson_check.py`, columns local-N and local-F), result under « Evidence ».

The seam (`Spec.landlockABI` → `sandboxConfig.LandlockABI` → `restrictOutput`): accepted.
- Not reachable from production input: the `Spec` field is unexported and no non-test file of `internal/` sets it (grep);
  the launcher reads it only from the JSON its own parent passes on the command line of `/proc/self/exe`.
- It can only lower the ruleset to an ABI the kernel has, never below 3: `restrictOutput` refuses `< 3` and `> kernel`
  before any ruleset exists (`launcher_linux.go`, « invalid requested Landlock ABI »), and the handshake still refuses a
  report below ABI 3. `TestLandlockABISeamRefusesUnsupported` proves 2 and kernel+1; a negative value falls in `< 3`.
- `landlockPolicy(abi)` gives ABI 3 the file rights only and ABI 4 the TCP rights, no scoping below 6: the F-abi3 and
  F-abi4 rows therefore prove that seccomp alone refuses the sockets and signals (plan, « Landlock ABI witnesses »).

Findings (none blocks the block; the ones marked *residual* are listed again at the end): → SB2b

| # | Severity | Where | Finding | Decision |
|---|---|---|---|---|
| R1 | medium | `launch_contract_linux_test.go` `TestHandshakeFailsClosed` (`no-report`, `cancel-before-report`), `tier_modes_linux_test.go` `fakeLauncher` | the two subtests run the same fake branch and the same 300 ms cancellation: `no-report` proves the cancellation, not the handshake's own deadline. No test lets `handshakeTimeout` (5 s, parent side `await`, launcher side `pollReadable`) expire; the 2 s bound of the rule cannot hold for an uncancelled silent launcher | *residual*: needs a seam on the deadline (a production change), outside /simplify; T-5 is covered for the run by `TestRunReturnsAfterCancel` and MS5 → SB2b |
| R2 | **major for the gate** | `scope_linux_test.go:705-706` `TestCgroupSessionSharesSliceBound`, `TestCgroupSessionCopiesLeafBound` (not Codex's, not in the inventory, but inside the `go` criterion's run) | unreliable, and older than the block. Without `-race`: `ancestor bound did not kill the sandbox: cgroup cause=unknown signal=9`, 80 runs of the first test on an idle machine: HEAD c822d52 (main checkout, before the block) 4 failures, the block 5. With `-race` (what the `go` criterion runs), 6 runs of both tests: HEAD 8 failures of 12, the block 7 of 12, all `cause=none signal=0` (the ancestor bound never fired) | root causes found and treated 2026-11-15 (three defects of the test, none of production; all three on HEAD too). **R2a** (`-race`, `cause=none signal=0`): the app helper's target kept `MemoryLimit: 2<<30` without the race reserve; measured, `mmap` fails with ENOMEM at VmSize 2 089 768 kB under the 2 GiB `RLIMIT_AS`, RSS 97.8 MB, exit 1 « allocation-failed », before the 128 MiB bound. Applied: `appHelper` hands `raceAddressReserve` to the helper (`INKWELL_TEST_ADDRESS_RESERVE`). **R2b** (plain, `cause=unknown signal=9`, shared slice only): measured, the watch receives `IN_Q_OVERFLOW` (mask 0x4000) after 3 or 4 samples, never in the leaf case (0 of 50). Explanation cited from memory, the kernel source is not installed here (`fs/notify/inotify/inotify_fsnotify.c`: the event is charged to the watcher's memory cgroup, and a failed allocation is reported as a queue overflow): the watcher is the app, inside the full slice. ADR 0042 rule M9 answers `unknown` on a lost watch and `internal/typst/engine.go` treats `victim` and `unknown` alike. Applied: the shared-slice case accepts `victim watch-lost=false` or `unknown watch-lost=true`; the leaf case stays strict. **R2c** (`-race` mostly, « target did not report out= », 2 of 80): measured in the kernel log (10:06:30.657), the slice's OOM kills the run, and 0.8 ms later the app helper's own page fault finds the slice still full with the app as only candidate: the kernel kills the app. A harness artefact (the helper shares the test's slice; `oom_score_adj` cannot go under 100 without privilege). Applied: the attempt is played again, 3 at most, only on the exact evidence (helper ended by SIGKILL without a report and `oom_kill` > 0 in its own scope's `memory.events.local`, kept readable by an idle process and `OOMPolicy=continue`); *residual* for production, see the list → SB2b |
| R3 | medium | `font_contract_linux_test.go` `TestFontFoldersExactlyThoseGiven/{N,F}` | the witness against the old hard-coded fallback needs a `.ttf` under `/usr/share/fonts` and fails (`t.Fatal`) on a machine without one. Correct on this machine; unverified on the stock Ubuntu runner | *residual*: read in the `github` criterion's first candidate run; failing is right (a skip would hide the oracle, and no skip is accepted in F) → SB2b |
| R4 | low | `launch_contract_linux_test.go` `TestProbeDeadline` | a context cancelled before the call: proves the probe honours its context, not that a running probe is bounded | kept: the bound itself is `probeLaunch`'s `WithTimeout`; noted |
| R5 | low | `launch_contract_linux_test.go` `TestNoCapabilityNamespaceGivesF` | the EACCES of the id-map write is injected through the `probeLaunch` seam (the plan says « as measured »); the real refusal is exercised only on a machine that refuses it (stock Ubuntu) | kept: the decision path N refused → F probed → F is what the rule states; the real launch is `TestProbeIsTheLaunch` |
| R6 | low | `launch_contract_linux_test.go` `TestProbeIsTheLaunch` | no recorder seam (plan wording): the oracle is the real probe going through the same `run` and the handshake's `proc=true`. MS3 is caught by it | kept: stronger than a recorder (nothing to keep in step) |
| R7 | low | `escape_contract_linux_test.go` `sb2-attack` | `socket-abstract` is the same call as `socket-unix` (seccomp refuses `socket(2)`, an abstract address is never reached); `tkill-other-thread` targets the parent's main thread; the accepted errno set (EPERM, EACCES, ENOENT, EFBIG) is shared by every attempt; `ptrace-parent` would also get EPERM from Yama on Ubuntu | kept, commented in the helper: for the file attempts the oracle that counts is the sentinel read back unchanged; the seccomp answers are proven call by call by the interpreted-filter tests (S-1, S-2, S-4) |
| R8 | low | `filiation_contract_linux_test.go` `parent-dies-after-arming`, `parent-dies-during-run` | the launcher also dies of the closed handshake pipe (after arming) and of the `Pdeathsig` Go sets at fork (during run): the launcher's own `prctl(PR_SET_PDEATHSIG)` has no witness of its own. The parent check has one (`mismatch`, MS9); the `disarm` subtest proves the target cannot undo it | *residual*: the property (gone within 1 s, by pid) holds by three mechanisms; a mutant « launcher does not arm » would survive → SB2b |
| R9 | low | all Codex test files | failure messages of `TestEscape` are tagged `S-4 escape` (the rule is ADR 0044 point 5) | kept: no mutant expects that text, and the tag is only a label |
| R12 | medium | `launch_contract_linux_test.go` `TestHandshakeFailsClosed/{eof,truncated}` | failed under `-race` (first run of the `go` criterion): the 300 ms cancellation timer, armed for every fake launcher, fired before the race-built launcher had started and closed its report, so the error was `context canceled` instead of « no tier report ». A real-time race in the test, not a product defect | applied: the timer is armed only for the launchers that never report (`no-report`, `cancel-before-report`); a cut or invalid report ends the handshake by itself. 6 of 6 under `-race` afterwards |
| R10 | low | the eight Codex files | `mise run lint` was red on them (11 findings: 7 unchecked `Close`, `err != io.EOF`, an if-else chain, `real` shadowing the builtin, `context.Context` not first) | applied, lint green |
| R11 | info | `internal/sidecar/SB2a1-inventory.md` (left by Codex) | a snapshot from before the tests existed (every row « no »), and the statement « no production seam » is no longer true | deleted; its still-true notes are R1, R7, R8 here, plus: MS10 is caught by the diagnostic oracle (« no tier report » required for `eof` and `truncated`), not by an escape; the LSP cancellation accepts a result or RequestCancelled (-32800), then requires a working hover and a clean shutdown (cancellation is not forced to win); the N fork bomb is bounded to eight descendants on purpose |

## /simplify findings

| Finding | Decision |
|---|---|
| The tier guard (`requireN` for N, `requireLaunch` for F) was written inline five times and once as `escapeTier` | applied: one `requireTier` in `tier_helpers_linux_test.go`, eight call sites |
| LSP plumbing written twice (`cloneLSPInitialize`, `lspPerimeter`): two pipes, the confined or unrestricted server, the framed write with its deadline, the framed read | applied: `startLSP`, `writeLSP`, `readLSP` in `lsp_contract_linux_test.go`, used by both. `embedded_test.go` has a third, older copy: not in the block's diff, left |
| `sb2-fake` duplicated `fakeLauncher` (report, wait for proceed, sleep) and the helper mode `fakelauncher` had no caller | applied: `wrong-tier` is a case of `fakeLauncher`, `sb2-fake` dispatches to it, the dead mode is gone |
| `targetReadyWriter` and `treeReadyWriter`: the same writer with two predicates | applied: one `readyWriter{seen}` |
| The « `<word> <pid>` » barrier line parsed three times, the PID-namespace inode twice, a 5 s line read three times | applied: `phasePID`, `pidNamespace`, `readLine` |
| `sb2CloneThree` lived in the escape file, used only by the clone helper | applied: inlined in `cloneThreeHelper` |
| The socket attempt's domain chosen by `HasPrefix(attempt, "socket-u") && attempt != "socket-udp" || …` | applied: a `switch`, with the reason `socket-abstract` equals `socket-unix` |
| `TestEscape`'s acceptance `!(a && b && c && d) && (…)` | applied: named `overBudget` with its reason |
| `TestCloneThreeFallback/typst` built `dir + "-out"` by hand and removed it | applied: `t.TempDir()` |
| The system-font search walked all of `/usr/share/fonts` | applied: stops at the first `.ttf` (`filepath.SkipAll`) |
| `[]byte(fmt.Sprintf(…))` in the fake launcher | applied: `fmt.Appendf` |
| `probeTimeout`, `probeBackoffMin/Max` and `userNamespaceLimit` stayed in `mount_linux.go` after the block moved the probe out of it | applied: the constants next to `probeLaunch` (`launch_linux.go`), the limit next to its only reader (`tier_linux.go`) |
| `parseTierReport` built the same error four times | applied: one `invalid` value |
| Stale comments: `Output.Isolated` (« ran without network ([Spec.NoNetwork]) », now tier N), `internal/typst/engine.go` (« rather than run proxy-only », a mode the block removed) | applied |
| `sandboxInit` (`launcher_linux.go`) has only test callers | skipped: already so at HEAD, not this block's change (surgical-change rule) |
| `restrictOutput` repeats the ABI query `landlockABI()` wraps | skipped: it needs the errno, which the wrapper drops; changing the wrapper is a behaviour change in the launcher |
| `namespaceIdentity` could reuse `reportNumber` | skipped: the limit and return type differ (64-bit inode against a small int); two lines saved for a cast with a gosec waiver |
| `launcherFilter` open-codes the tier name → `tier` switch; `tier.String` indexes an array | skipped: one call site; the array is indexed by a three-value private type |
| `launchPlan.isolated()` and `mounted()` have the same body | kept: two decisions that SB2b separates |
| `scripts/gojson_check.py` `outcome`/`names`/`related_output` scan every entry per call | skipped: a few hundred tests, no measured cost; an index would be tooling without a demonstrated gain |
| `scripts/gh_verdict_test.py` `go_json` and `scripts/gojson_check_test.py` `run` are the same event builder | skipped: each test file is run by path (`python3 -m unittest scripts/x.py`) and stays self-contained |
| The observer of `TestCloneThreeFallback` (mutex, atomic flag, `OnceFunc`, two channels) | kept: each piece closes one ordering (the hook may never run; the stop may come twice); simplifying it risks a flaky S-6 |
| `seccomp_linux.go`, `sysarch_linux.go`, `program_linux.go`, `memory_linux.go`, `dir_linux.go`, `supervisor.go`, the `*_other.go` files, `internal/app/preview.go`, the four frontend files, `ci.yml`, `sidecar-allowlist.py` | read, nothing to change |

Checklists: go-style-guide and modern Go (range-over-int, `strings.SplitSeq`, `errors.AsType`, `t.Context`, `fmt.Appendf`
already used; lint `modernize` green); vuln-guard (the seam above; the config still travels only from the parent; no new
input from a peer); secret-guard (none; the sentinels are literals); helper-ergonomics (no exported API beyond
`Spec.Protected`, `ErrProtectedRefused`, `ErrSandboxRefused`, documented); repo-janitor (`SB2a1-inventory.md` removed,
`.task/` ignored).

## Evidence (after the changes, this machine)

Commands run after the last change, on an idle machine, one heavy job at a time:

| Command | Exit | Read in the output |
|---|---|---|
| `timeout -k 10 900 ./scripts/heavy.sh go test -count=1 ./internal/sidecar/...` | 0 | `ok` for `internal/sidecar` (17.4 s) and `internal/sidecar/fetch` |
| `mise run lint` | 0 | no finding (11 before, R10) |
| criterion `docs` (verbatim) | 1 | `['ADR 0044 not accepted']`: the ADR's front matter still says `status: proposed`; the other checks of the criterion pass. Left to the integrator (accepting an ADR is not a /simplify change) |
| criterion `go` (verbatim: `-json -count=1 -race`, three packages, then `gojson_check.py` on local-N and local-F) | 1 | the only problems reported: `go test exit status 1`, `package …/internal/sidecar failed`, `TestCgroupSessionCopiesLeafBound: fail` (R2). No inventory cell missing, failed or skipped: every name and subtest path of the inventory exists and passes in its column; the race detector reports nothing. The first run, before fix R12, also listed `TestHandshakeFailsClosed/{eof,truncated}` and `TestCgroupSessionSharesSliceBound` |
| criterion `mutants` (verbatim) | 0 | `netsim mutants: all 19 caught` (MS1..MS19, each by its named test) |
| criterion `frontend` (verbatim) | 0 | the three named Vitest tests passed, 0 failed |
| criterion `checkers` (verbatim) | 0 | `Ran 59 tests … OK` |
| criterion `simplify-report` (verbatim) | 0 | this file |

Not run here: `github`, `regression`, `ux-walkthrough`. The Codex-written test files (the eight `*_contract_`,
`escape_lifecycle`, `tier_helpers`, `tier_modes`) go from 2258 to 2161 lines with the shared helpers.

## 2026-11-15, probe target and wording (after the interface walkthrough)

Diff reviewed: the probe target (`launch_linux.go`, `program_linux.go`, `netns_*.go`, `run.go`, `store.go`,
`launcher_linux.go`, callers in `internal/typst` and `internal/language`), the retry kind and About modes
(`internal/app/preview.go`), and the webview (`Preview.svelte`, `preview.svelte.ts`, `PreviewClient.ts`, `protocol.ts`,
`AboutDialog.svelte`, `App.svelte`, both catalogues).

| # | Finding | Outcome |
|---|---|---|
| P1 | reuse: the probe had its own descriptor (`probeProgram`: the shared table, no prctl, no ioctl) | applied: removed; the probe takes typst's registered descriptor through `programFor(probeName)`, `probeName` living in `program_linux.go` so that rule S-10 (« no other file names the two programs », `TestProgramDescriptors`) holds |
| P2 | dead code: the hidden no-op mode of the executable (`noopMode`, its branch in the launcher's `init`) has no caller once the target is typst | applied: removed |
| P3 | altitude: where the store comes from. A package-level store set by the first caller would be hidden state; resolving in the default cache from every caller would write typst into the user's cache from tests that use their own store | applied: explicit, `NetworkIsolation(store)` and `Spec.Store`, nil meaning the machine's store (`defaultStore`); the test binary points `defaultStore` at its temporary cache |
| P4 | a target that cannot be verified (`ErrNotEmbedded`, `ErrIntegrity`) would have been classed « inconclusive » by `probeOutcome` and offered « Réessayer » for ever | applied: `cachedFacts` resolves the target before the probe and returns that error as it is, uncached (`TestUnverifiableProbeTargetIsNotInconclusive`); the interface then shows `preview.fail.unavailable`, which is the right sentence for a broken installation |
| P5 | reuse: « Détails » had a private button style (`.details`) | applied: removed; « Détails » and « Réessayer » use the shared `.btn`, with the lone-target height |
| P6 | reuse: `noteEdit` and the new `retry` both show « updating » until Go answers | applied: one private `#working()` |
| P7 | simplification: the About line was a chain of conditionals, two more modes would have made four levels | applied: one table from mode to key |
| P8 | the retry uses the existing `refresh` request of the preview stream (already specified and encoded, never sent by the webview) | applied: no new request |
| P9 | the refusal and retry sentences stay in the empty state of the preview (centre of the pane, where the walkthrough found the eye), not in the `.notice` strip, which is the place of a failure shown over older pages | kept as is: no second pattern for the same state |
| P10 | `TestProbeTargetWithRunpathLibrary` no longer needs the `probeTarget` seam (`runProbe` takes its target) | applied; the seam stays for `TestUnverifiableProbeTargetIsNotInconclusive` |

Evidence of this section is in the report of the session that made it (probe tests red first, package plain and
`-race`, `internal/typst`, `internal/app`, `internal/language`, mutants MS1..MS19, Vitest, `e2e/preview.spec.ts`).

## Residual list

1. R2c in production (decision of 2026-11-15, not a defect of SB2a1): rule S-11 covers the run's own bound → SB2b
   (`memory.high` and the watchdog). A bound set on an ancestor of the application (its slice, or its own scope's
   `memory.max` copied onto the session scope) is the person's or the system's own limit (W64) and stays enforced by
   the kernel: when the application and its sandbox fill it together, the kernel kills the run and may then kill the
   application itself, whose unit ends with `oom-kill` (the desktop's memory notification, W73). Measured: the
   `inkwell-sandbox-*` scopes have `Delegate=yes`, hence `OOMPolicy=continue`: 37 kernel kills, `Result=success`, no
   notification; only the kill of the application's own unit raises it. Proposed root fix, to decide: the watchdog also
   kills the run when a finite ancestor `memory.max` (session scope, slices up to `user@.service`) is about to be
   reached. Costs: inkwell cannot set `memory.high` on a slice it does not own, so it is a poll of `memory.current`
   at the 10 ms tick with a margin, racy against a fast allocation; the document is stopped when the application
   itself fills the ancestor; a new rule, model row and mutant.
2. `docs` criterion: ADR 0044 is still `status: proposed`. → SB2a1
3. R1: no test lets `handshakeTimeout` expire (parent and launcher side); `no-report` duplicates `cancel-before-report`. → SB2b
4. R3: the system-font witness depends on a `.ttf` under `/usr/share/fonts` on the stock runner. → SB2b
5. R8: the launcher's own parent-death arming has no witness that the two other mechanisms do not mask. → SB2b
6. Still to run before the full gate: `github` (candidate branch published, `gates/SB2a1/github.json`), `regression`, → SB2a1
   `ux-walkthrough` (`gates/SB2a1/UX.md`), the smoke under the global cap, the Opus or Codex review of the code after
   the smoke, `PROGRESS.md` and `docs/exigences.md`.

## Interface residuals after the re-judgement (2026-11-15, `gates/SB2a1/UX.md`, verdict ship)

Severity 2, for the interface block that follows (not blocking): UX-SB2a1-6 « Réessayer » gives no feedback and a click
during the backoff is swallowed (the preview came back after about 40 s; it should say « Nouvelle tentative… » and
retry by itself when the backoff ends); UX-SB2a1-7 in the refusal, « Exporter en PDF… » ends in a toast with no reason;
UX-SB2a1-8 in the refusal the status bar says « Aucun problème » beside a real Typst error. Severity 1: the About
title in the refusal and retry states, the words « isoler » / « isolation complète », « moteur de mise en page »
absent from the glossary, muted text at the AA limit in light mode. Not walked: the protected-project refusal, 200 %
zoom, forced colours.

