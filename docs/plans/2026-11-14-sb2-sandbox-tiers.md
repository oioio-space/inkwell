# SB2 - the sidecar sandbox off the developer machine: two blocks

Status: step 1 draft, not reviewed, gates not frozen. Decision: `docs/adr/0044-sandbox-tiers.md`. Inputs:
`docs/reviews/2026-11-14-ultrareview-sandbox-bridge.md`, `docs/plans/2026-11-14-ubuntu-2404-sandbox.md`,
`gates/CI1c/GITHUB.md`. Requirements W69 (Ubuntu 24.04, R1 criterion), W68. Rigour: **complete** (sandbox boundary).

| Order | Block | Content | Gate |
|---|---|---|---|
| 1 | **SB2a** | tiers and one decision, no wait without a deadline, `prlimit64` on self only, the memory fallback on the Landlock path, what the person is told, the AppArmor profile, a CI job with the restriction on | `gates/SB2a.toml` (draft) |
| 2 | **SB2b** | arm64: `setrlimit` filtered, allow-list names translated, an arm64 trace, architecture-neutral tests; findings 8 and 9 | `gates/SB2b.toml`, written when SB2a is merged |

SB2a first: it carries W69, and the Landlock tier it makes the normal path on Ubuntu is only acceptable with S-1.

## Review findings against the code (main e256ea6)

| # | Finding | Status |
|---|---|---|
| 1 | arm64 lacks `ppoll` | **confirmed in code**: of the 71 allowed names, 8 have no arm64 number (`access`, `arch_prctl`, `epoll_wait`, `lstat`, `open`, `poll`, `readlink`, `stat`) and `scripts/sidecar-allowlist.py:37-45` drops them; their arm64 forms `faccessat`, `epoll_pwait`, `newfstatat`, `openat`, `readlinkat`, `statx` are already listed, `arch_prctl` has none, **`ppoll` (73) is the only one missing**. That it is what kills typst and tinymist on the arm64 runner (run 43629366160) is likely, not proved: an unlisted call gets EPERM (`seccomp_linux.go:112-117`), and no trace was ever taken on arm64 |
| 2 | `prlimit64` on any pid | **confirmed in code** (`seccomp_linux.go:143-144`: only `RLIMIT_CORE` with a new limit is refused) |
| 3 | exit 125 without a mount namespace | **confirmed in code** (ADR 0044, context 1); not observed on a real Ubuntu 24.04 |
| 4 | cgroup fallback fails on the Landlock path | **confirmed in code** (`run.go:213-217`, `dir_linux.go:36-51`) |
| 8 | `setrlimit` on arm64 | **confirmed in code and by a run**: `corelimit_linux_arm64.go:4` is `^uint32(0)`, `SYS_SETRLIMIT` is 164; `TestTargetCannotLowerCoreLimit/setrlimit` fails on the arm64 runner. The amd64 file's comment is the arm64 one, copied |
| 9 | `watchDir` polls in mount mode | **confirmed in code**: `run.go:228` starts it whatever the mode, 20 ms ticks, on a host folder the private tmpfs hides |

## SB2a - rules

