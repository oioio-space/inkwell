---
status: proposed # draft for block SB2a (plan: docs/plans/2026-11-14-sb2-sandbox-tiers.md); not reviewed yet
date: 2026-11-14
amends: ADR 0033 (the namespace probe and its fallback), ADR 0042 A3 (which fallback is named); spec §9.11 (refuse rather than weaken) is kept and made total
decision-makers: [Mathieu]
---

# The sidecar sandbox has explicit tiers, decided once by launching each tier for real; a machine without usable namespaces gets the Landlock tier, said to the person, never a hang or a bare status 125

Requirements: W69 (inkwell works on a stock Ubuntu 24.04, an R1 criterion), W68 (cloud review of 2026-11-14,
`docs/reviews/2026-11-14-ultrareview-sandbox-bridge.md`). Diagnosis: `docs/plans/2026-11-14-ubuntu-2404-sandbox.md`.
GitHub evidence: `gates/CI1c/GITHUB.md`. Rigour: **complete** — the compiler runs on documents written by peers, so
this is the boundary the trust rule keeps (`docs/engineering-principles.md` §4, « Frontière de confiance »).

## Context and problem

`internal/sidecar` confines `typst` and `tinymist` with user, mount, PID, IPC and network namespaces, a private
tmpfs for the output, Landlock, seccomp, rlimits and a systemd user scope. Its stated policy is already to degrade on
a definitive refusal and to refuse otherwise: « Spec §9.11, refuse rather than weaken: only a DEFINITIVE negative
result selects the launch without namespaces » (`mount_linux.go:26-35`); « An actual failed namespace/mount probe
selects the fallback, including Ubuntu… » (ADR 0033); the memory fallback is « never an error and never a weaker
bound » (ADR 0042, point 2); Landlock and seccomp are mandatory (« mandatory Landlock unavailable; refusing peer
document », `launcher_linux.go:138`, `:152`, `:205`). What fails is the mechanism, read in the code on `main` e256ea6:

1. **Two probes that test different things.** `probeOutputMount` runs the child with `Overlay` and a tmpfs mount
   (`mount_linux.go:49-60`); `probeIsolation` runs it with `Probe: true` only (`netns_linux.go:34-40`), and
   `sandboxExec` returns at `if c.Probe` (`launcher_linux.go:104-106`) **before** the `/proc` mount that a real
   namespaced launch performs as PID 1 (`:110-119`). Where a user namespace can be created but holds no capability,
   the first says no and the second yes; the launch then keeps the caller's uid (`isolate` maps the uid onto itself
   when the mount namespace did not set the root mapping, `netns_linux.go:22-25`), fails to mount `/proc` and exits
   125. Confirmed as a code path; not yet observed on a real Ubuntu 24.04 desktop.
2. **An errno without a decision.** `definitiveRefusal` knows EPERM, EINVAL and ENOSPC-at-limit-0
   (`mount_linux.go:64-70`). The arm64 GitHub runner answered « fork/exec /proc/self/exe: permission denied »
   (EACCES, run 43620631116): inconclusive for ever, every launch refused for two weeks.
3. **Waits without a deadline.** `captureDeath` blocks in `waitid` on the pidfd (`death_linux.go:72-83`), called
   before `cmd.Wait` (`run.go:229`); on the amd64 GitHub runner `TestCgroupWorkloadAndDescendantsInside` and
   `TestRunNoNetworkFallsBack` never return (run 43629366160, package killed after 8 min). Cause not established.
4. **The fallback of the fallback cannot run** (review finding 4, confirmed in code): a refused cgroup placement
   re-runs `run` (`run.go:213-217`), but the Landlock path has already created the output slots in the watched
   folder (`dir_linux.go:43-51`) and refuses a folder that is not empty (`:36-39`).
5. **A pid argument that nothing checks** (finding 2, confirmed in code): the filter refuses `prlimit64` only for
   `RLIMIT_CORE` with a new limit (`seccomp_linux.go:143-144`); for every other resource the pid argument is free.
   Without a PID namespace the target can lower the limits of any process of the same user, Inkwell included. It is
   the only pid-taking call of the allow-list that writes (`sched_getaffinity` reads; `tgkill` is already bound to
   self, `seccomp_linux.go:104-110`).
