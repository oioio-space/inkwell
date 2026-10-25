package typst

import (
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"testing"
	"testing/fstest"
)

func TestFontCache(t *testing.T) {
	newFonts := func(t *testing.T) *fontCache {
		t.Helper()
		return &fontCache{root: filepath.Join(t.TempDir(), "fonts"), fsys: embeddedFonts}
	}

	t.Run("the bundled files match their pins and the license is included", func(t *testing.T) {
		pins, err := loadFonts()
		if err != nil {
			t.Fatalf("loadFonts() = %v", err)
		}
		var names []string
		for _, f := range pins.Files {
			names = append(names, f.Name)
			data := mustReadFS(t, embeddedFonts, f.Name)
			if got := sumHex(data); got != f.SHA256 {
				t.Errorf("%s sha256 = %s, pinned %s", f.Name, got, f.SHA256)
			}
		}
		slices.Sort(names)
		want := []string{"Inter-Bold.ttf", "Inter-BoldItalic.ttf", "Inter-LICENSE.txt", "Inter-Italic.ttf", "Inter-Regular.ttf"}
		slices.Sort(want)
		if !slices.Equal(names, want) {
			t.Errorf("fonts = %v, want %v", names, want)
		}
		if pins.License != "OFL-1.1" {
			t.Errorf("license = %q, want OFL-1.1", pins.License)
		}
	})

	t.Run("first use extracts and a clean cache is left alone", func(t *testing.T) {
		c := newFonts(t)
		changed, err := c.Ensure()
		if err != nil || !changed {
			t.Fatalf("Ensure() = %v, %v; want it to extract", changed, err)
		}
		if _, err := os.Stat(filepath.Join(c.root, "Inter-Regular.ttf")); err != nil {
			t.Error(err)
		}
		changed, err = c.Ensure()
		if err != nil || changed {
			t.Errorf("second Ensure() = %v, %v; want nothing to do", changed, err)
		}
	})

	t.Run("a damaged or extra font is replaced", func(t *testing.T) {
		c := newFonts(t)
		if _, err := c.Ensure(); err != nil {
			t.Fatal(err)
		}
		mustWrite(t, filepath.Join(c.root, "Evil.ttf"), []byte("x"))
		changed, err := c.Ensure()
		if err != nil || !changed {
			t.Fatalf("Ensure() = %v, %v; want a repair", changed, err)
		}
		if _, err := os.Stat(filepath.Join(c.root, "Evil.ttf")); !os.IsNotExist(err) {
			t.Errorf("the extra font is still there: %v", err)
		}
	})

	t.Run("an embedded file that is not the pinned one is refused", func(t *testing.T) {
		c := newFonts(t)
		c.fsys = fstest.MapFS{"Inter-Regular.ttf": {Data: []byte("not Inter")}}
		if _, err := c.Ensure(); err == nil {
			t.Fatal("Ensure() = nil, want an error")
		}
	})
}

func mustReadFS(t testing.TB, fsys fs.FS, name string) []byte {
	t.Helper()
	data, err := fs.ReadFile(fsys, name)
	if err != nil {
		t.Fatal(err)
	}
	return data
}
