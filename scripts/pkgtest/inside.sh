#!/usr/bin/env bash
# shellcheck disable=SC2329 # the checks are run through check(), which shellcheck cannot see
# Runs INSIDE a fresh distribution container (Debian, Ubuntu or Fedora); driven by run.sh, which
# mounts the packages read-only on /pkg (and the previous release on /prev). One phase per call:
#
#   tools       install what the checks need (not what Inkwell needs) and record the file list
#   install     PKG-01/02/03/07/10/11: install the package, resolve its dependencies, check it
#   smoke       PKG-01/04: start the installed app headless, with the network cut
#   uninstall   PKG-06: remove the package and prove nothing is left behind, the home untouched
#   upgrade     PKG-08: install the previous package, create a project, upgrade, check
#
# Environment: PKG_FILE (file name in /pkg), EXPECT_VERSION (SemVer), PKG_PREV_FILE (in /prev),
# PKG_SIGNED=1 with /pkg/public.asc when the packages were signed (PKG-11).
# Every check prints "ok", "FAIL" or "skip" with its PKG id; the exit status is the number of FAIL.
set -uo pipefail

phase="${1:?usage: inside.sh PHASE}"
state=/var/tmp/pkgtest # survives the phases (committed image) but is excluded from the file lists
mkdir -p "$state"
failures=0
app_id=io.github.oioio_space.Inkwell

ok() { printf 'ok    %s\n' "$*"; }
skip() { printf 'skip  %s\n' "$*"; }
fail() {
  printf 'FAIL  %s\n' "$*"
  failures=$((failures + 1))
}
# check ID DESCRIPTION CMD...: runs CMD quietly; ok when it succeeds.
check() {
  local id="$1" what="$2"
  shift 2
  local out
  if out="$("$@" 2>&1)"; then ok "$id $what"; else
    fail "$id $what"
    printf '%s\n' "$out" | sed 's/^/        /' | head -20
  fi
}

if command -v apt-get >/dev/null 2>&1; then family=deb; else family=rpm; fi
export DEBIAN_FRONTEND=noninteractive

# --- package manager abstraction --------------------------------------------------------------
pm_install() { # pm_install FILE...: install local packages, resolving dependencies from the repositories
  if [[ $family == deb ]]; then apt-get install -y --no-install-recommends "$@"; else dnf -y --setopt=install_weak_deps=False install "$@"; fi
}
pm_remove() { # pm_remove: remove inkwell and the dependencies nothing else needs
  # Recommends do not keep a dependency installed: otherwise gnupg (a test tool) would keep
  # dbus-user-session, and with it systemd, which Inkwell's dependencies pulled in.
  if [[ $family == deb ]]; then apt-get purge -y --autoremove -o APT::AutoRemove::RecommendsImportant=false inkwell; else dnf -y remove inkwell; fi
}
pm_installed_version() {
  if [[ $family == deb ]]; then dpkg-query -W -f='${Version}' inkwell; else rpm -q --qf '%{VERSION}' inkwell; fi
}
# The package version as SemVer: "0.1.0~rc.1-1" (deb) and "0.1.0~rc.1" (rpm) are 0.1.0-rc.1.
semver_of() { sed -e 's/~/-/' -e 's/-1$//' <<<"$1"; }

# The list of files outside the places the system changes by itself (package databases, caches,
# logs, temporary files, home directories): what an installation could leave behind.
filelist() {
  find / -xdev \( -path /proc -o -path /sys -o -path /dev -o -path /tmp -o -path /run -o -path /var/tmp \
    -o -path /var/cache -o -path /var/log -o -path /var/lib/dpkg -o -path /var/lib/apt -o -path /var/lib/rpm \
    -o -path /var/lib/dnf -o -path /usr/lib/sysimage -o -path /root -o -path /home -o -path /etc/ld.so.cache \
    -o -path /etc/alternatives -o -path /var/lib/systemd -o -path /usr/share/mime -o -path /usr/share/icons/hicolor/icon-theme.cache \) \
    -prune -o -print | sort
}

