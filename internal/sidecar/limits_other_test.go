//go:build !linux

package sidecar

// reportLimits is only meaningful on Linux, where the limits are enforced.
func reportLimits() int { return 2 }