| Id | Rule | Test (oracle) | Mutant -> expected text |
|---|---|---|---|
| T-1 | `decideTier` is total and follows the ADR's table | `TestDecideTierExhaustive`: every combination of the five facts (2 x 2 x 3 x 3 x 2 = 72) against a table written by hand from the ADR, not from the code | MS1 `refused` + `plain` failed gives Landlock -> `T-1 tier` |
| T-2 | « refused » is total over errno: EPERM, EACCES, EINVAL, ENOSPC at limit 0, child status 120 are definitive; anything else is inconclusive | `TestRefusalErrnoTable` (every errno of `clone(2)`, `unshare(2)`, `mount(2)` man pages listed, each with its expected class) | MS2 EACCES dropped -> `T-2 errno` |
| T-3 | The probe is the launch: the namespaces step performs the id maps, the tmpfs, the IPC overlay, the `/proc` mount as PID 1, Landlock, seccomp and `execve`; a refusal at any of them selects the Landlock tier | `TestProbeNamespacesWithoutCapabilities` (seam: the probe child is started in a user namespace that maps the uid onto itself, so it holds no capability, as on Ubuntu 24.04): tier Landlock, and a real compile then succeeds; `TestProbeUsesLaunchSteps` (the probe and the launch share one function) | MS3 probe returns before the `/proc` mount -> `T-3 probe is the launch` |
| T-4 | A launch uses the decided tier only; `probeOutputMount` and `probeIsolation` no longer exist | `TestLaunchFollowsTier` (forced tiers: clone flags, mounts and `Output.Isolated` match the tier); `deadcode` | MS4 `isolated()` probes again -> `T-4 one decision` |
| T-5 | No wait without a deadline: `Run` returns within 2 s of its context ending, in each tier and each memory bound, with a target that never ends and answers nothing | `TestRunReturnsAfterCancel/{namespaces,landlock}x{cgroup,rlimit}` (controlled: the target blocks on a pipe the test holds) | MS5 `captureDeath` waits without the context -> `T-5 deadline` |
| T-6 | The cgroup-to-rlimit fallback works on the Landlock path | `TestPlacementRefusedFallsBackOnLandlockPath` (seam: placement error on the first start) | MS6 slots created before the retry -> `T-6 fallback` |
| S-1 | `prlimit64` is allowed only with pid 0 or the caller's own pid, on both architectures; the `RLIMIT_CORE` rule stays | `TestSeccompPrlimitOnlySelf` (filter interpreted in the test over pid x resource x pointer, as `TestCoreSentinelSeccompPolicy` does); `TestTargetCannotLimitAnotherProcess` (a real target in the Landlock tier calls `prlimit64` on its parent: EPERM, the parent's limit unchanged) | MS7 pid check removed -> `S-1 prlimit64` |
| U-1 | The About line and the failure sentences follow the tier (keys of ADR 0044 §5); a refused launch never shows « exited with status » | `TestIsolationReportsTier` (Go, the three outcomes); Vitest « SB2a about shows the tier: namespaces / reduced »; « SB2a a refused sandbox shows its sentence and Détails » | in-test: status text restored -> U-1 |
| P-1 | The .deb ships `/etc/apparmor.d/inkwell` with the ADR's text, loads it when AppArmor is there, unloads it on removal, and no step can fail the installation; the .rpm ships none | `pkg:test` cases PKG-12 (file content, deb only), PKG-13 (install and removal succeed without `apparmor_parser`) | - |
| C-1 | A CI job `sandbox-stock` runs on `ubuntu-24.04` **without** lifting the restriction: it prints the decided tier and the errno that decided it, runs the `internal/sidecar` and `internal/typst` tests tier-aware, and compiles one document through the built application | the job itself; its verdict reaches the gate through `gates/SB2a/github.json` | - |

Tier-aware tests (no skip for « not my machine »): a helper reports the tier and the memory bound of the machine; each
test that asserted cgroup or namespace behaviour unconditionally (`TestCgroupSessionScopeDelegated`,
`TestCgroupRunLimitsVerifiedBeforeStart`, `TestCgroupWorkloadAndDescendantsInside`, `TestRunNoNetworkFallsBack`,
`TestHostileCgroupMemoryIsMemoryLimit`, `TestHostileMemoryDeathStartsNoCoreHelper`, `TestEmbeddedSidecarsRun`) asserts
the guarantees of the tier it runs in, and the hostile cases run in every tier the machine can force. A test may skip
only a tier the machine cannot offer, and says which. First task of the block: reproduce the two tests that never
return on the runner (no delegated scope: unset the session bus locally) and write down the cause before T-5.

Where each proof exists:

