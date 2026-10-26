//go:build darwin && arm64

package sidecar

import "embed"

// embeddedFiles holds the sidecars fetched for this platform by
// `mise run sidecar:fetch` (<name>.gz), plus a placeholder that keeps the
// directory present in a fresh checkout.
//
//go:embed all:bin/darwin_arm64
var embeddedFiles embed.FS

const embeddedDir = "bin/darwin_arm64"
