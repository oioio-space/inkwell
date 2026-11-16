# SB2 - the sidecar sandbox off the developer machine: four blocks

Status: step 1, revision 3 (2026-11-14): the integrator's design decisions D1–D9 after Codex's round 2 (REJECT,
`gates/SB2/REVIEW.md` « Round 2 »), confirmed by Codex (« Round 3 »); gate `gates/SB2a1.toml` frozen. Decision:
`docs/adr/0044-sandbox-tiers.md` rev 3. Measurements: `gates/SB2/MEASUREMENTS.md` (runs 1 to 4 on stock Ubuntu
24.04) and « Local measurements » below. Requirements W69 (an R1 criterion), W68. Rigour: **complete** for SB2a1,
SB2a2 and SB2p, standard for SB2b.

| Order | Block | Content | Gate |
|---|---|---|---|
| 0 | probe runs 3 and 4 | done: Q9, Q12, Q13 on both runners, settled facts below | `gates/SB2/MEASUREMENTS.md` |
| 1 | **SB2a1** | tiers N and F, one decision, verification at each launch, the filiation protocol (D1), no process creation in F, the perimeter (D2), `clone3` fallback on amd64 (D4), `prlimit64` pid 0, deadlines, sentences, the stock-Ubuntu job on amd64, the candidate published and checked (D6), arm64 **refused** with its sentence | `gates/SB2a1.toml` (frozen) |
| 2 | **SB2a2** | arm64: `ppoll`, then a remeasure; anything more only if it shows normal work needs it (D5), `setrlimit` 164 with the raw-call test (M4), the arm64 inventory (D7), the refusal of step 1 lifted | `gates/SB2a2.toml`, written and frozen before its implementation, after SB2a1 is merged |
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

`strace` is not installed here: Q9 and Q13 ran on the runners (runs 3 and 4).

## Probe runs 3 and 4 — settled facts (`gates/SB2/MEASUREMENTS.md`)

