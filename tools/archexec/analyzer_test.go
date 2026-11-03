package archexec

import (
	"testing"

	"golang.org/x/tools/go/analysis/analysistest"
)

func TestBypasses(t *testing.T) {
	t.Setenv("CGO_ENABLED", "0")
	analysistest.Run(t, analysistest.TestData(), Analyzer, "bypass", "native", "desktop")
}

// The desktop-only launch must be type checked on every shipped GOOS, while
// the same package's server build excludes that file.
func TestDesktopBuildConfigurations(t *testing.T) {
	for _, target := range []string{"linux", "darwin", "windows"} {
		for _, mode := range []string{"server", "production"} {
			t.Run(target+"/"+mode, func(t *testing.T) {
				t.Setenv("GOOS", target)
				t.Setenv("GOARCH", "amd64")
				cgo := "0"
				if target == "linux" {
					cgo = "1"
				}
				t.Setenv("CGO_ENABLED", cgo)
				t.Setenv("GOFLAGS", "-tags="+mode)
				analysistest.Run(t, analysistest.TestData(), Analyzer, "desktop")
			})
		}
	}
}
