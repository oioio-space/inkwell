---
status: accepted # 2026-11-15, block SB2a1; revision 3, confirmed by Codex on 2026-11-14 (gates/SB2/REVIEW.md, « Round 3 »), gate SB2a1 frozen; design decisions D1-D9 after Codex's round 2; measured facts: gates/SB2/MEASUREMENTS.md runs 1 to 4
date: 2026-11-14
amends: ADR 0033 (the namespace probe and its fallback), ADR 0042 A3 (the memory fallback; the cgroup bound needs a PID namespace, unchanged; the normal bound is memory.high with inkwell's watchdog, memory.max a backstop: point 1g); spec §9.11 (refuse rather than weaken) is kept and made total
decision-makers: [Mathieu]
---

# The sidecar sandbox has two tiers and a refusal, decided once by real launches and verified at every launch; without usable namespaces the sidecar runs as one process that cannot create another

Requirements: W69 (inkwell works on a stock Ubuntu 24.04, an R1 criterion), W68 (cloud review of 2026-11-14), W70
(no ready-made cross-platform sandbox exists: `docs/plans/2026-11-14-sandbox-existing-solutions.md`). Plan:
`docs/plans/2026-11-14-sb2-sandbox-tiers.md`. Rigour: **complete** — the compiler runs on documents written by peers,
the boundary the trust rule keeps (`docs/engineering-principles.md` §4, « Frontière de confiance »).

## Context: what was measured

On GitHub's stock Ubuntu 24.04 runners, amd64 and arm64, kernel 7.0, run 43638847261 (`gates/SB2/MEASUREMENTS.md`):

1. `kernel.apparmor_restrict_unprivileged_userns=1`: an unconfined process creates a user namespace but may not
   write its id map (`unshare -Ur`: « write failed /proc/self/uid_map: Operation not permitted »).
2. inkwell's launcher then fails with `fork/exec /proc/self/exe: permission denied` (**EACCES**), which
   `definitiveRefusal` does not know (`internal/sidecar/mount_linux.go:64-70`: EPERM, EINVAL, ENOSPC at limit 0): the
   probe stays « inconclusive » and **every sidecar launch is refused**: no preview, no export, no completion.
3. With a profile `profile NAME PATH flags=(unconfined) { userns, }` loaded for the binary's path, both probes
   succeed and a `NoNetwork` launch is isolated; the label is inherited by the `/proc/self/exe` re-execution and by a
   binary extracted to a temporary folder (as the sidecar store extracts typst and tinymist). The same binary from an
   unprofiled path is still refused. 92 shipped profiles on 24.04 use this form (chrome, firefox, code, obsidian…).
4. Landlock ABI 8 on that kernel; a ruleset with TCP rules is accepted.
5. On this development machine (Fedora 45, kernel 7.3, SELinux enforcing, `unconfined_t`): namespaces work, and
   `TestSandboxProbeTiers` runs typst `--version`, a compile to standard output and tinymist `--version` both in the
   decided tier (isolated) and with the namespace-free tier forced.
6. Locally, with a seccomp filter that refuses every process creation (fork and vfork denied, `clone` without
   `CLONE_THREAD` denied, `clone3` answered ENOSYS): the fixture project (`internal/typst/engine_test.go`: cmarker,
   lilaq, touying, images, CSV, bibliography) compiles to PDF and SVG with the same bytes as without the filter
   (fixed timestamp); `typst query` works; a tinymist language-server session (initialize, open, completion, hover,
   shutdown) gives the same 7 completions and 2 diagnostics; a Go program's runtime threads work, its child process is
   refused. Peak resident memory: compile 70 MiB, tinymist session 180 MiB; the compile needs an address space between
   128 and 256 MiB (fails at 128, succeeds at 256; the default `RLIMIT_AS` is 4 GiB, `internal/typst/engine.go:46`).

7. Probe run 2 (run 43642240366): tier F forced on the stock amd64 runner compiles to PDF and starts tinymist; with
   the candidate profile both run isolated; on arm64 both die because the allow-list refuses `ppoll` (73) and `tkill`
   (130) (Rust's abort path calls `tkill` after the failed `ppoll`). The trace of a tinymist session showed one process
   creation (`vfork` on amd64, `clone` without `CLONE_THREAD` on arm64) and two `execve`; that trace also followed the
   Python driver, whose `Popen` started tinymist (Codex, round 2) — run 3 settled it (point 9).
8. Linking (this machine, Q12): typst is a static-pie musl binary; tinymist is a dynamic `linux-gnu` binary
   (interpreter `/lib64/ld-linux-x86-64.so.2`, needs `libc.so.6`, `libm`, `libdl`, `libpthread`, `libgcc_s`; glibc
   symbol versions up to `GLIBC_2.28`), so it runs on the machine's glibc (2.44 here, 2.39 on Ubuntu 24.04).
9. Probe runs 3 and 4 (runs 43645254398 and 43647599763), the sidecar traced alone (the tracer is its parent), on both
   runners: across the whole perimeter of point 1b, **no process is created** — typst compile to PDF and SVG, query and
   eval run with 4 threads, the tinymist session with 14 threads, each with one `execve` (its own start); the only
   `prctl` option used is `PR_SET_NAME` (tinymist, thread names), typst uses none; the `ioctl` requests are `TIOCGWINSZ`
   (typst), `FIONREAD` and `TCGETS` (tinymist); typst is static on both architectures (no interpreter); tinymist needs
   glibc symbol versions up to `GLIBC_2.28` on amd64 and `GLIBC_2.30` on arm64, the runners have glibc 2.39 — this does
   not replace the named test S-6 of the plan. On arm64 the allow-list refuses `ppoll` (both sidecars call it at start)
   and `tkill` appears only in the abort path that follows; how both behave once `ppoll` is allowed is SB2a2's
   measurement.

A real Ubuntu 24.04 **desktop** (kernel 6.8, Landlock ABI 4 according to the kernel documentation's ABI table — not
measured) is the owner's check at the end.

## Decision

### 1. Tiers

A launch runs in exactly one of two tiers, or is refused.

| | **N — namespaces** | **F — namespace-free** |
|---|---|---|
| Namespaces | user, mount, PID, IPC; network when `NoNetwork`; `/proc` remounted as PID 1 | none |
| Files | private mount namespace, output on a private tmpfs sized to the budget, IPC folders hidden; Landlock on top | Landlock only: read the listed paths; write only the output slots created beforehand, `RLIMIT_FSIZE` on each; **execute only the target and the files its ELF interpreter needs** (today `readAccess` grants execute on every read path, `launcher_linux.go:422`) |
| Processes | the sidecar's children live in its PID namespace and die with it | **no process creation**: seccomp allows `clone` only with `CLONE_THREAD`, answers `clone3` ENOSYS (glibc then uses `clone`, proven by a named test per architecture, point 1c; Go never uses `clone3` for threads), denies fork and vfork; the **filiation protocol** of point 1a (parent-death signal armed, parent checked, `prctl` filtered so that it cannot be disarmed) |
| Other processes | not visible (PID namespace) | their **content** is not readable (Landlock grants `/proc/self` and `/proc/thread-self` only, `launcher_linux.go:437`); their **existence and metadata** stay visible to `stat` and `sched_getaffinity` (residual R-3); signals: `kill`, `tkill`, `rt_sigqueueinfo`, `pidfd_*` are not allowed, `tgkill` only to its own pid (exact here, since the process cannot fork), Landlock signal scoping when ABI ≥ 6 |
| Resource limits of others | n/a | `prlimit64` allowed only with pid 0 (rule S-1, both tiers) |
| Network | private network namespace when `NoNetwork`; seccomp denies `socket` in all cases | seccomp denies `socket`, so no TCP, UDP, pathname or abstract UNIX socket; only `socketpair(AF_UNIX, SOCK_STREAM)` for the runtime's self-pipe; Landlock TCP rules when ABI ≥ 4, UDP when ABI ≥ 10, abstract-socket scoping when ABI ≥ 6 |
| Memory | cgroup scope when one is delegated **and** a PID namespace exists (ADR 0042, `scope_linux.go:158-160`), else `RLIMIT_AS` per process | `RLIMIT_AS` only; with no descendant possible it bounds the whole job |
| Always | `no_new_privs`, capabilities dropped, the seccomp allow-list (`prctl` by option, `tkill` denied), Landlock (ABI ≥ 3), the caller's deadline | same; the memory bound is on the **address space** (`RLIMIT_AS`), never a promise about kernel memory |

**Refuse** when: Landlock is unavailable or its ABI is below 3; seccomp cannot be installed; neither tier's launch
succeeds; a launch's own verification (point 3) disagrees with the decided tier; **on arm64 until block SB2a2 has
proven its inventory** (plan, D7): today both sidecars die there in either tier, so a refusal with its sentence is the
truth, not a regression.

### 1a. Filiation in tier F (decision D1)

Without a PID namespace nothing kills the sidecar with its parent except the parent-death signal, which a process can
change for itself and which is tied to the thread that forked it (`PR_SET_PDEATHSIG(2const)`). So, in this order, in
the launcher, before the seccomp filter and before `execve(target)`:

1. `prctl(PR_SET_PDEATHSIG, SIGKILL)`;
2. read the parent pid (`PPid:` of `/proc/self/status`, the principle `armNamespaceParentDeath` already uses,
   `internal/sidecar/launcher_linux.go:684`) and compare it with the pid the parent passed; a difference means the
   parent died before step 1: exit at once with a distinct status (covers death before arming);
3. install the seccomp filter, in which `prctl` is allowed only with `PR_SET_NAME` — the one option the two sidecars
   use during the perimeter (measured, point 9) — and `PR_SET_PDEATHSIG` is therefore **denied**: a document turned
   into code cannot disarm the signal. `ioctl` is allowed only with `TIOCGWINSZ`, `FIONREAD` and `TCGETS` (point 9).

The parent side keeps the forking goroutine on its OS thread for the child's whole life (`runtime.LockOSThread` in
`Run`, `internal/sidecar/run.go:206-208`, unchanged and now a named rule), since the signal fires when that thread
exits. Witnesses: `prctl(PR_SET_PDEATHSIG, 0)` from the target is refused; the parent killed before arming, right
after arming, and during the run: the sidecar is gone within 1 s (checked by pid).

### 1b. The functional perimeter guaranteed in tier F (decision D2)

F guarantees exactly what inkwell uses today, nothing more:

| Sidecar | Item | Caller |
|---|---|---|
| typst | `compile --format pdf` (preview and export), `compile --format svg --pages` (visible pages), `query --format json`, `eval --in` (source-to-preview anchors) | `internal/typst/engine.go:578,583`, `query.go:48`, `anchors.go:83` |
| tinymist | `initialize`, `initialized`, `textDocument/didOpen`, `didChange`, `completion`, `signatureHelp`, `hover` (the allow-list `internal/language/language.go:183-187`), `$/cancelRequest`, `workspace/didChangeConfiguration`, `shutdown`, `exit` (sent by Go), `textDocument/publishDiagnostics` (from the server) | `internal/language/language.go`, `frontend/src/language/client.ts` |

Everything else tinymist can do (formatting, rename, references, code actions, commands, an external tool) is outside
the guarantee: inkwell does not call it, and the language bridge refuses it. Each item is compared, in F with the final
filter, against an unrestricted run of the same pinned binary (`TestPerimeterF/<item>`, structured results), on
amd64 in SB2a1 and on arm64 in SB2a2. No item of the perimeter creates a process (measured, point 9), so F's
no-process rule removes none of them; F never allows process creation.

### 1c. Threads without `clone3` (decision D4)

glibc creates threads with `clone3` since 2.34 and falls back to `clone` when `clone3` answers ENOSYS
(`sysdeps/unix/sysv/linux/clone-internal.c`); typst is static musl, which uses `clone`. A named test runs a real
tinymist session and a real typst compile with `clone3` answered ENOSYS and asserts that several threads were created
and that the perimeter's results are unchanged, on amd64 (SB2a1) and on arm64 (SB2a2), recording the glibc version
loaded (point 9).

### 1d. Font folders: exactly the list Go computes (requirement N43)

Both tiers grant read-only access to **exactly** the font folders the caller passes, which are the list of spec §6.12
FT-1 and FT-4 computed by Go (project `fonts/`, inkwell's extracted fonts, « mes polices », the system folders of
FT-4), the same list typst receives as `--font-path` and tinymist as `fontPaths` (`internal/typst/engine.go:474`,
`internal/language/language.go:340`). The launcher's hard-coded fallback (`internal/sidecar/launcher_linux.go:434-435`,
two system folders when the caller passes none) is removed: an empty list grants no font folder. Computing the FT-4
system folders is the fonts lot's work (N43); this block guarantees that whatever list it computes is what the sandbox
opens, nothing more. Witnesses: a font folder not in the list is unreadable from the target, in N and in F; a compile
that needs a font provided in the project's `fonts/` succeeds in F.

