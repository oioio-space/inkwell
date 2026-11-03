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
