#!/usr/bin/env bash
# Measures what a machine lets the sidecar sandbox do (block SB2): raw facts, one "Qn: ..." line each,
# for a person and for a script. It never fails on an answer, only on an error of the probe itself.
# Run by .github/workflows/sandbox-probe.yml on stock Ubuntu 24.04 runners (amd64 and arm64), where
# nothing lifts the AppArmor restriction on unprivileged user namespaces; it also runs locally.
#
#   scripts/sandbox-probe.sh              # every question this machine can answer without sudo
#   scripts/sandbox-probe.sh q1 q2        # some questions
#   INKWELL_PROBE_SUDO=1 scripts/sandbox-probe.sh   # also q8 (loads two test AppArmor profiles) and sudo reads
#
# Questions: q1 kernel and AppArmor state; q2 what an unconfined binary gets from unshare; q3 and q4 the
# repository's own probes and Landlock (a Go test, INKWELL_SANDBOX_PROBE=1); q5 systemd user session and
# cgroup delegation; q6 the two tests that never returned on the runner, with Go's goroutine dump;
# q7 the system calls of the pinned typst and tinymist at start, outside and inside the sandbox (needs
# strace and the fetched sidecars); q8 the same as q2 and q3 under a candidate AppArmor profile.
set -uo pipefail

repo=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
cd "$repo" || exit 2
work=$(mktemp -d)
trap 'rm -rf "$work"' EXIT
sudo_ok=${INKWELL_PROBE_SUDO:-0}

# say Q TEXT: one answer line.
say() { printf '%s: %s\n' "$1" "$2"; }

# try Q LABEL CMD...: runs CMD with a deadline and reports its exit status and output on one line.
try() {
  local q=$1 label=$2 out rc
  shift 2
  out=$(timeout -k 5 60 "$@" 2>&1)
  rc=$?
  say "$q" "$label: exit=$rc out=$(printf '%s' "$out" | tr '\n' '|' | cut -c1-600)"
}

# sysctl_value NAME: the value, or "absent".
sysctl_value() {
  local path="/proc/sys/${1//./\/}"
  if [[ -r $path ]]; then cat "$path"; else echo absent; fi
}

q1() {
  say Q1 "kernel=$(uname -r) arch=$(uname -m) os=$(. /etc/os-release 2>/dev/null && echo "${PRETTY_NAME:-unknown}")"
  local name
  for name in kernel.apparmor_restrict_unprivileged_userns kernel.apparmor_restrict_unprivileged_unconfined \
    kernel.unprivileged_userns_clone user.max_user_namespaces kernel.yama.ptrace_scope; do
    say Q1 "sysctl $name=$(sysctl_value "$name")"
  done
  say Q1 "lsm=$(cat /sys/kernel/security/lsm 2>/dev/null || echo unreadable)"
  say Q1 "apparmor enabled=$(cat /sys/module/apparmor/parameters/enabled 2>/dev/null || echo absent)"
  say Q1 "own label=$(tr -d '\0' </proc/self/attr/current 2>/dev/null || echo unreadable)"
  if [[ -d /etc/apparmor.d ]]; then
    say Q1 "profiles mentioning userns: $(grep -rlE '^[[:space:]]*userns' /etc/apparmor.d 2>/dev/null | wc -l)"
    grep -rlE '^[[:space:]]*userns' /etc/apparmor.d 2>/dev/null | head -60 | while read -r f; do say Q1 "userns profile file: $f"; done
    local sample
    for sample in /etc/apparmor.d/chrome /etc/apparmor.d/firefox /etc/apparmor.d/bwrap-userns-restrict /etc/apparmor.d/unprivileged_userns; do
      [[ -r $sample ]] || continue
      say Q1 "content of $sample:"
      sed 's/^/Q1:   /' "$sample" | head -60
    done
  else
    say Q1 "/etc/apparmor.d: absent"
  fi
  if [[ $sudo_ok == 1 ]] && command -v aa-status >/dev/null 2>&1; then
    try Q1 "aa-status (head)" sudo sh -c 'aa-status 2>&1 | head -12'
  fi
}

# unshare_matrix Q BINARY: what BINARY (unshare or a copy of it) gets for each namespace set.
unshare_matrix() {
  local q=$1 bin=$2 inner
  # shellcheck disable=SC2016 # the inner shell expands it, inside the namespace
  inner='printf "label=%s " "$(tr -d "\\0" </proc/self/attr/current 2>/dev/null)"; grep -E "^(Uid|CapEff)" /proc/self/status | tr "\n\t" "  "; mount -t proc proc /proc 2>&1; echo "mount_exit=$?"'
  try "$q" "$bin -Ur" "$bin" -Ur sh -c "$inner"
  try "$q" "$bin -Urm" "$bin" -Urm sh -c "$inner"
  try "$q" "$bin -Urn" "$bin" -Urn sh -c "$inner"
  try "$q" "$bin -Urpf --mount-proc" "$bin" -Urpf --mount-proc sh -c 'grep -E "^(Uid|CapEff)" /proc/self/status | tr "\n\t" "  "; echo "pid=$$"'
  try "$q" "$bin -Urm tmpfs" "$bin" -Urm sh -c 'mount -t tmpfs tmpfs /mnt 2>&1; echo "tmpfs_exit=$?"'
}

