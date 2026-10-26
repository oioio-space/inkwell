//go:build unix && !linux

package sidecar

import (
	"os/exec"
	"syscall"
)

// orphanSafe is false here: there is no parent-death signal, so a parent
// killed with SIGKILL can leave its child running.
const orphanSafe = false

// setProcAttr puts the child in its own process group.
func setProcAttr(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
}
