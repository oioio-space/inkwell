//go:build !linux

package sidecar

import (
	"errors"
	"os/exec"
)

// Spec 9.11: no launch until confinement is proven on this platform.
func confinementAvailable() error {
	return errors.New("aperçu indisponible sur ce système pour les documents reçus")
}

func configureConfinement(_ *exec.Cmd) error { return confinementAvailable() }