### 1e. Protected projects in tier F (requirement N37, coherence C03)

A protected (encrypted) project compiles from a plaintext working folder held in memory (tmpfs without swap) or on a
disk the system encrypts (C03). Tier N gets that folder from its private mount namespace; tier F has no mount namespace
and cannot create one. Until the « Projet protégé » lot specifies how tier F would bound such a folder (a directory
Landlock can confine and no other user can read, on memory that is never swapped), **a launch for a protected project
is refused in tier F** with its own sentence (`preview.fail.protected`), never run on an unbounded folder. The caller
marks the launch (`Spec.Protected`); tier N runs it as today. No protected project exists yet: the rule closes the
gap before the vault lot instead of letting it reopen the tier table.

### 1f. One descriptor per program (requirements N21, N23, N32, AD-57)

The tier decision and every allow-list are per **program**, not special cases of typst and tinymist: a `Program`
descriptor holds its executable, its seccomp allow-list per architecture, its `prctl` options and `ioctl` requests, its
memory bound and its perimeter, and `decideTier` takes the program (arm64 is refused per program until that program's
inventory passes). typst and tinymist are the two entries today; the plugin host of AD-57 becomes a third entry with
its own measured lists, not a redesign. No plugin code is written here.

### 1g. A bound violation is inkwell's to report, not a desktop memory emergency (requirement W73)

