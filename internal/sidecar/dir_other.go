//go:build !linux

package sidecar

import "errors"

// Bounded disk output has no portable fallback; never silently use polling.
func directoryLimits(spec Spec) (Spec, []string, error) {
	if spec.WatchDir != "" && spec.MaxDirBytes > 0 {
		return spec, nil, errors.New("sidecar: hard directory bounds are unsupported on this OS")
	}
	return spec, nil, nil
}

func directoryLimitHit(Spec, Output) bool { return false }

func cleanupEmptyOutputs(Spec) {}
