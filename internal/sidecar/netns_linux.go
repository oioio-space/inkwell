package sidecar

import (
	"context"
	"os"
	"os/exec"
	"syscall"
	"time"
)

// isolate gives the child a private user and network namespace: it sees only a
// loopback of its own, down, so not even 127.0.0.1 leads anywhere. The user
// namespace maps the caller's ids onto themselves, so file access is unchanged.
func isolate(cmd *exec.Cmd) {
	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}
	a := cmd.SysProcAttr
	a.Cloneflags |= syscall.CLONE_NEWUSER | syscall.CLONE_NEWNET | syscall.CLONE_NEWPID | syscall.CLONE_NEWIPC | syscall.CLONE_NEWNS
	if len(a.UidMappings) > 0 {
		return
	}
	a.UidMappings = []syscall.SysProcIDMap{{ContainerID: os.Getuid(), HostID: os.Getuid(), Size: 1}}
	a.GidMappings = []syscall.SysProcIDMap{{ContainerID: os.Getgid(), HostID: os.Getgid(), Size: 1}}
	a.GidMappingsEnableSetgroups = false
}

// probeIsolation tries to start a harmless program in the namespaces. Kernels
// and distributions may forbid unprivileged user namespaces (sysctl, AppArmor
// on Ubuntu 24.04, containers). As for probeOutputMount (spec §9.11): only a
// definitive refusal is cached as "no isolation", which leaves NoNetwork to the
// caller's proxy settings; a timeout or transient error refuses the launch with
// ErrProbeInconclusive and is probed again after a backoff.
var probeIsolation = cachedProbe(func(ctx context.Context) (bool, error) {
	path, args, err := sandboxPrefix(sandboxConfig{Probe: true})
	if err != nil {
		return false, err
	}
	return runNamespaceProbe(ctx, isolate, []string{}, path, args...)
}, probeTimeout, time.Now)
