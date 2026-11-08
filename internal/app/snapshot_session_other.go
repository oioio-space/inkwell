//go:build !unix

package app

import (
	"errors"
	"os"
)

func newSnapshotSession(string) (string, *os.File, error) {
	return "", nil, errors.New("snapshot process leases unavailable on this platform")
}