# --- phase: tools ----------------------------------------------------------------------------
tools() {
  if [[ $family == deb ]]; then
    apt-get update -qq &&
      apt-get install -y -qq --no-install-recommends desktop-file-utils appstream shared-mime-info gnupg \
        xvfb xauth xdotool x11-apps python3 procps dbus-bin dbus-daemon ca-certificates file binutils >/dev/null
  else
    dnf -y -q --setopt=install_weak_deps=False install desktop-file-utils appstream shared-mime-info gnupg2 \
      xorg-x11-server-Xvfb xorg-x11-xauth xdotool xwd python3 procps-ng dbus-daemon file binutils >/dev/null
  fi || { fail "tools: installing the test tools"; return; }
  filelist >"$state/before.list"
  ok "tools: $(wc -l <"$state/before.list") files before the installation"
}

# --- checks on an installed package ----------------------------------------------------------
has_icons() {
  local n
  n="$(find /usr/share/icons/hicolor -name "$app_id*" | wc -l)"
  [[ $n -eq 10 ]] || { echo "found $n icon files, want 10 (8 PNG, scalable, symbolic)"; return 1; }
}
mime_registered() {
  grep -q '\*\.typ$' /usr/share/mime/globs && grep -q "text/x-typst=.*$app_id.desktop" /usr/share/applications/mimeinfo.cache
}
no_missing_libs() {
  local missing
  missing="$(ldd /usr/bin/inkwell 2>&1 | grep 'not found')" || return 0
  echo "$missing"
  return 1
}
version_matches() {
  local got
  got="$(inkwell --version)" || return 1
  [[ $got == "inkwell $EXPECT_VERSION "* || $got == "inkwell $EXPECT_VERSION" ]] || { echo "inkwell --version = $got, want $EXPECT_VERSION"; return 1; }
}
package_version_matches() {
  local got
  got="$(semver_of "$(pm_installed_version)")"
  [[ $got == "$EXPECT_VERSION" ]] || { echo "package version $got, want $EXPECT_VERSION"; return 1; }
}
icon_set_matches_version() { # a pre-release has the striped (nightly) icon, a release the plain one
  local svg=/usr/share/icons/hicolor/scalable/apps/$app_id.svg
  if [[ $EXPECT_VERSION == *-* ]]; then grep -q 'stripes' "$svg"; else ! grep -q 'stripes' "$svg"; fi
}

verify_signature() {
  if [[ ${PKG_SIGNED:-} != 1 ]]; then skip "PKG-11 signature (packages are unsigned; PKG_SIGN=1 builds and checks signed ones)"; return; fi
  export GNUPGHOME="$state/gnupg"
  mkdir -p "$GNUPGHOME" && chmod 700 "$GNUPGHOME"
  gpg --batch --quiet --import /pkg/public.asc || { fail "PKG-11 importing the public key"; return; }
  if [[ $family == deb ]]; then
    # nfpm signs like dpkg-sig: an ar member _gpgorigin over debian-binary + control + data.
    local pkg=/pkg/$PKG_FILE members
    members="$(ar t "$pkg" | grep -v '^_gpg')"
    ar t "$pkg" | grep -q '^_gpgorigin$' || { fail "PKG-11 the .deb has no _gpgorigin signature"; return; }
    # shellcheck disable=SC2086 # the member names are plain words
    if (cd "$state" && rm -f _gpgorigin $members && ar x "$pkg" && cat $members | gpg --batch --verify _gpgorigin - 2>&1 | grep -q 'Good signature'); then
      ok "PKG-11 .deb signature is good"
    else fail "PKG-11 .deb signature does not verify"; fi
  else
    rpm --import /pkg/public.asc
    local out
    out="$(rpm -K "/pkg/$PKG_FILE" 2>&1)"
    if grep -qi 'signatures OK\|pgp.*OK' <<<"$out" && ! grep -qi 'NOT OK\|NOKEY' <<<"$out"; then ok "PKG-11 .rpm signature is good ($out)"; else fail "PKG-11 .rpm signature: $out"; fi
  fi
}

