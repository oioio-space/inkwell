//go:build linux

package sidecar

import (
	"os/signal"
	"syscall"
)

func ignoreFileSizeSignal() { signal.Ignore(syscall.SIGXFSZ) }
