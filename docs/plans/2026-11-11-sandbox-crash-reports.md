# W49: the sandbox refuses a document without the desktop reporting a crash

- **Requirement:** W49 (`docs/exigences.md`, 2026-11-11). GNOME keeps showing crash notices on the user's
  Fedora 44 desktop.
- **Status:** design. Nothing is implemented yet. The gate is `gates/SB1.toml` (see « Gate SB1 », which replaces the `gates/W49.toml` of § 8.2 and § 9); the decision is ADR 0042.
- **Rigour tier:** **complet** (`docs/engineering-principles.md` § 0). The change is in the sandbox that
  confines documents received from peers. No TLA+/Alloy model: the change has no state, no concurrency
  and no journey, so the § 0 trigger for a formal model does not fire. The executable specification is
  the decision table in § 5, tested row by row, with the mutants of § 8.3 and an independent review by
  another model family (Codex adversarial review once it is available again).
- **Scope:** Linux (`internal/sidecar`, `internal/typst`, `internal/app/preview.go`, the frontend
  strings). No system setting changes, no filtering of reports, no new package.

## 1. Evidence

`coredumpctl list --since 2026-11-10` shows 30 records, all with `COREFILE none`:

| Signal | Count | Process | Origin |
|---|---|---|---|
| SIGSYS | 14 | `sidecar.test` | `TestSeccompKillsABIMismatch/{arch,x32}`, two per run. This is the deliberate `SECCOMP_RET_KILL_PROCESS` on an audit-arch or x32 mismatch. |
| SIGXFSZ | 7 | `sidecar.test` | `TestInitPreservesFileSizeSignal` (helper `xfsz` resets the signal to `SIG_DFL` and raises it) |
| SIGXFSZ | 6 | `typst` | `TestCompileHostile/TYP-19` (PDF and SVG over `MaxOutput = 200`) |
| SIGSEGV | 3 | `typst` | `TestCompileHostile/TYP-16` (`range(2000000000)` under a 512 MiB `RLIMIT_AS`) |

For each record the journal holds the same three lines, for example for PID 2925376:
`systemd-coredump: Process … (typst) terminated abnormally with signal 25/XFSZ, processing...` →
`Resource limits disable core dumping for process …` → `… terminated abnormally without generating a coredump.`
Right after each one, `abrt-dump-journal-core` (unit `abrt-journal-core.service`) wakes up and tries to
file a problem. For these test binaries, which sit in temporary folders that are already deleted, it fails
("Échec de l'obtention de toutes les informations nécessaires de journald"). For an installed Inkwell,
whose executable path persists, nothing stops the save, and so nothing stops the notice.

## 2. Root cause (question 1)

The chain, verified on this machine (kernel 7.2.11, systemd 259, abrt 2.17.9) and in current sources:

1. **The process dies of a signal whose default action is "core".** SIGSYS, SIGXFSZ, SIGSEGV and
   SIGABRT are all in that class (`signal(7)`). Seccomp is not special: `SECCOMP_RET_KILL_PROCESS` just
   forces a SIGSYS core-class death (`seccomp(2)`). The other two signals come from the rlimits and from
   musl (point 4).
2. **`RLIMIT_CORE = 0` does not apply when `core_pattern` is a pipe.** Fedora sets
   `kernel.core_pattern=|/usr/lib/systemd/systemd-coredump %P %u %g %s %t %c %h %d %F`
   (`/usr/lib/sysctl.d/50-coredump.conf`). `core(5)` says: "The RLIMIT_CORE limit is not enforced for core
   dumps that are piped to a program." In `fs/coredump.c` (`coredump_pipe`) the only limit value the
   kernel looks at is **1**, its own sentinel for "never dump" (it puts that value on the helper itself to
   catch recursive crashes). It then logs `RLIMIT_CORE is set to 1, aborting core` (KERN_WARNING,
   rate-limited) and returns before it starts the helper. With 0, the kernel starts systemd-coredump as
   usual.
3. **`COREFILE none` is systemd-coredump's decision, not the kernel's.** It gets the limit as `%c`, sees 0,
   stores no core ("Resource limits disable core dumping"), and still writes the journal entry
   `MESSAGE_ID=fc2e22bc6ee647b6b90729ab34a250b1` with the executable, the command line (project paths
   included) and the signal. `coredumpctl` lists exactly these entries.
4. **The SIGSEGV is a memory limit hit, not a memory bug.** Typst is a static-pie musl binary. When an
   allocation fails, Rust's default `handle_alloc_error` prints `memory allocation of N bytes failed`
   and calls `abort()`. musl's `abort()` raises SIGABRT with `tkill`. Our seccomp filter denies `tkill`
   with EPERM, so `abort()` falls through to `a_crash()`, which on x86_64 is `hlt`, and the process dies
   of SIGSEGV. On arm64 musl's generic `a_crash` also ends in SIGSEGV. The engine already classifies this
   as `ErrMemoryLimit` from stderr (`internal/typst/engine.go::failure`).
5. **What shows the notice.** GNOME itself has no crash dialog. Fedora Workstation ships ABRT
   (`abrt-desktop`). `abrt-journal-core.service` follows the systemd-coredump journal entries and turns
   each one into a problem through `abrtd`. `abrt-applet` (`org.freedesktop.problems.applet`, autostarted
   from `/etc/xdg/autostart`) then shows the GNOME notification. Other crash interfaces read the same
   record: KDE's DrKonqi reads systemd-coredump, and Ubuntu's apport is itself the pipe helper. So the one
   upstream fix is that the kernel never hands our sandboxed process to a core helper. Hiding what the
   helper produces would not fix it.

So none of the three suspects is the cause on its own. The cause is their combination: a core-class
death, plus a pipe `core_pattern`, plus `RLIMIT_CORE = 0`, which a pipe ignores.

## 3. Constraints that shape the fix

- **The target cannot change its own dumpability.** `PR_SET_DUMPABLE` is reset on every `execve`
  (`fs/exec.c`, `begin_new_exec`: `task_exec_state_set_dumpable(TASK_DUMPABLE_OWNER)` unless the
  binary is unreadable or the credentials change; in those cases the value becomes `fs.suid_dumpable`,
  which is **2** on Fedora, and 2 still dumps through the pipe). Typst is static (no `LD_PRELOAD` hook),
  pinned and hash-checked (ADR 0024), and has no option for memory or core files (checked in the pinned
  0.15.1 binary).
