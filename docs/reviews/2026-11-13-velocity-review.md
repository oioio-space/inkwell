# Velocity review, 2026-11-13 — how the project can move faster, with evidence

Question: "comment faire pour que notre projet avance mieux, preuve à l'appui". Scope: process only; no proof is
weakened. Method v2 (`docs/plans/2026-11-12-method-v2.md`) already states most of the right rules; the data below
shows which ones are **not yet enforced** and what they cost.

Sources: `git log main` (472 commits, 13 active days, 2026-10-25 → 11-13), `gates/*.toml` (30) and `gates/*.json`
(last run only), `gates/SB1b/regression.log`, `.remember/today-*.md` (session notes), `docs/autonomous-decisions.md`.
Limits: one week of method-v2-era data; commit classes come from a keyword classifier on subjects; rerun counts come
from notes because gate manifests keep only the last run (see change 5). Estimates are marked "est.".

## 1. What the data says

**Throughput.** Product commits fell while process commits rose.

| Period (active days) | Commits | Tooling/process share | Product feat/fix commits |
|---|---|---|---|
| 11-07 → 11-09 (3) | 71 | 21 (29 %) | 13 |
| 11-10 → 11-13 (4) | 51 | 32 (62 %) | 5 (3 features, all landed 11-12 after 20:26) |

Rule 12 caps tooling at 20 %. Any proposal below must therefore be small (each fits in under a day) or be a rule
applied by hand.

**Lead time, gate freeze → block on `main`** (commit timestamps).

| Open frozen gates at the time | Blocks | Lead time | Median |
|---|---|---|---|
| 1–2 (11-08 → 11-09) | P2, RL1, SP1b, P3 | 1.9 h, 2.8 h, 3.2 h, 7.8 h | 3.0 h |
| up to 8 (11-10 → 11-12) | PB, RC1, C5S, LF0a3 | 11.2 h, 19.9 h, 59.0 h, 62.5 h | 39 h |
| still open (11-13 22:00) | SB1b, OI, SI1b | > 31 h, > 37 h, > 100 h | — |

Confound: C5S is complete-tier with a 322 min gate. But RC1 and LF0a3 are not, and the decision log of 11-12 says the
same: "No product block reached main today although C5S, RC1 and OI are implemented and reviewed". On 11-12 at 09:49
eight frozen gates were open (C5S, LF0a3, RC1, OI, SB1, RG, SI1b, NI0); four were frozen within 9 hours. Rule 1 says 1+1.

**Cheap checks failing late.** A check that runs standalone in under a minute (format, lint, `lint:env`, deadcode,
shellcheck, a moved path) first failed inside `done`, a gate or pre-push 12 times in 7 active days:

| Date | Block | Check | Where it surfaced |
|---|---|---|---|
| 11-08 | L3 | Biome format; 18 golangci-lint findings | push blocked (`f66524f`); before the gate |
| 11-08 | P3 | `lint:env` (2 variables) | `done` rerun |
| 11-08 | RL1 | deadcode (`83c1161`) | pre-push; 5 push attempts for the block |
| 11-09 | CI1b, SI1b | `lint:env` (twice) | `done` rerun each |
| 11-10 | — | `lint:env` (`GIT_DIR`, `6d241d9`) | pre-push; 4 attempts that morning |
| 11-12 | PB | Biome format | `fe:check` red on `main` 05:35 → 14:27 (8.9 h, `e691515`) |
| 11-12 | PB | base guard, 17 findings | gate red |
| 11-12 | C5S | shellcheck (`7f11ea1`), deadcode (`cc0134a`) | pre-push |
| 11-13 | DU | moved doc path (`40ac8da`) | gate red, 9 tests |
| 11-13 | SB1b | gosec 17, golangci-lint 27, then `lint:env` | two ~25 min gate runs |