Measured on the owner's GNOME desktop (2026-11-14, 23:49–23:52): the hostile-memory tests made the kernel OOM-kill
their targets inside `inkwell-sandbox-*` and `inkwell-test-app-*` scopes under `app.slice` (expected kills, 19 GiB
still available), and GNOME showed « L'application a été arrêtée. La mémoire du périphérique est presque pleine… ».
The source is gnome-settings-daemon's housekeeping plugin (`plugins/housekeeping/gsd-systemd-notify.c`): it subscribes
to `PropertiesChanged` of every systemd user Scope and Service unit and raises that notification whenever a unit's
`Result` becomes `oom-kill`, whatever the cause or the memory left. A cgroup OOM kill at `memory.max` in a scope is
exactly that. On a person's machine a hostile document hitting its bound would raise the same false alarm.

Rule: in tier N with a cgroup scope, the bound a document can reach is **`memory.high`**, not `memory.max`: above it
the kernel throttles and reclaims and « never invokes the OOM killer » (kernel cgroup-v2 documentation, memory.high);
inkwell watches the scope's `memory.events` (a file-modified event on every change, same documentation) and, on the
first `high` event above the bound, kills the target itself with SIGKILL and reports « document too large » in its own
words (the existing bound sentence). A kill by inkwell is not an OOM kill: the unit's `Result` is not `oom-kill`, so
the desktop shows nothing. `memory.max` stays as the **hard backstop**, set above the bound (bound + 25 %, at least
64 MiB more), reached only if the watchdog fails; that case is still an OOM kill and GNOME may then show its
notification — documented as residual R-7, never the normal path. Tier F has no cgroup: `RLIMIT_AS` makes the
allocation fail inside the process (abort), which is not an OOM kill either. Witness: a hostile allocation in N with a
scope ends with inkwell's bound outcome, the scope's `memory.events` `oom_kill` stays 0 and, when a systemd user
manager is reachable, the scope's `Result` is not `oom-kill`; the hard-backstop test is opt-in
(`INKWELL_OOM_BACKSTOP=1`), so the normal suite no longer raises the desktop notification.

