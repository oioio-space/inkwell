//go:build !linux && !darwin

package project

import "io/fs"

// Platforms without a reliable change time re-read content rather than trust mtime.
func snapshotFileVersion(fs.FileInfo) string { return "" }
