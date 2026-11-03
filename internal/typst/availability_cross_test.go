//go:build linux

package typst

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// Compile the refusal assertions themselves, including their public API calls.
func TestUnsupportedPlatformsCompile(t *testing.T) {
	for _, platform := range []string{"darwin", "windows"} {
		t.Run(platform, func(t *testing.T) {
			cmd := exec.Command("go", "test", "-c", "-o", filepath.Join(t.TempDir(), "typst.test"), ".")
			cmd.Env = append(os.Environ(), "GOOS="+platform, "CGO_ENABLED=0")
			if out, err := cmd.CombinedOutput(); err != nil {
				t.Fatalf("%v: %s", err, out)
			}
		})
	}
}
