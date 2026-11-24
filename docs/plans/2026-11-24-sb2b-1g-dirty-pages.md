# SB2b-1g — the sandbox watchdog stops counting dirty and writeback file pages

Block SB2b-1g, rigour **complete** (sandbox boundary). Step 1: specification and gate (`gates/SB2b1g.toml`), no product
code. Amends ADR 0044 point 1g and rule S-11 of `docs/plans/2026-11-14-sb2-sandbox-tiers.md`.

## 1. Defect and evidence

`unreclaimable()` (`internal/sidecar/memory_linux.go:159`) measures `memory.current − (file − (shmem + file_dirty +
file_writeback + unevictable))`; `enforce()` (`:288`) kills the run (`cause=local-limit`) once it has had a `high`
event and that measure stays above `memory.high` for `boundGrace` (100 ms). While a run writes more than its bound
into a disk file, almost every file page it holds is dirty or under writeback, so the measure ≈ `memory.current`,
which the kernel holds at `memory.high`. ADR 0044 1g justified this with « the kernel throttles a cgroup's dirtying
to a share of its memory (dirty plus writeback peaked at 69 to 102 MiB …) ». That premise is false.

- **GitHub**: run 45384984242 (branch `candidate/lp1`, 18a02a7), attempt 1, job `ci`: `TestBoundCountsDirtyFileCache`
  failed at 1.63 s with `signal=9 exit=-1 cause=local-limit` (`bound_linux_test.go:216`); attempt 2 passed. The
  runner is Azure kernel 7.0, ext4, with io not delegated. The test is flaky there, and the rule is what fails.
- **Local** (Fedora 45, kernel 7.3.3, NVMe, btrfs with compression off, `memory.high` 128 MiB, `memory.max` 192 MiB,
  swap 0). A sampler computes the exact formula every 10 ms (scripts and raw lines in `gates/SB2b1g/WITNESSES.md`,
  « Measurements »). Writing 512 MiB at 150 to 300 MB/s through `io.max`: dirty plus writeback reaches
  112 to 123 MiB of the 128 MiB, so the measure sits at the bound. Its longest span above the bound is 0 to 92 ms in
  15 runs: here it stays under the 100 ms grace; on the runner it went past it. With a heap held (80 MiB of anonymous
  memory, then 96 MiB written at 8 MB/s; or 96 MiB held, then 64 MiB written), the measure stays above the bound for
  194 to 358 ms in 6 of 6 runs, so the current rule kills. The measure without dirty and writeback peaks at 87 and
  104 MiB in those runs. Every run completed (`rc=0`, `memory.max` never reached).

## 2. Why dirty pages were counted — threat analysis

History: dirty and writeback pages were added to the kept set by the SB2a1 amendment of 2026-11-16 (squashed in
e86b0ab). The only reasons on record are ADR 1g's « pages that wait on the run's own writes » and the lost mutant MS22:
« dirty file pages are left out …: a run holding dirty cache above its bound is only stopped at memory.max ». MS1–MS22
are no longer in `scripts/netsim/mutants.toml`: PU1 (a541db5) deleted them, and the SB2a1 gate's `mutants` criterion
has been red since; see §6. No threat analysis, test or review gives another reason. The candidate threats:

| # | Threat | Evidence | Needs dirty pages counted? |
|---|--------|----------|----------------------------|
| T1 | A document fills tmpfs or shared memory (memfd, SysV, the tier-N output tmpfs) to escape the bound | shmem stays in the kept set. tmpfs pages never count as `file_dirty`: shmem's `dirty_folio` is `noop_dirty_folio` (`mm/shmem.c:5214`, v7.0) | no: still counted through `shmem` |
| T2 | A document holds memory as dirty pages of a disk file (`write` or a shared mapping) | In tier N the only writable files are the output slots on the private tmpfs (ADR 0044 « Files »; `TestLauncherMountConstruction` checks tmpfs). Landlock refuses every other write. tinymist has no write slot (`internal/language/language.go:137`). A scoped production run cannot dirty a disk page | no: no write path to disk, and each slot is capped by `RLIMIT_FSIZE` |
| T3 | Dirty pages that never get written (stalled device, FUSE or NFS) keep the charge forever | Reclaim does not free them until writeback ends (`mm/vmscan.c:1180-1240`). The charge stays in `memory.current`, so `memory.max` OOM-kills: the hard backstop, residual R-7 | no: memory.max bounds it, and T2 says production cannot create them | → SB2b
| T4 | The desktop OOM notification (W73) when the backstop is reached | Happens only under T3, which production cannot reach | no |
| T5 | Mlocked file pages | `unevictable` stays kept | no |

Verdict: counting dirty and writeback pages protects against nothing reachable, and it kills ordinary runs. The
sidecar is generic: today the test path writes to disk, and AD-57's plugin host may get a disk slot. So the rule
must be right in itself, not only harmless in production.

