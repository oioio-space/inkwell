# SB2 - the sidecar sandbox off the developer machine: four blocks

Status: step 1, revision 3 (2026-11-14): the integrator's design decisions D1–D9 after Codex's round 2 (REJECT,
`gates/SB2/REVIEW.md` « Round 2 »); round 1 answered in rev 2; gates not frozen. Decision:
`docs/adr/0044-sandbox-tiers.md` rev 3. Measurements: `gates/SB2/MEASUREMENTS.md` (runs 1 and 2 on stock Ubuntu
24.04) and « Local measurements » below. Requirements W69 (an R1 criterion), W68. Rigour: **complete** for SB2a1,
SB2a2 and SB2p, standard for SB2b.

| Order | Block | Content | Gate |
|---|---|---|---|
| 0 | probe run 3 | `sandbox-probe.yml` with the new Q9, Q12, Q13 (below), before SB2a1's freeze | `gates/SB2/MEASUREMENTS.md` |
| 1 | **SB2a1** | tiers N and F, one decision, verification at each launch, the filiation protocol (D1), no process creation in F, the perimeter (D2), `clone3` fallback on amd64 (D4), `prlimit64` pid 0, deadlines, sentences, the stock-Ubuntu job on amd64, the candidate published and checked (D6), arm64 **refused** with its sentence | `gates/SB2a1.toml` (draft rev 3) |
| 2 | **SB2a2** | arm64: `ppoll` and only what probe run 3 proves needed (D5), `setrlimit` 164 with the raw-call test (M4), the arm64 inventory (D7), the refusal of step 1 lifted | `gates/SB2a2.toml`, written and frozen before its implementation, after SB2a1 is merged |
| 3 | **SB2p** | the AppArmor profile in the .deb (ADR point 6), its upgrade matrix (D9), the stock job's N step | `gates/SB2p.toml`, frozen before its implementation |
| 4 | **SB2b** | the allow-list generator translates names through a reviewed table and checks the generated files; `watchDir` not in mount mode; true comments | `gates/SB2b.toml` |

**Why SB2a is split (D8).** Estimated local criteria with arm64 included: docs 1, go 16, mutants 19 (MS1–MS15 plus four
arm64 ones), frontend 2, test:py 1, github 1, regression 4 → **44 min**, at the ceiling with no margin, and arm64 needs probe-run round trips
(D5) that would hold the whole block. Split: SB2a1 **about 39 min** (below), SB2a2 **about 10 min** local (interpreted
filter tests per architecture 2, its mutants 4, docs 1, github 1, test:py 1, regression of the sidecar packages 1); the
arm64 runs themselves happen on GitHub before the gate and are only read by it. Each gate also gets a global cap of
45 min (`timeout` around the whole run, `gates/<block>.toml` header) and is re-cut, never weakened, if the measured
smoke exceeds it.

## Local measurements (this machine, Fedora 45, kernel 7.3, amd64, 2026-11-14)

| What | How | Result |
|---|---|---|
| Process creation needed? | a scratch seccomp wrapper: fork and vfork EPERM, `clone` without `CLONE_THREAD` EPERM, `clone3` ENOSYS | the fixture project compiles to PDF and SVG **byte-identical** to the unfiltered run (`--creation-timestamp` fixed); `typst query` works; a tinymist session gives **7 completions and 2 diagnostics, as without the filter** |
| Go runtime and `clone3` | a Go program starting 16 locked threads under the same filter | threads work; `exec.Command` is refused |
| Memory | `/usr/bin/time -v`; `ulimit -v` bisection | peak resident 70 MiB (compile), 180 MiB (tinymist); the compile needs between 128 and 256 MiB of address space |
| Linking (Q12) | `file`, `readelf`, `ldd` | typst: static-pie musl; tinymist: dynamic `linux-gnu`, interpreter `/lib64/ld-linux-x86-64.so.2`, glibc symbol versions up to 2.28, runs on the machine's glibc (2.44) |

