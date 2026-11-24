# SB2b1g — witnesses at step 1 (2026-11-24, tree 8e23cff, before any code)

Every criterion of `gates/SB2b1g.toml` was run once on this tree (Fedora 45, kernel 7.3.3, amd64, io delegated to the
user manager: `cpu io memory pids`).

| Criterion | Result | Why (the right reason) |
|-----------|--------|------------------------|
| `docs` | red | ADR 0044 1g lacks the 2026-11-24 amendment and still states « Writes stay ordinary runs: the kernel throttles a cgroup's dirtying… » and « less `file_dirty` and `file_writeback` »; the S-11 row of the SB2 plan still names `TestBoundCountsDirtyFileCache`; `memory_linux.go` and `bound_linux_test.go` keep the false premise. Checked able to pass: the plan's §5 text, applied to the ADR, satisfies every ADR clause |
| `rules` | red | `TestMemoryWatchdogSparesFileCache/{dirty_file_spared,writeback_file_spared}` and `TestUnreclaimableLeavesOutOnlyFileCache/{dirty_left_out,writeback_left_out}` missing (today « dirty file » is expected *killed*) |
| `witness` | red | `TestBoundSparesDirtyFileCache` missing (`no tests to run`), `TestBoundCountsDirtyFileCache` still present |
| `mutants` | red | no `SB2b1g` entry in `scripts/netsim/mutants.toml` (they are written at step 2) |
| `regression` | green | `internal/sidecar` 386 s, `sidecar/fetch` 1 s, `typst` 78 s, `language` 3 s (389 s in all, cap 900); `internal/app` 77 s, `cmd/inkwell` 18 s (84 s, cap 300); `-race`, `-count=1` |
| `github` | red | `gates/SB2b1g/GITHUB.md` has no run id (no candidate yet) |
| `simplify-report` | red | `gates/SB2b1g/SIMPLIFY.md` missing |

## The defect on GitHub

Run 45384984242 (`candidate/lp1`, 18a02a7, `workflow_dispatch`), attempt 1, job `ci`:

    === FAIL: internal/sidecar TestBoundCountsDirtyFileCache (1.63s)
        bound_linux_test.go:216: S-11 page cache: dirty run: signal=9 exit=-1 cause=local-limit stdout="" stderr="", want a complete write and no bound

Attempt 2 of the same run succeeded: on the runner the former rule kills an ordinary write some of the time.

## Measurements (the former rule's measure vs S-11a's, real kernel)

A transient user scope (`MemoryHigh=128M`, `MemoryMax=192M`, `MemorySwapMax=0`, optional `IOWriteBandwidthMax` /
`IOWriteIOPSMax` on `/dev/nvme0n1`) runs a writer into `~/.cache/inkwell-dirtyrepro-n` (btrfs, compression off with
`chattr +m`). A sampler reads `memory.current` and `memory.stat` every 10 ms and computes the former measure
(`max_measure`, with `longest_over_bound` the longest span above 128 MiB, and `old_kill_at` when it first lasted
100 ms) and S-11a's (`new_max`). Each run went under `scripts/heavy.sh --exclusive`.

512 MiB written, no heap held (former measure only; 12 runs at 150-300 MB/s or 100 IOPS, plus the diagnosis' three at
200 MB/s: 81, 82 and 0 ms):

    wbps150M#1: elapsed=3.13s max_measure=128MiB max_dirty+wb=121MiB max_current=128MiB longest_over_bound=92ms watchdog_would_kill_at=never rc=0
    wbps200M#1: elapsed=2.24s max_measure=129MiB max_dirty+wb=122MiB max_current=129MiB longest_over_bound=73ms watchdog_would_kill_at=never rc=0
    wbps300M#1: elapsed=1.53s max_measure=128MiB max_dirty+wb=122MiB max_current=128MiB longest_over_bound=72ms watchdog_would_kill_at=never rc=0
    wiops100#1: elapsed=2.43s max_measure=128MiB max_dirty+wb=122MiB max_current=129MiB longest_over_bound=73ms watchdog_would_kill_at=never rc=0
    wbps150M#2: elapsed=3.33s max_measure=129MiB max_dirty+wb=122MiB max_current=129MiB longest_over_bound=82ms watchdog_would_kill_at=never rc=0
    wbps200M#2: elapsed=2.45s max_measure=129MiB max_dirty+wb=122MiB max_current=129MiB longest_over_bound=81ms watchdog_would_kill_at=never rc=0
    wbps300M#2: elapsed=1.54s max_measure=120MiB max_dirty+wb=113MiB max_current=128MiB longest_over_bound=0ms watchdog_would_kill_at=never rc=0
    wiops100#2: elapsed=2.16s max_measure=129MiB max_dirty+wb=123MiB max_current=129MiB longest_over_bound=62ms watchdog_would_kill_at=never rc=0
    wbps150M#3: elapsed=2.76s max_measure=128MiB max_dirty+wb=122MiB max_current=129MiB longest_over_bound=82ms watchdog_would_kill_at=never rc=0
    wbps200M#3: elapsed=2.35s max_measure=119MiB max_dirty+wb=112MiB max_current=128MiB longest_over_bound=0ms watchdog_would_kill_at=never rc=0
    wbps300M#3: elapsed=1.46s max_measure=129MiB max_dirty+wb=122MiB max_current=129MiB longest_over_bound=82ms watchdog_would_kill_at=never rc=0
    wiops100#3: elapsed=2.07s max_measure=129MiB max_dirty+wb=122MiB max_current=129MiB longest_over_bound=51ms watchdog_would_kill_at=never rc=0