- **Q9 (D3)** — traced alone (the tracer is the sidecar's parent), across the whole perimeter (D2), on amd64 and
  arm64: **no process is created**; typst compile to PDF and SVG, query and eval run with 4 threads, the tinymist
  session with 14 threads, each with one `execve` (its own start). Run 2's « child » was the probe driver's `Popen`.
  F's no-process rule removes no perimeter item.
- **Q12 (D4)** — typst is static on both architectures (no interpreter); tinymist is dynamic glibc, symbol versions
  up to `GLIBC_2.28` (amd64) and `GLIBC_2.30` (arm64); the runners have glibc 2.39. This is a fact for S-6, not a
  replacement for it.
- **Q13 (D1, D3)** — `prctl`: `PR_SET_NAME` only (tinymist, thread names), none for typst, so F's `prctl` allow-list
  is `PR_SET_NAME` and `PR_SET_PDEATHSIG` is denied; `ioctl`: `TIOCGWINSZ` (typst), `FIONREAD` and `TCGETS`
  (tinymist), F's `ioctl` allow-list.
- **arm64** — the allow-list refuses `ppoll` (both sidecars call it at start); `tkill` appears only in the abort path
  that follows. How both behave once `ppoll` is allowed is SB2a2's first measurement (D5), not SB2a1's.

## SB2a1 - rules

Mutants are entries of `scripts/netsim/mutants.toml` (block `SB2a1`), each with `expect`: the named test must fail with
the rule tag in its own `go test -json` events.

| Id | Rule | Test (oracle) | Mutant → expected |
|---|---|---|---|
| T-1 | `decideTier` is total and follows ADR point 2, with arm64 refused | `TestDecideTierExhaustive`: landlock {none, 2, 3, 6, 8} × seccomp {ok, no} × ns {ok, refused, inconclusive} × f {ok, failed, inconclusive} × arch {amd64, arm64} (180) against a hand table | MS1 refused + f failed gives F → `T-1 tier` |
| T-2 | errno classes of `ns` are total | `TestRefusalErrnoTable`: every errno of `clone(2)`, `unshare(2)`, `mount(2)` and the id-map writes, each with its class | MS2 EACCES dropped → `T-2 errno` |
| T-3 | the probes are the launches; a refusal at any step gives F; the target is the pinned, verified `typst --version` under typst's descriptor (amended 2026-11-15, ADR 0044 point 2: the host executable depended on host GTK libraries) | `TestProbeIsTheLaunch` (a recorder seam requires the same steps); `TestProbeTargetIsTheVerifiedTypst`; `TestProbeRunsVersionUnderTypstDescriptor`; `TestUnverifiableProbeTargetIsNotInconclusive`; `TestNoCapabilityNamespaceGivesF` (a user namespace whose id map cannot be written: EACCES, as measured) | MS3 the N probe returns before mounting `/proc` → `T-3 probe is the launch` |
| T-4 | one configuration per tier; the launch verifies its tier before the target starts; the handshake fails closed | `TestLaunchVerifiesTier` (wrong tier reported → target never starts, error names both); `TestOneConfigurationPerTier`; `TestHandshakeFailsClosed/{no-report,truncated,invalid,eof,no-proceed,cancel-before-report,cancel-after-report,cancel-after-proceed}` (each: error, launcher killed and reaped within 2 s, target never started where the phase is before proceed); `TestDescriptorsClosedBeforeExec` (the target lists `/proc/self/fd`: only 0, 1, 2 and the report pipe) | MS4 proceed without comparing → `T-4 verified`; MS11 a descriptor leaked → `T-4 descriptors` |
| T-5 | no wait without a deadline, termination and reaping included | `TestRunReturnsAfterCancel/{N,F}×{cgroup,rlimit where the tier allows}` (returns within 2 s, launcher and target reaped: no zombie); `TestProbeDeadline` | MS5 `captureDeath` waits without the context → `T-5 deadline` |
| T-6 | the cgroup → rlimit fallback (N, PID namespace present) creates the output slots once; no cgroup without a PID namespace | `TestPlacementRefusedFallsBack`; `TestNoCgroupWithoutPIDNamespace` | MS6 slots created before the retry → `T-6 fallback` |
| S-1 | `prlimit64` only with pid 0, both tiers, both architectures (interpreted filters) | `TestSeccompPrlimitPidZero` (pid {0, self, parent, 1, -1, 2^31, 2^32+self} × resource × pointer {nil, set}, for `AUDIT_ARCH_X86_64` and `AUDIT_ARCH_AARCH64`); `TestTargetCannotLimitAnotherProcess/{N,F}` | MS7 pid check removed → `S-1 prlimit64` |
| S-2 | F: no process creation | `TestSeccompNoProcessInF` (interpreted filter: `clone` with and without `CLONE_THREAD` and every flag combination of `CLONE_VM`, `CLONE_VFORK`, `CLONE_PARENT`, `CLONE_NEWUSER`; `clone3`; fork; vfork; per architecture) | MS8 `clone` allowed without `CLONE_THREAD` → `S-2 no process` |
| S-4 | F: filiation (D1): parent-death signal armed, parent checked, `prctl` limited to `PR_SET_NAME` and `ioctl` to `TIOCGWINSZ`, `FIONREAD`, `TCGETS` (measured, Q13), `PR_SET_PDEATHSIG` denied after arming; the forking thread locked for the child's life | `TestSeccompPrctlOptions` (interpreted: `PR_SET_NAME` allowed, `PR_SET_PDEATHSIG` and every other option EPERM, both architectures); `TestSeccompIoctlRequests` (interpreted: the three requests allowed, every other request EPERM, both architectures); `TestFiliation/{disarm,parent-dies-before-arming,parent-dies-after-arming,parent-dies-during-run}` (the sidecar is gone within 1 s, by pid; « before arming » forced by a seam that delays step 1 and kills the parent) | MS9 no parent check → `S-4 parent check`; MS12 `PR_SET_PDEATHSIG` allowed → `S-4 prctl` |
| S-5 | the perimeter works in F with the final filter (D2), amd64 | `TestPerimeterF/{compile-pdf,compile-svg,query,eval,lsp}`: same bytes (PDF and SVG with the timestamp fixed), same JSON, same eval output; LSP: same capabilities, diagnostics before and after `didChange`, completion labels, signature help, hover, cancellation honoured; against an unrestricted run of the same binary | MS13 `ppoll`-like gap: one call the perimeter needs removed from the amd64 list → `S-5 perimeter` |
| S-6 | threads without `clone3` (D4), amd64 | `TestCloneThreeFallback/{tinymist,typst}`: `clone3` answered ENOSYS, more than one thread created (`/proc/<pid>/status` `Threads:`), the perimeter's results unchanged, the glibc version recorded in the test output | MS14 `clone3` answered EPERM instead of ENOSYS → `S-6 clone3` |
| S-7 | arm64 refused until SB2a2 | `TestArm64Refused` (`decideTier` with arch arm64 → refuse with `preview.fail.sandbox`; on the arm64 runner a real `Run` returns that refusal, never a crash) | MS15 arm64 allowed → `S-7 arm64` |
| S-8 | font folders: exactly the caller's list, read-only, both tiers; no hard-coded fallback (ADR 1d, N43) | `TestFontFoldersExactlyThoseGiven/{N,F}` (a folder holding a font file not in the list: the target cannot open it; one in the list: readable); `TestFontFoldersExactlyThoseGiven/F-provided-font` (a compile whose document uses a font present only in the project's `fonts/` succeeds in F, and its PDF embeds that font) | MS16 the two-folder fallback restored → `S-8 fonts` |
| S-9 | a protected project is refused in tier F with `preview.fail.protected`, runs in N (ADR 1e, N37) | `TestProtectedRefusedInF` (pure: `decideTier` with `Protected` and F → refuse with that key; a real `Run` in forced F returns it, the target never starts); `TestProtectedRunsInN` | MS17 protected allowed in F → `S-9 protected` |
| S-10 | one `Program` descriptor per program: allow-list per architecture, `prctl` and `ioctl` lists, memory bound, perimeter; `decideTier` per program (ADR 1f; N21, N23, N32, AD-57) | `TestProgramDescriptors` (typst and tinymist are table entries; no other code path names them); `TestThirdProgramDescriptor/{N,F}` (a test program registered with its own minimal lists runs in both tiers, a call outside its list is refused, typst's lists are not applied to it) | MS18 the `ioctl` list shared by every program instead of the descriptor's → `S-10 descriptor` |
| S-11 | a bound violation in N with a scope is inkwell's own kill above `memory.high`, never a cgroup OOM kill; `memory.max` is a backstop above it; tier F fails the allocation through `RLIMIT_AS` (ADR 1g, W73) | `TestBoundViolationIsNotAnOOMKill/{cgroup,rlimit}` (a target allocating without end: the run returns the bound outcome; the scope's `memory.events` `oom_kill` is 0; with a systemd user manager reachable, the unit's `Result` is not `oom-kill`; in F the target dies of its failed allocation, no cgroup); `TestHostileCgroupMemoryIsMemoryLimit` rewritten to that outcome; the backstop test `TestMemoryMaxBackstop` runs only with `INKWELL_OOM_BACKSTOP=1` (it is the one that may raise the desktop notification) | MS19 the watchdog disabled (the bound reached at `memory.max`) → `S-11 no oom-kill` |
| U-1 | the About line and the refusal sentence follow the outcome; never « exited with status »; amended 2026-11-15 (UX-SB2a1-2 to -5, sentences in ADR 0044 point 7): refusal « L'aperçu est désactivé : ce système ne permet pas d'isoler le moteur de mise en page. Le texte et la collaboration fonctionnent normalement. », About states the same fact (`about.isolation.refused`, `about.isolation.pending`); an inconclusive probe shows « L'aperçu n'a pas pu démarrer cette fois. » with « Réessayer », never « réinstallez-le »; reduced tier in About: « Vos fichiers restent protégés : le moteur de mise en page ne peut ni lire vos autres fichiers ni utiliser le réseau. Ce système ne permet pas l'isolation complète : le moteur n'est pas entièrement séparé des autres programmes ouverts. »; the protected sentence ends with its next step (« Vous pouvez continuer à écrire ; pour voir l'aperçu, ouvrez-le sur un autre ordinateur. »); focus returns to « Détails » | `TestIsolationReportsTier`; `TestNetworkModeNamesARefusedOrPendingSandbox`; `TestFailureOf` (retry); Vitest « SB2a1 about shows the tier: namespaces », « …: reduced », « SB2a1 a refused sandbox shows its sentence and Détails », « SB2a1 an engine that could not start this time offers Réessayer, never a reinstallation », « SB2a1 retry asks Go to compile again… », « SB2a1 about explains a refused sandbox and what to do », « SB2a1 about says a start that did not succeed yet can be tried again », « SB2a1 about gives the focus back to the button that opened it » | in-test: the status text restored → U-1 |
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
`gates/inventories/SB2a1.toml`, the machine-readable form of this table, written with the gate's freeze; every test name it lists is the contract. A cell is **pass**
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
`scripts/gh-verdict.py --check gates/SB2a1/github.json --inventory gates/inventories/SB2a1.toml` re-fetches through the
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

**Revised 2026-11-16 (debated with Codex).** The candidate's GitHub run 43820841198 was green on every test but red on
`cover:check`: `internal/sidecar` 73.7 % < 85 %, because with `ci` no longer lifting the restriction, no GitHub job
exercised tier N. Decision: the `ci` job lifts the restriction again and asserts, from the sandbox's own tier report,
that tier N really ran (`TestIsolationReportsTier`: `tier=N n=available`); `sandbox-stock` keeps the stock restriction
and now also runs `internal/app` (its tests compile and export a real document: the application's integration with the
sandbox in tier F, which `ci` no longer exercises); `sidecars-arm64` is unchanged. GitHub then proves tiers N and F and
the arm64 refusal on every push. Rejected: per-tier thresholds or exemptions (weaken the coverage gate); merging the
coverage of two jobs (orchestration for the same proof). The frozen `github` criterion is unchanged.

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

`docs` < 1 min; `go` (one `-json` run of the three packages, both local columns) about 15 min; `mutants` (MS1–MS19,
all runnable here: each mutates a rule checked by an interpreted filter or a local launch, 19 × about 1 min) about 19 min; `frontend` about 2 min; `test:py` (gh-verdict,
gojson_check) about 1 min; `github` (API reads and artefact downloads) about 1 min; `regression` about 4 min (the
preview spec only) → **about 43 min**, under the 45-minute cap with 2 min of margin (the four rules S-8–S-11 added at the freeze, 2026-11-14: fonts N43, protected projects N37, program descriptors for plugins, no desktop OOM alarm W73). If the smoke exceeds the cap, SB2a1 is re-cut (S-10 first, into its own block), never weakened.
