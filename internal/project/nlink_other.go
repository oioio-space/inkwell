//go:build !unix

package project

import "io/fs"

// hardLinks cannot tell on this platform; it reports a single name.
func hardLinks(fs.FileInfo) uint64 { return 1 }
