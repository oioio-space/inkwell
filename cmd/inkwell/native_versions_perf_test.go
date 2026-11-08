//go:build perf_native && !production && linux && !server

package main

import (
	"go/build"
	"path/filepath"
	"testing"
)

func TestNativeEngineVersionsUsesRuntimeMetadata(t *testing.T) {
	webkit, gtk := nativeEngineVersions(map[string]any{
		"webkitgtk6-runtime": "2.50.1", "gtk4-runtime": "4.20.2",
		"webkitgtk6-compiled": "wrong", "gtk4-compiled": "wrong",
	})
	if webkit != "2.50.1" || gtk != "4.20.2" {
		t.Fatalf("runtime versions = %q, %q", webkit, gtk)
	}
	webkit, gtk = nativeEngineVersions(map[string]any{"gtk4-runtime": 42})
	if webkit != "" || gtk != "" {
		t.Fatalf("missing metadata must remain unavailable: %q, %q", webkit, gtk)
	}
}

func TestNativeAssetsAreAbsentFromNormalAndProductionBuilds(t *testing.T) {
	for _, tc := range []struct {
		name         string
		tags         []string
		normal, perf bool
	}{
		{name: "normal", normal: true},
		{name: "production", tags: []string{"production"}, normal: true},
		{name: "native-perf", tags: []string{"perf_native"}, perf: true},
		{name: "production-perf-refused", tags: []string{"production", "perf_native"}},
		{name: "server-perf-refused", tags: []string{"server", "perf_native"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx := build.Default
			ctx.GOOS = "linux"
			ctx.CgoEnabled = true
			ctx.BuildTags = tc.tags
			for file, want := range map[string]bool{"inkwell.go": tc.normal, "inkwell_perf_native.go": tc.perf} {
				got, err := ctx.MatchFile(filepath.Join("..", ".."), file)
				if err != nil || got != want {
					t.Fatalf("%s with %v: included=%v want=%v error=%v", file, tc.tags, got, want, err)
				}
			}
		})
	}
}