run_checks() {
  check PKG-01 "dependencies resolved, no shared library missing" no_missing_libs
  check PKG-07 "inkwell --version is $EXPECT_VERSION" version_matches
  check PKG-07 "the package manager reports version $EXPECT_VERSION" package_version_matches
  check PKG-10 "desktop entry is valid" desktop-file-validate "/usr/share/applications/$app_id.desktop"
  check PKG-10 "AppStream metadata is valid" appstreamcli validate --no-net "/usr/share/metainfo/$app_id.metainfo.xml"
  check PKG-10 "hicolor icons are installed (PNG 16 to 512, scalable, symbolic)" has_icons
  check PKG-10 "the icon set matches the version ($([[ $EXPECT_VERSION == *-* ]] && echo nightly || echo stable))" icon_set_matches_version
  check PKG-10 "*.typ is text/x-typst and opens with Inkwell (mimeinfo.cache)" mime_registered
  if grep -qs 'path-exclude.*/usr/share/man' /etc/dpkg/dpkg.cfg.d/* 2>/dev/null; then
    # Docker's Ubuntu image drops man pages at unpack time; the package still lists it.
    check PKG-10 "the manual page is in the package (this image excludes man pages)" bash -c 'dpkg -L inkwell | grep -qx /usr/share/man/man1/inkwell.1.gz'
  else
    check PKG-10 "the manual page is installed" test -s /usr/share/man/man1/inkwell.1.gz
  fi
  check PKG-10 "the executable is /usr/bin/inkwell" test -x /usr/bin/inkwell
  verify_signature
}

# --- phase: install --------------------------------------------------------------------------
install() {
  local start
  start="$(date +%s)"
  if pm_install "/pkg/$PKG_FILE" >"$state/install.log" 2>&1; then ok "PKG-01 installs on a clean system ($(($(date +%s) - start)) s, dependencies resolved)"; else
    fail "PKG-01 installation"
    tail -20 "$state/install.log" | sed 's/^/        /'
    return
  fi
  run_checks
}

# --- phase: smoke ----------------------------------------------------------------------------
# Counts the different colours of an X screenshot (xwd): a blank or black window has one or two.
screenshot_colours() {
  python3 -I - "$1" <<'EOF'
import struct, sys
data = open(sys.argv[1], "rb").read()
hdr = struct.unpack(">25I", data[:100])
header_size, width, height, bits, ncolors = hdr[0], hdr[4], hdr[5], hdr[11], hdr[19]
pixels = data[header_size + ncolors * 12:]
step = bits // 8
print(len({pixels[i:i + step] for i in range(0, min(len(pixels), width * height * step), step)}))
EOF
}

smoke() {
  export HOME="$state/smoke-home" XDG_RUNTIME_DIR="$state/xdg" DISPLAY=:99
  rm -rf "$HOME" "$XDG_RUNTIME_DIR"
  mkdir -p "$HOME" "$XDG_RUNTIME_DIR" && chmod 700 "$HOME" "$XDG_RUNTIME_DIR"
  # A container has neither user namespaces for WebKit's sandbox nor a GPU: software rendering.
  export WEBKIT_DISABLE_SANDBOX_THIS_IS_DANGEROUS=1 WEBKIT_DISABLE_DMABUF_RENDERER=1 WEBKIT_DISABLE_COMPOSITING_MODE=1
  export GSK_RENDERER=cairo LIBGL_ALWAYS_SOFTWARE=1 GDK_BACKEND=x11 NO_AT_BRIDGE=1

  Xvfb :99 -screen 0 1280x800x24 -nolisten tcp >"$state/xvfb.log" 2>&1 &
  local xvfb=$!
  for _ in $(seq 50); do [[ -S /tmp/.X11-unix/X99 ]] && break; sleep 0.2; done
  [[ -S /tmp/.X11-unix/X99 ]] || { fail "PKG-01 smoke: the X server did not start"; cat "$state/xvfb.log"; return; }

  # The network is cut by run.sh (podman --network none): the offline start is PKG-04.
  local has_net=0 nic
  for nic in /sys/class/net/*; do [[ ${nic##*/} == lo ]] || has_net=$((has_net + 1)); done
  if [[ $has_net -eq 0 ]]; then ok "PKG-04 smoke runs without any network interface but loopback"; else skip "PKG-04 the container still has a network ($has_net interface(s))"; fi

  dbus-run-session -- inkwell >"$state/app.log" 2>&1 &
  local app=$!
  local win=""
  for _ in $(seq 100); do
    win="$(xdotool search --name '^Inkwell$' 2>/dev/null | head -1)"
    [[ -n $win ]] && break
    kill -0 "$app" 2>/dev/null || break
    sleep 0.5
  done
  if [[ -n $win ]]; then ok "PKG-01 the Inkwell window appears (id $win)"; else
    fail "PKG-01 no Inkwell window"
    tail -30 "$state/app.log" | sed 's/^/        /'
    kill "$app" "$xvfb" 2>/dev/null
    return
  fi

  sleep 10 # the webview loads the embedded frontend
  check PKG-01 "the application is still running" kill -0 "$app"
  check PKG-01 "the web content process of WebKit runs" pgrep -f WebKitWebProcess
  check PKG-01 "the data folder was created in the home directory" test -d "$HOME/Documents/Inkwell"
  xwd -root -display :99 -silent -out "$state/shot.xwd" 2>/dev/null
  local colours
  colours="$(screenshot_colours "$state/shot.xwd" 2>&1)"
  if [[ $colours =~ ^[0-9]+$ && $colours -gt 20 ]]; then ok "PKG-01 the window shows the interface ($colours distinct colours on screen)"; else fail "PKG-01 the screen is blank or unreadable ($colours colours)"; fi
  if grep -qiE 'panic|fatal|cannot open display|Failed to' "$state/app.log"; then
    fail "PKG-01 the application log has errors"
    grep -iE 'panic|fatal|cannot open display|Failed to' "$state/app.log" | head -5 | sed 's/^/        /'
  else ok "PKG-01 no error in the application log"; fi

  kill -TERM "$app" 2>/dev/null
  for _ in $(seq 40); do kill -0 "$app" 2>/dev/null || break; sleep 0.25; done
  if kill -0 "$app" 2>/dev/null; then fail "PKG-01 the application ignores SIGTERM"; kill -KILL "$app"; else ok "PKG-01 the application stops on SIGTERM"; fi
  kill "$xvfb" 2>/dev/null
}

