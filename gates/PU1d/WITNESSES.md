# PU1d - red witnesses

Recorded 2026-11-24 at step 1, before any production code, on the worktree `inkwell-pu1d` (branch `pu1d` from `main`
`abe7bac`), after the cross-family review (`SPEC-REVIEW.md`, DeepSeek V4 Pro, « accepter avec corrections »; findings
1, 2 and 4 applied). Each criterion of the draft `gates/PU1d.toml` was run once with its own command, through a small
runner that reads the TOML and runs each `command` with bash (`MISE_TRUSTED_CONFIG_PATHS` set to the worktree, whose
`mise.toml` is not trusted otherwise). The machine was shared with another session's netsim run and the coordinator's
`mise run formal`, which ran first; the TLA runs waited for it to end. Raw logs are in the session scratchpad
(`crit/*.log`); TLC logs are in `~/.cache/inkwell/tla/logs/PU1dCloseScope*.log`.

| Criterion | Cap | Exit | Duration | Reason read in the output | Right reason? |
|---|---|---|---|---|---|
| `docs` | 30 s | 0 | 0 s | every anchor present (spec §8.7.4-§8.7.9 and §16.1, TLA README, CHANGES) | green as expected: the documents are step 1's |
| `tla` | 390 s | 0 | 139 s | 10 ✓, no ✗: Any (7,407,601 states, 1,345,391 distinct, 33 s), Negotiated (17,257,071, 2,484,923, 83 s), Live (63,469, 19,957, liveness, 7 s); MC1 → `CrossedIndependent`, MC2 → `NeverStranded`, MC3 → `OpeningNamed`, MC3's consequence → `CrossedIndependent`, three witnesses; PeerUpAck byte-identical to `c7c90c7`; sound operators in place | green as expected: the model is step 1's |
| `vectors` | 120 s | 1 | 1 s | digests of `syncack.json` (34) and `syncorder.json` (20) match, no `close_peer`, the 13 vectors `close_conn`, `TestGenerateMatchesCommittedCorpus` passes; `TestCorpus` fails in `protocol/syncack` (9 `close_peer, want close_conn`) and in `protocol/conformance` (13: the 9 opener events and the 4 `expectation action: got close_peer, want close_conn`) | **yes**: the reference still answers `close_peer` (step 2) |
| `go` | 300 s | 1 | 6 s | the 7 named tests are missing (`missing or not passing`) | **yes**: step 2 writes them |
| `mutants` | 420 s | 1 | 0 s | `PU1d mutant entries differ from the frozen table: ['MD1', 'MD2', 'MD3']` (no entry yet, header) | **yes**: their anchors are step 2's code |
| `mutation` | 300 s | 1 | 0 s | gremlins: `failed to gather coverage`, because `protocol/syncack` `TestCorpus` fails (same 9 vectors) | **yes**: no valid measurement while the reference lags its vectors; it measures once step 2 lands |
| `regression` | 300 s | 0 | 107 s | `internal/transport/p2p` ok in 105.8 s, `internal/app` ok in 33.6 s | green as expected: a baseline on today's code. The first try failed in `internal/app` (`typst.gz is not in this build`). That was an environment prerequisite of a fresh worktree, not a defect; `mise run sidecar:fetch` (pinned, from the download cache) fixed it |
| `netsim` | 840 s | 0 | 18 s | scenario 9 crash, seed 20771, strict: C1d 6.5 s, C2d 7.0 s, C3k pass (CJ, CJd pass); the result file is fresh (03:39:33) | green as expected: a regression guard. Today's `ClosePeer` does not fire in this run, so the defect itself is shown red by `go` (tests 1-7), not here. The draft first required C3c; block PC1b renamed it C3k (« 0 in the crash window »), and the gate now names C3k |
| `simplify-report` | — | 2 | 0 s | `gates/PU1d/SIMPLIFY.md` absent | **yes**: step 3 |

Changes made to the draft while recording the witnesses:

- The `tla` count is 10, not 11: 3 sound runs, 4 mutant runs, 3 witnesses.
- `CloseScopeNegotiated` first had Any's Drop and took 116,425,537 states, 14,766,859 distinct and 9 min (no error, run
  once by the coordinator's formal). It now has no Drop and two openings per node: 83 s.
- Netsim clause C3c became C3k.
- Caps (G-1): 2700 s = `max_minutes` 45, checked with `gate.deadline_problem` (None).
