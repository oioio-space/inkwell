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
	if filepath.IsAbs(xdg) && privateDir(xdg, uid) {
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

// privateDir reports whether dir is a directory, not a symlink, owned by uid, with mode 0700.
func privateDir(dir string, uid int) bool {
	info, err := os.Lstat(dir)
	if err != nil || !info.IsDir() || info.Mode().Perm() != 0o700 {
		return false
	}
	st, ok := info.Sys().(*syscall.Stat_t)
	return ok && int64(st.Uid) == int64(uid)
}
