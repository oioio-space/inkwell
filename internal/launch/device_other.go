//go:build !unix

package launch

import "os"

// device tells only whether path exists where stat gives no device number (Windows, N35): the
// walk then stops at the volume root, never at a mounted folder.
func device(path string) (uint64, bool) {
	_, err := os.Stat(path)
	return 0, err == nil
}
