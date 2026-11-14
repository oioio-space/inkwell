# SB2 - measurements on GitHub's stock Ubuntu 24.04 (2026-11-14)

Run 43638847261 of `.github/workflows/sandbox-probe.yml` on commit 4b27a6c, runners `ubuntu-24.04` (x86_64) and
`ubuntu-24.04-arm` (aarch64), kernel 7.0.0-1017-azure, the AppArmor restriction left on. Raw answers:
`PROBE-ubuntu-24.04-amd64.md`, `PROBE-ubuntu-24.04-arm64.md`. Both architectures answered the
same on every question below unless noted.

| # | Question | Measured answer | Settles |
|---|---|---|---|
| Q1 | Is the restriction on by default? | `kernel.apparmor_restrict_unprivileged_userns=1`, `…_unconfined=0`, AppArmor enabled, the process is `unconfined`; LSMs include landlock | yes: GitHub's stock runner IS the stock-Ubuntu case; ADR 0044's E-1 settled |
| Q1 | Real profile syntax on 24.04 | `abi <abi/4.0>,` `include <tunables/global>` `profile NAME PATH flags=(unconfined) { userns, include if exists <local/NAME> }` (chrome, firefox, code, obsidian, element-desktop… 92 such files) | the profile form (not `default_allow`) |
| Q2 | What an unconfined program gets | `unshare -Ur…`: « write failed /proc/self/uid_map: Operation not permitted » for every variant | a namespace is created but cannot be mapped: unusable |
| Q3 | What inkwell's launcher does today | `fork/exec /proc/self/exe: permission denied` (**EACCES**), classed « inconclusive », launch **refused** for both `Run` variants | **on stock Ubuntu 24.04 every sidecar launch is refused today**: no preview, no export, no completion (W69 confirmed) |
| Q4 | Landlock | ABI **8**, a ruleset with TCP rules accepted | the namespace-free tier has Landlock incl. network and signal/abstract-socket scoping (ABI ≥ 6) on this kernel |
| Q5 | cgroup delegation | the job runs in `system.slice/hosted-compute-agent.service` (not writable); `systemd-run --user --scope` works and lands in `user@1001.service/app.slice` | a user scope IS available on the runner; the earlier « no delegated scope » comes from how the test process is started, not from the machine |
| Q6 | The two tests that « never return » | with the restriction on, both FAIL at once (0.006 s) on the refused launch; they hung only on the runs that lifted the restriction | not a hang of the product here; `TestCgroupWorkloadAndDescendantsInside`'s wait cycle (Codex) remains a test defect to fix |
| Q7 | System calls at start, outside the sandbox | arm64: typst and tinymist both call **`ppoll`** once at start (amd64: `poll`); tinymist also calls `prlimit64` ×2 | finding 1 is confirmed as far as the call goes: the arm64 allow-list lacks `ppoll` (73); the inside trace was not informative (the launch was refused before the target ran) |
| Q8 | The candidate profile (`flags=(unconfined) { userns, }` on the binary's path) | with it loaded: both probes **supported**, `Run … NoNetwork=true` **isolated=true**, containment `rlimit`; children, **including a binary extracted to /tmp, inherit the profile's label**; `/proc/self/exe` re-exec works; the same binary from an unprofiled path is still refused (EACCES) | ADR 0044's E-2 settled: a profile for `/usr/bin/inkwell` covers the re-exec and the extracted sidecars; non-packaged runs need the namespace-free tier |
| Q8 | `mount /proc` in a plain `unshare -Ur` under the profile | still « permission denied » | the launcher's own path (PID namespace first) works; a mount of /proc without a PID namespace is not a path to rely on |

Not measured: a desktop session (Wayland, a logged-in user), the installed `.deb`, Fedora's SELinux policy, and the
syscalls typst and tinymist make *inside* the sandbox on arm64 (the launch never reached them). Next probe run:
with the profile loaded, trace the arm64 sidecars inside the sandbox.
