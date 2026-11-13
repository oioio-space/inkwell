//go:build !linux

package sidecar

import (
	"context"
	"os/exec"
)

type SandboxScope struct{}
type cgroupRun struct{}

var appSandbox SandboxScope

func (*SandboxScope) prepare(context.Context, Spec, *exec.Cmd) *cgroupRun { return nil }
func (*cgroupRun) started()                                               {}
func (*cgroupRun) finish(int) MemoryCause                                 { return MemoryNone }
func (*cgroupRun) placementFailed()                                       {}

func (*cgroupRun) start(cmd *exec.Cmd) error { return cmd.Start() }

func (*cgroupRun) afterStart() {}

func (*cgroupRun) placementError(error) bool { return false }
