# CI3b — a content-keyed verdict cache for the formal models

Block CI3b, standard rigour (tooling), a slice of CI3 (`PROGRESS.md`). User remark, 2026-11-23: `mise run formal` and
the GitHub `formal` shards rerun every TLC and Alloy model on each push, although a verdict depends only on its inputs.

## 1. Measured cost before

- Local: the last full `mise run formal` (2026-11-24, 280 ✓ lines, exit 0) took **2752 s (45.9 min)**.
- GitHub, the three most recent green `main` runs (45361983697, 45337536707, 45306932828): `formal (tla 1/3)` 21.0,
  18.9, 18.9 min; `tla 2/3` 12.7, 10.1, 12.5 min; `tla 3/3` 32.3, 32.2, 21.2 min; `alloy` 0.5 to 0.8 min. The formal
  check is the longest leg of the push CI (tla 3/3 reaches 32 of its 45 min cap).
- Most pushes touch no model: of the last 12 commits of `main` (fadb655 back to e2dac6a), 2 changed
  `docs/protocol/tla` or `docs/protocol/alloy` (RF1, PU1d).

## 2. Design

**Unit and key.** One cache entry per `run NAME CFG MODULE KIND [PROP]` of `scripts/tla-check.sh` and per Alloy
execution of `scripts/alloy_check.py` (a model file, or one mutant). The key is the SHA-256 of a canonical JSON of:
the schema version; the tool; the SHA-256 of every file of the model's closure — the module, and every module it
`EXTENDS` or `INSTANCE`s, resolved transitively after comments are stripped, each name looked up in the referring
file's folder and in TLC's working folder (every local file found is included, so no lookup order is assumed); a name
found nowhere must be a module shipped in the jar (`Naturals`, `FiniteSets`, …), else the run is not cacheable and
runs normally; Alloy: the (mutated) text and its local `open`s, `util/*` being in the jar; the cfg; the pinned jar's
SHA-256; the full `java -version` text; the run's arguments (name, kind, expected property, workers, and for a
simulation its seed, depth and trace count); `scripts/formal_cache.py` itself; and the code that judges the run —
every function `tla-check.sh` defines (`declare -f` of `run` and of each helper, environment functions left out) for
TLC, the whole of `alloy_check.py` minus its `MUTANTS` list (data) for Alloy. The judging code, not the list of runs: the run list is in each key's arguments, so adding a model changes no other key, while any change to how a
verdict is decided changes them all. This is the shape of Bazel's action key (the digest of the command and of the
whole input tree; an action declares every input, and an undeclared input is a correctness bug, never a guess) [1][2].

**Only green verdicts.** An entry is written only when the run is green (sound, safety, simulate: no error; mutant: a
violation of the expected property); a failure, a timeout or an interrupted run writes nothing, so a red verdict
always reruns. Writes are atomic (temporary file, then `rename`). The remote API leaves caching to the client's
judgement (`do_not_cache`) [1]; here the rule is fixed, as a cached failure would hide a fix.

**Output.** A reused run prints the stored ✓ line byte for byte, then `reused <12 hex>: <name>` on its own line: line
counts and `grep "✓ …"` checks see the same lines, and the reuse is visible.

**Bypass.** `INKWELL_FORMAL_NO_CACHE=1` skips the cache entirely (no lookup, no store). `scripts/gate.py` sets it in
every criterion's environment (`criterion_env`), inherited through `heavy.sh`, `timeout` and `mise run`: a gate —
e.g. `gates/PU1d.toml` and `gates/RF1.toml` `tla`, which call `tla-check.sh` — always runs every model itself.
`mise run done` step 4 may reuse (inner loop); its step 6, the gate, does not.

**Expiry.** An entry older than `INKWELL_FORMAL_CACHE_TTL_DAYS` (default 7) is a miss and is deleted; reuse does not
refresh it. So every verdict is re-earned by a real run at least weekly, locally and on GitHub, without a scheduled
workflow (the `ci` workflow has none, and adding one would rerun the whole CI, netsim included).

**Store.** Locally `${XDG_CACHE_HOME:-~/.cache}/inkwell/formal-verdicts` (outside the repo; `INKWELL_FORMAL_CACHE`
overrides). On GitHub each `formal-checks` leg restores with `actions/cache/restore` and saves with
`actions/cache/save` (`if: always()`, safe since only green verdicts are in the folder). A cache key is immutable [3],
so each run saves under its own key (`…-formal-verdicts-<shard>-<run id>-<attempt>`) and restores the newest of its
shard by prefix; GitHub evicts entries unused for 7 days and lets a branch restore `main`'s caches [3]. The folder is
kept apart from the tool cache (`~/.cache/inkwell`), whose key does not change per run.

## 3. Alternatives rejected

- Hash the whole `docs/protocol/tla` folder plus the runner files: one edited model, or one added run, would rerun all
  308 runs, i.e. most blocks that touch the models would gain nothing.
- Key on git tree ids (`git rev-parse HEAD:docs/protocol/tla`): same granularity problem, and wrong for uncommitted work.
- Let TLC's own `-recover`/fingerprint files serve as cache: they resume a state space, they do not certify a verdict.
- Cache failures too, to save their rerun: a failure is rerun on purpose, it is what the author is fixing.
- `schedule:` trigger for the periodic full run: reruns the whole workflow; the TTL gives the same bound locally too.

## 4. Proof

`scripts/formal_cache_test.py` (in `test:py`): the key changes with each input (jar, Java, runner, arguments, cfg, a
module two `EXTENDS` away); comments are not read as modules; an unknown module is not cacheable; expiry; refusal of an
empty verdict or a bad key; `tla-check.sh` end to end on a fixture tree with a fake `java` that counts its calls (the
real `run()` kept byte for byte): second run reuses with identical ✓ lines and runs nothing, an `EXTENDS`ed module edit
reruns, a failure is not stored, the bypass reruns; `alloy_check.check` reuse, an opened-module edit and a failure;
`gate.py`'s `criterion_env` sets the bypass. Manual mutations (no transitive closure; storing a failed sound run) each
turn tests red. Gain: § 5.

## 5. Measured gain

Local, 2026-11-24, same tree: first full `mise run formal` (empty cache, machine shared with a push CI) 4223 s; second
run, nothing changed, **19 s**: 308 TLC and 15 Alloy verdicts reused, 0 ✗, the same 308 ✓ lines. After the runner key
was widened to every function (review): fill 2610 s, then 14 s with 323 of 323 reused. One run alone:
`ConnCredit` 0.11 s reused, 1.35 s with `INKWELL_FORMAL_NO_CACHE=1`. GitHub: to be measured on the first two pushes after
the merge (the first fills the cache; a push that touches no model should bring each TLA leg from 10-32 min to its
setup time, about 1 min).

## Sources

1. Bazel, Remote Execution API, `Action` (`command_digest`, `input_root_digest`, `do_not_cache`):
   https://github.com/bazelbuild/remote-apis/blob/main/build/bazel/remote/execution/v2/remote_execution.proto
2. Bazel, Remote caching (action cache: action hash → result; inputs declared per action):
   https://bazel.build/remote/caching
3. GitHub, Dependency caching reference (immutable keys, `restore-keys` prefix, newest match, 7-day eviction, branch
   scope): https://docs.github.com/en/actions/reference/workflows-and-actions/dependency-caching
