Linux syscall policy
====================

Run `scripts/heavy.sh python3 scripts/sidecar-trace.py OUTPUT` with STRACE,
TYPST and TINYMIST pointing to strace and the verified pinned binaries. Generate
PM-1 with the L0 baseline task first. The script runs real compile, query, eval,
and a Tinymist initialize/open/document-symbol/shutdown LSP session. Then run
`python3 scripts/sidecar-allowlist.py OUTPUT` and review the generated diff.
CI should retain the raw traces and compare the generated tables on each Linux
architecture; adding a syscall requires review, never an automatic expansion.
The initial amd64 traces and their hashes are in gates/L1/seccomp-traces.
The arm64 table translates the observed syscall names to arm64 numbers; native
arm64 trace validation is still required before claiming coverage there.

Unknown native syscalls return EPERM, including userfaultfd, perf_event_open,
and modify_ldt. ABI mismatch and x32 kill the process (SIGSYS), since a syscall
number cannot safely be interpreted under a different ABI. clone3 returns
ENOSYS to select libc's inspectable clone fallback. IPC, networking, external
signals and privileged operations remain denied even if observed in a trace.
fcntl ownership/signal commands and O_ASYNC, and ioctl FIOSETOWN/SIOCSPGRP, TIOCSTI and TIOCLINUX are
denied by argument. clone(CLONE_PARENT) is also denied; launches start a new
session with Setsid to detach from the controlling terminal. The table explicitly adds the Go runtime/test helper calls
and epoll_pwait for architecture portability; see the derivation script.

Pinned linux/amd64 `file`/`ldd`: Typst is static PIE; Tinymist is dynamic with
/lib64/ld-linux-x86-64.so.2 and libgcc_s, libpthread, libm, libdl, libc.
The launcher reads PT_INTERP and recursively resolves DT_NEEDED without running
ldd. Landlock permits those exact library files only for dynamic binaries,
plus the executable, project, explicit verified font/package directories,
/dev/null, /dev/urandom, self procfs and CPU/memory information. Certificate
bundles are readable because Typst initializes TLS before attempting a denied
socket. Home itself has no read rule. All inherited descriptors >=3 become
close-on-exec. The namespace launcher remains PID 1, mounts private procfs,
reaps orphans and forwards TERM/INT to its confined workload.

Violations and core dumps (ADR 0042, amendment A5 and A6)
---------------------------------------------------------

A sandboxed death must never reach the desktop's crash reporter. The target
starts with RLIMIT_CORE = {1, 1}: in `fs/coredump.c` a limit of exactly 1 is
the kernel's own recursion sentinel for pipe helpers ("RLIMIT_CORE is set to 1,
aborting core"), so a core-class signal (the SIGSYS of an ABI mismatch, a
SIGSEGV after a failed allocation, a SIGABRT) starts no systemd-coredump,
apport or abrt helper; in file mode one byte is below a core file's minimum.
Max = 1, so the target cannot raise it. The guarantee is stated for the pipe and
file modes of kernel.core_pattern only: the socket modes (`@`, `@@`, Linux
6.16 and 6.17, `coredump_sock_connect`) ignore the limit (residual RS1). One
rate-limited kernel line stays, as does an audit ANOM_ABEND record where audit
is on; neither is a desktop alert. `Output.CoreDump` reports the kernel's
PIDFD_INFO_COREDUMP record (Linux 6.16) for the target, carried across the PID
namespace by the init's report byte; the tests require it to be "none".

The target also starts with SIGXFSZ ignored, and no other signal ignored
(inherited ignored signals are reset): execve(2) keeps ignored signals ignored,
so a write past RLIMIT_FSIZE is shortened at the cap and the next one fails with
EFBIG instead of killing the target. The byte cap is the kernel's either way; a
target that restores the default action dies of SIGXFSZ, still at its cap and
still with no core helper.


Core sentinel integrity (P1-a)
-----------------------------

A hard RLIMIT_CORE of one prevents raising, but does not prevent lowering its
soft value to zero, which reopens pipe helpers. After setting {1,1}, seccomp
refuses every native setrlimit(RLIMIT_CORE, ...) and every
prlimit64(any_pid, RLIMIT_CORE, non_null_new_limit, ...), with EPERM. A
query-only prlimit64 remains allowed. The resource argument is a kernel
unsigned int (lower 32 bits); both words of the new-limit pointer are checked.
The filter never reads target memory, is inherited by descendants and survives
exec. arm64 has no legacy setrlimit entry point. Sources: getrlimit(2),
https://man7.org/linux/man-pages/man2/getrlimit.2.html and kernel seccomp docs,
https://docs.kernel.org/userspace-api/seccomp_filter.html.

Aggregate memory scope (ADR 0042 A1-A4, A8-A9)
--------------------------------------------

SandboxScope lazily creates a transient delegated user scope with a trusted
holder. The holder's membership, ownership, controller availability, move into
holder/, empty scope root and enabled memory subtree are checked, rather than
inferred from a completed D-Bus job. The scope is in the app's own slice, so
limits on that slice and above bound app plus sandbox together. The app leaf's
finite memory.max and memory.swap.max are copied to the scope: that bounds the
sandbox alone; a sibling cannot share a leaf bound (RS5).

Each run has its own run-N cgroup. memory.max is set to the page-rounded
resident budget (memory.high is not written: equal to memory.max it would be a
no-op), followed by memory.swap.max=0 and memory.oom.group=1. Every value is read back before Start. RLIMIT_AS stays at
MemoryLimit; neither budget is raised or ordered relative to the other.
SysProcAttr.UseCgroupFD/CgroupFD uses clone3 CLONE_INTO_CGROUP to create the
outer launcher in that cgroup, without migration or accounting jitter.
This trusted pre-seccomp clone3 is distinct from target clone3, which still
returns ENOSYS. The bootstrap checks membership before mounts/forks/exec and
refuses a mismatch. Descendants and private tmpfs charges share the run bound.

A cgroup launch requires a PID namespace. Its init rearms parent-death SIGKILL
on the locked launching thread and checks its parent through the original
host procfs (getppid is zero for an external parent). Init death causes the
kernel to kill the namespace. The holder sees app-pipe EOF, kills and removes
remaining run groups within five seconds, then exits. Dead holders invalidate
sessions; placement failures close the directory fd, retire that version and
retry once with rlimits. Active runs retain their own session version. Next
session setup sweeps stale groups of dead apps. Scope setup backs off from one
to thirty seconds; fallback logs a fixed reason once and reports rlimit.

An inotify timeline of memory.events.local, global oom_kill and each ancestor's
local oom establishes MemoryCause. local-limit requires SIGKILL, a local OOM
within one second of the first local kill, matching global/run kill deltas,
and no ancestor OOM. A different history is victim; read failure, overflow or
bounded timeline overflow is unknown. Counters are read after Wait and before
cgroup.kill/rmdir. Collector loss after holder death remains unknown (RS6).
Victim/unknown SIGKILL errors are fixed text and never trust compiler stderr.

Test death evidence is opt-in through DeathTest and INKWELL_SANDBOX_DEATHS.
The trusted namespace init retains a CLOEXEC handle to the original procfs,
reads the target's host PID, NSpid and actual comm before reap, and reports
that identity with its PIDFD core fact. Targets inherit neither this handle,
the holder pipe, nor a cgroup directory. The journal checker keeps its native
socket marker and collector-quiescence barrier and matches kernel evidence to
those recorded identities; socket core_pattern modes still fail with RS1.
