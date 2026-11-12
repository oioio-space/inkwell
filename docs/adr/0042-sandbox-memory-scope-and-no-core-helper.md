---
status: accepted
date: 2026-11-12
amends: the memory bound of ADR 0024 (sidecars) and ADR 0033 (sidecar directory bound): RLIMIT_AS becomes the fallback, a transient systemd user scope the primary bound
decision-makers: [Claude, under the project's autonomy rule (docs/autonomous-decisions.md, rows SB1 and SB1 → SB1b, 2026-11-12); user decisions D1 and D2 of 2026-11-11]
amended: 2026-11-12, block SB1b (section "Amendment 2026-11-12"; supersedes decision points 1 and 2 and the Confirmation; gate gates/SB1b.toml)
---

# The sandboxed sidecar gets its memory limit from a transient systemd user scope, and never reaches a crash helper

Plan: `docs/plans/2026-11-11-sandbox-crash-reports.md` (sections 2, 5.0 to 5.4, 6, "Gate SB1" and its feasibility
findings). Gate: `gates/SB1.toml`, superseded by `gates/SB1b.toml` (see "Amendment 2026-11-12" at the end, which
wins wherever it disagrees with the text above). Requirement: W49.

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

## Amendment 2026-11-12 (block SB1b)

An adversarial review (Codex, verdict "amend ADR first") found that decision points 1 and 2 do not deliver what they
claim, and that `gates/SB1.toml` would pass an implementation that leaves the workload outside its cgroup. SB1 was
stopped before any code. `gates/SB1.toml` stays frozen as history; **`gates/SB1b.toml` supersedes it**, and this
section supersedes decision points 1 and 2, the "Consequences" lines that depend on them, and the "Confirmation".
Points 3 to 5 stay, with the narrower claims of A5 to A7. Plan: section "Gate SB1b" of
`docs/plans/2026-11-11-sandbox-crash-reports.md` (decision tables S, R, M, F and the test list).

### What was wrong (read in the code and the kernel documentation)

- **Placement.** Point 1 hands `PIDs` of a running process to `StartTransientUnit`, holding the target at the
  `mountOutput` ready/proceed handshake. That handshake exists only when `MountBytes > 0`
  (`launcher_linux.go::sandboxExec`); tinymist (`internal/language`, no `WatchDir`) and the directory fallback without
  namespaces never reach it. When it does run, the outer launcher has already started its Go runtime, made its mounts
  and allocated. systemd then *migrates* that process, and cgroup v2 says that "migrating a process to a different
  cgroup doesn't move the memory usages that it instantiated while in the previous cgroup"; a process forked before
  the move stays where it was. A job result `done` says that systemd accepted the unit, not that our workload is in it.
- **Budgets.** Point 2 sets `RLIMIT_AS` "at or above `MemoryMax`, so the cgroup fires first". Virtual size is never below
  resident size, so an equal or larger address limit orders nothing, and raising it would weaken the reservation bound
  that exists today. The two limits measure different things and neither can be made to fire first.
- **Counters.** With `CollectMode=inactive-or-failed` systemd may remove the cgroup as soon as it is empty, racing the
  read of `memory.events` after the run. And `oom_kill` is "the number of processes belonging to this cgroup killed by
  any kind of OOM killer" (cgroup-v2): a system-wide OOM victim counts too, so it does not prove the document hit our limit.

### A1. Placement: the workload is born inside its cgroup (replaces point 1)

The sandbox's outer process is created **directly inside a per-run cgroup** with `clone3(CLONE_INTO_CGROUP)`
(Linux 5.7; Go: `SysProcAttr.UseCgroupFD`/`CgroupFD`, which makes `os/exec` use `clone3`, with no silent fallback: a
refused placement fails `cmd.Start` with EACCES, EBUSY or EOPNOTSUPP, clone(2)). clone(2): the flag "eliminates the
accounting jitter that would be caused if the child process was first created in the parent's cgroup". Every
instruction of the sandbox (outer launcher, namespace init, second launcher, target, any descendant) runs and allocates
inside the run cgroup, whatever path launches it (`withLimits`, `directoryMount`, `directoryLimits`).

The run cgroups live in **one delegated transient scope per app session**, made through the user manager:

- **Session setup** (lazily, at the first sandboxed launch, under a mutex, bounded by 5 s, rows S1 to S9 of the plan).
  S1 start a *holder*: our own binary in a hidden mode (dispatched in `init()` like the sandbox mode), trusted, never
  given a document, which blocks on a pipe from the app and exits on its EOF. It is an unreaped child of the app, so
  its PID cannot be reused while we use it. S2 connect to the user bus (`godbus/dbus/v5`), `Subscribe`, match
  `JobRemoved`. S3 `StartTransientUnit("inkwell-sandbox-<pid>-<random>.scope", "fail", …)` with `PIDs` = the holder,
  `Delegate` = true (scopes, since systemd 218), `Slice` = the slice that contains the app's own cgroup,
  `CollectMode` = `inactive-or-failed`, and the inherited bound of A3. S4 await the matching `JobRemoved` with result
  `done`. S5 **verify, never assume**: `/proc/<holder>/cgroup` is exactly `<slice>/<name>.scope`, the directory is owned
  by our UID, and its `cgroup.controllers` lists `memory`. S6 make `holder/` and move the holder into it (cgroup v2:
  no process may stay in a cgroup that enables controllers for children; systemd's CGROUP_DELEGATION: "move your main
  daemon process out of that cgroup (and into a sub-cgroup)"), then check that the scope's own `cgroup.procs` is empty.
  S7 write `+memory` to the scope's `cgroup.subtree_control` (systemd makes delegated controllers available but "does
  not enable them"; the delegatee writes `subtree_control`) and read it back. S8 read back the inherited bound. S9
  check that the app's cgroup and the scope share an ancestor inside `user@<uid>.service` (the kernel requires write
  access to the common ancestor's `cgroup.procs`; outside the user manager, e.g. a TTY `session-N.scope`, it is
  root-owned and the placement would fail).
- **Per run** (rows R1 to R8). R1 `mkdir <scope>/run-<n>`. R2 write `memory.max` = the resident budget (A2),
  `memory.swap.max` = 0, `memory.oom.group` = 1, all on our own sub-cgroup, which the delegation contract lets us write
  (the scope's own attributes stay systemd's). R3 read back all three: `memory.max` equal to the budget rounded down to
  the page size, the other two exact; any mismatch removes the cgroup and the run uses the fallback. R4 open the
  directory `O_PATH|O_DIRECTORY|O_CLOEXEC` and pass it as `CgroupFD`; the sandbox configuration carries the expected
  path. R5 `Start`; a placement error (nothing ran) removes the cgroup, marks the session unavailable and retries this
  run once in the fallback. R6 **the trusted bootstrap checks its own membership** before anything else
  (`/proc/self/cgroup` equal to the expected path, first statement of `sandboxExec`), before any mount, fork or exec;
  a mismatch exits with a dedicated status and the launch is **refused** (an error, not a fallback: the code is wrong).
  R7 after `Wait`, read `memory.events.local` and `memory.events` (A4). R8 only then write `1` to `cgroup.kill`
  (Linux 5.14; "handles concurrent forks and is protected against migrations"), wait for `populated 0` in
  `cgroup.events` (bounded), and `rmdir`. We own the run cgroup, so nothing removes it before R7.
- **Bootstrap accounting.** Charged to the run cgroup: everything from the first instruction of the outer launcher,
  including its Go runtime (a few MiB, measured in the tests and kept inside the budget), the namespace init, the target,
  its descendants, and the pages of the private output tmpfs they write (shmem is charged to the allocating cgroup).
  Not charged to it: the holder (in `holder/`, outside every run), page-cache pages first faulted by another cgroup (fonts
  or packages the app already read: "a memory area is charged to the cgroup which instantiated it"), and kernel
  objects the app allocated before `clone3`. These exclusions are bounded and never document-controlled.
- **Why not `PIDs` of a paused bootstrap per run** (the shape of runc's systemd driver, `opencontainers/cgroups`,
  `systemd/v2.go`: `Delegate`, `PIDs` of the paused `runc init`): it migrates, so the pause must sit before the Go
  runtime's first allocation, which our re-executed launcher cannot do (its runtime starts before `init()`); it costs one
  D-Bus job per compile; `OOMPolicy` on scopes needs systemd 253 (Debian 12 ships 252); and the unit's collection races
  the counters. **Measured 2026-11-12 on the development machine**: a child created from the app's own
  `app-gnome-…scope` with `CLONE_INTO_CGROUP|CLONE_NEWUSER|CLONE_NEWPID|CLONE_NEWNS` into `run1` of a sibling
  `Delegate=yes` user scope read `/proc/self/cgroup` = `…/run-…scope/run1`; with `memory.max` = 64 MiB, touching
  400 MiB ended in SIGKILL and `memory.events.local` showed `oom 1`, `oom_kill 2`, `oom_group_kill 1`.
  `DelegateSubgroup=` (systemd 254) would do S6 for us; it is not used so that one code path covers systemd 249 and later.

### A2. Two budgets, both kept (replaces the RLIMIT_AS part of point 2)

- **Address budget** `RLIMIT_AS` = `Spec.MemoryLimit`, exactly as today, with or without a cgroup: the reservation bound
  is never raised. It bounds one process's mappings.
- **Resident budget** `memory.max` = `Spec.ResidentLimit` (new field; zero means `MemoryLimit`): it bounds the run's
  aggregate charged memory, every process of the sandbox plus its tmpfs output, which `RLIMIT_AS` never counted.
- **No order is claimed.** Whichever budget is reached first ends the run: an allocation that fails under `RLIMIT_AS`
  (musl's SIGSEGV after Rust's abort, or ENOMEM handled by the program) or a cgroup OOM kill (SIGKILL). Both are
  allowed, both map to `ErrMemoryLimit` by their own evidence (A4), and with point 3 neither starts a core helper. Tests
  exercise each outcome by choosing `ResidentLimit` below and above the address budget.
- The engines keep `ResidentLimit` = `MemoryLimit` (4 GiB for typst and tinymist): no honest document that compiles
  today needs a decision to keep compiling (alternative rejected: a lower resident budget, a product change without data).

### A3. Confinement is verified, the fallback is named for what it is (replaces the rest of point 2)

- `done` from the job is a precondition, never the proof. The proof is S5, S6, S7, R3 and R6, each a read of kernel
  state; any failure of S1 to S9 or R1 to R5 selects the fallback (rows F1 to F11: no session bus, no user manager,
  `StartTransientUnit` refused, including an unknown property on an old systemd, job result other than `done` or
  timeout, holder not in the scope, `memory` not delegated, `subtree_control` refused, `mkdir` or a limit write refused,
  read-back mismatch, placement refused at `clone3`, app outside the user manager). A failed session setup is retried
  after a backoff (1 s doubling to 30 s), never on every launch.
- **No escape from the app's own bound.** A sibling scope leaves the app's cgroup, and with it any limit set on that
  cgroup (an administrator's `MemoryMax=` on the app, or `scripts/heavy.sh`'s 8 GiB scope in the tests). The session
  scope is created with `MemoryMax`/`MemorySwapMax` equal to the app's own cgroup's `memory.max`/`memory.swap.max` when
  they are not `max` (the levels above are shared, since the scope sits in the same slice). A8 narrows this: the copy
  bounds the sandbox alone; only a limit on the shared slice or above bounds app plus sandbox together.
- **The fallback is not equivalent.** It keeps `RLIMIT_AS`, `RLIMIT_FSIZE`, `RLIMIT_CORE` {1,1}, seccomp, Landlock, the
  namespaces where available, the byte caps and the timeout, exactly as today, but it has **no aggregate memory
  containment**: per-process limits only, tmpfs and kernel memory uncounted, and a memory death that is a SIGSEGV
  classified from stderr. `Output.Containment` reports `cgroup` or `rlimit` for every run, so tests, diagnostics and
  logs can tell; the app logs the fallback reason once per session, without paths.

### A4. Counters read before cleanup, and the cause proven (replaces the `memory.events` part of point 1)

Read at R7, before R8. `Output.MemoryCause`:

- `local-limit` (narrowed by A8, which adds the timeline condition): `memory.events.local` shows `oom >= 1` (raised only when *this* cgroup's usage "reached the limit and
  allocation was about to fail") and `oom_kill >= 1`, and the outer process died of SIGKILL. → `ErrMemoryLimit`.
- `victim`: `oom_kill >= 1` with `oom = 0`: killed by a global or ancestor OOM, not by our limit. → not
  `ErrMemoryLimit`; the generic "stopped" error with a fixed text, preview kind `internal`.
- `unknown`: a counter file could not be read or parsed. Never reported as zero; never `ErrMemoryLimit` by this route
  (the stderr route of the address budget still applies).
- `none`: no OOM event.

### A5. The no-core-helper guarantee, scoped (narrows point 3)

`RLIMIT_CORE` {1,1} is the kernel's own recursion sentinel in `coredump_pipe()` (`fs/coredump.c`: "we use cprm.limit of 1
here as a special value"), which also covers systemd-coredump since it is a pipe helper; in file mode 1 byte is below
`min_coredump`. **The guarantee is stated for pipe and file `core_pattern` only.** Socket mode (`@`, `@@`) goes through
`coredump_sock_connect()`, which sets the limit to infinity without that check: there, R3, R4 and R7 of the decision
table may again produce records (RS1). Behaviour in socket mode: the launcher changes nothing (no detection-based
weakening or strengthening of the sandbox); `corePatternMode()` classifies the mode for tests and diagnostics; the
core-helper test and the gate's journal check **fail** (not skip) on a socket-mode machine with an "RS1" message. Two
kernel traces remain by design and are not desktop alerts: the rate-limited `KERN_WARNING` "coredump: PID(comm):
RLIMIT_CORE is set to 1, aborting core" (`printk_ratelimited`, `include/linux/coredump.h`), which the gate uses as
positive evidence that the sentinel path ran, and an audit `ANOM_ABEND` record where audit is enabled
(`audit_core_dumps()` runs before any mode check).

### A6. SIGXFSZ, what is and is not claimed (narrows point 4)

Guaranteed for any target: the kernel byte cap (`RLIMIT_FSIZE`, tmpfs size, watch-dir backstop) and the inherited
`SIG_IGN`. Not guaranteed: how the target ends. It may exit non-zero on EFBIG, abort, loop until the timeout, or reset
the disposition and die of SIGXFSZ (still no core helper by A5). Every such end with an output at its cap is
`OutputLimit`, hence `ErrOutputTooLarge`. "Exits with status 1, `os error 27`, no signal" is claimed only for the
pinned typst (`internal/sidecar/manifest.json`) on the TYP-19 documents. Tests cover the short write at the cap, EFBIG
after it, abort on EFBIG, a disposition reset, an EFBIG loop ended by the timeout, and that the cap is never exceeded.

### A7. `blocked` is a fixed public kind (narrows point 5)

`typst.ErrBlocked` carries a fixed text: no stderr, path, argument or document byte. The preview frame for it is exactly
`{"kind":"blocked"}`, the app's log line for it carries only that fixed error, and the frontend shows only the i18n
string of the kind, whatever else a frame holds. Tests feed a canary through stderr, the project path and the document
and assert it reaches neither the frame, nor the log, nor the rendered text.

### Consequences (replacing those of points 1 and 2)

- Good: membership is a kernel fact from the first instruction, on every launch path; one D-Bus call per session, not per
  compile; per run, a few cgroupfs operations; counters are read while we still own them; the cause of a memory death is
  proven from a timeline, not guessed; limits on the app's slice bound app plus sandbox (A8).
- Less good: one long-lived holder process per session; Linux 5.7 for the placement and 5.14 for `cgroup.kill` (older
  kernels: fallback, and the process-group kill of today); four Linux outcomes to test (cgroup, fallback, refused
  bootstrap, macOS). `godbus/dbus/v5` still becomes a direct dependency (ADR 0008).
- Residual: RS1 (socket mode, A5) unchanged; on a machine without a delegated user manager, aggregate containment is
  absent (A3), reported by `Output.Containment`.

### Confirmation (replaces the one above)

Gate `gates/SB1b.toml`: `spec`, `tla` (model `SandboxScope`, its mutants and witnesses), `core-limit` and
`no-core-helper` (A5), `file-size` (A6), `blocked` (A7), `memory-scope` (A1, A3: delegated session scope, read-back
limits, target and descendant membership, refused bootstrap, inherited bound, removal, fallback at every step),
`memory-budgets` (A2, A4: both budgets, each outcome, cause from counters read before cleanup), `no-coredump-record`
(`scripts/sandbox_no_coredump.py`, see A8), `messages`, `hostile-kept`, `mutants` SB1b-01 to SB1b-32, `regression`.

### A8. Re-review of the amendment (Codex, 2026-11-12, "fix first"; wins over A1 to A7 where they differ)

- **Mixed OOM history (narrows A4).** Counters alone cannot tell a fatal local OOM from a non-fatal local `oom` event
  followed by a global kill: both end with `oom >= 1`, `oom_kill >= 1`, `oom_group_kill >= 1` (a global victim inside an
  `oom.group` cgroup is group-killed too). So the cause is read from a **timeline**: from R4 to R7 the app watches the
  run cgroup's `memory.events.local` with inotify (cgroup-v2: "a value change in this file generates a file modified
  event") and keeps every read with its monotonic time; it also snapshots `oom_kill` of `/proc/vmstat` (every OOM kill
  of the system) and the local `oom` of the session scope and of each ancestor up to `user@<uid>.service`, at R4 and R7.
  `local-limit` requires all of: the first increase of the local `oom_kill` is seen in the same read as an increase of
  the local `oom`, or at most W = 1 s after a read where `oom` rose with no kill; the `/proc/vmstat` `oom_kill` delta
  equals the run's local `oom_kill` delta (no OOM kill anywhere else in the window); no ancestor's local `oom` rose; the
  outer process died of SIGKILL; the watch had no overflow and no failed read. Otherwise a local `oom_kill` is `victim`;
  a failed read or an overflow is `unknown`. Residual RS4: a global OOM that kills only this run within W of a non-fatal
  local `oom` event is called `local-limit`. The kernel raises a non-fatal `oom` only when the OOM killer finds the
  allocating task already exiting or a victim already chosen, so the run was at its own limit in that window; the
  only consequence is the "memory" message instead of the generic one.
- **Aggregate bound (narrows A3).** Copying the app's numeric limit to a sibling does not bound app plus sandbox. The
  claim becomes: the session scope is placed in the **app's own slice**, so every limit set on that slice or above is
  shared by the app and the sandbox and enforced on their sum by the kernel; a limit set on the app's **own leaf**
  cgroup cannot be shared by a sibling, so it is copied to the session scope, which bounds the sandbox alone, and app
  plus sandbox may reach twice it (residual RS5). `scripts/heavy.sh` puts each test in a leaf scope with an 8 GiB limit,
  so RS5 applies to the tests: their per-run `ResidentLimit` stays small. Running heavy jobs in a limited slice instead
  is a separate tooling block (it changes gate machinery, so it needs its own guard override).
- **Holder lifetime and app crash.** The holder reads a pipe whose write end only the app holds. On EOF (the app exited
  or was killed) it writes `1` to `cgroup.kill` of every `run-*` of its scope, waits (bounded, 5 s) for `populated 0`,
  removes them and exits; the empty scope is then collected (`CollectMode`). This is the backstop: each run's outer
  launcher already dies by `Pdeathsig` (the forking thread stays locked from `Start` to `Wait`), which ends its PID
  namespace. The app keeps a pidfd of the holder and checks it before each run (R1). A dead holder invalidates the
  session: a new one is set up for that run (S1 to S9; backoff only if that fails). If the holder dies between R3 and
  R5 and the scope empties, systemd removes it: `mkdir` fails with ENOENT or `clone3` fails because the cgroup is dead
  (ENODEV); both join R5's list, so the run is retried once in the fallback. If other runs keep the scope populated,
  the run starts in a live, verified cgroup, and the stale session is dropped when its last run ends. Session state is
  versioned under a mutex; each run holds a reference to the version it was verified against. In every case R6 still
  refuses a run outside its expected cgroup.
- **Descriptor ownership.** The run cgroup's directory descriptor is opened at R4 with `O_CLOEXEC` and closed as soon as
  `cmd.Start` returns, on every path (success, placement error, retry, refused bootstrap, fallback). Both ends of the
  holder pipe come from `os.Pipe` (`pipe2(O_CLOEXEC)`, atomic, never cleared). The read end is passed only as the
  holder's own `ExtraFiles[0]` and closed in the app right after the holder starts. The write end is never in a launch's
  `ExtraFiles`, so no fork in any goroutine inherits it. This does not depend on the launcher's `CloseRange`
  (`launcher_linux.go::sandboxExec`, after the mounts), which only guards what crossed an `exec`. A sandbox target
  inherits exactly stdin, stdout, stderr and the launcher's declared descriptors, which are closed before its `exec`.
- **Proof of no crash record (narrows A5's gate).** A journal marker is not a completion barrier, and a sentinel line is
  not proof unless it is attributed to a tested death. `scripts/sandbox_no_coredump.py` therefore works from a manifest
  of the deliberate deaths. Each hostile test appends the host PID, the namespace PIDs, the command name, the signal and
  the PIDFD coredump bit (`INKWELL_SANDBOX_DEATHS`). The barrier is: no `systemd-coredump@` unit active, activating or
  deactivating and no collector process, on two polls a second apart; then a marker sent with `logger --journald`
  through journald's native datagram socket (the one systemd-coredump uses, read in order) must become visible. Records
  are matched on `COREDUMP_PID` or the executable. The kernel line must carry the innermost namespace PID and command
  name of a recorded death (the kernel prints `task_tgid_vnr`, the PID in the dying task's own namespace). Its
  `--self-test` replays a delayed collector, unattributed lines, a refused journal and skipped subtests.
- **Skipped subtests (gate).** Every criterion that names tests fails on any `--- SKIP` or `--- FAIL` line at any depth
  and lists the mandatory subtests that must each print `--- PASS`.

### Sources of the amendment (fetched 2026-11-12)

- clone(2), `CLONE_INTO_CGROUP`: https://man7.org/linux/man-pages/man2/clone.2.html
- cgroup v2 (`memory.events` `oom`/`oom_kill`, `memory.events.local`, migration and charges, delegation containment,
  `cgroup.kill`): https://docs.kernel.org/admin-guide/cgroup-v2.html
- systemd cgroup delegation (cut-off point, `subtree_control`, inner-node rule, `DelegateSubgroup=` since 254):
  https://systemd.io/CGROUP_DELEGATION/ ; `Delegate=` since 218:
  https://www.freedesktop.org/software/systemd/man/latest/systemd.resource-control.html
- runc's systemd driver (prior art: `Delegate`, `PIDs`, `OOMPolicy` gated on the systemd version):
  https://github.com/opencontainers/cgroups/blob/main/systemd/v2.go
- Linux `fs/coredump.c` (`coredump_pipe`, `coredump_sock_connect`, `coredump_file`, `vfs_coredump`/`audit_core_dumps`)
  and `include/linux/coredump.h` (`printk_ratelimited`): https://github.com/torvalds/linux (master)
- Go `syscall/exec_linux.go` (`UseCgroupFD` selects `clone3`, no fallback), Go 1.27.4.