`lint:env` alone: 5 times in the 6 days since it exists. Root cause: lint, gosec and `lint:env` run at pre-commit, and a
block is committed only after its gate (step 8), so code written by an executor outside Claude's hooks (Codex) meets them
first inside the gate. In SB1b's frozen `regression` criterion the order is Go tests → `fe:check` (382 s) → `lint` (6 s) →
`lint:env` (< 1 s): the cheapest check runs last.

**Gate failures by cause** (from notes; last-run manifests cannot show it). C5S: at least 5 full campaigns of 322 min
(≈ 27 h of an exclusive machine). Recorded causes: test rig (shared netns), oracle defect (`c5model.py`), a flaky test
rule (2/30), two simultaneous runs in one tree, a micro-benchmark shifted 6 % by code layout. None was a product defect;
the three real identity races were found by reading. Before rule 6, nested gates were 55–90 % of gate minutes (SP1b 42.6
of 78.1 min, P2 24.0 of 26.6). `gates/deferred.jsonl` is empty.

**Frozen gates superseded:** 8 of 30 (CI1, WB, SP1, SI1, LF0, LF0a, LF0a2, SB1), after 4 min to 6.3 h. Causes: a command
could not run with the real tools (2), an unreachable bound (4), a review after the freeze (2). The rules of 11-10 and
11-12 target exactly this; only SB1 → SB1b happened since.

**Blocked time, not labour.**

| Blocker | Duration | Evidence |
|---|---|---|
| ~20 min user session for OI | > 35 h (since 11-12 10:38) | notes 11-12, 11-13 (6 mentions on 11-13) |
| same for SI1b | > 18 h (since 11-13 03:52) | notes 11-13 |
| `/simplify` marker refused | ~6.7 h, zero commits (11-12 23:58 → 11-13 06:44) | notes; fixed by a durable permission |
| SB1b executor (Opus guardrail stop, Codex refusal) | ~4.5 h (02:26 → 06:58) | notes 11-13 |
| Codex quota out | 3.5 days (11-08 → 11-12 08:41) | notes 11-08 |
| machine busy / not free | 7 mentions on 11-12; gortex OOM with 16 worktrees | notes 11-12 |
| monitor expired and re-armed | 13 mentions on 11-11 | notes 11-11 |

Rule 7 (`next` → `main`) is not in use: no `next` branch exists, and OI, SI1b and SB1b (53 files) sit uncommitted in
worktrees.

## 2. External practice

- DORA, WIP limits: "shorter lead times, higher quality, lower costs"; "Don't relax your WIP limits when people are
  idle"; make blocked work visible. <https://dora.dev/capabilities/wip-limits/>
- DORA, trunk-based development: "three or fewer active branches", "merge branches to trunk at least once a day".
  <https://dora.dev/capabilities/trunk-based-development/> (here: 38 local branches, 4 unmerged, 6 worktrees.)
- DORA, continuous integration: fast tests "upper limit of about 10 minutes"; longer tests go to a later pipeline stage;
  a broken build is fixed or reverted within minutes. <https://dora.dev/capabilities/continuous-integration/>
- Fowler, staged builds: a fast commit build first, slower stages after. <https://martinfowler.com/articles/continuousIntegration.html>
- DORA metrics (change lead time, deployment frequency, change fail rate, recovery time, rework rate).
  <https://dora.dev/guides/dora-metrics/>
- Google on flaky tests: reruns "only for tests that are marked as flaky"; the cost to workflows is tracked.
  <https://testing.googleblog.com/2016/05/flaky-tests-at-google-and-how-we.html>

All six pages were fetched on 2026-11-13.

## 3. Proposed changes, ranked by expected time saved per week