# --- phase: uninstall ------------------------------------------------------------------------
uninstall() {
  export HOME=/root
  mkdir -p "$HOME/Documents/Inkwell"
  printf '= My thesis\nNothing may touch this.\n' >"$HOME/Documents/Inkwell/thesis.typ"
  local sum
  sum="$(sha256sum "$HOME/Documents/Inkwell/thesis.typ")"

  # What the package installed, according to the package manager (files and links, not directories).
  if [[ $family == deb ]]; then dpkg -L inkwell; else rpm -ql inkwell; fi | while read -r f; do
    [[ -f $f || -L $f ]] && echo "$f"
  done >"$state/owned.list"
  local owned
  owned="$(wc -l <"$state/owned.list")"

  pm_remove >"$state/remove.log" 2>&1 || { fail "PKG-06 removal"; tail -20 "$state/remove.log" | sed 's/^/        /'; return; }
  ok "PKG-06 the package is removed (with the dependencies nothing else needs)"

  local left=""
  while read -r f; do
    [[ -e $f || -L $f ]] && left+="$f"$'\n'
  done <"$state/owned.list"
  if [[ -z $left && $owned -gt 10 ]]; then ok "PKG-06 all $owned files of the package are gone"; else
    fail "PKG-06 files of the package are still there ($owned owned)"
    printf '%s' "$left" | head -20 | sed 's/^/        /'
  fi
  # Nothing named after Inkwell survives anywhere but the home directory (projects, identity).
  left="$(find / -xdev \( -path /proc -o -path /sys -o -path /root -o -path /home -o -path /tmp -o -path /var/tmp -o -path /var/cache \) -prune -o -iname '*inkwell*' -print)"
  if [[ -z $left ]]; then ok "PKG-06 nothing named inkwell is left outside the home directory"; else
    fail "PKG-06 leftovers named after inkwell"
    printf '%s\n' "$left" | head -20 | sed 's/^/        /'
  fi
  # Informative: files that the removed dependencies' own packaging leaves (machine-id, shadow backups...).
  filelist >"$state/after.list"
  skip "PKG-06 info: $(comm -13 "$state/before.list" "$state/after.list" | wc -l) files outside the package remain from dependencies' own packaging (not Inkwell's)"
  if [[ ! -e /usr/bin/inkwell ]]; then ok "PKG-06 /usr/bin/inkwell is gone"; else fail "PKG-06 /usr/bin/inkwell is still there"; fi
  if ! grep -qs "$app_id" /usr/share/applications/mimeinfo.cache /usr/share/mime/globs2 2>/dev/null; then ok "PKG-06 the desktop caches no longer mention Inkwell"; else fail "PKG-06 the desktop caches still mention Inkwell"; fi
  if [[ "$(sha256sum "$HOME/Documents/Inkwell/thesis.typ")" == "$sum" ]]; then ok "PKG-06 the projects in the home directory are untouched"; else fail "PKG-06 a project was changed"; fi
}