- **Chromium does not face this problem because it never execs.** Its sandboxed processes are forked
  from a zygote and call `prctl(PR_SET_DUMPABLE, 0)`
  (`content/common/sandbox_linux/sandbox_debug_handling_linux.cc`). Without an exec the flag survives,
  so no kernel core and no systemd record exist. Violations go to `SECCOMP_RET_TRAP` →
  `CrashSIGSYS_Handler`, which crashes on purpose into Chromium's *own* reporter (Crashpad), and the page
  shows its in-app "Aw, Snap!". Firefox release builds answer most violations with ENOSYS and report them
  to the parent (`SandboxReporter`); only Nightly crashes (`MOZ_SANDBOX_CRASH_ON_ERROR`). minijail and
  nsjail kill by default (nsjail configs: `DEFAULT KILL_PROCESS`; minijail `-L` only on dev images), and
  on ChromeOS that is deliberate because `crash_reporter` collects those kills. Bubblewrap only loads the
  filter it is given.
- **What Inkwell differs on:** it collects no telemetry (`docs/ux-principles.md`, rule 10). A system
  crash record therefore has no fleet value to us. It only frightens the person and leaks the project
  paths into a root-readable journal.

## 4. Options considered

| # | Option | Verdict |
|---|---|---|
| a | `RLIMIT_CORE = {1,1}` in the target (kernel sentinel) | **Adopted** as the general fix (§ 5.0) |
| b | `PR_SET_DUMPABLE 0` before exec | Does not work: exec resets it (§ 3) |
| c | Execute-only typst binary → `suid_dumpable` | Does not work on Fedora (2 still dumps); also depends on a system setting |
| d | ptrace the target and turn the fatal signal into SIGKILL | Does not work for seccomp: a forced SIGSYS is `SA_IMMUTABLE`, so the tracer never sees it. It would also add a tracer and ptrace surface |
| e | Userland ELF loader shim (non-dumpable, no exec) | Rejected for W49: a new ELF loader would be attack surface in the trusted part. Kept as research for the socket-mode residual (§ 6) |
| f | cgroup v2 `memory.max` (OOM → SIGKILL, which never dumps) | Follow-up block, not W49 (§ 5.3, § 6) |
| g | `coredump.conf`, abrt `IgnoredPaths`, sysctl | Forbidden by the requirement (system settings, hides reports) |
| h | `SIGXFSZ` ignored in the target → EFBIG | **Adopted** (§ 5.1) |
| i | Seccomp: ERRNO instead of kill on ABI mismatch | Not adopted by default; **user decision D1** (§ 5.2) |

Option a is not "hiding a report". RLIMIT_CORE is the per-process POSIX control for "this process
must not dump". The sandbox already intended exactly that (`{RLIMIT_CORE, 0}` in `limitAndExec`), and 1
is the value the kernel itself uses to say "never" on pipe systems. Nothing changes system-wide. It is
also a security gain. A sandboxed process holds a *peer's* document in memory, and every hand-off to a
root core helper is a disclosure path and attack surface: the 2025 Qualys findings CVE-2025-4598
(systemd-coredump) and CVE-2025-5054 (apport) were races in exactly these helpers.

## 5. Design (question 2)

### 5.0 Common to every signal: the target can never reach a core helper

In `internal/sidecar/launcher_linux.go::limitAndExec`, change `{unix.RLIMIT_CORE, 0}` to
`{unix.RLIMIT_CORE, 1}`, with `Cur = Max = 1` so the target cannot raise it again. This keeps
the existing order: limits are prepared before `RLIMIT_AS`, `RLIMIT_AS` goes last with nothing allocated
after it, and seccomp is applied last before the raw `execve` (commit 2e812de). Rlimits survive `execve`.

Effect per kernel `core_pattern` mode:
- **File:** `1 < binfmt->min_coredump` (one page), so no file.
- **Pipe** (Fedora, Debian, Ubuntu/apport today): `coredump_pipe` stops before it starts the helper, so no
  record. The only trace is one rate-limited kernel line, `coredump: PID(typst): RLIMIT_CORE is set to 1,
  aborting core`, which matches none of abrt-oops' suspect strings (`abrt/src/lib/kernel.c`).
- **Socket** (`@`/`@@`, kernel ≥ 6.16/6.17, used by systemd ≥ 262 when `systemd-coredump-register.service`
  is enabled): the kernel does **not** check the limit. See § 6.

This also covers tinymist (same launcher) and every future sidecar.

### 5.1 SIGXFSZ: ignore it in the target, so the write fails with EFBIG and typst exits with status 1

- **Mechanism.** `execve(2)`: handlers that are being caught are reset to the default, but ignored
  signals stay ignored. In the launcher, Go's runtime has its own SIGXFSZ handler (`sigtab`: `_SigNotify`),
  which exec would reset to `SIG_DFL`. That is why typst dies today. `signal.Ignore(syscall.SIGXFSZ)`
  sets the kernel disposition to `SIG_IGN` and clears `handlingSig` (`runtime.sigignore`). Go's
  post-fork `clearSignalHandlers` "clears all signal handlers that are not ignored", so `SIG_IGN` also
  survives `os/exec`, and nothing else touches it before the raw `execve`. Rust's std only changes SIGPIPE
  at startup, and musl changes nothing.
- **Where.** In `limitAndExec`, among the preparations made before the allocation cut-off (next to
  `seccompProgram()` and the pointer slices), with a comment that cites `execve(2)`.
- **Resulting behaviour.** The kernel shortens the write that reaches the limit (`generic_write_check_limits`).
  The next write returns EFBIG, typst prints `error: failed to write PDF file (File too large (os error 27))`
  and exits with status 1. The output file stops at exactly the cap, which `directoryLimitHit` and
  `mount.limitHit` already treat as `OutputLimit` (`internal/sidecar/dir_linux.go`), so
  `ErrOutputTooLarge` is kept.
- **Security trade-off: none.** The guarantee is the byte cap, which the kernel still enforces. The
  signal was only the way the process was told. The watch-dir backstop, the tmpfs size and the timeout
  stay as they are. A program that loops on EFBIG is bounded by the timeout, and `TestRunDirectoryBound`
  already proves that such a program (`boundattack`) stays within the bound.
- **Docs to update:** `Spec.FileSizeLimit` (`run.go`: "A write past it ends the child with SIGXFSZ" →
  "fails with EFBIG"), and the comment of the SIGXFSZ branch in `engine.go::failure`. That branch stays
  for an `Output.Signal == 25` reported from elsewhere, and its unit test stays.

### 5.2 SIGSYS: keep the kill on ABI mismatch, with no core helper

Facts: our filter already answers every non-allowlisted *native* syscall with EPERM, a deliberate
"fail-continue" policy (`seccomp_linux.go`: "Only ABI confusion kills"). The kill fires only on a wrong
audit architecture or an x32 syscall number, which the pinned static x86_64/arm64 typst never makes.
**So in production a SIGSYS can only mean that someone tried to exploit typst.** No honest document,
however large or broken, can cause it. The 14 SIGSYS records all come from tests that cause it on purpose.

