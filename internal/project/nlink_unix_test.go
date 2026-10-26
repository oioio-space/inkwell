//go:build unix

package project

import (
	"os"
	"syscall"
	"testing"
)

// linkCount returns the number of hard links to the file name of base.
func linkCount(t *testing.T, base *os.Root, name string) uint64 {
	t.Helper()
	info, err := base.Stat(name)
	if err != nil {
		t.Fatal(err)
	}
	st, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		t.Skip("no link count on this platform")
	}
	return uint64(st.Nlink) //nolint:unconvert // uint16 on some BSDs
}
