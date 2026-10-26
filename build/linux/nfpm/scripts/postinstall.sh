#!/bin/sh
# Runs after the files are installed (deb: postinst, rpm: %post): refresh the caches the desktop
# reads, so the menu entry, the icon and the .typ association show at once. Every step is
# optional and never fails the installation (a server has none of these tools).
refresh() {
  command -v "$1" >/dev/null 2>&1 || return 0
  "$@" >/dev/null 2>&1 || true
}

refresh update-desktop-database -q /usr/share/applications
refresh update-mime-database /usr/share/mime
# The icon cache only exists where something built it; keep it in step with the new icons.
if [ -f /usr/share/icons/hicolor/icon-theme.cache ]; then
  refresh gtk4-update-icon-cache -q -t -f /usr/share/icons/hicolor
  refresh gtk-update-icon-cache -q -t -f /usr/share/icons/hicolor
fi
exit 0
