package sidecar

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestFontFoldersExactlyThoseGiven(t *testing.T) {
	for _, tr := range []tier{tierN, tierF} {
		t.Run(tr.String(), func(t *testing.T) {
			if !requireTier(t, tr) {
				return
			}
			// Outside the project: project files are readable regardless of ReadPaths.
			fonts := t.TempDir()
			font := filepath.Join(fonts, "private-font.ttf")
			if err := os.WriteFile(font, []byte("font sentinel"), 0o600); err != nil {
				t.Fatalf("S-8 fonts: %v", err)
			}
			for _, allowed := range []bool{false, true} {
				s := spec("read:" + font)
				s.Dir = t.TempDir()
				s.tier = tr
				if allowed {
					s.ReadPaths = []string{fonts}
				}
				out, err := Run(t.Context(), s)
				want := "read: EACCES\n"
				if allowed {
					want = "read: ok\n"
				}
				if err != nil || out.ExitCode != 0 || string(out.Stdout) != want {
					t.Fatalf("S-8 fonts: %s allowed=%t: %+v %v", tr, allowed, out, err)
				}
			}
			// A system font outside the list must also remain unreadable. The custom
			// sentinel alone would miss the old hard-coded /usr/share/fonts fallback.
			var systemFont string
			_ = filepath.WalkDir("/usr/share/fonts", func(path string, d os.DirEntry, err error) error {
				if err == nil && !d.IsDir() && strings.HasSuffix(strings.ToLower(path), ".ttf") {
					systemFont = path
					return filepath.SkipAll
				}
				return nil
			})
			if systemFont == "" {
				t.Fatal("S-8 fonts: no system font for the exclusion witness")
			}
			s := spec("read:" + systemFont)
			s.Dir = t.TempDir()
			s.tier = tr
			out, err := Run(t.Context(), s)
			if err != nil || out.ExitCode != 0 || string(out.Stdout) != "read: EACCES\n" {
				t.Fatalf("S-8 fonts: unlisted system font: %+v %v", out, err)
			}
		})
	}
	t.Run("F-provided-font", func(t *testing.T) {
		if !requireLaunch(t) {
			return
		}
		requireEmbedded(t)
		source := "../typst/fonts/Inter-Regular.ttf"
		font, err := os.ReadFile(source)
		if err != nil {
			t.Fatalf("S-8 fonts: fixture font: %v", err)
		}
		project := t.TempDir()
		fonts := filepath.Join(project, "fonts")
		if err := os.Mkdir(fonts, 0o700); err != nil {
			t.Fatalf("S-8 fonts: %v", err)
		}
		if err := os.WriteFile(filepath.Join(fonts, "provided.ttf"), font, 0o600); err != nil {
			t.Fatalf("S-8 fonts: %v", err)
		}
		if err := os.WriteFile(filepath.Join(project, "main.typ"), []byte(`#set text(font: "Inter")
Provided font.`), 0o600); err != nil {
			t.Fatalf("S-8 fonts: %v", err)
		}
		binary, err := (&Store{Dir: sharedCache}).Resolve(Typst)
		if err != nil {
			t.Fatalf("S-8 fonts: %v", err)
		}
		outdir := t.TempDir()
		pdf := filepath.Join(outdir, "out.pdf")
		s := Spec{Path: binary, Dir: project, tier: tierF, ReadPaths: []string{fonts}, WatchDir: outdir, WriteFiles: []string{"out.pdf"}, MaxDirBytes: 16 << 20, Args: []string{"compile", "--ignore-system-fonts", "--font-path", fonts, "main.typ", pdf}}
		ctx, cancel := context.WithTimeout(t.Context(), 15*time.Second)
		defer cancel()
		out, err := Run(ctx, s)
		data, readErr := os.ReadFile(pdf)
		if err != nil || readErr != nil || out.ExitCode != 0 || !bytes.Contains(data, []byte("Inter")) || !bytes.Contains(data, []byte("/FontFile2")) {
			t.Fatalf("S-8 fonts: provided font not embedded: %+v %v read=%v", out, err, readErr)
		}
	})
}
