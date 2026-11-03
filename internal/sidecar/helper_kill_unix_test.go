//go:build unix

package sidecar

import "syscall"

func syscallKill(pid int) error { return syscall.Kill(pid, syscall.SIGKILL) }
