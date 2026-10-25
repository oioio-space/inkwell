//go:build !linux

package sidecar

// limitMemory is a no-op: RLIMIT_AS is only enforced on Linux. The timeout and
// the output bounds still apply.
func limitMemory(int, uint64) error { return nil }
