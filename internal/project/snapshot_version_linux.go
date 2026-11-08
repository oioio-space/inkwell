package project

import (
	"fmt"
	"io/fs"
	"syscall"
)

// ctime cannot be restored with Chtimes: even same-size external edits with a
// restored mtime invalidate content reuse. Device/inode detect atomic replacement.
func snapshotFileVersion(info fs.FileInfo) string {
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return ""
	}
	return fmt.Sprintf("%d:%d:%d:%d:%d:%d", stat.Dev, stat.Ino, info.Size(), info.ModTime().UnixNano(), stat.Ctim.Sec, stat.Ctim.Nsec)
}
