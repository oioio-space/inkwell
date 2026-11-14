//go:build darwin || windows

package instance

import "os"

// runtimeDir returns the folder of the second socket candidate: os.TempDir(), which is private to
// the user on macOS (/var/folders/…/T) and on Windows (%LOCALAPPDATA%\Temp). Built and tested at
// N35.
func runtimeDir(bool) string { return os.TempDir() }