6. **Nothing tells the person.** The About dialog has two sentences (`about.network.namespace`,
   `about.network.proxy`), fed by `sidecar.NetworkIsolation()` (`internal/typst/engine.go:249`): the second probe
   only. A refused launch surfaces as « exited with status 125 »; tinymist's failure is a log line.
7. **No test runs where the problem is**: every CI job that starts a sidecar lifts
   `kernel.apparmor_restrict_unprivileged_userns` first; the package tests start no sidecar; the .deb ships no
   AppArmor profile (`build/linux/nfpm/scripts/postinstall.sh` only refreshes desktop caches).

## Decision

### 1. Tiers

A launch runs in exactly one tier, the pair of an isolation level and a memory bound:

| | **Namespaces** | **Landlock** (no namespace) |
|---|---|---|
| Files | private mount namespace; output on a private tmpfs sized to the budget; IPC folders hidden; Landlock on top | Landlock only: read the project, the fonts and the listed system files; write only the output slots created beforehand, each capped by `RLIMIT_FSIZE` (budget shared between slots) |
| Other processes | private PID namespace (`/proc` remounted): none visible | visible to the kernel, not to the target: Landlock denies `/proc` outside `self`; seccomp denies `kill`, `tkill`, `ptrace`, `process_vm_*`, `pidfd_*`; `tgkill` and **`prlimit64` only on itself** (rule S-1); Landlock signal scoping where ABI ≥ 6 |
| Network | private network namespace when `NoNetwork`; seccomp denies every `socket` in all cases | seccomp denies every `socket`, `connect`, `bind`, `send*`; Landlock TCP rules where ABI ≥ 4; no private namespace (`Output.Isolated` false) |
| IPC | private IPC namespace; SysV and POSIX queues denied by seccomp | SysV and POSIX queues denied by seccomp; abstract UNIX sockets need `socket`, denied |
| Privileges | `no_new_privs`, capabilities dropped, bounding set emptied inside the namespace | `no_new_privs`, capabilities dropped |

| Memory bound | When | What it bounds |
|---|---|---|
| **cgroup** | a delegated systemd user scope was created and verified (ADR 0042 A3) | every process of the run and its tmpfs output, in aggregate; `RLIMIT_AS` still set |
| **rlimit** | anything else | each process's address space (`RLIMIT_AS`); no aggregate bound (ADR 0042, accepted) |

Both features (the Typst compile; tinymist with `NoNetwork`) run in all four tiers. **Refuse** is the fifth outcome.

### 2. One decision, made by launching each tier for real

`decideTier(facts) (Tier, error)` is a pure, total function; the facts come from one probe sequence, run once and
cached, each step bounded by `probeTimeout`:

| Fact | How it is obtained | Values |
|---|---|---|
| `landlockABI` | `landlock_create_ruleset(VERSION)` | integer; `< 3` or an error is « unusable » |
| `seccomp` | the filter is built and installed in a probe child | usable, unusable |
| `namespaces` | **the real namespaced launch** of a no-op target: same clone flags and id maps as a `NoNetwork` run with a bounded folder, the tmpfs, the IPC overlay, the `/proc` mount as PID 1, Landlock, seccomp, then `execve` | ok, refused (definitive), inconclusive |
| `plain` | the real Landlock-tier launch of the same no-op target (no clone flag) | ok, failed, inconclusive |
| `cgroup` | the scope session of ADR 0042 (S1–S9), under the caller's context | delegated, not delegated |

| `landlockABI` | `seccomp` | `namespaces` | `plain` | Tier |
|---|---|---|---|---|
| unusable | any | any | any | **refuse**: « mandatory Landlock unavailable » |
| usable | unusable | any | any | **refuse**: « mandatory seccomp unavailable » |
| usable | usable | ok | any | **Namespaces** |
| usable | usable | refused | ok | **Landlock** |
| usable | usable | refused | failed | **refuse**: the launcher itself cannot run here (noexec, a policy on the binary); the error names it |
| usable | usable | inconclusive | any | **refuse, retry** (`ErrProbeInconclusive`, backoff as today); nothing cached |
| usable | usable | refused | inconclusive | **refuse, retry** |

The memory bound is chosen independently (cgroup when delegated, else rlimit) and may change from run to run as today.