The watchdog does not kill on the `high` event alone (amended 2026-11-15). `memory.current` is « the total amount of
memory currently being used by the cgroup » and that includes the page cache (kernel cgroup-v2 documentation, Memory
Ownership: a page is charged to the cgroup that instantiated it; memory.stat `file` is « memory used to cache
filesystem data, including tmpfs and shared memory »), and memory.events `high` counts the times the run was
« throttled and routed to perform direct memory reclaim », so a compile that only reads large fonts or packages
reaches it too, and reclaim then gives the clean cache back at once. Rule: once the run has had a `high` event,
inkwell kills it when `memory.current` stays above `memory.high` at every check (at most 10 ms apart) for 100 ms. With
`memory.swap.max` at 0, only unreclaimable memory (anonymous, tmpfs/shmem, kernel) stays above the bound that long.
Counting anonymous memory alone (`memory.stat` `anon`) was rejected: a document filling tmpfs would escape the
watchdog and meet the OOM killer at `memory.max`. Witness: `TestBoundSparesPageCache` (a target reads a 512 MiB file,
evicted beforehand, under a 128 MiB bound: `high` events, no kill, no bound outcome; red under the first-event rule).

Amended 2026-11-16: the bound is on what reclaim cannot give back, not on `memory.current`. GitHub's `ci` runner
(ubuntu-26.04, the first run with a delegated scope, run 43929170947) killed `TestBoundSparesPageCache` as a bound
violation: reclaim does not give the cache back « at once » while the pages are still waiting for their read, which
cannot be reclaimed until the I/O completes. Measured on the owner's machine (2026-11-16, a 32 KiB reader of an evicted,
incompressible 512 MiB file under `memory.high` = 128 MiB, `memory.current` checked every 10 ms, read bandwidth capped
by `io.max`): uncapped (NVMe), the longest span above `memory.high` is 0 to 22 ms; at 200 MB/s 85 to 96 ms; at
100 MB/s 74 to 116 ms; at 50 MB/s 160 to 201 ms, every run — while `memory.current` less the file cache stays at
7 to 9 MiB throughout. Storage at 200 MB/s or less (a hard disk, a USB stick, eMMC, a cloud disk) is ordinary.
Rule: the watchdog's measure is `memory.current` less the file cache that reclaim can give back without the run's
cooperation: `memory.stat` `file` less `shmem` (tmpfs and shared memory, inside `file`: « file: Amount of memory used
to cache filesystem data, including tmpfs and shared memory », kernel cgroup-v2 documentation), less `unevictable` (mlocked pages; it holds mlocked
anonymous pages too, which are then counted twice, on the side of the bound). Clean mapped file pages (`file_mapped`)
are left out: reclaim unmaps and drops them itself — measured the same day, a run mapping and touching a 512 MiB file
at 50 MB/s under a 128 MiB bound kept `file_mapped` at 112 MiB, `memory.current` at the bound and the measure at
18 MiB, never above the bound; counting them would bring the false kill back for the fonts and packages typst maps.
The 100 ms at
every check is kept for the moment between the two reads. A `memory.stat` lacking any of these counters makes no
decision. Guarantee, stated exactly: the watchdog bounds what it measures; whatever it leaves out is still charged to
the run, so `memory.max` is the hard backstop for it (a cgroup OOM kill, residual R-7). Neither is an absolute
guarantee against every pressure from the host: the kernel can overshoot `memory.high` briefly, and I/O stalls delay
both reclaim and the watchdog's kill. Witnesses: `TestUnreclaimableLeavesOutOnlyFileCache` and
`TestMemoryWatchdogSparesFileCache` (the rule, on ordinary files: page cache and clean mapped pages spared; the same
charge as tmpfs or an mlocked file mapping killed); `TestBoundSparesPageCache/slow_storage` (50 MB/s
through the scope's `io.max` where the io controller is delegated; reproduced the runner's `signal=9
cause=local-limit` under the former rule); `TestBoundSparesDirtyFileCache` (amendment below).

