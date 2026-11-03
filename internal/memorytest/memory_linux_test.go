//go:build linux

package memorytest

import (
	"runtime"
	"testing"
	"unsafe"

	"github.com/pbnjay/memory"
	"golang.org/x/sys/unix"
)

func TestLinuxMemory(t *testing.T) {
	total := memory.TotalMemory()
	free := memory.FreeMemory()
	if total == 0 || free == 0 || free > total {
		t.Fatalf("invalid memory: free=%d total=%d", free, total)
	}
}

func TestUnavailableMemory(t *testing.T) {
	// Model a kernel denying sysinfo. Destroy the confined thread afterward.
	done := make(chan error, 1)
	go func() {
		runtime.LockOSThread()
		f := []unix.SockFilter{
			{Code: unix.BPF_LD | unix.BPF_W | unix.BPF_ABS, K: 0},
			{Code: unix.BPF_JMP | unix.BPF_JEQ | unix.BPF_K, K: unix.SYS_SYSINFO, Jf: 1},
			{Code: unix.BPF_RET | unix.BPF_K, K: unix.SECCOMP_RET_ERRNO | uint32(unix.EPERM)},
			{Code: unix.BPF_RET | unix.BPF_K, K: unix.SECCOMP_RET_ALLOW},
		}
		err := unix.Prctl(unix.PR_SET_NO_NEW_PRIVS, 1, 0, 0, 0)
		prog := unix.SockFprog{Len: uint16(len(f)), Filter: &f[0]}
		if err == nil {
			err = unix.Prctl(unix.PR_SET_SECCOMP, unix.SECCOMP_MODE_FILTER, uintptr(unsafe.Pointer(&prog)), 0, 0)
		}
		if err == nil && (memory.TotalMemory() != 0 || memory.FreeMemory() != 0) {
			err = unix.EINVAL
		}
		done <- err
	}()
	if err := <-done; err != nil {
		t.Fatal(err)
	}
}