- **« Refused » is total over errno.** A failure of `clone`, of the id-map setup or of a mount in the probe child is
  definitive when it is EPERM, **EACCES**, EINVAL, or ENOSPC with `user.max_user_namespaces` = 0; the child reports a
  mount refusal with its own status (120, as today). EACCES is safe to count because the `plain` step tells a
  namespace refusal from a launcher that cannot be executed at all (row 5). Any other error, a timeout or a crash is
  inconclusive.
- **The launch uses the decided tier and nothing else.** `Spec.isolated()`, `configureConfinement`,
  `newDirectoryMount` and `directoryLimits` read the cached tier; `probeOutputMount` and `probeIsolation` disappear.
  A launch that then fails in a way the probe said could not happen is an error with its cause, never a silent retry
  in another tier.
- **Nothing waits without a deadline.** Every blocking call of `Run` and of the scope code is bound to the caller's
  context or to a named constant: the pidfd wait of `captureDeath` (poll on the pidfd with the context), the ready and
  proceed pipes of the mount launcher, the D-Bus calls, the cgroup removal. A test starts a target that never ends
  and never answers, cancels the context and requires `Run` to return within the deadline in each tier.
- **The memory fallback can run on the Landlock path** (finding 4): the output slots are created once per `Run`,
  after the tier and the memory bound are settled, or removed before the second attempt.

### 3. What a hostile document can do in the Landlock tier, and why it is accepted

It can read what every tier lets it read (the project folder, the fonts, the listed system files), use the processor
until the caller's timeout, and allocate up to `RLIMIT_AS` in each process it forks (no aggregate bound without a
cgroup: the same residual as the Namespaces tier with the rlimit bound, ADR 0042). It cannot write outside the output
slots, open a socket, signal, trace or re-limit another process (rule S-1 closes the last one), nor see another
process through `/proc`. What it keeps over the Namespaces tier is the kernel's attack surface behind the allowed
calls **in the host's namespaces**: a kernel bug reachable through the 71 allowed calls is not further contained.
That is the level of Landlock-plus-seccomp sandboxes used where namespaces are unavailable, and refusing instead would
leave Ubuntu 24.04, its derivatives and every non-packaged run without preview, export or completion. Accepted,
stated to the person, and removed for packaged installs by point 4.

### 4. Ubuntu: the package allows the namespaces

The .deb installs `/etc/apparmor.d/inkwell`:

```
abi <abi/4.0>,
include <tunables/global>

profile inkwell /usr/bin/inkwell flags=(unconfined) {
  userns,

  include if exists <local/inkwell>
}
```

It is the form Ubuntu documents for applications that need unprivileged user namespaces (an otherwise unconfined
profile carrying the `userns,` rule; the 23.10 announcement spells the flag `default_allow`, AppArmor 4.0 on 24.04
`unconfined` — **to verify against a profile shipped by the 24.04 `apparmor` package before the freeze**). nfpm lists
it as a configuration file of the deb only; `postinstall.sh` loads it when `apparmor_parser` exists and AppArmor is
enabled (`apparmor_parser -r -T -W`), `postremove.sh` unloads it; neither can fail the installation. The launcher
re-executes `/proc/self/exe`, which resolves to the same `/usr/bin/inkwell`: whether the child stays under the
profile, so that its `clone(CLONE_NEWUSER)` is allowed, **is not known** and is settled by experiment E-2 (plan). If
it is not, the profile gains an explicit transition rule or the packaged install stays in the Landlock tier, said so.
A binary run from anywhere else (a build, an archive in the home folder) matches no profile: Landlock tier. The .rpm
ships nothing: Fedora's SELinux policy does not restrict user namespaces for unconfined users (unverified here; the
`sidecars` tests of the package run on Fedora decide).

### 5. What the person is told

One fact, one place (study W63, P12): the About dialog's existing line, now fed by the decided tier.

| Tier | Key | Sentence |
|---|---|---|
| Namespaces | `about.network.namespace` (kept) | « La mise en page tourne dans un espace isolé : elle ne peut joindre aucun réseau, pas même cet ordinateur. » |
| Landlock | `about.network.proxy` → renamed `about.isolation.reduced` | « La mise en page tourne avec une isolation réduite : elle ne peut ni joindre le réseau ni écrire hors du document, mais ce système ne permet pas de la séparer des autres programmes. » |
| refuse | `preview.fail.sandbox` (new), in the preview's failure state and for completion | « La mise en page ne peut pas démarrer : ce système ne fournit pas la protection dont Inkwell a besoin. » with the action « Détails » (opens About) |