Amended 2026-11-24 (block SB2b-1g): dirty and writeback file pages are left out of the measure like clean cache. The
kernel's flushers write them back and reclaim then frees them, without the run's cooperation (kernel cgroup-v2
documentation, Writeback; `mm/vmscan.c`: reclaim skips folios under writeback and throttles on it). The former premise,
that the kernel throttles a cgroup's dirtying to a small share of its memory, was false: the per-memcg dirty threshold
is `dirty_ratio` of the cgroup's file pages plus its headroom, about zero at the bound, and balance_dirty_pages treats
it as a hard limit only when pausing the writer is not enough (`mm/page-writeback.c`). Measured 2026-11-24, writing
512 MiB under a 128 MiB `memory.high`: dirty plus writeback reached 112 to 123 MiB and the former measure sat on the
bound; GitHub's runner (kernel 7.0, ext4, io not delegated) killed the dirty-cache test as a bound violation (run
45384984242, attempt 1). Counting them protected nothing: tmpfs pages are `shmem` and never `file_dirty`, and in tier N
the only writable files are the output slots on the private tmpfs. Rule: the measure is `memory.current` less
(`file` less `shmem` and `unevictable`); a `memory.stat` lacking one of these three makes no decision; `memory.max`
stays the hard backstop for everything left out, and a stalled device under a disk slot is residual R-7. Witnesses:
`TestUnreclaimableLeavesOutOnlyFileCache`, `TestMemoryWatchdogSparesFileCache/{dirty_file_spared,writeback_file_spared}`,
`TestBoundSparesDirtyFileCache/{local_storage,held_heap_slow_storage}` (the second keeps 108 MiB of anonymous memory
and writes 64 MiB through the scope's `io.max` at 8 MB/s: killed in 5 of 5 runs under the former rule, complete in
5 of 5 under this one; it needs the io controller delegated and runs without the race detector, whose launcher alone
takes tens of MiB of the bound).

### 1h. The address limit is set by the last raw system call before `execve` (2026-11-16)

The launcher is a Go program and shares the target's `RLIMIT_AS` until `execve`. Its runtime has already reserved
far more address space than a small limit: 1.6 GiB (`VmSize` 1 697 048 kB, `VmRSS` 9.7 MiB, 6 threads) measured in
the coverage-built launcher at the moment it lowered `RLIMIT_AS` to 512 MiB (2026-11-16). `setrlimit` accepts a limit
under the current size; from then on every mapping fails, and a Go runtime treats that as fatal. The launcher used to
set the limit before installing seccomp, completing its tier report and waiting for « proceed »: blocking system calls
in which the scheduler may start a thread or take a fresh span. GitHub's `ci` runner lost that race
(`TestHostileMemoryDeathStartsNoCoreHelper/address-limit`: « no tier report: EOF (fatal error: runtime: cannot
allocate memory … persistentalloc1) »): the target never started and the person would have seen a launch failure,
not « document too large ». Rule: everything else is unchanged and stays before the report (`RLIMIT_CORE`,
`RLIMIT_FSIZE`, seccomp); `RLIMIT_AS` is applied after « proceed » by a raw `prlimit64` on the launcher itself (the
filter allows it, S-1), followed by the raw `execve`: no scheduler entry between the two, so the runtime has no
occasion to map memory under the limit, whatever its size; the window before `prlimit64` runs no code of the document.
The target is under the limit from its first instruction (`execve` replaces the address space), soft limit equal to
hard: without a capability it cannot raise it, though the filter still lets it lower its own limits. A failed
`prlimit64` returns at once and `execve` is never called. Witnesses: `TestAddressLimitWaitsForExec` (at the verified report the
launcher still has the address limit it inherited, N and F) and `TestRunLimitsHoldFromStartup` (the target has the
exact limit).

### 2. One decision, by launching each tier for real

`decideTier(facts) (Tier, error)` is pure and total. The facts come from one probe sequence, run once per process and
cached once conclusive, each step bounded by `probeTimeout` (5 s):

| Fact | Obtained by | Values |
|---|---|---|
| `landlock` | `landlock_create_ruleset(VERSION)` | ABI number, or unavailable |
| `seccomp` | installing the real filter in the probe child | ok, unavailable |
| `ns` | **the real N launch** of the pinned, SHA-256-verified `typst --version` (amended 2026-11-15, see below): the same code path, flags, id maps, tmpfs, IPC overlay, `/proc` mount as PID 1, Landlock, seccomp and `execve` as every N launch (`NoNetwork` set, output bounded: the strictest variant, and the only one N uses, see point 3) | ok · refused · inconclusive |
| `f` | **the real F launch** of the same target | ok · failed · inconclusive |

Errno classes of `ns` (total): EPERM, **EACCES**, EINVAL, and ENOSPC with `user.max_user_namespaces` = 0, returned by
`clone`, the id-map writes, `unshare` or a mount in the probe child (reported with status 120) → **refused**. Any other
errno, a timeout, a signal or a crash → **inconclusive**. EACCES counts as a namespace refusal only because `f` is
launched next: a launcher that cannot be executed at all (a `noexec` mount, a policy on the binary) fails both.

