package instance

import (
	"crypto/sha256"
	"encoding/hex"
	"path/filepath"
)

const (
	// lockName is the lock file in the profile folder.
	lockName = "instance.lock"
	// sockName is the first socket candidate, beside the lock.
	sockName = "instance.sock"
	// maxSunPath is the longest socket path on every supported system: sun_path is 108 bytes on
	// Linux and Windows and 104 on macOS, and holds the terminating NUL.
	maxSunPath = 103
)

// socketPaths returns the socket candidates of the canonical profile folder dir, in the order
// both ends try them: dir/instance.sock, then inkwell-<first 32 hex digits of SHA-256(dir)>.sock
// in the private runtime folder runtime ("" when there is none). A candidate longer than
// maxSunPath is left out.
func socketPaths(dir, runtime string) []string {
	var out []string
	if p := filepath.Join(dir, sockName); len(p) <= maxSunPath {
		out = append(out, p)
	}
	if runtime != "" {
		sum := sha256.Sum256([]byte(dir))
		if p := filepath.Join(runtime, "inkwell-"+hex.EncodeToString(sum[:16])+".sock"); len(p) <= maxSunPath {
			out = append(out, p)
		}
	}
	return out
}
