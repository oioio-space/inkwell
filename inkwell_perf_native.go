//go:build perf_native && !production && linux && !server

package inkwell

import (
	"embed"
	"io/fs"

	"github.com/oioio-space/inkwell/internal/nativeperf"
)

//go:embed all:frontend/perf-dist
var nativeAssets embed.FS

// Assets contains only the dedicated performance build's webview resources.
// Normal and production builds embed frontend/dist from inkwell.go instead.
var Assets = func() fs.FS {
	assets, err := fs.Sub(nativeAssets, "frontend/perf-dist")
	if err != nil {
		panic(err)
	}
	return nativeperf.ClockAssets(assets)
}()
