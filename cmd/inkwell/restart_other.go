//go:build !linux

package main

import "errors"

// canRestart: this build cannot restart itself, so the take-over is not offered (residual R-8).
const canRestart = false

// errRestartUnsupported: the restart is specified for Linux only; the macOS and Windows ports
// implement the spawn restart of the model (residual R-8 of the PC1 plan).
var errRestartUnsupported = errors.New("restart: unsupported on this platform")

// restartImage reports that the restart is unsupported here.
func restartImage(string) error { return errRestartUnsupported }
