# SB2 - the sidecar sandbox off the developer machine: three blocks

Status: step 1, revision 2 (2026-11-14), on measured facts; round 1 of review answered (`gates/SB2/REVIEW.md`, column
« Answered in rev 2 »); round 2 pending; gates not frozen. Decision: `docs/adr/0044-sandbox-tiers.md` rev 2.
Measurements: `gates/SB2/MEASUREMENTS.md` (stock Ubuntu 24.04, run 43638847261) and « Local measurements » below.
Requirements W69 (an R1 criterion), W68. Rigour: **complete** for SB2a and SB2p, standard for SB2b.

| Order | Block | Content | Gate |
|---|---|---|---|
| 0 | probe run 2 | `sandbox-probe.yml` again with Q8–Q11 (below), before SB2a's freeze | `gates/SB2/MEASUREMENTS.md`, round 2 |
| 1 | **SB2a** | tiers N and F, one decision, verification at each launch, no process creation in F, `prlimit64` pid 0, deadlines, the person's sentences, the stock-Ubuntu CI job (F), arm64's measured allow-list gaps | `gates/SB2a.toml` (draft rev 2) |
| 2 | **SB2p** | the AppArmor profile in the .deb, built and installed by the gate; the CI job's second step (N under the installed package) | `gates/SB2p.toml`, written when SB2a is merged |
| 3 | **SB2b** | the allow-list generator translates names through a reviewed table and checks the generated files; `watchDir` not in mount mode; true comments | `gates/SB2b.toml`, written when SB2p is merged |

SB2p is split from SB2a for the 45-minute ceiling: building and testing the package on three distributions is about
15 minutes alone (estimate from `pkg:test` on 2026-11-13).

## Local measurements (this machine, Fedora 45, kernel 7.3, amd64, 2026-11-14)

| What | How | Result |
|---|---|---|
| Process creation needed? | a scratch wrapper installs a seccomp filter: fork and vfork EPERM, `clone` without `CLONE_THREAD` EPERM, `clone3` ENOSYS; then runs the pinned binaries | the rule bites (`sh -c 'true'`: « fork: Operation not permitted »); the fixture project (`internal/typst/engine_test.go`) compiles to PDF and SVG, **byte-identical** to the unfiltered run with `--creation-timestamp` fixed; `typst query` works; a tinymist session (initialize, open, completion, hover, shutdown) gives **7 completions and 2 diagnostics, as without the filter** |
| Go runtime and `clone3` | a Go program starting 16 locked threads under the same filter | threads work; `exec.Command` is refused (« operation not permitted ») |
| Memory | `/usr/bin/time -v`; `ulimit -v` bisection on the fixture compile | peak resident 70 MiB (compile), 180 MiB (tinymist session); the compile fails with 128 MiB of address space and succeeds with 256 MiB; the default `RLIMIT_AS` is 4 GiB |
| Both tiers through `Run` | `TestSandboxProbeTiers` (inert test, `INKWELL_SANDBOX_PROBE=1`) | decided tier: typst `--version`, compile to stdout (11 601-byte PDF, isolated), tinymist `--version` (isolated); forced namespace-free: the same three succeed, not isolated |

Not measured here: arm64, the stock runner's amd64 sidecars inside the sandbox, a desktop session. `strace` is not
installed on this machine: process creation was measured by refusing it, not by tracing it.

## Probe run 2 (before SB2a's freeze)