- `SECCOMP_RET_KILL_PROCESS` always dumps (`seccomp(2)`). `RET_KILL_THREAD` dumps only for the last
  thread and otherwise leaves the process half-alive, which is worse. `RET_TRAP` without a handler in
  typst also dumps. `RET_USER_NOTIF` plus a SIGKILL from our init would keep fail-stop without a dump,
  but filter precedence (ERRNO beats USER_NOTIF) would force two stacked filters and passing the
  listener fd before `sendmsg` is denied. That is complex, so it is rejected for W49.
- **Recommendation: keep `KILL_PROCESS`, plus § 5.0.** Containment, fail-stop and the evidence (the
  signal number reaches the engine through the report channel) all stay. Pipe and file modes leave no
  record.
- **The alternative (decision D1):** `RET_ERRNO(ENOSYS)` on ABI mismatch. Against an attacker who has
  read our public policy, containment is the same: the syscall never runs in either case, and Landlock,
  the namespaces, the rlimits and the timeout are unchanged. What is lost is fail-stop against a naive
  exploit and our only local proof that an attack happened. It is consistent with Firefox release
  builds. It would require an ADR, because `seccomp-policy.md`, `TestSeccompKillsABIMismatch` and
  `TestSeccompPolicyVM` freeze the kill.
- **Classification (new).** `failure()` maps `run.Signal == SIGSYS` (31 on Linux) to a new
  `typst.ErrBlocked` instead of the generic "stopped by a signal" (`FailInternal` today). A seccomp kill
  is a security event, not an internal error. `failureKinds` maps it to `FailBlocked = "blocked"`.

### 5.3 Memory: there is no clean exit with this typst, so the death must not be a dump

