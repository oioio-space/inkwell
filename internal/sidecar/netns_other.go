//go:build !linux

package sidecar

import "os/exec"

// There is no unprivileged network namespace here. On macOS a sandbox-exec
// profile denying network access would do it; it is not wired (the tool is
// deprecated) and the callers' proxy settings remain the only barrier.
func isolate(*exec.Cmd) {}

func probeIsolation() bool { return false }
