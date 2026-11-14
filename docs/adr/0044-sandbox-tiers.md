---
status: proposed # revision 2 (2026-11-14), written on measured facts (gates/SB2/MEASUREMENTS.md, plan « Local measurements »); round 1 of review: gates/SB2/REVIEW.md (REJECT, every finding answered here or in the plan); round 2 pending
date: 2026-11-14
amends: ADR 0033 (the namespace probe and its fallback), ADR 0042 A3 (the memory fallback; the cgroup bound needs a PID namespace, unchanged); spec §9.11 (refuse rather than weaken) is kept and made total
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

Not measured yet (the next probe run, questions Q8–Q11 of `scripts/sandbox-probe.sh`): process creation by the
sidecars on arm64 and on the runner's amd64, the namespace-free tier on a stock runner, the calls refused inside the
sandbox on arm64. A real Ubuntu 24.04 **desktop** (kernel 6.8, Landlock ABI 4 according to the kernel documentation's
ABI table — not measured) is the owner's check at the end.

## Decision

### 1. Tiers

A launch runs in exactly one of two tiers, or is refused.

| | **N — namespaces** | **F — namespace-free** |
|---|---|---|
| Namespaces | user, mount, PID, IPC; network when `NoNetwork`; `/proc` remounted as PID 1 | none |
| Files | private mount namespace, output on a private tmpfs sized to the budget, IPC folders hidden; Landlock on top | Landlock only: read the listed paths; write only the output slots created beforehand, `RLIMIT_FSIZE` on each; **execute only the target and the files its ELF interpreter needs** (today `readAccess` grants execute on every read path, `launcher_linux.go:422`) |
| Processes | the sidecar's children live in its PID namespace and die with it | **no process creation**: seccomp allows `clone` only with `CLONE_THREAD`, answers `clone3` ENOSYS (the C library then uses `clone`; Go never uses `clone3` for threads), denies fork and vfork; `PR_SET_PDEATHSIG` SIGKILL before the target starts, so the sidecar never outlives its parent |
| Other processes | not visible (PID namespace) | their **content** is not readable (Landlock grants `/proc/self` and `/proc/thread-self` only, `launcher_linux.go:437`); their **existence and metadata** stay visible to `stat` and `sched_getaffinity` (residual R-3); signals: `kill`, `tkill`, `rt_sigqueueinfo`, `pidfd_*` are not allowed, `tgkill` only to its own pid (exact here, since the process cannot fork), Landlock signal scoping when ABI ≥ 6 |
| Resource limits of others | n/a | `prlimit64` allowed only with pid 0 (rule S-1, both tiers) |
| Network | private network namespace when `NoNetwork`; seccomp denies `socket` in all cases | seccomp denies `socket`, so no TCP, UDP, pathname or abstract UNIX socket; only `socketpair(AF_UNIX, SOCK_STREAM)` for the runtime's self-pipe; Landlock TCP rules when ABI ≥ 4, UDP when ABI ≥ 10, abstract-socket scoping when ABI ≥ 6 |
| Memory | cgroup scope when one is delegated **and** a PID namespace exists (ADR 0042, `scope_linux.go:158-160`), else `RLIMIT_AS` per process | `RLIMIT_AS` only; with no descendant possible it bounds the whole job |
| Always | `no_new_privs`, capabilities dropped, the seccomp allow-list, Landlock (ABI ≥ 3), the caller's deadline | same |

**Refuse** when: Landlock is unavailable or its ABI is below 3; seccomp cannot be installed; neither tier's launch
succeeds; a launch's own verification (point 3) disagrees with the decided tier.

### 2. One decision, by launching each tier for real

`decideTier(facts) (Tier, error)` is pure and total. The facts come from one probe sequence, run once per process and
cached once conclusive, each step bounded by `probeTimeout` (5 s):

| Fact | Obtained by | Values |
|---|---|---|
| `landlock` | `landlock_create_ruleset(VERSION)` | ABI number, or unavailable |
| `seccomp` | installing the real filter in the probe child | ok, unavailable |
| `ns` | **the real N launch** of a no-op target: the same code path, flags, id maps, tmpfs, IPC overlay, `/proc` mount as PID 1, Landlock, seccomp and `execve` as every N launch (`NoNetwork` set, output bounded: the strictest variant, and the only one N uses, see point 3) | ok · refused · inconclusive |
| `f` | **the real F launch** of the same no-op target | ok · failed · inconclusive |

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

`probeOutputMount` and `probeIsolation` are replaced by this sequence; `NetworkIsolation()` reports the tier.

### 3. One configuration per tier, verified at every launch

- **N** always uses the full set: every namespace including network, the private tmpfs, the IPC overlay. A launch that
  does not need a network barrier or a bounded folder gets them anyway (the probe proved exactly this set). The
  variants `run.go:368-369` and `mount_linux.go:153-155` allow today disappear. **F** has one configuration.
- **Before the document reaches the target**: the launcher writes to its report descriptor the tier it actually
  established (namespaces present, `/proc` remounted, seccomp mode, Landlock ABI used) after every confinement step,
  then waits — with a deadline — for the parent's « proceed » before `execve(target)`. The parent compares with the
  decided tier; on any difference it kills the launcher and returns an error naming both. The target is the only
  process that reads the project, and it is not started until the check passes.