`strace` is not installed here: Q9 and Q13 run on the runners only.

## Probe run 3 (before SB2a1's freeze) — what it must settle

- **Q9 (D3)** — every thread, process creation and `execve` of typst and tinymist during the whole perimeter (D2),
  tracing only the sidecar and its descendants (the tracer is their parent; the driver is not traced), with pid,
  parent pid, call, path, argv and result. If tinymist creates a process: which program, for which perimeter item.
  That item is then refused in F (never process creation allowed), and SB2a1 names it and its sentence.
- **Q12** — the runners' glibc and the sidecars' linking on amd64 and arm64 (D4's matrix).
- **Q13 (D1, D3)** — the `prctl` options and `ioctl` requests each perimeter item makes: the `prctl` allow-list of F
  is exactly that set minus `PR_SET_PDEATHSIG`.
- **Q8 on arm64 with `ppoll` allowed** is SB2a2's first measurement (D5), not SB2a1's.

## SB2a1 - rules

Mutants are entries of `scripts/netsim/mutants.toml` (block `SB2a1`), each with `expect`: the named test must fail with
the rule tag in its own `go test -json` events.

| Id | Rule | Test (oracle) | Mutant → expected |
|---|---|---|---|
| T-1 | `decideTier` is total and follows ADR point 2, with arm64 refused | `TestDecideTierExhaustive`: landlock {none, 2, 3, 6, 8} × seccomp {ok, no} × ns {ok, refused, inconclusive} × f {ok, failed, inconclusive} × arch {amd64, arm64} (180) against a hand table | MS1 refused + f failed gives F → `T-1 tier` |
| T-2 | errno classes of `ns` are total | `TestRefusalErrnoTable`: every errno of `clone(2)`, `unshare(2)`, `mount(2)` and the id-map writes, each with its class | MS2 EACCES dropped → `T-2 errno` |
| T-3 | the probes are the launches; a refusal at any step gives F | `TestProbeIsTheLaunch` (a recorder seam requires the same steps); `TestNoCapabilityNamespaceGivesF` (a user namespace whose id map cannot be written: EACCES, as measured) | MS3 the N probe returns before mounting `/proc` → `T-3 probe is the launch` |
| T-4 | one configuration per tier; the launch verifies its tier before the target starts; the handshake fails closed | `TestLaunchVerifiesTier` (wrong tier reported → target never starts, error names both); `TestOneConfigurationPerTier`; `TestHandshakeFailsClosed/{no-report,truncated,invalid,eof,no-proceed,cancel-before-report,cancel-after-report,cancel-after-proceed}` (each: error, launcher killed and reaped within 2 s, target never started where the phase is before proceed); `TestDescriptorsClosedBeforeExec` (the target lists `/proc/self/fd`: only 0, 1, 2 and the report pipe) | MS4 proceed without comparing → `T-4 verified`; MS11 a descriptor leaked → `T-4 descriptors` |
| T-5 | no wait without a deadline, termination and reaping included | `TestRunReturnsAfterCancel/{N,F}×{cgroup,rlimit where the tier allows}` (returns within 2 s, launcher and target reaped: no zombie); `TestProbeDeadline` | MS5 `captureDeath` waits without the context → `T-5 deadline` |
| T-6 | the cgroup → rlimit fallback (N, PID namespace present) creates the output slots once; no cgroup without a PID namespace | `TestPlacementRefusedFallsBack`; `TestNoCgroupWithoutPIDNamespace` | MS6 slots created before the retry → `T-6 fallback` |
| S-1 | `prlimit64` only with pid 0, both tiers, both architectures (interpreted filters) | `TestSeccompPrlimitPidZero` (pid {0, self, parent, 1, -1, 2^31, 2^32+self} × resource × pointer {nil, set}, for `AUDIT_ARCH_X86_64` and `AUDIT_ARCH_AARCH64`); `TestTargetCannotLimitAnotherProcess/{N,F}` | MS7 pid check removed → `S-1 prlimit64` |
| S-2 | F: no process creation | `TestSeccompNoProcessInF` (interpreted filter: `clone` with and without `CLONE_THREAD` and every flag combination of `CLONE_VM`, `CLONE_VFORK`, `CLONE_PARENT`, `CLONE_NEWUSER`; `clone3`; fork; vfork; per architecture) | MS8 `clone` allowed without `CLONE_THREAD` → `S-2 no process` |
| S-4 | F: filiation (D1): parent-death signal armed, parent checked, `prctl` limited to the measured options, `PR_SET_PDEATHSIG` denied after arming; the forking thread locked for the child's life | `TestSeccompPrctlOptions` (interpreted: each measured option allowed, `PR_SET_PDEATHSIG` and every other option EPERM, both architectures); `TestFiliation/{disarm,parent-dies-before-arming,parent-dies-after-arming,parent-dies-during-run}` (the sidecar is gone within 1 s, by pid; « before arming » forced by a seam that delays step 1 and kills the parent) | MS9 no parent check → `S-4 parent check`; MS12 `PR_SET_PDEATHSIG` allowed → `S-4 prctl` |
| S-5 | the perimeter works in F with the final filter (D2), amd64 | `TestPerimeterF/{compile-pdf,compile-svg,query,eval,lsp}`: same bytes (PDF and SVG with the timestamp fixed), same JSON, same eval output; LSP: same capabilities, diagnostics before and after `didChange`, completion labels, signature help, hover, cancellation honoured; against an unrestricted run of the same binary | MS13 `ppoll`-like gap: one call the perimeter needs removed from the amd64 list → `S-5 perimeter` |
| S-6 | threads without `clone3` (D4), amd64 | `TestCloneThreeFallback/{tinymist,typst}`: `clone3` answered ENOSYS, more than one thread created (`/proc/<pid>/status` `Threads:`), the perimeter's results unchanged, the glibc version recorded in the test output | MS14 `clone3` answered EPERM instead of ENOSYS → `S-6 clone3` |
| S-7 | arm64 refused until SB2a2 | `TestArm64Refused` (`decideTier` with arch arm64 → refuse with `preview.fail.sandbox`; on the arm64 runner a real `Run` returns that refusal, never a crash) | MS15 arm64 allowed → `S-7 arm64` |
| U-1 | the About line and the refusal sentence follow the outcome; never « exited with status » | `TestIsolationReportsTier`; Vitest « SB2a1 about shows the tier: namespaces », « …: reduced », « SB2a1 a refused sandbox shows its sentence and Détails » | in-test: the status text restored → U-1 |
| C-1 | the candidate is published and its GitHub runs are checked at gate time (D6) | criterion `github` with `scripts/gh-verdict.py` (below) | - |

**Escape tests** (ADR point 5), each a real target under `Run` with a sentinel the test checks
(`TestEscape/<tier>/<attempt>`): `parent-environ`, `parent-maps` (a secret in the parent's environment must not
appear), `proc-self-root`, `write-outside`, `replace-outside`, `slot-budget`, `exec-slot`, `signal-parent`
(`kill`, `tgkill`), `tkill-other-thread` (a thread of the parent: refused), `prlimit-parent`, `socket-tcp`,
`socket-udp`, `socket-unix`, `socket-abstract`, `fork-bomb`, `allocation`, `parent-death`, `ptrace-parent`.
**Landlock ABI witnesses**: `TestEscape/F-abi3/<attempt>` and `TestEscape/F-abi4/<attempt>` run the same attempts with
the ruleset created for ABI 3 and ABI 4 (the launcher takes the ABI to request as a test seam: it asks
`landlock_create_ruleset` with only the access rights that ABI knows and no scoping), proving that seccomp alone gives
the socket and signal protections the lower ABIs lack.

**Tier-aware suites**: `TestCgroupSessionScopeDelegated`, `TestCgroupRunLimitsVerifiedBeforeStart`,
`TestCgroupWorkloadAndDescendantsInside` (its wait cycle fixed: stdin released before waiting on `runPath`),
`TestRunNoNetworkFallsBack`, `TestHostileCgroupMemoryIsMemoryLimit`, `TestHostileMemoryDeathStartsNoCoreHelper`,
`TestEmbeddedSidecarsRun` assert the promises of the tier their machine gives, reported in each test's output.

## The mandatory inventory (D7)

Each machine kind runs its column and uploads `go test -json`; the gate reads every column from
`gates/SB2a1/inventory.toml`, the machine-readable form of this table, written and reviewed with the gate's freeze. A cell is **pass**
(required), **n-unavailable** (accepted only in an N column, and only when the launch's tier report for that test
says N is unavailable on that machine; a distinct status, never counted as a pass), or **refused** (required on arm64
in SB2a1). No skip is accepted anywhere in an F column; a missing cell, an extra skip or a failure fails the gate.

| Tests | local N (this machine) | local F (forced) | stock amd64 (runner, restriction on) | arm64 runner |
|---|---|---|---|---|
| T-1…T-6, S-1, S-2, S-4 interpreted tests, U-1 (Go) | pass | pass | pass | pass |
| `TestEscape/<attempt>` (every attempt above) | pass | pass | F: pass; N: n-unavailable | refused |
| `TestEscape/F-abi3/*`, `TestEscape/F-abi4/*` | — | pass | pass | refused |
| `TestFiliation/*` | pass | pass | pass | refused |
| `TestPerimeterF/*`, `TestCloneThreeFallback/*` | — | pass | pass | refused |
| tier-aware suites | pass | pass | pass (in F) | refused |
| `TestArm64Refused` | pass | pass | pass | pass |

## The candidate and its GitHub proof (D6)

The block's candidate is a commit on a branch `candidate/sb2a1-<n>` of the public repository (never `main`); the
integrator pushes it and starts `ci.yml` on it with `workflow_dispatch` (the trigger is added by this block; the
`sandbox-stock` job — `ubuntu-24.04`, restriction left on, asserted to be 1 with AppArmor enabled — and the
`sidecars-arm64` job run there). The local file `gates/SB2a1/github.json` holds only `{ "run_id": N }`. At gate time
`scripts/gh-verdict.py --check gates/SB2a1/github.json --inventory gates/SB2a1/inventory.toml` re-fetches through the
GitHub API, and fails unless all hold: repository `oioio-space/inkwell` (also `head_repository`, so no fork);
workflow path `.github/workflows/ci.yml`; event `workflow_dispatch`; head branch `candidate/sb2a1-*`; `head_sha`
equal to `git rev-parse HEAD` of the gate's checkout, whose tracked tree is clean; status `completed` and conclusion
`success` for the run and for each required job (`ci`, `netsim`, `sandbox-stock`, `sidecars-arm64`); each required
artefact present, downloaded and parsed as `go test -json`; and every cell of the inventory satisfied (the tier report
lines of the artefacts decide `n-unavailable`). Integration is a fast-forward of `main` to that same commit.
`scripts/gh_verdict_test.py` (in `test:py`) replays recorded API answers: another repository, a fork, another
workflow path, another event, another SHA, a dirty tree, `in_progress`, a failed job, a missing artefact, a missing
cell, an extra skip in F, `n-unavailable` without the matching tier report, `n-unavailable` in an F column — each
refused; the true record accepted.

## CI

`.github/workflows/ci.yml`: `workflow_dispatch` added; the `ci` and `sidecars-arm64` jobs stop lifting
`kernel.apparmor_restrict_unprivileged_userns` once SB2a1 lands; new job `sandbox-stock` on `ubuntu-24.04`: (1) assert
the sysctl is 1 and AppArmor is enabled, else fail; (2) `go test -json` of `internal/sidecar`, `internal/typst`,
`internal/language`, uploaded as an artefact, the decided tier and its errno printed (expected: F, EACCES); (3, added
by SB2p) install the built .deb, assert the profile is loaded, run again and expect N. Every step has a timeout.

Where each proof exists: the interpreted filters anywhere; the escape tests, filiation, handshake and the ABI
witnesses here (N and F) and on the stock runner (F); the perimeter and `clone3` here (F) and on the stock runner;
arm64's refusal on the arm64 runner; N under the installed package (SB2p); a desktop Ubuntu 24.04 with the package:
the owner (`gates/SB2p/UX.md`).

Formal model: none. `decideTier` is a pure total function over 180 inputs, proven exhaustively against a hand oracle;
each tier's effect is proven by the escape tests inside it; the filiation and the handshake are a two-process protocol
with one direction of trust, tested at every phase with controlled faults (`docs/engineering-principles.md` §0).

Files: `internal/sidecar/` (`tier_linux.go` new; `launcher_linux.go`, `run.go`, `mount_linux.go`, `netns_linux.go`,
`dir_linux.go`, `death_linux.go`, `seccomp_linux.go`, `scope_linux.go`), `internal/typst/engine.go`,
`internal/language`, `internal/app`, `frontend/src/ui/AboutDialog.svelte`, `preview/`, `i18n/*.json`,
`.github/workflows/ci.yml`, `scripts/gh-verdict.py` and `scripts/gojson_check.py` (new, each with its test in
`test:py`), `scripts/netsim/mutants.py` (`--exclude`), `docs/architecture/spec.md` §9.11.

## SB2a2, SB2p and SB2b (gates written and frozen before their implementation)

| Id | Rule | Test |
|---|---|---|
| A-1 | arm64: `ppoll` (73) allowed; any other addition only from a remeasure that shows normal work needs it; `tkill` (130) stays denied unless a remeasure shows it is needed for normal work, then allowed only for the caller's own threads (arguments filtered) with `TestEscape/F/tkill-other-thread` deciding | the arm64 column of the inventory turned to pass; `TestArm64Refused` replaced by `TestArm64Runs` |
| A-2 | arm64: `setrlimitSyscall` is 164; a raw `syscall(164, RLIMIT_CORE, …)` from the target tells apart « wrong constant » (the filter's rule never matches), « already refused by the allow-list » (EPERM from the default action) and « attack reached » (the limit changed) | `TestRawSetrlimitArm64` |
| P-1 | the .deb ships, loads and unloads the profile; never fails the installation; the .rpm ships none | `pkg:test` PKG-12, PKG-13 per distribution |
| P-2 | the upgrade matrix: install; upgrade from a version without the profile; upgrade with it; remove; purge; AppArmor absent; AppArmor disabled — each leaves the profile loaded or absent as stated and the decided tier as expected | `pkg:test` PKG-14 (matrix), `sandbox-stock` step 3 |
| B-1 | the generator translates a name without a number on an architecture through a reviewed table and fails on a name with neither; `--check` compares the generated files | `scripts/sidecar_allowlist_test.py`; `--check` in `ci:core` |
| B-2 | `watchDir` does not run in mount mode; the two `corelimit_*` comments are true | `TestNoPollingInMountMode` |

## Durations of SB2a1 (estimates; measured at the smoke; the gate has a global cap of 45 min)

`docs` < 1 min; `go` (one `-json` run of the three packages, both local columns) about 15 min; `mutants` (MS1–MS15,
all runnable here: each mutates a rule checked by an interpreted filter or a local launch, 15 × about 1 min) about 15 min; `frontend` about 2 min; `test:py` (gh-verdict,
gojson_check) about 1 min; `github` (API reads and artefact downloads) about 1 min; `regression` about 4 min (the
preview spec only) → **about 39 min**, under the 45-minute cap with 6 min of margin.
