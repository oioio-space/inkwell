package sidecar

import (
	"os/exec"
	"syscall"
)

// orphanSafe reports whether the kernel kills a child when its parent dies.
const orphanSafe = true

// setProcAttr puts the child in its own process group, so the whole tree can
// be signalled, and asks the kernel to SIGKILL it when its parent dies.
//
// Pdeathsig is tied to the thread that created the child, not to the process:
// callers keep their goroutine on that thread with runtime.LockOSThread from
// Start to Wait (golang/go#27505).
func setProcAttr(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true, Pdeathsig: syscall.SIGKILL}
}