- Rust aborts on allocation failure by design (`std::alloc::handle_alloc_error`: "print a message to
  standard error and abort"). `-Z oom=panic` is unstable and is not used by the typst release build,
  and typst has no memory option. Under `RLIMIT_AS`, the pinned binary therefore always ends with a
  signal (SIGSEGV through musl, § 2.4). No sandbox-side setting changes that into an exit status.
- **Allowing self-directed `tkill`** (so that `abort()` really raises SIGABRT) would give a more
  accurate signal, but SIGABRT is also core-class, and `tkill(tid)` cannot be restricted to the caller in
  BPF (only the tgid of `tgkill` can). It would reopen cross-process signals in the fallback without
  namespaces. **Rejected.**
- **cgroup v2 `memory.max` + `memory.oom.group`.** The OOM killer sends SIGKILL, which never dumps, and
  `memory.events: oom_kill` names the cause exactly. It also accounts resident memory instead of the
  address-space reservations that `RLIMIT_AS` counts. But a single allocation larger than RAM plus swap
  still fails in `mmap` under the overcommit heuristic, which means an abort. It needs a delegated subtree
  (systemd user manager, `clone3(CLONE_INTO_CGROUP)` through `SysProcAttr.UseCgroupFD`) and a fallback,
  and macOS does not have it. **A follow-up block (proposed: M-cg), not W49.** *(Superseded by decision D2 and ADR 0042: the memory cgroup is part of block SB1.)* For W49, `RLIMIT_AS` stays
  the hard bound and § 5.0 removes the record.
- Classification is unchanged: stderr `memory allocation of … failed` → `ErrMemoryLimit`.

### 5.4 Decision table (the executable specification)

| # | Cause in the target | Death | Core helper started (pipe mode) | Engine error | Preview kind |
|---|---|---|---|---|---|
| R1 | write past `FileSizeLimit` | exit 1 (EFBIG), **no signal** | no | `ErrOutputTooLarge` | `tooLarge` |
| R2 | output dir or tmpfs bound | exit ≠ 0 or backstop SIGKILL | no | `ErrOutputTooLarge` | `tooLarge` |
| R3 | allocation past `RLIMIT_AS` | SIGSEGV (x86_64/arm64 musl) | **no** (RLIMIT_CORE 1) | `ErrMemoryLimit` | `memory` |
| R4 | ABI-mismatch syscall | SIGSYS (KILL_PROCESS) | **no** (RLIMIT_CORE 1) | `ErrBlocked` | `blocked` |
| R5 | denied native syscall | EPERM, typst continues | no | as typst reports | as today |
| R6 | timeout or cancel | group SIGKILL | no (SIGKILL never dumps) | `ErrTimeout` / ctx | `timeout` |
| R7 | any other core-class signal | that signal | **no** (RLIMIT_CORE 1) | "stopped by a signal" | `internal` |

## 6. Residuals

- **RS1: socket-mode `core_pattern`.** systemd ≥ 262 can switch the kernel to `@@/run/systemd/coredumpd/kernel`.
  In that mode the kernel ignores RLIMIT_CORE, and R3, R4 and R7 would again produce records, probably
  still journaled as "Resource limits disable core dumping" (not verified, because systemd 262 is not on
  this machine). Only R4, the exploit-only case, cannot be removed by our design; R3 is removed by M-cg
  for honest documents. Mitigations: the G4 gate check (§ 8.2) fails on any machine that has the mode, so
  we will see it the day it arrives. M-cg covers honest large documents. Research to follow: upstream,
  ask systemd-coredumpd to reply `COREDUMP_REJECT` without journaling when the limit is ≤ 1, or a kernel
  check of limit 1 in `coredump_socket`; and option e.
- **RS2: macOS.** The limits do not apply there (`Spec` docs), so W49 has no counterpart there.
  ReportCrash writes `.ips` files for crashing child processes without a dialog. To check when the macOS
  limits come.
- **RS3: the kernel log line.** One rate-limited KERN_WARNING per stop (§ 5.0). It is accepted: it names
  no project path, and abrt-oops ignores it. The G4 gate check also asserts that no abrt problem appears.

## 7. What the person sees (question 3)

No new component. The existing preview failure state in `frontend/src/preview/Preview.svelte` (warning
icon in the stale banner over the last good pages, or the empty-state message) shows `preview.fail.<kind>`.
Export reuses it through `Preview.currentPDF` → `failureOf`. UX rule 5 ("every message says what to do")
is not met today by `memory` and `tooLarge`. Vocabulary follows `docs/glossary.md`: "document",
"layout", never "sandbox", "signal", "seccomp" or "crash".

| Key | English | French |
|---|---|---|
| `preview.fail.memory` | Laying out the document needs more memory than Inkwell allows. Shorten it or split it into several files. | La mise en page demande plus de mémoire qu'Inkwell n'en permet. Raccourcissez le document ou découpez-le en plusieurs fichiers. |
| `preview.fail.tooLarge` | The result is too large for the preview. Shorten the document or split it into several files. | Le résultat est trop volumineux pour l'aperçu. Raccourcissez le document ou découpez-le en plusieurs fichiers. |
| `preview.fail.blocked` (new) | This document tried to do something Inkwell does not allow, so its layout was stopped. Your files are safe. Only open it if you trust where it comes from. | Ce document a tenté une action qu'Inkwell n'autorise pas : sa mise en page a été arrêtée. Vos fichiers ne risquent rien. Ne l'ouvrez que si vous avez confiance en sa provenance. |

`blocked` is added to `PreviewFailureKind` and its list in `frontend/src/preview/protocol.ts`, in both
i18n files, and in `TestFailureOf` (row "PRV-4x blocked"). The UX walkthrough (`ux-walkthrough`,
`ux-tester`) covers the three states in light and dark. The final wording is to be confirmed in a short
session with the user (R14). The local diagnostics event (W44) for R3, R4 and R7 is noted for the
diagnostics plan and is not part of W49.

## 8. Test plan (question 4)

### 8.1 New tests (red today for the stated reason)

- **T1 `TestTargetCoreLimitIsSentinel`** (`internal/sidecar`, Linux). Extend the `rlimits` helper of
  `TestRunLimitsHoldFromStartup` to also print `core=%d/%d` (getrlimit, which the Go runtime does not
  touch), and assert `1/1`. Red today: `0/0`. Holds on any `core_pattern` (CI included).
- **T2 `TestSandboxDeathStartsNoCoreHelper`** (`internal/sidecar`, Linux, kernel with
  `PIDFD_INFO_COREDUMP`). For the attacks `arch`, `x32` (SIGSYS), `xfsz` (SIGXFSZ forced to `SIG_DFL`) and
  a new `segv` (null write after `SIG_DFL`): start the launcher with `SysProcAttr.PidFD`. After the wait,
  call `ioctl(PIDFD_GET_INFO)` with `PIDFD_INFO_COREDUMP` (layout from `include/uapi/linux/pidfd.h`).
  Assert both that the **intended signal** killed it (the security guarantee stays) and that
  `coredump_mask & PIDFD_COREDUMPED == 0`. The kernel sets that bit in `umh_coredump_setup` exactly
  when it starts the pipe helper with `%F`, which is Fedora's pattern. Red today on this machine. Skip
  with an explicit message (not a silent pass) when the ioctl or the info field is missing, or the
  pattern is not a pipe with `%F`; T1 still guards the mechanism there.
- **T3 `TestTargetIgnoresOnlyFileSizeSignal`** (`internal/typst`, Linux, real typst). Run the TYP-14 loop
  document. While typst runs, read `SigIgn` in `/proc/<pid>/status` (PID found with `typstProcesses`), then
  cancel. Assert that the ignored set is exactly {SIGPIPE (set by Rust), SIGXFSZ}. Red today: SIGXFSZ is
  not ignored. This also catches a mutant that ignores the wrong signal, or SIGTERM.
- **T4 TYP-19 strengthened** (internal test in `package typst`, through `e.prepare` + `e.run`, so it gets
  the `sidecar.Output` without a new production hook). For the PDF and SVG cases over `MaxOutput = 200`:
  `Signal == 0`, `ExitCode == 1`, stderr contains `os error 27`, and `Compile` still returns
  `ErrOutputTooLarge`. Red today: `Signal == 25`.
- **T5 TYP-16 strengthened.** Still `ErrMemoryLimit`. Add: the run died of SIGSEGV or SIGABRT
  (documented as unavoidable, § 5.3), and, under the T2 conditions, no `PIDFD_COREDUMPED`. The pidfd
  of the typst process is taken from the namespace init, which already waits for it (`sandboxInit`): its
  report byte gains a bit "core helper started", read with the same ioctl before the reap. Red today.
- **T6 `TestFailure` rows** (unit): `Output{Signal: 31}` → `ErrBlocked`; `Signal: 11` together with
  stderr `memory allocation of 8 bytes failed` → `ErrMemoryLimit`; `Signal: 11` without it →
  "stopped by a signal". **`TestFailureOf`**: `ErrBlocked` → `FailBlocked`.

### 8.2 The gate `gates/W49.toml` (written and frozen before the code)

- G1: T1, T2, T3, T4, T5 and T6 by name, under `scripts/heavy.sh` with a `timeout`.
- G2: the existing hostile tests, unchanged in what they assert: `TestSeccompKillsABIMismatch` (still
  SIGSYS), `TestSeccompPolicyVM` (still `KILL_PROCESS` for x32 and arch), `TestInitPreservesFileSizeSignal`
  (still SIGXFSZ through the PID namespace, since the helper itself resets the signal to `SIG_DFL`),
  `TestRunFileSizeLimit`, `TestRunDirectoryBound`, and `TestCompileHostile` (TYP-14 to TYP-19).
- G3: `mise run lint`, `fe:check` (i18n keys, protocol kinds), `lint:env`.
- G4 (**the user-visible criterion**, Linux with journald): note the time, run G1 and G2, then
  `journalctl MESSAGE_ID=fc2e22bc6ee647b6b90729ab34a250b1 --since <t0> -o json`, keeping entries whose
  `COREDUMP_EXE` is `sidecar.test` or an Inkwell `typst`/`tinymist`. The required count is **0**, and
  `abrt-cli list --since <t0>` must show no new problem for them. Red today: about 10 per run. Where
  there is no systemd-coredump, the step says "not applicable" in its report; it never passes silently.

### 8.3 Mutants (manual, recorded in `gates/W49/SIMPLIFY.md`; gremlins on `internal/sidecar` if it runs in time)

| Mutant | Killed by |
|---|---|
| RLIMIT_CORE 1 → 0 | T1, T2, T5, G4 |
| RLIMIT_CORE entry removed (inherits unlimited) | T1 |
| `Cur: 1, Max: unlimited` (target could raise it again) | T1 (asserts Max) |
| `signal.Ignore` removed | T3, T4 |
| ignores SIGXCPU instead of SIGXFSZ | T3, T4 |
| `Ignore` placed after the exec preparation and skipped on a branch (for example `c.Directory == ""`) | T3 (default engine config) |
| `failure`: SIGSYS branch removed, or 31 → 30 | T6 |
| `failureKinds`: `ErrBlocked` row removed | `TestFailureOf` |
| seccomp ABI action KILL_PROCESS → ERRNO | `TestSeccompKillsABIMismatch`, `TestSeccompPolicyVM` (existing) |
| report byte: core bit always 0 | T2 as a cross-check on the direct path; T5 under the T2 conditions |

## 9. Blueprint for `go-dev` (in order, TDD)

1. Write `gates/W49.toml` (G1 to G4) and run each command: red for the reasons in § 8.1. Commit it, then freeze it.
2. T1 → change the RLIMIT_CORE value in `limitAndExec` (§ 5.0) → T1 and T2 green.
3. T3, T4 → `signal.Ignore(syscall.SIGXFSZ)` in `limitAndExec` before the allocation cut-off, plus the
   doc updates of § 5.1 → T3 and T4 green. Check that `TestInitPreservesFileSizeSignal` and
   `TestRunFileSizeLimit` stay green unchanged.
4. T5 → core bit in the init's report byte (`sandboxExitStatus`, `signalReport` decoding in
   `report_linux.go`), plus an `Output` field. That field is the only change to `Output`; name it
   `CoreHelper bool`, with a doc comment explaining that it is a test and diagnostics fact.
