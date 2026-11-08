package project

import (
	"fmt"
	"io/fs"
	"syscall"
)

func snapshotFileVersion(info fs.FileInfo) string {
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return ""
	}
	return fmt.Sprintf("%d:%d:%d:%d:%d:%d", stat.Dev, stat.Ino, info.Size(), info.ModTime().UnixNano(), stat.Ctimespec.Sec, stat.Ctimespec.Nsec)
}
