# TC1 - the fold's cost is asserted by counts, not by the clock

Status: **draft, step 1** (2026-11-16). Rigour: **standard** (the fold's semantics do not change; no model).
Gate: `gates/TC1.toml` (draft). Red witnesses: `gates/TC1/WITNESSES.md`.

## Why

`TestCreatorSelfCutsCostBounded` failed on GitHub Actions (run 43782425437, job `ci`): 7.99 s against a 25.8 ms
baseline, 309×. It passes locally only through its absolute 5 s clause; its 50× ratio clause never held, because
the ratio grows with the number of self-cuts (80×, 157×, 175× in time at 256, 512, 1024 ops, instrumented copy,
2026-11-16). `TestCascadeSingleSweep` (`< 100ms` of wall clock) has the same flaw. Both measure the machine, not the
fold. Project rule (method v2, « test quality »): a real-time test that fails under load is redone on controlled
time, each assertion at the cheapest layer that keeps its defect class. The defect class here is « the fold does
more work than stated »: work is a count, and a count does not depend on the load.

The cause of the cost, unchanged by this block: `selfCut(k)` (`protocol/trust/fold.go`) judges the self-cut at
position k by one full `sweep` over its ancestors `anc(k)`, memoised by position. `protocol/trust/doc.go` says it
(« plus one more fold per self-cut »); `docs/protocol/spec.md` §4.9 states O(n²/64 + n·e) and omits that term.

## Rules

n = ops in the list. A **self-cut** is, by the code's predicate (`isSelfCut`), any op that is not a grant (`level()`
below admin: a Remove, or an Add or SetRole to reader or writer) and whose subject is its author; s is their number.
A Suspend or Resume of oneself never counts: `Op.validate` refuses it as malformed, so no list holds one. `anc(k)` =
the ops the self-cut k sees, k excluded.

