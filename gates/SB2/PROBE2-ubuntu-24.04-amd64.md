# SB2 - raw sandbox probe answers, run 2, stock Ubuntu 24.04 amd64 (run 43642240366, 2026-11-14)

```text
Q1: kernel=7.0.0-1014-aws arch=x86_64 os=Ubuntu 24.04.5 LTS
Q1: sysctl kernel.apparmor_restrict_unprivileged_userns=1
Q1: sysctl kernel.apparmor_restrict_unprivileged_unconfined=0
Q1: sysctl kernel.unprivileged_userns_clone=1
Q1: sysctl user.max_user_namespaces=55514
Q1: sysctl kernel.yama.ptrace_scope=1
Q1: lsm=lockdown,capability,landlock,yama,apparmor,ima,evm
Q1: apparmor enabled=Y
Q1: own label=unconfined
Q1: profiles mentioning userns: 92
Q1: userns profile file: /etc/apparmor.d/runc
Q1: userns profile file: /etc/apparmor.d/nautilus
Q1: userns profile file: /etc/apparmor.d/MongoDB_Compass
Q1: userns profile file: /etc/apparmor.d/kchmviewer
Q1: userns profile file: /etc/apparmor.d/lc-compliance
Q1: userns profile file: /etc/apparmor.d/lxc-destroy
Q1: userns profile file: /etc/apparmor.d/thunderbird
Q1: userns profile file: /etc/apparmor.d/sbuild
Q1: userns profile file: /etc/apparmor.d/balena-etcher
Q1: userns profile file: /etc/apparmor.d/mmdebstrap
Q1: userns profile file: /etc/apparmor.d/sbuild-hold
Q1: userns profile file: /etc/apparmor.d/chrome
Q1: userns profile file: /etc/apparmor.d/lxc-usernsexec
Q1: userns profile file: /etc/apparmor.d/lxc-create
Q1: userns profile file: /etc/apparmor.d/wike
Q1: userns profile file: /etc/apparmor.d/qmapshack
Q1: userns profile file: /etc/apparmor.d/tuxedo-control-center
Q1: userns profile file: /etc/apparmor.d/notepadqq
Q1: userns profile file: /etc/apparmor.d/goldendict
Q1: userns profile file: /etc/apparmor.d/lxc-unshare
Q1: userns profile file: /etc/apparmor.d/qutebrowser
Q1: userns profile file: /etc/apparmor.d/libcamerify
Q1: userns profile file: /etc/apparmor.d/buildah
Q1: userns profile file: /etc/apparmor.d/github-desktop
Q1: userns profile file: /etc/apparmor.d/userbindmount
Q1: userns profile file: /etc/apparmor.d/sbuild-unhold
Q1: userns profile file: /etc/apparmor.d/surfshark
Q1: userns profile file: /etc/apparmor.d/tup
Q1: userns profile file: /etc/apparmor.d/crun
Q1: userns profile file: /etc/apparmor.d/vpnns
Q1: userns profile file: /etc/apparmor.d/scide
Q1: userns profile file: /etc/apparmor.d/lxc-attach
Q1: userns profile file: /etc/apparmor.d/lxc-execute
Q1: userns profile file: /etc/apparmor.d/toybox
Q1: userns profile file: /etc/apparmor.d/cam
Q1: userns profile file: /etc/apparmor.d/trinity
Q1: userns profile file: /etc/apparmor.d/sbuild-checkpackages
Q1: userns profile file: /etc/apparmor.d/microsoft-edge-stable
Q1: userns profile file: /etc/apparmor.d/systemd-coredump
Q1: userns profile file: /etc/apparmor.d/1password
Q1: userns profile file: /etc/apparmor.d/vdens
Q1: userns profile file: /etc/apparmor.d/sbuild-upgrade
Q1: userns profile file: /etc/apparmor.d/sbuild-apt
Q1: userns profile file: /etc/apparmor.d/sbuild-adduser
Q1: userns profile file: /etc/apparmor.d/foliate
Q1: userns profile file: /etc/apparmor.d/rssguard
Q1: userns profile file: /etc/apparmor.d/epiphany
Q1: userns profile file: /etc/apparmor.d/loupe
Q1: userns profile file: /etc/apparmor.d/podman
Q1: userns profile file: /etc/apparmor.d/sbuild-distupgrade
Q1: userns profile file: /etc/apparmor.d/evolution
Q1: userns profile file: /etc/apparmor.d/vivaldi-bin
Q1: userns profile file: /etc/apparmor.d/signal-desktop
Q1: userns profile file: /etc/apparmor.d/rpm
Q1: userns profile file: /etc/apparmor.d/linux-sandbox
Q1: userns profile file: /etc/apparmor.d/wpcom
Q1: userns profile file: /etc/apparmor.d/slirp4netns
Q1: userns profile file: /etc/apparmor.d/flatpak
Q1: userns profile file: /etc/apparmor.d/lxc-stop
Q1: userns profile file: /etc/apparmor.d/code
Q1: content of /etc/apparmor.d/chrome:
Q1:   # This profile allows everything and only exists to give the
Q1:   # application a name instead of having the label "unconfined"
Q1:   
Q1:   abi <abi/4.0>,
Q1:   include <tunables/global>
Q1:   
Q1:   profile chrome /opt/google/chrome/chrome flags=(unconfined) {
Q1:     userns,
Q1:   
Q1:     # Site-specific additions and overrides. See local/README for details.
Q1:     include if exists <local/chrome>
Q1:   }
Q1: content of /etc/apparmor.d/firefox:
Q1:   # This profile allows everything and only exists to give the
Q1:   # application a name instead of having the label "unconfined"
Q1:   
Q1:   abi <abi/4.0>,
Q1:   include <tunables/global>
Q1:   
Q1:   profile firefox /{usr/lib/firefox{,-esr,-beta,-devedition,-nightly},opt/firefox}/firefox{,-esr,-bin} flags=(unconfined) {
Q1:     userns,
Q1:   
Q1:     # Site-specific additions and overrides. See local/README for details.
Q1:     include if exists <local/firefox>
Q1:   }
Q1: content of /etc/apparmor.d/unprivileged_userns:
Q1:   # Special profile transitioned to by unconfined when creating an unprivileged
Q1:   # user namespace.
Q1:   #
Q1:   abi <abi/4.0>,
Q1:   include <tunables/global>
Q1:   
Q1:   profile unprivileged_userns {
Q1:        audit deny capability,
Q1:        audit deny change_profile,
Q1:   
Q1:        # allow block to be replaced by allow when x dominance test is fixed
Q1:        #allow all,
Q1:        allow network,
Q1:        allow signal,
Q1:        allow dbus,
Q1:        allow file rwlkm /**,
Q1:        allow unix,
Q1:        allow mqueue,
Q1:        allow ptrace,
Q1:        allow userns,
Q1:   
Q1:        # stack children to strip capabilities
Q1:        allow pix /** -> &unprivileged_userns ,
Q1:   
Q1:        # Site-specific additions and overrides. See local/README for details.
Q1:        include if exists <local/unprivileged_userns>
Q1:   }
Q1: aa-status (head): exit=0 out=apparmor module is loaded.|124 profiles are loaded.|29 profiles are in enforce mode.|   /snap/snapd/28332/usr/lib/snapd/snap-confine|   /usr/bin/man|   /usr/lib/snapd/snap-confine|   /usr/sbin/chronyd|   /usr/sbin/haveged|   /usr/sbin/mysqld|   docker-default|   lsb_release|   man_filter
Q2: /usr/bin/unshare -Ur: exit=1 out=unshare: write failed /proc/self/uid_map: Operation not permitted
Q2: /usr/bin/unshare -Urm: exit=1 out=unshare: write failed /proc/self/uid_map: Operation not permitted
Q2: /usr/bin/unshare -Urn: exit=1 out=unshare: write failed /proc/self/uid_map: Operation not permitted
Q2: /usr/bin/unshare -Urpf --mount-proc: exit=1 out=unshare: write failed /proc/self/uid_map: Operation not permitted
Q2: /usr/bin/unshare -Urm tmpfs: exit=1 out=unshare: write failed /proc/self/uid_map: Operation not permitted
Q3: go probe go test: exit=0
Q3: [Q3] process label=unconfined
Q3: [Q3] child /usr/bin/cat label=unconfined err=<nil>
Q3: [Q3] child /tmp/TestSandboxProbeReport1547548028/001/extracted-cat label=unconfined err=<nil>
Q3: [Q3] probeOutputMount supported=false err=sidecar: namespace probe inconclusive; launch refused, retry: fork/exec /proc/self/exe: permission denied errno=EACCES
Q3: [Q3] probeIsolation supported=false err=sidecar: namespace probe inconclusive; launch refused, retry: fork/exec /proc/self/exe: permission denied errno=EACCES
Q3: [Q3] raw probe mount namespaces: supported=false err=fork/exec /proc/self/exe: permission denied errno=EACCES definitive=false
Q3: [Q3] raw probe user+net+pid+ipc+mount namespaces: supported=false err=fork/exec /proc/self/exe: permission denied errno=EACCES definitive=false
Q3: [Q3] Run echo NoNetwork=false: err=sidecar: namespace probe inconclusive; launch refused, retry: fork/exec /proc/self/exe: permission denied errno=EACCES exit=0 signal=0 isolated=false containment="" stderr=""
Q3: [Q3] Run echo NoNetwork=true: err=sidecar: namespace probe inconclusive; launch refused, retry: fork/exec /proc/self/exe: permission denied errno=EACCES exit=0 signal=0 isolated=false containment="" stderr=""
Q3: [Q4] landlock ABI=8
Q3: [Q4] landlock ruleset with TCP rules: accepted
Q5: systemctl --user is-system-running: exit=0 out=running
Q5: loginctl show-user: exit=0 out=RuntimePath=/run/user/1001|State=lingering|Sessions=|Linger=yes
Q5: XDG_RUNTIME_DIR=/run/user/1001 DBUS_SESSION_BUS_ADDRESS=unset
Q5: /proc/self/cgroup=0::/system.slice/hosted-compute-agent.service|
Q5: own cgroup /sys/fs/cgroup/system.slice/hosted-compute-agent.service: controllers=cpuset cpu io memory pids writable=no
Q5: systemd-run --user --scope: exit=0 out=0::/user.slice/user-1001.slice/user@1001.service/app.slice/run-r13d3e178d9604eb383c5297274084329.scope|scope_ok
Q6: go test exit=1 (124 or a panic « test timed out » means a test never returned)
Q6: === RUN   TestRunNoNetworkFallsBack
Q6: === RUN   TestCgroupWorkloadAndDescendantsInside
Q6: --- FAIL: TestCgroupWorkloadAndDescendantsInside (0.00s)
Q6: --- FAIL: TestRunNoNetworkFallsBack (0.00s)
Q6: FAIL
Q6: FAIL	github.com/oioio-space/inkwell/internal/sidecar	0.005s
Q6: FAIL
Q6:   === RUN   TestRunNoNetworkFallsBack
Q6:   === PAUSE TestRunNoNetworkFallsBack
Q6:   === RUN   TestCgroupWorkloadAndDescendantsInside
Q6:       scope_linux_test.go:173: sidecar: namespace probe inconclusive; launch refused, retry: fork/exec /proc/self/exe: permission denied
Q6:       scope_linux_test.go:182: scope unavailable
Q6:   --- FAIL: TestCgroupWorkloadAndDescendantsInside (0.00s)
Q6:   === CONT  TestRunNoNetworkFallsBack
Q6:       limits_test.go:146: Run = sidecar: namespace probe inconclusive; launch refused, retry: fork/exec /proc/self/exe: permission denied exit 0 isolated false, want it to run without isolation
Q6:   --- FAIL: TestRunNoNetworkFallsBack (0.00s)
Q6:   FAIL
Q6:   FAIL	github.com/oioio-space/inkwell/internal/sidecar	0.005s
Q6:   FAIL
Q7: tinymist --version outside the sandbox: exit=0 calls: mmap×26 read×10 mprotect×8 openat×7 fstat×7 close×7 rt_sigaction×5 brk×5 sigaltstack×3 prlimit64×2 pread64×2 munmap×2 write×1 set_tid_address×1 set_robust_list×1 sched_getaffinity×1 rseq×1 poll×1 ioctl×1 getrandom×1 exit_group×1 execve×1 arch_prctl×1 access×1 
Q7: typst --version outside the sandbox: exit=0 calls: mmap×35 munmap×30 rt_sigaction×6 sigaltstack×3 brk×2 write×1 set_tid_address×1 rt_sigprocmask×1 poll×1 mprotect×1 ioctl×1 exit_group×1 execve×1 arch_prctl×1 
Q7: TestEmbeddedSidecarsRun under strace: exit=1 --- FAIL: TestEmbeddedSidecarsRun (1.66s)|FAIL|FAIL	github.com/oioio-space/inkwell/internal/sidecar	1.670s|FAIL|
Q7: calls answered ENOSYS inside: 
Q7: calls answered EPERM inside: epoll_ctl×602 
Q7: calls answered EACCES inside: execve×2 openat×1 
Q7: signals inside: --- SIGURG  ×261 --- SIGCHLD  ×10 
Q7: last calls before each kill: 
Q9: typst compile (PDF): exit=1 clone_thread=0 clone_process=0 clone3=0 fork=0 vfork=0 execve=1
Q9: typst compile (SVG): exit=1 clone_thread=0 clone_process=0 clone3=0 fork=0 vfork=0 execve=1
Q9: tinymist lsp session: exit=0 replies=[1, 2, 3, 9] diagnostics=2 server_exit=0  clone_thread=0 clone_process=0 clone3=15 fork=0 vfork=1 execve=2
Q10: go test TestSandboxProbeTiers: exit=0
Q10: process label=unconfined; the tier this machine decides:
Q10: typst --version: err=sidecar: namespace probe inconclusive; launch refused, retry: fork/exec /proc/self/exe: permission denied errno=EACCES exit=0 signal=0 isolated=false containment="" stdout=0 bytes "" stderr=""
Q10: typst compile to stdout: err=sidecar: namespace probe inconclusive; launch refused, retry: fork/exec /proc/self/exe: permission denied errno=EACCES exit=0 signal=0 isolated=false containment="" stdout=0 bytes "" stderr=""
Q10: tinymist --version: err=sidecar: namespace probe inconclusive; launch refused, retry: fork/exec /proc/self/exe: permission denied errno=EACCES exit=0 signal=0 isolated=false containment="" stdout=0 bytes "" stderr=""
Q11: the namespace-free tier forced (both probes answer no):
Q11: typst --version: err=<nil> errno=- exit=0 signal=0 isolated=false containment="rlimit" stdout=24 bytes "typst 0.15.1 (9dfd3a08)" stderr=""
Q11: typst compile to stdout: err=<nil> errno=- exit=0 signal=0 isolated=false containment="rlimit" stdout=11601 bytes "%PDF-1.7\n%\x80\x80\x80\x80\n\n1 0 obj\n<</Type/Pages/Co" stderr=""
Q11: tinymist --version: err=<nil> errno=- exit=0 signal=0 isolated=false containment="rlimit" stdout=412 bytes "tinymist \nBuild Timestamp:     2026-10-1" stderr=""
Q8: before any profile:
Q8: /usr/local/bin/inkwell-probe-unshare -Ur: exit=1 out=inkwell-probe-unshare: write failed /proc/self/uid_map: Operation not permitted
Q8: /usr/local/bin/inkwell-probe-unshare -Urm: exit=1 out=inkwell-probe-unshare: write failed /proc/self/uid_map: Operation not permitted
Q8: /usr/local/bin/inkwell-probe-unshare -Urn: exit=1 out=inkwell-probe-unshare: write failed /proc/self/uid_map: Operation not permitted
Q8: /usr/local/bin/inkwell-probe-unshare -Urpf --mount-proc: exit=1 out=inkwell-probe-unshare: write failed /proc/self/uid_map: Operation not permitted
Q8: /usr/local/bin/inkwell-probe-unshare -Urm tmpfs: exit=1 out=inkwell-probe-unshare: write failed /proc/self/uid_map: Operation not permitted
Q8: go probe /usr/local/bin/inkwell-sandbox-probe: exit=0
Q8: [Q3] process label=unconfined
Q8: [Q3] child /usr/bin/cat label=unconfined err=<nil>
Q8: [Q3] child /tmp/TestSandboxProbeReport2831003238/001/extracted-cat label=unconfined err=<nil>
Q8: [Q3] probeOutputMount supported=false err=sidecar: namespace probe inconclusive; launch refused, retry: fork/exec /proc/self/exe: permission denied errno=EACCES
Q8: [Q3] probeIsolation supported=false err=sidecar: namespace probe inconclusive; launch refused, retry: fork/exec /proc/self/exe: permission denied errno=EACCES
Q8: [Q3] raw probe mount namespaces: supported=false err=fork/exec /proc/self/exe: permission denied errno=EACCES definitive=false
Q8: [Q3] raw probe user+net+pid+ipc+mount namespaces: supported=false err=fork/exec /proc/self/exe: permission denied errno=EACCES definitive=false
Q8: [Q3] Run echo NoNetwork=false: err=sidecar: namespace probe inconclusive; launch refused, retry: fork/exec /proc/self/exe: permission denied errno=EACCES exit=0 signal=0 isolated=false containment="" stderr=""
Q8: [Q3] Run echo NoNetwork=true: err=sidecar: namespace probe inconclusive; launch refused, retry: fork/exec /proc/self/exe: permission denied errno=EACCES exit=0 signal=0 isolated=false containment="" stderr=""
Q8: [Q4] landlock ABI=8
Q8: [Q4] landlock ruleset with TCP rules: accepted
Q8: apparmor_parser -r /etc/apparmor.d/inkwell-sandbox-probe: exit=0 out=
Q8: apparmor_parser -r /etc/apparmor.d/inkwell-probe-unshare: exit=0 out=
Q8: with the candidate profiles loaded:
Q8: /usr/local/bin/inkwell-probe-unshare -Ur: exit=0 out=label=inkwell-probe-unshare (unconfined) Uid: 0 0 0 0 CapEff: 000001ffffffffff mount: /proc: permission denied.|       dmesg(1) may have more information after failed mount system call.|mount_exit=32
Q8: /usr/local/bin/inkwell-probe-unshare -Urm: exit=0 out=label=inkwell-probe-unshare (unconfined) Uid: 0 0 0 0 CapEff: 000001ffffffffff mount: /proc: permission denied.|       dmesg(1) may have more information after failed mount system call.|mount_exit=32
Q8: /usr/local/bin/inkwell-probe-unshare -Urn: exit=0 out=label=inkwell-probe-unshare (unconfined) Uid: 0 0 0 0 CapEff: 000001ffffffffff mount: /proc: permission denied.|       dmesg(1) may have more information after failed mount system call.|mount_exit=32
Q8: /usr/local/bin/inkwell-probe-unshare -Urpf --mount-proc: exit=0 out=Uid: 0 0 0 0 CapEff: 000001ffffffffff pid=1
Q8: /usr/local/bin/inkwell-probe-unshare -Urm tmpfs: exit=0 out=tmpfs_exit=0
Q8: go probe /usr/local/bin/inkwell-sandbox-probe: exit=0
Q8: [Q3] process label=inkwell-sandbox-probe (unconfined)
Q8: [Q3] child /usr/bin/cat label=inkwell-sandbox-probe (unconfined) err=<nil>
Q8: [Q3] child /tmp/TestSandboxProbeReport717204741/001/extracted-cat label=inkwell-sandbox-probe (unconfined) err=<nil>
Q8: [Q3] probeOutputMount supported=true err=<nil> errno=-
Q8: [Q3] probeIsolation supported=true err=<nil> errno=-
Q8: [Q3] raw probe mount namespaces: supported=true err=<nil> errno=- definitive=false
Q8: [Q3] raw probe user+net+pid+ipc+mount namespaces: supported=true err=<nil> errno=- definitive=false
Q8: [Q3] Run echo NoNetwork=false: err=<nil> errno=- exit=0 signal=0 isolated=false containment="rlimit" stderr="err"
Q8: [Q3] Run echo NoNetwork=true: err=<nil> errno=- exit=0 signal=0 isolated=true containment="rlimit" stderr="err"
Q8: [Q4] landlock ABI=8
Q8: [Q4] landlock ruleset with TCP rules: accepted
Q8: the same binary from an unprofiled path:
Q8: go probe /tmp/tmp.RMFNIHgTmx/elsewhere.test: exit=0
Q8: [Q3] process label=unconfined
Q8: [Q3] child /usr/bin/cat label=unconfined err=<nil>
Q8: [Q3] child /tmp/TestSandboxProbeReport2345443123/001/extracted-cat label=unconfined err=<nil>
Q8: [Q3] probeOutputMount supported=false err=sidecar: namespace probe inconclusive; launch refused, retry: fork/exec /proc/self/exe: permission denied errno=EACCES
Q8: [Q3] probeIsolation supported=false err=sidecar: namespace probe inconclusive; launch refused, retry: fork/exec /proc/self/exe: permission denied errno=EACCES
Q8: [Q3] raw probe mount namespaces: supported=false err=fork/exec /proc/self/exe: permission denied errno=EACCES definitive=false
Q8: [Q3] raw probe user+net+pid+ipc+mount namespaces: supported=false err=fork/exec /proc/self/exe: permission denied errno=EACCES definitive=false
Q8: [Q3] Run echo NoNetwork=false: err=sidecar: namespace probe inconclusive; launch refused, retry: fork/exec /proc/self/exe: permission denied errno=EACCES exit=0 signal=0 isolated=false containment="" stderr=""
Q8: [Q3] Run echo NoNetwork=true: err=sidecar: namespace probe inconclusive; launch refused, retry: fork/exec /proc/self/exe: permission denied errno=EACCES exit=0 signal=0 isolated=false containment="" stderr=""
Q8: [Q4] landlock ABI=8
Q8: [Q4] landlock ruleset with TCP rules: accepted
Q8: under the profile, traced: exit=0 --- PASS: TestEmbeddedSidecarsRun (1.70s)|--- PASS: TestSandboxProbeTiers (0.39s)|
Q8: [Q10 traced, profile] process label=inkwell-sandbox-probe (unconfined); the tier this machine decides:
Q8: [Q10 traced, profile] typst --version: err=<nil> errno=- exit=0 signal=0 isolated=false containment="rlimit" stdout=24 bytes "typst 0.15.1 (9dfd3a08)" stderr=""
Q8: [Q10 traced, profile] typst compile to stdout: err=<nil> errno=- exit=0 signal=0 isolated=true containment="rlimit" stdout=11601 bytes "%PDF-1.7\n%\x80\x80\x80\x80\n\n1 0 obj\n<</Type/Pages/Co" stderr=""
Q8: [Q10 traced, profile] tinymist --version: err=<nil> errno=- exit=0 signal=0 isolated=true containment="rlimit" stdout=412 bytes "tinymist \nBuild Timestamp:     2026-10-1" stderr=""
Q8: [Q11 traced, profile] the namespace-free tier forced (both probes answer no):
Q8: [Q11 traced, profile] typst --version: err=<nil> errno=- exit=0 signal=0 isolated=false containment="rlimit" stdout=24 bytes "typst 0.15.1 (9dfd3a08)" stderr=""
Q8: [Q11 traced, profile] typst compile to stdout: err=<nil> errno=- exit=0 signal=0 isolated=false containment="rlimit" stdout=11601 bytes "%PDF-1.7\n%\x80\x80\x80\x80\n\n1 0 obj\n<</Type/Pages/Co" stderr=""
Q8: [Q11 traced, profile] tinymist --version: err=<nil> errno=- exit=0 signal=0 isolated=false containment="rlimit" stdout=412 bytes "tinymist \nBuild Timestamp:     2026-10-1" stderr=""
Q8: calls answered ENOSYS inside, under the profile: 
Q8: calls answered EPERM inside, under the profile: epoll_ctl×59 
Q8: calls answered EACCES inside, under the profile: open×12 openat×1 
Q8: kills under the profile: 0 
```
