package sidecar

import "os/exec"

// configureConfinement matches the overlay policy encoded by withLimits.
func configureConfinement(cmd *exec.Cmd) error {
	if probeOutputMount() {
		mountNamespace(cmd)
	}
	return nil
}

func confinementAvailable() error { return nil }
