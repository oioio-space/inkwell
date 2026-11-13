# Smoke failure follow-up (2026-11-13)

The frozen `gates/SB1b.toml` and original host logs are unchanged.

- memory-scope: the test-owned `io.PipeWriter` remained open after the release
  byte. `exec.Cmd`'s stdin copier was blocked reading that writer, independently
  of the dead sandbox and its descendants. Close the writer before awaiting
  Run. No timeout or ownership change to Spec.Stdin was introduced.
- session-lifetime / hostile-kept: Landlock's `/proc/self` rule refers to the
  original target's proc directory, so the descendant cannot enumerate its own
  `/proc/self/fd`. The descriptor helper now queries every fd below RLIMIT_NOFILE
  with F_GETFD and reads the symlink of each open fd; no production permission
  was broadened. Helper exit status and query errors are checked explicitly.
- session-lifetime: placement-error invalidated and closed the shared session;
  refused-bootstrap then created a replacement (holder pipe plus two pidfds).
  Give each path an independently warmed session before taking its baseline.
  Retain EBADF and count checks, and also reject any new fd/target pair.
- memory-budgets: x/sys.Getrlimit uses the legacy getrlimit syscall on amd64,
  which the workload allowlist does not permit. Use query-only prlimit64 for
  AS and CORE inspection, and NOFILE for descriptor inspection. A pure BPF
  regression checks queries for all three resources. The two budgets and the
  syscall policy were not changed.
- no-coredump-record: the hostile test used options{}; its year-one timestamp
  became -62135596800, which Typst parsed as an unexpected option. Supply the
  engine's DefaultTimestamp, as the normal compile path does.
- messages: the host log reports only `pnpm: No such file or directory`; no
  frontend assertion executed. On the real host, invoke the unchanged gate
  with the pinned toolchain on PATH: `mise exec -- ./scripts/gate.sh SB1b`.

Light validation completed here: `go build ./...`,
`go vet ./internal/sidecar/... ./internal/typst/...`, `gofmt -l .` (empty),
and selected pure sidecar policy/memory/manifest and Typst failure tests.
Go scratch/cache directories for these commands are under `.task/sb1b-light`.
No heavy, cgroup, systemd, Typst compilation, or journal test was run here.
These changes require the real-host frozen criteria to run again; no new green
host result is claimed. No commit or stash was made.

## Review round (Opus)

1. **Refusal/status collision:** removed the reserved exit 124. The trusted
   bootstrap reads the report descriptor before checking membership and writes
   a dedicated one-byte refusal record. Run uses that record, independent of
   exit status; the target never inherits the writer across exec.
   `TestReviewBootstrapReport` covers both records and an ordinary exit 124;
   `TestCgroupTargetExit124` covers the full scoped host path. The existing
   unexpected-membership test now requires the refusal record.
2. **Older-kernel cleanup:** setup checks for writable `cgroup.kill`, accepting
   readable `cgroup.procs` as the pre-5.14 fallback. Cleanup repeatedly sweeps
   this leaf's processes until `cgroup.events` is empty. Each kill pins a pidfd
   and rechecks membership, avoiding PID reuse and unrelated-process kills.
   EOF cleanup uses the same function. `TestReviewKillFallback` exercises a fake
   scope without `cgroup.kill`, including refusal when neither interface exists.
3. **Start errors:** session invalidation/retry requires a placement error
   (`errors.Is`: EACCES, EBUSY, EOPNOTSUPP, ENODEV; ENOENT also requires a
   vanished run directory). Executable ENOENT, ENOEXEC and EAGAIN preserve the
   session and propagate the error. `TestReviewPlacementErrors` covers both
   classes; `TestCgroupStartErrorPreservesSession` checks the live-session path.
4. **Optional proc children:** host identity lookup is best effort for normal
   runs, including the final status read. DeathTest explicitly requires identity,
   propagated through each launcher configuration. `TestReviewOptionalHostIdentity`
   checks missing CONFIG_PROC_CHILDREN evidence with and without that requirement.
5. **Counter oracle:** `TestCgroupCountersReadBeforeCleanup` now reads the actual
   local counter file inside the memory watch's final-read hook. SB1b-17 moves
   removal ahead of `watch.finish` while leaving trace order unchanged: it must
   fail on unreadable counters, rather than a synthetic event-order mismatch.
   That cgroup mutant is staged for the host rerun; it was not executed here.
6. **Refusal test:** fixed `&&` to `||`, removed the dead relative-path mutation
   and no-op fault injection, and changed the oracle to the trusted report.
7. **Fallback evidence:** runs without a memory watch report `unknown`, never
   an observed zero (`none`). The fallback test now asserts this as well.
8. **Unspecified limit:** removed memory.high writes/readbacks; retain the
   memory.max, swap and OOM-group write/readback assertions.
9. **Descriptor helper:** exhaustive scanning has a hard 1,048,576-descriptor
   ceiling; exceeding it fails the helper explicitly, rather than truncating
   the oracle or scanning an arbitrary hard limit.
10. **Style:** replaced the job-wait goto with `waitJob`, used `err`, expanded
    affected long literals, grouped imports, replaced static Errorf calls with
    errors.New, and used SplitSeq for line iteration. Refreshed affected mutant
    anchors; all 32 SB1b before-patterns still match exactly once.

Validation: all four focused regression tests pass; behavior mutants for items
1–4 each produced a real `--- FAIL` in the corresponding pure test. Selected
pure memory attribution, identity/manifest, exit-status and Typst failure tests
pass. `go build ./...`, `go vet ./internal/sidecar/... ./internal/typst/...`, and
`gofmt -l .` pass (format list empty). No cgroup/systemd/heavy tests were run;
the new full-run tests and SB1b-17 require Claude's frozen host rerun.
`gates/SB1b.toml` is unchanged. No commit or stash was made.