5. T6 → `ErrBlocked` in `internal/typst/errors.go`, the branch in `failure`, `FailBlocked` in
   `internal/app/preview.go`, and the frontend kind and strings of § 7.
6. Update `internal/sidecar/seccomp-policy.md` (paragraph "Violations and core dumps": why there is
   no core helper, and the citations) and `docs/exigences.md` W49 → state.
7. `/simplify` → `gates/W49/SIMPLIFY.md`, then the definition of done, steps 4 to 8.

## 10. Decisions for the user

- **D1 (security):** keep kill-on-ABI-mismatch (recommended: the event only happens during an attack,
  and on current systems it leaves no record), or switch to ENOSYS (no signal on any system, same
  containment against an informed attacker, but we lose fail-stop and our local evidence; needs an ADR).
- **D2 (scope):** accept RS1, so that a future socket-mode systemd can again report a memory-limit stop,
  and schedule M-cg (cgroup memory bound) as the follow-up block for that case. Or require M-cg inside W49.
- **D3 (wording):** the three messages of § 7, to be confirmed in a short session in front of the app.

## Sources

- Linux `fs/coredump.c` (`coredump_pipe`, `coredump_skip`, `umh_coredump_setup`), `fs/exec.c`
  (`begin_new_exec`, `would_dump`), `include/linux/coredump.h`, `include/uapi/linux/pidfd.h`, `fs/pidfs.c`:
  https://github.com/torvalds/linux (master, fetched 2026-11-11)
- core(5): https://man7.org/linux/man-pages/man5/core.5.html; seccomp(2):
  https://man7.org/linux/man-pages/man2/seccomp.2.html; execve(2):
  https://man7.org/linux/man-pages/man2/execve.2.html; PR_SET_DUMPABLE(2const):
  https://man.archlinux.org/man/PR_SET_DUMPABLE.2const.en.raw
- systemd-coredump(8) (MESSAGE_ID, socket mode, systemd 262):
  https://www.freedesktop.org/software/systemd/man/latest/systemd-coredump.html
- Kernel task_exec_state series (7.2): https://lkml.rescloud.iu.edu/hypermail/linux/kernel/2606.1/11377.html
- Go runtime `sigtab_linux_generic.go`, `signal_unix.go` (`clearSignalHandlers`, `sigignore`), `os/signal`
  docs: https://github.com/golang/go/tree/master/src/runtime
- musl `src/signal/raise.c`, `src/exit/abort.c`, `arch/x86_64/atomic_arch.h`: https://git.musl-libc.org/cgit/musl/
- Rust `handle_alloc_error`: https://doc.rust-lang.org/std/alloc/fn.handle_alloc_error.html
- abrt oops patterns: https://github.com/abrt/abrt/blob/master/src/lib/kernel.c
- Chromium: https://chromium.googlesource.com/chromium/src/+/main/sandbox/linux/README.md,
  https://chromium.googlesource.com/chromium/src/+/02ecb041e55718091f962666c02e313be7020fc5/sandbox/linux/seccomp-bpf-helpers/sigsys_handlers.cc,
  https://chromium.googlesource.com/chromium/src/+/0a356b3c/content/common/sandbox_linux/sandbox_debug_handling_linux.cc,
  https://chromium.googlesource.com/chromium/src/+show/master/docs/seccomp_sandbox_crash_dumping.md
- Firefox: https://searchfox.org/firefox-main/source/security/sandbox/linux/Sandbox.cpp,
  https://bugzilla.mozilla.org/show_bug.cgi?id=1995035
- minijail: https://google.github.io/minijail/minijail0.1.html,
  https://groups.google.com/a/chromium.org/g/chromium-os-dev/c/zYP4tlXQmRg/m/aMyd2l-SBAAJ;
  nsjail example: https://chromium.googlesource.com/infra/goma/server/+/refs/tags/v0.0.24/remoteexec/nsjail.cfg
- Local evidence: `coredumpctl list --since 2026-11-10`, `coredumpctl info 2925376`, `journalctl` around
  2026-11-11 11:19:35, `sysctl fs.suid_dumpable` (2), the pinned typst 0.15.1 binary (`file`, `strings`).

## Decisions of the user (2026-11-11)

- D1: keep SECCOMP_RET_KILL_PROCESS on a forbidden syscall.
- D2: the cgroup `memory.max` limit is in this block (W49), not a follow-up.
- D3: wording to confirm at the UX review of the block.

## Gate SB1 (`gates/SB1.toml`, block SB1 = requirement W49, step 1)

**Superseded on 2026-11-12 by block SB1b** (section "Gate SB1b" below, `gates/SB1b.toml`, ADR 0042 amendment):
Codex's adversarial review showed that the scope of ADR 0042 points 1-2 attaches a process already running, orders
the two memory limits wrongly and is not verified, and that this gate cannot catch a workload left outside its
cgroup. `gates/SB1.toml` stays frozen as history; no code was written against it.

Replaces the `gates/W49.toml` of § 8.2/§ 9 (same criteria, plus D2). Every Go criterion requires `--- PASS` for
each named test; a skip fails the gate. Each command was run once before freezing (the heavy `hostile-kept` and
`regression` ones were not, to leave the machine to the perf gates; their parts are green today and their new
parts are covered by the other criteria).

| Criterion | Proves | Red now because |
|---|---|---|
| `core-limit` | § 5.0, T1: target `RLIMIT_CORE` is `{1,1}` | `TestTargetCoreLimitIsSentinel` does not exist (and the launcher sets `{0,0}`) |
| `no-core-helper` | § 5.0, T2, D1: the intended signal still kills (SIGSYS arch/x32, SIGXFSZ, SIGSEGV) and `PIDFD_COREDUMPED` is never set | `TestSandboxDeathStartsNoCoreHelper` does not exist; with `{0,0}` the pipe helper starts |
| `sigxfsz` | § 5.1, T3, T4: SIGXFSZ ignored (only it and SIGPIPE), TYP-19 exits 1 with no signal and `ErrOutputTooLarge`; byte caps unchanged | the three new tests do not exist; the child dies of signal 25 today |
| `blocked` | § 5.2, T6: SIGSYS gives `ErrBlocked` and `FailBlocked` | the symbols do not exist; the test packages do not compile |
| `memory-cgroup` | D2, § 5.3: `memory.max`/`swap.max`/`oom.group` values, SIGKILL with `oom_kill >= 1`, cleanup, `ErrMemoryLimit`, fallback unchanged when cgroups are unavailable | the tests and the cgroup code do not exist |
| `no-coredump-record` | G4: zero systemd-coredump journal entries and zero abrt problems from the hostile runs | about ten records per run (30 since 2026-11-10) |
| `messages` | § 7, D3 (rules frozen, wording confirmed at UX review): every failure kind has EN and FR text, three say what to do, no jargon, same keys in both files | `frontend/src/preview/failureMessages.test.ts` does not exist |
| `hostile-kept` | the hostile tests keep their guarantees | green today; must stay green (also catches a dropped test) |
| `mutants` | § 8.3 plus cgroup mutants SB1a-SB1l each turn a named test red | the entries are missing from `scripts/netsim/mutants.toml` |
| `regression` | sidecar/typst/app tests, `fe:check`, `lint`, `lint:env`, the frozen gate L1 (introduced the sandbox) | green today |
| `simplify-report` | `/simplify` done | `gates/SB1/SIMPLIFY.md` absent |