| Proof | Where |
|---|---|
| T-1, T-2, S-1 filter, U-1 | anywhere (pure functions, interpreted filter) |
| Landlock tier by a real launch; T-3 with the no-capability seam; T-5, T-6 | this machine (it offers Namespaces + cgroup, and can force the three lower combinations) |
| The Landlock tier decided by a real refusal, with its errno (experiment **E-1**: is the restriction on by default on the amd64 image, and is the refusal EACCES at clone or EPERM at mount?) | job `sandbox-stock` |
| The Namespaces tier under the installed profile, through the `/proc/self/exe` re-execution (experiment **E-2**: the job installs the built .deb, loads the profile, and expects the Namespaces tier) | job `sandbox-stock`, second step |
| arm64 | job `sidecars-arm64`: green only after SB2b; SB2a must not make it worse (same failing set or smaller) |
| A desktop Ubuntu 24.04 with the package: preview, export, completion, the About line | the owner, at the end (R1 acceptance, `gates/SB2a/UX.md`) |

Formal model: none. `decideTier` is a pure function over 72 inputs; the exhaustive table test against a hand-written
oracle covers every case, which a TLA+ or Alloy model could only restate (`docs/engineering-principles.md` §0: a model
is for interleavings and unbounded state). The deadline rule is a liveness property of one process and is tested with
controlled time. Test usefulness: manual mutants MS1 to MS7 in `scripts/netsim/mutants.toml` (block `SB2a`, with the
`expect` field that lands with W58), each caught by its named test failing with its rule tag.

Wording: `about.isolation.reduced`, `preview.fail.sandbox`, `preview.fail.sandbox.details` (« Détails ») as in ADR
0044 §5; `about.network.proxy` is removed with its last reader.

Files: `internal/sidecar/` (`tier_linux.go` new: facts, `decideTier`, the probe sequence; `launcher_linux.go`,
`run.go`, `mount_linux.go`, `netns_linux.go`, `dir_linux.go`, `death_linux.go`, `seccomp_linux.go`,
`scope_linux.go`), `internal/typst/engine.go`, `internal/language`, `internal/app` (what the About dialog reads),
`frontend/src/ui/AboutDialog.svelte`, `preview/`, `i18n/*.json`, `build/linux/nfpm/` (profile, scripts),
`scripts/pkg/`, `.github/workflows/ci.yml`, `docs/architecture/spec.md` §9.11 (the table), `docs/exigences.md`.

## SB2b - arm64 (written in full when SB2a is merged)

| Id | Rule | Test |
|---|---|---|
| A-1 | `setrlimitSyscall` is 164 on arm64; the two `corelimit_*` comments say what is true | `TestCoreSentinelSeccompPolicy` built per architecture (it hard-codes `AUDIT_ARCH_X86_64` today), `TestTargetCannotLowerCoreLimit` on the arm64 runner |
| A-2 | The generator translates a name without a number on an architecture through a reviewed table (`poll` -> `ppoll`, `open` -> `openat`, `stat`/`lstat` -> `newfstatat`, `access` -> `faccessat`, `readlink` -> `readlinkat`, `epoll_wait` -> `epoll_pwait`; `arch_prctl` -> none) and fails on a name that has neither a number nor an entry | `scripts/sidecar_allowlist_test.py`; `--check` in `ci:core` |
| A-3 | The allow-list is the union of traces taken on both architectures; the arm64 job runs `sidecar:trace` and keeps the evidence | the job; `TestEmbeddedSidecarsRun` green on arm64 |
| A-4 | `watchDir` does not run in mount mode (finding 9) | `TestNoPollingInMountMode` |

## Risks

The no-capability seam (T-3) imitates Ubuntu's restriction and may differ from it: E-1 is run before the freeze of
`gates/SB2a.toml`, and the gate is frozen only once the job has shown the real errno. The AppArmor flag name and the
re-execution (E-2) can change point 4 of the ADR, not the tiers. Budget: the local criteria are sized under 45 min;
if the tier matrix of the hostile tests exceeds it, the matrix is cut by pairwise coverage, never by dropping a tier.
