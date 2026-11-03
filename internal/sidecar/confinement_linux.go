package sidecar

import "os/exec"

// configureConfinement matches the overlay policy encoded by withLimits.
func configureConfinement(cmd *exec.Cmd) error {
	supported, err := probeOutputMount()
	if err != nil {
		return err
	}
	if supported {
		mountNamespace(cmd)
	}
	return nil
}

func confinementAvailable() error { return nil }
