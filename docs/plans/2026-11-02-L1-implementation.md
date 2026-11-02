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
the dependency graph with a scan of all platform sources.

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
