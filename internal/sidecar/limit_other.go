//go:build !linux

package sidecar

// withLimits does nothing: the memory and file size limits are only enforced
// on Linux. The timeout, the output bounds and the directory watch still apply.
func withLimits(spec Spec) (string, []string, error) {
	return spec.Path, spec.Args, nil
}
