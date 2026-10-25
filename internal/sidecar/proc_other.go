//go:build !unix

package sidecar

import (
	"os"
	"os/exec"
	"syscall"
)

// orphanSafe is false here: no process group and no parent-death signal.
const orphanSafe = false

func setProcAttr(*exec.Cmd) {}

// signalGroup can only reach the process itself on this platform.
func signalGroup(pid int, _ syscall.Signal) error {
	p, err := os.FindProcess(pid)
	if err != nil {
		return err
	}
	return p.Kill()
}
