---
status: accepted
date: 2026-11-12
amends: the memory bound of ADR 0024 (sidecars) and ADR 0033 (sidecar directory bound): RLIMIT_AS becomes the fallback, a transient systemd user scope the primary bound
decision-makers: [Claude, under the project's autonomy rule (docs/autonomous-decisions.md, row SB1, 2026-11-12); user decisions D1 and D2 of 2026-11-11]
---

# The sandboxed sidecar gets its memory limit from a transient systemd user scope, and never reaches a crash helper

Plan: `docs/plans/2026-11-11-sandbox-crash-reports.md` (sections 2, 5.0 to 5.4, 6, "Gate SB1" and its feasibility
findings). Gate: `gates/SB1.toml`. Requirement: W49.

## Context and problem

The sandbox that confines `typst` and `tinymist` (`internal/sidecar`, `launcher_linux.go`) refuses a hostile
document by killing the target: SIGSYS from the seccomp ABI check, SIGXFSZ past `RLIMIT_FSIZE`, SIGSEGV when musl
fails an allocation under `RLIMIT_AS`. All three are core-class signals. On Fedora, Debian and Ubuntu
`kernel.core_pattern` is a pipe (`|/usr/lib/systemd/systemd-coredump …` on the development machine), and core(5)
states that "RLIMIT_CORE will be ignored if the system is configured to pipe core dumps to a program": the
launcher's `RLIMIT_CORE {0,0}` stops nothing. Each refusal therefore starts `systemd-coredump`, writes a journal
record and, on GNOME, shows a crash alert for a sandbox that worked as designed (about ten records per test run,
30 since 2026-11-10; plan section 1).

Two further problems sit beside it. `RLIMIT_AS` counts address-space reservations, not memory in use, and a Rust
binary aborts on allocation failure by design, so the memory bound always ends in a signal (plan 5.3). And the user
decided (D2) that a real memory bound belongs in this block: cgroup v2 `memory.max`, whose OOM kill is SIGKILL
(never a core) and is named exactly by `memory.events: oom_kill`.

The app cannot create that cgroup itself. On the development machine `user@1000.service` is delegated `cpu io
memory pids`, but the app runs in a leaf `app-*.scope` that holds processes, and cgroup v2 says that "non-root
cgroups can distribute domain resources to their children only when they don't have any processes of their own"
(the no-internal-process constraint). Enabling `memory` in the app's own cgroup and making a child there is refused.
systemd also forbids writing outside a delegated subtree: "Managers should refrain from making any changes to the
cgroup tree outside of the subtrees" of units they created with `Delegate=`. A sibling scope created by the user
manager is the supported route; `systemd-run --user --scope -p MemoryMax=… -p MemorySwapMax=0` was measured to work.

## Decision drivers

- A refusal by the sandbox is never reported as a crash by the desktop (W49, G4 of the plan).
- The security guarantees are unchanged: fail-stop on ABI confusion (D1), the byte cap, Landlock, namespaces,
  timeout; the memory bound is never weaker than today, wherever the app runs.
- Memory is bounded by what the target uses, and the death is classified exactly (`ErrMemoryLimit`).
- "Nothing to install": no new binary on `PATH`, no parsing of a tool's text output; a pinned dependency only if
  it is already in the module graph (ADR 0008).
- A machine without a user manager (container, non-systemd Linux, macOS) keeps today's behaviour, never an error.

## Considered options

Memory bound:

- A: `RLIMIT_AS` only (today).
- B: the app makes a child cgroup of its own cgroup and starts the target in it (`clone3(CLONE_INTO_CGROUP)`).
- C: run the `systemd-run --user --scope` binary.
- D: create a transient scope over D-Bus with `org.freedesktop.systemd1.Manager.StartTransientUnit` on the user
  manager, through `github.com/godbus/dbus/v5`, with `RLIMIT_AS` kept as the fallback and as an outer bound.
- E: a cgroup library (`containerd/cgroups`, `coreos/go-systemd/dbus`).

Crash record:

- F: `RLIMIT_CORE {0,0}` (today), G: `PR_SET_DUMPABLE 0`, H: `RLIMIT_CORE {1,1}`, I: `SECCOMP_RET_ERRNO` or
  `RET_USER_NOTIF` instead of `RET_KILL_PROCESS` on ABI mismatch.

## Decision

Options retained: **D** for memory and **H** for the record, with SIGXFSZ ignored in the target and
`SECCOMP_RET_KILL_PROCESS` kept and classified `blocked`.

1. **Transient scope over D-Bus (Linux, primary bound).** Before a document byte reaches the target, the launcher
   asks the user manager (`org.freedesktop.systemd1` on the session bus) for
   `StartTransientUnit(name, "fail", properties, [])` with a unique name `inkwell-sidecar-<random>.scope` and the
   properties `PIDs` = the sandbox's outer process, `MemoryMax` = `Spec.MemoryLimit`, `MemorySwapMax` = 0,
   `OOMPolicy` = `kill` and `CollectMode` = `inactive-or-failed`, then waits for the `JobRemoved` signal of the
   returned job with result `done`. The target is held at the launcher's existing ready/proceed handshake
   (`mountOutput`, fd 3 and 4) until the scope is in place; every process it forks inherits the cgroup.
   - `MemoryMax=` controls `memory.max`: "If memory usage cannot be contained under the limit, out-of-memory killer
     is invoked inside the unit" (systemd.resource-control). The kill is SIGKILL, which never dumps.
   - `MemorySwapMax=0` sets `memory.swap.max` to 0, so "anonymous memory of the cgroup will not be swapped out"
     (cgroup-v2) and the limit is a limit on RAM, not on RAM plus swap.
   - `OOMPolicy=kill`: "the kernel is instructed to kill all remaining processes of the unit too … by setting the
     `memory.oom.group` attribute to 1" (systemd.scope; scope property since systemd 253). Init and target die
     together; no half-alive sandbox.
   - `CollectMode=inactive-or-failed`: an OOM-killed scope ends `failed`, and with the default `inactive` "it is not
     unloaded if it is in the failed state" (systemd.unit); this mode removes the scope and its cgroup after every
     run.
   - The engine reads `memory.events` of the scope's `ControlGroup` before release: `oom_kill >= 1` ("the number of
     processes belonging to this cgroup killed by any kind of OOM killer", cgroup-v2) maps to `ErrMemoryLimit`.
2. **Fallback.** No session bus, no user manager, a refused or failed job, macOS: the run goes on exactly as today
   (`RLIMIT_AS`), never an error and never a weaker bound. With a scope, `RLIMIT_AS` is still set, at or above
   `MemoryMax`, so the cgroup fires first on resident growth and `RLIMIT_AS` still bounds a single huge reservation.
3. **`RLIMIT_CORE {1,1}` in the target.** In `coredump_pipe()` the kernel checks `cprm->limit == 1` and stops with
   "RLIMIT_CORE is set to 1, aborting core" before it starts the helper (fs/coredump.c). The value is the sentinel
   the kernel itself puts on the helper (`umh_coredump_setup()` sets `RLIMIT_CORE` to `{1, 1}`) to catch recursive
   crashes. In file mode 1 byte is below `min_coredump`, so no file. `Max = 1` stops the target raising it. This
   covers every core-class signal of every sidecar, so no systemd-coredump record and no GNOME or abrt alert.
4. **SIGXFSZ ignored in the target.** `signal.Ignore(syscall.SIGXFSZ)` before the raw `execve`: ignored
   dispositions survive exec (execve(2)), caught ones are reset to default, which is why typst dies today. A write
   past `FileSizeLimit` then fails with EFBIG, typst exits 1 ("os error 27"), the file stays at the cap and
   `ErrOutputTooLarge` is kept. The byte cap is the guarantee; the signal was only the notice.
5. **SIGSYS stays a kill, classified `blocked`.** `SECCOMP_RET_KILL_PROCESS` on ABI mismatch is kept (D1): it can only
   be an exploit attempt, and fail-stop plus the evidence (the signal number reaches the engine) are worth keeping.
   `failure()` maps signal 31 to the new `typst.ErrBlocked`, and the preview maps it to the kind `blocked`. The
   record disappears by point 3, not by weakening the filter.

### Why not the other options

- **A:** the death stays SIGSEGV (a core-class signal) and the bound counts reservations, not use; it remains only
  as the fallback.
- **B:** forbidden by the no-internal-process constraint in a leaf scope, and writing outside our delegated subtree
  breaks the systemd contract. Re-scoping the whole app with `Delegate=yes` would mean relaunching it under a new
  unit at start-up, for every launcher (desktop file, terminal, `.deb`/`.rpm`).
- **C:** a binary looked up on `PATH`, flags and text output parsed per systemd version, one more process per
  compile, and failures that surface as exit codes instead of typed D-Bus errors.
- **E:** a new dependency tree for one method call; `godbus/dbus/v5` is already in `go.mod` (indirect, through
  Wails), so D adds no module, only makes it direct.
- **F:** ignored by pipe mode (core(5)). **G:** `PR_SET_DUMPABLE` is reset at `execve` for a non-setuid binary
  (`begin_new_exec`, `would_dump`), so it cannot be set for the target from the launcher.
- **I:** `RET_ERRNO` loses fail-stop and the local evidence and would require amending `seccomp-policy.md`;
  `RET_USER_NOTIF` needs two stacked filters and passing a listener fd that our policy denies (plan 5.2). Rejected
  by D1.

### Consequences

- Good: zero crash records and alerts from sandbox refusals in pipe and file mode; the memory death is SIGKILL with
  an exact cause; the bound follows resident memory; no binary on `PATH`; unchanged behaviour where systemd is absent.
- Less good: one D-Bus round trip and one job wait per run (to be measured against the compile budget, not
  assumed); a third Linux code path (scope, fallback, macOS) that the tests must each exercise; `godbus/dbus/v5`
  becomes a direct dependency, pinned under ADR 0008.
- Residual: socket mode (`@`/`@@` in `core_pattern`, kernel 6.16+, systemd-coredump registering it from systemd 262)
  does not check the `RLIMIT_CORE == 1` sentinel (`coredump_socket()` sets the limit to infinity). The development
  machine runs systemd 259 with a pipe. Tracked in plan section 6; reopen this ADR when a supported distribution
  ships socket mode by default.
- Residual: a single allocation larger than RAM plus swap still fails in `mmap` and aborts (SIGSEGV through musl);
  point 3 removes its record.

### Confirmation

Gate `gates/SB1.toml`: `core-limit` (point 3), `no-core-helper` (points 3 and 5: the intended signal still kills and
`PIDFD_COREDUMPED` is never set), `sigxfsz` (point 4), `blocked` (point 5), `memory-cgroup` (points 1 and 2:
`memory.max`, `memory.swap.max = 0`, `memory.oom.group = 1`, SIGKILL with `oom_kill >= 1`, removal after the run,
fallback unchanged), `no-coredump-record` (no systemd-coredump or abrt record from any hostile run),
`hostile-kept` and `regression` (guarantees and the L1 gate unchanged), `mutants` SB1a to SB1l (each decision point
reverted turns a named test red).

## More information

- core(5): https://man7.org/linux/man-pages/man5/core.5.html
- `kernel.core_pattern`, `core_pipe_limit`: https://www.kernel.org/doc/html/latest/admin-guide/sysctl/kernel.html
- Linux `fs/coredump.c` (`coredump_pipe`, `umh_coredump_setup`, `coredump_socket`):
  https://github.com/torvalds/linux/blob/master/fs/coredump.c
- execve(2): https://man7.org/linux/man-pages/man2/execve.2.html; seccomp(2):
  https://man7.org/linux/man-pages/man2/seccomp.2.html
- cgroup v2 (no-internal-process constraint, `memory.max`, `memory.swap.max`, `memory.oom.group`, `memory.events`):
  https://docs.kernel.org/admin-guide/cgroup-v2.html
- systemd.resource-control(5) (`MemoryMax=`, `MemorySwapMax=`, delegation):
  https://www.freedesktop.org/software/systemd/man/latest/systemd.resource-control.html
- systemd.scope(5) (`OOMPolicy=`): https://www.freedesktop.org/software/systemd/man/latest/systemd.scope.html
- systemd.unit(5) (`CollectMode=`): https://www.freedesktop.org/software/systemd/man/latest/systemd.unit.html
- org.freedesktop.systemd1(5) (`StartTransientUnit`, `JobRemoved`, Scope `ControlGroup`, `OOMPolicy` since 253):
  https://www.freedesktop.org/software/systemd/man/latest/org.freedesktop.systemd1.html
- Control group interfaces (scopes only through `StartTransientUnit`, `PIDs` at construction, delegation rules):
  https://systemd.io/CONTROL_GROUP_INTERFACE/
- systemd-coredump(8): https://www.freedesktop.org/software/systemd/man/latest/systemd-coredump.html
- godbus: https://github.com/godbus/dbus

All fetched and checked on 2026-11-12. Measured on the development machine (Fedora 44, kernel 7.2.11, systemd 259):
pipe `core_pattern` with `%F`, `user@1000.service` delegating `cpu io memory pids`.