## 3. Kernel facts (primary sources, Linux v7.0 as on the runner; local 7.3.3)

- `Documentation/admin-guide/cgroup-v2.rst`. memory.high (l. 1350-1362): above it the cgroup is « throttled and put
  under heavy reclaim pressure », and it « never invokes the OOM killer ». memory.max (l. 1376-1385): « the OOM
  killer is invoked … if [usage] can't be reduced ». memory.stat `file_dirty` / `file_writeback` (l. 1592-1598):
  data « modified but not yet written back » or « currently being written back ». Writeback (l. 2173-2232): the
  memory controller defines the domain whose dirty ratio is maintained, the io controller the domain that writes it
  out; « both system-wide and per-cgroup dirty memory states are examined and the more restrictive … is enforced »;
  `vm.dirty_ratio` applies « with the amount of available memory capped by limits imposed by the memory controller ».
- `mm/memcontrol.c:3386-3394`: every non-root memcg has its own writeback domain, so a run cgroup has one whether or
  not io is delegated. `:3414-3437` (`mem_cgroup_wb_stats`): headroom = min(max, high) − usage, which is ≈ 0 at the
  bound. `mm/backing-dev.c:676,799`: the wb's io domain is the *effective* io css, so with io not delegated (GitHub)
  writeback IO is charged to an ancestor and the memcg domain is unchanged.
- `mm/page-writeback.c:829-838` (`mdtc_calc_avail`): available = the memcg's file pages + min(headroom, others' clean
  pages). `:347-410`: thresh = `dirty_ratio` × available (20 % by default, 40 % on Fedora), so ≈ 24 to 48 MiB at a
  128 MiB bound. `:849-866`: dirty + writeback count against it. `:873-880`: balance_dirty_pages « will only seriously
  take dirty throttling threshold as a hard limit when sleeping max_pause per page is not enough … In the other
  normal situations, it acts more gently ». The threshold is a soft target for the writer, not a cap on the charge:
  measured, dirty plus writeback reached 123 MiB against a 24 to 48 MiB threshold.
- `mm/vmscan.c:1180-1240`: in cgroup-v2 reclaim, folios under writeback are marked for immediate reclaim and
  skipped. `:2040-2064`: reclaim wakes the flushers and throttles on writeback (`VMSCAN_THROTTLE_WRITEBACK`). So dirty
  and writeback pages are freed by the kernel's flushers **without the run's cooperation**, once the I/O completes:
  the same as pages under read I/O, which ADR 1g (2026-11-16) already spares.

## 4. Decision

**S-11a** (replaces the measure of S-11): the watchdog measures `memory.current − (file − min(shmem + unevictable,
file))`. Dirty and writeback file pages are file cache that writeback gives back without the run, and they are left out
like clean cache. A `memory.stat` without `file`, `shmem` or `unevictable` makes no decision; `file_dirty` and
`file_writeback` are no longer read. **S-11b** (guarantee, restated): what the measure leaves out (clean, dirty and
writeback cache) is still charged, and `memory.max` (bound + 25 %, at least + 64 MiB) is its hard backstop. In tier N
a run can dirty no disk page (T2), so the backstop is reached that way only by a stalled device under a future disk
slot: that is residual R-7. [done]

Alternatives rejected: (a) a longer grace or a larger bound: this treats the symptom, the spans depend on storage
speed, and it is forbidden. (b) Counting dirty pages only above a share (for example `dirty_ratio` of the bound): the
kernel threshold depends on the kernel version and sysctls, and §3 shows it is not a cap. (c) Counting them only when
writeback makes no progress: memory.stat has no per-memcg write-completion counter, and `io.stat` exists only where
io is delegated (not on GitHub). This would be a new state machine against a threat that T2 rules out. (d) Keeping
the rule and moving the test onto tmpfs: this hides the false rule from the one test that exercises it.

**Model.** `SandboxScope.tla` models SB1b's scope lifecycle and OOM attribution, not the `memory.high` watchdog. The
new rule is a pure function of one counter snapshot. Its dynamics (how fast writeback drains) are the kernel's, and
that environment hypothesis is exactly what was wrong; a model would have assumed it. It is proven instead by pure
tests, Go mutants and a real-kernel witness. No TLA+ or Alloy extension is warranted (a question for the review).

## 5. ADR 0044 amendment (ADR 0044 is written in English)

In point 1g, replace the sentence from « less `file_dirty` and `file_writeback` (pages that wait on the run's own
writes), » to « … the measure at 53 to 69 MiB). » with « less `unevictable` (…unchanged…). », and add after the
2026-11-16 amendment:

