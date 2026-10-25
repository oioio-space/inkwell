//go:build unix

package project

import (
	"io/fs"
	"syscall"
)

// hardLinks returns the number of names of the file; 1 where it cannot tell.
func hardLinks(info fs.FileInfo) uint64 {
	if st, ok := info.Sys().(*syscall.Stat_t); ok {
		return uint64(st.Nlink) //nolint:unconvert // uint16 on some BSDs
	}
	return 1
}
