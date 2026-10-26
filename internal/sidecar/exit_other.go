//go:build !unix

package sidecar

import "os"

func exitSignal(*os.ProcessState) int { return 0 }