> Amended 2026-11-24 (block SB2b-1g): dirty and writeback file pages are left out of the measure like clean cache. [done]
> The kernel's flushers write them back and reclaim then frees them, without the run's cooperation (kernel cgroup-v2
> documentation, Writeback; `mm/vmscan.c`, reclaim skips folios under writeback and throttles on it). The premise
> « the kernel throttles a cgroup's dirtying to a share of its memory » was false. The per-memcg dirty threshold is
> `dirty_ratio` of the cgroup's file pages plus its headroom, ≈ 0 at the bound, and balance_dirty_pages treats it as
> a hard limit only when pausing the writer is not enough (`mm/page-writeback.c`). Measured 2026-11-24, writing
> 512 MiB under a 128 MiB `memory.high`: dirty plus writeback reached 112 to 123 MiB, the former measure sat at the
> bound, and GitHub's runner (kernel 7.0, ext4, io not delegated) killed `TestBoundCountsDirtyFileCache` as a bound
> violation (run 45384984242, attempt 1). Counting them protected nothing: tmpfs pages are `shmem` and never
> `file_dirty`, and in tier N the only writable files are the output slots on the private tmpfs. Rule: the measure
> is `memory.current` less (`file` less `shmem` and `unevictable`); `memory.max` stays the hard backstop for
> everything left out, and a stalled device under a disk slot is residual R-7. Witnesses:
> `TestUnreclaimableLeavesOutOnlyFileCache`, `TestMemoryWatchdogSparesFileCache/{dirty_file_spared,writeback_file_spared}`,
> `TestBoundSparesDirtyFileCache/{local_storage,held_heap_slow_storage}`.

The last sentence of 1g's witness list replaces `TestBoundCountsDirtyFileCache` with these names. The S-11 row of
`docs/plans/2026-11-14-sb2-sandbox-tiers.md` gets the same measure and test names.

## 6. Tests and mutants (step 2)

- `TestUnreclaimableLeavesOutOnlyFileCache`, its rows as subtests (`dirty_left_out`, `writeback_left_out`, …): dirty
  100 → 10 MiB, writeback 100 → 10 MiB, « all three » (60/60/60)
  → 70 MiB, « more kept than cache » (190/190) → 10 MiB. The stat `{file, shmem, unevictable}` becomes complete; the
  stats lacking `unevictable` or `shmem` stay errors. Failure text « S-11a dirty cache » for the new rows.
- `TestMemoryWatchdogSparesFileCache`: `dirty file spared` → no kill; a new `writeback file spared` → no kill; `tmpfs` and
  `mlocked file` still killed.
- `TestBoundCountsDirtyFileCache` is renamed `TestBoundSparesDirtyFileCache`, and its comment loses the false figures.
  Subtests: `local_storage` (today's 512 MiB unthrottled write, the case the runner killed) and the **new witness**
  `held_heap_slow_storage`. As implemented at step 2, the target keeps its resident anonymous memory at 108 MiB: an
  mmap outside the Go heap, faulted in with `MADV_POPULATE_WRITE`, tops it up and is given back as the runtime grows.
  It writes 64 MiB to a disk file under the scope's `io.max wbps=8 MB/s`. The step-1 sizing (80 MiB held, 96 MiB
  written) killed the Go target in only 0 of 3 runs, and 96 + 64 in 1 of 3, under the former rule. Want: complete, no
  bound outcome, no OOM kill, `high` > 0. Red under the former rule in 5 of 5 runs, green in 5 of 5 under S-11a (2026-11-24, final helper).
  It skips where io is not delegated or the directory is not on a block device, like
  `TestBoundSparesPageCache/slow_storage`, and under the race detector: there the run's launcher (the race-built test
  binary) alone adds tens of MiB of anonymous memory to the cgroup (measured: 127 MiB of anon with nothing written).
  The gate's `witness` and mutant MD2 run without `-race`. Where it skips (GitHub), the documented equivalent is
  `local_storage` on the runner's own storage, the run that reproduced the kill.
- Go mutants (`scripts/netsim/mutants.toml`, block `SB2b1g`; anchors written at step 2): MD1 dirty and writeback back
  in the kept set → `TestMemoryWatchdogSparesFileCache` « S-11a dirty cache »; MD2 the same edit →
  `TestBoundSparesDirtyFileCache` « S-11a dirty cache » (real kernel); MD3 writeback alone kept →
  `TestUnreclaimableLeavesOutOnlyFileCache` « S-11a dirty cache »; MD4 shmem dropped → `TestMemoryWatchdogSparesFileCache`
  « S-11 page cache »; MD5 unevictable dropped → same test and text; MD6 the missing-counter check removed →
  `TestUnreclaimableLeavesOutOnlyFileCache` « S-11 page cache »; MD7 the whole file cache counted (former MS20) →
  `TestMemoryWatchdogSparesFileCache` « S-11 page cache ».
- Outside this block's scope: SB2a1's mutants MS1–MS22, deleted by a541db5, need restoring under their own inventory
  block (SB2a1's gate criterion `mutants` is red at HEAD). MD4, MD5 and MD7 restore the S-11 ones.