q2() {
  command -v unshare >/dev/null 2>&1 || { say Q2 "unshare: absent"; return; }
  unshare_matrix Q2 "$(command -v unshare)"
}

# go_probe Q BINARY: the repository's own probes (TestSandboxProbeReport), through `go test` when BINARY
# is empty, else through that compiled test binary.
go_probe() {
  local q=$1 bin=${2:-} out rc
  if [[ -n $bin ]]; then
    out=$(INKWELL_SANDBOX_PROBE=1 timeout -k 5 180 "$bin" -test.run '^TestSandboxProbeReport$' -test.v -test.count=1 2>&1)
  else
    out=$(INKWELL_SANDBOX_PROBE=1 timeout -k 5 300 go test ./internal/sidecar -run '^TestSandboxProbeReport$' -v -count=1 2>&1)
  fi
  rc=$?
  say "$q" "go probe ${bin:-go test}: exit=$rc"
  # The test prints its own Q3 and Q4 lines; under another question (q8) they are re-tagged.
  printf '%s\n' "$out" | grep -E '^Q[0-9]+: ' | sed "s/^Q\\([0-9]*\\): /$q: [Q\\1] /"
  [[ $rc -eq 0 ]] || printf '%s\n' "$out" | tail -15 | sed "s/^/$q:   /"
}

q3() { go_probe Q3 ""; }

q5() {
  try Q5 "systemctl --user is-system-running" systemctl --user is-system-running
  try Q5 "loginctl show-user" loginctl show-user "$(id -un)" -p State -p Linger -p Sessions -p RuntimePath
  say Q5 "XDG_RUNTIME_DIR=${XDG_RUNTIME_DIR:-unset} DBUS_SESSION_BUS_ADDRESS=${DBUS_SESSION_BUS_ADDRESS:-unset}"
  say Q5 "/proc/self/cgroup=$(tr '\n' '|' </proc/self/cgroup)"
  local cg
  cg="/sys/fs/cgroup$(sed -n 's/^0:://p' /proc/self/cgroup)"
  say Q5 "own cgroup $cg: controllers=$(cat "$cg/cgroup.controllers" 2>/dev/null || echo unreadable) writable=$([[ -w $cg/cgroup.procs ]] && echo yes || echo no)"
  try Q5 "systemd-run --user --scope" systemd-run --user --scope --quiet -p Delegate=yes sh -c 'cat /proc/self/cgroup; echo scope_ok'
}

q6() {
  local out rc
  out=$(timeout -k 10 240 go test ./internal/sidecar -run 'TestCgroupWorkloadAndDescendantsInside|TestRunNoNetworkFallsBack' -timeout 90s -v -count=1 2>&1)
  rc=$?
  say Q6 "go test exit=$rc (124 or a panic « test timed out » means a test never returned)"
  printf '%s\n' "$out" | grep -E '^(=== RUN|--- (PASS|FAIL|SKIP)|PASS|FAIL|ok|panic:)' | sed 's/^/Q6: /'
  if printf '%s\n' "$out" | grep -q 'test timed out'; then
    say Q6 "goroutine dump (first 220 lines):"
    printf '%s\n' "$out" | sed -n '/test timed out/,$p' | head -220 | sed 's/^/Q6:   /'
  else
    printf '%s\n' "$out" | tail -12 | sed 's/^/Q6:   /'
  fi
}

# syscall_names FILE: the system calls of a strace log, with their counts, most frequent first.
syscall_names() {
  sed -E 's/^[0-9]+ +//' "$1" | grep -oE '^[a-z_0-9]+\(' | tr -d '(' | sort | uniq -c | sort -rn | awk '{printf "%s×%s ", $2, $1}'
}

