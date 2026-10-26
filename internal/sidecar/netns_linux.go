package sidecar

import (
	"os"
	"os/exec"
	"sync"
	"syscall"
)

// isolate gives the child a private user and network namespace: it sees only a
// loopback of its own, down, so not even 127.0.0.1 leads anywhere. The user
// namespace maps the caller's ids onto themselves, so file access is unchanged.
func isolate(cmd *exec.Cmd) {
	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}
	a := cmd.SysProcAttr
	a.Cloneflags |= syscall.CLONE_NEWUSER | syscall.CLONE_NEWNET
	a.UidMappings = []syscall.SysProcIDMap{{ContainerID: os.Getuid(), HostID: os.Getuid(), Size: 1}}
	a.GidMappings = []syscall.SysProcIDMap{{ContainerID: os.Getgid(), HostID: os.Getgid(), Size: 1}}
	a.GidMappingsEnableSetgroups = false
}

// probeIsolation tries to start a harmless program in the namespaces. Kernels
// and distributions may forbid unprivileged user namespaces (sysctl, AppArmor
// on Ubuntu 24.04, containers).
var probeIsolation = sync.OnceValue(func() bool {
	cmd := exec.Command(shell, "-c", "exit 0") // #nosec G204 -- constant command
	cmd.Env = []string{}
	setProcAttr(cmd)
	isolate(cmd)
	return cmd.Run() == nil
})