| # | Change | Evidence | Expected gain and its measure | Risk to quality |
|---|---|---|---|---|
| 1 | **Enforce WIP 1+1 and use `next`.** `gate-freeze.sh` refuses a new freeze commit while more than two frozen gates have neither a passing manifest nor an explicit `parked`/`superseded` line in a tracked `gates/wip.jsonl`. A candidate green on T1 is committed to `next` the same day (rule 7), freeing its worktree. | Median lead time 3.0 h at 1–2 open gates, 39 h at up to 8; a day with no product block on `main`; OOM from 16 worktrees; no `next` branch | est. 5–10 h/week of restarts and waiting. Measure: open frozen gates (target ≤ 2), median freeze → `main` for standard blocks (target ≤ 1 day), age of the oldest uncommitted block | None: no proof changes; parking is a logged decision. Specs may still be drafted, only not frozen |
| 2 | **`mise run pregate -- <bloc>`, required before a campaign.** It runs the cheap set on the working tree (lint, `lint:env`, `lint:claude-md`, `scan:code`, Biome on changed files, deadcode, shellcheck, gate base guard, `mutants.py --check`; target ≤ 3 min) and stamps the tree hash; `gate.py` refuses to start without a matching stamp. Executor briefs require "pregate green" in the hand-back. | 12 late cheap-check failures in 7 days; `lint:env` 5 times in 6 days; SB1b lost two ~25 min runs today | est. 3–4 h/week (12 incidents × one 20–25 min gate or push cycle, plus re-arming). Measure: gate, `done` and pre-push failures whose first failing command is in the cheap set (target 0) | None: an added check; frozen gates untouched and still run their own lint |
| 3 | **One batched user session per day.** Each visible block ships `gates/<bloc>/SESSION.md` (launch command, 5–8 steps, SEQ line); the French status message proposes one 20–30 min slot covering every waiting block. Until then the block waits on `next`, not in a worktree. | OI > 35 h and SI1b > 18 h blocked on ~20 min each; 2 of 3 open blocks | About one day of lead time per UI block. Measure: hours between "ready for session" and the session | None: the session (R14) stays mandatory before `main` |
| 4 | **Trial: long non-native campaigns off the laptop.** Run the next complete-tier campaign's simulation and netsim criteria on a second Linux host (rented VM or self-hosted runner); native perf stays local. Compare verdicts on one block before adopting. | 27 h of exclusive laptop for C5S; "no free machine" delayed RC1; selftest fails when another heavy job runs | est. 5 h of laptop per complete-tier campaign. Measure: laptop-exclusive hours per block | Medium: an environment change is a contract change (rule 9): old/new comparison plus a negative control first; cost to check |
| 5 | **Append-only run ledger.** `gate.py` appends one line per run to a tracked `gates/runs.jsonl` (lot, tree, start, duration, status, first failing criterion; cause class filled before any rerun: product, test or oracle, environment, cheap check, orchestration). A small `scripts/velocity.py` prints rule 15's weekly figures. | Manifests keep only the last run; this review had to rebuild reruns and causes from notes | est. 1 h/week, and it makes rules 5, 12 and 15 measurable. Measure: the weekly report exists and needs no manual digging | None |
| 6 | **Flake register with captured evidence.** A test that fails then passes on an unchanged tree is recorded (test, seed, log path) and owes a redesign onto controlled time within 7 days (rule 16); never retried into a pass. Failing output is always kept. | `TestLinkRTTFromProbe`, `gate-selftest` under load, p2p flake 2/15, the LF0 random-edit failure whose message was not captured | est. 1–2 h/week of reruns. Measure: open flakes and their age | None: stricter than today |
| 7 | **Track gate supersessions** as a method-v2 metric (count, time to supersede, cause); no new tool. | 8 of 30 gates superseded, 6 before any code | Confirms that review-before-freeze works. Target: 0 supersessions caused by an unrunnable command | None |

**Order, within the 20 % tooling cap:** apply 1 and 3 by hand today (create `next`, commit OI and SI1b there, propose one
session); build 2 as the single tooling block of the week (smallest, clearest measured loss); 5 with the first weekly
look; 4, 6 and 7 after R1.
