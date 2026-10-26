//go:build unix

package sidecar

import "syscall"

// signalGroup sends sig to the process group led by pid.
func signalGroup(pid int, sig syscall.Signal) error {
	return syscall.Kill(-pid, sig)
}
