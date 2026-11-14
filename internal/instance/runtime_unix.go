//go:build !darwin && !windows

package instance

import (
	"os"
	"path/filepath"
	"strconv"
	"syscall"
)

// runtimeDir returns the private folder of the second socket candidate: $XDG_RUNTIME_DIR when it
// is the user's own with mode 0700 (XDG base directory specification), else
// os.TempDir()/inkwell-<uid>, created 0700 when create is set (the primary; a second launch
// creates nothing). A folder that is not a directory of the user with mode 0700 (a symlink, a
// folder another user made first in /tmp) is skipped: "" then.
func runtimeDir(create bool) string {
	return runtimeDirFrom(os.Getenv("XDG_RUNTIME_DIR"), os.TempDir(), os.Getuid(), create)
}

func runtimeDirFrom(xdg, tmp string, uid int, create bool) string {
	if privateDir(xdg, uid) {
		return xdg
	}
	dir := filepath.Join(tmp, "inkwell-"+strconv.Itoa(uid))
	if create {
		_ = os.Mkdir(dir, 0o700) // an existing one is checked below like a new one
	}
	if privateDir(dir, uid) {
		return dir
	}
	return ""
}

// privateDir reports whether dir is an absolute path to a directory, not a symlink, owned by uid,
// with mode 0700. A relative path (an empty or relative $XDG_RUNTIME_DIR or $TMPDIR) would make
// the socket depend on the working directory, so it is refused.
func privateDir(dir string, uid int) bool {
	if !filepath.IsAbs(dir) {
		return false
	}
	// #nosec G703 -- dir is absolute, and Lstat only reads metadata and follows no symlink; the
	// result is trusted only for a directory of uid with mode 0700 (ADR 0037).
	info, err := os.Lstat(dir)
	if err != nil || !info.IsDir() || info.Mode().Perm() != 0o700 {
		return false
	}
	st, ok := info.Sys().(*syscall.Stat_t)
	return ok && int64(st.Uid) == int64(uid)
}
