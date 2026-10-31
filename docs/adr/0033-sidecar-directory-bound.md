---
status: accepted
decided: 2026-10-31 (utilisatrice)
date: 2026-10-31
---

# F5: kernel bounds for sidecar output

The previous `WatchDir` poller allowed load-dependent overshoot. Linux already
used a user + network namespace for `NoNetwork`, when available; it did not
create a mount namespace or use bubblewrap. The output backend now probes a
user + mount namespace and an actual tmpfs mount independently of networking.

The preferred backend mounts private tmpfs over the empty output directory,
with `size=` rounded **down** to whole pages and bounded `nr_inodes=`. Declared
files are precreated; Landlock permits writing/truncating only those inodes.
The inode quota includes the root plus two units per declared file, since
security labels can consume inode quota too. Namespace capabilities are dropped
before executing the child so it cannot remount the filesystem. RLIMIT_FSIZE
also caps each individual logical size. All limits precede execution and survive
exec/descendants. Unused files do not reserve data pages: pages share the budget.
The caller must keep the directory private and must not mutate it concurrently.

A descriptor pins the mount before exec via a pipe handshake and `/proc/PID/root`.
After exit the supervisor kills the remaining process group, checks all output
entries through `os.Root`, then copies regular files into the original directory.
Empty precreated slots are omitted. Copying has a cumulative byte ceiling as well
as verification, so even a surviving writer cannot cause export overshoot.
Mount lifetime follows the namespace and the pinned descriptor; no host mount
or privileged broker is installed. Paths and filenames are positional arguments,
never shell source. Polling remains only a backstop, not the enforcement mechanism.

## Precise scope of the bound

**tmpfs size limits allocated pages, not the sum of logical file lengths.**
Sparse writes/truncate can create larger logical totals inside the private mount.
Verification rejects such totals before export; copying never exceeds MaxDirBytes.
Thus allocated tmpfs data and exported logical bytes have hard bounds independent
of load, but the aggregate private sparse logical lengths do not. A literal
aggregate logical-size bound during execution would require a different kernel
interface or mediation; tmpfs alone cannot supply it. Partial pages can cause
ENOSPC before the nominal byte budget, and budgets below one page use fallback.
Metadata, swap policy, total I/O over time and arbitrary native-code confinement
are outside this output bound. This ADR remains proposed for that distinction.

## Restricted Linux hosts and other platforms

An actual failed namespace/mount probe selects the fallback, including Ubuntu
24.04 AppArmor restrictions, `kernel.unprivileged_userns_clone=0`,
`user.max_user_namespaces=0`, container policies and missing utilities.
Fallback requires Landlock ABI >= 3 and util-linux `setpriv` with Landlock support
plus `prlimit`; otherwise bounded runs fail closed. Each of N declared files gets
min(floor(MaxDirBytes/N), FileSizeLimit if set), using exact byte hard/soft limits.
Landlock prevents replacing, creating, linking, removing or renaming files and
writing elsewhere. This gives an exact aggregate logical bound, but unused shares
cannot be borrowed. Non-page-aligned fallback limits remain exact. Restrictions
and hard limits are inherited by descendants. Failed runs with a capped file
report OutputLimit; tmpfs exhaustion is checked after failed runs via statfs.

Per [ADR 0003](0003-wails-v3-cgo-native-builds.md), macOS and Windows need native
backends and native CI verification. Neither currently has a hard directory-bound
implementation here: bounded launches are rejected before executing the child.
This is not a portable hard-output guarantee.

## Sources and verification

- Kernel [Documentation/filesystems/tmpfs](https://docs.kernel.org/filesystems/tmpfs.html):
  `size=`, page allocation, `nr_inodes=` and extended-attribute quota accounting.
- [user_namespaces(7)](https://man7.org/linux/man-pages/man7/user_namespaces.7.html)
  and [mount_namespaces(7)](https://man7.org/linux/man-pages/man7/mount_namespaces.7.html):
  unprivileged tmpfs mounts and private mount propagation.
- [Landlock](https://man7.org/linux/man-pages/man7/landlock.7.html),
  [setpriv](https://man7.org/linux/man-pages/man1/setpriv.1.html), and
  [getrlimit](https://man7.org/linux/man-pages/man2/getrlimit.2.html).

Tests exercise deterministic ENOSPC across multiple files (checking the byte
sum after each write), uneven sharing, sparse export verification, and injected
namespace unavailability with hostile fallback writes/descendants. Namespace
tests explicitly skip when the host cannot create user/mount namespaces.