| `landlock` | `seccomp` | `ns` | `f` | Outcome |
|---|---|---|---|---|
| unavailable or ABI < 3 | any | any | any | **refuse** « mandatory Landlock unavailable » |
| ABI ≥ 3 | unavailable | any | any | **refuse** « mandatory seccomp unavailable » |
| ABI ≥ 3 | ok | ok | any | **N** |
| ABI ≥ 3 | ok | refused | ok | **F** |
| ABI ≥ 3 | ok | refused | failed | **refuse**, the launcher cannot run here; the error names `f`'s errno |
| ABI ≥ 3 | ok | inconclusive | any | **refuse, retry** (`ErrProbeInconclusive`, backoff as today); nothing cached |
| ABI ≥ 3 | ok | refused | inconclusive | **refuse, retry** |

`probeOutputMount` and `probeIsolation` are replaced by this sequence; `NetworkIsolation(store)` reports the tier.

The probe's target (amended 2026-11-15). The target was first « a no-op target »: the host executable
(`os.Executable()`) run in a hidden no-op mode. In the application that executable is linked to GTK and WebKitGTK, whose
libraries differ from one distribution to the next and are found through `DT_RUNPATH`: the launcher's library resolver
refused it before the tier report, every probe was inconclusive and the preview never started (`gates/SB2a1/UX.md`,
UX-SB2a1-1), while the tests, whose binary is nearly static, saw nothing. The target is now the embedded typst,
verified against its pinned SHA-256 in the caller's `Store` before each probe as before every launch (ADR 0024), run as
`typst --version` under typst's own descriptor (its system-call, `prctl` and `ioctl` lists, point 1f): the program,
the filter and the library set of a real launch, and nothing of the host's. A target that cannot be verified is that
error (`ErrNotEmbedded`, `ErrIntegrity`), not an inconclusive probe. The `DT_RUNPATH` resolution stays (right for any
dynamically linked sidecar). Rejected: keeping the host executable with a wider resolver (the probe would still depend
on libraries no launch uses, and a new host library could break it again); a dedicated static helper binary (one more
artefact to pin and ship, and still not the program a launch runs). Witnesses: `TestProbeTargetIsTheVerifiedTypst`,
`TestProbeRunsVersionUnderTypstDescriptor`, `TestUnverifiableProbeTargetIsNotInconclusive`, `TestProbeIsTheLaunch`.

### 3. One configuration per tier, verified at every launch

- **N** always uses the full set: every namespace including network, the private tmpfs, the IPC overlay. A launch that
  does not need a network barrier or a bounded folder gets them anyway (the probe proved exactly this set). The
  variants `run.go:368-369` and `mount_linux.go:153-155` allow today disappear. **F** has one configuration.
- **Before the document reaches the target**: the launcher writes to its report descriptor the tier it actually
  established (namespaces present, `/proc` remounted, seccomp mode, Landlock ABI used) after every confinement step,
  then waits — with a deadline — for the parent's « proceed » before `execve(target)`. The parent compares with the
  decided tier; on any difference it kills the launcher and returns an error naming both. The target is the only
  process that reads the project, and it is not started until the check passes.
- The handshake fails closed: a report that is absent, truncated or invalid, an end of file, a « proceed » that never
  comes, or a cancellation at any phase ends the launch with an error, the launcher killed and reaped within a bound;
  every descriptor but the target's standard ones and the report pipe is closed before `execve(target)` (plan T-4,
  T-5).
- A launch failure in a tier the probe accepted is an error with its cause, never a silent retry in another tier.
  The memory fallback (cgroup → rlimit, within N) creates the output slots once, after the bound is settled
  (finding 4).

### 4. Nothing waits without a deadline

Every blocking call of `Run`, of the probe and of the scope code is bound to the caller's context or to a named
constant: the pidfd wait in `captureDeath` (poll on the pidfd with the context), the report, ready and proceed pipes,
the D-Bus calls, the cgroup removal, and the termination and reaping of the launcher and the target. Measured on the stock runner, the two tests that « never returned » fail at once
there; the hang seen earlier came with the restriction lifted, and `TestCgroupWorkloadAndDescendantsInside`'s wait
cycle (Codex, round 1, M2) is a defect of that test.

### 5. What a hostile document can do in tier F, and why it is accepted

Assume the document turns typst or tinymist into arbitrary code. In F it can: read the listed paths; use the CPU until
the caller's deadline; allocate up to `RLIMIT_AS` (the whole job: it cannot fork); write the output slots up to their
budget; `execve` only the target itself or its interpreter, under the same filter and ruleset; learn that other pids
exist and their CPU masks; reach the kernel through the allowed calls in the host's namespaces. It cannot: open any
socket; read another process's memory, environment or files under `/proc`; signal (`tkill` denied, `tgkill` to itself
only), trace or re-limit another process; create a process; disarm its parent-death signal or outlive inkwell; write
outside its slots. Accepted, because the alternative on a stock
Ubuntu without the package (a build, an archive, an AppImage) is no preview at all, and because the remaining surface is
the kernel's own, the level of the Landlock-and-seccomp confinement used where namespaces are unavailable. Packaged
Ubuntu runs in N (point 6).

### 6. Ubuntu: the package allows the namespaces (measured form)

