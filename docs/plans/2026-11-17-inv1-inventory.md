# Block INV1 (tooling, rigour « standard »): every open item has a place in the ordered list

Status: built 2026-11-17 (`scripts/inventory.py`, `scripts/inventory_test.py`, `mise run inventory`, part of
`pregate`). First output on `main`: kept outside the repository, filed into `PROGRESS.md` by the integrator next.

## 1. Root cause

Work was being set aside without anyone deciding to set it aside. Two read-only inventories (2026-11-17) found:

- 30+ open items (requirements, residuals of `SIMPLIFY.md`, `test.fixme`, proposed ADRs) that had no place in
  `PROGRESS.md`'s ordered list « Cap à long terme ». The list is the only thing a session reads to choose the next
  block, so an item outside it is never chosen: it is not refused, it is forgotten. → INV1
- unmerged branches and dirty worktrees left for weeks: their content is invisible to `main`, to the CI and to the
  next session, and a worktree costs about 1 GB (CLAUDE.md « Pièges connus »).
- `main`'s GitHub CI had no green run since 2026-10-25. The `ci` job always failed early, so the jobs after the
  failing step were never seen to fail (`protocol/trust` was found red on 2026-11-16 only once the sandbox tests
  passed). « Is main green? » was answered from the local CI, which is not the same machine.

The common cause: the project had rules saying where each thing goes (requirements → registry, residual →
`SIMPLIFY.md`, decision → ADR) but no rule, and no machine check, saying that each open thing must also be
*scheduled*. Each place was correct; the union was not reachable from the plan. → INV1

## 2. The rule (single home: CLAUDE.md « Registre des exigences »; pointer in PROGRESS.md's header)

Every open item anywhere carries the ID of a block present in PROGRESS.md's ordered list (« Cap à long terme »).
Unmerged branches and dirty worktrees are listed in a PROGRESS.md section « Travaux en cours », one line each,
naming the branch or worktree and a block of the list.

A block ID nests: `HB1a`, `HB1b` are filed when `HB1` is listed (a lower-case suffix after the number is a
refinement); `W7` is not filed by `W74` (digits are never stripped).

## 3. The checks

`mise run inventory` fails on any finding; `mise run inventory -- --report` prints the same grouped list and exits
0 (for the start of a session). Each finding names `file:line` (or the branch, worktree or run) and the fix.

| # | Check | Open when | Filed when |
|---|---|---|---|
| 1 | requirements | a row of `docs/exigences.md` whose state does not start with ✅ | its own ID, its « Où » or its « État » names a listed block |
| 2 | residuals | an item of `gates/*/SIMPLIFY.md`, `gates/*/WITNESSES.md`, `docs/plans/*.md` that says résiduel, residual, deferred/deferral, différé, reporté, follow-up, plus tard, not done, pas fait, or « for/until later », « to fix later », « later block »; or any list item under a section (`##` and below) or a short label (« Residuals: ») that says so | it carries « → ID » or « [ID] » of a listed block; « [accepted ADR NNNN] » (consciously accepted, the ADR exists); or « [done] » |
| 3 | skips | `test.fixme`; an unconditional `test.skip("…")` or top-level `t.Skip`; a skip saying TODO, FIXME, not merged, not yet, needs B…/wave/ADR; a conditional skip with no environment reason | a listed block is named on the skip line or the line around it; an environment skip (mDNS, root, links, OS, kernel, sockets, AppArmor, namespaces, short mode, opt-in variables) matches `ENV_SKIP` and is allowed |
| 4 | adrs | `status: proposed` and `date:` more than 14 days ago | the ordered list cites « ADR NNNN », or the status line carries a tag |
| 5 | git | a local or remote branch not merged into `main` whose tip is more than 7 days old; a worktree whose oldest uncommitted change is more than 3 days old; a folder of `.claude/worktrees/` that `git worktree list` does not know | named in « Travaux en cours » on a line that also names a listed block |
| 6 | ci | (only when `gh` is authenticated, otherwise a notice) the latest completed `ci.yml` run on `main` is more than 7 days old, or one of its jobs is not `completed`/`success` | never: it is fixed on `main` (« Cap à long terme », point 1) |
| 7 | deferred | an obligation of `gates/deferred.jsonl` is overdue (`gate.py`'s `deferred_report`, reused, not copied) | the lot's gate is run alone → INV1 |

Choices:

- **Explicit tags in prose, plain IDs in tables and tests.** Plans name blocks in passing all the time; a residual
  is filed only by a deliberate « → ID » or « [ID] ». A requirement row or a test title has no prose around it: any
  listed ID in it is the filing. → INV1
- **« later » is matched only in deferral phrases**: « a later request fails » is not a residual. → INV1
- **Environment skips are an allowlist**, not a denylist of TODO words: a new reason (« flaky ») fails until it is
  either a block or a named environment condition.
- **« [done] » and « [accepted ADR NNNN] »** exist so that a closed or accepted residual is marked, never
  reworded to dodge the marker — a rewording would hide it from the next reader too.
- The worktree age is the **oldest** uncommitted change: an old change does not become fresh because the tree was
  touched today.

## 4. Why each threshold

- **ADR proposed > 14 days.** An ADR is written by the block that needs it; blocks here last from a day to about a
  week (`PROGRESS.md`, R1 table). Two weeks is two block lengths: past it the decision is not in progress, it is
  waiting for a block nobody scheduled. Six ADRs (0021–0029) were proposed for 22–23 days on 2026-11-17.
- **Branch > 7 days.** Trunk-based practice keeps branches to a day or two and treats older ones as integration
  risk ([trunkbaseddevelopment.com, « Short-lived feature
  branches »](https://trunkbaseddevelopment.com/short-lived-feature-branches/)); GitHub only calls a branch
  « stale » after three months ([GitHub docs, « Viewing branches in your
  repository »](https://docs.github.com/en/repositories/configuring-branches-and-merges-in-your-repository/managing-branches-in-your-repository/viewing-branches-in-your-repository)),
  which is far too late for a single-integrator project. Seven days matches the weekly deferral audit of method
  v2 (rule 6, `OVERDUE_DAYS` in `scripts/gate.py`) and the longest normal block. → INV1
- **Dirty worktree > 3 days.** Uncommitted work has no backup and no CI; it is lost by one wrong cleanup
  (CLAUDE.md « Pièges connus », 2026-11-08) and it keeps a worktree alive (≈ 1 GB each, OOM on 2026-11-12). Three
  days is the gap between two sessions plus a margin; a worktree that stays dirty longer is set aside, and the
  listing makes it a decision.
- **CI on main > 7 days or any job not green.** The local CI is not GitHub's machine (AppArmor, arm64, a kernel
  that differs: four SB2a1 runs, each a real gap). Every job, not only the run's conclusion, because the failure
  that hid the others was one early job. Seven days: `main` is pushed after every block, so a week without a run
  means no block landed or the push did not run CI.
- **Deferred > 7 days**: unchanged, owned by `scripts/gate.py` (`OVERDUE_DAYS`). → INV1

## 5. What this block does not do

It does not file the existing findings: the integrator files them into `PROGRESS.md` next (expected: every check
fails on the first run). It adds no session-start hook: `.claude/settings.json` has none, and `pregate` already
runs before every gate, `done` and push (`scripts/qpush.sh`).