Heap held, then written at 8 MB/s; and the 512 MiB write at 200 MB/s with both measures:

    anon80-w96-wbps8M#1: elapsed=10.49s max_measure=129MiB max_dirty+wb=42MiB max_current=130MiB longest_over_bound=195ms new_max=87MiB new_longest=0ms max_anon=85MiB old_kill_at=0.18s held=80 rc=0
    anon96-w64-wbps8M#1: elapsed=7.08s max_measure=130MiB max_dirty+wb=27MiB max_current=130MiB longest_over_bound=214ms new_max=104MiB new_longest=0ms max_anon=100MiB old_kill_at=4.29s held=96 rc=0
    anon0-w512-wbps200M#1: elapsed=2.35s max_measure=128MiB max_dirty+wb=121MiB max_current=128MiB longest_over_bound=61ms new_max=14MiB new_longest=0ms max_anon=11MiB old_kill_at=never held=0 rc=0
    anon0-w256-wbps8M#1: elapsed=27.74s max_measure=77MiB max_dirty+wb=70MiB max_current=128MiB longest_over_bound=0ms new_max=13MiB new_longest=0ms max_anon=10MiB old_kill_at=never held=0 rc=0
    anon80-w96-wbps8M#2: elapsed=10.81s max_measure=128MiB max_dirty+wb=41MiB max_current=128MiB longest_over_bound=194ms new_max=87MiB new_longest=0ms max_anon=84MiB old_kill_at=0.18s held=80 rc=0
    anon96-w64-wbps8M#2: elapsed=6.98s max_measure=129MiB max_dirty+wb=27MiB max_current=130MiB longest_over_bound=227ms new_max=103MiB new_longest=0ms max_anon=100MiB old_kill_at=0.18s held=96 rc=0
    anon0-w512-wbps200M#2: elapsed=2.04s max_measure=129MiB max_dirty+wb=122MiB max_current=129MiB longest_over_bound=83ms new_max=9MiB new_longest=0ms max_anon=6MiB old_kill_at=never held=0 rc=0
    anon0-w256-wbps8M#2: elapsed=27.69s max_measure=62MiB max_dirty+wb=50MiB max_current=128MiB longest_over_bound=0ms new_max=14MiB new_longest=0ms max_anon=11MiB old_kill_at=never held=0 rc=0
    anon80-w96-wbps8M#3: elapsed=10.40s max_measure=129MiB max_dirty+wb=42MiB max_current=129MiB longest_over_bound=195ms new_max=87MiB new_longest=0ms max_anon=84MiB old_kill_at=0.18s held=80 rc=0
    anon96-w64-wbps8M#3: elapsed=6.99s max_measure=129MiB max_dirty+wb=26MiB max_current=129MiB longest_over_bound=358ms new_max=104MiB new_longest=0ms max_anon=100MiB old_kill_at=0.18s held=96 rc=0
    anon0-w512-wbps200M#3: elapsed=2.44s max_measure=129MiB max_dirty+wb=121MiB max_current=129MiB longest_over_bound=61ms new_max=14MiB new_longest=0ms max_anon=11MiB old_kill_at=never held=0 rc=0
    anon0-w256-wbps8M#3: elapsed=27.74s max_measure=57MiB max_dirty+wb=50MiB max_current=128MiB longest_over_bound=0ms new_max=12MiB new_longest=0ms max_anon=9MiB old_kill_at=never held=0 rc=0

