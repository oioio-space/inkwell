package typst

import (
	"bytes"
	"crypto/sha256"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sync"
)

//go:embed fonts/*
var fontsFS embed.FS

//go:embed fonts.json
var fontsJSON []byte

// embeddedFonts holds the fonts shipped in the executable: Inter 4.1 (SIL OFL
// 1.1) with its license. Typst itself already carries Libertinus Serif, New
// Computer Modern and DejaVu Sans Mono; these add a sans-serif text face so
// that slides and notes do not depend on what the machine has installed.
var embeddedFonts = func() fs.FS {
	sub, err := fs.Sub(fontsFS, "fonts")
	if err != nil { // "fonts" is a constant that exists.
		panic(err)
	}
	return sub
}()

// fontPins is fonts.json.
type fontPins struct {
	Family  string `json:"family"`
	Version string `json:"version"`
	License string `json:"license"`
	Source  string `json:"source"`
	Files   []struct {
		Name   string `json:"name"`
		SHA256 string `json:"sha256"`
	} `json:"files"`
	Schema int `json:"schema"`
}

func loadFonts() (*fontPins, error) {
	dec := json.NewDecoder(bytes.NewReader(fontsJSON))
	dec.DisallowUnknownFields()
	var pins fontPins
	if err := dec.Decode(&pins); err != nil {
		return nil, fmt.Errorf("typst: fonts.json: %w", err)
	}
	if pins.Schema != 1 || len(pins.Files) == 0 {
		return nil, errors.New("typst: fonts.json: unknown schema or no files")
	}
	return &pins, nil
}

// fontCache keeps the bundled fonts extracted in root, which is passed to
// Typst with --font-path. Like the packages, they are checked on every use.
type fontCache struct {
	fsys fs.FS
	want map[string]fileSum
	root string
	mu   sync.Mutex
}

func newFontCache(root string) *fontCache {
	return &fontCache{root: root, fsys: embeddedFonts}
}

// Ensure makes root hold exactly the bundled fonts and reports whether it had
// to write them.
func (c *fontCache) Ensure() (bool, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.want == nil {
		pins, err := loadFonts()
		if err != nil {
			return false, err
		}
		want := make(map[string]fileSum, len(pins.Files))
		for _, f := range pins.Files {
			data, err := fs.ReadFile(c.fsys, f.Name)
			if err != nil {
				return false, fmt.Errorf("typst: bundled font %s: %w", f.Name, err)
			}
			if got := sumHex(data); got != f.SHA256 {
				return false, fmt.Errorf("%w: bundled font %s has sha256 %s, pinned %s", ErrIntegrity, f.Name, got, f.SHA256)
			}
			want[f.Name] = fileSum{sum: sha256.Sum256(data), size: int64(len(data))}
		}
		c.want = want
	}
	if verifyTree(c.root, c.want) == nil {
		return false, nil
	}
	err := installTree(c.root, c.want, func(tmp string) error {
		for name := range c.want {
			data, err := fs.ReadFile(c.fsys, name)
			if err != nil {
				return err
			}
			if err := os.WriteFile(filepath.Join(tmp, name), data, 0o600); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return false, fmt.Errorf("typst: install fonts: %w", err)
	}
	return true, nil
}
