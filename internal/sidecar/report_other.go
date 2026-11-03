//go:build !linux

package sidecar

import "os/exec"

func signalReport(_ *exec.Cmd) (func(*Output), func(), error) {
	return func(*Output) {}, func() {}, nil
}
