//go:build !unix && !windows

package blob

import "errors"

// freeSpace is not available on this platform; the store then skips the check.
func freeSpace(string) (uint64, error) { return 0, errors.ErrUnsupported }
