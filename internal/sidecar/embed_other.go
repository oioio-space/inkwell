//go:build !((linux || darwin) && (amd64 || arm64))

package sidecar

import "embed"

// embeddedFiles is empty on a platform without pinned sidecars: the manifest
// has no target for it, so [Store.Resolve] reports [ErrUnsupported].
var embeddedFiles embed.FS

const embeddedDir = "."
