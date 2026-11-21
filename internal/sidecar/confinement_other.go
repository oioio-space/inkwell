//go:build !linux

package sidecar

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
)

// Spec 9.11: no launch until confinement is proven on this platform.
func confinementAvailable() error {
	return errors.New("aperçu indisponible sur ce système pour les documents reçus")
}

// launchPlan has no meaning where no launch is allowed.
type launchPlan struct{}

func planLaunch(Spec) (launchPlan, error) { return launchPlan{}, confinementAvailable() }

func tierNamespaces(*exec.Cmd, launchPlan) {}

type handshake struct {
	afterReport func()
	skipProceed bool
}

func newHandshake(*exec.Cmd, launchPlan) (*handshake, error) { return nil, confinementAvailable() }
func (*handshake) await(context.Context) error               { return confinementAvailable() }
func (*handshake) close()                                    {}

func (launchPlan) isolated() bool { return false }
func (launchPlan) mounted() bool  { return false }

func launcherExit(code int) error {
	return fmt.Errorf("sidecar: the launcher failed with status %d", code)
}