No toast for the reduced tier: the person can do nothing at that moment, and the state does not change during a
session. The refusal is shown where the work fails (the preview, once), not as a generic status 125; tinymist's
refusal uses the same sentence in the problems popover instead of a log line.

## Considered options

| Option | Verdict |
|---|---|
| **Explicit tiers decided by real launches, plus the AppArmor profile** | chosen: the decision is total, the proof is the launch itself, packaged Ubuntu gets the strong tier, everything else works in the reduced one |
| AppArmor profile only | a build, an AppImage or a home-folder archive stays refused; the probe disagreement and the EACCES gap remain |
| Landlock tier only | works everywhere but gives packaged Ubuntu less than the kernel allows |
| Fix the two probes separately (make `probeIsolation` mount `/proc`, add EACCES) | the smallest patch, and the third of its kind: two probes can disagree again at the next kernel or policy change (golden rule) |
| Ask the person to set `kernel.apparmor_restrict_unprivileged_userns=0` | weakens the whole machine for one application; against « nothing to install » |
| bubblewrap or a setuid helper | bubblewrap needs the same AppArmor allowance; a setuid binary is a larger risk than the one removed |

## Consequences

- Good: Ubuntu 24.04 works packaged (strong tier, pending E-2) and unpackaged (reduced tier); no hang; one probe;
  the person reads the truth; CI runs the sandbox where it used to be switched off.
- Bad: a second isolation level to test for ever (tier-aware tests, a CI job without the sysctl); a packaging file
  that only Ubuntu-family systems use; the reduced tier's kernel surface.

## Residuals

- R-1: aggregate memory in the rlimit bound (ADR 0042, unchanged).
- R-2: the Landlock tier on kernels with Landlock ABI 3 has no Landlock network rule; seccomp alone denies sockets.
- R-3: `sched_getaffinity` on another pid stays allowed (reads a CPU mask).
- R-4: arm64 (`setrlimit` never filtered, `ppoll` missing from the allow-list, no arm64 trace): block SB2b.
- R-5: an AppImage or Flatpak packaging would need its own answer.

## Sources

- Ubuntu, « Restricted unprivileged user namespaces are coming to Ubuntu 23.10 » (read 2026-11-14): unconfined
  processes may create user namespaces only with the `userns,` rule or CAP_SYS_ADMIN; sysctls
  `kernel.apparmor_restrict_unprivileged_userns`, `…_unconfined`; example profile `abi <abi/4.0>, /path
  flags=(default_allow) { userns, include if exists <local/…> }` in `/etc/apparmor.d/`.
  <https://ubuntu.com/blog/ubuntu-23-10-restricted-unprivileged-user-namespaces>
- Kernel documentation, Landlock (read 2026-11-14): ABI 3 truncate, ABI 4 TCP bind and connect, ABI 6 scoping of
  signals and abstract UNIX sockets, ABI 9 pathname UNIX sockets, ABI 10 UDP; « chroot(2) calls are not denied »;
  Landlock first appeared in Linux 5.13. The kernel release of each ABI is not on that page: Ubuntu 24.04's 6.8 kernel
  is expected at ABI 4 (**unverified**; the probe reads the ABI, the design does not depend on it).
  <https://docs.kernel.org/userspace-api/landlock.html>
- `golang.org/x/sys/unix`, `zsysnum_linux_{amd64,arm64}.go` (read locally): `SYS_PRLIMIT64` 302 / 261,
  `SYS_SETRLIMIT` 160 / 164, `SYS_PPOLL` 271 / 73.
- GitHub runs 43620631116, 43624790761, 43629366160 (`gates/CI1c/GITHUB.md`): on `ubuntu-24.04-arm` a launch with
  `CLONE_NEWUSER` fails with EACCES until `kernel.apparmor_restrict_unprivileged_userns=0`, so the restriction is on
  by default there (measured); for the amd64 image it is expected and **not measured** (experiment E-1).
- Not read: the profiles shipped by Ubuntu 24.04's `apparmor` package, `dh_apparmor`, AppArmor's rule for a
  re-execution of the same binary, Fedora's SELinux policy on user namespaces, how bubblewrap and Chromium fall back.