| Id | Rule | Test (oracle) | Mutant -> expected assertion text |
|---|---|---|---|
| K-1 | **Sweeps.** A fold makes one sweep, plus one per self-cut it judges, each once (the memo): **exactly 1 + s** when the author of every self-cut holds a rank (always so for the creator), and **between 1 and 1 + s** on any list (a self-cut of a key that is never ranked is not judged). Measured on the instrumented fold: 421 sweeps for the cascade list (n = 1024, s = 420), 513 and 1024 at the seal (K-2's two lists) | `TestCreatorSelfCutsCostBounded` (420 sibling cuts of the creator over a cascade of 150, to the seal: 421), `TestSelfCutsCostAtSeal` (subtest `chain of 12` first: 13 sweeps, and the test stops there when it fails; then `sibling`: 513, `chained`: 1024), `TestFoldCostWithinSelfCutBound` (random lists of `genList`, self-cuts read from the reference model: 1 ≤ sweeps ≤ 1 + s, and at least one drawn list holds a self-cut) | MK1 the verdict no longer memoised (`v.memo[k] = c` dropped; measured 841 sweeps for 421) -> `K-1 sweeps`; MK3 a second sweep per cut (measured 25 for 13 on the chain of 12) -> `K-1 sweeps`; MK5 MK1's edit on chained cuts (measured 531 441 sweeps for 13 on the chain of 12, in 0.7 s; the seal lists are not built) -> `K-1 sweeps`; MK6 MK3's edit on the random lists (measured 3 sweeps for 1 self-cut) -> `K-1 bound` |
| K-2 | **Ops swept.** The sweeps of K-1 read **exactly n + Σ\|anc(k)\|** ops, the sum over the judged self-cuts (at most that sum on any list, at least n). Closed forms, n = m + s: **sibling** cuts (s cuts that all see the same m ops): n + s·m, largest at m = s = n/2, i.e. n + n²/4; **chained** cuts (each sees the previous one, the first sees m ops): n + s·m + s(s−1)/2, largest at m = 1, i.e. n(n+1)/2, which is also the largest sum of any list. Proof: in the order ≺ an op sees only ops before it, so the op at position i (from 0) sees at most i ops, and the genesis (position 0) is a grant; hence Σ\|anc(k)\| ≤ 1 + 2 + … + (n−1) = n(n−1)/2 and the ops swept are at most n + n(n−1)/2 = n(n+1)/2. The chain from the genesis reaches it: every op but the genesis is a self-cut that sees all the ops before it. **At the seal (n = 1024): sibling 1024 + 512·512 = 263 168 ops swept in 513 sweeps; chained 1024 + 1023 + 1023·1022/2 = 524 800 ops swept in 1024 sweeps.** The cascade list of K-1: 1024 + 420·604 = 254 704 | the same three tests, through `wantFoldCost`: the closed form given by the test is first checked against `ancestorCount` (a walk of `Prev`, independent of the fold's ancestor bitsets), then the counters must equal it; the random lists check n ≤ swept ≤ n + Σ\|anc(k)\| | MK2 a self-cut judged on every op before it, not on its ancestors alone (same 421 sweeps; measured 342 694 ops for 254 704) -> `K-2 ops swept` |
| K-3 | **The cascade costs one sweep.** With no self-cut (the list of `buildCascade(250)`: 250 admins each demoting another while each is removed, every verdict depending on the previous one), the fold makes 1 sweep over n = 1001 ops, and the roles are those the test already required | `TestCascadeSingleSweep` (1 sweep, 1001 ops swept, then the roles) | MK4 the fold sweeps the whole list twice (measured 2 sweeps over 2002 ops) -> `K-3 one sweep` |
| K-4 | **The documents say it, truly, and agree.** In §4.9 of `docs/protocol/spec.md`, the paragraph that states O(n²/64 + n·e) gets this sentence after its first one, word for word: « Chaque coupe de soi (une op qui n'est pas un octroi et dont le sujet est l'auteur) est en plus jugée par un balayage de ses seuls ancêtres, fait une seule fois : un pli fait au plus 1 + s balayages pour s coupes de soi (exactement 1 + s quand l'auteur de chacune a un rang), chacun au coût d'un balayage donné ci-dessus, et ces balayages lisent au plus n + Σ\|anc(k)\| ops, soit au plus 524 800 ops balayées pour une liste scellée ; ce nombre compte les ops données aux balayages, ce n'est pas une borne de temps. » No requirement word is added (the paragraph keeps its one DOIT). `protocol/trust/doc.go` replaces « plus one more fold per self-cut » by: « plus, for each self-cut (a non-grant op whose subject is its author), one sweep of its ancestors alone, made once: at most 1 + s sweeps for s self-cuts (exactly 1 + s when the author of each holds a rank), each at the cost of one sweep, over at most n + Σ\|anc(k)\| ops, that is at most 524 800 ops swept for a sealed list; that number counts sweep inputs, it is not a time bound. » | gate criterion `docs`: in that paragraph « dont le sujet est l'auteur », « au plus 1 + s balayages », « exactement 1 + s quand », « au coût d'un balayage », « 524 800 ops balayées », « pas une borne de temps », and one DOIT; in `doc.go` « non-grant op whose subject is its author », « at most 1 + s sweeps », « exactly 1 + s when », « at the cost of one sweep », « 524 800 ops swept », « not a time bound », the old sentence gone | none: a text; its red witness is today's spec |

Why counts are a sound oracle: `sweep` is the only place where the fold rules are evaluated and the only caller of
`decide`; its work is a function of the ops it is given. The count of sweeps and of ops handed to them is therefore
the same on every machine, under `-race`, and under any load. What the counts do not bound (the cost of one sweep,
O(n²/64 + n·e)) stays covered by `BenchmarkFoldAtCap`, indicative as every micro-benchmark (method v2).

Prior art, and why it applies here: the Go standard library asserts cost by counts where the clock would be noise —
`testing.AllocsPerRun` (https://pkg.go.dev/testing#AllocsPerRun) for allocations, and `TestCountSortOps` and
`TestCountStableOps` of `sort` (https://cs.opensource.google/go/go/+/master:src/sort/sort_test.go), which count
calls to `Less` and `Swap` through the tested type. The fold has no caller-supplied operation to count through, so
the count lives on its result; alternatives weighed: a hook or an exported statistic (an API for a test: refused),
a build-tagged counter (two builds to keep equal), `testing/synctest` (it controls timers, not CPU time, so it
does not apply), a benchmark threshold (the clock again).

## Step 2 (the only production change)

Two unexported counters on the fold's `view`, `sweeps int` and `swept int`, incremented at the top of
`(*view).sweep` (`v.sweeps++`, `v.swept += len(in)`). No hook, no exported API, no build tag, no change of any
verdict. The tests read them right after `fold(l.ops)`: the reads made later on a stored view (`otherAdminSeenBy`
at `Issue`, `current()`) add their own sweeps and are not part of a fold's cost. `foldcost_counts_test.go`, the
test-only accessor that today stops every count test with « TC1 step 1: the fold does not count its sweeps yet »,
keeps its name: step 2 changes only the body of `foldCost`, to `return v.sweeps, v.swept`. §4.9 and `doc.go` get K-4's sentence. No `time.Now` or
`time.Since`, and no import of `time`, is left in `regress_test.go`, `regress2_test.go`, `foldcost_test.go` or
`foldcost_counts_test.go` (already so at step 1; the gate keeps it so).

Step 1 chose a red that is an **assertion**, not a compile error: a build failure would turn the whole package red
(every test of `trust`, with no reason shown), and `mutants.py` rightly never counts a build failure as a catch.
With the accessor, each named test fails alone with its reason, the rest of the package stays green, and no mutant
is « caught » by the stub's text (it carries none of the three expected texts).

## Mutants

MK1..MK6 in `scripts/netsim/mutants.toml`, block `TC1`, kind `go`, each with `test` and `expect`. The package is in
the nested module: the entries name it by import path (`github.com/oioio-space/inkwell/protocol/trust`), which
`go test` resolves from the root through the `replace` of `go.mod` — measured: `ok … 0.203s` from the root, where
`./protocol/trust` is refused. `mutants.py` is unchanged. Each mutant was applied by hand to a copy carrying the two
counters; the numbers in the table are those runs. Without the memo, chained cuts cost at least 2^s sweeps (3^12 measured on the chain of 12): a list at the seal
would run to the cap, which is not a catch. So `TestSelfCutsCostAtSeal` checks a chain of 12 first and stops when it
fails (MK5), MK1 stays on sibling cuts, and both `go test` commands of the gate carry `-timeout=90s` so that Go
names a stuck test. `TestFoldCostWithinSelfCutBound` fails with one text for both bounds, because rapid shrinks to
any failure (MK6 was first seen caught under the other bound's text).

## Non-goal, recorded as residual R-TC1 for TR1

The **algorithmic improvement** is not in this block: memo by (author, ancestor set) so that sibling cuts share one
sweep, or an incremental verdict that extends the previous sweep along a chain. It changes how verdicts are
computed, so it belongs to TR1 at « complete » rigour (model, mutants, equivalence with the reference fold).
Measured today, to size it: a sealed list costs up to 1024 sweeps and 524 800 ops swept (chained) or 513 sweeps and
263 168 ops (sibling) where a list without self-cut costs 1 sweep and 1024 ops; the with/without ratio in time is
80×, 157×, 175× at 256, 512, 1024 ops; the fold of the cascade list took 7.99 s on a loaded GitHub runner. The cost
is bounded by the 1024-op seal, and only an admin can spend it. **Not bounded by any gate after TC1:** the cost
inside one sweep — `spared` for each pair of an op and an exit, `survivors` in e² per key, the O(n²/64 + n·e) the
spec states. TC1 counts sweeps and their inputs only; `BenchmarkFoldAtCap` is indicative. That term stays an
unproven residual for TR1, which must bound it by counts too or prove it. → TR1

## Budget

`max_minutes = 12`. Caps: go 120 s (measured 17.3 s green with `-race` on a copy carrying the counters), mutant-tool
60 s (under 1 s), mutants 300 s (six mutants; MK6 took 3.6 s, and 30 s when rapid shrank for long), regression 120 s
(9 s), docs and simplify-report uncapped (text checks); 600 s in all.
