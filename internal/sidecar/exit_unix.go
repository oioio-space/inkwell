//go:build unix

package sidecar

import (
	"os"
	"syscall"
)

// exitSignal is the signal that ended the process, 0 if it exited by itself.
func exitSignal(ps *os.ProcessState) int {
	if ws, ok := ps.Sys().(syscall.WaitStatus); ok && ws.Signaled() {
		return int(ws.Signal())
	}
	return 0
}