# --- phase: upgrade --------------------------------------------------------------------------
upgrade() {
  if [[ -z ${PKG_PREV_FILE:-} || ! -f /prev/${PKG_PREV_FILE:-x} ]]; then skip "PKG-08 no previous package"; return; fi
  local prev=/prev/$PKG_PREV_FILE
  pm_install "$prev" >"$state/prev.log" 2>&1 || { fail "PKG-08 installing the previous package"; tail -10 "$state/prev.log"; return; }
  ok "PKG-08 the previous package installs ($(inkwell --version))"

  export HOME=/root
  mkdir -p "$HOME/Documents/Inkwell"
  printf '= Draft\nWritten with the previous version.\n' >"$HOME/Documents/Inkwell/draft.typ"
  local sum
  sum="$(sha256sum "$HOME/Documents/Inkwell/draft.typ")"

  if pm_install "/pkg/$PKG_FILE" >"$state/upgrade.log" 2>&1; then ok "PKG-08 the upgrade installs over the previous version"; else
    fail "PKG-08 the upgrade"
    tail -20 "$state/upgrade.log" | sed 's/^/        /'
    return
  fi
  check PKG-08 "the upgraded executable is $EXPECT_VERSION" version_matches
  check PKG-08 "the package manager reports $EXPECT_VERSION, once" package_version_matches
  if [[ "$(sha256sum "$HOME/Documents/Inkwell/draft.typ")" == "$sum" ]]; then ok "PKG-08 the project opened before the upgrade is intact"; else fail "PKG-08 the project was changed by the upgrade"; fi
  check PKG-08 "icons still there after the upgrade" has_icons
  check PKG-08 "the desktop caches still know Inkwell" mime_registered
}

case "$phase" in
tools) tools ;;
install) install ;;
smoke) smoke ;;
uninstall) uninstall ;;
upgrade) upgrade ;;
*) echo "unknown phase $phase" >&2; exit 64 ;;
esac
exit "$failures"
