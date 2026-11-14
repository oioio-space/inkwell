# inkwell on stock Ubuntu 24.04: sandbox diagnosis and options

Read-only investigation, 2026-11-14. Nothing in the repository was modified. "Verified" = read in the code or run on this machine
(Fedora 45, kernel 7.3, no AppArmor). "Unverified" = from a secondary source or reasoning; no Ubuntu 24.04 host was available.

## Part 1 - the code

### 1.1 Launch trace (all paths in `internal/sidecar/`)

Every Run goes through `run()` (`run.go:149`):

1. `launchCommand` -> `withLimits` (`limit_linux.go:4`) or `directoryLimits` (`dir_linux.go:15`) call `probeOutputMount()` (`mount_linux.go:49`).
   This is probe #1: re-exec `/proc/self/exe` with `CLONE_NEWUSER|NEWNS|NEWPID|NEWIPC`, uid map `0 -> caller` (`mountNamespace`, `mount_linux.go:339`),
   child does `mount / MS_PRIVATE` + tmpfs mounts (`mountOutput` / `mountIPC`, `launcher_linux.go:236,273`).
2. `configureRunCommand` (`run.go:283`): `configureConfinement` (`confinement_linux.go:6`) adds `mountNamespace` only if probe #1 is true.
   Then `spec.isolated()` (`run.go:367`) -> probe #2 `probeIsolation` (`netns_linux.go:34`) only when `NoNetwork`. If true, `isolate(cmd)` adds
   `NEWUSER|NEWNET|NEWPID|NEWIPC|NEWNS` with an IDENTITY uid map (`netns_linux.go:14-26`; it returns early only if `UidMappings` is already set by probe #1's `mountNamespace`).
3. Child side: `init()` in `launcher_linux.go:47` -> `sandboxExec` (`:76`). If `Getpid()==1` (PID namespace) it mounts a fresh `/proc` (`:113-123`) and re-execs itself as
   the sandbox child; otherwise it sets no_new_privs, drops capabilities (`:293`), applies Landlock (mandatory, `restrictOutput`, `:359`), then `limitAndExec` -> rlimits + seccomp
   (mandatory) + execve of typst/tinymist.
4. Probe outcome rules (comment `mount_linux.go:22-40`, spec §9.11): only a DEFINITIVE refusal is cached as "no namespaces". Definitive =
   `definitiveRefusal` (`:66`): EPERM or EINVAL from clone/id-map, ENOSPC with `max_user_namespaces==0`, or the probe child exiting 120 after its mounts got EPERM/EINVAL.
   Anything else (timeout, crash, EAGAIN, **EACCES**) = `ErrProbeInconclusive` ("namespace probe inconclusive; launch refused, retry", `store.go:52`), nothing cached,
   re-probed with a backoff of 1 s doubling to 30 s (`cachedProbe`, `:116-143`).
5. Fallback when both probes are definitively false: `withLimits` with `Overlay:false`, no namespaces. The child keeps no_new_privs, dropped caps, Landlock, seccomp, RLIMIT_AS/FSIZE/CORE.
   Lost: user/mount/PID/IPC/net namespaces, the tmpfs output bound (ADR 0033 uses a Landlock + rlimit fallback instead, `dir_linux.go`), the cgroup scope (ADR 0042 line ~410: no PID ns => rlimit
   containment, never cgroup), the `/tmp/.X11-unix` and runtime-dir IPC overlay. Typst then runs "proxy only" (`typst/engine.go:249-256`); seccomp still forbids socket creation (`seccomp-policy.md:18`).

Stated policy (ADR 0033 "Restricted Linux hosts", `mount_linux.go` comment, ADR 0042): fail closed on the two mandatory layers (Landlock, seccomp: "refusing peer document"), but DEGRADE
(drop the namespaces) on an *actual, definitive* namespace refusal, naming Ubuntu 24.04 AppArmor explicitly. So the design intends "stock 24.04 => Landlock+seccomp+rlimits".
ADR 0033 says the fallback requires Landlock ABI >= 3 (+ `setpriv`/`prlimit` in the ADR text; the code uses its own launcher, not setpriv).

### 1.2 What actually happens on stock 24.04 (default `apparmor_restrict_unprivileged_userns=1`, binary unconfined, installed at `/usr/bin/inkwell`)

From Ubuntu's mechanism (Part 2): `clone(CLONE_NEWUSER)` SUCCEEDS, the child is moved into the `unprivileged_userns` profile which denies capabilities, so the first
privileged op fails EPERM. Tracing that into the code:

- Probe #1: clone OK, child's `mount("/", MS_REC|MS_PRIVATE)` -> EPERM -> exit 120 -> `(false, nil)`: definitive "no mount namespace", cached. (Unverified on a real host, follows from the code + Ubuntu docs.)
- Probe #2 (only when `NoNetwork`, i.e. tinymist always, `language.go:117`, and typst when it believes it is isolated, `engine.go:491`): the isolation probe runs `sandboxExec` with
  `Probe:true`, which returns at `launcher_linux.go:107` BEFORE the PID-1 `/proc` mount at `:113`. clone + exec of an identity-mapped child succeed -> probe #2 = TRUE ("isolation supported").
  The two probes DISAGREE.
- Real launch then = `isolate(cmd)` WITHOUT the uid-0 map (probe #1 false, so `mountNamespace` is not applied): child is PID 1 of a new pid ns, uid != 0 in a user ns.
  After exec it has no capabilities (VERIFIED here, no AppArmor needed: `unshare -U -n -p -m -f --map-user=$UID --map-group=$GID -- sh -c 'grep Cap /proc/self/status; mount -t proc proc /proc'`
  prints CapEff 0 and "must be superuser to use mount"). So `unix.Mount("proc", ...)` at `launcher_linux.go:117` fails EPERM, `init()` prints
  "sidecar: sandbox: operation not permitted" and exits 125. Reviewer claim (b) is therefore CONFIRMED as a code path, with a sharper cause: it needs the probe disagreement
  (mount probe false, isolation probe true), which AppArmor's "create allowed, privileges denied" behaviour produces. It is not specific to 23.10+ PID-1 capabilities only; the same exit-125 happens on ANY host where probe #1 is false and #2 true.
- No test covers (mount probe false, isolation probe true): tests stub only `isolationProbe=false` (`launcher_linux_test.go:433`, `limits_test.go:142`) or `probeOutputMount` alone (`mount_linux_test.go:188`).
  Existing tests skip when the host denies namespaces (`mount_linux_test.go:33`), so an AppArmor host just skips them.
- Result for the person: tinymist (completion, diagnostics, hover) dies with exit 125; the only trace is `slog.Warn("language sidecar stopped", ...)` (`language.go:119`), no UI message
  (silent loss of completion). Typst (`noNet` true because probe #2 true) exits 125 on every compile: reported as the generic `fmt.Errorf("typst: the compiler exited with status %d: %s")` (`engine.go:696`)
  with stderr "sidecar: sandbox: operation not permitted"; preview and PDF export would show a compile failure, not the clean `preview.fail.unavailable` text. (Unverified: exact UI rendering of that generic error.)
- The refusal path that does give a clean message: `typst.New` returns the probe error (`engine.go:249`), `preview.go:205-218` retries each preview, and the UI shows
  `preview.fail.unavailable` ("The layout engine does not start. Restart Inkwell; if it persists, reinstall it.", `frontend/src/i18n/en.json:449`; export: `export.unavailable`, `:469`). The text blames
  the install and gives no hint about AppArmor.

### 1.3 The GitHub failure (a): "fork/exec /proc/self/exe: permission denied"

"permission denied" is EACCES, "operation not permitted" is EPERM. `definitiveRefusal` only accepts EPERM/EINVAL/ENOSPC, so this is inconclusive => refused for ever (retried every 30 s). Possible sources (unverified, no runner log):
- Ubuntu's kernel logs, when the `unprivileged_userns` profile is NOT loaded: `apparmor="DENIED" operation="userns_create" info="Userns create restricted - failed to find unprivileged_userns profile" error=-13` (-13 = EACCES). Minimal images, containers
  and runners without that profile loaded get this (trackr.live; Incus thread: "namespace needs a profile unprivileged_userns").
- bwrap on 24.04 reports "setting up uid map: Permission denied" (etbe.coker.com.au), so an id-map write can also yield EACCES.
Either way the intended "definitive refusal -> fallback" never triggers, and the app fails closed instead of degrading. Contrast: rootlesskit shows the same Go pattern with EPERM ("fork/exec /proc/self/exe: operation not permitted", rootlesscontaine.rs).
So on a stock desktop two failure modes exist: EPERM-after-clone (exit 125 as in 1.2) and EACCES at clone (permanent "inconclusive" refusal). Which one a real 24.04 desktop gives is unverified; both are bad.
Note CI hides both: `ci.yml:69-72`, `:115-117`, `:174-176` lift the sysctl in ALL three jobs that run the sandbox (even the amd64 job), and `release.yml` pkg tests run in containers.

### 1.4 PKG-01..11 (what they really cover)

`scripts/pkgtest/run.sh` + `inside.sh`, `mise run pkg:test` (`mise.toml:268`): podman containers `debian:13`, `ubuntu:24.04`, `fedora:44`; phases tools/install/smoke/uninstall/upgrade.
- smoke (`inside.sh:179-228`) starts the installed app under Xvfb with `--network none`, checks the window, WebKitWebProcess, data folder, a colour count, no errors in the log, SIGTERM. It does NOT open a `.typ`, does NOT compile, does NOT start typst or tinymist (PKG-05 "sidecar tampered" and the plan's "compile a PDF offline" in PKG-01/04 are not exercised by this script; the plan table at `docs/plans/2026-10-25-t11-interface.md:1008-1011` is wider than the script).
- A container shares the HOST kernel and LSM. Locally that is Fedora 45 (SELinux, no AppArmor); in `release.yml:208-216` it is a root podman on a GitHub runner whose kernel does have the Ubuntu AppArmor setup, but the sidecar is never started, so nothing would notice. The "ubuntu:24.04" container only provides userspace.
=> PKG-01..11 passing on 24.04 says nothing about the sandbox. Evidence (c) does not contradict (a)/(b).

### 1.5 Packaging: AppArmor/SELinux

Nothing. `build/linux/nfpm/scripts/` holds only `postinstall.sh` (desktop/mime/icon caches) and `postremove.sh`; no `/etc/apparmor.d/*`, no `apparmor_parser`, no SELinux module, nothing in `.goreleaser.yaml` for them (grep for apparmor/selinux across non-worktree files finds only comments, the test skip, netsim and pkgtest remarks).

## Part 2 - facts from primary sources

Sources fetched: Ubuntu blog "Ubuntu 23.10 restricted unprivileged user namespaces" (ubuntu.com/blog/ubuntu-23-10-restricted-unprivileged-user-namespaces); discourse.ubuntu.com/t/understanding-apparmor-user-namespace-restriction/58007;
AppArmor wiki `unprivileged_userns_restriction` (gitlab.com/apparmor/apparmor/-/wikis/...); Ubuntu 25.04 release notes (discourse 48687); Ubuntu server AppArmor docs; kernel docs.kernel.org/userspace-api/landlock.html;
trackr.live/?p=1157; etbe.coker.com.au/2024/04/24/ubuntu-24-04-bubblewrap; rootlesscontaine.rs AppArmor page; microsoft/vscode#316046; Debian/Fedora via search results only.

- Mechanism (AppArmor wiki, trackr.live): with `kernel.apparmor_restrict_unprivileged_userns=1` (Ubuntu's `10-apparmor.conf`; default on 24.04 per Ubuntu blog: opt-in in 23.10, enabled by default in 24.04 through SRU),
  an unconfined unprivileged task CAN create the user namespace but is moved into the `unprivileged_userns` profile (`allow all`, `deny capability`, `allow pix /**`). Capability checks inside the namespace then fail
  ("Creation succeeds; the privileged operations inside it don't"; log `operation="capable" profile="unprivileged_userns" capname="sys_admin"`). If that profile is not loaded, creation itself is denied: `operation="userns_create" ... error=-13`.
  Confined tasks need an explicit `userns,` rule. `kernel.apparmor_restrict_unprivileged_unconfined=1` additionally stops unconfined tasks switching to a friendlier profile with `aa-exec`.
  Master switch 0 disables everything. The wiki also lists `_force` and `_complain` sysctls (kernel 6.2+ patchset, unverified in 24.04).
- What Canonical documents for an application: ship a profile for the binary path with `abi <abi/4.0>,`, `include <tunables/global>`, `profile NAME /path/to/bin flags=(unconfined) { userns, include if exists <local/NAME> }`
  (Ubuntu blog uses `flags=(default_allow)` for Chrome; bwrap/rootlesskit/Firefox-style profiles use `flags=(unconfined)`). Without an `abi` line, an old-ABI profile is not subject to the restriction at all (wiki) - do not rely on it.
  Profiles are path based ("a binary installed elsewhere" is not covered, Ubuntu blog). Installed in `/etc/apparmor.d/` named by path with dots (`usr.bin.inkwell`), conffile, local overrides in `/etc/apparmor.d/local/`; reload with `apparmor_parser -r FILE` (Ubuntu server docs) or `systemctl reload apparmor`.
  `dh_apparmor` (debhelper) generates the postinst/postrm to load it - not fetched, unverified here. Ubuntu notes vendors are expected to ship their own profile.
- Unconfined profiles that allow userns are themselves a bypass vector (Ubuntu discourse; Qualys bypasses via aa-exec, busybox, LD_PRELOAD). Ubuntu 25.04 removed the busybox and nautilus profiles and added `bwrap-userns-restrict` (release notes);
  on 24.04 it is in `/usr/share/apparmor/extra-profiles/` and must be copied manually (vscode#316046). A 24.04.x apparmor update that shipped it by default broke Flatpak saves and was dropped (Launchpad 2072811).
  25.10/26.04 changes: not fetched, UNVERIFIED.
- Non-packaged binary (home dir, dev build, AppImage mounted under /tmp/.mount_*): no profile matches => restricted (EPERM-on-capability or EACCES as above). Users are told to write their own profile for the path or set the sysctl to 0 (rootlesscontaine.rs does exactly this for `/usr/local/bin/rootlesskit`).
- `/proc/self/exe` re-exec and children: AppArmor attaches by the resolved executable path, so a re-exec of the same file should land in the same profile; and the profile of the parent is inherited across `clone` (userns rule evaluated at creation). I found no document stating this for re-exec explicitly: UNVERIFIED. rootlesskit (same Go re-exec pattern) works with a profile on its own path (rootlesscontaine.rs), which is supporting evidence. Test it on a real host.
- Other software: bubblewrap/Flatpak: dedicated `bwrap-userns-restrict` profile (above); VS Code uses it (#316046); Electron/Chromium: need a profile for the app binary (`chrome-sandbox`), electron-builder tracks it (#8635 per search results), Cursor ships a profile and had to fix a missing `abi`/network rule; Chrome and Firefox (Ubuntu's own profile; Mozilla refused to ship one in its .deb, bugzilla 1884980 WONTFIX) as above; rootless Podman/Docker: profile for rootlesskit only needed outside `/usr/bin`; AppImage: broken unless the user adds a profile (Vesktop thread). Zed: no source found, UNVERIFIED.
- Landlock: docs.kernel.org lists ABI 4 = TCP bind/connect rules, 5 = IOCTL_DEV, 6 = abstract unix socket + signal scopes, 7 = logging flags, 8 = TSYNC, 9 = pathname UNIX sockets, 10 = UDP rights, 11 = NO_NEW_PRIVS flag, 12 = capability/namespace rules, but does not give kernel versions. Kernel mapping ABI 4 = Linux 6.7 is from my own knowledge, not re-verified in this session. Ubuntu 24.04 GA kernel is 6.8 (>= ABI 4: TCP rules available; no scopes, so abstract-unix-socket and signal scoping are missing; ABI 5 = 6.10 and 6 = 6.12 come with the HWE kernels). Ubuntu builds Landlock in; the Raspberry Pi 6.8 kernel did not enable it by default (LP #2066885) - so ABI must be probed (code already refuses without Landlock, `launcher_linux.go:131`). Landlock gives no PID, mount, IPC or UTS isolation, does not restrict `chdir/stat/chmod/chown/...`, does not block already-open fds, and cannot restrict `/proc/<pid>/fd` sockets (docs). UDP rules need ABI 10 (new kernels only); on 6.8 UDP is covered by seccomp socket denial, not Landlock.
- A Landlock + seccomp + rlimits sandbox without namespaces is the standard unprivileged baseline (what Landlock was designed for; also what the code already does after a definitive refusal). I did not find a source calling it a formally "recognised degraded mode"; treat that as my characterisation.
- Debian 13: `kernel.unprivileged_userns_clone` defaults to 1 (debian-user 2025-10 reply, via search; deprecated 2022). The AppArmor userns restriction sysctl default on Debian 13: UNVERIFIED (appears off). Fedora 44: no AppArmor; SELinux has a `user_namespace { create }` permission (kernel commit ed5d44d42c95) but I found no source stating restriction for `unconfined_t`.
  Local evidence: on this Fedora 45 host an unconfined user creates userns, net, pid and mount namespaces and mounts as namespace root (the sidecar sandbox is developed here); `/sys/kernel/security/lsm` lists selinux, landlock. Confined SELinux domains may be denied (unverified).
- GitHub runners: I found no primary source saying the `ubuntu-24.04` image ships with the restriction on. The repository's own history says so (`ci.yml` comments, netsim notes, a479283 "Ubuntu 24.04 restricts ... the netsim job already did") and a runner-images issue (#10443) reports `unshare: Operation not permitted`. TREAT AS LIKELY, UNCONFIRMED: first CI step should print `sysctl kernel.apparmor_restrict_unprivileged_userns` and `aa-status`.

## Part 3 - options

Prior finding that matters for every option: the current degraded path is broken for the stock case (1.2) and EACCES is not classed as a refusal (1.3). Both are bugs independent of A/B and need a fix + test whichever option is chosen.

**A. AppArmor profile in the .deb (`/etc/apparmor.d/usr.bin.inkwell`: `abi <abi/4.0>, include <tunables/global>, profile inkwell /usr/bin/inkwell flags=(unconfined) { userns, include if exists <local/usr.bin.inkwell> }`, loaded by postinst via `dh_apparmor`-style `apparmor_parser -r`, removed in postrm)**
- Protects: restores the FULL sandbox (user/mount/PID/IPC/net ns, tmpfs output bound, cgroup scope) for packaged installs, as developed and tested.
- Costs: the whole app (webview, libp2p) runs under an `unconfined`-flag profile that is exempt from the restriction (so the profile is a global grant for the inkwell binary - an exploited webview could create namespaces; Ubuntu itself ships this pattern for Chrome); a package-owned `/etc` conffile; rpm/Fedora does nothing (not needed); `/usr/bin/inkwell` only. Narrower alternative to evaluate: a dedicated launcher binary/path used only for the sandbox, or a `flags=(attach_disconnected)` profile with a child profile for the sandbox re-exec (needs testing).
- Needs: nfpm `contents` entry + postinstall/postremove, profile test on a real 24.04 (does the `/proc/self/exe` re-exec stay in the profile? unverified).
- Non-packaged runs (home dir, dev build, AppImage, tarball): NOT covered; still restricted.

**B. Namespace-free tier (Landlock incl. TCP rules on ABI >= 4 + seccomp + rlimits [+ cgroup scope when systemd user manager allows without a PID ns]) when namespaces are unusable**
- Protects: filesystem confinement, no new privileges, no sockets (seccomp), memory/file/core limits; on 6.8 Landlock could also bind/connect-deny TCP as a second layer. Lacks PID/mount/IPC/net isolation, the exact output-size tmpfs bound (ADR 0033 already has a Landlock+rlimit fallback), and cgroup death guarantees (ADR 0042 requires a PID ns for cgroup).
- Costs: a threat-model/ADR statement (is "Landlock+seccomp" acceptable for peer documents? the code's own error strings say Landlock+seccomp are the mandatory layers, so it is consistent), a fix of the probe disagreement, EACCES handling (only definitive when the AppArmor sysctl reads 1, else stay fail-closed), a visible notice in the UI ("reduced isolation on this system"), tests with a fake probe for each disagreeing combination.
- Needs: no packaging, works for AppImage/home/dev builds too; Landlock must exist (probe; Pi kernels lack it).

**C. Both**: A makes packaged installs full-strength, B keeps every other run working and honest. B is the safety net, A the default experience.

**D. Other findings**
- Make the isolation probe test what the launch really does (apply the uid-0 map and the /proc mount) so probe #2 cannot say "yes" when #1 says "no"; collapse them into one "namespaces usable" fact (removes the whole class of mixed states).
- Ship the real dependency-free option first: do not run the sandbox as an uid-0 map in a nested userns on restricted hosts; use B.
- Setting `kernel.apparmor_restrict_unprivileged_userns=0` in postinst is unacceptable (weakens the whole machine; the Ubuntu blog and discourse call the control hardening).
- Stage a visible diagnostic: when the sidecar cannot start, say why (and the fix) instead of `preview.fail.unavailable`; log tinymist failure to the UI.

**Recommendation for a security-conscious desktop app today: C.** A first-class AppArmor profile for the packaged binary (what Chrome, Firefox-in-Ubuntu, bwrap, rootlesskit do; Canonical's documented route) plus a namespace-free Landlock+seccomp+rlimit tier that is selected on a definitive refusal, never fails closed into "does not start", and tells the person. If only one: B, because it fixes every unprofiled path (home dir, AppImage, dev, the tarball) and the current bug; A alone leaves non-packaged runs refused.

## How to test it for real
- GitHub `ubuntu-24.04` runners are Ubuntu 24.04 hosts; whether the restriction is on by default there is NOT confirmed by a primary source (the repo's CI comments and a479283 say yes). First step of the job: `sysctl kernel.apparmor_restrict_unprivileged_userns; aa-status | head; uname -r; cat /sys/kernel/security/lsm` as evidence.
- Add a CI job on `ubuntu-24.04` (and `ubuntu-24.04-arm`) that does NOT lift the sysctl and runs: (1) `go test` of `internal/sidecar` with a new test for each probe combination; (2) a real end-to-end: install the built `.deb`, run `inkwell`'s sidecar self-test (or a small `cmd/` helper) that compiles a trivial `.typ` and starts `tinymist` for one LSP round trip, asserting the containment mode ("namespace" vs "degraded"); (3) the same with the profile removed (`apparmor_parser -R`) and from a binary copied to `$HOME`, asserting the degraded tier works and the UI/log says so. Run (2) with the profile loaded and assert `Containment`, `Isolated` and cgroup scope.
- Add the second tier for PKG-04/05: make `inside.sh` smoke compile a document (today it never starts typst).
- A VM (QEMU image, `cloud-image` for 24.04) is only needed to also test the live-image/"profiles not loaded" case (LP #2065088) and `noble-hwe` kernels (6.11+: Landlock ABI 5+); use `systemd-nspawn` is NOT valid (shares host kernel/LSM).

## Summary (<= 15 lines)
1. Code intends: refuse only the mandatory layers; degrade to Landlock+seccomp+rlimits on a definitive namespace refusal (ADR 0033, `mount_linux.go:22-40`).
2. Stock 24.04: userns creation succeeds, privileges are denied; the mount probe says "no", the isolation probe (`Probe` returns before the /proc mount, `launcher_linux.go:107`) says "yes": mixed state.
3. Mixed state => real NoNetwork launch is PID 1 with uid != 0 and no caps => `mount /proc` EPERM => exit 125 (verified mechanism locally via `unshare --map-user`; review claim (b) confirmed, no test covers it).
4. tinymist (always NoNetwork) and typst then fail; completion fails silently (log only), compile shows a generic "exited with status 125" error.
5. The CI error was EACCES ("permission denied"); `definitiveRefusal` accepts only EPERM/EINVAL/ENOSPC => permanent "inconclusive" refusal, shown as `preview.fail.unavailable` (blames the install).
6. PKG-01..11 never start a sidecar (smoke = window only) and run in containers on the host kernel: no evidence about the sandbox. All three CI sandbox jobs lift the sysctl.
7. The .deb ships nothing for AppArmor (only postinstall.sh for desktop caches).
8. Canonical's route: a profile for the binary path with `abi <abi/4.0>`, `flags=(unconfined)`, `userns,` in `/etc/apparmor.d/`, loaded with `apparmor_parser -r`; non-packaged paths are not covered; `/proc/self/exe` re-exec staying in the profile is unverified.
9. Landlock on 24.04's 6.8: ABI 4 (TCP rules) expected, ABI mapping to 6.7 not re-verified; Pi kernels lack Landlock; no PID/mount/net isolation without namespaces.
10. Recommended: C (profile in the .deb + a fixed, announced Landlock/seccomp/rlimit tier); if only one, B. Also fix the probe disagreement and EACCES handling, and add a real 24.04 job without the sysctl and a sidecar-starting smoke.
11. Unverified: real behaviour on a stock 24.04 host (EPERM vs EACCES), GitHub runner default, 25.10/26.04 changes, Debian 13 AppArmor default, Fedora SELinux userns policy, dh_apparmor details.
