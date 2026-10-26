#!/bin/sh
# Runs after the files are removed (deb: postrm, rpm: %postun): drop the menu entry, the icons
# and the file type from the caches the desktop reads. Nothing in the home directory is touched:
# the projects and the identity of the user stay (docs/adr/0025).
refresh() {
  command -v "$1" >/dev/null 2>&1 || return 0
  "$@" >/dev/null 2>&1 || true
}

refresh update-desktop-database -q /usr/share/applications
refresh update-mime-database /usr/share/mime
if [ -f /usr/share/icons/hicolor/icon-theme.cache ]; then
  refresh gtk4-update-icon-cache -q -t -f /usr/share/icons/hicolor
  refresh gtk-update-icon-cache -q -t -f /usr/share/icons/hicolor
fi
exit 0