- A launch failure in a tier the probe accepted is an error with its cause, never a silent retry in another tier.
  The memory fallback (cgroup → rlimit, within N) creates the output slots once, after the bound is settled
  (finding 4).

### 4. Nothing waits without a deadline

Every blocking call of `Run`, of the probe and of the scope code is bound to the caller's context or to a named
constant: the pidfd wait in `captureDeath` (poll on the pidfd with the context), the report, ready and proceed pipes,
the D-Bus calls, the cgroup removal. Measured on the stock runner, the two tests that « never returned » fail at once
there; the hang seen earlier came with the restriction lifted, and `TestCgroupWorkloadAndDescendantsInside`'s wait
cycle (Codex, round 1, M2) is a defect of that test.

### 5. What a hostile document can do in tier F, and why it is accepted

Assume the document turns typst or tinymist into arbitrary code. In F it can: read the listed paths; use the CPU until
the caller's deadline; allocate up to `RLIMIT_AS` (the whole job: it cannot fork); write the output slots up to their
budget; `execve` only the target itself or its interpreter, under the same filter and ruleset; learn that other pids
exist and their CPU masks; reach the kernel through the allowed calls in the host's namespaces. It cannot: open any
socket; read another process's memory, environment or files under `/proc`; signal, trace or re-limit another
process; create a process; outlive inkwell; write outside its slots. Accepted, because the alternative on a stock
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
is not measured). The profile is unconfined: it adds the right to create user namespaces and nothing else, the right
Ubuntu grants to the browsers and editors it ships profiles for; inkwell then confines its own children itself.

### 7. What the person is told

The About dialog's existing line follows the tier (one fact, one place). A sentence appears in the preview and the
problems list only when a launch is **refused**; the reduced tier raises no toast (the person can do nothing about it).

| Outcome | Key | Sentence |
|---|---|---|
| N | `about.network.namespace` (kept) | « La mise en page tourne dans un espace isolé : elle ne peut joindre aucun réseau, pas même cet ordinateur. » |
| F | `about.isolation.reduced` (replaces `about.network.proxy`) | « La mise en page tourne avec une isolation réduite : elle ne peut ni joindre le réseau, ni lancer d'autre programme, ni écrire hors du document, mais ce système ne permet pas de la séparer complètement des autres programmes. » |
| refuse | `preview.fail.sandbox`, action `preview.fail.sandbox.details` « Détails » (opens About) | « La mise en page ne peut pas démarrer : ce système ne fournit pas la protection dont Inkwell a besoin. » |

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
| bubblewrap, a setuid helper, Typst as WebAssembly | bubblewrap needs the same allowance; setuid is a larger risk; WebAssembly is block SW1, measured first (W70) |

## Consequences

- Good: Ubuntu 24.04 works packaged (N) and unpackaged (F); one decision; no hang; the person reads the truth; the
  stock runner becomes a permanent proof.
- Bad: two tiers to test for ever; an Ubuntu-only packaging file; F's kernel surface; N always pays for the network
  namespace and the tmpfs even where a run does not need them.

## Residuals

- R-1: aggregate memory without a cgroup in N (ADR 0042, unchanged); not a residual in F (one process).
- R-2: on Landlock ABI 3–5, signal and abstract-socket scoping come from seccomp alone (sockets denied, `tgkill` to self).
- R-3: other processes' existence and CPU masks are visible in F.
- R-4: arm64's allow-list gaps (`ppoll` measured as called at start; inside the sandbox not yet traced): plan, SB2a.
- R-5: AppImage or Flatpak packaging would need its own answer; a desktop session differs from the runner (owner's check).

## Sources

- Measured: `gates/SB2/MEASUREMENTS.md` and `PROBE-ubuntu-24.04-{amd64,arm64}.md` (run 43638847261); the plan's
  « Local measurements » (this machine, 2026-11-14).
- Ubuntu, « Restricted unprivileged user namespaces are coming to Ubuntu 23.10 » (read 2026-11-14):
  <https://ubuntu.com/blog/ubuntu-23-10-restricted-unprivileged-user-namespaces>. The flag spelled there,
  `default_allow`, is not what 24.04 ships: the measured profiles use `unconfined`.
- Kernel documentation, Landlock (read 2026-11-14): ABI 3 truncate, 4 TCP, 5 ioctl on devices, 6 signal and abstract
  UNIX socket scoping, 9 pathname UNIX sockets, 10 UDP; <https://docs.kernel.org/userspace-api/landlock.html>.
  The kernel release of each ABI comes from the man-pages Landlock table (Codex, round 1): ABI 4 = Linux 6.7, ABI 6 =
  Linux 6.12; Ubuntu 24.04's GA kernel 6.8 therefore has ABI 4 (not measured on a desktop).
- `prctl(2)`, `PR_SET_PDEATHSIG`: kept across `execve` of a non-set-user-ID binary; `clone(2)`: `CLONE_THREAD`;
  `golang.org/x/sys/unix` `zsysnum_linux_{amd64,arm64}.go` (read locally): `SYS_PRLIMIT64` 302/261, `SYS_SETRLIMIT`
  160/164, `SYS_PPOLL` 271/73, `SYS_CLONE3` 435/435; arm64 has no `fork` or `vfork` system call.