The .deb installs `/etc/apparmor.d/inkwell` as a configuration file:

```
abi <abi/4.0>,
include <tunables/global>

profile inkwell /usr/bin/inkwell flags=(unconfined) {
  userns,

  include if exists <local/inkwell>
}
```

`postinstall.sh` loads it when AppArmor is enabled (`/sys/module/apparmor/parameters/enabled` is `Y` and
`apparmor_parser` exists): `apparmor_parser -r -T -W /etc/apparmor.d/inkwell`; `postremove.sh` unloads it
(`apparmor_parser -R`) on remove and purge; neither can fail the installation (errors are printed, the exit status is
0). Measured: the label covers the re-execution and the extracted sidecars, so the packaged binary runs in N. A binary
anywhere else runs in F (measured on the runner: refused today, F after SB2a). The .rpm ships no profile: Fedora's
SELinux lets `unconfined_t` create user namespaces (measured on Fedora 45 here; the policy of other SELinux distributions
is not measured).

**The widening, accepted explicitly (decision D9).** The profile is `unconfined` plus `userns`: it removes Ubuntu's
restriction for `/usr/bin/inkwell` and everything it starts, before any confinement of ours. Beyond our sandbox this
allows the whole application and its descendants (the webview's helper processes included) to create user namespaces,
and so exposes more kernel interfaces (namespaced mounts, network stacks, capabilities inside a namespace) to code
running as the person; it gives no root on the host and no capability outside a namespace. Accepted under the peer
model because the code that runs there is inkwell's own and the pinned, checked sidecars, and because the document
from a peer reaches only the sidecars, which tier N then confines (namespaces, Landlock, seccomp, no capability); it is
the right Ubuntu itself grants to the 92 browsers and editors it ships profiles for (measured, Q1). Install, upgrade
(from a version without the profile and with it), remove, purge, and AppArmor absent or disabled are the upgrade
matrix of block SB2p, whose gate is frozen before its implementation.

### 7. What the person is told

The About dialog's existing line follows the tier (one fact, one place). A sentence appears in the preview and the
problems list only when a launch is **refused** or could not be decided; the reduced tier raises no toast (the person
can do nothing about it). Sentences amended 2026-11-15 after the interface walkthrough (`gates/SB2a1/UX.md`,
UX-SB2a1-2 to -5): each says what it means for the person and what to do, an inconclusive probe is no longer shown as a
broken installation, and About states the same fact as the preview (Go reports `namespace`, `reduced`, `refused` or
`pending` through `NetworkMode`). « Détails » and « Réessayer » use the shared button style; closing About gives the
focus back to « Détails ».

| Outcome | Key | Sentence |
|---|---|---|
| N | `about.network.namespace` (kept) | « La mise en page tourne dans un espace isolé : elle ne peut joindre aucun réseau, pas même cet ordinateur. » |
| F | `about.isolation.reduced` (replaces `about.network.proxy`) | « Vos fichiers restent protégés : le moteur de mise en page ne peut ni lire vos autres fichiers ni utiliser le réseau. Ce système ne permet pas l'isolation complète : le moteur n'est pas entièrement séparé des autres programmes ouverts. » |
| refuse | `preview.fail.sandbox`, action `preview.fail.sandbox.details` « Détails » (opens About) | « L'aperçu est désactivé : ce système ne permet pas d'isoler le moteur de mise en page. Le texte et la collaboration fonctionnent normalement. » |
| refuse, in About | `about.isolation.refused` | « L'aperçu est désactivé sur cet ordinateur : ce système ne permet pas d'isoler le moteur de mise en page, et Inkwell ne le lance jamais sans cette protection. Le texte et la collaboration fonctionnent normalement ; pour voir l'aperçu, ouvrez le document sur un autre ordinateur, ou mettez à jour le système et Inkwell. » |
| refuse, protected project in F (point 1e) | `preview.fail.protected`, same action | « Ce document est protégé : sur ce système, Inkwell ne peut pas encore le mettre en page sans risquer d'exposer son contenu. Vous pouvez continuer à écrire ; pour voir l'aperçu, ouvrez-le sur un autre ordinateur. » |
| refuse, retry (inconclusive probe) | `preview.fail.retry`, action `preview.retry` « Réessayer » (compiles again, hence probes again once the backoff has elapsed) | « L'aperçu n'a pas pu démarrer cette fois. » |
| refuse, retry, in About | `about.isolation.pending` | « L'aperçu n'a pas pu démarrer cette fois : Inkwell n'a pas pu vérifier l'isolation du moteur de mise en page. Choisissez « Réessayer » dans l'aperçu. » |

## Considered options

| Option | Verdict |
|---|---|
| **Two tiers decided by real launches, verified at each launch, plus the measured profile** | chosen |
| Profile only | builds, archives and AppImages stay without preview |
| F only | works everywhere, gives packaged Ubuntu and Fedora less than the kernel allows |
| Patch the two probes (mount `/proc` in `probeIsolation`, add EACCES) | the third patch of its kind; the probes could disagree again at the next policy change |
| Cgroup bound in F | refused: ADR 0042's `NoOrphan` needs a PID namespace (Codex C1) |
| F allowing children with `RLIMIT_NPROC` | refused: `RLIMIT_NPROC` is per user, not per job (Codex C2); measured unnecessary for typst and tinymist |
| Ask the person to set the sysctl to 0 | weakens the whole machine for one program |
| Allow `tkill` on arm64 because the abort path called it | refused (D5): it was called only because `ppoll` failed; any thread-kill call stays denied unless a remeasure shows normal work needs it, and then only restricted to the caller's own threads with a witness that targets an outside thread |
| Allow tinymist's child process in F | refused (D3): F never allows process creation; if a perimeter item needs it, that item is refused in F |
| Rely on the parent-death signal alone | refused (D1): it can be disarmed by the process and races with the parent's death; the parent check and the `prctl` filter close both |
| bubblewrap, a setuid helper, Typst as WebAssembly | bubblewrap needs the same allowance; setuid is a larger risk; WebAssembly is block SW1, measured first (W70) |

## Consequences

- Good: Ubuntu 24.04 works packaged (N) and unpackaged (F); one decision; no hang; the person reads the truth; the
  stock runner becomes a permanent proof.
- Bad: two tiers to test for ever; an Ubuntu-only packaging file; F's kernel surface; N always pays for the network
  namespace and the tmpfs even where a run does not need them.

## Residuals

- R-1: aggregate memory without a cgroup in N (ADR 0042, unchanged); not a residual in F (one process). In both
  tiers the bound is the address space: memory the kernel allocates on the process's behalf is not bounded.
- R-2: on Landlock ABI 3–5, signal and abstract-socket scoping come from seccomp alone (sockets denied, `tgkill` to self).
- R-3: other processes' existence and CPU masks are visible in F.
- R-4: arm64: `ppoll` and `tkill` measured as refused inside (run 2); the sidecars are refused on arm64 until block
  SB2a2 adds `ppoll` (then measures again; `tkill` stays denied) and passes the arm64 inventory.
- R-5: AppImage or Flatpak packaging would need its own answer; a desktop session differs from the runner (owner's check).
- R-6: protected projects are refused in tier F until the « Projet protégé » lot specifies a bounded working folder
  without a mount namespace (point 1e).
- R-7: if inkwell's memory watchdog fails, the hard backstop `memory.max` is an OOM kill and GNOME's housekeeping plugin
  may show « La mémoire du périphérique est presque pleine… » (point 1g); the normal path never reaches it.

## Sources

- gnome-settings-daemon, `plugins/housekeeping/gsd-systemd-notify.c` (master, read 2026-11-14): notifies on a user
  unit's `Result` = `oom-kill` — https://gitlab.gnome.org/GNOME/gnome-settings-daemon/-/raw/master/plugins/housekeeping/gsd-systemd-notify.c
  (module added in 2022: https://mail.gnome.org/archives/commits-list/2022-April/msg09028.html).
- Linux kernel, Control Group v2, memory controller (`memory.high` never invokes the OOM killer; `memory.max` does;
  `memory.events` fields, hierarchical, file-modified events) — https://docs.kernel.org/admin-guide/cgroup-v2.html

- Measured: `gates/SB2/MEASUREMENTS.md` and `PROBE-ubuntu-24.04-{amd64,arm64}.md` (run 43638847261); the plan's
  « Local measurements » (this machine, 2026-11-14).
- Ubuntu, « Restricted unprivileged user namespaces are coming to Ubuntu 23.10 » (read 2026-11-14):
  <https://ubuntu.com/blog/ubuntu-23-10-restricted-unprivileged-user-namespaces>. The flag spelled there,
  `default_allow`, is not what 24.04 ships: the measured profiles use `unconfined`.
- Kernel documentation, Landlock (read 2026-11-14): ABI 3 truncate, 4 TCP, 5 ioctl on devices, 6 signal and abstract
  UNIX socket scoping, 9 pathname UNIX sockets, 10 UDP; <https://docs.kernel.org/userspace-api/landlock.html>.
  The kernel release of each ABI comes from the man-pages Landlock table (Codex, round 1): ABI 4 = Linux 6.7, ABI 6 =
  Linux 6.12; Ubuntu 24.04's GA kernel 6.8 therefore has ABI 4 (not measured on a desktop).
- `PR_SET_PDEATHSIG(2const)` (man-pages): the signal is sent when the parent **thread** terminates, is kept across
  `execve` of a non-set-user-ID binary, and the process may change it — hence point 1a;
  <https://man7.org/linux/man-pages/man2/PR_SET_PDEATHSIG.2const.html>; `clone(2)`: `CLONE_THREAD`;
- glibc `sysdeps/unix/sysv/linux/clone-internal.c`: `clone3` first, `clone` when it answers ENOSYS (read by Codex,
  round 2): <https://raw.githubusercontent.com/bminor/glibc/master/sysdeps/unix/sysv/linux/clone-internal.c>;
  `golang.org/x/sys/unix` `zsysnum_linux_{amd64,arm64}.go` (read locally): `SYS_PRLIMIT64` 302/261, `SYS_SETRLIMIT`
  160/164, `SYS_PPOLL` 271/73, `SYS_CLONE3` 435/435; arm64 has no `fork` or `vfork` system call.
