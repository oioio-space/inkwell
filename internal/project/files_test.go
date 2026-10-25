package project

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"
)

func put(t *testing.T, p *Project, rel, content string) {
	t.Helper()
	if dir := filepath.Dir(rel); dir != "." {
		if err := p.root.MkdirAll(filepath.ToSlash(dir), 0o700); err != nil {
			t.Fatal(err)
		}
	}
	if err := p.root.WriteFile(rel, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

func paths(files []File) []string {
	out := make([]string, len(files))
	for i, f := range files {
		out[i] = f.Path
	}
	return out
}

func TestList(t *testing.T) {
	t.Run("TXT-12 hides .inkwell, dotfiles, temporary files and links leaving the project", func(t *testing.T) {
		base := newBase(t)
		p := create(t, base, "p")
		outside := t.TempDir()
		if err := os.WriteFile(filepath.Join(outside, "secret.txt"), []byte("s"), 0o600); err != nil {
			t.Fatal(err)
		}
		put(t, p, "main.typ", "= A\n")
		put(t, p, "notes.txt", "n")
		put(t, p, ".hidden", "h")
		put(t, p, ".git/config", "c")
		put(t, p, "sub/.env", "e")
		put(t, p, "sub/ok.md", "# ok")
		put(t, p, "main.typ.abc.tmp", "left by a crash")
		root := p.root.Name()
		for link, target := range map[string]string{
			"escape.txt": filepath.Join(outside, "secret.txt"),
			"escdir":     outside,
			"inside.txt": "notes.txt",
			"dirlink":    "sub",
			"dangling":   "nowhere",
		} {
			if err := os.Symlink(target, filepath.Join(root, link)); err != nil {
				t.Skipf("symlinks unavailable: %v", err)
			}
		}

		files, err := p.List()
		if err != nil {
			t.Fatal(err)
		}
		want := []string{"sub/ok.md", "inside.txt", "main.typ", "notes.txt"}
		if got := paths(files); !slices.Equal(got, want) {
			t.Errorf("List paths = %q, want %q", got, want)
		}
	})

	t.Run("kind, size and main", func(t *testing.T) {
		p := create(t, newBase(t), "p")
		put(t, p, "main.typ", "= A\n")
		put(t, p, "chapters/ch1.typ", "x")
		put(t, p, "readme.MD", "# r")
		put(t, p, "data.csv", "a,b")
		put(t, p, "refs.bib", "@a{}")
		put(t, p, "fig.PNG", "\x89PNG\r\n\x1a\n\x00")
		put(t, p, "doc.pdf", "%PDF")
		put(t, p, "conf.json", "{}")
		put(t, p, "LICENSE", "Mozilla Public License\n")
		put(t, p, "weights.bin", "\x00\x01\x02")
		put(t, p, "notes.txt", "n")

		files, err := p.List()
		if err != nil {
			t.Fatal(err)
		}
		got := map[string]File{}
		for _, f := range files {
			got[f.Path] = f
		}
		for path, want := range map[string]Kind{
			"main.typ": KindTypst, "chapters/ch1.typ": KindTypst, "readme.MD": KindMarkdown,
			"data.csv": KindCSV, "refs.bib": KindBib, "fig.PNG": KindImage, "doc.pdf": KindPDF,
			"conf.json": KindData, "LICENSE": KindText, "weights.bin": KindOther, "notes.txt": KindText,
		} {
			if got[path].Kind != want {
				t.Errorf("Kind(%s) = %v, want %v", path, got[path].Kind, want)
			}
		}
		if !got["main.typ"].Main || got["chapters/ch1.typ"].Main || got["main.typ"].Size != 4 {
			t.Errorf("main.typ = %+v, chapters/ch1.typ = %+v", got["main.typ"], got["chapters/ch1.typ"])
		}
	})

	t.Run("sorts directories first, then naturally", func(t *testing.T) {
		p := create(t, newBase(t), "p")
		for _, rel := range []string{
			"ch10.typ", "ch2.typ", "Ch1.typ", "a.typ", "zdir/x.typ", "adir/ch10.typ", "adir/ch9.typ", "adir/sub/z.typ",
		} {
			put(t, p, rel, "x")
		}
		files, err := p.List()
		if err != nil {
			t.Fatal(err)
		}
		want := []string{
			"adir/sub/z.typ", "adir/ch9.typ", "adir/ch10.typ", "zdir/x.typ", "a.typ", "Ch1.typ", "ch2.typ", "ch10.typ",
		}
		if got := paths(files); !slices.Equal(got, want) {
			t.Errorf("List paths = %q, want %q", got, want)
		}
	})
}

func TestNaturalLess(t *testing.T) {
	for _, c := range [][2]string{
		{"a", "a1"},
		{"a1", "a2"},
		{"a2", "a10"},
		{"ch2", "ch10"},
		{"ch10", "ch010"},
		{"Ch1", "ch2"},
		{"a", "B"},
		{"B", "c"},
		{"a.typ", "a1.typ"},
	} {
		if !naturalLess(c[0], c[1]) || naturalLess(c[1], c[0]) {
			t.Errorf("want %q before %q", c[0], c[1])
		}
	}
	if naturalLess("x", "x") {
		t.Error("naturalLess(x, x) = true")
	}
}

// buildTree creates n small files spread over n/100 directories.
func buildTree(tb testing.TB, p *Project, n int) {
	tb.Helper()
	for d := range n / 100 {
		dir := fmt.Sprintf("d%03d", d)
		if err := p.root.Mkdir(dir, 0o700); err != nil {
			tb.Fatal(err)
		}
		for i := range 100 {
			if err := p.root.WriteFile(fmt.Sprintf("%s/ch%d.typ", dir, i), []byte("x"), 0o600); err != nil {
				tb.Fatal(err)
			}
		}
	}
}

func TestListLargeProject(t *testing.T) {
	t.Run("TXT-13 10000 files are listed within the budget and in natural order", func(t *testing.T) {
		p := create(t, newBase(t), "p")
		buildTree(t, p, 10_000)
		// Best of three: the first listing also pays for cold directory caches.
		var files []File
		elapsed := time.Hour
		for range 3 {
			start := time.Now()
			var err error
			if files, err = p.List(); err != nil {
				t.Fatal(err)
			}
			elapsed = min(elapsed, time.Since(start))
		}
		if len(files) != 10_000 {
			t.Fatalf("List returned %d files, want 10000", len(files))
		}
		if files[0].Path != "d000/ch0.typ" || files[10].Path != "d000/ch10.typ" || files[100].Path != "d001/ch0.typ" {
			t.Errorf("order: %q %q %q", files[0].Path, files[10].Path, files[100].Path)
		}
		if budget := listBudget; elapsed > budget {
			t.Errorf("List of 10000 files took %v, budget %v", elapsed, budget)
		}
		for _, f := range files {
			if strings.HasPrefix(f.Path, ".") {
				t.Fatalf("listed %q", f.Path)
			}
		}
	})
}

func BenchmarkList(b *testing.B) {
	base, err := os.OpenRoot(b.TempDir())
	if err != nil {
		b.Fatal(err)
	}
	b.Cleanup(func() { _ = base.Close() })
	p, err := Create(base, "p", "")
	if err != nil {
		b.Fatal(err)
	}
	b.Cleanup(func() { _ = p.Close() })
	buildTree(b, p, 10_000)
	b.ReportAllocs()
	for b.Loop() {
		if _, err := p.List(); err != nil {
			b.Fatal(err)
		}
	}
}
