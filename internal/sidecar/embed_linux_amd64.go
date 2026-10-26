//go:build linux && amd64

package sidecar

import "embed"

// embeddedFiles holds the sidecars fetched for this platform by
// `mise run sidecar:fetch` (<name>.gz), plus a placeholder that keeps the
// directory present in a fresh checkout.
//
//go:embed all:bin/linux_amd64
var embeddedFiles embed.FS

const embeddedDir = "bin/linux_amd64"
