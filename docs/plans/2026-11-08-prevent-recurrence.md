# Preventing recurrence — formal guards, tests and diagnostics for every failure class (W36)

User request (2026-11-08, French): « Trouve des moyens formels pour que ça n'arrive plus, de quoi tester,
de quoi diagnostiquer. Pour tout. » Every failure met on 2026-11-08 is a *class*, not an incident. Each
class below gets three things, none optional: a **guard** (a formal or mechanical rule that makes the
class impossible or rejected at the earliest step), a **test** that fails when the guard is removed
(mutant), and a **diagnostic** that keeps the evidence when it still happens. Same rule as the rest of
the project: no weakened check, no « rerun until green ».

## 1. Classes met, with their guards

| # | Class (evidence, 2026-11-08) | Guard (formal / mechanical) | Test proving the guard (mutant) | Diagnostic kept on failure |
|---|---|---|---|---|
| C1 | **Silent data loss** — failed write without resync (NI-2, fixed in RL1) | Spec rule L, TLA+ `RepoGen` + mutant `RepoGenLossNoBump` | `TestRuleLSendFailureResyncs`; netsim mutant RL1a | per-peer `Resyncs` counter in Links; netsim C1–C4 lines |
| C2 | **UI element covered** — the format toolbar intercepts « Partager » (netsim 1 and 6, 2 runs in ~15) | **Hit-test invariant**: every interactive control of the header bar, popovers and dialogs is its own hit target (`elementFromPoint` at its centre is the control or a descendant) in every app state. Cause still unknown: a programmatic scroll of the shell was tested and refuted (no ancestor ever scrolled; the failing click is the first action of a fresh page), so no CSS change on a guess — the next failure's kept trace decides | an e2e « header controls are never covered » run after each step of the netsim and UX scenarios, at 3 viewports, light and dark; mutant: an element painted over the header (the test must turn red) | Playwright `trace: retain-on-failure` kept per run (never wiped by the next scenario), the hit target and every ancestor's `scrollTop` printed in the failure |
| C3 | **Contract wired but data wrong** — Links reaches the webview with `path unknown`, no RTT (NI-1) | One contract, three layers checked at each boundary: Go `transport.Link` → `protocol/bridge` vectors → TS view; a property « the view of a peer equals the transport's link at the last push, ages aside » | Go+TS integration test on memnet with a scripted link (kind, RTT, loss) asserting the `__inkwellLinks` view; mutant: drop the path or the RTT in the encoder | the Links payload of the failure decoded in the netsim log (no address: privacy rule) |
| C4 | **Build race** — `e2e:build` empties `frontend/dist` while Go tasks embed it | Every mise task declares `sources`/`outputs`; a checker refuses two tasks that may run together in `ci` when one writes what the other reads; builds write to a side directory and swap (done) | `scripts/task_graph_check.py` + its test with a two-task conflicting fixture; mutant: remove a `wait_for` | the task graph and the conflicting pair printed |
| C5 | **Late failure** — coverage, deadcode, arch found only at push; `done` failing at step 7 after 35 min | Shift left: affected packages' coverage, deadcode, arch at commit (< 30 s); `done` runs a preflight of every step's cheap prerequisites before mutation (study R4, R9) | a fixture commit below 85 % refused at commit; `done --preflight` failing on a stale report | the failing item and its exact command in one line |
| C6 | **Phantom dependency** — a test fixture path (`scripts/x.py`) seen as a gate dependency | gate-guard self-test (exists); fixtures use real files or a non-path marker | `gate-guard.py --self-test` in `test:py` and at commit when `gates/` or `scripts/` change | the dependency chain that reached the file |
| C7 | **Flake hidden by a retry** | A failure is either reproduced (seed + trace) or quarantined **with evidence and an owner**; Playwright retries never turn red into green silently (a passed retry is reported as « flaky ») | a quarantine list checked by CI (entry needs issue, seed, date); a fake flaky test must appear in the report | the seed, the trace, and N reruns with the same seed (`mise run e2e:repro -- <test> <seed>`) |
| C8 | **A claim not checked** (agent says « already broken », « not reproducible ») | Rule in CLAUDE.md (exists): replay on a tree from before the change; now also: every « cannot reproduce » carries the number of tries, the oracle and the kept trace | — (process rule, checked in review) | the reproduction log path in the report |

## 2. Diagnose: one bundle for any failure

`mise run diagnose -- <log>` (new) gathers, without any secret: the failing step and command, the seed,
the kept Playwright traces, the netsim per-scenario logs, the decoded Links payloads, the qdisc
statistics, `go env`/versions, and the last 1 000 network messages (étape 5 journal). The bundle goes to
the scratchpad; `scripts/llm.sh triage` (free model, no secret) gives the first summary.

## 3. Order (in the global plan)

- With NI0 (lot « Collaborer sans perdre »): C2 hit-test invariant + `overflow: clip`, C3 integration
  test, C7 trace retention. They unblock the push CI now.
- CI/CD tooling lot (W35, study `docs/plans/2026-11-08-ci-cd-study.md`): C4 task-graph checker, C5
  shift-left and preflight, C7 quarantine, `diagnose`.
- Each item is a frozen gate first (`gates/<bloc>.toml`), then code, like every block.

## Sources

- CSS Overflow Module Level 3 / MDN `overflow`: `clip` forbids all scrolling, including programmatic
  scrolling, unlike `hidden` — https://developer.mozilla.org/en-US/docs/Web/CSS/overflow
- Playwright actionability, « receives events » hit test — https://playwright.dev/docs/actionability
