# Cloud review of the sidecar sandbox and the bridge (2026-11-14)

Scope: the 58 source files of `internal/sidecar`, `internal/bridge` and `protocol/bridge` on `main` 3b516c7 (6 976
lines), reviewed by `/code-review ultra` (multi-agent cloud review, launched by the owner). Ten findings, four normal
and six minor. **None has been replayed yet: each is a hypothesis until a failing test or a measured run proves it**
(rule « no empiricism »); the owning block writes that test first.

| # | Severity | Where | Finding | Block |
|---|---|---|---|---|
| 1 | normal | `internal/sidecar/syscalls_linux_arm64.go`, `scripts/sidecar-allowlist.py` | arm64: the seccomp list has neither `poll` nor its arm64 equivalent `ppoll` (73), because the generator drops names without an arm64 number instead of translating them; the Rust runtime calls `poll` before `main`, gets EPERM and aborts, so tinymist would never start on arm64 | SB2 |
| 2 | normal, security | `internal/sidecar/seccomp_linux.go:145` | `prlimit64` is not restricted to the caller: in the fallback without a PID namespace a compromised sidecar can zero the limits of any process of the user, Inkwell included. Proposed: allow pid 0 or self only, as for `tgkill` | SB2, first |
| 3 | normal | `internal/sidecar/launcher_linux.go:113-123` | a NoNetwork launch without a mount namespace exits 125: on Ubuntu 23.10+ and 24.04 with AppArmor, PID 1 lacks the capabilities to mount `/proc`, so the language server would not start | SB2 |
| 4 | normal | `internal/sidecar/run.go:213-217` | the fallback from cgroup to per-process limits always fails on the Landlock path: the second attempt finds the folder filled by the first and refuses it as « not empty » | SB2 |
| 5 | minor | `internal/bridge/router.go:233-256` | the per-peer output queue has no byte cap: 64 messages of up to 64 MiB, about 4 GiB per slow peer; `protocol.RepoOutputBytesPerPeer` (16 MiB) exists and is not used | BR1 |
| 6 | minor | `internal/bridge/router.go:241-248` | the webview can create a goroutine, a queue and a counter for any peer identifier, without limit or cleanup | BR1 |
| 7 | minor | `internal/bridge/storestream.go:437`, `:573`, `:642` | the store stream logs one line per faulty frame without a limiter (the router has one) | BR1 |
| 8 | minor | `internal/sidecar/corelimit_linux_amd64.go:3` | dead `setrlimit` branch and wrong comments: arm64 does have `setrlimit` (164) | SB2 |
| 9 | minor | `internal/sidecar/run.go:228` | in mount mode `watchDir` scans, 50 times a second, a host folder that stays empty | SB2 |
| 10 | minor | `internal/bridge/protocol.go:83` | `maxWire = 64 << 20` is a private copy of `protocol.MaxNetEnvelopeBytes`, against the invariant « no private copy of the protocol's codecs » | BR1 |

## Disposition (integrator, 2026-11-14)

- **SB2 — sidecar sandbox** (findings 2 first, then 1, 3, 4, 8, 9): rigour **complete** (sandbox boundary). A failing
  test per finding before any fix. Findings 1 and 3 need a real environment (arm64; Ubuntu 24.04 with AppArmor). Finding
  3 and the GitHub job `sidecars-arm64` (red since 2026-10-31 on « namespace probe inconclusive … permission denied »,
  repaired in CI on 2026-11-14 by lifting the AppArmor restriction on the runner) point at the same question, to settle
  first because it decides whether R1 runs on a stock Ubuntu 24.04: what does a person get there, where nobody lifts
  the restriction? The package tests PKG-01…11 passed on Ubuntu 24.04 on 2026-11-13; what they cover of the sidecar
  start is to check.
- **BR1 — bridge** (findings 5, 6, 7, 10): rigour **standard**; 5 and 6 are bounds on what the webview and a slow
  peer can make Go hold.
- Order: after W58 (its gate is the candidate in verification and is independent of these files); SB2 before PR1.
  Finding 2 does not jump ahead of W58's gate: it needs the fallback path without a PID namespace and a sidecar that
  is already compromised, and R1 is an alpha between trusted people; it is the first item of SB2.

## Evidence gathered the same day (GitHub run 43624790761, code reading, `docs/plans/2026-11-14-ubuntu-2404-sandbox.md`)

- Finding 8 is **confirmed in the code and is a security defect on arm64, not a minor one**: `corelimit_linux_arm64.go:4`
  sets `setrlimitSyscall = ^uint32(0)` under the comment « arm64 has no legacy setrlimit syscall », but
  `SYS_SETRLIMIT = 164` exists (x/sys `zsysnum_linux_arm64.go:173`), so the core-limit filter never denies
  `setrlimit(RLIMIT_CORE)` there; `TestTargetCannotLowerCoreLimit/setrlimit` fails on the arm64 runner.
- Finding 1 is **partly confirmed**: `scripts/sidecar-allowlist.py:37-45` drops `poll` (no arm64 number) instead of
  mapping it to `ppoll` (73), absent from `syscalls_linux_arm64.go`; on the arm64 runner typst and tinymist die at every
  launch (`exit -1`, empty stderr), but the log holds no direct proof that `poll` is the cause (unlisted calls get
  ENOSYS, `seccomp_linux.go:74`). To settle with the trace task on arm64.
- Finding 3 is **confirmed as a code path**: on Ubuntu 24.04 a user namespace is created but holds no capability; the
  mount probe answers « no », the isolation probe (`launcher_linux.go:107`) returns before mounting `/proc` and answers
  « yes », the real launch then fails to mount `/proc` and exits 125. tinymist fails silently, typst with « exited with
  status 125 ». Not yet observed on a real 24.04 machine.
- Also found: `definitiveRefusal` accepts EPERM, EINVAL and ENOSPC but not EACCES, which is what the runner returned
  (« permission denied »): the launch stayed « inconclusive » and was refused instead of degrading.
- The package tests PKG-01…11 start the window in a container and never start a sidecar: they prove nothing about the
  sandbox on Ubuntu 24.04; the .deb ships no AppArmor profile; every CI job that runs the sandbox lifts the restriction.
