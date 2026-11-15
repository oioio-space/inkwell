//go:build !linux

package sidecar

// There is no unprivileged network namespace here; [Run] refuses every launch
// (spec §9.11), so there is no tier to report.
func networkIsolation(*Store) (bool, error) { return false, confinementAvailable() }