Reading: without a heap, the former measure sits at the bound (dirty plus writeback 112-123 MiB) but here never
stays above it for 100 ms (longest 92 ms); the runner's storage made it stay longer. With 80 or 96 MiB held, the
former measure stays above the bound for 194-358 ms in 6 of 6 runs (the old rule kills), while S-11a's measure peaks at
87-104 MiB, never above. Every run completed, and `memory.max` was never reached. This is the `held_heap_slow_storage`
witness. (The sampler computed `new_longest` with the previous sample's clock; that only matters above the bound,
which S-11a's measure never reached.)

### Scripts as measured

`writer_aw.py` (ANON_MIB, WRITE_MIB from the environment):

```python
import os, sys
# Holds ANON MiB of touched anonymous memory, then writes 512 MiB in 1 MiB chunks, no sync.
path, anon = sys.argv[1], int(os.environ.get("ANON_MIB", "96"))
held = bytearray(os.urandom(1 << 20)) * anon
chunk = os.urandom(1 << 20)
fd = os.open(path, os.O_WRONLY | os.O_CREAT | os.O_TRUNC, 0o600)
n = 0
for _ in range(int(os.environ.get("WRITE_MIB","512"))):
    n += os.write(fd, chunk)
os.close(fd)
print(f"wrote={n} held={len(held)>>20}")
```

`sample2.py LABEL OUTFILE [systemd-run property…]` (WRITER = path of the writer):

```python
import os, sys, time, subprocess
# Usage: sample.py LABEL OUTFILE [systemd-run -p props...]
# Runs writer.py in a transient user scope with memory.high=128M, max=192M, swap 0,
# samples the scope's cgroup every 10 ms and computes the watchdog measure of
# internal/sidecar/memory_linux.go unreclaimable(): current - (file - min(shmem+dirty+wb+unevictable, file)).
label, out = sys.argv[1], sys.argv[2]
props = sys.argv[3:]
here = os.path.dirname(os.path.abspath(__file__))
unit = f"dirtyrepro-{os.getpid()}"
cmd = ["systemd-run", "--user", "--scope", "--quiet", f"--unit={unit}",
       "-p", "MemoryHigh=128M", "-p", "MemoryMax=192M", "-p", "MemorySwapMax=0"]
for p in props:
    cmd += ["-p", p]
cmd += ["python3", "-I", os.environ['WRITER'], out]
t0 = time.monotonic()
proc = subprocess.Popen(cmd, stdout=subprocess.PIPE, stderr=subprocess.PIPE, text=True)
cg = None
for _ in range(500):
    r = subprocess.run(["systemctl", "--user", "show", "-p", "ControlGroup", "--value", unit + ".scope"], capture_output=True, text=True)
    if r.stdout.strip():
        cg = "/sys/fs/cgroup" + r.stdout.strip()
        if os.path.exists(cg + "/memory.stat"):
            break
    time.sleep(0.002)
MIB = 1 << 20
BOUND = 128 * MIB
mx = {"measure": 0, "dw": 0, "current": 0}
over_start, longest_over, kill_at = None, 0.0, None
o2, l2 = None, 0.0
while proc.poll() is None:
    try:
        cur = int(open(cg + "/memory.current").read())
        st = dict(l.split() for l in open(cg + "/memory.stat"))
        st = {k: int(v) for k, v in st.items()}
    except (OSError, ValueError):
        break
    f = st["file"]
    kept = st["shmem"] + st["file_dirty"] + st["file_writeback"] + st["unevictable"]
    clean = f - min(kept, f)
    m = cur - min(clean, cur)
    keptn = st["shmem"] + st["unevictable"]
    m2 = cur - min(f - min(keptn, f), cur)
    mx["new"] = max(mx.get("new", 0), m2)
    if m2 > BOUND:
        o2 = o2 or now
        l2 = max(l2, now - o2)
    else:
        o2 = None
    mx["anon"] = max(mx.get("anon",0), st["anon"])
    now = time.monotonic()
    mx["measure"] = max(mx["measure"], m)
    mx["dw"] = max(mx["dw"], st["file_dirty"] + st["file_writeback"])
    mx["current"] = max(mx["current"], cur)
    if m > BOUND:
        over_start = over_start or now
        span = now - over_start
        longest_over = max(longest_over, span)
        if span >= 0.1 and kill_at is None:
            kill_at = now - t0
    else:
        over_start = None
    time.sleep(0.01)
so, se = proc.communicate()
ev = {}
try:
    pass
except Exception:
    pass
el = time.monotonic() - t0
print(f"{label}: elapsed={el:.2f}s max_measure={mx['measure']/MIB:.0f}MiB max_dirty+wb={mx['dw']/MIB:.0f}MiB "
      f"max_current={mx['current']/MIB:.0f}MiB longest_over_bound={longest_over*1000:.0f}ms "
      f"new_max={mx.get('new',0)/MIB:.0f}MiB new_longest={l2*1000:.0f}ms max_anon={mx.get('anon',0)/MIB:.0f}MiB old_kill_at={'%.2fs' % kill_at if kill_at else 'never'} out={so.strip()} rc={proc.returncode}", flush=True)
try:
    os.unlink(out)
except OSError:
    pass
```

## Step 2 (2026-11-24, working tree on 6a73087)

Red first: under the former rule, the new subtests of `rules` failed with « S-11a dirty cache », and
`TestBoundSparesDirtyFileCache/held_heap_slow_storage` was killed (`signal=9 cause=local-limit`) in 5 of 5 runs with
the final helper (anonymous memory kept at 108 MiB, 64 MiB written at 8 MB/s through `io.max`). Under S-11a it
completed in 5 of 5. The step-1 sizing (80 MiB held, 96 MiB written) killed the Go target in 0 of 3 runs, and 96 + 64
in 1 of 3. Under `-race` the run's launcher alone fills the room (127 MiB of anon with nothing written): the subtest
skips there, and `witness` and MD2 run without `-race`.

Each criterion once on the final tree, except `github` (the integrator's): `docs` 0, `rules` 0 (5 s), `witness` 0
(8 s), `mutants` 0 (MD1-MD7 all caught, 26 s), `regression` 0 (473 s: sidecar 392 s, typst 78 s, app 78 s),
`simplify-report` 0.