q7() {
  command -v strace >/dev/null 2>&1 || { say Q7 "strace: absent"; return; }
  local arch dir gz name bin
  case $(uname -m) in x86_64) arch=amd64 ;; aarch64) arch=arm64 ;; *) arch=$(uname -m) ;; esac
  dir="internal/sidecar/bin/linux_$arch"
  if ! compgen -G "$dir/*.gz" >/dev/null; then
    say Q7 "no fetched sidecar in $dir (run: go run ./cmd/sidecar-fetch)"
    return
  fi
  for gz in "$dir"/*.gz; do
    name=$(basename "$gz" .gz)
    bin="$work/$name"
    gunzip -c "$gz" >"$bin" && chmod 0700 "$bin"
    timeout -k 5 60 strace -f -o "$work/$name.outside" "$bin" --version >/dev/null 2>&1
    say Q7 "$name --version outside the sandbox: exit=$? calls: $(syscall_names "$work/$name.outside" | cut -c1-1500)"
  done
  # Inside the sandbox: the package's own launcher on the pinned binaries, traced; the calls a filter
  # or the kernel refused show as ENOSYS (not in the allow-list) or EPERM (explicit denial).
  timeout -k 10 300 strace -f -o "$work/inside" go test ./internal/sidecar -run '^TestEmbeddedSidecarsRun$' -count=1 -timeout 120s >"$work/inside.out" 2>&1
  say Q7 "TestEmbeddedSidecarsRun under strace: exit=$? $(grep -E '^(--- |ok|FAIL)' "$work/inside.out" | tr '\n' '|' | cut -c1-600)"
  local errno
  for errno in ENOSYS EPERM EACCES; do
    say Q7 "calls answered $errno inside: $(grep -E "= -1 $errno" "$work/inside" | sed -E 's/^[0-9]+ +//' | grep -oE '^[a-z_0-9]+\(' | tr -d '(' | sort | uniq -c | sort -rn | awk '{printf "%s×%s ", $2, $1}' | cut -c1-900)"
  done
  say Q7 "signals inside: $(grep -oE '(\+\+\+ killed by [A-Z]+|--- SIG[A-Z]+)' "$work/inside" | sort | uniq -c | sort -rn | awk '{printf "%s %s %s %s×%s ", $2, $3, $4, $5, $1}' | cut -c1-400)"
  say Q7 "last calls before each kill: $(grep -B3 -E '\+\+\+ killed by' "$work/inside" | sed -E 's/^[0-9]+ +//' | cut -c1-110 | tr '\n' '|' | cut -c1-900)"
}

# load_profile NAME PATH: writes and loads a candidate profile for the binary at PATH, in the form
# Ubuntu 24.04 ships for the applications it allows (compare with the files q1 prints).
load_profile() {
  local name=$1 path=$2 file="/etc/apparmor.d/$1"
  printf 'abi <abi/4.0>,\ninclude <tunables/global>\n\nprofile %s %s flags=(unconfined) {\n  userns,\n}\n' "$name" "$path" | sudo tee "$file" >/dev/null
  try Q8 "apparmor_parser -r $file" sudo apparmor_parser -r "$file"
}

q8() {
  if [[ $sudo_ok != 1 ]]; then
    say Q8 "skipped: needs INKWELL_PROBE_SUDO=1 (loads two test AppArmor profiles)"
    return
  fi
  if ! command -v apparmor_parser >/dev/null 2>&1; then
    say Q8 "apparmor_parser: absent"
    return
  fi
  local probe=/usr/local/bin/inkwell-sandbox-probe copy=/usr/local/bin/inkwell-probe-unshare
  if ! go test -c -o "$work/probe.test" ./internal/sidecar >"$work/build.out" 2>&1; then
    say Q8 "building the test binary failed: $(tail -3 "$work/build.out" | tr '\n' '|')"
    return
  fi
  sudo install -m 0755 "$work/probe.test" "$probe"
  sudo install -m 0755 "$(command -v unshare)" "$copy"
  say Q8 "before any profile:"
  unshare_matrix Q8 "$copy"
  go_probe Q8 "$probe"
  load_profile inkwell-sandbox-probe "$probe"
  load_profile inkwell-probe-unshare "$copy"
  say Q8 "with the candidate profiles loaded:"
  unshare_matrix Q8 "$copy"
  go_probe Q8 "$probe"
  # The same test binary from another path is not covered by the profile: what a non-packaged run gets.
  install -m 0700 "$work/probe.test" "$work/elsewhere.test"
  say Q8 "the same binary from an unprofiled path:"
  go_probe Q8 "$work/elsewhere.test"
  sudo rm -f /etc/apparmor.d/inkwell-sandbox-probe /etc/apparmor.d/inkwell-probe-unshare "$probe" "$copy"
}

questions=("$@")
[[ ${#questions[@]} -gt 0 ]] || questions=(q1 q2 q3 q5 q6 q7 q8)
for q in "${questions[@]}"; do
  case $q in
  q1 | q2 | q3 | q5 | q6 | q7 | q8) "$q" ;;
  q4) q3 ;; # Landlock is answered by the same Go test as q3
  *)
    echo "sandbox-probe: unknown question $q (q1 q2 q3 q4 q5 q6 q7 q8)" >&2
    exit 2
    ;;
  esac
done
