//go:build perf_native && !production && linux && !server

package main

// Wails queries the actual linked libraries; never use user-agent or build versions.
func nativeEngineVersions(platform map[string]any) (string, string) {
	webkit, _ := platform["webkitgtk6-runtime"].(string)
	gtk, _ := platform["gtk4-runtime"].(string)
	return webkit, gtk
}
