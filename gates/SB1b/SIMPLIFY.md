# SB1b simplification review — host validation pending

P1-a uses one inherited argument filter, not changes to dumpability at exec.
P1-b performs one early scan shared by both output formats before diagnostics.
The scope manager owns session versions; each run owns its directory fd and
watch; the holder alone owns app-EOF cleanup. Platform stubs preserve rlimits.
No child is migrated after launch, no untrusted pointer is dereferenced, and
an unavailable or ambiguous memory fact stays unknown. Memory history storage
is bounded and fails conservatively on overflow. Diagnostic errors from
nonlocal SIGKILL use fixed text. The journal check retains attributed evidence.

The core-policy and multi-format blocked tests were observed red then green.
Pure memory-cause tests and policy VM tests pass. Host cgroup, lifecycle,
journal, pinned typst and mutation checks must be supplied by Claude; this
report is not a claim that the frozen gate passed. The previous attempt logs
are historical environmental failures. No frozen gate, threshold or delay was
changed. No commits or stashes were created.

## /simplify review of the final diff (2026-11-13, Sonnet go-reviewer, read by Claude Opus)

| # | Angle | Finding | Decision |
|---|---|---|---|
| 1 | Docs | this report listed properties, not findings | applied: this table |
| 2 | Reuse | `/proc/<pid>/cgroup` read + `unifiedCgroup` duplicated (holder check, kill sweep) | applied: `pidCgroup`/`procCgroup` beside `selfCgroup` |
| 3 | Reuse | `/proc/<pid>/status` read in `death_linux.go` | discarded: once per signalled run, nothing to share |
| 4 | Simplification | hierarchical `memory.events` read only for readability | applied: comment says why the values are dropped; the `oom`/`oom_kill` loop helper discarded (low value) |
| 5 | Style | missing blank line before `recordRunManifest` | applied |
| 6 | Efficiency | per-ancestor memory samples at start and finish | discarded: bounded walk (stops at `user@UID.service`), needed to attribute the cause before cleanup |
| 7 | Efficiency | `Waitid(WNOWAIT)` on every run with a pidfd | discarded: the status is unknown before the wait |
| 8 | Altitude | `unix.Siginfo` reinterpreted via unsafe | discarded: x/sys exposes no `Pid`; documented, ABI-bound to 64-bit |
| 9 | Altitude | `*_other.go` stubs mirror the Linux API | discarded: existing project pattern |
| 10 | Simplification | `run`, `prepare`, `setup`, `sandboxInitWithProc` over the cognitive-complexity bound (lint, gate `regression`) | applied: split into `configureRunCommand`/`openRunMount`/`finishRunOutputs`, `ensureSession`/`configureRunLimits`, `inheritedScopeProperties`/`validateSession`, `runSandboxInit`/`reapSandboxTarget`/`refreshTargetIdentity`; same step order and cleanup (Opus review of the diff) |
| 11 | Style | `os.Exit` after `defer runtime.UnlockOSThread`, `strings.Fields` loops, `HasPrefix`+`TrimPrefix`, unchecked deferred closes, doc comments | applied: exit status returned to the caller, `FieldsSeq`, `CutPrefix`, explicit `_ =`, comments |

Earlier rounds: Codex's own review (2 P1, fixed), host smoke (6 criteria, fixed), Claude Opus review (5 + 5 items, fixed; narrow
re-check ACCEPT), recorded in `gates/SB1b/FIXES.md`.
