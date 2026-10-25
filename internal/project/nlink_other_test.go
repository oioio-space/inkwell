//go:build !unix

package project

import (
	"os"
	"testing"
)

// linkCount cannot tell on this platform; it reports the expected single link.
func linkCount(*testing.T, *os.Root, string) uint64 { return 1 }