`scripts/sandbox-probe.sh` gains: **Q9** — `strace -f` of process creation (thread clones, process clones, `clone3`,
fork, vfork, execve) during a typst compile (PDF and SVG) and a tinymist language-server session, outside the
sandbox, on both architectures; **Q10** — the pinned sidecars through `Run` in the tier the machine decides
(`TestSandboxProbeTiers`, unprofiled: on stock Ubuntu, the refusal today); **Q11** — the same with F forced (does F
work on a stock runner?); **Q8** extended — under the candidate profile, `TestEmbeddedSidecarsRun` and the tier probe
run traced, with the calls refused inside (arm64's allow-list gaps, whether `setrlimit` gets EPERM: finding M4).
`load_profile` writes the ADR's exact text. A non-zero Q9 `clone_process`, `fork` or extra `execve` for a sidecar
changes ADR 0044 point 1 before the freeze.

## SB2a - rules

Mutants are entries of `scripts/netsim/mutants.toml` (block `SB2a`), each with `expect`: the named test must fail with
the rule tag in its own `go test -json` events (the `expect` field lands with W58a).

| Id | Rule | Test (oracle) | Mutant → expected |
|---|---|---|---|
| T-1 | `decideTier` is total and follows ADR point 2 | `TestDecideTierExhaustive`: every combination of landlock {none, 2, 3, 6, 8} × seccomp {ok, no} × ns {ok, refused, inconclusive} × f {ok, failed, inconclusive} (90) against a table written by hand from the ADR | MS1 refused + f failed gives F → `T-1 tier` |
| T-2 | errno classes of `ns` are total (ADR point 2) | `TestRefusalErrnoTable`: every errno of `clone(2)`, `unshare(2)`, `mount(2)` and the id-map writes, each with its class | MS2 EACCES dropped → `T-2 errno` |
| T-3 | the probes are the launches: `ns` runs the N code path with N's single configuration, `f` the F path; a refusal at any step gives F | `TestProbeIsTheLaunch` (both probes and both launches call one function per tier; a recorder seam lists the steps of each and requires equality); `TestNoCapabilityNamespaceGivesF` (seam: the probe child starts in a user namespace whose id map it cannot write, as measured on Ubuntu: EACCES) | MS3 the N probe returns before mounting `/proc` → `T-3 probe is the launch` |
| T-4 | one configuration per tier; the launch verifies its tier before the target starts (report, then proceed) | `TestLaunchVerifiesTier` (seam: the launcher reports a tier other than the decided one → the target never starts, the error names both); `TestOneConfigurationPerTier` (N launches with and without `NoNetwork` use the same flags and mounts) | MS4 the parent proceeds without comparing → `T-4 verified` |
| T-5 | no wait without a deadline | `TestRunReturnsAfterCancel/{N,F}×{cgroup,rlimit where the tier allows}` (the target blocks on a pipe the test holds; `Run` returns within 2 s of the context ending); `TestProbeDeadline` (a probe child that never answers) | MS5 `captureDeath` waits without the context → `T-5 deadline` |
| T-6 | the cgroup → rlimit fallback (N) creates the output slots once | `TestPlacementRefusedFallsBack` (seam: placement error on the first start) | MS6 slots created before the retry → `T-6 fallback` |
| S-1 | `prlimit64` only with pid 0, both tiers, both architectures; the `RLIMIT_CORE` rule stays | `TestSeccompPrlimitPidZero` (the filter interpreted in the test over pid {0, self, parent, 1, -1, 2^31, 2^32+self (64-bit high bits)} × resource × new-limit pointer {nil, set}, for `AUDIT_ARCH_X86_64` and `AUDIT_ARCH_AARCH64`); `TestTargetCannotLimitAnotherProcess` (F and N: a real target calls `prlimit64` on its parent: EPERM, the parent's limit unchanged) | MS7 pid check removed → `S-1 prlimit64` |
| S-2 | F: no process creation; `PR_SET_PDEATHSIG` SIGKILL before the target; execute only the target and its interpreter's files | `TestSeccompNoProcessInF` (interpreted filter: `clone` with and without `CLONE_THREAD`, `clone3`, fork, vfork, per architecture); escape tests below | MS8 `clone` allowed without `CLONE_THREAD` → `S-2 no process`; MS9 no parent-death signal → `S-2 parent death` |
| S-3 | arm64: the allow-list holds the calls probe run 2 lists as refused inside for a working sidecar (expected: `ppoll`); `setrlimitSyscall` is 164; a sidecar on arm64 runs only once its tests are green on the arm64 runner | `TestEmbeddedSidecarsRun` and the escape tests on the arm64 runner (CI); `TestCoreSentinelSeccompPolicy` built per architecture | MS10 `ppoll` removed (on arm64) → `S-3 allow-list` |
| U-1 | the About line and the refusal sentence follow the outcome (ADR point 7); a refusal never shows « exited with status » | `TestIsolationReportsTier` (Go); Vitest « SB2a about shows the tier: namespaces », « SB2a about shows the tier: reduced », « SB2a a refused sandbox shows its sentence and Détails » | in-test: the status text restored → U-1 |
| C-1 | job `sandbox-stock` on `ubuntu-24.04` and `ubuntu-24.04-arm`, restriction left on | the job (below); its verdict reaches the gate (criterion `github`) | - |

**Escape tests** (S-2 and ADR point 5), each a real target under `Run` with a sentinel the test checks, in F always and
in N where the machine offers it (`TestEscape/<tier>/<attempt>`): read the parent's `/proc/<ppid>/environ` and
`/proc/<ppid>/maps` (a secret placed in the parent's environment must not appear); follow `/proc/self/root` out of
the allowed paths; write or replace a file outside the output slots, and an output slot beyond its budget; execute a
binary written into an output slot; `tgkill` and `kill` on the parent; `prlimit64` on the parent; open TCP, UDP,
pathname and abstract UNIX sockets; a fork bomb (F: refused; N: bounded by the namespace and the cgroup when present);
an allocation beyond `RLIMIT_AS`; the parent killed while the target runs (the target is gone within 1 s, checked by
pid). A test may skip only a tier the machine cannot offer, with the message « tier unavailable: <tier> ».

**Tier-aware suites**: `TestCgroupSessionScopeDelegated`, `TestCgroupRunLimitsVerifiedBeforeStart`,
`TestCgroupWorkloadAndDescendantsInside` (its wait cycle fixed: stdin released before waiting on `runPath`),
`TestRunNoNetworkFallsBack`, `TestHostileCgroupMemoryIsMemoryLimit`, `TestHostileMemoryDeathStartsNoCoreHelper`,
`TestEmbeddedSidecarsRun` assert the promises of the tier they run in; none assumes the developer's machine.

**CI** (`.github/workflows/ci.yml`): the `ci` and `sidecars-arm64` jobs stop lifting
`kernel.apparmor_restrict_unprivileged_userns` once SB2a lands. New job `sandbox-stock` on both runners: (1) assert the
sysctl is 1 and AppArmor is enabled, else fail; (2) `go test -json` of `internal/sidecar`, `internal/typst`,
`internal/language` and a compile, an export and a language-server session through the engine, uploaded as artefacts,
with the decided tier and its errno printed (expected: F, EACCES); (3, added by SB2p) build the .deb, install it,
assert the profile is loaded, run the same in N. Every step has a timeout.

Where each proof exists: T-1, T-2, S-1 and S-2 filters, U-1 anywhere; T-3 to T-6 and the escape tests in both tiers on
this machine (it offers N and can force F); F on a stock Ubuntu and every arm64 proof on the runners (C-1); N under the
installed package (SB2p); a desktop Ubuntu 24.04 with the package: the owner (`gates/SB2p/UX.md`).

Formal model: none. `decideTier` is a pure function over 90 inputs, proven exhaustively against a hand oracle; each
tier's effect is proven by the escape tests run inside it; the deadline rule is a single process's liveness, tested
with controlled cancellation (`docs/engineering-principles.md` §0).

Files: `internal/sidecar/` (`tier_linux.go` new; `launcher_linux.go`, `run.go`, `mount_linux.go`, `netns_linux.go`,
`dir_linux.go`, `death_linux.go`, `seccomp_linux.go`, `scope_linux.go`, `corelimit_linux_{amd64,arm64}.go`,
`syscalls_linux_arm64.go`), `internal/typst/engine.go`, `internal/language`, `internal/app`,
`frontend/src/ui/AboutDialog.svelte`, `preview/`, `i18n/*.json`, `.github/workflows/ci.yml`, `scripts/gh-verdict.py`
and `scripts/gojson_check.py` (new, each with its test in `test:py`), `scripts/netsim/mutants.py` (`--exclude`), `docs/architecture/spec.md` §9.11.

## SB2p and SB2b (written in full when the previous block is merged)

| Id | Rule | Test |
|---|---|---|
| P-1 | the .deb ships `/etc/apparmor.d/inkwell` (ADR point 6), loads it when AppArmor is enabled, unloads it on remove and purge, never fails the installation; the .rpm ships none | `pkg:test` PKG-12 (content, deb only), PKG-13 (install and remove succeed with and without `apparmor_parser`), each printing `PKG-nn: pass` per distribution |
| P-2 | under the installed package on a stock Ubuntu runner the decided tier is N | `sandbox-stock` step 3 |
| B-1 | the generator translates a name without a number on an architecture through a reviewed table and fails on a name with neither; `--check` compares the generated files | `scripts/sidecar_allowlist_test.py`; `--check` in `ci:core` |
| B-2 | `watchDir` does not run in mount mode (finding 9); the two `corelimit_*` comments are true (finding 8) | `TestNoPollingInMountMode` |

## Durations (estimates; measured at the smoke, the gate is re-cut if the total exceeds 45 min)

SB2a: `docs` < 1 min; `go` (one `-json` run of the three packages, named tests and suites together) about 12 min;
`mutants` (10 × about 1 min) about 12 min; `frontend` about 2 min; `github` < 1 min; `regression` about 8 min;
total about 35 min.
