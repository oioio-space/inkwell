# L1 — implementation and evidence

The L1 implementation uses the hidden re-exec mode explicitly permitted by the
lot brief. Linux sidecars re-execute the current binary with an explicit versioned
argument before any application entry point; this is our own sandbox launcher.
No shell or util-linux executable participates. The current runner remains in
internal/sidecar (the allowlist in arch-check); moving it to the future
internal/sandbox port and the full Verified type migration remain specification
targets, not claims of this lot. Store.Resolve still verifies embedded sidecars
before the production caller builds its Spec.

The child locks its OS thread, mounts private tmpfs when supported, completes the
existing parent handshake, sets no_new_privs, clears ambient and inheritable,
effective and permitted capabilities, and drops the bounding set when it has
CAP_SETPCAP (in particular inside the private mount user namespace). In that case
securebits prevent root capability recovery. Ordinary unprivileged fallback
processes cannot drop the bounding set; empty capability sets plus no_new_privs
prevent privilege acquisition. Landlock write policy is unchanged: only declared
precreated files can be written/truncated, no creation, removal or replacement.
ABI < 3 refuses bounded output. Rlimits are exact bytes, including AS, FSIZE and
CORE=0, applied immediately before exec on the locked thread. No untrusted code
runs in the launcher. Go runtime threads disappear at exec.

Namespace probes re-execute the same binary. The tmpfs success probe includes
the actual mount; failures are retried and the existing fixed-share Landlock
fallback remains. As before, a host denying user namespaces cannot certify the
tmpfs path: existing namespace-dependent tests report their original skips.
The fallback attack tests run without skipping. Non-Linux bounded launches still
fail closed; this lot does not certify P-SANDBOX-MAC or P-SANDBOX-WIN.

WebKitGTK itself launches WebKitWebProcess, WebKitNetworkProcess,
WebKitGPUProcess, bwrap and xdg-dbus-proxy. Those are guaranteed library helpers
(architecture §14.1), never programs launched by our application code.

scripts/sidecar-no-tools.sh compiles the race-instrumented test binary, then runs
the entire existing sidecar suite with PATH empty. arch-exec-check complements
the dependency graph with a scan of all platform sources. (2026-11-03: the
dependency-graph scan was removed by the user's N42 clarification; only our own
code is checked. See docs/objectif-en-cours.md.)

scripts/gate-guard.py compares main with working-tree content (including untracked
files). It rejects removed/deferred criteria, loosened or deleted thresholds,
added skips and increased/removed/rewritten timeout expressions. Unknown threshold
changes fail closed. Overrides require exact finding/content hashes, a reason and
an existing decision document; both the override file and decision must appear in
manifest raw_results. No override is used by L1. Its --self-test exercises
positive and negative cases. Overrides require human review, as does the guard
itself; no script can protect itself from a malicious replacement in its own lot.

Sources used to check syscall semantics:
- https://docs.kernel.org/userspace-api/landlock.html
- https://pkg.go.dev/golang.org/x/sys/unix

Run mise run gate -- L1 for machine evidence in gates/L1.json and raw logs;
the fifth criterion runs L0 itself. Security review remains mandatory before merge.


L0 worktree reproducibility: the baseline build now writes frontend/dist, the
directory actually embedded by Go, instead of an unrelated shared cache. pnpm
exec has verifyDepsBeforeRun=false so validation uses installed dependencies and
never attempts to modify frontend/node_modules. No threshold, sample count or
timeout is changed.

The existing timeout/cancellation fixtures materialized range(3000000000), so
memory exhaustion could precede cancellation. They now use nested range(10000)
loops with bounded memory; every deadline and assertion remains unchanged.
Failed gate attempts are retained under gates/L1/failed-attempt-2-*.
A diagnostic performance-core affinity run removed the frame benchmark
regressions observed with default scheduling. The acceptance gate keeps its
original scheduling, sample counts, frozen reference and thresholds.

## Part 2: mandatory launch confinement

Every Linux Run and Supervisor target enters sandboxExec, including unlimited
query/eval commands. There is no limits-dependent exemption. The child mounts
IPC overlays before dropping capabilities, sets no_new_privs, installs a pure Go
seccomp-BPF filter, applies the ABI-sized Landlock policy, and then execs.
Failure to install either mandatory policy refuses the target with an explicit
sandbox error. Landlock ABI 3 remains the minimum filesystem policy; ABIs 4, 5,
6, 9 and 10 add TCP, device ioctl, IPC scopes, pathname UNIX sockets and UDP.
UDP constants follow Linux's include/uapi/linux/landlock.h, bits 2 and 3.

Available user/mount namespaces also use CLONE_NEWPID. Without namespaces,
seccomp independently blocks external signals and all socket creation, regardless
of the Landlock ABI's IPC or TCP support. Self-directed tgkill remains available
for runtime preemption. Audit architecture mismatch and x32 kill the process;
ordinary prohibited calls return EPERM. io_uring_setup is denied to avoid its
alternate socket/IPC operations.

The real tinymist stdio initialize test established that Tokio's signal handler
requires an unnamed AF_UNIX SOCK_STREAM socketpair. Only that pair type is
permitted; connect, bind, sendto, sendmsg and sendmmsg stay denied. Typst download
tests assert an actual failed connection with EPERM/EACCES, rather than inferring
an attempted download from an idle listener. The namespace-free syscall probe,
ABI mismatch tests and unlimited/limited/bounded/supervised child-status tests
exercise the independent filter. Both unlimited and bounded runs test hidden
host sentinels and tmpfs identity at XDG_RUNTIME_DIR and /tmp/.X11-unix.

The architecture gate now builds a go/analysis vet tool and type-checks all four
shipped OS/architecture targets in server mode. Function identities catch aliases,
dot imports and function values. It rejects raw syscall APIs, linkname, import C,
and native C/assembly sources outside internal/sidecar. Ignored files are also
checked for C imports, linkname and native sources. Analysistest covers syscall 59,
aliased API values, linkname to unix.Exec, C syscall(SYS_execve), token-pasted
system macros and wordexp. The build-time downloader now reads tar, gzip and xz
in Go, so it needs no process-launch allowlist exemption.


The archived 2026-11-02 baseline is immutable. Gate entry and completion verify
its complete file set and SHA256SUMS without resealing it. All fresh acquisition
scripts default to gates/L0 and inherit INKWELL_PERF_OUT from the current lot.
The archive self-test runs a gate, compares every archived byte, and proves
corruption is rejected without updating checksums. Evidence report paths are
not acceptance thresholds; the guard continues to protect threshold values.
