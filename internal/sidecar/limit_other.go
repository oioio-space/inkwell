//go:build !linux

package sidecar

// The former unconfined fallback relied on this insufficient guarantee:
// on Linux. The timeout, the output bounds and the directory watch still apply.
// Spec 9.11 now requires refusal before launch instead.
func withLimits(_ Spec) (string, []string, error) {
	return "", nil, confinementAvailable()
}
