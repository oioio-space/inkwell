//go:build !linux

package sidecar

import "os/exec"

type directoryMount struct{}

func newDirectoryMount(Spec) (*directoryMount, error) { return nil, nil }
func (*directoryMount) configure(*exec.Cmd) error     { return nil }
func (*directoryMount) open(int) error                { return nil }
func (*directoryMount) export() error                 { return nil }
func (*directoryMount) close()                        {}

func (*directoryMount) limitHit(Output) bool { return false }
