//go:build !linux

package sidecar

import "testing"

// Outside Linux there is no tier: every launch runs and may create processes.

func requireN(*testing.T) bool { return true }

func requireLaunch(*testing.T) bool { return true }

func forksInSandbox(*testing.T) bool { return true }