### Feasibility findings (read from the code and the machine, 2026-11-12)

- cgroup v2 is mounted; `user@1000.service` is delegated with `cpu io memory pids`, but the app runs inside a
  leaf `app-*.scope` that holds processes, so by the no-internal-process rule the app cannot enable `memory` on its
  own cgroup and make a child there. `systemd-run --user --scope -p MemoryMax=… -p MemorySwapMax=0` does work (it
  makes a sibling scope), so the practical path is a transient scope for the target (D-Bus `StartTransientUnit`, or
  the `systemd-run` binary), with the fallback of § 5.3 when there is no user manager (containers, non-systemd, macOS).
  The implementer must pick one, justify it against "nothing to install" and the dependency rule, and add an ADR.
  **Decided (2026-11-12): D-Bus `StartTransientUnit`, recorded in
  [ADR 0042](../adr/0042-sandbox-memory-scope-and-no-core-helper.md)** (0039 to 0041 are reserved for plugins and
  AI in `docs/architecture/decisions.md`). It supersedes the "follow-up block (M-cg), not W49" of § 5.3 (decision D2).
- Open point for the implementer: with a cgroup bound, `RLIMIT_AS` must stay a bound (the gate requires it never be
  lower than the cgroup's) but cannot be what fires first on resident growth, or the death stays SIGSEGV.
- The cgroup tests skip (so the gate fails) on a machine without a delegated user cgroup; that is deliberate.

## Gate SB1b (`gates/SB1b.toml`, block SB1b = requirement W49, supersedes SB1)

Rigour: complete (security, concurrency, state). Decision: ADR 0042, section "Amendment 2026-11-12" (A1 to A7), which
wins over sections 5.3, 8 and "Gate SB1" above wherever they disagree. Sections 5.0, 5.1, 5.2, 5.4 (rows R1 to R7 of the
first table), 6 and 7 stay, with the scoped claims of A5 to A7. Model: `docs/protocol/tla/SandboxScope.tla` (agent
`formal-modeler`, README section `SandboxScope`) with the mutants and witnesses named in the gate.

### Review findings and their resolution

| # | Finding (Codex, 2026-11-12) | Resolution |
|---|---|---|
| 1 | the handshake comes after Go start, mounts and allocation, only when `MountBytes > 0`; `PIDs` migrates a running process; charges and earlier children stay outside | born inside: `clone3(CLONE_INTO_CGROUP)` into a per-run child of a delegated session scope (A1, rows S and R); bootstrap self-check of membership before any mount, fork or exec; accounting table in A1 |
| 2 | `RLIMIT_AS >= MemoryMax` orders nothing and weakens the old bound | `RLIMIT_AS` = `MemoryLimit` unchanged; `memory.max` = `ResidentLimit`; no order claimed; rows M1 and M2 tested both ways (A2) |
| 3 | job success is not confinement; fallback not equivalent | read-backs S5-S7, R3, R6; rows F1-F11 each tested; `Output.Containment`; fallback described as per-process only (A3) |
| 4 | auto-collection races the counters; `oom_kill` counts global kills | we own the run cgroup; counters read at R7 before R8; `oom` local versus `oom_kill` (rows M3-M5, A4) |
| 5 | `{1,1}` is bypassed by socket mode; a kernel line remains | guarantee scoped to pipe and file; socket mode fails the tests with "RS1"; sentinel line used as positive evidence (A5) |
| 6 | ignored SIGXFSZ does not make every target exit 1 | "exit 1" claimed only for the pinned typst; the cap and `OutputLimit` claimed for every end (rows X1-X5, A6) |
| 7 | `blocked` could carry raw text | fixed kind and text; canary tests on frame, log and rendered text (A7) |
| 8 | no "configured but outside" mutant; journal check ignores failures and sleeps | mutants SB1b-10 to SB1b-12, SB1b-17, SB1b-23; membership of target and descendant asserted before allocation; `scripts/sandbox_no_coredump.py` propagates every failure, waits on a collector barrier and attributes evidence to recorded deaths (rows J) |

### Session setup (rows S; any failure → fallback, row F named)

| Row | Step | Verified by reading | Failure row |
|---|---|---|---|
| S1 | start the holder (hidden mode of our binary, blocks on a pipe from the app, exits on EOF; never given a document) | the holder is alive (`pidfd`) | F11 |
| S2 | connect the user bus, `Subscribe`, match `JobRemoved` | connection established within 5 s | F1 |
| S3 | `StartTransientUnit(inkwell-sandbox-<pid>-<random>.scope, fail)`: `PIDs` holder, `Delegate` true, `Slice` = the app's slice, `CollectMode` inactive-or-failed, `MemoryMax`/`MemorySwapMax` inherited (A3) | the call returns a job path | F2 |
| S4 | await `JobRemoved` for that job | result is `done` before 5 s | F3 |
| S5 | find the scope cgroup | `/proc/<holder>/cgroup` = `<slice>/<name>.scope`; directory owned by our UID; `cgroup.controllers` lists `memory` | F4, F5 |
| S6 | `mkdir holder/`, write the holder PID to `holder/cgroup.procs` | `/proc/<holder>/cgroup` ends in `/holder`; the scope's `cgroup.procs` is empty | F6 |
| S7 | write `+memory` to the scope's `cgroup.subtree_control` | read back lists `memory` | F6 |
| S8 | inherited bound | scope `memory.max`/`memory.swap.max` equal the app cgroup's values (or `max`) | F7 |
| S9 | placement is permitted | the app cgroup and the scope share an ancestor inside `user@<uid>.service` | F10 |

### Per run (rows R)

| Row | Step | Rule |
|---|---|---|
| R1 | `mkdir <scope>/run-<n>` | a failure is F8 for this run |
| R2 | write `memory.max` = `ResidentLimit`, `memory.swap.max` = 0, `memory.oom.group` = 1 | before `Start` |
| R3 | read the three back | `memory.max` = `ResidentLimit` rounded down to the page size, the others exact; a mismatch is F9 |
| R4 | open the directory `O_PATH\|O_DIRECTORY\|O_CLOEXEC`; `SysProcAttr.UseCgroupFD`; expected path in `sandboxConfig` | every Linux launch path (`withLimits`, `directoryMount`, `directoryLimits`); never the probes |
| R5 | `cmd.Start` | EACCES, EBUSY, EOPNOTSUPP, ENOSYS: nothing ran, so remove the cgroup, mark the session unavailable (F10) and retry once in the fallback |
| R6 | first statement of `sandboxExec`: `/proc/self/cgroup` equals the expected path | mismatch: exit with the dedicated status, `Run` returns an error (refused, not fallback) |
| R7 | after `Wait`: read `memory.events.local` and `memory.events` | a failed read gives `unknown`, never zero |
| R8 | then `cgroup.kill`, wait for `populated 0` (bounded), `rmdir` | never before R7 |

### Memory outcomes (rows M)

| Row | Cause | Death | Counters | `MemoryCause` | Engine error | Preview kind |
|---|---|---|---|---|---|---|
| M1 | aggregate charge reaches `ResidentLimit` | SIGKILL of the whole run (`oom.group`) | local `oom >= 1`, `oom_kill >= 1` | `local-limit` | `ErrMemoryLimit` | `memory` |
| M2 | a mapping past `RLIMIT_AS` | SIGSEGV (musl after Rust's abort) or a program's own exit on ENOMEM; no core helper | `oom = 0` | `none` | `ErrMemoryLimit` (stderr "memory allocation of") | `memory` |
| M3 | global or ancestor OOM picks the target | SIGKILL | `oom_kill >= 1`, local `oom = 0` | `victim` | "stopped" (fixed text) | `internal` |
| M4 | counters unreadable | any | read error | `unknown` | by stderr only, else "stopped" | as mapped |
| M5 | fallback run (no cgroup) | as M2 | none | `none` | as M2 | `memory` |

### Fallback (rows F; each keeps rlimits, seccomp, Landlock, namespaces where available, byte caps, timeout; `Output.Containment` = `rlimit`)

| Row | Condition | Injected in `TestCgroupFallbackAtEveryStep` by |
|---|---|---|
| F1 | no session bus or no user manager | a bus address that does not exist (real) |
| F2 | `StartTransientUnit` refused (unknown property on an old systemd, `InvalidArgs`, `UnitExists`) | fake manager |
| F3 | job result not `done`, or no `JobRemoved` within 5 s | fake manager |
| F4 | holder not in the scope after `done` | fake manager reporting `done` without moving it |
| F5 | `memory` not in the scope's `cgroup.controllers` | cgroupfs seam |
| F6 | holder move or `subtree_control` write refused | cgroupfs seam (EACCES, EBUSY) |
| F7 | inherited bound not in place | cgroupfs seam |
| F8 | `mkdir` or a limit write refused | cgroupfs seam |
| F9 | read-back mismatch (a write silently ignored) | cgroupfs seam |
| F10 | placement refused at `clone3`, or the app outside the user manager | errno seam on `Start`; a cgroup path outside `user@<uid>.service` |
| F11 | holder dead or never started | kill it between runs |

A setup failure is retried after a backoff (1 s doubling to 30 s), not at every launch; the app logs the reason once.

### File size (rows X, A6)

| Row | Target behaviour (sidecar helper attacks) | Required |
|---|---|---|
| X1 | write straddling the cap | short count, file exactly at the cap, next write EFBIG, `Signal = 0` |
| X2 | aborts or exits non-zero on EFBIG | `OutputLimit`, file at the cap |
| X3 | resets SIGXFSZ to `SIG_DFL`, then writes past the cap | dies of SIGXFSZ, `OutputLimit`, no core helper, cap kept |
| X4 | loops on EFBIG | ended by the timeout, `OutputLimit`, cap kept |
| X5 | pinned typst, TYP-19 PDF and SVG | exit 1, `Signal = 0`, stderr `os error 27`, `ErrOutputTooLarge` |

### Tests named by the gate (each red today because it does not exist)

- `internal/sidecar`: `TestTargetCoreLimitIsSentinel`, `TestCorePatternGuaranteeScope` (parse `|…`, a file pattern, `@…`,
  `@@…`; only pipe and file are guaranteed), `TestSandboxDeathStartsNoCoreHelper` (fails with "RS1" in socket mode),
  `TestFileSizeCapShortWriteThenEFBIG` (X1), `TestFileSizeCapAbortOnEFBIGIsOutputLimit` (X2),
  `TestFileSizeCapDispositionResetIsOutputLimit` (X3), `TestFileSizeCapEFBIGLoopEndsAtTimeout` (X4),
  `TestCgroupSessionScopeDelegated` (S1-S9 on the real user manager), `TestCgroupRunLimitsVerifiedBeforeStart` (R2-R3),
  `TestCgroupWorkloadAndDescendantsInside` (a helper target forks a child, both print their PIDs and wait on a pipe
  before allocating; while they wait, both PIDs are in `run-<n>/cgroup.procs` and neither is in the app's cgroup),
  `TestBootstrapRefusesUnexpectedCgroup` (R6, expected path altered through a seam), `TestCgroupSessionSharesSliceBound`
  and `TestCgroupSessionCopiesLeafBound` (rows A1-A2), `TestCgroupRemovedAfterRun` (R8, no
  process left), `TestCgroupFallbackAtEveryStep` (F1-F11, one subtest each; each run still confined and reports
  `rlimit`), `TestFallbackReportsRlimitOnly`, `TestBothMemoryBudgetsHold` (with a cgroup the target's `RLIMIT_AS` is
  exactly `MemoryLimit` and `memory.max` is `ResidentLimit`), `TestResidentBudgetKillsWithSIGKILL` (M1, `ResidentLimit`
  below the address budget), `TestAddressBudgetFailsAllocationFirst` (M2, `ResidentLimit` above it),
  `TestMemoryCauseFromEvents` (M1-M4 from counter texts and read errors), `TestCgroupCountersReadBeforeCleanup` (R7
  before R8, order recorded through the cgroupfs seam).
- `internal/typst`: `TestTargetIgnoresOnlyFileSizeSignal`, `TestPinnedTypstOutputCapExitsOne` (X5),
  `TestFailureBlocked`, `TestBlockedErrorCarriesNoDetail`, `TestHostileCgroupMemoryIsMemoryLimit` (M1 with real typst),
  `TestHostileMemoryDeathStartsNoCoreHelper` (M1 and M2), `TestFailureMemoryCause` (M3 and M4 are not `ErrMemoryLimit`).
- `internal/app`: `TestFailureOfBlocked`, `TestBlockedFailureLeaksNothing` (a canary in stderr, the project path and the
  document reaches neither the preview frame, exactly `{"kind":"blocked"}`, nor the captured log).
- `frontend/src/preview/failureMessages.test.ts`: the four SB1 message rules, renamed "SB1b …", and "SB1b a failure shows
  only the fixed message of its kind".
- `scripts/sandbox_no_coredump.py` (G4, rows J): see the re-review section below.

### Red reasons recorded at freeze (2026-11-12)

Each criterion of `gates/SB1b.toml` was run once before the freeze; its comment says why it is red (or, for
`hostile-kept` and `regression`, that it is green and must stay so).

### Re-review (Codex, 2026-11-12, "fix first"): additions, which win over the tables above

ADR 0042, section A8. Mandatory subtests are named `Test…/<row>`; every Go criterion fails on any `--- SKIP` or
`--- FAIL` line at any depth (a parent can pass while a subtest skips).

**Memory cause from a timeline (rows M6-M9; M1 now also needs the timeline conditions).** From R4 to R7 the app watches
the run cgroup's `memory.events.local` (inotify) and keeps each read with its monotonic time; it snapshots
`/proc/vmstat` `oom_kill` and the local `oom` of the session scope and each ancestor up to `user@<uid>.service` at R4 and R7.

| Row | Timeline | `MemoryCause` |
|---|---|---|
| M6 | local `oom` rises; the first `oom_kill` rise comes more than W = 1 s later | `victim` |
| M7 | as M1, but the `/proc/vmstat` `oom_kill` delta exceeds the run's local `oom_kill` delta | `victim` |
| M8 | as M1, but an ancestor's local `oom` rose (an ancestor limit) | `victim` |
| M9 | inotify overflow, or a failed read during the watch | `unknown` |

`TestMemoryCauseFromEvents` has the subtests `M1`, `M2`, `M3`, `M4`, `M6`, `M7`, `M8`, `M9` (scripted timelines through
the cgroupfs seam); M1 includes the two-read case (oom, then the kill within W).

**Aggregate bound (rows A).**

| Row | Where the limit is set | Claim | Test |
|---|---|---|---|
| A1 | the app's slice or above | app plus sandbox together never exceed it (kernel, shared ancestor) | `TestCgroupSessionSharesSliceBound`: a child "app" (hidden test mode) in a scope under a fresh limited transient slice holds half the limit, then its sandbox workload grows past the rest: the slice's local `oom` rises and the workload dies (`victim`, M8) |
| A2 | the app's own leaf cgroup | copied to the session scope: the sandbox alone never exceeds it; the sum may reach twice it (RS5) | `TestCgroupSessionCopiesLeafBound`: the scope's `memory.max`/`swap.max` equal the leaf's; a workload past it is killed by the scope's limit |

**Holder lifetime (rows H).**

| Row | Event | Required | Test |
|---|---|---|---|
| H1 | holder dead before R1 (pidfd readable) | session invalidated; S1-S9 for this run | `TestHolderDeathBetweenVerifyAndStart/before-run` |
| H2 | holder killed between R3 and R5, scope empties | ENOENT or ENODEV at R1/R5 → retried once in the fallback, `rlimit`; next run on a new session | `TestHolderDeathBetweenVerifyAndStart/after-verify` |
| H3 | holder killed while other runs keep the scope alive | each run is either in a live verified cgroup or in the fallback; none in the app's cgroup; the stale session is dropped when its last run ends | `TestConcurrentRunsDuringHolderDeath` (8 runs) |
| H4 | the app is SIGKILLed during a run whose target has a forked descendant | within 5 s no process remains in any `run-*`, the scope unit is gone | `TestAppSIGKILLLeavesNoWorkload` (child "app" in hidden test mode) |
| H5 | holder sees EOF | `cgroup.kill` on every `run-*`, wait `populated 0` (5 s), `rmdir`, exit | `TestAppSIGKILLLeavesNoWorkload`, and H4 |

**Descriptors (rows D).**

| Row | Rule | Test |
|---|---|---|
| D1 | the run cgroup fd is `O_CLOEXEC` and closed right after `Start` on every path (success, placement error, retry, refused bootstrap, fallback, new session) | `TestCgroupDescriptorClosedOnEveryPath`: the app's `/proc/self/fd` count and set are unchanged after each path |
| D2 | both holder-pipe ends come from `os.Pipe` (CLOEXEC, atomic); the read end is only the holder's `ExtraFiles[0]` and is closed in the app after the holder starts; the write end is in no launch's `ExtraFiles` | `TestSandboxInheritsNoSessionDescriptor`: a target and its descendant list `/proc/self/fd`; no entry has the inode of either holder-pipe end, nor is a cgroup directory |
| D3 | a target inherits exactly 0, 1, 2 and nothing else of ours | same test |

**G4 evidence (rows J), `scripts/sandbox_no_coredump.py`.**

| Row | Rule |
|---|---|
| J1 | every hostile test that causes a deliberate sandbox death appends one JSON line to `$INKWELL_SANDBOX_DEATHS`: `test`, host `pid`, `nspid` (from `NSpid` of `/proc/<pid>/status`, read before the death), `comm`, `signal`, `coredumped` (the PIDFD bit); an empty manifest or a `coredumped` other than `false` fails (the variable goes in `.env.reference`) |
| J2 | barrier: no `systemd-coredump@` unit active, activating or deactivating and no `systemd-coredum` process, on two polls 1 s apart (bounded 60 s); then a marker sent by `logger --journald` (native datagram socket, the one systemd-coredump writes to) is visible (bounded 60 s) |
| J3 | zero records with `COREDUMP_PID` in the manifest or `COREDUMP_EXE` naming sidecar.test, typst or tinymist; zero abrt lines naming them |
| J4 | at least one kernel line `coredump: N(comm): RLIMIT_CORE is set to 1, aborting core` whose N and comm are the innermost namespace PID and command name of a recorded death |
| J5 | every named test and mandatory subtest prints `--- PASS`, no `--- SKIP` or `--- FAIL` anywhere; any failed or refused read fails; socket mode fails with RS1 |
| J6 | `--self-test` (green before the code): scripted journals with a delayed collector, unattributed records and kernel lines, a refused journal, an empty manifest, a skipped subtest |

Mutants added: SB1b-25 the W window ignored (any earlier `oom` makes `local-limit`; M6); SB1b-26 `/proc/vmstat` check
removed (M7); SB1b-27 ancestor check removed (M8); SB1b-28 session scope not placed in the app's slice (A1); SB1b-29 the
holder ignores EOF (H4); SB1b-30 ENOENT/ENODEV at placement returns an error instead of the fallback (H2); SB1b-31 the
cgroup fd is not closed after `Start` (D1); SB1b-32 the holder pipe's write end is passed to a launch (D2). SB1b-22 now
reads "leaf bound not copied" (A2). TLA+ mutants added: `MixedHistoryAsLimit`, `HolderDeathEscapes`,
`OrphanAfterAppKill`; witnesses `WitnessMixedHistory`, `WitnessHolderDeath`.
