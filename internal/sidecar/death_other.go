//go:build !linux

package sidecar

import "os/exec"

type SandboxDeath struct {
	PID   int
	NSPID []int
	Comm  string
}

func recordRunManifest(string, Output) error { return nil }

func captureDeath(*exec.Cmd) *SandboxDeath { return nil }
